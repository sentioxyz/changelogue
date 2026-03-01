package routing

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/sentioxyz/changelogue/internal/models"
)

type webhookConfig struct {
	URL string `json:"url"`
}

// WebhookSender sends notifications as JSON POST requests to a configured URL.
type WebhookSender struct {
	Client *http.Client
}

// webhookSemanticPayload is the structured webhook payload for semantic reports.
type webhookSemanticPayload struct {
	Title       string                `json:"title"`
	Version     string                `json:"version"`
	ProjectName string                `json:"project_name,omitempty"`
	Provider    string                `json:"provider,omitempty"`
	Repository  string                `json:"repository,omitempty"`
	ReleaseURL  string                `json:"release_url,omitempty"`
	SourceURL   string                `json:"source_url,omitempty"`
	Report      *models.SemanticReport `json:"report"`
}

func (s *WebhookSender) Send(ctx context.Context, ch *models.NotificationChannel, msg Notification) error {
	var cfg webhookConfig
	if err := json.Unmarshal(ch.Config, &cfg); err != nil {
		return fmt.Errorf("parse webhook config: %w", err)
	}

	// Try to parse body as SemanticReport for structured output.
	var payload any
	var report models.SemanticReport
	if err := json.Unmarshal([]byte(msg.Body), &report); err == nil && report.Subject != "" {
		payload = webhookSemanticPayload{
			Title:       msg.Title,
			Version:     msg.Version,
			ProjectName: msg.ProjectName,
			Provider:    msg.Provider,
			Repository:  msg.Repository,
			ReleaseURL:  msg.ReleaseURL,
			SourceURL:   msg.SourceURL,
			Report:      &report,
		}
	} else {
		payload = msg
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal notification: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, cfg.URL, bytes.NewReader(body))
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
		return fmt.Errorf("send webhook: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return fmt.Errorf("webhook returned status %d", resp.StatusCode)
	}
	return nil
}
