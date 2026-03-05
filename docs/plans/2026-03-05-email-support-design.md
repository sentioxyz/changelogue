# Email Notification Channel Design

## Summary

Add email as a fourth notification channel (alongside Slack, Discord, Webhook) using Go's `net/smtp` with STARTTLS and an embedded HTML template.

## Architecture

Follows the existing sender plugin pattern exactly:

- **New file:** `internal/routing/email.go` — `EmailSender` implementing `Sender` interface
- **New file:** `internal/routing/email.html.tmpl` — embedded HTML template
- **New file:** `internal/routing/email_test.go` — unit tests
- **One-line change:** `worker.go:NewSenders()` — register `"email"` sender

No database migration needed — `notification_channels.type` is a free-form string and `config` is JSONB.

## Config Schema

```go
type emailConfig struct {
    SMTPHost    string   `json:"smtp_host"`
    SMTPPort    int      `json:"smtp_port"`
    Username    string   `json:"username"`
    Password    string   `json:"password"`
    FromAddress string   `json:"from_address"`
    ToAddresses []string `json:"to_addresses"`
}
```

## Email Content

Two message types (matching Slack/Discord pattern):

1. **Semantic report** — parsed from `SemanticReport` JSON. HTML with urgency badge, changelog summary, download command, links.
2. **Source release fallback** — raw release data. Version announcement with optional changelog and links.

Both produce multipart MIME (HTML + plain text). Plain text uses existing `markdownToASCII()`.

## SMTP Delivery

- Go `net/smtp` with STARTTLS (port 587) or direct TLS (port 465)
- `PLAIN` auth with username/password
- 10-second timeout via context
- Errors returned to River for retry

## HTML Template

Embedded via `//go:embed email.html.tmpl`. Inline CSS for email client compatibility. Responsive, clean layout with urgency color coding.

## What This Does NOT Include

- No per-user email preferences (uses channel-level `to_addresses`)
- No unsubscribe links (channels are managed via API)
- No attachment support
- No unauthenticated SMTP relay mode
