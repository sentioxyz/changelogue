package ingestion

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

const defaultNpmBaseURL = "https://registry.npmjs.org"

// npmPackage represents the top-level response from the npm registry API.
type npmPackage struct {
	Name string `json:"name"`
	// Time maps version strings to their publish timestamps.
	// Also contains "created" and "modified" keys which we skip.
	Time map[string]string `json:"time"`
}

// NpmSource polls the npm registry API for package releases.
type NpmSource struct {
	client   *http.Client
	pkg      string
	baseURL  string
	sourceID string
}

func NewNpmSource(client *http.Client, pkg string, sourceID string) *NpmSource {
	return &NpmSource{
		client:   client,
		pkg:      pkg,
		baseURL:  defaultNpmBaseURL,
		sourceID: sourceID,
	}
}

func (s *NpmSource) Name() string    { return "npm" }
func (s *NpmSource) SourceID() string { return s.sourceID }

func (s *NpmSource) FetchNewReleases(ctx context.Context) ([]IngestionResult, error) {
	url := fmt.Sprintf("%s/%s", s.baseURL, s.pkg)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Accept", "application/json")

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetch releases: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, httpStatusError(resp)
	}

	var pkg npmPackage
	if err := json.NewDecoder(resp.Body).Decode(&pkg); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	results := make([]IngestionResult, 0, len(pkg.Time))
	for version, publishedAt := range pkg.Time {
		// Skip metadata keys that aren't real versions.
		if version == "created" || version == "modified" {
			continue
		}
		if version == "" {
			continue
		}

		var ts time.Time
		if publishedAt != "" {
			parsed, err := time.Parse(time.RFC3339, publishedAt)
			if err == nil {
				ts = parsed
			}
		}

		releaseURL := fmt.Sprintf("https://www.npmjs.com/package/%s/v/%s", s.pkg, version)

		results = append(results, IngestionResult{
			Repository: s.pkg,
			RawVersion: version,
			Changelog:  "",
			Metadata: map[string]string{
				"release_url": releaseURL,
			},
			Timestamp: ts,
		})
	}
	return results, nil
}
