package ingestion

import (
	"context"

	"github.com/sentioxyz/releaseguard/internal/models"
)

// ReleaseStore persists release events using the transactional outbox pattern.
// The implementation inserts the release and enqueues a pipeline job atomically.
type ReleaseStore interface {
	IngestRelease(ctx context.Context, event *models.ReleaseEvent) error
}
