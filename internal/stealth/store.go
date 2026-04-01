package stealth

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/google/uuid"
	"github.com/sentioxyz/changelogue/internal/ingestion"
	"github.com/sentioxyz/changelogue/internal/models"
	_ "modernc.org/sqlite"
)

// Store is a SQLite-backed store for stealth mode.
type Store struct {
	db *sql.DB
}

// NewStore opens (or creates) the SQLite database at dbPath and runs migrations.
func NewStore(dbPath string) (*Store, error) {
	dir := filepath.Dir(dbPath)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, fmt.Errorf("create db directory: %w", err)
	}

	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}

	// Set pragmas
	for _, pragma := range []string{
		"PRAGMA journal_mode=WAL",
		"PRAGMA busy_timeout=5000",
		"PRAGMA foreign_keys=ON",
	} {
		if _, err := db.Exec(pragma); err != nil {
			db.Close()
			return nil, fmt.Errorf("set %s: %w", pragma, err)
		}
	}

	// Run migrations
	if _, err := db.Exec(schema); err != nil {
		db.Close()
		return nil, fmt.Errorf("run migrations: %w", err)
	}

	return &Store{db: db}, nil
}

// Close closes the database connection.
func (s *Store) Close() error {
	return s.db.Close()
}

// PingDB checks that the database is reachable.
func (s *Store) PingDB(ctx context.Context) error {
	return s.db.PingContext(ctx)
}

// parseTime parses a TEXT timestamp stored in SQLite (ISO 8601 / RFC3339Nano).
func parseTime(s string) (time.Time, error) {
	t, err := time.Parse(time.RFC3339Nano, s)
	if err != nil {
		// Fallback: SQLite strftime produces milliseconds like "2006-01-02T15:04:05.000Z"
		t, err = time.Parse("2006-01-02T15:04:05.999Z07:00", s)
	}
	return t, err
}

// ─────────────────────────────────────────
// ProjectsStore implementation
// ─────────────────────────────────────────

// ListProjects returns a paginated list of projects and the total count.
func (s *Store) ListProjects(ctx context.Context, page, perPage int) ([]models.Project, int, error) {
	var total int
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM projects`).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count projects: %w", err)
	}

	offset := (page - 1) * perPage
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, name,
		       COALESCE(description,''), COALESCE(agent_prompt,''), COALESCE(agent_rules,'{}'),
		       created_at, updated_at
		FROM projects
		ORDER BY created_at ASC
		LIMIT ? OFFSET ?`, perPage, offset)
	if err != nil {
		return nil, 0, fmt.Errorf("list projects: %w", err)
	}
	defer rows.Close()

	var projects []models.Project
	for rows.Next() {
		var p models.Project
		var createdAt, updatedAt, agentRulesStr string
		if err := rows.Scan(&p.ID, &p.Name, &p.Description, &p.AgentPrompt, &agentRulesStr, &createdAt, &updatedAt); err != nil {
			return nil, 0, fmt.Errorf("scan project: %w", err)
		}
		p.AgentRules = json.RawMessage(agentRulesStr)
		if p.CreatedAt, err = parseTime(createdAt); err != nil {
			return nil, 0, fmt.Errorf("parse created_at: %w", err)
		}
		if p.UpdatedAt, err = parseTime(updatedAt); err != nil {
			return nil, 0, fmt.Errorf("parse updated_at: %w", err)
		}
		projects = append(projects, p)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, err
	}
	return projects, total, nil
}

// CreateProject inserts a new project, generating its ID and timestamps.
func (s *Store) CreateProject(ctx context.Context, p *models.Project) error {
	p.ID = uuid.New().String()
	now := time.Now().UTC()
	p.CreatedAt = now
	p.UpdatedAt = now

	agentRules := "{}"
	if len(p.AgentRules) > 0 {
		agentRules = string(p.AgentRules)
	}

	_, err := s.db.ExecContext(ctx, `
		INSERT INTO projects (id, name, description, agent_prompt, agent_rules, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)`,
		p.ID, p.Name, p.Description, p.AgentPrompt, agentRules,
		now.Format(time.RFC3339Nano), now.Format(time.RFC3339Nano),
	)
	if err != nil {
		return fmt.Errorf("create project: %w", err)
	}
	return nil
}

