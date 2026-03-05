# Email Notification Channel Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Add email as a notification channel so users can receive release alerts via SMTP email.

**Architecture:** Implements the `Sender` interface with an `EmailSender` struct. Uses Go's `net/smtp` for STARTTLS delivery and `html/template` with `//go:embed` for HTML email rendering. Multipart MIME (HTML + plain text).

**Tech Stack:** Go stdlib (`net/smtp`, `html/template`, `mime/multipart`, `crypto/tls`)

---

### Task 1: Create HTML email template

**Files:**
- Create: `internal/routing/email.html.tmpl`

**Step 1: Write the HTML template**

Create `internal/routing/email.html.tmpl` with this content:

```html
<!DOCTYPE html>
<html>
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1.0">
<title>{{.Subject}}</title>
</head>
<body style="margin:0;padding:0;background-color:#f4f4f5;font-family:-apple-system,BlinkMacSystemFont,'Segoe UI',Roboto,sans-serif;">
<table width="100%" cellpadding="0" cellspacing="0" style="background-color:#f4f4f5;padding:24px 0;">
<tr><td align="center">
<table width="600" cellpadding="0" cellspacing="0" style="background-color:#ffffff;border-radius:8px;overflow:hidden;">

<!-- Header -->
<tr><td style="background-color:{{.UrgencyColor}};padding:20px 24px;">
<span style="color:#ffffff;font-size:20px;font-weight:600;">{{.Subject}}</span>
</td></tr>

{{if .UrgencyBadge}}
<!-- Urgency Badge -->
<tr><td style="padding:16px 24px 0;">
<span style="display:inline-block;padding:4px 12px;border-radius:4px;background-color:{{.UrgencyColor}};color:#ffffff;font-size:13px;font-weight:600;">{{.UrgencyBadge}}</span>
{{if .UrgencyReason}}<p style="margin:8px 0 0;color:#71717a;font-size:14px;">{{.UrgencyReason}}</p>{{end}}
</td></tr>
{{end}}

{{if .Summary}}
<!-- Summary -->
<tr><td style="padding:16px 24px 0;">
<p style="margin:0;color:#27272a;font-size:15px;line-height:1.6;">{{.Summary}}</p>
</td></tr>
{{end}}

{{if .Changelog}}
<!-- Changelog -->
<tr><td style="padding:16px 24px 0;">
<div style="background-color:#f4f4f5;border-radius:6px;padding:12px 16px;">
<pre style="margin:0;font-family:'SF Mono',Monaco,Consolas,monospace;font-size:13px;line-height:1.5;color:#3f3f46;white-space:pre-wrap;word-wrap:break-word;">{{.Changelog}}</pre>
</div>
</td></tr>
{{end}}

{{if .DownloadCommand}}
<!-- Download Command -->
<tr><td style="padding:16px 24px 0;">
<code style="display:block;background-color:#18181b;color:#e4e4e7;padding:10px 14px;border-radius:6px;font-family:'SF Mono',Monaco,Consolas,monospace;font-size:13px;">{{.DownloadCommand}}</code>
</td></tr>
{{end}}

<!-- Links -->
<tr><td style="padding:16px 24px;">
{{if .SourceURL}}<a href="{{.SourceURL}}" style="color:#2563eb;font-size:14px;text-decoration:none;">View on {{.ProviderLabel}}</a>{{end}}
{{if and .SourceURL .ReleaseURL}}<span style="color:#d4d4d8;margin:0 8px;">|</span>{{end}}
{{if .ReleaseURL}}<a href="{{.ReleaseURL}}" style="color:#2563eb;font-size:14px;text-decoration:none;">View in Changelogue</a>{{end}}
</td></tr>

<!-- Footer -->
<tr><td style="padding:12px 24px;border-top:1px solid #e4e4e7;">
<p style="margin:0;color:#a1a1aa;font-size:12px;">{{.FooterText}}</p>
</td></tr>

</table>
</td></tr>
</table>
</body>
</html>
```

**Step 2: Commit**

```bash
git add internal/routing/email.html.tmpl
git commit -m "feat(email): add HTML email template"
```

---

### Task 2: Write failing tests for EmailSender

**Files:**
- Create: `internal/routing/email_test.go`

**Step 1: Write the test file**

Create `internal/routing/email_test.go`:

```go
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
```

**Step 2: Run tests to verify they fail**

Run: `go test -v -run TestEmailSender ./internal/routing/...`
Expected: FAIL — `EmailSender` not defined.

**Step 3: Commit**

```bash
git add internal/routing/email_test.go
git commit -m "test(email): add failing tests for EmailSender"
```

---

### Task 3: Implement EmailSender

**Files:**
- Create: `internal/routing/email.go`

**Step 1: Write the implementation**

Create `internal/routing/email.go`:

