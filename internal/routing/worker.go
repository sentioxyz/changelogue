package routing

import (
	"context"
	"encoding/json"
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
	GetProject(ctx context.Context, id string) (*models.Project, error)
	GetPreviousRelease(ctx context.Context, sourceID string, beforeVersion string) (*models.Release, error)
	EnqueueAgentRun(ctx context.Context, projectID, trigger string) error
}

// NotifyWorker is a River worker that processes NotifyJobArgs.
// It looks up the release, finds all source-level subscriptions, resolves
// the notification channel for each, and dispatches via the appropriate sender.
// After sending notifications, it checks the project's agent rules and
// auto-triggers an agent run if the version criteria are met.
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
// sends notifications to all matching channels, and checks agent rules to
// auto-trigger agent runs when version criteria are met.
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

	// Check agent rules and auto-trigger if criteria are met.
	w.checkAgentRules(ctx, release, job.Args.SourceID)

	return nil
}

// checkAgentRules evaluates the project's agent rules against the new release
// and the previous release for this source. If any rule matches, an agent run
// is enqueued. Errors are logged but do not fail the job — notification delivery
// is the primary responsibility.
func (w *NotifyWorker) checkAgentRules(ctx context.Context, release *models.Release, sourceID string) {
	source, err := w.store.GetSource(ctx, sourceID)
	if err != nil {
		slog.Error("get source for agent rules", "source_id", sourceID, "err", err)
		return
	}

	project, err := w.store.GetProject(ctx, source.ProjectID)
	if err != nil {
		slog.Error("get project for agent rules", "project_id", source.ProjectID, "err", err)
		return
	}

	// Parse the project's agent_rules JSON.
	var rules models.AgentRules
	if len(project.AgentRules) > 0 {
		if err := json.Unmarshal(project.AgentRules, &rules); err != nil {
			slog.Error("unmarshal agent rules", "project_id", project.ID, "err", err)
			return
		}
	}

	// Determine the previous version for comparison.
	var previousVersion string
	prev, err := w.store.GetPreviousRelease(ctx, sourceID, release.Version)
	if err != nil {
		slog.Error("get previous release for agent rules", "source_id", sourceID, "err", err)
		return
	}
	if prev != nil {
		previousVersion = prev.Version
	}

	if CheckAgentRules(&rules, release.Version, previousVersion) {
		trigger := fmt.Sprintf("auto:version:%s", release.Version)
		if err := w.store.EnqueueAgentRun(ctx, source.ProjectID, trigger); err != nil {
			slog.Error("enqueue agent run", "project_id", source.ProjectID, "err", err)
		} else {
			slog.Info("agent run triggered by version rules",
				"project_id", source.ProjectID,
				"version", release.Version,
				"previous_version", previousVersion,
			)
		}
	}
}
