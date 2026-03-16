package ingestion

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// ScheduledSource wraps an IIngestionSource with its scheduling metadata
// so the orchestrator can decide whether the source is due for polling.
type ScheduledSource struct {
	Source              IIngestionSource
	PollIntervalSeconds int
	LastPolledAt         *time.Time
}

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
// appropriate IIngestionSource for each one, along with scheduling metadata.
func (l *SourceLoader) LoadEnabledSources(ctx context.Context) ([]ScheduledSource, error) {
	rows, err := l.pool.Query(ctx,
		`SELECT id, provider, repository, poll_interval_seconds, last_polled_at
		 FROM sources WHERE enabled = true`)
	if err != nil {
		return nil, fmt.Errorf("query enabled sources: %w", err)
	}
	defer rows.Close()

	var sources []ScheduledSource
	for rows.Next() {
		var id string
		var sourceType, repository string
		var pollInterval int
		var lastPolledAt *time.Time
		if err := rows.Scan(&id, &sourceType, &repository, &pollInterval, &lastPolledAt); err != nil {
			return nil, fmt.Errorf("scan source row: %w", err)
		}

		src := l.buildSource(id, sourceType, repository)
		if src == nil {
			slog.Warn("unsupported source type, skipping",
				"id", id, "type", sourceType, "repo", repository)
			continue
		}
		sources = append(sources, ScheduledSource{
			Source:              src,
			PollIntervalSeconds: pollInterval,
			LastPolledAt:         lastPolledAt,
		})
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
	return BuildSource(l.client, id, sourceType, repository)
}

// BuildSource constructs the appropriate IIngestionSource based on provider type.
func BuildSource(client *http.Client, id string, sourceType, repository string) IIngestionSource {
	switch sourceType {
	case "dockerhub":
		return NewDockerHubSource(client, repository, id)
	case "github":
		return NewGitHubSource(client, repository, id)
	case "ecr-public":
		return NewECRPublicSource(client, repository, id)
	case "gitlab":
		return NewGitLabSource(client, repository, id)
	case "pypi":
		return NewPyPISource(client, repository, id)
	default:
		return nil
	}
}
