package ingestion

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/riverqueue/river"
	"github.com/sentioxyz/releaseguard/internal/models"
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

// IngestRelease inserts a release and enqueues a pipeline job in a single transaction.
// Returns an error on unique constraint violation (caller treats as idempotent skip).
func (s *PgStore) IngestRelease(ctx context.Context, event *models.ReleaseEvent) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback(ctx)

	payload, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("marshal payload: %w", err)
	}

	_, err = tx.Exec(ctx,
		`INSERT INTO releases (id, source, repository, version, payload) VALUES ($1, $2, $3, $4, $5)`,
		event.ID, event.Source, event.Repository, event.RawVersion, payload,
	)
	if err != nil {
		return fmt.Errorf("insert release: %w", err)
	}

	_, err = s.river.InsertTx(ctx, tx, queue.PipelineJobArgs{ReleaseID: event.ID}, nil)
	if err != nil {
		return fmt.Errorf("enqueue job: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit: %w", err)
	}
	return nil
}