```go
package routing

import (
	"bytes"
	"context"
	"crypto/tls"
	_ "embed"
	"encoding/json"
	"fmt"
	"html/template"
	"mime/multipart"
	"net"
	"net/smtp"
	"net/textproto"
	"strings"

	"github.com/sentioxyz/changelogue/internal/models"
)

//go:embed email.html.tmpl
var emailTemplateRaw string

var emailTmpl = template.Must(template.New("email").Parse(emailTemplateRaw))

type emailConfig struct {
	SMTPHost    string   `json:"smtp_host"`
	SMTPPort    int      `json:"smtp_port"`
	Username    string   `json:"username"`
	Password    string   `json:"password"`
	FromAddress string   `json:"from_address"`
	ToAddresses []string `json:"to_addresses"`
}

// emailData is the template rendering context.
type emailData struct {
	Subject         string
	UrgencyBadge    string
	UrgencyColor    string
	UrgencyReason   string
	Summary         string
	Changelog       string
	DownloadCommand string
	SourceURL       string
	ReleaseURL      string
	ProviderLabel   string
	FooterText      string
}

// EmailSender sends notifications via SMTP email.
type EmailSender struct{}

func urgencyColor(level string) string {
	switch strings.ToUpper(strings.TrimSpace(level)) {
	case "CRITICAL":
		return "#111113"
	case "HIGH":
		return "#DC2626"
	case "MEDIUM":
		return "#D97706"
	case "LOW":
		return "#16A34A"
	default:
		return "#2563EB"
	}
}

func (s *EmailSender) Send(ctx context.Context, ch *models.NotificationChannel, msg Notification) error {
	var cfg emailConfig
	if err := json.Unmarshal(ch.Config, &cfg); err != nil {
		return fmt.Errorf("parse email config: %w", err)
	}

	data := s.buildEmailData(msg)

	// Render HTML body.
	var htmlBuf bytes.Buffer
	if err := emailTmpl.Execute(&htmlBuf, data); err != nil {
		return fmt.Errorf("render email template: %w", err)
	}

	// Build plain text body.
	plainText := s.buildPlainText(data)

	// Assemble multipart/alternative MIME message.
	var msgBuf bytes.Buffer
	msgBuf.WriteString(fmt.Sprintf("From: %s\r\n", cfg.FromAddress))
	msgBuf.WriteString(fmt.Sprintf("To: %s\r\n", strings.Join(cfg.ToAddresses, ", ")))
	msgBuf.WriteString(fmt.Sprintf("Subject: %s\r\n", data.Subject))
	msgBuf.WriteString("MIME-Version: 1.0\r\n")

	mw := multipart.NewWriter(&msgBuf)
	msgBuf.WriteString(fmt.Sprintf("Content-Type: multipart/alternative; boundary=%s\r\n\r\n", mw.Boundary()))

	// Plain text part.
	textHeader := make(textproto.MIMEHeader)
	textHeader.Set("Content-Type", "text/plain; charset=utf-8")
	pw, err := mw.CreatePart(textHeader)
	if err != nil {
		return fmt.Errorf("create text part: %w", err)
	}
	pw.Write([]byte(plainText))

	// HTML part.
	htmlHeader := make(textproto.MIMEHeader)
	htmlHeader.Set("Content-Type", "text/html; charset=utf-8")
	hw, err := mw.CreatePart(htmlHeader)
	if err != nil {
		return fmt.Errorf("create html part: %w", err)
	}
	hw.Write(htmlBuf.Bytes())

	mw.Close()

	// Send via SMTP.
	addr := fmt.Sprintf("%s:%d", cfg.SMTPHost, cfg.SMTPPort)

	var client *smtp.Client
	if cfg.SMTPPort == 465 {
		// Direct TLS (port 465).
		conn, err := tls.DialWithDialer(
			&net.Dialer{},
			"tcp", addr,
			&tls.Config{ServerName: cfg.SMTPHost},
		)
		if err != nil {
			return fmt.Errorf("tls dial: %w", err)
		}
		client, err = smtp.NewClient(conn, cfg.SMTPHost)
		if err != nil {
			conn.Close()
			return fmt.Errorf("smtp client over tls: %w", err)
		}
	} else {
		// STARTTLS (port 587 or other).
		client, err = smtp.Dial(addr)
		if err != nil {
			return fmt.Errorf("smtp dial: %w", err)
		}
		if err := client.Hello("localhost"); err != nil {
			client.Close()
			return fmt.Errorf("smtp hello: %w", err)
		}
		// Only attempt STARTTLS if the server supports it.
		if ok, _ := client.Extension("STARTTLS"); ok {
			if err := client.StartTLS(&tls.Config{ServerName: cfg.SMTPHost}); err != nil {
				client.Close()
				return fmt.Errorf("smtp starttls: %w", err)
			}
		}
	}
	defer client.Close()

	auth := smtp.PlainAuth("", cfg.Username, cfg.Password, cfg.SMTPHost)
	if err := client.Auth(auth); err != nil {
		return fmt.Errorf("smtp auth: %w", err)
	}

	if err := client.Mail(cfg.FromAddress); err != nil {
		return fmt.Errorf("smtp mail from: %w", err)
	}
	for _, to := range cfg.ToAddresses {
		if err := client.Rcpt(to); err != nil {
			return fmt.Errorf("smtp rcpt to %s: %w", to, err)
		}
	}

	wc, err := client.Data()
	if err != nil {
		return fmt.Errorf("smtp data: %w", err)
	}
	if _, err := wc.Write(msgBuf.Bytes()); err != nil {
		wc.Close()
		return fmt.Errorf("smtp write: %w", err)
	}
	if err := wc.Close(); err != nil {
		return fmt.Errorf("smtp data close: %w", err)
	}

	return client.Quit()
}

func (s *EmailSender) buildEmailData(msg Notification) emailData {
	data := emailData{
		SourceURL:     msg.SourceURL,
		ReleaseURL:    msg.ReleaseURL,
		ProviderLabel: ProviderLabel(msg.Provider),
		UrgencyColor:  "#2563EB", // default blue
	}

	// Footer
	if msg.Provider != "" && msg.Repository != "" {
		data.FooterText = fmt.Sprintf("%s · %s", ProviderLabel(msg.Provider), msg.Repository)
	}

	// Try semantic report first.
	var report models.SemanticReport
	if err := json.Unmarshal([]byte(msg.Body), &report); err == nil && report.Subject != "" {
		urgency := report.Urgency
		if urgency == "" {
			urgency = report.RiskLevel
		}
		urgencyReason := report.UrgencyReason
		if urgencyReason == "" {
			urgencyReason = report.RiskReason
		}

		data.Subject = fmt.Sprintf("%s %s", msg.Title, msg.Version)
		data.UrgencyColor = urgencyColor(urgency)
		if urgency != "" {
			data.UrgencyBadge = fmt.Sprintf("%s %s Urgency", urgencyEmoji(urgency), urgency)
		}
		upperUrgency := strings.ToUpper(strings.TrimSpace(urgency))
		if (upperUrgency == "CRITICAL" || upperUrgency == "HIGH") && urgencyReason != "" {
			data.UrgencyReason = urgencyReason
		}
		summary := report.ChangelogSummary
		if summary == "" {
			summary = report.Summary
		}
		data.Summary = summary
		if len(report.DownloadCommands) > 0 {
			data.DownloadCommand = report.DownloadCommands[0]
		}
		return data
	}

	// Fallback: source-level release.
	data.Subject = fmt.Sprintf("%s %s", msg.Title, msg.Version)
	if fields, ok := parseRawBody(msg.Body); ok && fields.Changelog != "" {
		data.Changelog = markdownToASCII(fields.Changelog)
	}
	return data
}

func (s *EmailSender) buildPlainText(data emailData) string {
	var b strings.Builder
	b.WriteString(data.Subject + "\n")
	b.WriteString(strings.Repeat("=", len(data.Subject)) + "\n\n")

	if data.UrgencyBadge != "" {
		b.WriteString(data.UrgencyBadge + "\n")
		if data.UrgencyReason != "" {
			b.WriteString(data.UrgencyReason + "\n")
		}
		b.WriteString("\n")
	}

	if data.Summary != "" {
		b.WriteString(data.Summary + "\n\n")
	}

	if data.Changelog != "" {
		b.WriteString(data.Changelog + "\n\n")
	}

	if data.DownloadCommand != "" {
		b.WriteString("  " + data.DownloadCommand + "\n\n")
	}

	if data.SourceURL != "" {
		b.WriteString("View on " + data.ProviderLabel + ": " + data.SourceURL + "\n")
	}
	if data.ReleaseURL != "" {
		b.WriteString("View in Changelogue: " + data.ReleaseURL + "\n")
	}

	if data.FooterText != "" {
		b.WriteString("\n---\n" + data.FooterText + "\n")
	}

	return b.String()
}
```

