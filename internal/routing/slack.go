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

// slackPayload is the Slack incoming webhook payload using Block Kit.
type slackPayload struct {
	Blocks []slackBlock `json:"blocks"`
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

// riskEmoji returns an emoji indicator for the given risk level.
func riskEmoji(level string) string {
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

// buildSemanticBlocks builds rich Slack Block Kit blocks from a SemanticReport.
func buildSemanticBlocks(title string, report *models.SemanticReport, msg Notification) []slackBlock {
	blocks := []slackBlock{
		{Type: "header", Text: &slackText{Type: "plain_text", Text: title}},
	}

	// Subject as the lead section
	if report.Subject != "" {
		blocks = append(blocks, slackBlock{
			Type: "section",
			Text: &slackText{Type: "mrkdwn", Text: fmt.Sprintf("*%s*", report.Subject)},
		})
	}

	// Risk + Urgency + Availability as compact fields
	blocks = append(blocks, slackBlock{
		Type: "section",
		Fields: []slackText{
			{Type: "mrkdwn", Text: fmt.Sprintf("*Risk Level:*\n%s %s", riskEmoji(report.RiskLevel), report.RiskLevel)},
			{Type: "mrkdwn", Text: fmt.Sprintf("*Urgency:*\n%s", report.Urgency)},
			{Type: "mrkdwn", Text: fmt.Sprintf("*Availability:*\n%s", report.Availability)},
		},
	})

	// Risk reason
	if report.RiskReason != "" {
		blocks = append(blocks,
			slackBlock{Type: "divider"},
			slackBlock{
				Type: "section",
				Text: &slackText{Type: "mrkdwn", Text: fmt.Sprintf("*Risk Assessment*\n%s", report.RiskReason)},
			},
		)
	}

	// Status checks
	if len(report.StatusChecks) > 0 {
		var items []string
		for _, check := range report.StatusChecks {
			items = append(items, fmt.Sprintf("• %s", check))
		}
		blocks = append(blocks,
			slackBlock{Type: "divider"},
			slackBlock{
				Type: "section",
				Text: &slackText{Type: "mrkdwn", Text: fmt.Sprintf("*Status Checks*\n%s", strings.Join(items, "\n"))},
			},
		)
	}

	// Changelog summary
	summary := report.ChangelogSummary
	if summary == "" {
		summary = report.Summary
	}
	if summary != "" {
		blocks = append(blocks,
			slackBlock{Type: "divider"},
			slackBlock{
				Type: "section",
				Text: &slackText{Type: "mrkdwn", Text: fmt.Sprintf("*Changelog*\n%s", summary)},
			},
		)
	}

	// Adoption
	if report.Adoption != "" {
		blocks = append(blocks, slackBlock{
			Type: "section",
			Text: &slackText{Type: "mrkdwn", Text: fmt.Sprintf("*Adoption*\n%s", report.Adoption)},
		})
	}

	// Download commands as code block
	if len(report.DownloadCommands) > 0 {
		blocks = append(blocks,
			slackBlock{Type: "divider"},
			slackBlock{
				Type: "section",
				Text: &slackText{Type: "mrkdwn", Text: fmt.Sprintf("*Download Commands*\n```%s```", strings.Join(report.DownloadCommands, "\n"))},
			},
		)
	}

	// Download links
	if len(report.DownloadLinks) > 0 {
		var links []string
		for _, link := range report.DownloadLinks {
			links = append(links, fmt.Sprintf("• <%s>", link))
		}
		blocks = append(blocks, slackBlock{
			Type: "section",
			Text: &slackText{Type: "mrkdwn", Text: fmt.Sprintf("*Download Links*\n%s", strings.Join(links, "\n"))},
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
	footerText := "📡 _Sent by Changelogue_"
	if len(footerParts) > 0 {
		footerText = strings.Join(footerParts, "  |  ") + "\n" + footerText
	}
	blocks = append(blocks,
		slackBlock{Type: "divider"},
		slackBlock{
			Type: "context",
			Elements: []slackText{
				{Type: "mrkdwn", Text: footerText},
			},
		},
	)

	return blocks
}

func (s *SlackSender) Send(ctx context.Context, ch *models.NotificationChannel, msg Notification) error {
	var cfg slackConfig
	if err := json.Unmarshal(ch.Config, &cfg); err != nil {
		return fmt.Errorf("parse slack config: %w", err)
	}

	// Try to parse body as a SemanticReport for rich formatting.
	var blocks []slackBlock
	var report models.SemanticReport
	if err := json.Unmarshal([]byte(msg.Body), &report); err == nil && report.Subject != "" {
		blocks = buildSemanticBlocks(msg.Title, &report, msg)
	} else {
		// Fallback: extract known fields from raw data for a readable message.
		blocks = []slackBlock{
			{Type: "header", Text: &slackText{Type: "plain_text", Text: msg.Title}},
		}
		if fields, ok := parseRawBody(msg.Body); ok {
			if fields.Changelog != "" {
				blocks = append(blocks, slackBlock{
					Type: "section",
					Text: &slackText{Type: "mrkdwn", Text: fields.Changelog},
				})
			}
		} else {
			blocks = append(blocks, slackBlock{
				Type: "section",
				Text: &slackText{Type: "mrkdwn", Text: msg.Body},
			})
		}

		// Add source info and links
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
	}

	payload := slackPayload{Blocks: blocks}

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
