package routing

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/riverqueue/river"
	"github.com/sentioxyz/changelogue/internal/models"
	"github.com/sentioxyz/changelogue/internal/queue"
)

// NotifyStore is the data access interface required by the notification worker.
type NotifyStore interface {
	GetRelease(ctx context.Context, id string) (*models.Release, error)
	GetSource(ctx context.Context, id string) (*models.Source, error)
	ListSourceSubscriptions(ctx context.Context, sourceID string) ([]models.Subscription, error)
	GetChannel(ctx context.Context, id string) (*models.NotificationChannel, error)
	GetProject(ctx context.Context, id string) (*models.Project, error)
	GetPreviousRelease(ctx context.Context, sourceID string, beforeVersion string) (*models.Release, error)
	EnqueueAgentRun(ctx context.Context, projectID, trigger, version string) error
	CreateReleaseTodo(ctx context.Context, releaseID string) (string, error)
	HasReleaseGate(ctx context.Context, projectID string) (bool, error)
}

// NotifyWorker is a River worker that processes NotifyJobArgs.
// It looks up the release, finds all source-level subscriptions, resolves
// the notification channel for each, and dispatches via the appropriate sender.
// After sending notifications, it checks the project's agent rules and
// auto-triggers an agent run if the version criteria are met.
type NotifyWorker struct {
	river.WorkerDefaults[queue.NotifyJobArgs]
	store     NotifyStore
	senders   map[string]Sender
	publicURL string // base URL for internal Changelogue links (e.g. "https://changelogue.example.com")
}

// NewSenders returns the default sender map for all notification channel types.
func NewSenders() map[string]Sender {
	return map[string]Sender{
		"webhook": &WebhookSender{Client: &http.Client{Timeout: 10 * time.Second}},
		"slack":   &SlackSender{Client: &http.Client{Timeout: 10 * time.Second}},
		"discord": &DiscordSender{Client: &http.Client{Timeout: 10 * time.Second}},
		"email":   &EmailSender{},
	}
}

// NewNotifyWorker creates a NotifyWorker with default senders for webhook, Slack, and Discord.
func NewNotifyWorker(store NotifyStore, publicURL string) *NotifyWorker {
	return &NotifyWorker{
		store:     store,
		senders:   NewSenders(),
		publicURL: strings.TrimRight(publicURL, "/"),
	}
}

// VersionPassesFilter returns true if the version string passes the source's
// include/exclude regex filters. When both are nil the version always passes.
func VersionPassesFilter(version string, include, exclude *string) bool {
	if include != nil && *include != "" {
		matched, err := regexp.MatchString(*include, version)
		if err != nil || !matched {
			return false
		}
	}
	if exclude != nil && *exclude != "" {
		matched, err := regexp.MatchString(*exclude, version)
		if err != nil || matched {
			return false
		}
	}
	return true
}

// Work processes a notify job: fetches the release, resolves subscriptions,
// sends notifications to all matching channels, and checks agent rules to
// auto-trigger agent runs when version criteria are met.
func (w *NotifyWorker) Work(ctx context.Context, job *river.Job[queue.NotifyJobArgs]) error {
	release, err := w.store.GetRelease(ctx, job.Args.ReleaseID)
	if err != nil {
		return fmt.Errorf("get release: %w", err)
	}

	// Check source version filters — skip entirely if filtered out.
	source, err := w.store.GetSource(ctx, job.Args.SourceID)
	if err != nil {
		return fmt.Errorf("get source: %w", err)
	}
	if !VersionPassesFilter(release.Version, source.VersionFilterInclude, source.VersionFilterExclude) {
		slog.Debug("release filtered by version filter", "version", release.Version, "source_id", job.Args.SourceID)
		return nil
	}

	// Check exclude_prereleases — skip if source excludes prereleases and this is one.
	if source.ExcludePrereleases {
		var rawData map[string]interface{}
		if err := json.Unmarshal(release.RawData, &rawData); err == nil {
			if prerelease, _ := rawData["prerelease"].(string); prerelease == "true" {
				slog.Debug("release filtered by exclude_prereleases", "version", release.Version, "source_id", job.Args.SourceID)
				return nil
			}
		}
	}

	// Create a TODO for this release (idempotent — safe for retries).
	todoID, todoErr := w.store.CreateReleaseTodo(ctx, release.ID)
	if todoErr != nil {
		slog.Error("create release todo failed", "release_id", release.ID, "err", todoErr)
		// Continue — notification delivery is primary responsibility.
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

		// Build title with project name if available.
		title := release.Version
		if release.ProjectName != "" {
			title = release.ProjectName
		}

		msg := Notification{
			Title:       title,
			Body:        string(release.RawData),
			Version:     release.Version,
			ProjectName: release.ProjectName,
			Provider:    release.Provider,
			Repository:  release.Repository,
			SourceURL:   ProviderURL(release.Provider, release.Repository, release.Version),
		}
		if w.publicURL != "" {
			msg.ReleaseURL = fmt.Sprintf("%s/releases/%s", w.publicURL, release.ID)
		}
		if todoID != "" {
			msg.TodoID = todoID
			msg.PublicURL = w.publicURL
		}

		if err := sender.Send(ctx, ch, msg); err != nil {
			slog.Error("send notification failed", "channel", ch.Name, "err", err)
		}
	}

	// Check agent rules and auto-trigger if criteria are met.
	w.checkAgentRules(ctx, release, source)

	return nil
}

// checkAgentRules evaluates the project's agent rules against the new release
// and the previous release for this source. If any rule matches, an agent run
// is enqueued. Errors are logged but do not fail the job — notification delivery
// is the primary responsibility.
func (w *NotifyWorker) checkAgentRules(ctx context.Context, release *models.Release, source *models.Source) {
	project, err := w.store.GetProject(ctx, source.ProjectID)
	if err != nil {
		slog.Error("get project for agent rules", "project_id", source.ProjectID, "err", err)
		return
	}

	// If the project has an active release gate, skip agent rule checking here.
	// The gate worker handles agent triggering.
	hasGate, err := w.store.HasReleaseGate(ctx, source.ProjectID)
	if err != nil {
		slog.Error("check release gate", "project_id", source.ProjectID, "err", err)
		return
	}
	if hasGate {
		slog.Debug("agent rules skipped — project has release gate", "project_id", source.ProjectID)
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
	prev, err := w.store.GetPreviousRelease(ctx, source.ID, release.Version)
	if err != nil {
		slog.Error("get previous release for agent rules", "source_id", source.ID, "err", err)
		return
	}
	if prev != nil {
		previousVersion = prev.Version
	}

	if CheckAgentRules(&rules, release.Version, previousVersion) {
		trigger := fmt.Sprintf("auto:version:%s", release.Version)
		if err := w.store.EnqueueAgentRun(ctx, source.ProjectID, trigger, release.Version); err != nil {
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
