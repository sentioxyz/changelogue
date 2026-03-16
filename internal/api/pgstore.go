package api

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

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

// SetRiverClient updates the River client on an existing PgStore.
// This is used during bootstrap to inject the client after worker registration.
func (s *PgStore) SetRiverClient(rc *river.Client[pgx.Tx]) {
	s.river = rc
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
		        COALESCE(config,'{}'), version_filter_include, version_filter_exclude, exclude_prereleases,
		        last_polled_at, last_error, created_at, updated_at
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
			&src.VersionFilterInclude, &src.VersionFilterExclude, &src.ExcludePrereleases,
			&src.LastPolledAt, &src.LastError, &src.CreatedAt, &src.UpdatedAt); err != nil {
			return nil, 0, fmt.Errorf("scan source: %w", err)
		}
		sources = append(sources, src)
	}
	return sources, total, nil
}

func (s *PgStore) CreateSource(ctx context.Context, src *models.Source) error {
	return s.pool.QueryRow(ctx,
		`INSERT INTO sources (project_id, provider, repository, poll_interval_seconds, enabled, config,
		        version_filter_include, version_filter_exclude, exclude_prereleases)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		 RETURNING id, enabled, created_at, updated_at`,
		src.ProjectID, src.Provider, src.Repository, src.PollIntervalSeconds, src.Enabled, src.Config,
		src.VersionFilterInclude, src.VersionFilterExclude, src.ExcludePrereleases,
	).Scan(&src.ID, &src.Enabled, &src.CreatedAt, &src.UpdatedAt)
}

