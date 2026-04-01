package ingestion

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/riverqueue/river"
	"github.com/sentioxyz/changelogue/internal/queue"
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

	// Build raw_data from Metadata + Changelog so nothing is lost.
	raw := make(map[string]string)
	for k, v := range result.Metadata {
		raw[k] = v
	}
	if result.Changelog != "" {
		raw["changelog"] = result.Changelog
	}
	rawData, err := json.Marshal(raw)
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

	_, err = s.river.InsertTx(ctx, tx, queue.GateCheckJobArgs{
		SourceID:  sourceID,
		ReleaseID: releaseID,
		Version:   result.RawVersion,
	}, nil)
	if err != nil {
		return fmt.Errorf("enqueue gate check: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit: %w", err)
	}
	return nil
}

// ListEnabledSources implements SourceLister.
func (s *PgStore) ListEnabledSources(ctx context.Context) ([]EnabledSource, error) {
	rows, err := s.pool.Query(ctx,
		`SELECT id, provider, repository, poll_interval_seconds, last_polled_at
		 FROM sources WHERE enabled = true`)
	if err != nil {
		return nil, fmt.Errorf("query enabled sources: %w", err)
	}
	defer rows.Close()

	var sources []EnabledSource
	for rows.Next() {
		var e EnabledSource
		if err := rows.Scan(&e.ID, &e.Provider, &e.Repository, &e.PollIntervalSeconds, &e.LastPolledAt); err != nil {
			return nil, fmt.Errorf("scan source row: %w", err)
		}
		sources = append(sources, e)
	}
	return sources, rows.Err()
}

// UpdateSourcePollStatus implements PollStatusUpdater.
func (s *PgStore) UpdateSourcePollStatus(ctx context.Context, id string, pollErr error) error {
	var lastError *string
	if pollErr != nil {
		s := pollErr.Error()
		lastError = &s
	}
	_, err := s.pool.Exec(ctx,
		`UPDATE sources SET last_polled_at = NOW(), last_error = $1 WHERE id = $2`,
		lastError, id)
	return err
}
