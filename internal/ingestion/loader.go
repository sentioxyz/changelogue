package ingestion

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"time"
)

// SourceLister abstracts the database query for enabled sources so both
// PostgreSQL and SQLite stores can provide sources to the orchestrator.
type SourceLister interface {
	ListEnabledSources(ctx context.Context) ([]EnabledSource, error)
}

// EnabledSource carries the fields the loader needs to construct an IIngestionSource.
type EnabledSource struct {
	ID                  string
	Provider            string
	Repository          string
	PollIntervalSeconds int
	LastPolledAt        *time.Time
}

// ScheduledSource wraps an IIngestionSource with its scheduling metadata
// so the orchestrator can decide whether the source is due for polling.
type ScheduledSource struct {
	Source              IIngestionSource
	PollIntervalSeconds int
	LastPolledAt        *time.Time
}

// SourceLoader reads source configuration from the database and constructs
// IIngestionSource instances. It bridges the API-managed sources table with
// the polling orchestrator.
type SourceLoader struct {
	lister SourceLister
	client *http.Client
}

func NewSourceLoader(lister SourceLister, client *http.Client) *SourceLoader {
	return &SourceLoader{lister: lister, client: client}
}

// LoadEnabledSources queries all enabled sources and constructs the
// appropriate IIngestionSource for each one, along with scheduling metadata.
func (l *SourceLoader) LoadEnabledSources(ctx context.Context) ([]ScheduledSource, error) {
	enabled, err := l.lister.ListEnabledSources(ctx)
	if err != nil {
		return nil, fmt.Errorf("list enabled sources: %w", err)
	}
	var sources []ScheduledSource
	for _, e := range enabled {
		src := BuildSource(l.client, e.ID, e.Provider, e.Repository)
		if src == nil {
			slog.Warn("unsupported source type, skipping",
				"id", e.ID, "type", e.Provider, "repo", e.Repository)
			continue
		}
		sources = append(sources, ScheduledSource{
			Source:              src,
			PollIntervalSeconds: e.PollIntervalSeconds,
			LastPolledAt:        e.LastPolledAt,
		})
	}
	return sources, nil
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
	case "npm":
		return NewNpmSource(client, repository, id)
	default:
		return nil
	}
}
