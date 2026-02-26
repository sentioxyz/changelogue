package api

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/riverqueue/river"
	"github.com/sentioxyz/changelogue/internal/models"
	"github.com/sentioxyz/changelogue/internal/queue"
)

// PgStore implements all store interfaces using a PostgreSQL connection pool.
type PgStore struct {
	pool  *pgxpool.Pool
	river *river.Client[pgx.Tx]
}

// NewPgStore returns a new PgStore backed by the given connection pool and River client.
// The river client may be nil if agent triggering is not needed.
func NewPgStore(pool *pgxpool.Pool, riverClient *river.Client[pgx.Tx]) *PgStore {
	return &PgStore{pool: pool, river: riverClient}
}

// --- ProjectsStore ---

func (s *PgStore) ListProjects(ctx context.Context, page, perPage int) ([]models.Project, int, error) {
	var total int
	err := s.pool.QueryRow(ctx, `SELECT COUNT(*) FROM projects`).Scan(&total)
	if err != nil {
		return nil, 0, fmt.Errorf("count projects: %w", err)
	}
	offset := (page - 1) * perPage
	rows, err := s.pool.Query(ctx,
		`SELECT id, name, COALESCE(description,''), COALESCE(agent_prompt,''),
		        COALESCE(agent_rules,'{}'), created_at, updated_at
		 FROM projects ORDER BY created_at DESC LIMIT $1 OFFSET $2`, perPage, offset)
	if err != nil {
		return nil, 0, fmt.Errorf("list projects: %w", err)
	}
	defer rows.Close()
	var projects []models.Project
	for rows.Next() {
		var p models.Project
		if err := rows.Scan(&p.ID, &p.Name, &p.Description, &p.AgentPrompt, &p.AgentRules, &p.CreatedAt, &p.UpdatedAt); err != nil {
			return nil, 0, fmt.Errorf("scan project: %w", err)
		}
		projects = append(projects, p)
	}
	return projects, total, nil
}

func (s *PgStore) CreateProject(ctx context.Context, p *models.Project) error {
	return s.pool.QueryRow(ctx,
		`INSERT INTO projects (name, description, agent_prompt, agent_rules)
		 VALUES ($1, $2, $3, $4)
		 RETURNING id, created_at, updated_at`,
		p.Name, p.Description, p.AgentPrompt, p.AgentRules,
	).Scan(&p.ID, &p.CreatedAt, &p.UpdatedAt)
}

func (s *PgStore) GetProject(ctx context.Context, id string) (*models.Project, error) {
	var p models.Project
	err := s.pool.QueryRow(ctx,
		`SELECT id, name, COALESCE(description,''), COALESCE(agent_prompt,''),
		        COALESCE(agent_rules,'{}'), created_at, updated_at
		 FROM projects WHERE id = $1`, id,
	).Scan(&p.ID, &p.Name, &p.Description, &p.AgentPrompt, &p.AgentRules, &p.CreatedAt, &p.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return &p, nil
}

func (s *PgStore) UpdateProject(ctx context.Context, id string, p *models.Project) error {
	return s.pool.QueryRow(ctx,
		`UPDATE projects SET name=$1, description=$2, agent_prompt=$3, agent_rules=$4, updated_at=NOW()
		 WHERE id=$5 RETURNING updated_at`,
		p.Name, p.Description, p.AgentPrompt, p.AgentRules, id,
	).Scan(&p.UpdatedAt)
}

func (s *PgStore) DeleteProject(ctx context.Context, id string) error {
	tag, err := s.pool.Exec(ctx, `DELETE FROM projects WHERE id = $1`, id)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("not found")
	}
	return nil
}

// --- KeyStore ---

func (s *PgStore) ValidateKey(ctx context.Context, rawKey string) (bool, error) {
	hash := sha256.Sum256([]byte(rawKey))
	keyHash := hex.EncodeToString(hash[:])
	var exists bool
	err := s.pool.QueryRow(ctx,
		`SELECT EXISTS(SELECT 1 FROM api_keys WHERE key_hash = $1)`, keyHash,
	).Scan(&exists)
	return exists, err
}

func (s *PgStore) TouchKeyUsage(ctx context.Context, rawKey string) {
	hash := sha256.Sum256([]byte(rawKey))
	keyHash := hex.EncodeToString(hash[:])
	_, _ = s.pool.Exec(ctx, `UPDATE api_keys SET last_used_at = NOW() WHERE key_hash = $1`, keyHash)
}

