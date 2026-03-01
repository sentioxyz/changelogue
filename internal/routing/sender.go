package routing

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"

	"github.com/sentioxyz/changelogue/internal/models"
)

// Notification is the payload sent through all notification channels.
type Notification struct {
	Title       string `json:"title"`
	Body        string `json:"body"`
	Version     string `json:"version"`
	ProjectName string `json:"project_name,omitempty"`
	Provider    string `json:"provider,omitempty"`
	Repository  string `json:"repository,omitempty"`
	ReleaseURL  string `json:"release_url,omitempty"`  // internal Changelogue link
	SourceURL   string `json:"source_url,omitempty"`   // upstream provider link
}

// Sender is the interface that all notification channel implementations must satisfy.
type Sender interface {
	Send(ctx context.Context, ch *models.NotificationChannel, msg Notification) error
}

// rawBodyFields contains the human-readable fields extracted from raw release JSON.
type rawBodyFields struct {
	Changelog  string
	ReleaseURL string
}

// parseRawBody extracts known fields from a raw JSON release body.
func parseRawBody(body string) (rawBodyFields, bool) {
	var raw map[string]interface{}
	if err := json.Unmarshal([]byte(body), &raw); err != nil {
		return rawBodyFields{}, false
	}

	fields := rawBodyFields{
		Changelog:  stringField(raw, "changelog"),
		ReleaseURL: stringField(raw, "release_url"),
	}

	if fields.Changelog == "" && fields.ReleaseURL == "" {
		return fields, false
	}
	return fields, true
}

func stringField(m map[string]interface{}, key string) string {
	v, _ := m[key].(string)
	return v
}

// ProviderURL builds the upstream release page URL for a given provider.
func ProviderURL(provider, repository, version string) string {
	switch strings.ToLower(provider) {
	case "github":
		return fmt.Sprintf("https://github.com/%s/releases/tag/%s", repository, version)
	case "dockerhub":
		return fmt.Sprintf("https://hub.docker.com/r/%s/tags?name=%s", repository, url.QueryEscape(version))
	case "ecr-public":
		return fmt.Sprintf("https://gallery.ecr.aws/%s", repository)
	case "gitlab":
		return fmt.Sprintf("https://gitlab.com/%s/-/releases/%s", repository, version)
	default:
		return ""
	}
}

// ProviderLabel returns a human-readable label for a provider.
func ProviderLabel(provider string) string {
	switch strings.ToLower(provider) {
	case "github":
		return "GitHub"
	case "dockerhub":
		return "Docker Hub"
	case "ecr-public":
		return "ECR Public"
	case "gitlab":
		return "GitLab"
	default:
		return provider
	}
}
