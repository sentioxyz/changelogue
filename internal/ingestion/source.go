package ingestion

import (
	"context"
	"time"
)

// IngestionResult is raw release data returned by an ingestion source.
type IngestionResult struct {
	Repository string
	RawVersion string
	Changelog  string
	Metadata   map[string]string
	Timestamp  time.Time
}

// IIngestionSource abstracts a polling-based release data provider.
// Each implementation fetches the latest releases from a specific registry.
type IIngestionSource interface {
	// Name returns the source identifier (e.g., "dockerhub", "github").
	Name() string
	// FetchNewReleases polls the upstream registry and returns discovered releases.
	FetchNewReleases(ctx context.Context) ([]IngestionResult, error)
}
