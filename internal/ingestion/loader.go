package ingestion

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"

	"github.com/jackc/pgx/v5/pgxpool"
)

// SourceLoader reads source configuration from the database and constructs
// IIngestionSource instances. It bridges the API-managed sources table with
// the polling orchestrator.
type SourceLoader struct {
	pool   *pgxpool.Pool
	client *http.Client
}

func NewSourceLoader(pool *pgxpool.Pool, client *http.Client) *SourceLoader {
	return &SourceLoader{pool: pool, client: client}
}

// LoadEnabledSources queries all enabled sources and constructs the
// appropriate IIngestionSource for each one based on provider.
func (l *SourceLoader) LoadEnabledSources(ctx context.Context) ([]IIngestionSource, error) {
	rows, err := l.pool.Query(ctx,
		`SELECT id, provider, repository FROM sources WHERE enabled = true`)
	if err != nil {
		return nil, fmt.Errorf("query enabled sources: %w", err)
	}
	defer rows.Close()

	var sources []IIngestionSource
	for rows.Next() {
		var id string
		var sourceType, repository string
		if err := rows.Scan(&id, &sourceType, &repository); err != nil {
			return nil, fmt.Errorf("scan source row: %w", err)
		}

		src := l.buildSource(id, sourceType, repository)
		if src == nil {
			slog.Warn("unsupported source type, skipping",
				"id", id, "type", sourceType, "repo", repository)
			continue
		}
		sources = append(sources, src)
	}
	return sources, rows.Err()
}

// LookupSourceID finds the source ID for a given (provider, repository) pair.
// Returns empty string and false if no matching enabled source exists.
func (l *SourceLoader) LookupSourceID(ctx context.Context, provider, repository string) (string, bool) {
	var id string
	err := l.pool.QueryRow(ctx,
		`SELECT id FROM sources WHERE provider = $1 AND repository = $2 AND enabled = true`,
		provider, repository,
	).Scan(&id)
	if err != nil {
		return "", false
	}
	return id, true
}

func (l *SourceLoader) buildSource(id string, sourceType, repository string) IIngestionSource {
	switch sourceType {
	case "dockerhub":
		return NewDockerHubSource(l.client, repository, id)
	case "github":
		return NewGitHubAtomSource(l.client, repository, id)
	default:
		return nil
	}
}
