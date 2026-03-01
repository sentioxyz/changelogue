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

// discordRiskColor returns a Discord embed color based on risk level.
func discordRiskColor(level string) int {
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

// buildSemanticEmbed builds a rich Discord embed from a SemanticReport.
func buildSemanticEmbed(title string, version string, report *models.SemanticReport, msg Notification) discordEmbed {
	var descParts []string

	if report.Subject != "" {
		descParts = append(descParts, fmt.Sprintf("**%s**", report.Subject))
	}

	if report.RiskReason != "" {
		descParts = append(descParts, fmt.Sprintf(">>> %s", report.RiskReason))
	}

	if len(report.StatusChecks) > 0 {
		var checks []string
		for _, check := range report.StatusChecks {
			checks = append(checks, fmt.Sprintf("✅ %s", check))
		}
		descParts = append(descParts, strings.Join(checks, "\n"))
	}

	summary := report.ChangelogSummary
	if summary == "" {
		summary = report.Summary
	}
	if summary != "" {
		descParts = append(descParts, fmt.Sprintf("**Changelog**\n%s", summary))
	}

	if len(report.DownloadCommands) > 0 {
		descParts = append(descParts, fmt.Sprintf("**Download Commands**\n```\n%s\n```", strings.Join(report.DownloadCommands, "\n")))
	}

	if len(report.DownloadLinks) > 0 {
		var links []string
		for _, link := range report.DownloadLinks {
			links = append(links, fmt.Sprintf("• %s", link))
		}
		descParts = append(descParts, fmt.Sprintf("**Download Links**\n%s", strings.Join(links, "\n")))
	}

	// Add links section
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

	fields := []discordEmbedField{
		{Name: "Risk Level", Value: fmt.Sprintf("%s %s", riskEmoji(report.RiskLevel), report.RiskLevel), Inline: true},
		{Name: "Urgency", Value: report.Urgency, Inline: true},
	}
	if report.Availability != "" {
		fields = append(fields, discordEmbedField{Name: "Availability", Value: report.Availability, Inline: true})
	}
	if report.Adoption != "" {
		fields = append(fields, discordEmbedField{Name: "Adoption", Value: report.Adoption, Inline: false})
	}

	// Footer with source info
	var footerText string
	if msg.Provider != "" && msg.Repository != "" {
		footerText = fmt.Sprintf("%s · %s", ProviderLabel(msg.Provider), msg.Repository)
	}

	embed := discordEmbed{
		Title:       title,
		Description: description,
		Color:       discordRiskColor(report.RiskLevel),
		Fields:      fields,
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
				descParts = append(descParts, fields.Changelog)
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
