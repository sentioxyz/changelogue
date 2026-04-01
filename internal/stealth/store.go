package stealth

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/sentioxyz/changelogue/internal/api"
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

// ─────────────────────────────────────────
// ReleasesStore implementation
// ─────────────────────────────────────────

const releaseSelectCols = `
	r.id, r.source_id, r.version, COALESCE(r.raw_data,''), r.released_at, r.created_at,
	s.project_id, p.name, s.provider, s.repository`

func scanRelease(row interface {
	Scan(dest ...any) error
}) (models.Release, error) {
	var r models.Release
	var rawData string
	var releasedAtNull, createdAt sql.NullString
	if err := row.Scan(
		&r.ID, &r.SourceID, &r.Version, &rawData, &releasedAtNull, &createdAt,
		&r.ProjectID, &r.ProjectName, &r.Provider, &r.Repository,
	); err != nil {
		return r, err
	}
	if rawData != "" {
		r.RawData = json.RawMessage(rawData)
	}
	if releasedAtNull.Valid && releasedAtNull.String != "" {
		t, err := parseTime(releasedAtNull.String)
		if err != nil {
			return r, fmt.Errorf("parse released_at: %w", err)
		}
		r.ReleasedAt = &t
	}
	if createdAt.Valid && createdAt.String != "" {
		t, err := parseTime(createdAt.String)
		if err != nil {
			return r, fmt.Errorf("parse created_at: %w", err)
		}
		r.CreatedAt = t
	}
	return r, nil
}

// buildReleaseFilterClauses returns WHERE clause fragments and args for a ReleaseFilter.
// baseWhere is any existing WHERE predicate (e.g. "r.source_id = ?").
func buildReleaseFilterClauses(baseWhere string, baseArgs []any, filter models.ReleaseFilter) (string, []any) {
	where := baseWhere
	args := append([]any{}, baseArgs...)
	if filter.Provider != "" {
		if where != "" {
			where += " AND "
		}
		where += "s.provider = ?"
		args = append(args, filter.Provider)
	}
	return where, args
}

// ListAllReleases returns a paginated list of all releases with enriched fields.
func (s *Store) ListAllReleases(ctx context.Context, page, perPage int, _ bool, filter models.ReleaseFilter) ([]models.Release, int, error) {
	where, args := buildReleaseFilterClauses("", nil, filter)
	countSQL := `SELECT COUNT(*) FROM releases r JOIN sources s ON r.source_id = s.id JOIN projects p ON s.project_id = p.id`
	if where != "" {
		countSQL += " WHERE " + where
	}
	var total int
	if err := s.db.QueryRowContext(ctx, countSQL, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count releases: %w", err)
	}

	offset := (page - 1) * perPage
	querySQL := `SELECT ` + releaseSelectCols + `
		FROM releases r
		JOIN sources s ON r.source_id = s.id
		JOIN projects p ON s.project_id = p.id`
	if where != "" {
		querySQL += " WHERE " + where
	}
	querySQL += " ORDER BY r.created_at DESC LIMIT ? OFFSET ?"
	args = append(args, perPage, offset)

	rows, err := s.db.QueryContext(ctx, querySQL, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("list releases: %w", err)
	}
	defer rows.Close()

	var releases []models.Release
	for rows.Next() {
		rel, err := scanRelease(rows)
		if err != nil {
			return nil, 0, fmt.Errorf("scan release: %w", err)
		}
		releases = append(releases, rel)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, err
	}
	return releases, total, nil
}

