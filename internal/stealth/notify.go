package stealth

import (
	"context"
	"log/slog"

	"github.com/sentioxyz/changelogue/internal/routing"
)

// NotifyRelease dispatches notifications for a newly ingested release.
// It reuses the routing.Sender implementations but runs synchronously
// instead of through a River job queue.
func (s *Store) NotifyRelease(ctx context.Context, releaseID, sourceID string, senders map[string]routing.Sender) {
	release, err := s.GetRelease(ctx, releaseID)
	if err != nil {
		slog.Error("notify: get release", "release_id", releaseID, "err", err)
		return
	}

	source, err := s.GetSource(ctx, sourceID)
	if err != nil {
		slog.Error("notify: get source", "source_id", sourceID, "err", err)
		return
	}

	// Check version filters
	if !routing.VersionPassesFilter(release.Version, source.VersionFilterInclude, source.VersionFilterExclude) {
		slog.Debug("release filtered by version filter", "version", release.Version)
		return
	}

	subs, err := s.ListSourceSubscriptions(ctx, sourceID)
	if err != nil {
		slog.Error("notify: list subscriptions", "source_id", sourceID, "err", err)
		return
	}

	for _, sub := range subs {
		ch, err := s.GetChannel(ctx, sub.ChannelID)
		if err != nil {
			slog.Error("notify: get channel", "channel_id", sub.ChannelID, "err", err)
			continue
		}

		sender, ok := senders[ch.Type]
		if !ok {
			slog.Warn("notify: unknown channel type", "type", ch.Type)
			continue
		}

		msg := routing.Notification{
			Title:      release.Version,
			Body:       string(release.RawData),
			Version:    release.Version,
			Provider:   release.Provider,
			Repository: release.Repository,
			SourceURL:  routing.ProviderURL(release.Provider, release.Repository, release.Version),
		}

		// For shell senders, pass subscription config for per-sub commands
		if ch.Type == "shell" {
			if ss, ok := sender.(*routing.ShellSender); ok {
				if err := ss.SendWithConfig(ctx, ch, msg, sub.Config); err != nil {
					slog.Error("notify: shell send failed", "channel", ch.Name, "err", err)
				}
				continue
			}
		}

		if err := sender.Send(ctx, ch, msg); err != nil {
			slog.Error("notify: send failed", "channel", ch.Name, "err", err)
		}
	}
}
