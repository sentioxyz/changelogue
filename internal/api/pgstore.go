package api

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/sentioxyz/releaseguard/internal/models"
)

// PgStore implements all store interfaces using a PostgreSQL connection pool.
// Methods for additional resources will be added in later tasks.
type PgStore struct {
	pool *pgxpool.Pool
}

// NewPgStore returns a new PgStore backed by the given connection pool.
func NewPgStore(pool *pgxpool.Pool) *PgStore {
	return &PgStore{pool: pool}
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
		`SELECT id, name, COALESCE(description,''), COALESCE(url,''), pipeline_config, created_at, updated_at
		 FROM projects ORDER BY created_at DESC LIMIT $1 OFFSET $2`, perPage, offset)
	if err != nil {
		return nil, 0, fmt.Errorf("list projects: %w", err)
	}
	defer rows.Close()
	var projects []models.Project
	for rows.Next() {
		var p models.Project
		if err := rows.Scan(&p.ID, &p.Name, &p.Description, &p.URL, &p.PipelineConfig, &p.CreatedAt, &p.UpdatedAt); err != nil {
			return nil, 0, fmt.Errorf("scan project: %w", err)
		}
		projects = append(projects, p)
	}
	return projects, total, nil
}

func (s *PgStore) CreateProject(ctx context.Context, p *models.Project) error {
	return s.pool.QueryRow(ctx,
		`INSERT INTO projects (name, description, url, pipeline_config)
		 VALUES ($1, $2, $3, $4)
		 RETURNING id, created_at, updated_at`,
		p.Name, p.Description, p.URL, p.PipelineConfig,
	).Scan(&p.ID, &p.CreatedAt, &p.UpdatedAt)
}

