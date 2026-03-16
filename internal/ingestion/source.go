package ingestion

import (
	"context"
	"fmt"
	"net/http"
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
	// SourceID returns the database ID of the sources row for this provider.
	SourceID() string
	// FetchNewReleases polls the upstream registry and returns discovered releases.
	FetchNewReleases(ctx context.Context) ([]IngestionResult, error)
}

// httpStatusError returns a descriptive error for non-200 HTTP responses.
func httpStatusError(resp *http.Response) error {
	switch resp.StatusCode {
	case http.StatusNotFound:
		return fmt.Errorf("not found (HTTP 404) — check that the repository name is correct")
	case http.StatusForbidden:
		return fmt.Errorf("access denied (HTTP 403) — the repository may be private or require authentication")
	case http.StatusUnauthorized:
		return fmt.Errorf("unauthorized (HTTP 401) — authentication is required")
	case http.StatusTooManyRequests:
		return fmt.Errorf("rate limited (HTTP 429) — too many requests, will retry later")
	default:
		return fmt.Errorf("upstream error (HTTP %d)", resp.StatusCode)
	}
}
