package ingestion

import (
	"context"
	"fmt"
	"log/slog"
)

// Service orchestrates ingestion sources and persists results.
type Service struct {
	store ReleaseStore
}

func NewService(store ReleaseStore) *Service {
	return &Service{store: store}
}

// ProcessResults persists raw ingestion results via the store.
// Unchanged releases are silently skipped; updated releases are re-processed.
func (s *Service) ProcessResults(ctx context.Context, sourceID string, sourceName string, results []IngestionResult) error {
	for _, r := range results {
		res, err := s.store.IngestRelease(ctx, sourceID, &r)
		if err != nil {
			return fmt.Errorf("ingest %s: %w", r.RawVersion, err)
		}
		switch res {
		case IngestNew:
			slog.Info("ingested release", "source", sourceName, "version", r.RawVersion)
		case IngestUpdated:
			slog.Info("updated release", "source", sourceName, "version", r.RawVersion)
		default:
			slog.Debug("release unchanged, skipping", "version", r.RawVersion)
		}
	}
	return nil
}