// GetProject retrieves a single project by ID. Returns an error if not found.
func (s *Store) GetProject(ctx context.Context, id string) (*models.Project, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT id, name,
		       COALESCE(description,''), COALESCE(agent_prompt,''), COALESCE(agent_rules,'{}'),
		       created_at, updated_at
		FROM projects WHERE id = ?`, id)

	var p models.Project
	var createdAt, updatedAt, agentRulesStr string
	if err := row.Scan(&p.ID, &p.Name, &p.Description, &p.AgentPrompt, &agentRulesStr, &createdAt, &updatedAt); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("project not found: %s", id)
		}
		return nil, fmt.Errorf("get project: %w", err)
	}
	p.AgentRules = json.RawMessage(agentRulesStr)
	var err error
	if p.CreatedAt, err = parseTime(createdAt); err != nil {
		return nil, fmt.Errorf("parse created_at: %w", err)
	}
	if p.UpdatedAt, err = parseTime(updatedAt); err != nil {
		return nil, fmt.Errorf("parse updated_at: %w", err)
	}
	return &p, nil
}

// UpdateProject updates the mutable fields of an existing project.
func (s *Store) UpdateProject(ctx context.Context, id string, p *models.Project) error {
	now := time.Now().UTC()
	p.UpdatedAt = now

	agentRules := "{}"
	if len(p.AgentRules) > 0 {
		agentRules = string(p.AgentRules)
	}

	result, err := s.db.ExecContext(ctx, `
		UPDATE projects
		SET name = ?, description = ?, agent_prompt = ?, agent_rules = ?, updated_at = ?
		WHERE id = ?`,
		p.Name, p.Description, p.AgentPrompt, agentRules, now.Format(time.RFC3339Nano), id,
	)
	if err != nil {
		return fmt.Errorf("update project: %w", err)
	}
	n, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return fmt.Errorf("project not found: %s", id)
	}
	return nil
}

// DeleteProject removes a project by ID.
func (s *Store) DeleteProject(ctx context.Context, id string) error {
	result, err := s.db.ExecContext(ctx, `DELETE FROM projects WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("delete project: %w", err)
	}
	n, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return fmt.Errorf("project not found: %s", id)
	}
	return nil
}

// ─────────────────────────────────────────
// SourcesStore + SourceLister + PollStatusUpdater implementation
// ─────────────────────────────────────────

// scanSource scans a row into a models.Source.
// Column order: id, project_id, provider, repository, poll_interval_seconds,
//               enabled, config, version_filter_include, version_filter_exclude,
//               exclude_prereleases, last_polled_at, last_error, created_at, updated_at
func scanSource(row interface {
	Scan(dest ...any) error
}) (models.Source, error) {
	var src models.Source
	var enabledInt, excludePrereleasesInt int
	var configNull, vfiNull, vfeNull, lastPolledAtNull, lastErrorNull sql.NullString
	var createdAt, updatedAt string

	if err := row.Scan(
		&src.ID, &src.ProjectID, &src.Provider, &src.Repository,
		&src.PollIntervalSeconds, &enabledInt,
		&configNull, &vfiNull, &vfeNull,
		&excludePrereleasesInt,
		&lastPolledAtNull, &lastErrorNull,
		&createdAt, &updatedAt,
	); err != nil {
		return src, err
	}

	src.Enabled = enabledInt != 0
	src.ExcludePrereleases = excludePrereleasesInt != 0

	if configNull.Valid && configNull.String != "" {
		src.Config = json.RawMessage(configNull.String)
	}
	if vfiNull.Valid {
		v := vfiNull.String
		src.VersionFilterInclude = &v
	}
	if vfeNull.Valid {
		v := vfeNull.String
		src.VersionFilterExclude = &v
	}
	if lastPolledAtNull.Valid && lastPolledAtNull.String != "" {
		t, err := parseTime(lastPolledAtNull.String)
		if err != nil {
			return src, fmt.Errorf("parse last_polled_at: %w", err)
		}
		src.LastPolledAt = &t
	}
	if lastErrorNull.Valid {
		e := lastErrorNull.String
		src.LastError = &e
	}

	var err error
	if src.CreatedAt, err = parseTime(createdAt); err != nil {
		return src, fmt.Errorf("parse created_at: %w", err)
	}
	if src.UpdatedAt, err = parseTime(updatedAt); err != nil {
		return src, fmt.Errorf("parse updated_at: %w", err)
	}
	return src, nil
}

