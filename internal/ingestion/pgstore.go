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

// IngestResult describes what happened when a release was ingested.
type IngestResult int

const (
	IngestNew     IngestResult = iota // brand-new release inserted
	IngestUpdated                     // existing release had its data updated
	IngestSkipped                     // existing release unchanged
)

// IngestRelease upserts a release and enqueues downstream jobs in a single transaction.
// New releases get notify + gate-check jobs. Updated releases (e.g. release notes added
// to a former pre-release) get re-notified so the agent can re-analyze.
func (s *PgStore) IngestRelease(ctx context.Context, sourceID string, result *IngestionResult) (IngestResult, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return IngestSkipped, fmt.Errorf("begin tx: %w", err)
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
		return IngestSkipped, fmt.Errorf("marshal raw_data: %w", err)
	}

	var releaseID string
	var xmax uint32
	err = tx.QueryRow(ctx,
		`INSERT INTO releases (id, source_id, version, raw_data, released_at)
		 VALUES ($1, $2, $3, $4, $5)
		 ON CONFLICT (source_id, version) DO UPDATE
		    SET raw_data = EXCLUDED.raw_data, released_at = EXCLUDED.released_at
		    WHERE releases.raw_data IS DISTINCT FROM EXCLUDED.raw_data
		       OR releases.released_at IS DISTINCT FROM EXCLUDED.released_at
		 RETURNING id, xmin::text::int`,
		uuid.New().String(), sourceID, result.RawVersion, rawData, result.Timestamp,
	).Scan(&releaseID, &xmax)
	if err != nil {
		if err.Error() == "no rows in result set" {
			return IngestSkipped, nil
		}
		return IngestSkipped, fmt.Errorf("upsert release: %w", err)
	}

	// xmax == 0 means a fresh insert; xmax > 0 means an existing row was updated.
	isNew := xmax == 0

	_, err = s.river.InsertTx(ctx, tx, queue.NotifyJobArgs{
		ReleaseID: releaseID,
		SourceID:  sourceID,
	}, nil)
	if err != nil {
		return IngestSkipped, fmt.Errorf("enqueue job: %w", err)
	}

	if isNew {
		_, err = s.river.InsertTx(ctx, tx, queue.GateCheckJobArgs{
			SourceID:  sourceID,
			ReleaseID: releaseID,
			Version:   result.RawVersion,
		}, nil)
		if err != nil {
			return IngestSkipped, fmt.Errorf("enqueue gate check: %w", err)
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return IngestSkipped, fmt.Errorf("commit: %w", err)
	}
	if isNew {
		return IngestNew, nil
	}
	return IngestUpdated, nil
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
