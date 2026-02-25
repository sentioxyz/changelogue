package ingestion

import "context"

// ReleaseStore persists release events using the transactional outbox pattern.
// The implementation inserts the release and enqueues a notify job atomically.
type ReleaseStore interface {
	IngestRelease(ctx context.Context, sourceID string, result *IngestionResult) error
}