// ListReleasesBySource returns a paginated list of releases for a specific source.
func (s *Store) ListReleasesBySource(ctx context.Context, sourceID string, page, perPage int, _ bool, filter models.ReleaseFilter) ([]models.Release, int, error) {
	where, args := buildReleaseFilterClauses("r.source_id = ?", []any{sourceID}, filter)
	countSQL := `SELECT COUNT(*) FROM releases r JOIN sources s ON r.source_id = s.id JOIN projects p ON s.project_id = p.id WHERE ` + where
	var total int
	if err := s.db.QueryRowContext(ctx, countSQL, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count releases by source: %w", err)
	}

	offset := (page - 1) * perPage
	querySQL := `SELECT ` + releaseSelectCols + `
		FROM releases r
		JOIN sources s ON r.source_id = s.id
		JOIN projects p ON s.project_id = p.id
		WHERE ` + where + ` ORDER BY r.created_at DESC LIMIT ? OFFSET ?`
	args = append(args, perPage, offset)

	rows, err := s.db.QueryContext(ctx, querySQL, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("list releases by source: %w", err)
	}
	defer rows.Close()

	var releases []models.Release
	for rows.Next() {
		rel, err := scanRelease(rows)
		if err != nil {
			return nil, 0, fmt.Errorf("scan release: %w", err)
		}
		releases = append(releases, rel)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, err
	}
	return releases, total, nil
}

// ListReleasesByProject returns a paginated list of releases for all sources in a project.
func (s *Store) ListReleasesByProject(ctx context.Context, projectID string, page, perPage int, _ bool, filter models.ReleaseFilter) ([]models.Release, int, error) {
	where, args := buildReleaseFilterClauses("s.project_id = ?", []any{projectID}, filter)
	countSQL := `SELECT COUNT(*) FROM releases r JOIN sources s ON r.source_id = s.id JOIN projects p ON s.project_id = p.id WHERE ` + where
	var total int
	if err := s.db.QueryRowContext(ctx, countSQL, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count releases by project: %w", err)
	}

	offset := (page - 1) * perPage
	querySQL := `SELECT ` + releaseSelectCols + `
		FROM releases r
		JOIN sources s ON r.source_id = s.id
		JOIN projects p ON s.project_id = p.id
		WHERE ` + where + ` ORDER BY r.created_at DESC LIMIT ? OFFSET ?`
	args = append(args, perPage, offset)

	rows, err := s.db.QueryContext(ctx, querySQL, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("list releases by project: %w", err)
	}
	defer rows.Close()

	var releases []models.Release
	for rows.Next() {
		rel, err := scanRelease(rows)
		if err != nil {
			return nil, 0, fmt.Errorf("scan release: %w", err)
		}
		releases = append(releases, rel)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, err
	}
	return releases, total, nil
}

// GetRelease retrieves a single release by ID with enriched fields.
func (s *Store) GetRelease(ctx context.Context, id string) (*models.Release, error) {
	row := s.db.QueryRowContext(ctx, `SELECT `+releaseSelectCols+`
		FROM releases r
		JOIN sources s ON r.source_id = s.id
		JOIN projects p ON s.project_id = p.id
		WHERE r.id = ?`, id)
	rel, err := scanRelease(row)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("release not found: %s", id)
		}
		return nil, fmt.Errorf("get release: %w", err)
	}
	return &rel, nil
}

// ─────────────────────────────────────────
// ChannelsStore implementation
// ─────────────────────────────────────────

// ListChannels returns a paginated list of notification channels.
func (s *Store) ListChannels(ctx context.Context, page, perPage int) ([]models.NotificationChannel, int, error) {
	var total int
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM notification_channels`).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count channels: %w", err)
	}

	offset := (page - 1) * perPage
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, name, type, config, created_at, updated_at
		FROM notification_channels
		ORDER BY created_at ASC
		LIMIT ? OFFSET ?`, perPage, offset)
	if err != nil {
		return nil, 0, fmt.Errorf("list channels: %w", err)
	}
	defer rows.Close()

	var channels []models.NotificationChannel
	for rows.Next() {
		ch, err := scanChannel(rows)
		if err != nil {
			return nil, 0, fmt.Errorf("scan channel: %w", err)
		}
		channels = append(channels, ch)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, err
	}
	return channels, total, nil
}

