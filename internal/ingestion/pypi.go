package ingestion

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

const defaultPyPIBaseURL = "https://pypi.org"

// pypiRelease represents version-level metadata from the PyPI JSON API.
type pypiRelease struct {
	UploadTime string `json:"upload_time_iso_8601"`
}

// pypiProject represents the top-level response from the PyPI JSON API.
type pypiProject struct {
	Info struct {
		Name    string `json:"name"`
		Summary string `json:"summary"`
	} `json:"info"`
	Releases map[string][]pypiRelease `json:"releases"`
}

// PyPISource polls the PyPI JSON API for package releases.
type PyPISource struct {
	client   *http.Client
	pkg      string
	baseURL  string
	sourceID string
}

func NewPyPISource(client *http.Client, pkg string, sourceID string) *PyPISource {
	return &PyPISource{
		client:   client,
		pkg:      pkg,
		baseURL:  defaultPyPIBaseURL,
		sourceID: sourceID,
	}
}

func (s *PyPISource) Name() string    { return "pypi" }
func (s *PyPISource) SourceID() string { return s.sourceID }

func (s *PyPISource) FetchNewReleases(ctx context.Context) ([]IngestionResult, error) {
	url := fmt.Sprintf("%s/pypi/%s/json", s.baseURL, s.pkg)

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

	var project pypiProject
	if err := json.NewDecoder(resp.Body).Decode(&project); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	results := make([]IngestionResult, 0, len(project.Releases))
	for version, files := range project.Releases {
		if version == "" {
			continue
		}
		// Skip versions with no files (yanked/empty).
		if len(files) == 0 {
			continue
		}

		// Use the earliest upload timestamp from the version's files.
		var ts time.Time
		for _, f := range files {
			if f.UploadTime != "" {
				parsed, err := time.Parse(time.RFC3339, f.UploadTime)
				if err == nil {
					if ts.IsZero() || parsed.Before(ts) {
						ts = parsed
					}
				}
			}
		}

		releaseURL := fmt.Sprintf("%s/project/%s/%s/", s.baseURL, s.pkg, version)

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
