package ingestion

import "context"

// ReleaseStore persists release events using the transactional outbox pattern.
// The implementation upserts the release and enqueues downstream jobs atomically.
type ReleaseStore interface {
	IngestRelease(ctx context.Context, sourceID string, result *IngestionResult) (IngestResult, error)
}
