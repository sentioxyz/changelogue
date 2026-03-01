package routing

import (
	"context"
	"encoding/json"

	"github.com/sentioxyz/changelogue/internal/models"
)

// Notification is the payload sent through all notification channels.
type Notification struct {
	Title   string `json:"title"`
	Body    string `json:"body"`
	Version string `json:"version"`
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