func scanChannel(row interface {
	Scan(dest ...any) error
}) (models.NotificationChannel, error) {
	var ch models.NotificationChannel
	var configStr, createdAt, updatedAt string
	if err := row.Scan(&ch.ID, &ch.Name, &ch.Type, &configStr, &createdAt, &updatedAt); err != nil {
		return ch, err
	}
	ch.Config = json.RawMessage(configStr)
	var err error
	if ch.CreatedAt, err = parseTime(createdAt); err != nil {
		return ch, fmt.Errorf("parse created_at: %w", err)
	}
	if ch.UpdatedAt, err = parseTime(updatedAt); err != nil {
		return ch, fmt.Errorf("parse updated_at: %w", err)
	}
	return ch, nil
}

// CreateChannel inserts a new notification channel, generating its ID and timestamps.
func (s *Store) CreateChannel(ctx context.Context, ch *models.NotificationChannel) error {
	ch.ID = uuid.New().String()
	now := time.Now().UTC()
	ch.CreatedAt = now
	ch.UpdatedAt = now

	_, err := s.db.ExecContext(ctx, `
		INSERT INTO notification_channels (id, name, type, config, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?)`,
		ch.ID, ch.Name, ch.Type, string(ch.Config),
		now.Format(time.RFC3339Nano), now.Format(time.RFC3339Nano),
	)
	if err != nil {
		return fmt.Errorf("create channel: %w", err)
	}
	return nil
}

// GetChannel retrieves a single notification channel by ID.
func (s *Store) GetChannel(ctx context.Context, id string) (*models.NotificationChannel, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT id, name, type, config, created_at, updated_at
		FROM notification_channels WHERE id = ?`, id)
	ch, err := scanChannel(row)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("channel not found: %s", id)
		}
		return nil, fmt.Errorf("get channel: %w", err)
	}
	return &ch, nil
}

// UpdateChannel updates the mutable fields of an existing notification channel.
func (s *Store) UpdateChannel(ctx context.Context, id string, ch *models.NotificationChannel) error {
	now := time.Now().UTC()
	ch.UpdatedAt = now

	result, err := s.db.ExecContext(ctx, `
		UPDATE notification_channels
		SET name = ?, type = ?, config = ?, updated_at = ?
		WHERE id = ?`,
		ch.Name, ch.Type, string(ch.Config), now.Format(time.RFC3339Nano), id,
	)
	if err != nil {
		return fmt.Errorf("update channel: %w", err)
	}
	n, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return fmt.Errorf("channel not found: %s", id)
	}
	return nil
}

// DeleteChannel removes a notification channel by ID.
func (s *Store) DeleteChannel(ctx context.Context, id string) error {
	result, err := s.db.ExecContext(ctx, `DELETE FROM notification_channels WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("delete channel: %w", err)
	}
	n, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return fmt.Errorf("channel not found: %s", id)
	}
	return nil
}

// ─────────────────────────────────────────
// SubscriptionsStore implementation
// ─────────────────────────────────────────

func scanSubscription(row interface {
	Scan(dest ...any) error
}) (models.Subscription, error) {
	var sub models.Subscription
	var sourceIDNull, projectIDNull, versionFilterNull, configNull sql.NullString
	var createdAt string
	if err := row.Scan(
		&sub.ID, &sub.ChannelID, &sub.Type,
		&sourceIDNull, &projectIDNull, &versionFilterNull, &configNull,
		&createdAt,
	); err != nil {
		return sub, err
	}
	if sourceIDNull.Valid {
		s := sourceIDNull.String
		sub.SourceID = &s
	}
	if projectIDNull.Valid {
		p := projectIDNull.String
		sub.ProjectID = &p
	}
	if versionFilterNull.Valid {
		sub.VersionFilter = versionFilterNull.String
	}
	if configNull.Valid && configNull.String != "" {
		sub.Config = json.RawMessage(configNull.String)
	}
	var err error
	if sub.CreatedAt, err = parseTime(createdAt); err != nil {
		return sub, fmt.Errorf("parse created_at: %w", err)
	}
	return sub, nil
}

