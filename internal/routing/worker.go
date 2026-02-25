package routing

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/riverqueue/river"
	"github.com/sentioxyz/releaseguard/internal/models"
	"github.com/sentioxyz/releaseguard/internal/queue"
)

// NotifyStore is the data access interface required by the notification worker.
type NotifyStore interface {
	GetRelease(ctx context.Context, id string) (*models.Release, error)
	GetSource(ctx context.Context, id string) (*models.Source, error)
	ListSourceSubscriptions(ctx context.Context, sourceID string) ([]models.Subscription, error)
	GetChannel(ctx context.Context, id string) (*models.NotificationChannel, error)
}

// NotifyWorker is a River worker that processes NotifyJobArgs.
// It looks up the release, finds all source-level subscriptions, resolves
// the notification channel for each, and dispatches via the appropriate sender.
type NotifyWorker struct {
	river.WorkerDefaults[queue.NotifyJobArgs]
	store   NotifyStore
	senders map[string]Sender
}

// NewNotifyWorker creates a NotifyWorker with default senders for webhook, Slack, and Discord.
func NewNotifyWorker(store NotifyStore) *NotifyWorker {
	return &NotifyWorker{
		store: store,
		senders: map[string]Sender{
			"webhook": &WebhookSender{Client: &http.Client{Timeout: 10 * time.Second}},
			"slack":   &SlackSender{Client: &http.Client{Timeout: 10 * time.Second}},
			"discord": &DiscordSender{Client: &http.Client{Timeout: 10 * time.Second}},
		},
	}
}

// Work processes a notify job: fetches the release, resolves subscriptions,
// and sends notifications to all matching channels.
func (w *NotifyWorker) Work(ctx context.Context, job *river.Job[queue.NotifyJobArgs]) error {
	release, err := w.store.GetRelease(ctx, job.Args.ReleaseID)
	if err != nil {
		return fmt.Errorf("get release: %w", err)
	}

	subs, err := w.store.ListSourceSubscriptions(ctx, job.Args.SourceID)
	if err != nil {
		return fmt.Errorf("list subscriptions: %w", err)
	}

	for _, sub := range subs {
		ch, err := w.store.GetChannel(ctx, sub.ChannelID)
		if err != nil {
			slog.Error("get channel failed", "channel_id", sub.ChannelID, "err", err)
			continue
		}

		sender, ok := w.senders[ch.Type]
		if !ok {
			slog.Warn("unknown channel type", "type", ch.Type)
			continue
		}

		msg := Notification{
			Title:   fmt.Sprintf("New release: %s", release.Version),
			Body:    string(release.RawData),
			Version: release.Version,
		}

		if err := sender.Send(ctx, ch, msg); err != nil {
			slog.Error("send notification failed", "channel", ch.Name, "err", err)
		}
	}

	return nil
}
