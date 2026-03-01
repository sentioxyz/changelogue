package ingestion

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"time"
)

const defaultGitLabAPIURL = "https://gitlab.com"

// glRelease represents a single release from the GitLab REST API v4.
type glRelease struct {
	TagName         string  `json:"tag_name"`
	Description     string  `json:"description"`
	ReleasedAt      string  `json:"released_at"`
	UpcomingRelease bool    `json:"upcoming_release"`
	Links           glLinks `json:"_links"`
}

type glLinks struct {
	Self string `json:"self"`
}

// GitLabSource polls the GitLab REST API v4 for project releases.
type GitLabSource struct {
	client     *http.Client
	repository string
	baseURL    string
	sourceID   string
}

func NewGitLabSource(client *http.Client, repository string, sourceID string) *GitLabSource {
	return &GitLabSource{
		client:     client,
		repository: repository,
		baseURL:    defaultGitLabAPIURL,
		sourceID:   sourceID,
	}
}

func (s *GitLabSource) Name() string     { return "gitlab" }
func (s *GitLabSource) SourceID() string { return s.sourceID }

func (s *GitLabSource) FetchNewReleases(ctx context.Context) ([]IngestionResult, error) {
	encoded := url.PathEscape(s.repository)
	apiURL := fmt.Sprintf("%s/api/v4/projects/%s/releases?per_page=20", s.baseURL, encoded)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, apiURL, nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	if token := os.Getenv("GITLAB_TOKEN"); token != "" {
		req.Header.Set("PRIVATE-TOKEN", token)
	}

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetch releases: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status: %d", resp.StatusCode)
	}

	var releases []glRelease
	if err := json.NewDecoder(resp.Body).Decode(&releases); err != nil {
		return nil, fmt.Errorf("decode releases: %w", err)
	}

	results := make([]IngestionResult, 0, len(releases))
	for _, rel := range releases {
		if rel.TagName == "" {
			continue
		}

		ts, _ := time.Parse(time.RFC3339, rel.ReleasedAt)

		prerelease := "false"
		if rel.UpcomingRelease {
			prerelease = "true"
		}

		results = append(results, IngestionResult{
			Repository: s.repository,
			RawVersion: rel.TagName,
			Changelog:  rel.Description,
			Metadata: map[string]string{
				"release_url": rel.Links.Self,
				"prerelease":  prerelease,
			},
			Timestamp: ts,
		})
	}
	return results, nil
}
