package pipeline

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/sentioxyz/releaseguard/internal/models"
)

// PgStore implements Store and SubscriptionChecker using PostgreSQL.
type PgStore struct {
	pool *pgxpool.Pool
}

// Compile-time interface assertions.
var (
	_ Store               = (*PgStore)(nil)
	_ SubscriptionChecker = (*PgStore)(nil)
)

// NewPgStore creates a PgStore backed by the given connection pool.
func NewPgStore(pool *pgxpool.Pool) *PgStore {
	return &PgStore{pool: pool}
}

// GetReleasePayload loads the ReleaseEvent IR from the releases table.
func (s *PgStore) GetReleasePayload(ctx context.Context, releaseID string) (*models.ReleaseEvent, error) {
	var payload []byte
	err := s.pool.QueryRow(ctx,
		`SELECT payload FROM releases WHERE id = $1`,
		releaseID,
	).Scan(&payload)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, fmt.Errorf("release %s not found", releaseID)
		}
		return nil, fmt.Errorf("query release payload: %w", err)
	}

	var event models.ReleaseEvent
	if err := json.Unmarshal(payload, &event); err != nil {
		return nil, fmt.Errorf("unmarshal release payload: %w", err)
	}
	return &event, nil
}

// CreatePipelineJob inserts a new pipeline_jobs row in "running" state.
func (s *PgStore) CreatePipelineJob(ctx context.Context, releaseID string) (int64, error) {
	var id int64
	err := s.pool.QueryRow(ctx,
		`INSERT INTO pipeline_jobs (release_id, state) VALUES ($1, 'running') RETURNING id`,
		releaseID,
	).Scan(&id)
	if err != nil {
		return 0, fmt.Errorf("insert pipeline job: %w", err)
	}
	return id, nil
}

// UpdateNodeProgress records the current node and accumulated results.
func (s *PgStore) UpdateNodeProgress(ctx context.Context, jobID int64, currentNode string, nodeResults map[string]json.RawMessage) error {
	resultsJSON, err := json.Marshal(nodeResults)
	if err != nil {
		return fmt.Errorf("marshal node results: %w", err)
	}

	_, err = s.pool.Exec(ctx,
		`UPDATE pipeline_jobs SET current_node = $2, node_results = $3 WHERE id = $1`,
		jobID, currentNode, resultsJSON,
	)
	if err != nil {
		return fmt.Errorf("update node progress: %w", err)
	}
	return nil
}

// CompletePipelineJob marks the job as "completed" with final node results.
func (s *PgStore) CompletePipelineJob(ctx context.Context, jobID int64, nodeResults map[string]json.RawMessage) error {
	resultsJSON, err := json.Marshal(nodeResults)
	if err != nil {
		return fmt.Errorf("marshal node results: %w", err)
	}

	_, err = s.pool.Exec(ctx,
		`UPDATE pipeline_jobs SET state = 'completed', node_results = $2, completed_at = NOW() WHERE id = $1`,
		jobID, resultsJSON,
	)
	if err != nil {
		return fmt.Errorf("complete pipeline job: %w", err)
	}
	return nil
}

// SkipPipelineJob marks the job as "skipped" (e.g., event dropped by a node).
func (s *PgStore) SkipPipelineJob(ctx context.Context, jobID int64, reason string) error {
	_, err := s.pool.Exec(ctx,
		`UPDATE pipeline_jobs SET state = 'skipped', error_message = $2, completed_at = NOW() WHERE id = $1`,
		jobID, reason,
	)
	if err != nil {
		return fmt.Errorf("skip pipeline job: %w", err)
	}
	return nil
}

// FailPipelineJob marks the job as "failed" with an error message.
func (s *PgStore) FailPipelineJob(ctx context.Context, jobID int64, errMsg string) error {
	_, err := s.pool.Exec(ctx,
		`UPDATE pipeline_jobs SET state = 'failed', error_message = $2, completed_at = NOW() WHERE id = $1`,
		jobID, errMsg,
	)
	if err != nil {
		return fmt.Errorf("fail pipeline job: %w", err)
	}
	return nil
}

// HasSubscribers returns true if the repository has at least one enabled
// subscription, or if the repository has no subscriptions configured at all
// ("default open" per-repository so releases are not silently dropped).
func (s *PgStore) HasSubscribers(ctx context.Context, repository string) (bool, error) {
	var has bool
	err := s.pool.QueryRow(ctx,
		`SELECT EXISTS(
			SELECT 1 FROM subscriptions sub
			JOIN sources src ON src.project_id = sub.project_id
			WHERE src.repository = $1 AND sub.enabled = true
		) OR NOT EXISTS(
			SELECT 1 FROM subscriptions sub
			JOIN sources src ON src.project_id = sub.project_id
			WHERE src.repository = $1
		)`,
		repository,
	).Scan(&has)
	if err != nil {
		return false, fmt.Errorf("check subscribers: %w", err)
	}
	return has, nil
}
