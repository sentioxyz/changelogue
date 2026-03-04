package routing

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/sentioxyz/changelogue/internal/models"
)

type slackConfig struct {
	WebhookURL string `json:"webhook_url"`
}

// SlackSender sends notifications as Slack Block Kit messages via incoming webhook.
type SlackSender struct {
	Client *http.Client
}

// slackPayload is the Slack incoming webhook payload using Block Kit and/or attachments.
type slackPayload struct {
	Blocks      []slackBlock      `json:"blocks,omitempty"`
	Attachments []slackAttachment `json:"attachments,omitempty"`
}

type slackBlock struct {
	Type     string      `json:"type"`
	Text     *slackText  `json:"text,omitempty"`
	Fields   []slackText `json:"fields,omitempty"`
	Elements []slackText `json:"elements,omitempty"`
}

type slackText struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

// slackAttachment uses Slack's legacy attachment format which auto-collapses
// long text with a "Show more" button.
type slackAttachment struct {
	Color     string   `json:"color,omitempty"`
	Title     string   `json:"title,omitempty"`
	TitleLink string   `json:"title_link,omitempty"`
	Text      string   `json:"text,omitempty"`
	MrkdwnIn  []string `json:"mrkdwn_in,omitempty"`
}

// urgencyEmoji returns an emoji indicator for the given urgency level.
func urgencyEmoji(level string) string {
	switch strings.ToUpper(strings.TrimSpace(level)) {
	case "LOW":
		return "🟢"
	case "MEDIUM":
		return "🟡"
	case "HIGH":
		return "🔴"
	case "CRITICAL":
		return "⚫"
	default:
		return "⚪"
	}
}

// buildSemanticBlocks builds compact Slack Block Kit blocks from a SemanticReport.
// For Critical/High urgency, includes the urgency reason. For Low/Medium, omits it.
func buildSemanticBlocks(title string, report *models.SemanticReport, msg Notification) []slackBlock {
	// Resolve urgency from new field or backward-compat risk_level
	urgency := report.Urgency
	if urgency == "" {
		urgency = report.RiskLevel
	}
	urgencyReason := report.UrgencyReason
	if urgencyReason == "" {
		urgencyReason = report.RiskReason
	}

	// Header: "ProjectName vX.Y.Z — 🟢 Low Urgency"
	headerText := title
	if urgency != "" {
		headerText = fmt.Sprintf("%s — %s %s Urgency", title, urgencyEmoji(urgency), urgency)
	}
	// Slack header max is 150 chars
	if len(headerText) > 150 {
		headerText = headerText[:147] + "..."
	}

	blocks := []slackBlock{
		{Type: "header", Text: &slackText{Type: "plain_text", Text: headerText}},
	}

	// Urgency reason (only for Critical/High)
	upperUrgency := strings.ToUpper(strings.TrimSpace(urgency))
	if (upperUrgency == "CRITICAL" || upperUrgency == "HIGH") && urgencyReason != "" {
		blocks = append(blocks, slackBlock{
			Type: "section",
			Text: &slackText{Type: "mrkdwn", Text: fmt.Sprintf("⚠️ %s", urgencyReason)},
		})
	}

	// Changelog summary
	summary := report.ChangelogSummary
	if summary == "" {
		summary = report.Summary
	}
	if summary != "" {
		blocks = append(blocks, slackBlock{
			Type: "section",
			Text: &slackText{Type: "mrkdwn", Text: summary},
		})
	}

	// First download command only
	if len(report.DownloadCommands) > 0 {
		blocks = append(blocks, slackBlock{
			Type: "section",
			Text: &slackText{Type: "mrkdwn", Text: fmt.Sprintf("`%s`", report.DownloadCommands[0])},
		})
	}

	// Footer context with source info and links
	var footerParts []string
	if msg.Provider != "" && msg.Repository != "" {
		footerParts = append(footerParts, fmt.Sprintf("%s · %s", ProviderLabel(msg.Provider), msg.Repository))
	}
	if msg.SourceURL != "" {
		footerParts = append(footerParts, fmt.Sprintf("<%s|View on %s>", msg.SourceURL, ProviderLabel(msg.Provider)))
	}
	if msg.ReleaseURL != "" {
		footerParts = append(footerParts, fmt.Sprintf("<%s|View in Changelogue>", msg.ReleaseURL))
	}
	if len(footerParts) > 0 {
		blocks = append(blocks, slackBlock{
			Type: "context",
			Elements: []slackText{
				{Type: "mrkdwn", Text: strings.Join(footerParts, "  |  ")},
			},
		})
	}

	return blocks
}

