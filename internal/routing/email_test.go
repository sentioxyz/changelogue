package routing

import (
	"context"
	"encoding/json"
	"net"
	"strings"
	"testing"

	"github.com/sentioxyz/changelogue/internal/models"
)

// mockSMTPServer starts a minimal SMTP server that captures the DATA payload.
// Returns the listener and a channel that receives the raw email message.
func mockSMTPServer(t *testing.T) (net.Listener, <-chan string) {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	ch := make(chan string, 1)

	go func() {
		conn, err := ln.Accept()
		if err != nil {
			return
		}
		defer conn.Close()

		write := func(s string) { conn.Write([]byte(s + "\r\n")) }
		read := func() string {
			buf := make([]byte, 4096)
			n, _ := conn.Read(buf)
			return string(buf[:n])
		}

		write("220 localhost ESMTP")
		cmd := read()
		if strings.HasPrefix(cmd, "EHLO") {
			write("250-localhost")
			write("250 AUTH PLAIN LOGIN")
		}
		read() // AUTH
		write("235 Authentication successful")
		read() // MAIL FROM
		write("250 OK")
		read() // RCPT TO
		write("250 OK")

		// Handle additional RCPT TO commands
		for {
			line := read()
			if strings.HasPrefix(line, "RCPT TO") {
				write("250 OK")
				continue
			}
			if strings.HasPrefix(line, "DATA") {
				write("354 Start mail input")
				break
			}
		}

		// Read DATA until lone "."
		var data strings.Builder
		for {
			line := read()
			data.WriteString(line)
			if strings.Contains(line, "\r\n.\r\n") {
				break
			}
		}
		write("250 OK")
		ch <- data.String()

		read() // QUIT
		write("221 Bye")
	}()

	return ln, ch
}

func TestEmailSender_Send(t *testing.T) {
	ln, dataCh := mockSMTPServer(t)
	defer ln.Close()

	addr := ln.Addr().String()
	host, port, _ := net.SplitHostPort(addr)

	sender := &EmailSender{}
	ch := &models.NotificationChannel{
		Type: "email",
		Config: json.RawMessage(`{
			"smtp_host": "` + host + `",
			"smtp_port": ` + port + `,
			"username": "user",
			"password": "pass",
			"from_address": "noreply@example.com",
			"to_addresses": ["team@example.com"]
		}`),
	}
	msg := Notification{
		Title:      "geth",
		Body:       `{"changelog":"Security fixes and performance improvements"}`,
		Version:    "v1.14.0",
		Provider:   "github",
		Repository: "ethereum/go-ethereum",
		SourceURL:  "https://github.com/ethereum/go-ethereum/releases/tag/v1.14.0",
	}

	err := sender.Send(context.Background(), ch, msg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	data := <-dataCh
	if !strings.Contains(data, "Content-Type: multipart/alternative") {
		t.Fatal("expected multipart/alternative content type")
	}
	if !strings.Contains(data, "text/plain") {
		t.Fatal("expected text/plain part")
	}
	if !strings.Contains(data, "text/html") {
		t.Fatal("expected text/html part")
	}
	if !strings.Contains(data, "geth v1.14.0") {
		t.Fatal("expected subject to contain project name and version")
	}
}

func TestEmailSender_SemanticReport(t *testing.T) {
	ln, dataCh := mockSMTPServer(t)
	defer ln.Close()

	addr := ln.Addr().String()
	host, port, _ := net.SplitHostPort(addr)

	sender := &EmailSender{}
	ch := &models.NotificationChannel{
		Type: "email",
		Config: json.RawMessage(`{
			"smtp_host": "` + host + `",
			"smtp_port": ` + port + `,
			"username": "user",
			"password": "pass",
			"from_address": "noreply@example.com",
			"to_addresses": ["team@example.com"]
		}`),
	}

	reportJSON := `{
		"subject": "Ready to Deploy: go-ethereum v1.16.4",
		"urgency": "High",
		"urgency_reason": "Security vulnerability patched",
		"changelog_summary": "Security fixes and performance improvements",
		"download_commands": ["docker pull ethereum/client-go:v1.16.4"]
	}`

	msg := Notification{
		Title:       "go-ethereum v1.16.4",
		Body:        reportJSON,
		Version:     "v1.16.4",
		ProjectName: "go-ethereum",
		Provider:    "github",
		Repository:  "ethereum/go-ethereum",
		SourceURL:   "https://github.com/ethereum/go-ethereum/releases/tag/v1.16.4",
		ReleaseURL:  "https://changelogue.example.com/sr/1",
	}

	err := sender.Send(context.Background(), ch, msg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	data := <-dataCh
	if !strings.Contains(data, "High") {
		t.Fatal("expected urgency level in email")
	}
	if !strings.Contains(data, "Security fixes and performance improvements") {
		t.Fatal("expected changelog summary in email")
	}
	if !strings.Contains(data, "docker pull") {
		t.Fatal("expected download command in email")
	}
}

func TestEmailSender_InvalidConfig(t *testing.T) {
	sender := &EmailSender{}
	ch := &models.NotificationChannel{
		Type:   "email",
		Config: json.RawMessage(`{invalid`),
	}

	err := sender.Send(context.Background(), ch, Notification{})
	if err == nil {
		t.Fatal("expected error for invalid config")
	}
}

func TestEmailSender_MultipleRecipients(t *testing.T) {
	ln, dataCh := mockSMTPServer(t)
	defer ln.Close()

	addr := ln.Addr().String()
	host, port, _ := net.SplitHostPort(addr)

	sender := &EmailSender{}
	ch := &models.NotificationChannel{
		Type: "email",
		Config: json.RawMessage(`{
			"smtp_host": "` + host + `",
			"smtp_port": ` + port + `,
			"username": "user",
			"password": "pass",
			"from_address": "noreply@example.com",
			"to_addresses": ["a@example.com", "b@example.com"]
		}`),
	}
	msg := Notification{
		Title:   "test",
		Body:    `{"changelog":"test changelog"}`,
		Version: "v1.0.0",
	}

	err := sender.Send(context.Background(), ch, msg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	data := <-dataCh
	if !strings.Contains(data, "a@example.com") || !strings.Contains(data, "b@example.com") {
		t.Fatal("expected both recipients in To header")
	}
}