**Step 2: Run tests**

Run: `go test -v -run TestEmailSender ./internal/routing/...`
Expected: All 4 tests PASS.

**Step 3: Commit**

```bash
git add internal/routing/email.go
git commit -m "feat(email): implement EmailSender with SMTP and HTML template"
```

---

### Task 4: Register EmailSender in NewSenders

**Files:**
- Modify: `internal/routing/worker.go:42-48`

**Step 1: Add email to the senders map**

In `internal/routing/worker.go`, change `NewSenders()`:

```go
func NewSenders() map[string]Sender {
	return map[string]Sender{
		"webhook": &WebhookSender{Client: &http.Client{Timeout: 10 * time.Second}},
		"slack":   &SlackSender{Client: &http.Client{Timeout: 10 * time.Second}},
		"discord": &DiscordSender{Client: &http.Client{Timeout: 10 * time.Second}},
		"email":   &EmailSender{},
	}
}
```

**Step 2: Run full test suite**

Run: `go test ./internal/routing/...`
Expected: All tests PASS.

**Step 3: Run vet**

Run: `go vet ./...`
Expected: No issues.

**Step 4: Commit**

```bash
git add internal/routing/worker.go
git commit -m "feat(email): register EmailSender in notification senders"
```

---

### Task 5: Final verification

**Step 1: Build**

Run: `go build -o changelogue ./cmd/server`
Expected: Builds successfully.

**Step 2: Run all tests**

Run: `go test ./...`
Expected: All pass.

**Step 3: Clean up binary**

Run: `rm changelogue`
