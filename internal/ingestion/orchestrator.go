package ingestion

import (
	"context"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Orchestrator runs polling ingestion sources on a fixed interval.
type Orchestrator struct {
	service       *Service
	loader        *SourceLoader
	pool          *pgxpool.Pool
	staticSources []IIngestionSource // for testing only
	interval      time.Duration
}

// NewOrchestrator creates an orchestrator that dynamically loads enabled
// sources from the database on each poll cycle.
func NewOrchestrator(service *Service, loader *SourceLoader, pool *pgxpool.Pool, interval time.Duration) *Orchestrator {
	return &Orchestrator{service: service, loader: loader, pool: pool, interval: interval}
}

// NewOrchestratorWithSources creates an orchestrator with a static source list.
// Intended for testing; production code should use NewOrchestrator with a SourceLoader.
func NewOrchestratorWithSources(service *Service, sources []IIngestionSource, interval time.Duration) *Orchestrator {
	return &Orchestrator{service: service, staticSources: sources, interval: interval}
}

// Run polls all sources immediately, then on every tick. Blocks until ctx is cancelled.
func (o *Orchestrator) Run(ctx context.Context) {
	ticker := time.NewTicker(o.interval)
	defer ticker.Stop()

	o.pollAll(ctx)

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			o.pollAll(ctx)
		}
	}
}

func (o *Orchestrator) pollAll(ctx context.Context) {
	sources := o.staticSources
	if sources == nil {
		scheduled, err := o.loader.LoadEnabledSources(ctx)
		if err != nil {
			slog.Error("load sources failed", "err", err)
			return
		}
		if len(scheduled) == 0 {
			slog.Debug("no enabled sources to poll")
			return
		}
		now := time.Now()
		for _, ss := range scheduled {
			if ss.LastPolledAt != nil {
				interval := time.Duration(ss.PollIntervalSeconds) * time.Second
				if now.Before(ss.LastPolledAt.Add(interval)) {
					continue
				}
			}
			o.pollSource(ctx, ss.Source)
		}
		return
	}
	if len(sources) == 0 {
		slog.Debug("no enabled sources to poll")
		return
	}
	for _, src := range sources {
		o.pollSource(ctx, src)
	}
}

func (o *Orchestrator) pollSource(ctx context.Context, src IIngestionSource) {
	results, err := src.FetchNewReleases(ctx)
	if err != nil {
		slog.Error("poll failed", "source", src.Name(), "err", err)
		o.updateSourcePollStatus(ctx, src.SourceID(), err)
		return
	}
	o.updateSourcePollStatus(ctx, src.SourceID(), nil)
	if len(results) == 0 {
		return
	}
	if err := o.service.ProcessResults(ctx, src.SourceID(), src.Name(), results); err != nil {
		slog.Error("process failed", "source", src.Name(), "err", err)
	}
}

func (o *Orchestrator) updateSourcePollStatus(ctx context.Context, sourceID string, pollErr error) {
	if o.pool == nil {
		return
	}
	var lastError *string
	if pollErr != nil {
		s := pollErr.Error()
		lastError = &s
	}
	_, err := o.pool.Exec(ctx,
		`UPDATE sources SET last_polled_at = NOW(), last_error = $1 WHERE id = $2`,
		lastError, sourceID)
	if err != nil {
		slog.Error("update source poll status", "source", sourceID, "err", err)
	}
}
