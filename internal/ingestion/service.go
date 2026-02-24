package ingestion

import (
	"context"
	"log/slog"

	"github.com/google/uuid"
	"github.com/sentioxyz/releaseguard/internal/models"
)

// Service orchestrates ingestion sources and persists normalized results.
type Service struct {
	store ReleaseStore
}

func NewService(store ReleaseStore) *Service {
	return &Service{store: store}
}

// ProcessResults normalizes raw ingestion results into ReleaseEvents and persists them.
// Duplicate releases (unique constraint violations) are logged and skipped, not fatal.
func (s *Service) ProcessResults(ctx context.Context, sourceID int, sourceName string, results []IngestionResult) error {
	for _, r := range results {
		event := &models.ReleaseEvent{
			ID:         uuid.New().String(),
			Source:     sourceName,
			Repository: r.Repository,
			RawVersion: r.RawVersion,
			Changelog:  r.Changelog,
			Metadata:   r.Metadata,
			Timestamp:  r.Timestamp,
		}

		if err := s.store.IngestRelease(ctx, sourceID, event); err != nil {
			slog.Warn("ingest failed (may be duplicate)",
				"repo", r.Repository, "version", r.RawVersion, "err", err)
			continue
		}
	}
	return nil
}
