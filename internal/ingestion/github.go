package ingestion

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/sentioxyz/changelogue/internal/githubauth"
)

const defaultGitHubAPIURL = "https://api.github.com"

// ghRelease represents a single release from the GitHub REST API.
type ghRelease struct {
	TagName     string `json:"tag_name"`
	Body        string `json:"body"`
	HTMLURL     string `json:"html_url"`
	PublishedAt string `json:"published_at"`
	Prerelease  bool   `json:"prerelease"`
	Draft       bool   `json:"draft"`
}

// GitHubSource polls the GitHub REST API for repository releases.
type GitHubSource struct {
	client        *http.Client
	repository    string
	baseURL       string
	sourceID      string
	tokenProvider githubauth.TokenProvider
}

func NewGitHubSource(client *http.Client, repository string, sourceID string) *GitHubSource {
	return NewGitHubSourceWithTokenProvider(client, repository, sourceID, githubauth.NewDefaultTokenProvider(client, defaultGitHubAPIURL))
}

func NewGitHubSourceWithTokenProvider(client *http.Client, repository string, sourceID string, tokenProvider githubauth.TokenProvider) *GitHubSource {
	return &GitHubSource{
		client:        client,
		repository:    repository,
		baseURL:       defaultGitHubAPIURL,
		sourceID:      sourceID,
		tokenProvider: tokenProvider,
	}
}

func (s *GitHubSource) Name() string     { return "github" }
func (s *GitHubSource) SourceID() string { return s.sourceID }

func (s *GitHubSource) FetchNewReleases(ctx context.Context) ([]IngestionResult, error) {
	url := fmt.Sprintf("%s/repos/%s/releases?per_page=30", s.baseURL, s.repository)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Accept", "application/vnd.github+json")

	if token, err := s.token(ctx); err == nil && token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	} else if err != nil && !errors.Is(err, githubauth.ErrNotConfigured) {
		return nil, err
	}

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetch releases: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, httpStatusError(resp)
	}

	var releases []ghRelease
	if err := json.NewDecoder(resp.Body).Decode(&releases); err != nil {
		return nil, fmt.Errorf("decode releases: %w", err)
	}

	results := make([]IngestionResult, 0, len(releases))
	for _, rel := range releases {
		if rel.Draft {
			continue
		}
		if rel.TagName == "" {
			continue
		}

		ts, _ := time.Parse(time.RFC3339, rel.PublishedAt)

		prerelease := "false"
		if rel.Prerelease {
			prerelease = "true"
		}

		results = append(results, IngestionResult{
			Repository: s.repository,
			RawVersion: rel.TagName,
			Changelog:  rel.Body,
			Metadata: map[string]string{
				"release_url": rel.HTMLURL,
				"prerelease":  prerelease,
			},
			Timestamp: ts,
		})
	}
	return results, nil
}

func (s *GitHubSource) token(ctx context.Context) (string, error) {
	if s.tokenProvider == nil {
		return "", githubauth.ErrNotConfigured
	}
	owner, repo, err := splitGitHubRepo(s.repository)
	if err != nil {
		return "", err
	}
	return s.tokenProvider.TokenForRepo(ctx, owner, repo)
}

func splitGitHubRepo(repository string) (string, string, error) {
	parts := strings.Split(strings.TrimSpace(repository), "/")
	if len(parts) < 2 || parts[0] == "" || parts[1] == "" {
		return "", "", fmt.Errorf("invalid GitHub repository %q: expected owner/repo", repository)
	}
	return parts[0], parts[1], nil
}
