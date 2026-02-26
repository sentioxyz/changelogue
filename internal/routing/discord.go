package routing

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"

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
	Description string              `json:"description"`
	Color       int                 `json:"color"`
	Fields      []discordEmbedField `json:"fields,omitempty"`
}

type discordEmbedField struct {
	Name   string `json:"name"`
	Value  string `json:"value"`
	Inline bool   `json:"inline"`
}

func (s *DiscordSender) Send(ctx context.Context, ch *models.NotificationChannel, msg Notification) error {
	var cfg discordConfig
	if err := json.Unmarshal(ch.Config, &cfg); err != nil {
		return fmt.Errorf("parse discord config: %w", err)
	}

	description := msg.Body
	if len(description) > discordEmbedDescriptionLimit {
		description = description[:discordEmbedDescriptionLimit-3] + "..."
	}

	payload := discordPayload{
		Embeds: []discordEmbed{
			{
				Title:       msg.Title,
				Description: description,
				Color:       0x5865F2, // Discord blurple
				Fields: []discordEmbedField{
					{Name: "Version", Value: msg.Version, Inline: true},
				},
			},
		},
	}

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
