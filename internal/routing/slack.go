package routing

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"

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
	Type string     `json:"type"`
	Text *slackText `json:"text,omitempty"`
}

type slackText struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

func (s *SlackSender) Send(ctx context.Context, ch *models.NotificationChannel, msg Notification) error {
	var cfg slackConfig
	if err := json.Unmarshal(ch.Config, &cfg); err != nil {
		return fmt.Errorf("parse slack config: %w", err)
	}

	payload := slackPayload{
		Blocks: []slackBlock{
			{
				Type: "header",
				Text: &slackText{Type: "plain_text", Text: msg.Title},
			},
			{
				Type: "section",
				Text: &slackText{Type: "mrkdwn", Text: msg.Body},
			},
		},
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