const sourceSelectCols = ` id, project_id, provider, repository, poll_interval_seconds,
	enabled, config, version_filter_include, version_filter_exclude,
	exclude_prereleases, last_polled_at, last_error, created_at, updated_at `

// ListSourcesByProject returns a paginated list of sources for a project.
func (s *Store) ListSourcesByProject(ctx context.Context, projectID string, page, perPage int) ([]models.Source, int, error) {
	var total int
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM sources WHERE project_id = ?`, projectID).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count sources: %w", err)
	}

	offset := (page - 1) * perPage
	rows, err := s.db.QueryContext(ctx, `SELECT`+sourceSelectCols+`FROM sources WHERE project_id = ?
		ORDER BY created_at ASC
		LIMIT ? OFFSET ?`, projectID, perPage, offset)
	if err != nil {
		return nil, 0, fmt.Errorf("list sources: %w", err)
	}
	defer rows.Close()

	var sources []models.Source
	for rows.Next() {
		src, err := scanSource(rows)
		if err != nil {
			return nil, 0, fmt.Errorf("scan source: %w", err)
		}
		sources = append(sources, src)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, err
	}
	return sources, total, nil
}

// CreateSource inserts a new source, generating its ID and timestamps.
func (s *Store) CreateSource(ctx context.Context, src *models.Source) error {
	src.ID = uuid.New().String()
	now := time.Now().UTC()
	src.CreatedAt = now
	src.UpdatedAt = now

	enabledInt := 0
	if src.Enabled {
		enabledInt = 1
	}
	excludePrereleasesInt := 0
	if src.ExcludePrereleases {
		excludePrereleasesInt = 1
	}

	var configStr sql.NullString
	if len(src.Config) > 0 {
		configStr = sql.NullString{String: string(src.Config), Valid: true}
	}

	var vfi, vfe sql.NullString
	if src.VersionFilterInclude != nil {
		vfi = sql.NullString{String: *src.VersionFilterInclude, Valid: true}
	}
	if src.VersionFilterExclude != nil {
		vfe = sql.NullString{String: *src.VersionFilterExclude, Valid: true}
	}

	_, err := s.db.ExecContext(ctx, `
		INSERT INTO sources
		  (id, project_id, provider, repository, poll_interval_seconds,
		   enabled, config, version_filter_include, version_filter_exclude,
		   exclude_prereleases, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		src.ID, src.ProjectID, src.Provider, src.Repository, src.PollIntervalSeconds,
		enabledInt, configStr, vfi, vfe,
		excludePrereleasesInt,
		now.Format(time.RFC3339Nano), now.Format(time.RFC3339Nano),
	)
	if err != nil {
		return fmt.Errorf("create source: %w", err)
	}
	return nil
}

// GetSource retrieves a single source by ID.
func (s *Store) GetSource(ctx context.Context, id string) (*models.Source, error) {
	row := s.db.QueryRowContext(ctx, `SELECT`+sourceSelectCols+`FROM sources WHERE id = ?`, id)
	src, err := scanSource(row)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("source not found: %s", id)
		}
		return nil, fmt.Errorf("get source: %w", err)
	}
	return &src, nil
}

