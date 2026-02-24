package ingestion

import (
	"context"
	"log/slog"
	"time"
)

// Orchestrator runs polling ingestion sources on a fixed interval.
type Orchestrator struct {
	service  *Service
	sources  []IIngestionSource
	interval time.Duration
}

func NewOrchestrator(service *Service, sources []IIngestionSource, interval time.Duration) *Orchestrator {
	return &Orchestrator{service: service, sources: sources, interval: interval}
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
	for _, src := range o.sources {
		results, err := src.FetchNewReleases(ctx)
		if err != nil {
			slog.Error("poll failed", "source", src.Name(), "err", err)
			continue
		}
		if len(results) == 0 {
			continue
		}
		if err := o.service.ProcessResults(ctx, src.SourceID(), src.Name(), results); err != nil {
			slog.Error("process failed", "source", src.Name(), "err", err)
		}
	}
}
