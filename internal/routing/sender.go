package routing

import (
	"context"

	"github.com/sentioxyz/releaseguard/internal/models"
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
