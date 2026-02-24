package api

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"

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

// --- HealthChecker ---

func (s *PgStore) Ping(ctx context.Context) error {
	return s.pool.Ping(ctx)
}
