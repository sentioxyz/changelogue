package ingestion

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
)

// Service orchestrates ingestion sources and persists results.
type Service struct {
	store ReleaseStore
}

func NewService(store ReleaseStore) *Service {
	return &Service{store: store}
}

// ProcessResults persists raw ingestion results via the store.
// Duplicate releases (unique constraint violations) are logged and skipped, not fatal.
func (s *Service) ProcessResults(ctx context.Context, sourceID string, sourceName string, results []IngestionResult) error {
	for _, r := range results {
		if err := s.store.IngestRelease(ctx, sourceID, &r); err != nil {
			if isUniqueViolation(err) {
				slog.Debug("duplicate release, skipping", "version", r.RawVersion)
				continue
			}
			return fmt.Errorf("ingest %s: %w", r.RawVersion, err)
		}
		slog.Info("ingested release", "source", sourceName, "version", r.RawVersion)
	}
	return nil
}

// isUniqueViolation checks if the error is a PostgreSQL unique constraint violation.
// pgx wraps the PG error; we check for the SQLSTATE code 23505.
func isUniqueViolation(err error) bool {
	return strings.Contains(err.Error(), "23505") ||
		strings.Contains(err.Error(), "unique_violation")
}