func (s *PgStore) GetSource(ctx context.Context, id string) (*models.Source, error) {
	var src models.Source
	err := s.pool.QueryRow(ctx,
		`SELECT id, project_id, provider, repository, poll_interval_seconds, enabled,
		        COALESCE(config,'{}'), version_filter_include, version_filter_exclude, exclude_prereleases,
		        last_polled_at, last_error, created_at, updated_at
		 FROM sources WHERE id = $1`, id,
	).Scan(&src.ID, &src.ProjectID, &src.Provider, &src.Repository,
		&src.PollIntervalSeconds, &src.Enabled, &src.Config,
		&src.VersionFilterInclude, &src.VersionFilterExclude, &src.ExcludePrereleases,
		&src.LastPolledAt, &src.LastError, &src.CreatedAt, &src.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return &src, nil
}

func (s *PgStore) UpdateSource(ctx context.Context, id string, src *models.Source) error {
	return s.pool.QueryRow(ctx,
		`UPDATE sources SET provider=$1, repository=$2, poll_interval_seconds=$3, enabled=$4,
		        config=$5, version_filter_include=$6, version_filter_exclude=$7, exclude_prereleases=$8, updated_at=NOW()
		 WHERE id=$9 RETURNING updated_at`,
		src.Provider, src.Repository, src.PollIntervalSeconds, src.Enabled, src.Config,
		src.VersionFilterInclude, src.VersionFilterExclude, src.ExcludePrereleases, id,
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

// --- ReleasesStore ---

func (s *PgStore) ListAllReleases(ctx context.Context, page, perPage int, includeExcluded bool) ([]models.Release, int, error) {
	var total int
	if includeExcluded {
		err := s.pool.QueryRow(ctx,
			`SELECT COUNT(*) FROM releases r
			 LEFT JOIN sources s ON r.source_id = s.id`).Scan(&total)
		if err != nil {
			return nil, 0, fmt.Errorf("count releases: %w", err)
		}
	} else {
		err := s.pool.QueryRow(ctx,
			`SELECT COUNT(*) FROM releases r
			 LEFT JOIN sources s ON r.source_id = s.id
			 WHERE (s.version_filter_include IS NULL OR r.version ~ s.version_filter_include)
			   AND (s.version_filter_exclude IS NULL OR r.version !~ s.version_filter_exclude)
			   AND (s.exclude_prereleases = false OR r.raw_data->>'prerelease' IS NULL OR r.raw_data->>'prerelease' != 'true')`).Scan(&total)
		if err != nil {
			return nil, 0, fmt.Errorf("count releases: %w", err)
		}
	}
	offset := (page - 1) * perPage
	var rows pgx.Rows
	var err error
	if includeExcluded {
		rows, err = s.pool.Query(ctx,
			`SELECT r.id, r.source_id, r.version, COALESCE(r.raw_data,'{}'), r.released_at, r.created_at,
			        COALESCE(p.id::text,''), COALESCE(p.name,''), COALESCE(s.provider,''), COALESCE(s.repository,''),
			        CASE WHEN
			          (s.version_filter_include IS NOT NULL AND r.version !~ s.version_filter_include)
			          OR (s.version_filter_exclude IS NOT NULL AND r.version ~ s.version_filter_exclude)
			          OR (s.exclude_prereleases = true AND r.raw_data->>'prerelease' = 'true')
			        THEN true ELSE false END,
			        COALESCE(sr_info.id::text,''), COALESCE(sr_info.status,''), COALESCE(sr_info.urgency,'')
			 FROM releases r
			 LEFT JOIN sources s ON r.source_id = s.id
			 LEFT JOIN projects p ON s.project_id = p.id
			 LEFT JOIN LATERAL (
			     (SELECT sr.id, sr.status, sr.report->>'urgency' AS urgency, 0 AS priority
			      FROM semantic_releases sr
			      WHERE sr.project_id = p.id AND sr.version = r.version
			      ORDER BY sr.created_at DESC LIMIT 1)
			     UNION ALL
			     (SELECT NULL::uuid, 'processing', '', 1
			      FROM agent_runs ar
			      WHERE ar.project_id = p.id AND ar.version = r.version
			        AND ar.status IN ('pending', 'running')
			      LIMIT 1)
			     ORDER BY priority LIMIT 1
			 ) sr_info ON true
			 ORDER BY COALESCE(r.released_at, r.created_at) DESC LIMIT $1 OFFSET $2`, perPage, offset)
	} else {
		rows, err = s.pool.Query(ctx,
			`SELECT r.id, r.source_id, r.version, COALESCE(r.raw_data,'{}'), r.released_at, r.created_at,
			        COALESCE(p.id::text,''), COALESCE(p.name,''), COALESCE(s.provider,''), COALESCE(s.repository,''),
			        false,
			        COALESCE(sr_info.id::text,''), COALESCE(sr_info.status,''), COALESCE(sr_info.urgency,'')
			 FROM releases r
			 LEFT JOIN sources s ON r.source_id = s.id
			 LEFT JOIN projects p ON s.project_id = p.id
			 LEFT JOIN LATERAL (
			     (SELECT sr.id, sr.status, sr.report->>'urgency' AS urgency, 0 AS priority
			      FROM semantic_releases sr
			      WHERE sr.project_id = p.id AND sr.version = r.version
			      ORDER BY sr.created_at DESC LIMIT 1)
			     UNION ALL
			     (SELECT NULL::uuid, 'processing', '', 1
			      FROM agent_runs ar
			      WHERE ar.project_id = p.id AND ar.version = r.version
			        AND ar.status IN ('pending', 'running')
			      LIMIT 1)
			     ORDER BY priority LIMIT 1
			 ) sr_info ON true
			 WHERE (s.version_filter_include IS NULL OR r.version ~ s.version_filter_include)
			   AND (s.version_filter_exclude IS NULL OR r.version !~ s.version_filter_exclude)
			   AND (s.exclude_prereleases = false OR r.raw_data->>'prerelease' IS NULL OR r.raw_data->>'prerelease' != 'true')
			 ORDER BY COALESCE(r.released_at, r.created_at) DESC LIMIT $1 OFFSET $2`, perPage, offset)
	}
	if err != nil {
		return nil, 0, fmt.Errorf("list all releases: %w", err)
	}
	defer rows.Close()
	var releases []models.Release
	for rows.Next() {
		var rel models.Release
		if err := rows.Scan(&rel.ID, &rel.SourceID, &rel.Version, &rel.RawData, &rel.ReleasedAt, &rel.CreatedAt,
			&rel.ProjectID, &rel.ProjectName, &rel.Provider, &rel.Repository, &rel.Excluded,
			&rel.SemanticReleaseID, &rel.SemanticReleaseStatus, &rel.SemanticReleaseUrgency); err != nil {
			return nil, 0, fmt.Errorf("scan release: %w", err)
		}
		releases = append(releases, rel)
	}
	return releases, total, nil
}

func (s *PgStore) ListReleasesBySource(ctx context.Context, sourceID string, page, perPage int, includeExcluded bool) ([]models.Release, int, error) {
	var total int
	if includeExcluded {
		err := s.pool.QueryRow(ctx,
			`SELECT COUNT(*) FROM releases r
			 JOIN sources s ON r.source_id = s.id
			 WHERE r.source_id = $1`, sourceID).Scan(&total)
		if err != nil {
			return nil, 0, fmt.Errorf("count releases: %w", err)
		}
	} else {
		err := s.pool.QueryRow(ctx,
			`SELECT COUNT(*) FROM releases r
			 JOIN sources s ON r.source_id = s.id
			 WHERE r.source_id = $1
			   AND (s.version_filter_include IS NULL OR r.version ~ s.version_filter_include)
			   AND (s.version_filter_exclude IS NULL OR r.version !~ s.version_filter_exclude)
			   AND (s.exclude_prereleases = false OR r.raw_data->>'prerelease' IS NULL OR r.raw_data->>'prerelease' != 'true')`, sourceID).Scan(&total)
		if err != nil {
			return nil, 0, fmt.Errorf("count releases: %w", err)
		}
	}
	offset := (page - 1) * perPage
	var rows pgx.Rows
	var err error
	if includeExcluded {
		rows, err = s.pool.Query(ctx,
			`SELECT r.id, r.source_id, r.version, COALESCE(r.raw_data,'{}'), r.released_at, r.created_at,
			        COALESCE(p.id::text,''), COALESCE(p.name,''), COALESCE(s.provider,''), COALESCE(s.repository,''),
			        CASE WHEN
			          (s.version_filter_include IS NOT NULL AND r.version !~ s.version_filter_include)
			          OR (s.version_filter_exclude IS NOT NULL AND r.version ~ s.version_filter_exclude)
			          OR (s.exclude_prereleases = true AND r.raw_data->>'prerelease' = 'true')
			        THEN true ELSE false END,
			        COALESCE(sr_info.id::text,''), COALESCE(sr_info.status,''), COALESCE(sr_info.urgency,'')
			 FROM releases r
			 LEFT JOIN sources s ON r.source_id = s.id
			 LEFT JOIN projects p ON s.project_id = p.id
			 LEFT JOIN LATERAL (
			     (SELECT sr.id, sr.status, sr.report->>'urgency' AS urgency, 0 AS priority
			      FROM semantic_releases sr
			      WHERE sr.project_id = p.id AND sr.version = r.version
			      ORDER BY sr.created_at DESC LIMIT 1)
			     UNION ALL
			     (SELECT NULL::uuid, 'processing', '', 1
			      FROM agent_runs ar
			      WHERE ar.project_id = p.id AND ar.version = r.version
			        AND ar.status IN ('pending', 'running')
			      LIMIT 1)
			     ORDER BY priority LIMIT 1
			 ) sr_info ON true
			 WHERE r.source_id = $1
			 ORDER BY COALESCE(r.released_at, r.created_at) DESC LIMIT $2 OFFSET $3`, sourceID, perPage, offset)
	} else {
		rows, err = s.pool.Query(ctx,
			`SELECT r.id, r.source_id, r.version, COALESCE(r.raw_data,'{}'), r.released_at, r.created_at,
			        COALESCE(p.id::text,''), COALESCE(p.name,''), COALESCE(s.provider,''), COALESCE(s.repository,''),
			        false,
			        COALESCE(sr_info.id::text,''), COALESCE(sr_info.status,''), COALESCE(sr_info.urgency,'')
			 FROM releases r
			 LEFT JOIN sources s ON r.source_id = s.id
			 LEFT JOIN projects p ON s.project_id = p.id
			 LEFT JOIN LATERAL (
			     (SELECT sr.id, sr.status, sr.report->>'urgency' AS urgency, 0 AS priority
			      FROM semantic_releases sr
			      WHERE sr.project_id = p.id AND sr.version = r.version
			      ORDER BY sr.created_at DESC LIMIT 1)
			     UNION ALL
			     (SELECT NULL::uuid, 'processing', '', 1
			      FROM agent_runs ar
			      WHERE ar.project_id = p.id AND ar.version = r.version
			        AND ar.status IN ('pending', 'running')
			      LIMIT 1)
			     ORDER BY priority LIMIT 1
			 ) sr_info ON true
			 WHERE r.source_id = $1
			   AND (s.version_filter_include IS NULL OR r.version ~ s.version_filter_include)
			   AND (s.version_filter_exclude IS NULL OR r.version !~ s.version_filter_exclude)
			   AND (s.exclude_prereleases = false OR r.raw_data->>'prerelease' IS NULL OR r.raw_data->>'prerelease' != 'true')
			 ORDER BY COALESCE(r.released_at, r.created_at) DESC LIMIT $2 OFFSET $3`, sourceID, perPage, offset)
	}
	if err != nil {
		return nil, 0, fmt.Errorf("list releases by source: %w", err)
	}
	defer rows.Close()
	var releases []models.Release
	for rows.Next() {
		var rel models.Release
		if err := rows.Scan(&rel.ID, &rel.SourceID, &rel.Version, &rel.RawData, &rel.ReleasedAt, &rel.CreatedAt,
			&rel.ProjectID, &rel.ProjectName, &rel.Provider, &rel.Repository, &rel.Excluded,
			&rel.SemanticReleaseID, &rel.SemanticReleaseStatus, &rel.SemanticReleaseUrgency); err != nil {
			return nil, 0, fmt.Errorf("scan release: %w", err)
		}
		releases = append(releases, rel)
	}
	return releases, total, nil
}

func (s *PgStore) ListReleasesByProject(ctx context.Context, projectID string, page, perPage int, includeExcluded bool) ([]models.Release, int, error) {
	var total int
	if includeExcluded {
		err := s.pool.QueryRow(ctx,
			`SELECT COUNT(*) FROM releases r JOIN sources s ON r.source_id = s.id WHERE s.project_id = $1`,
			projectID).Scan(&total)
		if err != nil {
			return nil, 0, fmt.Errorf("count releases: %w", err)
		}
	} else {
		err := s.pool.QueryRow(ctx,
			`SELECT COUNT(*) FROM releases r JOIN sources s ON r.source_id = s.id WHERE s.project_id = $1
			   AND (s.version_filter_include IS NULL OR r.version ~ s.version_filter_include)
			   AND (s.version_filter_exclude IS NULL OR r.version !~ s.version_filter_exclude)
			   AND (s.exclude_prereleases = false OR r.raw_data->>'prerelease' IS NULL OR r.raw_data->>'prerelease' != 'true')`,
			projectID).Scan(&total)
		if err != nil {
			return nil, 0, fmt.Errorf("count releases: %w", err)
		}
	}
	offset := (page - 1) * perPage
	var rows pgx.Rows
	var err error
	if includeExcluded {
		rows, err = s.pool.Query(ctx,
			`SELECT r.id, r.source_id, r.version, COALESCE(r.raw_data,'{}'), r.released_at, r.created_at,
			        p.id, p.name, s.provider, s.repository,
			        CASE WHEN
			          (s.version_filter_include IS NOT NULL AND r.version !~ s.version_filter_include)
			          OR (s.version_filter_exclude IS NOT NULL AND r.version ~ s.version_filter_exclude)
			          OR (s.exclude_prereleases = true AND r.raw_data->>'prerelease' = 'true')
			        THEN true ELSE false END,
			        COALESCE(sr_info.id::text,''), COALESCE(sr_info.status,''), COALESCE(sr_info.urgency,'')
			 FROM releases r
			 JOIN sources s ON r.source_id = s.id
			 JOIN projects p ON s.project_id = p.id
			 LEFT JOIN LATERAL (
			     (SELECT sr.id, sr.status, sr.report->>'urgency' AS urgency, 0 AS priority
			      FROM semantic_releases sr
			      WHERE sr.project_id = p.id AND sr.version = r.version
			      ORDER BY sr.created_at DESC LIMIT 1)
			     UNION ALL
			     (SELECT NULL::uuid, 'processing', '', 1
			      FROM agent_runs ar
			      WHERE ar.project_id = p.id AND ar.version = r.version
			        AND ar.status IN ('pending', 'running')
			      LIMIT 1)
			     ORDER BY priority LIMIT 1
			 ) sr_info ON true
			 WHERE s.project_id = $1
			 ORDER BY COALESCE(r.released_at, r.created_at) DESC LIMIT $2 OFFSET $3`, projectID, perPage, offset)
	} else {
		rows, err = s.pool.Query(ctx,
			`SELECT r.id, r.source_id, r.version, COALESCE(r.raw_data,'{}'), r.released_at, r.created_at,
			        p.id, p.name, s.provider, s.repository,
			        false,
			        COALESCE(sr_info.id::text,''), COALESCE(sr_info.status,''), COALESCE(sr_info.urgency,'')
			 FROM releases r
			 JOIN sources s ON r.source_id = s.id
			 JOIN projects p ON s.project_id = p.id
			 LEFT JOIN LATERAL (
			     (SELECT sr.id, sr.status, sr.report->>'urgency' AS urgency, 0 AS priority
			      FROM semantic_releases sr
			      WHERE sr.project_id = p.id AND sr.version = r.version
			      ORDER BY sr.created_at DESC LIMIT 1)
			     UNION ALL
			     (SELECT NULL::uuid, 'processing', '', 1
			      FROM agent_runs ar
			      WHERE ar.project_id = p.id AND ar.version = r.version
			        AND ar.status IN ('pending', 'running')
			      LIMIT 1)
			     ORDER BY priority LIMIT 1
			 ) sr_info ON true
			 WHERE s.project_id = $1
			   AND (s.version_filter_include IS NULL OR r.version ~ s.version_filter_include)
			   AND (s.version_filter_exclude IS NULL OR r.version !~ s.version_filter_exclude)
			   AND (s.exclude_prereleases = false OR r.raw_data->>'prerelease' IS NULL OR r.raw_data->>'prerelease' != 'true')
			 ORDER BY COALESCE(r.released_at, r.created_at) DESC LIMIT $2 OFFSET $3`, projectID, perPage, offset)
	}
	if err != nil {
		return nil, 0, fmt.Errorf("list releases by project: %w", err)
	}
	defer rows.Close()
	var releases []models.Release
	for rows.Next() {
		var rel models.Release
		if err := rows.Scan(&rel.ID, &rel.SourceID, &rel.Version, &rel.RawData, &rel.ReleasedAt, &rel.CreatedAt,
			&rel.ProjectID, &rel.ProjectName, &rel.Provider, &rel.Repository, &rel.Excluded,
			&rel.SemanticReleaseID, &rel.SemanticReleaseStatus, &rel.SemanticReleaseUrgency); err != nil {
			return nil, 0, fmt.Errorf("scan release: %w", err)
		}
		releases = append(releases, rel)
	}
	return releases, total, nil
}

func (s *PgStore) GetRelease(ctx context.Context, id string) (*models.Release, error) {
	var rel models.Release
	err := s.pool.QueryRow(ctx,
		`SELECT r.id, r.source_id, r.version, COALESCE(r.raw_data,'{}'), r.released_at, r.created_at,
		        COALESCE(p.id::text,''), COALESCE(p.name,''), COALESCE(s.provider,''), COALESCE(s.repository,''),
		        COALESCE(sr_info.id::text,''), COALESCE(sr_info.status,''), COALESCE(sr_info.urgency,'')
		 FROM releases r
		 LEFT JOIN sources s ON r.source_id = s.id
		 LEFT JOIN projects p ON s.project_id = p.id
		 LEFT JOIN LATERAL (
		     (SELECT sr.id, sr.status, sr.report->>'urgency' AS urgency, 0 AS priority
		      FROM semantic_releases sr
		      WHERE sr.project_id = p.id AND sr.version = r.version
		      ORDER BY sr.created_at DESC LIMIT 1)
		     UNION ALL
		     (SELECT NULL::uuid, 'processing', '', 1
		      FROM agent_runs ar
		      WHERE ar.project_id = p.id AND ar.version = r.version
		        AND ar.status IN ('pending', 'running')
		      LIMIT 1)
		     ORDER BY priority LIMIT 1
		 ) sr_info ON true
		 WHERE r.id = $1`, id,
	).Scan(&rel.ID, &rel.SourceID, &rel.Version, &rel.RawData, &rel.ReleasedAt, &rel.CreatedAt,
		&rel.ProjectID, &rel.ProjectName, &rel.Provider, &rel.Repository,
		&rel.SemanticReleaseID, &rel.SemanticReleaseStatus, &rel.SemanticReleaseUrgency)
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

func (s *PgStore) CreateSubscriptionBatch(ctx context.Context, subs []models.Subscription) ([]models.Subscription, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback(ctx)

	var result []models.Subscription
	for i := range subs {
		sub := subs[i]
		err := tx.QueryRow(ctx,
			`INSERT INTO subscriptions (channel_id, type, source_id, project_id, version_filter)
			 VALUES ($1, $2, $3, $4, $5)
			 RETURNING id, created_at`,
			sub.ChannelID, sub.Type, sub.SourceID, sub.ProjectID, sub.VersionFilter,
		).Scan(&sub.ID, &sub.CreatedAt)
		if err != nil {
			return nil, fmt.Errorf("insert subscription %d: %w", i, err)
		}
		result = append(result, sub)
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit tx: %w", err)
	}
	return result, nil
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

func (s *PgStore) DeleteSubscriptionBatch(ctx context.Context, ids []string) error {
	tag, err := s.pool.Exec(ctx, `DELETE FROM subscriptions WHERE id = ANY($1)`, ids)
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

func (s *PgStore) ListAllSemanticReleases(ctx context.Context, page, perPage int) ([]models.SemanticRelease, int, error) {
	var total int
	err := s.pool.QueryRow(ctx, `SELECT COUNT(*) FROM semantic_releases`).Scan(&total)
	if err != nil {
		return nil, 0, fmt.Errorf("count semantic releases: %w", err)
	}
	offset := (page - 1) * perPage
	rows, err := s.pool.Query(ctx,
		`SELECT sr.id, sr.project_id, COALESCE(p.name,''), sr.version, COALESCE(sr.report,'{}'), sr.status, COALESCE(sr.error,''),
		        sr.created_at, sr.completed_at
		 FROM semantic_releases sr
		 LEFT JOIN projects p ON sr.project_id = p.id
		 ORDER BY sr.created_at DESC LIMIT $1 OFFSET $2`, perPage, offset)
	if err != nil {
		return nil, 0, fmt.Errorf("list all semantic releases: %w", err)
	}
	defer rows.Close()
	var releases []models.SemanticRelease
	for rows.Next() {
		var sr models.SemanticRelease
		if err := rows.Scan(&sr.ID, &sr.ProjectID, &sr.ProjectName, &sr.Version, &sr.Report, &sr.Status, &sr.Error,
			&sr.CreatedAt, &sr.CompletedAt); err != nil {
			return nil, 0, fmt.Errorf("scan semantic release: %w", err)
		}
		releases = append(releases, sr)
	}
	return releases, total, nil
}

func (s *PgStore) ListSemanticReleases(ctx context.Context, projectID string, page, perPage int) ([]models.SemanticRelease, int, error) {
	var total int
	err := s.pool.QueryRow(ctx, `SELECT COUNT(*) FROM semantic_releases WHERE project_id = $1`, projectID).Scan(&total)
	if err != nil {
		return nil, 0, fmt.Errorf("count semantic releases: %w", err)
	}
	offset := (page - 1) * perPage
	rows, err := s.pool.Query(ctx,
		`SELECT sr.id, sr.project_id, COALESCE(p.name,''), sr.version, COALESCE(sr.report,'{}'), sr.status, COALESCE(sr.error,''),
		        sr.created_at, sr.completed_at
		 FROM semantic_releases sr
		 LEFT JOIN projects p ON sr.project_id = p.id
		 WHERE sr.project_id = $1 ORDER BY sr.created_at DESC LIMIT $2 OFFSET $3`, projectID, perPage, offset)
	if err != nil {
		return nil, 0, fmt.Errorf("list semantic releases: %w", err)
	}
	defer rows.Close()
	var releases []models.SemanticRelease
	for rows.Next() {
		var sr models.SemanticRelease
		if err := rows.Scan(&sr.ID, &sr.ProjectID, &sr.ProjectName, &sr.Version, &sr.Report, &sr.Status, &sr.Error,
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
		`SELECT sr.id, sr.project_id, COALESCE(p.name,''), sr.version, COALESCE(sr.report,'{}'), sr.status, COALESCE(sr.error,''),
		        sr.created_at, sr.completed_at
		 FROM semantic_releases sr
		 LEFT JOIN projects p ON sr.project_id = p.id
		 WHERE sr.id = $1`, id,
	).Scan(&sr.ID, &sr.ProjectID, &sr.ProjectName, &sr.Version, &sr.Report, &sr.Status, &sr.Error,
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

func (s *PgStore) DeleteSemanticRelease(ctx context.Context, id string) error {
	tag, err := s.pool.Exec(ctx, `DELETE FROM semantic_releases WHERE id = $1`, id)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("not found")
	}
	return nil
}

// --- AgentStore ---

func (s *PgStore) TriggerAgentRun(ctx context.Context, projectID, trigger, version string) (*models.AgentRun, error) {
	var run models.AgentRun
	err := s.pool.QueryRow(ctx,
		`INSERT INTO agent_runs (project_id, trigger, version, status)
		 VALUES ($1, $2, $3, 'pending')
		 RETURNING id, project_id, trigger, COALESCE(version,''), status, created_at`,
		projectID, trigger, nilIfEmpty(version),
	).Scan(&run.ID, &run.ProjectID, &run.Trigger, &run.Version, &run.Status, &run.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("insert agent run: %w", err)
	}

	// Enqueue a River job to process the agent run, if river client is available.
	if s.river != nil {
		_, err = s.river.Insert(ctx, queue.AgentJobArgs{
			AgentRunID: run.ID,
			ProjectID:  projectID,
			Version:    version,
		}, nil)
		if err != nil {
			// If the agent worker isn't registered (missing LLM API key),
			// mark the run as failed rather than returning an error.
			if strings.Contains(err.Error(), "not registered") {
				_, _ = s.pool.Exec(ctx,
					`UPDATE agent_runs SET status = 'failed', error = $2, completed_at = NOW() WHERE id = $1`,
					run.ID, "Agent worker not available — configure GOOGLE_API_KEY or OPENAI_API_KEY")
				run.Status = "failed"
				run.Error = "Agent worker not available — configure GOOGLE_API_KEY or OPENAI_API_KEY"
				return &run, nil
			}
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
		`SELECT id, project_id, semantic_release_id, trigger, COALESCE(version,''), status,
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
		if err := rows.Scan(&run.ID, &run.ProjectID, &run.SemanticReleaseID, &run.Trigger, &run.Version, &run.Status,
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
		`SELECT id, project_id, semantic_release_id, trigger, COALESCE(version,''), status,
		        COALESCE(prompt_used,''), COALESCE(error,''),
		        started_at, completed_at, created_at
		 FROM agent_runs WHERE id = $1`, id,
	).Scan(&run.ID, &run.ProjectID, &run.SemanticReleaseID, &run.Trigger, &run.Version, &run.Status,
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
		 WHERE type = 'source_release' AND source_id = $1`, sourceID)
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
func (s *PgStore) EnqueueAgentRun(ctx context.Context, projectID, trigger, version string) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback(ctx)

	var runID string
	err = tx.QueryRow(ctx,
		`INSERT INTO agent_runs (project_id, trigger, version, status)
		 VALUES ($1, $2, $3, 'pending')
		 RETURNING id`, projectID, trigger, version,
	).Scan(&runID)
	if err != nil {
		return fmt.Errorf("insert agent run: %w", err)
	}

	if s.river != nil {
		_, err = s.river.InsertTx(ctx, tx, queue.AgentJobArgs{
			AgentRunID: runID,
			ProjectID:  projectID,
			Version:    version,
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
		 WHERE type = 'semantic_release' AND project_id = $1`, projectID)
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

// HasReleaseForVersion checks if a source has a release matching the given version.
func (s *PgStore) HasReleaseForVersion(ctx context.Context, sourceID, version string) (bool, error) {
	var exists bool
	err := s.pool.QueryRow(ctx,
		`SELECT EXISTS(SELECT 1 FROM releases WHERE source_id = $1 AND version = $2)`,
		sourceID, version,
	).Scan(&exists)
	if err != nil {
		return false, fmt.Errorf("check release for version: %w", err)
	}
	return exists, nil
}

func nilIfEmpty(s string) *string {
	if s == "" {
		return nil
	}
	return &s
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
	if err := s.pool.QueryRow(ctx, `SELECT COUNT(*) FROM releases WHERE COALESCE(released_at, created_at) >= NOW() - INTERVAL '7 days'`).Scan(&stats.ReleasesThisWeek); err != nil {
		return nil, fmt.Errorf("count releases this week: %w", err)
	}
	if err := s.pool.QueryRow(ctx, `SELECT COUNT(*) FROM semantic_releases WHERE status = 'completed' AND report->>'urgency' IN ('critical', 'high', 'CRITICAL', 'HIGH')`).Scan(&stats.AttentionNeeded); err != nil {
		return nil, fmt.Errorf("count attention needed: %w", err)
	}
	return &stats, nil
}

func (s *PgStore) GetTrend(ctx context.Context, granularity string, start, end time.Time) ([]TrendBucket, error) {
	// Validate granularity to one of three literal values (no injection risk).
	var trunc string
	switch granularity {
	case "daily":
		trunc = "day"
	case "weekly":
		trunc = "week"
	case "monthly":
		trunc = "month"
	default:
		return nil, fmt.Errorf("invalid granularity: %s", granularity)
	}

	query := fmt.Sprintf(`
		WITH buckets AS (
			SELECT date_trunc('%s', gs)::date AS period
			FROM generate_series($1::timestamptz, $2::timestamptz, '1 %s'::interval) gs
		)
		SELECT
			b.period,
			COALESCE(r.cnt, 0) AS releases,
			COALESCE(sr.cnt, 0) AS semantic_releases
		FROM buckets b
		LEFT JOIN (
			SELECT date_trunc('%s', COALESCE(released_at, created_at))::date AS period,
			       COUNT(*) AS cnt
			FROM releases
			WHERE COALESCE(released_at, created_at) >= $1 AND COALESCE(released_at, created_at) < $2
			GROUP BY 1
		) r ON r.period = b.period
		LEFT JOIN (
			SELECT date_trunc('%s', created_at)::date AS period,
			       COUNT(*) AS cnt
			FROM semantic_releases
			WHERE created_at >= $1 AND created_at < $2
			GROUP BY 1
		) sr ON sr.period = b.period
		ORDER BY b.period
	`, trunc, trunc, trunc, trunc)

	rows, err := s.pool.Query(ctx, query, start, end)
	if err != nil {
		return nil, fmt.Errorf("get trend: %w", err)
	}
	defer rows.Close()

	var buckets []TrendBucket
	for rows.Next() {
		var b TrendBucket
		var period time.Time
		if err := rows.Scan(&period, &b.Releases, &b.SemanticReleases); err != nil {
			return nil, fmt.Errorf("scan trend bucket: %w", err)
		}
		b.Period = period.Format("2006-01-02")
		buckets = append(buckets, b)
	}
	if buckets == nil {
		buckets = []TrendBucket{}
	}
	return buckets, nil
}

// --- TodosStore ---

func (s *PgStore) ListTodos(ctx context.Context, status string, page, perPage int, aggregated bool) ([]models.Todo, int, error) {
	// Base WHERE clause
	whereClause := ""
	countArgs := []any{}
	if status != "" {
		whereClause = ` WHERE t.status = $1`
		countArgs = append(countArgs, status)
	}

	// Shared FROM/JOIN clause.
	fromClause := `
		FROM release_todos t
		LEFT JOIN releases r ON r.id = t.release_id
		LEFT JOIN sources src ON src.id = r.source_id
		LEFT JOIN projects p1 ON p1.id = src.project_id
		LEFT JOIN semantic_releases sr ON sr.id = t.semantic_release_id
		LEFT JOIN projects p2 ON p2.id = sr.project_id
	`

	// Shared select columns with aliases for subquery use.
	selectCols := `
			t.id, t.release_id, t.semantic_release_id, t.status,
			t.created_at, t.acknowledged_at, t.resolved_at,
			COALESCE(p1.id, p2.id, gen_random_uuid())::text AS project_id,
			COALESCE(p1.name, p2.name, '') AS project_name,
			COALESCE(r.version, sr.version, '') AS version,
			COALESCE(src.provider, '') AS provider,
			COALESCE(src.repository, '') AS repository,
			CASE WHEN t.release_id IS NOT NULL THEN 'release' ELSE 'semantic' END AS todo_type`

	var countQuery string
	var query string
	offset := (page - 1) * perPage

	if aggregated {
		// Aggregated: keep only the latest todo per grouping key.
		// Group by (source_id) for release todos, (project_id) for semantic todos.
		partitionExpr := `CASE WHEN t.release_id IS NOT NULL THEN r.source_id::text ELSE sr.project_id::text END`

		countQuery = `
			SELECT COUNT(*) FROM (
				SELECT t.id,
					ROW_NUMBER() OVER (PARTITION BY ` + partitionExpr + ` ORDER BY t.created_at DESC) AS rn
				` + fromClause + whereClause + `
			) sub WHERE sub.rn = 1`

		query = `
			SELECT id, release_id, semantic_release_id, status,
				created_at, acknowledged_at, resolved_at,
				project_id, project_name, version, provider, repository, todo_type
			FROM (
				SELECT ` + selectCols + `,
					ROW_NUMBER() OVER (PARTITION BY ` + partitionExpr + ` ORDER BY t.created_at DESC) AS rn
				` + fromClause + whereClause + `
			) sub WHERE sub.rn = 1
			ORDER BY created_at DESC`
	} else {
		countQuery = `SELECT COUNT(*) FROM release_todos t` + whereClause
		query = `SELECT ` + selectCols + fromClause + whereClause + ` ORDER BY t.created_at DESC`
	}

	// Execute count query
	var total int
	if err := s.pool.QueryRow(ctx, countQuery, countArgs...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count todos: %w", err)
	}

	// Add pagination
	queryArgs := append([]any{}, countArgs...)
	argIdx := len(queryArgs) + 1
	query += fmt.Sprintf(` LIMIT $%d OFFSET $%d`, argIdx, argIdx+1)
	queryArgs = append(queryArgs, perPage, offset)

	rows, err := s.pool.Query(ctx, query, queryArgs...)
	if err != nil {
		return nil, 0, fmt.Errorf("list todos: %w", err)
	}
	defer rows.Close()

	var todos []models.Todo
	for rows.Next() {
		var t models.Todo
		if err := rows.Scan(
			&t.ID, &t.ReleaseID, &t.SemanticReleaseID, &t.Status,
			&t.CreatedAt, &t.AcknowledgedAt, &t.ResolvedAt,
			&t.ProjectID, &t.ProjectName, &t.Version, &t.Provider, &t.Repository, &t.TodoType,
		); err != nil {
			return nil, 0, fmt.Errorf("scan todo: %w", err)
		}
		todos = append(todos, t)
	}
	return todos, total, nil
}

func (s *PgStore) GetTodo(ctx context.Context, id string) (*models.Todo, error) {
	var t models.Todo
	err := s.pool.QueryRow(ctx, `
		SELECT
			t.id, t.release_id, t.semantic_release_id, t.status,
			t.created_at, t.acknowledged_at, t.resolved_at,
			COALESCE(p1.id, p2.id, gen_random_uuid())::text,
			COALESCE(p1.name, p2.name, ''),
			COALESCE(r.version, sr.version, ''),
			COALESCE(src.provider, ''),
			COALESCE(src.repository, ''),
			CASE WHEN t.release_id IS NOT NULL THEN 'release' ELSE 'semantic' END
		FROM release_todos t
		LEFT JOIN releases r ON r.id = t.release_id
		LEFT JOIN sources src ON src.id = r.source_id
		LEFT JOIN projects p1 ON p1.id = src.project_id
		LEFT JOIN semantic_releases sr ON sr.id = t.semantic_release_id
		LEFT JOIN projects p2 ON p2.id = sr.project_id
		WHERE t.id = $1
	`, id).Scan(
		&t.ID, &t.ReleaseID, &t.SemanticReleaseID, &t.Status,
		&t.CreatedAt, &t.AcknowledgedAt, &t.ResolvedAt,
		&t.ProjectID, &t.ProjectName, &t.Version, &t.Provider, &t.Repository, &t.TodoType,
	)
	if err != nil {
		return nil, fmt.Errorf("get todo: %w", err)
	}
	return &t, nil
}

func (s *PgStore) AcknowledgeTodo(ctx context.Context, id string, cascade bool) error {
	tag, err := s.pool.Exec(ctx,
		`UPDATE release_todos SET status = 'acknowledged', acknowledged_at = NOW() WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("acknowledge todo: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("todo not found")
	}

	if cascade {
		// Also acknowledge older pending todos for the same source/project.
		_, _ = s.pool.Exec(ctx, `
			UPDATE release_todos SET status = 'acknowledged', acknowledged_at = NOW()
			WHERE id != $1 AND status = 'pending'
			AND (
				(release_id IS NOT NULL AND release_id IN (
					SELECT r2.id FROM releases r2
					JOIN releases r1 ON r1.source_id = r2.source_id
					JOIN release_todos t1 ON t1.release_id = r1.id
					WHERE t1.id = $1 AND r2.created_at <= r1.created_at AND r2.id != r1.id
				))
				OR
				(semantic_release_id IS NOT NULL AND semantic_release_id IN (
					SELECT sr2.id FROM semantic_releases sr2
					JOIN semantic_releases sr1 ON sr1.project_id = sr2.project_id
					JOIN release_todos t1 ON t1.semantic_release_id = sr1.id
					WHERE t1.id = $1 AND sr2.created_at <= sr1.created_at AND sr2.id != sr1.id
				))
			)`, id)
	}

	return nil
}

func (s *PgStore) ResolveTodo(ctx context.Context, id string, cascade bool) error {
	tag, err := s.pool.Exec(ctx,
		`UPDATE release_todos SET status = 'resolved', resolved_at = NOW() WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("resolve todo: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("todo not found")
	}

	if cascade {
		// Also resolve older pending/acknowledged todos for the same source/project.
		_, _ = s.pool.Exec(ctx, `
			UPDATE release_todos SET status = 'resolved', resolved_at = NOW()
			WHERE id != $1 AND status IN ('pending', 'acknowledged')
			AND (
				(release_id IS NOT NULL AND release_id IN (
					SELECT r2.id FROM releases r2
					JOIN releases r1 ON r1.source_id = r2.source_id
					JOIN release_todos t1 ON t1.release_id = r1.id
					WHERE t1.id = $1 AND r2.created_at <= r1.created_at AND r2.id != r1.id
				))
				OR
				(semantic_release_id IS NOT NULL AND semantic_release_id IN (
					SELECT sr2.id FROM semantic_releases sr2
					JOIN semantic_releases sr1 ON sr1.project_id = sr2.project_id
					JOIN release_todos t1 ON t1.semantic_release_id = sr1.id
					WHERE t1.id = $1 AND sr2.created_at <= sr1.created_at AND sr2.id != sr1.id
				))
			)`, id)
	}

	return nil
}

func (s *PgStore) ReopenTodo(ctx context.Context, id string) error {
	tag, err := s.pool.Exec(ctx,
		`UPDATE release_todos SET status = 'pending', acknowledged_at = NULL, resolved_at = NULL WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("reopen todo: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("todo not found")
	}
	return nil
}

// CreateReleaseTodo inserts a TODO for a source release. Returns the todo ID.
// Uses ON CONFLICT DO UPDATE for idempotency.
func (s *PgStore) CreateReleaseTodo(ctx context.Context, releaseID string) (string, error) {
	var id string
	err := s.pool.QueryRow(ctx,
		`INSERT INTO release_todos (release_id) VALUES ($1)
		 ON CONFLICT (release_id) WHERE release_id IS NOT NULL DO UPDATE SET release_id = EXCLUDED.release_id
		 RETURNING id`, releaseID).Scan(&id)
	if err != nil {
		return "", fmt.Errorf("create release todo: %w", err)
	}
	return id, nil
}

// CreateSemanticReleaseTodo inserts a TODO for a semantic release. Returns the todo ID.
// Uses ON CONFLICT DO UPDATE for idempotency.
func (s *PgStore) CreateSemanticReleaseTodo(ctx context.Context, semanticReleaseID string) (string, error) {
	var id string
	err := s.pool.QueryRow(ctx,
		`INSERT INTO release_todos (semantic_release_id) VALUES ($1)
		 ON CONFLICT (semantic_release_id) WHERE semantic_release_id IS NOT NULL DO UPDATE SET semantic_release_id = EXCLUDED.semantic_release_id
		 RETURNING id`, semanticReleaseID).Scan(&id)
	if err != nil {
		return "", fmt.Errorf("create semantic release todo: %w", err)
	}
	return id, nil
}

// --- Onboard Store ---

func (s *PgStore) CreateOnboardScan(ctx context.Context, repoURL string) (*models.OnboardScan, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback(ctx)

	var scan models.OnboardScan
	err = tx.QueryRow(ctx,
		`INSERT INTO onboard_scans (repo_url) VALUES ($1)
		 RETURNING id, repo_url, status, created_at`,
		repoURL,
	).Scan(&scan.ID, &scan.RepoURL, &scan.Status, &scan.CreatedAt)
	if err != nil {
		return nil, err
	}

	// Enqueue River job in the same transaction
	if s.river != nil {
		_, err = s.river.InsertTx(ctx, tx, queue.ScanDependenciesJobArgs{ScanID: scan.ID}, nil)
		if err != nil {
			return nil, fmt.Errorf("enqueue scan job: %w", err)
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit: %w", err)
	}
	return &scan, nil
}

func (s *PgStore) GetOnboardScan(ctx context.Context, id string) (*models.OnboardScan, error) {
	var scan models.OnboardScan
	err := s.pool.QueryRow(ctx,
		`SELECT id, repo_url, status, results, COALESCE(error, ''), created_at, started_at, completed_at
		 FROM onboard_scans WHERE id = $1`, id,
	).Scan(&scan.ID, &scan.RepoURL, &scan.Status, &scan.Results, &scan.Error,
		&scan.CreatedAt, &scan.StartedAt, &scan.CompletedAt)
	if err != nil {
		return nil, err
	}
	return &scan, nil
}

func (s *PgStore) UpdateOnboardScanStatus(ctx context.Context, id, status string, results json.RawMessage, scanErr string) error {
	_, err := s.pool.Exec(ctx,
		`UPDATE onboard_scans SET status = $2::text, results = $3, error = NULLIF($4, ''),
		 started_at = CASE WHEN $2::text = 'processing' AND started_at IS NULL THEN NOW() ELSE started_at END,
		 completed_at = CASE WHEN $2::text IN ('completed', 'failed') THEN NOW() ELSE completed_at END
		 WHERE id = $1`, id, status, results, scanErr,
	)
	return err
}

func (s *PgStore) ActiveScanForRepo(ctx context.Context, repoURL string) (*models.OnboardScan, error) {
	var scan models.OnboardScan
	err := s.pool.QueryRow(ctx,
		`SELECT id, repo_url, status, created_at FROM onboard_scans
		 WHERE repo_url = $1 AND status IN ('pending', 'processing')
		 ORDER BY created_at DESC LIMIT 1`, repoURL,
	).Scan(&scan.ID, &scan.RepoURL, &scan.Status, &scan.CreatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return &scan, nil
}

func (s *PgStore) ApplyOnboardScan(ctx context.Context, scanID string, selections []OnboardSelection) (*OnboardApplyResult, error) {
	result := &OnboardApplyResult{
		CreatedProjects: []models.Project{},
		CreatedSources:  []models.Source{},
		Skipped:         []string{},
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback(ctx)

	for _, sel := range selections {
		projectID := sel.ProjectID

		// Create project if needed
		if sel.NewProjectName != "" {
			var p models.Project
			err := tx.QueryRow(ctx,
				`INSERT INTO projects (name, agent_rules) VALUES ($1, '{}')
				 RETURNING id, name, COALESCE(description, ''), COALESCE(agent_prompt, ''), agent_rules, created_at, updated_at`,
				sel.NewProjectName,
			).Scan(&p.ID, &p.Name, &p.Description, &p.AgentPrompt, &p.AgentRules, &p.CreatedAt, &p.UpdatedAt)
			if err != nil {
				return nil, fmt.Errorf("create project %q: %w", sel.NewProjectName, err)
			}
			projectID = p.ID
			result.CreatedProjects = append(result.CreatedProjects, p)
		}

		// Create source — skip if duplicate
		var src models.Source
		err := tx.QueryRow(ctx,
			`INSERT INTO sources (project_id, provider, repository)
			 VALUES ($1, $2, $3)
			 ON CONFLICT (provider, repository) DO NOTHING
			 RETURNING id, project_id, provider, repository, poll_interval_seconds, enabled, created_at, updated_at`,
			projectID, sel.Provider, sel.UpstreamRepo,
		).Scan(&src.ID, &src.ProjectID, &src.Provider, &src.Repository,
			&src.PollIntervalSeconds, &src.Enabled, &src.CreatedAt, &src.UpdatedAt)
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				result.Skipped = append(result.Skipped, fmt.Sprintf("%s/%s: source already exists", sel.Provider, sel.UpstreamRepo))
				continue
			}
			return nil, fmt.Errorf("create source %s/%s: %w", sel.Provider, sel.UpstreamRepo, err)
		}
		result.CreatedSources = append(result.CreatedSources, src)
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit: %w", err)
	}
	return result, nil
}