// ListSubscriptions returns a paginated list of subscriptions.
func (s *Store) ListSubscriptions(ctx context.Context, page, perPage int) ([]models.Subscription, int, error) {
	var total int
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM subscriptions`).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count subscriptions: %w", err)
	}

	offset := (page - 1) * perPage
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, channel_id, type, source_id, project_id, version_filter, config, created_at
		FROM subscriptions
		ORDER BY created_at ASC
		LIMIT ? OFFSET ?`, perPage, offset)
	if err != nil {
		return nil, 0, fmt.Errorf("list subscriptions: %w", err)
	}
	defer rows.Close()

	var subs []models.Subscription
	for rows.Next() {
		sub, err := scanSubscription(rows)
		if err != nil {
			return nil, 0, fmt.Errorf("scan subscription: %w", err)
		}
		subs = append(subs, sub)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, err
	}
	return subs, total, nil
}

// CreateSubscription inserts a new subscription, generating its ID and timestamp.
func (s *Store) CreateSubscription(ctx context.Context, sub *models.Subscription) error {
	sub.ID = uuid.New().String()
	now := time.Now().UTC()
	sub.CreatedAt = now

	var sourceIDNull, projectIDNull, configNull sql.NullString
	if sub.SourceID != nil {
		sourceIDNull = sql.NullString{String: *sub.SourceID, Valid: true}
	}
	if sub.ProjectID != nil {
		projectIDNull = sql.NullString{String: *sub.ProjectID, Valid: true}
	}
	if len(sub.Config) > 0 {
		configNull = sql.NullString{String: string(sub.Config), Valid: true}
	}

	_, err := s.db.ExecContext(ctx, `
		INSERT INTO subscriptions (id, channel_id, type, source_id, project_id, version_filter, config, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		sub.ID, sub.ChannelID, sub.Type,
		sourceIDNull, projectIDNull, sub.VersionFilter, configNull,
		now.Format(time.RFC3339Nano),
	)
	if err != nil {
		return fmt.Errorf("create subscription: %w", err)
	}
	return nil
}

// CreateSubscriptionBatch inserts multiple subscriptions in a transaction.
func (s *Store) CreateSubscriptionBatch(ctx context.Context, subs []models.Subscription) ([]models.Subscription, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("begin transaction: %w", err)
	}
	defer func() {
		if err != nil {
			tx.Rollback()
		}
	}()

	now := time.Now().UTC()
	created := make([]models.Subscription, len(subs))
	for i := range subs {
		subs[i].ID = uuid.New().String()
		subs[i].CreatedAt = now

		var sourceIDNull, projectIDNull, configNull sql.NullString
		if subs[i].SourceID != nil {
			sourceIDNull = sql.NullString{String: *subs[i].SourceID, Valid: true}
		}
		if subs[i].ProjectID != nil {
			projectIDNull = sql.NullString{String: *subs[i].ProjectID, Valid: true}
		}
		if len(subs[i].Config) > 0 {
			configNull = sql.NullString{String: string(subs[i].Config), Valid: true}
		}

		_, err = tx.ExecContext(ctx, `
			INSERT INTO subscriptions (id, channel_id, type, source_id, project_id, version_filter, config, created_at)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
			subs[i].ID, subs[i].ChannelID, subs[i].Type,
			sourceIDNull, projectIDNull, subs[i].VersionFilter, configNull,
			now.Format(time.RFC3339Nano),
		)
		if err != nil {
			return nil, fmt.Errorf("create subscription batch[%d]: %w", i, err)
		}
		created[i] = subs[i]
	}

	if err = tx.Commit(); err != nil {
		return nil, fmt.Errorf("commit batch: %w", err)
	}
	return created, nil
}