// --- SourcesStore ---

func (s *PgStore) ListSourcesByProject(ctx context.Context, projectID string, page, perPage int) ([]models.Source, int, error) {
	var total int
	err := s.pool.QueryRow(ctx, `SELECT COUNT(*) FROM sources WHERE project_id = $1`, projectID).Scan(&total)
	if err != nil {
		return nil, 0, fmt.Errorf("count sources: %w", err)
	}
	offset := (page - 1) * perPage
	rows, err := s.pool.Query(ctx,
		`SELECT id, project_id, provider, repository, poll_interval_seconds, enabled,
		        COALESCE(config,'{}'), last_polled_at, last_error, created_at, updated_at
		 FROM sources WHERE project_id = $1 ORDER BY created_at DESC LIMIT $2 OFFSET $3`, projectID, perPage, offset)
	if err != nil {
		return nil, 0, fmt.Errorf("list sources: %w", err)
	}
	defer rows.Close()
	var sources []models.Source
	for rows.Next() {
		var src models.Source
		if err := rows.Scan(&src.ID, &src.ProjectID, &src.Provider, &src.Repository,
			&src.PollIntervalSeconds, &src.Enabled, &src.Config,
			&src.LastPolledAt, &src.LastError, &src.CreatedAt, &src.UpdatedAt); err != nil {
			return nil, 0, fmt.Errorf("scan source: %w", err)
		}
		sources = append(sources, src)
	}
	return sources, total, nil
}

func (s *PgStore) CreateSource(ctx context.Context, src *models.Source) error {
	return s.pool.QueryRow(ctx,
		`INSERT INTO sources (project_id, provider, repository, poll_interval_seconds, enabled, config)
		 VALUES ($1, $2, $3, $4, $5, $6)
		 RETURNING id, enabled, created_at, updated_at`,
		src.ProjectID, src.Provider, src.Repository, src.PollIntervalSeconds, src.Enabled, src.Config,
	).Scan(&src.ID, &src.Enabled, &src.CreatedAt, &src.UpdatedAt)
}

func (s *PgStore) GetSource(ctx context.Context, id string) (*models.Source, error) {
	var src models.Source
	err := s.pool.QueryRow(ctx,
		`SELECT id, project_id, provider, repository, poll_interval_seconds, enabled,
		        COALESCE(config,'{}'), last_polled_at, last_error, created_at, updated_at
		 FROM sources WHERE id = $1`, id,
	).Scan(&src.ID, &src.ProjectID, &src.Provider, &src.Repository,
		&src.PollIntervalSeconds, &src.Enabled, &src.Config,
		&src.LastPolledAt, &src.LastError, &src.CreatedAt, &src.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return &src, nil
}

func (s *PgStore) UpdateSource(ctx context.Context, id string, src *models.Source) error {
	return s.pool.QueryRow(ctx,
		`UPDATE sources SET provider=$1, repository=$2, poll_interval_seconds=$3, enabled=$4,
		        config=$5, updated_at=NOW()
		 WHERE id=$6 RETURNING updated_at`,
		src.Provider, src.Repository, src.PollIntervalSeconds, src.Enabled, src.Config, id,
	).Scan(&src.UpdatedAt)
}

func (s *PgStore) DeleteSource(ctx context.Context, id string) error {
	tag, err := s.pool.Exec(ctx, `DELETE FROM sources WHERE id = $1`, id)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("not found")
	}
	return nil
}

// --- ReleasesStore ---

func (s *PgStore) ListAllReleases(ctx context.Context, page, perPage int) ([]models.Release, int, error) {
	var total int
	err := s.pool.QueryRow(ctx, `SELECT COUNT(*) FROM releases`).Scan(&total)
	if err != nil {
		return nil, 0, fmt.Errorf("count releases: %w", err)
	}
	offset := (page - 1) * perPage
	rows, err := s.pool.Query(ctx,
		`SELECT id, source_id, version, COALESCE(raw_data,'{}'), released_at, created_at
		 FROM releases ORDER BY COALESCE(released_at, created_at) DESC LIMIT $1 OFFSET $2`, perPage, offset)
	if err != nil {
		return nil, 0, fmt.Errorf("list all releases: %w", err)
	}
	defer rows.Close()
	var releases []models.Release
	for rows.Next() {
		var rel models.Release
		if err := rows.Scan(&rel.ID, &rel.SourceID, &rel.Version, &rel.RawData, &rel.ReleasedAt, &rel.CreatedAt); err != nil {
			return nil, 0, fmt.Errorf("scan release: %w", err)
		}
		releases = append(releases, rel)
	}
	return releases, total, nil
}

