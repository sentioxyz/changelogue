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

// discordEmbedDescriptionLimit is Discord's maximum embed description length.
const discordEmbedDescriptionLimit = 4096

type discordConfig struct {
	WebhookURL string `json:"webhook_url"`
}

// DiscordSender sends notifications as Discord webhook messages with embeds.
type DiscordSender struct {
	Client *http.Client
}

// discordPayload is the Discord webhook payload.
type discordPayload struct {
	Embeds []discordEmbed `json:"embeds"`
}

type discordEmbed struct {
	Title       string              `json:"title"`
	URL         string              `json:"url,omitempty"`
	Description string              `json:"description"`
	Color       int                 `json:"color"`
	Fields      []discordEmbedField `json:"fields,omitempty"`
	Footer      *discordEmbedFooter `json:"footer,omitempty"`
}

type discordEmbedField struct {
	Name   string `json:"name"`
	Value  string `json:"value"`
	Inline bool   `json:"inline"`
}

type discordEmbedFooter struct {
	Text string `json:"text"`
}

// discordUrgencyColor returns a Discord embed color based on urgency level.
func discordUrgencyColor(level string) int {
	switch strings.ToUpper(strings.TrimSpace(level)) {
	case "CRITICAL":
		return 0x111113 // near-black
	case "HIGH":
		return 0xDC2626 // red
	case "MEDIUM":
		return 0xD97706 // amber
	case "LOW":
		return 0x16A34A // green
	default:
		return 0x5865F2 // Discord blurple
	}
}

// buildSemanticEmbed builds a compact Discord embed from a SemanticReport.
func buildSemanticEmbed(title string, version string, report *models.SemanticReport, msg Notification) discordEmbed {
	// Resolve urgency from new field or backward-compat risk_level
	urgency := report.Urgency
	if urgency == "" {
		urgency = report.RiskLevel
	}
	urgencyReason := report.UrgencyReason
	if urgencyReason == "" {
		urgencyReason = report.RiskReason
	}

	var descParts []string

	// Urgency badge
	if urgency != "" {
		descParts = append(descParts, fmt.Sprintf("%s **%s Urgency**", urgencyEmoji(urgency), urgency))
	}

	// Urgency reason (only for Critical/High)
	upperUrgency := strings.ToUpper(strings.TrimSpace(urgency))
	if (upperUrgency == "CRITICAL" || upperUrgency == "HIGH") && urgencyReason != "" {
		descParts = append(descParts, urgencyReason)
	}

	// Changelog summary
	summary := report.ChangelogSummary
	if summary == "" {
		summary = report.Summary
	}
	if summary != "" {
		descParts = append(descParts, summary)
	}

	// First download command
	if len(report.DownloadCommands) > 0 {
		descParts = append(descParts, fmt.Sprintf("`%s`", report.DownloadCommands[0]))
	}

	// Links
	var linkParts []string
	if msg.SourceURL != "" {
		linkParts = append(linkParts, fmt.Sprintf("[View on %s](%s)", ProviderLabel(msg.Provider), msg.SourceURL))
	}
	if msg.ReleaseURL != "" {
		linkParts = append(linkParts, fmt.Sprintf("[View in Changelogue](%s)", msg.ReleaseURL))
	}
	if len(linkParts) > 0 {
		descParts = append(descParts, strings.Join(linkParts, "  •  "))
	}

	description := strings.Join(descParts, "\n\n")
	if len(description) > discordEmbedDescriptionLimit {
		description = description[:discordEmbedDescriptionLimit-3] + "..."
	}

	// Footer with source info
	var footerText string
	if msg.Provider != "" && msg.Repository != "" {
		footerText = fmt.Sprintf("%s · %s", ProviderLabel(msg.Provider), msg.Repository)
	}

	embed := discordEmbed{
		Title:       title,
		Description: description,
		Color:       discordUrgencyColor(urgency),
	}
	if footerText != "" {
		embed.Footer = &discordEmbedFooter{Text: footerText}
	}
	return embed
}

func (s *DiscordSender) Send(ctx context.Context, ch *models.NotificationChannel, msg Notification) error {
	var cfg discordConfig
	if err := json.Unmarshal(ch.Config, &cfg); err != nil {
		return fmt.Errorf("parse discord config: %w", err)
	}

	var embed discordEmbed
	var report models.SemanticReport
	if err := json.Unmarshal([]byte(msg.Body), &report); err == nil && report.Subject != "" {
		embed = buildSemanticEmbed(msg.Title, msg.Version, &report, msg)
	} else {
		// Fallback: extract known fields from raw data for a readable message.
		var descParts []string
		if fields, ok := parseRawBody(msg.Body); ok {
			if fields.Changelog != "" {
				// Wrap changelog in a code block — Discord auto-collapses
				// long code blocks with a built-in "Show more" button.
				descParts = append(descParts, fmt.Sprintf("```markdown\n%s```", fields.Changelog))
			}
		} else {
			descParts = append(descParts, msg.Body)
		}

		// Add links
		var linkParts []string
		if msg.SourceURL != "" {
			linkParts = append(linkParts, fmt.Sprintf("[View on %s](%s)", ProviderLabel(msg.Provider), msg.SourceURL))
		}
		if msg.ReleaseURL != "" {
			linkParts = append(linkParts, fmt.Sprintf("[View in Changelogue](%s)", msg.ReleaseURL))
		}
		if len(linkParts) > 0 {
			descParts = append(descParts, strings.Join(linkParts, "  •  "))
		}

		description := strings.Join(descParts, "\n\n")
		if len(description) > discordEmbedDescriptionLimit {
			description = description[:discordEmbedDescriptionLimit-3] + "..."
		}

		embed = discordEmbed{
			Title:       msg.Title,
			Description: description,
			Color:       0x5865F2,
			Fields: []discordEmbedField{
				{Name: "Version", Value: msg.Version, Inline: true},
			},
		}
		if msg.Provider != "" && msg.Repository != "" {
			embed.Footer = &discordEmbedFooter{Text: fmt.Sprintf("%s · %s", ProviderLabel(msg.Provider), msg.Repository)}
		}
	}

	payload := discordPayload{Embeds: []discordEmbed{embed}}

	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal discord payload: %w", err)
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
		return fmt.Errorf("send discord notification: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return fmt.Errorf("discord returned status %d", resp.StatusCode)
	}
	return nil
}