func (s *SlackSender) Send(ctx context.Context, ch *models.NotificationChannel, msg Notification) error {
	var cfg slackConfig
	if err := json.Unmarshal(ch.Config, &cfg); err != nil {
		return fmt.Errorf("parse slack config: %w", err)
	}

	// Try to parse body as a SemanticReport for rich formatting.
	var payload slackPayload
	var report models.SemanticReport
	if err := json.Unmarshal([]byte(msg.Body), &report); err == nil && report.Subject != "" {
		payload.Blocks = buildSemanticBlocks(msg.Title, &report, msg)
	} else {
		// Fallback: source-level release notification.
		// Use attachments for changelog so Slack auto-collapses long text
		// with a "Show more" button.
		var changelogText string
		if fields, ok := parseRawBody(msg.Body); ok {
			changelogText = fields.Changelog
		}

		if changelogText != "" {
			// Build title link label
			titleLabel := msg.Title
			if msg.Repository != "" && msg.Provider != "" {
				titleLabel = fmt.Sprintf("%s on %s", msg.Repository, ProviderLabel(msg.Provider))
			}

			// Build attachment text: title + version badge, then changelog
			// converted to ASCII and wrapped in a code block. Slack
			// auto-collapses long code blocks in attachments with a
			// "Show more" button.
			asciiChangelog := markdownToASCII(changelogText)
			attachText := fmt.Sprintf("*%s* `%s`\n```%s```", msg.Title, msg.Version, asciiChangelog)

			// Append links after the code block
			var linkParts []string
			if msg.SourceURL != "" {
				linkParts = append(linkParts, fmt.Sprintf("<%s|View on %s>", msg.SourceURL, ProviderLabel(msg.Provider)))
			}
			if msg.ReleaseURL != "" {
				linkParts = append(linkParts, fmt.Sprintf("<%s|View in Changelogue>", msg.ReleaseURL))
			}
			if len(linkParts) > 0 {
				attachText += "\n" + strings.Join(linkParts, "  |  ")
			}

			attachment := slackAttachment{
				Color:    "#D3D3D3",
				Title:    titleLabel,
				Text:     attachText,
				MrkdwnIn: []string{"text"},
			}
			if msg.SourceURL != "" {
				attachment.TitleLink = msg.SourceURL
			}
			payload.Attachments = []slackAttachment{attachment}
		} else {
			// No changelog — simple blocks fallback
			blocks := []slackBlock{
				{Type: "header", Text: &slackText{Type: "plain_text", Text: msg.Title}},
				{Type: "section", Text: &slackText{Type: "mrkdwn", Text: msg.Body}},
			}

			var linkParts []string
			if msg.Provider != "" && msg.Repository != "" {
				linkParts = append(linkParts, fmt.Sprintf("%s · %s", ProviderLabel(msg.Provider), msg.Repository))
			}
			if msg.SourceURL != "" {
				linkParts = append(linkParts, fmt.Sprintf("<%s|View on %s>", msg.SourceURL, ProviderLabel(msg.Provider)))
			}
			if msg.ReleaseURL != "" {
				linkParts = append(linkParts, fmt.Sprintf("<%s|View in Changelogue>", msg.ReleaseURL))
			}
			if len(linkParts) > 0 {
				blocks = append(blocks, slackBlock{
					Type: "context",
					Elements: []slackText{
						{Type: "mrkdwn", Text: strings.Join(linkParts, "  |  ")},
					},
				})
			}
			payload.Blocks = blocks
		}
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal slack payload: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, cfg.WebhookURL, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	client := s.Client
	if client == nil {
		client = http.DefaultClient
	}

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("send slack notification: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return fmt.Errorf("slack returned status %d", resp.StatusCode)
	}
	return nil
}
