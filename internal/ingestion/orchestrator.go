package ingestion

import (
	"context"
	"log/slog"
	"time"
)

// Orchestrator runs polling ingestion sources on a fixed interval.
type Orchestrator struct {
	service       *Service
	loader        *SourceLoader
	staticSources []IIngestionSource // for testing only
	interval      time.Duration
}

// NewOrchestrator creates an orchestrator that dynamically loads enabled
// sources from the database on each poll cycle.
func NewOrchestrator(service *Service, loader *SourceLoader, interval time.Duration) *Orchestrator {
	return &Orchestrator{service: service, loader: loader, interval: interval}
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
		var err error
		sources, err = o.loader.LoadEnabledSources(ctx)
		if err != nil {
			slog.Error("load sources failed", "err", err)
			return
		}
	}
	if len(sources) == 0 {
		slog.Debug("no enabled sources to poll")
		return
	}
	for _, src := range sources {
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
