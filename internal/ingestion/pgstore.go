package ingestion

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/riverqueue/river"
	"github.com/sentioxyz/releaseguard/internal/queue"
)

// PgStore implements ReleaseStore using PostgreSQL + River for the transactional outbox.
type PgStore struct {
	pool  *pgxpool.Pool
	river *river.Client[pgx.Tx]
}

func NewPgStore(pool *pgxpool.Pool, riverClient *river.Client[pgx.Tx]) *PgStore {
	return &PgStore{pool: pool, river: riverClient}
}

// IngestRelease inserts a release and enqueues a notify job in a single transaction.
// Returns an error on unique constraint violation (caller treats as idempotent skip).
func (s *PgStore) IngestRelease(ctx context.Context, sourceID string, result *IngestionResult) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback(ctx)

	rawData, err := json.Marshal(result.Metadata)
	if err != nil {
		return fmt.Errorf("marshal raw_data: %w", err)
	}

	releaseID := uuid.New().String()
	_, err = tx.Exec(ctx,
		`INSERT INTO releases (id, source_id, version, raw_data, released_at) VALUES ($1, $2, $3, $4, $5)`,
		releaseID, sourceID, result.RawVersion, rawData, result.Timestamp,
	)
	if err != nil {
		return fmt.Errorf("insert release: %w", err)
	}

	_, err = s.river.InsertTx(ctx, tx, queue.NotifyJobArgs{
		ReleaseID: releaseID,
		SourceID:  sourceID,
	}, nil)
	if err != nil {
		return fmt.Errorf("enqueue job: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit: %w", err)
	}
	return nil
}