func (s *PgStore) ListReleasesBySource(ctx context.Context, sourceID string, page, perPage int) ([]models.Release, int, error) {
	var total int
	err := s.pool.QueryRow(ctx, `SELECT COUNT(*) FROM releases WHERE source_id = $1`, sourceID).Scan(&total)
	if err != nil {
		return nil, 0, fmt.Errorf("count releases: %w", err)
	}
	offset := (page - 1) * perPage
	rows, err := s.pool.Query(ctx,
		`SELECT id, source_id, version, COALESCE(raw_data,'{}'), released_at, created_at
		 FROM releases WHERE source_id = $1 ORDER BY COALESCE(released_at, created_at) DESC LIMIT $2 OFFSET $3`, sourceID, perPage, offset)
	if err != nil {
		return nil, 0, fmt.Errorf("list releases by source: %w", err)
	}
	defer rows.Close()
	var releases []models.Release
	for rows.Next() {
		var rel models.Release
		if err := rows.Scan(&rel.ID, &rel.SourceID, &rel.Version, &rel.RawData, &rel.ReleasedAt, &rel.CreatedAt); err != nil {
			return nil, 0, fmt.Errorf("scan release: %w", err)
		}
		releases = append(releases, rel)
	}
	return releases, total, nil
}

func (s *PgStore) ListReleasesByProject(ctx context.Context, projectID string, page, perPage int) ([]models.Release, int, error) {
	var total int
	err := s.pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM releases r JOIN sources s ON r.source_id = s.id WHERE s.project_id = $1`,
		projectID).Scan(&total)
	if err != nil {
		return nil, 0, fmt.Errorf("count releases: %w", err)
	}
	offset := (page - 1) * perPage
	rows, err := s.pool.Query(ctx,
		`SELECT r.id, r.source_id, r.version, COALESCE(r.raw_data,'{}'), r.released_at, r.created_at
		 FROM releases r JOIN sources s ON r.source_id = s.id
		 WHERE s.project_id = $1 ORDER BY COALESCE(r.released_at, r.created_at) DESC LIMIT $2 OFFSET $3`, projectID, perPage, offset)
	if err != nil {
		return nil, 0, fmt.Errorf("list releases by project: %w", err)
	}
	defer rows.Close()
	var releases []models.Release
	for rows.Next() {
		var rel models.Release
		if err := rows.Scan(&rel.ID, &rel.SourceID, &rel.Version, &rel.RawData, &rel.ReleasedAt, &rel.CreatedAt); err != nil {
			return nil, 0, fmt.Errorf("scan release: %w", err)
		}
		releases = append(releases, rel)
	}
	return releases, total, nil
}

func (s *PgStore) GetRelease(ctx context.Context, id string) (*models.Release, error) {
	var rel models.Release
	err := s.pool.QueryRow(ctx,
		`SELECT id, source_id, version, COALESCE(raw_data,'{}'), released_at, created_at
		 FROM releases WHERE id = $1`, id,
	).Scan(&rel.ID, &rel.SourceID, &rel.Version, &rel.RawData, &rel.ReleasedAt, &rel.CreatedAt)
	if err != nil {
		return nil, err
	}
	return &rel, nil
}

// --- SubscriptionsStore ---

func (s *PgStore) ListSubscriptions(ctx context.Context, page, perPage int) ([]models.Subscription, int, error) {
	var total int
	err := s.pool.QueryRow(ctx, `SELECT COUNT(*) FROM subscriptions`).Scan(&total)
	if err != nil {
		return nil, 0, fmt.Errorf("count subscriptions: %w", err)
	}
	offset := (page - 1) * perPage
	rows, err := s.pool.Query(ctx,
		`SELECT id, channel_id, type, source_id, project_id, COALESCE(version_filter,''), created_at
		 FROM subscriptions ORDER BY created_at DESC LIMIT $1 OFFSET $2`, perPage, offset)
	if err != nil {
		return nil, 0, fmt.Errorf("list subscriptions: %w", err)
	}
	defer rows.Close()
	var subs []models.Subscription
	for rows.Next() {
		var sub models.Subscription
		if err := rows.Scan(&sub.ID, &sub.ChannelID, &sub.Type, &sub.SourceID,
			&sub.ProjectID, &sub.VersionFilter, &sub.CreatedAt); err != nil {
			return nil, 0, fmt.Errorf("scan subscription: %w", err)
		}
		subs = append(subs, sub)
	}
	return subs, total, nil
}

func (s *PgStore) CreateSubscription(ctx context.Context, sub *models.Subscription) error {
	return s.pool.QueryRow(ctx,
		`INSERT INTO subscriptions (channel_id, type, source_id, project_id, version_filter)
		 VALUES ($1, $2, $3, $4, $5)
		 RETURNING id, created_at`,
		sub.ChannelID, sub.Type, sub.SourceID, sub.ProjectID, sub.VersionFilter,
	).Scan(&sub.ID, &sub.CreatedAt)
}

func (s *PgStore) GetSubscription(ctx context.Context, id string) (*models.Subscription, error) {
	var sub models.Subscription
	err := s.pool.QueryRow(ctx,
		`SELECT id, channel_id, type, source_id, project_id, COALESCE(version_filter,''), created_at
		 FROM subscriptions WHERE id = $1`, id,
	).Scan(&sub.ID, &sub.ChannelID, &sub.Type, &sub.SourceID,
		&sub.ProjectID, &sub.VersionFilter, &sub.CreatedAt)
	if err != nil {
		return nil, err
	}
	return &sub, nil
}

func (s *PgStore) UpdateSubscription(ctx context.Context, id string, sub *models.Subscription) error {
	return s.pool.QueryRow(ctx,
		`UPDATE subscriptions SET channel_id=$1, type=$2, source_id=$3, project_id=$4, version_filter=$5
		 WHERE id=$6 RETURNING created_at`,
		sub.ChannelID, sub.Type, sub.SourceID, sub.ProjectID, sub.VersionFilter, id,
	).Scan(&sub.CreatedAt)
}

func (s *PgStore) DeleteSubscription(ctx context.Context, id string) error {
	tag, err := s.pool.Exec(ctx, `DELETE FROM subscriptions WHERE id = $1`, id)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("not found")
	}
	return nil
}

// --- ChannelsStore ---

func (s *PgStore) ListChannels(ctx context.Context, page, perPage int) ([]models.NotificationChannel, int, error) {
	var total int
	err := s.pool.QueryRow(ctx, `SELECT COUNT(*) FROM notification_channels`).Scan(&total)
	if err != nil {
		return nil, 0, fmt.Errorf("count channels: %w", err)
	}
	offset := (page - 1) * perPage
	rows, err := s.pool.Query(ctx,
		`SELECT id, name, type, config, created_at, updated_at
		 FROM notification_channels ORDER BY created_at DESC LIMIT $1 OFFSET $2`, perPage, offset)
	if err != nil {
		return nil, 0, fmt.Errorf("list channels: %w", err)
	}
	defer rows.Close()
	var channels []models.NotificationChannel
	for rows.Next() {
		var ch models.NotificationChannel
		if err := rows.Scan(&ch.ID, &ch.Name, &ch.Type, &ch.Config, &ch.CreatedAt, &ch.UpdatedAt); err != nil {
			return nil, 0, fmt.Errorf("scan channel: %w", err)
		}
		channels = append(channels, ch)
	}
	return channels, total, nil
}

func (s *PgStore) CreateChannel(ctx context.Context, ch *models.NotificationChannel) error {
	return s.pool.QueryRow(ctx,
		`INSERT INTO notification_channels (name, type, config)
		 VALUES ($1, $2, $3)
		 RETURNING id, created_at, updated_at`,
		ch.Name, ch.Type, ch.Config,
	).Scan(&ch.ID, &ch.CreatedAt, &ch.UpdatedAt)
}

func (s *PgStore) GetChannel(ctx context.Context, id string) (*models.NotificationChannel, error) {
	var ch models.NotificationChannel
	err := s.pool.QueryRow(ctx,
		`SELECT id, name, type, config, created_at, updated_at
		 FROM notification_channels WHERE id = $1`, id,
	).Scan(&ch.ID, &ch.Name, &ch.Type, &ch.Config, &ch.CreatedAt, &ch.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return &ch, nil
}

func (s *PgStore) UpdateChannel(ctx context.Context, id string, ch *models.NotificationChannel) error {
	return s.pool.QueryRow(ctx,
		`UPDATE notification_channels SET name=$1, type=$2, config=$3, updated_at=NOW()
		 WHERE id=$4 RETURNING updated_at`,
		ch.Name, ch.Type, ch.Config, id,
	).Scan(&ch.UpdatedAt)
}

func (s *PgStore) DeleteChannel(ctx context.Context, id string) error {
	tag, err := s.pool.Exec(ctx, `DELETE FROM notification_channels WHERE id = $1`, id)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("not found")
	}
	return nil
}

// --- ContextSourcesStore ---

func (s *PgStore) ListContextSources(ctx context.Context, projectID string, page, perPage int) ([]models.ContextSource, int, error) {
	var total int
	err := s.pool.QueryRow(ctx, `SELECT COUNT(*) FROM context_sources WHERE project_id = $1`, projectID).Scan(&total)
	if err != nil {
		return nil, 0, fmt.Errorf("count context sources: %w", err)
	}
	offset := (page - 1) * perPage
	rows, err := s.pool.Query(ctx,
		`SELECT id, project_id, type, name, config, created_at, updated_at
		 FROM context_sources WHERE project_id = $1 ORDER BY created_at DESC LIMIT $2 OFFSET $3`, projectID, perPage, offset)
	if err != nil {
		return nil, 0, fmt.Errorf("list context sources: %w", err)
	}
	defer rows.Close()
	var sources []models.ContextSource
	for rows.Next() {
		var cs models.ContextSource
		if err := rows.Scan(&cs.ID, &cs.ProjectID, &cs.Type, &cs.Name, &cs.Config, &cs.CreatedAt, &cs.UpdatedAt); err != nil {
			return nil, 0, fmt.Errorf("scan context source: %w", err)
		}
		sources = append(sources, cs)
	}
	return sources, total, nil
}

func (s *PgStore) CreateContextSource(ctx context.Context, cs *models.ContextSource) error {
	return s.pool.QueryRow(ctx,
		`INSERT INTO context_sources (project_id, type, name, config)
		 VALUES ($1, $2, $3, $4)
		 RETURNING id, created_at, updated_at`,
		cs.ProjectID, cs.Type, cs.Name, cs.Config,
	).Scan(&cs.ID, &cs.CreatedAt, &cs.UpdatedAt)
}

func (s *PgStore) GetContextSource(ctx context.Context, id string) (*models.ContextSource, error) {
	var cs models.ContextSource
	err := s.pool.QueryRow(ctx,
		`SELECT id, project_id, type, name, config, created_at, updated_at
		 FROM context_sources WHERE id = $1`, id,
	).Scan(&cs.ID, &cs.ProjectID, &cs.Type, &cs.Name, &cs.Config, &cs.CreatedAt, &cs.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return &cs, nil
}

func (s *PgStore) UpdateContextSource(ctx context.Context, id string, cs *models.ContextSource) error {
	return s.pool.QueryRow(ctx,
		`UPDATE context_sources SET type=$1, name=$2, config=$3, updated_at=NOW()
		 WHERE id=$4 RETURNING updated_at`,
		cs.Type, cs.Name, cs.Config, id,
	).Scan(&cs.UpdatedAt)
}

func (s *PgStore) DeleteContextSource(ctx context.Context, id string) error {
	tag, err := s.pool.Exec(ctx, `DELETE FROM context_sources WHERE id = $1`, id)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("not found")
	}
	return nil
}

// --- SemanticReleasesStore ---

func (s *PgStore) ListSemanticReleases(ctx context.Context, projectID string, page, perPage int) ([]models.SemanticRelease, int, error) {
	var total int
	err := s.pool.QueryRow(ctx, `SELECT COUNT(*) FROM semantic_releases WHERE project_id = $1`, projectID).Scan(&total)
	if err != nil {
		return nil, 0, fmt.Errorf("count semantic releases: %w", err)
	}
	offset := (page - 1) * perPage
	rows, err := s.pool.Query(ctx,
		`SELECT id, project_id, version, COALESCE(report,'{}'), status, COALESCE(error,''),
		        created_at, completed_at
		 FROM semantic_releases WHERE project_id = $1 ORDER BY created_at DESC LIMIT $2 OFFSET $3`, projectID, perPage, offset)
	if err != nil {
		return nil, 0, fmt.Errorf("list semantic releases: %w", err)
	}
	defer rows.Close()
	var releases []models.SemanticRelease
	for rows.Next() {
		var sr models.SemanticRelease
		if err := rows.Scan(&sr.ID, &sr.ProjectID, &sr.Version, &sr.Report, &sr.Status, &sr.Error,
			&sr.CreatedAt, &sr.CompletedAt); err != nil {
			return nil, 0, fmt.Errorf("scan semantic release: %w", err)
		}
		releases = append(releases, sr)
	}
	return releases, total, nil
}

func (s *PgStore) GetSemanticRelease(ctx context.Context, id string) (*models.SemanticRelease, error) {
	var sr models.SemanticRelease
	err := s.pool.QueryRow(ctx,
		`SELECT id, project_id, version, COALESCE(report,'{}'), status, COALESCE(error,''),
		        created_at, completed_at
		 FROM semantic_releases WHERE id = $1`, id,
	).Scan(&sr.ID, &sr.ProjectID, &sr.Version, &sr.Report, &sr.Status, &sr.Error,
		&sr.CreatedAt, &sr.CompletedAt)
	if err != nil {
		return nil, err
	}
	return &sr, nil
}

func (s *PgStore) GetSemanticReleaseSources(ctx context.Context, id string) ([]models.Release, error) {
	rows, err := s.pool.Query(ctx,
		`SELECT r.id, r.source_id, r.version, COALESCE(r.raw_data,'{}'), r.released_at, r.created_at
		 FROM releases r
		 JOIN semantic_release_sources srs ON srs.release_id = r.id
		 WHERE srs.semantic_release_id = $1 ORDER BY COALESCE(r.released_at, r.created_at) DESC`, id)
	if err != nil {
		return nil, fmt.Errorf("list semantic release sources: %w", err)
	}
	defer rows.Close()
	var releases []models.Release
	for rows.Next() {
		var rel models.Release
		if err := rows.Scan(&rel.ID, &rel.SourceID, &rel.Version, &rel.RawData, &rel.ReleasedAt, &rel.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan release: %w", err)
		}
		releases = append(releases, rel)
	}
	return releases, nil
}

// --- AgentStore ---

func (s *PgStore) TriggerAgentRun(ctx context.Context, projectID, trigger string) (*models.AgentRun, error) {
	var run models.AgentRun
	err := s.pool.QueryRow(ctx,
		`INSERT INTO agent_runs (project_id, trigger, status)
		 VALUES ($1, $2, 'pending')
		 RETURNING id, project_id, trigger, status, created_at`,
		projectID, trigger,
	).Scan(&run.ID, &run.ProjectID, &run.Trigger, &run.Status, &run.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("insert agent run: %w", err)
	}

	// Enqueue a River job to process the agent run, if river client is available.
	if s.river != nil {
		_, err = s.river.Insert(ctx, queue.AgentJobArgs{
			AgentRunID: run.ID,
			ProjectID:  projectID,
		}, nil)
		if err != nil {
			return nil, fmt.Errorf("enqueue agent job: %w", err)
		}
	}

	return &run, nil
}

func (s *PgStore) ListAgentRuns(ctx context.Context, projectID string, page, perPage int) ([]models.AgentRun, int, error) {
	var total int
	err := s.pool.QueryRow(ctx, `SELECT COUNT(*) FROM agent_runs WHERE project_id = $1`, projectID).Scan(&total)
	if err != nil {
		return nil, 0, fmt.Errorf("count agent runs: %w", err)
	}
	offset := (page - 1) * perPage
	rows, err := s.pool.Query(ctx,
		`SELECT id, project_id, semantic_release_id, trigger, status,
		        COALESCE(prompt_used,''), COALESCE(error,''),
		        started_at, completed_at, created_at
		 FROM agent_runs WHERE project_id = $1 ORDER BY created_at DESC LIMIT $2 OFFSET $3`, projectID, perPage, offset)
	if err != nil {
		return nil, 0, fmt.Errorf("list agent runs: %w", err)
	}
	defer rows.Close()
	var runs []models.AgentRun
	for rows.Next() {
		var run models.AgentRun
		if err := rows.Scan(&run.ID, &run.ProjectID, &run.SemanticReleaseID, &run.Trigger, &run.Status,
			&run.PromptUsed, &run.Error, &run.StartedAt, &run.CompletedAt, &run.CreatedAt); err != nil {
			return nil, 0, fmt.Errorf("scan agent run: %w", err)
		}
		runs = append(runs, run)
	}
	return runs, total, nil
}

func (s *PgStore) GetAgentRun(ctx context.Context, id string) (*models.AgentRun, error) {
	var run models.AgentRun
	err := s.pool.QueryRow(ctx,
		`SELECT id, project_id, semantic_release_id, trigger, status,
		        COALESCE(prompt_used,''), COALESCE(error,''),
		        started_at, completed_at, created_at
		 FROM agent_runs WHERE id = $1`, id,
	).Scan(&run.ID, &run.ProjectID, &run.SemanticReleaseID, &run.Trigger, &run.Status,
		&run.PromptUsed, &run.Error, &run.StartedAt, &run.CompletedAt, &run.CreatedAt)
	if err != nil {
		return nil, err
	}
	return &run, nil
}

// --- NotifyStore (routing) ---

// ListSourceSubscriptions returns all source-level subscriptions for a given source.
func (s *PgStore) ListSourceSubscriptions(ctx context.Context, sourceID string) ([]models.Subscription, error) {
	rows, err := s.pool.Query(ctx,
		`SELECT id, channel_id, type, source_id, project_id, COALESCE(version_filter,''), created_at
		 FROM subscriptions
		 WHERE type = 'source' AND source_id = $1`, sourceID)
	if err != nil {
		return nil, fmt.Errorf("list source subscriptions: %w", err)
	}
	defer rows.Close()
	var subs []models.Subscription
	for rows.Next() {
		var sub models.Subscription
		if err := rows.Scan(&sub.ID, &sub.ChannelID, &sub.Type, &sub.SourceID,
			&sub.ProjectID, &sub.VersionFilter, &sub.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan subscription: %w", err)
		}
		subs = append(subs, sub)
	}
	return subs, nil
}

// GetPreviousRelease returns the most recent release for a source that is not
// the given version, ordered by created_at descending. Returns nil (no error)
// if no previous release exists.
func (s *PgStore) GetPreviousRelease(ctx context.Context, sourceID string, beforeVersion string) (*models.Release, error) {
	var rel models.Release
	err := s.pool.QueryRow(ctx,
		`SELECT id, source_id, version, COALESCE(raw_data,'{}'), released_at, created_at
		 FROM releases
		 WHERE source_id = $1 AND version != $2
		 ORDER BY COALESCE(released_at, created_at) DESC
		 LIMIT 1`, sourceID, beforeVersion,
	).Scan(&rel.ID, &rel.SourceID, &rel.Version, &rel.RawData, &rel.ReleasedAt, &rel.CreatedAt)
	if err != nil {
		if err.Error() == "no rows in result set" {
			return nil, nil
		}
		return nil, fmt.Errorf("get previous release: %w", err)
	}
	return &rel, nil
}

// EnqueueAgentRun creates an agent_run row and enqueues a River AgentJobArgs.
// This follows the transactional outbox pattern for zero-loss guarantee.
func (s *PgStore) EnqueueAgentRun(ctx context.Context, projectID, trigger string) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback(ctx)

	var runID string
	err = tx.QueryRow(ctx,
		`INSERT INTO agent_runs (project_id, trigger, status)
		 VALUES ($1, $2, 'pending')
		 RETURNING id`, projectID, trigger,
	).Scan(&runID)
	if err != nil {
		return fmt.Errorf("insert agent run: %w", err)
	}

	if s.river != nil {
		_, err = s.river.InsertTx(ctx, tx, queue.AgentJobArgs{
			AgentRunID: runID,
			ProjectID:  projectID,
		}, nil)
		if err != nil {
			return fmt.Errorf("enqueue agent job: %w", err)
		}
	}

	return tx.Commit(ctx)
}

// --- OrchestratorStore (agent layer) ---

// UpdateAgentRunStatus sets the status of an agent run and updates the
// started_at timestamp when transitioning to "running".
func (s *PgStore) UpdateAgentRunStatus(ctx context.Context, id, status string) error {
	var query string
	switch status {
	case "running":
		query = `UPDATE agent_runs SET status = $1, started_at = NOW() WHERE id = $2`
	case "completed":
		query = `UPDATE agent_runs SET status = $1, completed_at = NOW() WHERE id = $2`
	default:
		query = `UPDATE agent_runs SET status = $1 WHERE id = $2`
	}
	tag, err := s.pool.Exec(ctx, query, status, id)
	if err != nil {
		return fmt.Errorf("update agent run status: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("agent run not found: %s", id)
	}
	return nil
}

// CreateSemanticRelease inserts a semantic release and its source release links
// in a single transaction. The sr.ID field is populated on success.
func (s *PgStore) CreateSemanticRelease(ctx context.Context, sr *models.SemanticRelease, releaseIDs []string) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback(ctx)

	err = tx.QueryRow(ctx,
		`INSERT INTO semantic_releases (project_id, version, report, status, completed_at)
		 VALUES ($1, $2, $3, $4, $5)
		 RETURNING id, created_at`,
		sr.ProjectID, sr.Version, sr.Report, sr.Status, sr.CompletedAt,
	).Scan(&sr.ID, &sr.CreatedAt)
	if err != nil {
		return fmt.Errorf("insert semantic release: %w", err)
	}

	for _, relID := range releaseIDs {
		_, err = tx.Exec(ctx,
			`INSERT INTO semantic_release_sources (semantic_release_id, release_id)
			 VALUES ($1, $2) ON CONFLICT DO NOTHING`,
			sr.ID, relID,
		)
		if err != nil {
			return fmt.Errorf("insert semantic release source: %w", err)
		}
	}

	return tx.Commit(ctx)
}

// UpdateAgentRunResult sets the semantic_release_id for a completed agent run.
func (s *PgStore) UpdateAgentRunResult(ctx context.Context, id string, semanticReleaseID string) error {
	tag, err := s.pool.Exec(ctx,
		`UPDATE agent_runs SET semantic_release_id = $1 WHERE id = $2`,
		semanticReleaseID, id,
	)
	if err != nil {
		return fmt.Errorf("update agent run result: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("agent run not found: %s", id)
	}
	return nil
}

// ListProjectSubscriptions returns all project-level subscriptions for a given project.
func (s *PgStore) ListProjectSubscriptions(ctx context.Context, projectID string) ([]models.Subscription, error) {
	rows, err := s.pool.Query(ctx,
		`SELECT id, channel_id, type, source_id, project_id, COALESCE(version_filter,''), created_at
		 FROM subscriptions
		 WHERE type = 'project' AND project_id = $1`, projectID)
	if err != nil {
		return nil, fmt.Errorf("list project subscriptions: %w", err)
	}
	defer rows.Close()
	var subs []models.Subscription
	for rows.Next() {
		var sub models.Subscription
		if err := rows.Scan(&sub.ID, &sub.ChannelID, &sub.Type, &sub.SourceID,
			&sub.ProjectID, &sub.VersionFilter, &sub.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan subscription: %w", err)
		}
		subs = append(subs, sub)
	}
	return subs, nil
}

// --- HealthChecker ---

func (s *PgStore) Ping(ctx context.Context) error {
	return s.pool.Ping(ctx)
}

func (s *PgStore) PingDB(ctx context.Context) error {
	return s.pool.Ping(ctx)
}

func (s *PgStore) GetStats(ctx context.Context) (*DashboardStats, error) {
	var stats DashboardStats
	if err := s.pool.QueryRow(ctx, `SELECT COUNT(*) FROM releases`).Scan(&stats.TotalReleases); err != nil {
		return nil, fmt.Errorf("count releases: %w", err)
	}
	if err := s.pool.QueryRow(ctx, `SELECT COUNT(*) FROM sources WHERE enabled = true`).Scan(&stats.ActiveSources); err != nil {
		return nil, fmt.Errorf("count active sources: %w", err)
	}
	if err := s.pool.QueryRow(ctx, `SELECT COUNT(*) FROM projects`).Scan(&stats.TotalProjects); err != nil {
		return nil, fmt.Errorf("count projects: %w", err)
	}
	if err := s.pool.QueryRow(ctx, `SELECT COUNT(*) FROM agent_runs WHERE status = 'pending'`).Scan(&stats.PendingAgentRuns); err != nil {
		return nil, fmt.Errorf("count pending agent runs: %w", err)
	}
	return &stats, nil
}