func (s *PgStore) GetProject(ctx context.Context, id int) (*models.Project, error) {
	var p models.Project
	err := s.pool.QueryRow(ctx,
		`SELECT id, name, COALESCE(description,''), COALESCE(url,''), pipeline_config, created_at, updated_at
		 FROM projects WHERE id = $1`, id,
	).Scan(&p.ID, &p.Name, &p.Description, &p.URL, &p.PipelineConfig, &p.CreatedAt, &p.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return &p, nil
}

func (s *PgStore) UpdateProject(ctx context.Context, id int, p *models.Project) error {
	return s.pool.QueryRow(ctx,
		`UPDATE projects SET name=$1, description=$2, url=$3, pipeline_config=$4, updated_at=NOW()
		 WHERE id=$5 RETURNING updated_at`,
		p.Name, p.Description, p.URL, p.PipelineConfig, id,
	).Scan(&p.UpdatedAt)
}

func (s *PgStore) DeleteProject(ctx context.Context, id int) error {
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

func (s *PgStore) ListSources(ctx context.Context, page, perPage int) ([]models.Source, int, error) {
	var total int
	err := s.pool.QueryRow(ctx, `SELECT COUNT(*) FROM sources`).Scan(&total)
	if err != nil {
		return nil, 0, fmt.Errorf("count sources: %w", err)
	}
	offset := (page - 1) * perPage
	rows, err := s.pool.Query(ctx,
		`SELECT id, project_id, source_type, repository, poll_interval_seconds, enabled,
		        COALESCE(exclude_version_regexp,''), exclude_prereleases, last_polled_at, last_error,
		        created_at, updated_at
		 FROM sources ORDER BY created_at DESC LIMIT $1 OFFSET $2`, perPage, offset)
	if err != nil {
		return nil, 0, fmt.Errorf("list sources: %w", err)
	}
	defer rows.Close()
	var sources []models.Source
	for rows.Next() {
		var src models.Source
		if err := rows.Scan(&src.ID, &src.ProjectID, &src.SourceType, &src.Repository,
			&src.PollIntervalSeconds, &src.Enabled, &src.ExcludeVersionRegexp, &src.ExcludePrereleases,
			&src.LastPolledAt, &src.LastError, &src.CreatedAt, &src.UpdatedAt); err != nil {
			return nil, 0, fmt.Errorf("scan source: %w", err)
		}
		sources = append(sources, src)
	}
	return sources, total, nil
}

func (s *PgStore) CreateSource(ctx context.Context, src *models.Source) error {
	return s.pool.QueryRow(ctx,
		`INSERT INTO sources (project_id, source_type, repository, poll_interval_seconds, enabled, exclude_version_regexp, exclude_prereleases)
		 VALUES ($1, $2, $3, $4, $5, $6, $7)
		 RETURNING id, enabled, created_at, updated_at`,
		src.ProjectID, src.SourceType, src.Repository, src.PollIntervalSeconds, src.Enabled,
		src.ExcludeVersionRegexp, src.ExcludePrereleases,
	).Scan(&src.ID, &src.Enabled, &src.CreatedAt, &src.UpdatedAt)
}

func (s *PgStore) GetSource(ctx context.Context, id int) (*models.Source, error) {
	var src models.Source
	err := s.pool.QueryRow(ctx,
		`SELECT id, project_id, source_type, repository, poll_interval_seconds, enabled,
		        COALESCE(exclude_version_regexp,''), exclude_prereleases, last_polled_at, last_error,
		        created_at, updated_at
		 FROM sources WHERE id = $1`, id,
	).Scan(&src.ID, &src.ProjectID, &src.SourceType, &src.Repository,
		&src.PollIntervalSeconds, &src.Enabled, &src.ExcludeVersionRegexp, &src.ExcludePrereleases,
		&src.LastPolledAt, &src.LastError, &src.CreatedAt, &src.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return &src, nil
}

func (s *PgStore) UpdateSource(ctx context.Context, id int, src *models.Source) error {
	return s.pool.QueryRow(ctx,
		`UPDATE sources SET source_type=$1, repository=$2, poll_interval_seconds=$3, enabled=$4,
		        exclude_version_regexp=$5, exclude_prereleases=$6, updated_at=NOW()
		 WHERE id=$7 RETURNING updated_at`,
		src.SourceType, src.Repository, src.PollIntervalSeconds, src.Enabled,
		src.ExcludeVersionRegexp, src.ExcludePrereleases, id,
	).Scan(&src.UpdatedAt)
}

func (s *PgStore) DeleteSource(ctx context.Context, id int) error {
	tag, err := s.pool.Exec(ctx, `DELETE FROM sources WHERE id = $1`, id)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("not found")
	}
	return nil
}

func (s *PgStore) GetLatestRelease(ctx context.Context, sourceID int) (*ReleaseView, error) {
	var rv ReleaseView
	var isPreStr string
	var createdAt time.Time
	err := s.pool.QueryRow(ctx,
		releaseViewQuery+` WHERE s.id = $1 ORDER BY r.created_at DESC LIMIT 1`, sourceID,
	).Scan(&rv.ID, &rv.SourceID, &rv.SourceType, &rv.Repository, &rv.ProjectID, &rv.ProjectName,
		&rv.RawVersion, &isPreStr, &rv.PipelineStatus, &createdAt)
	if err != nil {
		return nil, err
	}
	rv.IsPreRelease = isPreStr == "true"
	rv.CreatedAt = createdAt.Format(time.RFC3339)
	return &rv, nil
}

func (s *PgStore) GetReleaseByVersion(ctx context.Context, sourceID int, version string) (*ReleaseView, error) {
	var rv ReleaseView
	var isPreStr string
	var createdAt time.Time
	err := s.pool.QueryRow(ctx,
		releaseViewQuery+` WHERE s.id = $1 AND r.version = $2`, sourceID, version,
	).Scan(&rv.ID, &rv.SourceID, &rv.SourceType, &rv.Repository, &rv.ProjectID, &rv.ProjectName,
		&rv.RawVersion, &isPreStr, &rv.PipelineStatus, &createdAt)
	if err != nil {
		return nil, err
	}
	rv.IsPreRelease = isPreStr == "true"
	rv.CreatedAt = createdAt.Format(time.RFC3339)
	return &rv, nil
}

// --- ReleasesStore ---

func (s *PgStore) ListReleases(ctx context.Context, opts ListReleasesOpts) ([]ReleaseView, int, error) {
	// Build dynamic WHERE clause.
	var conditions []string
	var args []any
	argIdx := 1

	if opts.ProjectID != nil {
		conditions = append(conditions, fmt.Sprintf("p.id = $%d", argIdx))
		args = append(args, *opts.ProjectID)
		argIdx++
	}
	if opts.SourceID != nil {
		conditions = append(conditions, fmt.Sprintf("s.id = $%d", argIdx))
		args = append(args, *opts.SourceID)
		argIdx++
	}

	where := ""
	if len(conditions) > 0 {
		where = " WHERE " + strings.Join(conditions, " AND ")
	}

	// Count query.
	countQuery := `SELECT COUNT(*) FROM releases r
		JOIN sources s ON r.source_id = s.id
		JOIN projects p ON s.project_id = p.id` + where
	var total int
	err := s.pool.QueryRow(ctx, countQuery, args...).Scan(&total)
	if err != nil {
		return nil, 0, fmt.Errorf("count releases: %w", err)
	}

	// Sort clause.
	sortCol := "r.created_at"
	if opts.Sort == "version" {
		sortCol = "r.version"
	}
	order := "DESC"
	if strings.EqualFold(opts.Order, "asc") {
		order = "ASC"
	}

	// Data query.
	dataQuery := releaseViewQuery + where +
		fmt.Sprintf(" ORDER BY %s %s LIMIT $%d OFFSET $%d", sortCol, order, argIdx, argIdx+1)
	args = append(args, opts.PerPage, (opts.Page-1)*opts.PerPage)

	rows, err := s.pool.Query(ctx, dataQuery, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("list releases: %w", err)
	}
	defer rows.Close()

	var releases []ReleaseView
	for rows.Next() {
		var rv ReleaseView
		var isPreStr string
		var createdAt time.Time
		if err := rows.Scan(&rv.ID, &rv.SourceID, &rv.SourceType, &rv.Repository, &rv.ProjectID, &rv.ProjectName,
			&rv.RawVersion, &isPreStr, &rv.PipelineStatus, &createdAt); err != nil {
			return nil, 0, fmt.Errorf("scan release: %w", err)
		}
		rv.IsPreRelease = isPreStr == "true"
		rv.CreatedAt = createdAt.Format(time.RFC3339)
		releases = append(releases, rv)
	}
	return releases, total, nil
}

func (s *PgStore) GetRelease(ctx context.Context, id string) (*ReleaseView, error) {
	var rv ReleaseView
	var isPreStr string
	var createdAt time.Time
	err := s.pool.QueryRow(ctx,
		releaseViewQuery+` WHERE r.id = $1`, id,
	).Scan(&rv.ID, &rv.SourceID, &rv.SourceType, &rv.Repository, &rv.ProjectID, &rv.ProjectName,
		&rv.RawVersion, &isPreStr, &rv.PipelineStatus, &createdAt)
	if err != nil {
		return nil, err
	}
	rv.IsPreRelease = isPreStr == "true"
	rv.CreatedAt = createdAt.Format(time.RFC3339)
	return &rv, nil
}

func (s *PgStore) GetReleaseNotes(ctx context.Context, id string) (string, error) {
	var changelog string
	err := s.pool.QueryRow(ctx,
		`SELECT COALESCE(payload->>'changelog', '') FROM releases WHERE id = $1`, id,
	).Scan(&changelog)
	if err != nil {
		return "", err
	}
	return changelog, nil
}

func (s *PgStore) GetPipelineStatus(ctx context.Context, releaseID string) (*PipelineStatus, error) {
	var ps PipelineStatus
	var nodeResults []byte
	err := s.pool.QueryRow(ctx,
		`SELECT release_id, state, current_node, node_results, attempt, completed_at
		 FROM pipeline_jobs WHERE release_id = $1 ORDER BY created_at DESC LIMIT 1`, releaseID,
	).Scan(&ps.ReleaseID, &ps.State, &ps.CurrentNode, &nodeResults, &ps.Attempt, &ps.CompletedAt)
	if err != nil {
		return nil, err
	}
	ps.NodeResults = json.RawMessage(nodeResults)
	return &ps, nil
}

// --- HealthChecker ---

func (s *PgStore) Ping(ctx context.Context) error {
	return s.pool.Ping(ctx)
}