// UpdateSource updates the mutable fields of an existing source.
func (s *Store) UpdateSource(ctx context.Context, id string, src *models.Source) error {
	now := time.Now().UTC()
	src.UpdatedAt = now

	enabledInt := 0
	if src.Enabled {
		enabledInt = 1
	}
	excludePrereleasesInt := 0
	if src.ExcludePrereleases {
		excludePrereleasesInt = 1
	}

	var configStr sql.NullString
	if len(src.Config) > 0 {
		configStr = sql.NullString{String: string(src.Config), Valid: true}
	}

	var vfi, vfe sql.NullString
	if src.VersionFilterInclude != nil {
		vfi = sql.NullString{String: *src.VersionFilterInclude, Valid: true}
	}
	if src.VersionFilterExclude != nil {
		vfe = sql.NullString{String: *src.VersionFilterExclude, Valid: true}
	}

	result, err := s.db.ExecContext(ctx, `
		UPDATE sources
		SET provider = ?, repository = ?, poll_interval_seconds = ?,
		    enabled = ?, config = ?,
		    version_filter_include = ?, version_filter_exclude = ?,
		    exclude_prereleases = ?, updated_at = ?
		WHERE id = ?`,
		src.Provider, src.Repository, src.PollIntervalSeconds,
		enabledInt, configStr,
		vfi, vfe,
		excludePrereleasesInt, now.Format(time.RFC3339Nano),
		id,
	)
	if err != nil {
		return fmt.Errorf("update source: %w", err)
	}
	n, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return fmt.Errorf("source not found: %s", id)
	}
	return nil
}

// DeleteSource removes a source by ID.
func (s *Store) DeleteSource(ctx context.Context, id string) error {
	result, err := s.db.ExecContext(ctx, `DELETE FROM sources WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("delete source: %w", err)
	}
	n, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return fmt.Errorf("source not found: %s", id)
	}
	return nil
}

// UpdateSourcePollStatus updates last_polled_at to now and sets (or clears) last_error.
func (s *Store) UpdateSourcePollStatus(ctx context.Context, id string, pollErr error) error {
	now := time.Now().UTC()
	var lastError sql.NullString
	if pollErr != nil {
		lastError = sql.NullString{String: pollErr.Error(), Valid: true}
	}
	_, err := s.db.ExecContext(ctx, `
		UPDATE sources SET last_polled_at = ?, last_error = ?, updated_at = ?
		WHERE id = ?`,
		now.Format(time.RFC3339Nano), lastError, now.Format(time.RFC3339Nano), id,
	)
	if err != nil {
		return fmt.Errorf("update source poll status: %w", err)
	}
	return nil
}

// ListAllSourceRepos returns the distinct (provider, repository) pairs across all sources.
func (s *Store) ListAllSourceRepos(ctx context.Context) ([]models.SourceRepo, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT DISTINCT provider, repository FROM sources ORDER BY provider, repository`)
	if err != nil {
		return nil, fmt.Errorf("list source repos: %w", err)
	}
	defer rows.Close()

	var repos []models.SourceRepo
	for rows.Next() {
		var r models.SourceRepo
		if err := rows.Scan(&r.Provider, &r.Repository); err != nil {
			return nil, fmt.Errorf("scan source repo: %w", err)
		}
		repos = append(repos, r)
	}
	return repos, rows.Err()
}

// ListEnabledSources implements ingestion.SourceLister.
func (s *Store) ListEnabledSources(ctx context.Context) ([]ingestion.EnabledSource, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, provider, repository, poll_interval_seconds, last_polled_at
		FROM sources WHERE enabled = 1`)
	if err != nil {
		return nil, fmt.Errorf("list enabled sources: %w", err)
	}
	defer rows.Close()

	var sources []ingestion.EnabledSource
	for rows.Next() {
		var e ingestion.EnabledSource
		var lastPolledAtNull sql.NullString
		if err := rows.Scan(&e.ID, &e.Provider, &e.Repository, &e.PollIntervalSeconds, &lastPolledAtNull); err != nil {
			return nil, fmt.Errorf("scan enabled source: %w", err)
		}
		if lastPolledAtNull.Valid && lastPolledAtNull.String != "" {
			t, err := parseTime(lastPolledAtNull.String)
			if err != nil {
				return nil, fmt.Errorf("parse last_polled_at: %w", err)
			}
			e.LastPolledAt = &t
		}
		sources = append(sources, e)
	}
	return sources, rows.Err()
}
