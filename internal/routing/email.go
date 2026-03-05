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