// GetSubscription retrieves a single subscription by ID.
func (s *Store) GetSubscription(ctx context.Context, id string) (*models.Subscription, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT id, channel_id, type, source_id, project_id, version_filter, config, created_at
		FROM subscriptions WHERE id = ?`, id)
	sub, err := scanSubscription(row)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("subscription not found: %s", id)
		}
		return nil, fmt.Errorf("get subscription: %w", err)
	}
	return &sub, nil
}

// UpdateSubscription updates the mutable fields of an existing subscription.
func (s *Store) UpdateSubscription(ctx context.Context, id string, sub *models.Subscription) error {
	var sourceIDNull, projectIDNull, configNull sql.NullString
	if sub.SourceID != nil {
		sourceIDNull = sql.NullString{String: *sub.SourceID, Valid: true}
	}
	if sub.ProjectID != nil {
		projectIDNull = sql.NullString{String: *sub.ProjectID, Valid: true}
	}
	if len(sub.Config) > 0 {
		configNull = sql.NullString{String: string(sub.Config), Valid: true}
	}

	result, err := s.db.ExecContext(ctx, `
		UPDATE subscriptions
		SET channel_id = ?, type = ?, source_id = ?, project_id = ?, version_filter = ?, config = ?
		WHERE id = ?`,
		sub.ChannelID, sub.Type, sourceIDNull, projectIDNull, sub.VersionFilter, configNull, id,
	)
	if err != nil {
		return fmt.Errorf("update subscription: %w", err)
	}
	n, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return fmt.Errorf("subscription not found: %s", id)
	}
	return nil
}

// DeleteSubscription removes a subscription by ID.
func (s *Store) DeleteSubscription(ctx context.Context, id string) error {
	result, err := s.db.ExecContext(ctx, `DELETE FROM subscriptions WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("delete subscription: %w", err)
	}
	n, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return fmt.Errorf("subscription not found: %s", id)
	}
	return nil
}

// DeleteSubscriptionBatch removes multiple subscriptions by their IDs.
func (s *Store) DeleteSubscriptionBatch(ctx context.Context, ids []string) error {
	if len(ids) == 0 {
		return nil
	}
	placeholders := strings.Repeat("?,", len(ids))
	placeholders = placeholders[:len(placeholders)-1]
	query := fmt.Sprintf("DELETE FROM subscriptions WHERE id IN (%s)", placeholders)
	args := make([]any, len(ids))
	for i, id := range ids {
		args[i] = id
	}
	_, err := s.db.ExecContext(ctx, query, args...)
	if err != nil {
		return fmt.Errorf("delete subscription batch: %w", err)
	}
	return nil
}

// ─────────────────────────────────────────
// KeyStore implementation
// ─────────────────────────────────────────

// ValidateKey checks whether a raw API key is valid (compares its SHA-256 hash).
func (s *Store) ValidateKey(ctx context.Context, rawKey string) (bool, error) {
	hash := fmt.Sprintf("%x", sha256.Sum256([]byte(rawKey)))
	var id string
	err := s.db.QueryRowContext(ctx, `SELECT id FROM api_keys WHERE key_hash = ?`, hash).Scan(&id)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return true, nil
}

// TouchKeyUsage updates last_used_at for the key matching rawKey (fire-and-forget).
func (s *Store) TouchKeyUsage(ctx context.Context, rawKey string) {
	hash := fmt.Sprintf("%x", sha256.Sum256([]byte(rawKey)))
	now := time.Now().UTC().Format(time.RFC3339Nano)
	s.db.ExecContext(ctx, `UPDATE api_keys SET last_used_at = ? WHERE key_hash = ?`, now, hash)
}

// ─────────────────────────────────────────
// HealthChecker implementation
// ─────────────────────────────────────────

// GetStats returns aggregate dashboard statistics from the SQLite store.
func (s *Store) GetStats(ctx context.Context) (*api.DashboardStats, error) {
	var stats api.DashboardStats
	s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM releases`).Scan(&stats.TotalReleases)
	s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM sources WHERE enabled = 1`).Scan(&stats.ActiveSources)
	s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM projects`).Scan(&stats.TotalProjects)
	// PendingAgentRuns, ReleasesThisWeek, AttentionNeeded are not applicable in stealth mode.
	return &stats, nil
}

// GetTrend returns time-bucketed release counts. Stealth mode returns an empty slice.
func (s *Store) GetTrend(_ context.Context, _ string, _, _ time.Time) ([]api.TrendBucket, error) {
	return []api.TrendBucket{}, nil
}
