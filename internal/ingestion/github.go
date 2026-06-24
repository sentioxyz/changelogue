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

// ghTag represents a single tag from the GitHub REST API.
type ghTag struct {
	Name   string `json:"name"`
	Commit struct {
		SHA string `json:"sha"`
	} `json:"commit"`
}

// ghCommit holds the commit date used to timestamp tag-derived releases.
type ghCommit struct {
	Commit struct {
		Committer struct {
			Date string `json:"date"`
		} `json:"committer"`
	} `json:"commit"`
}

// GitHubSource polls the GitHub REST API for repository releases.
type GitHubSource struct {
	client        *http.Client
	repository    string
	baseURL       string
	sourceID      string
	releasesOnly  bool
	tokenProvider githubauth.TokenProvider
}

func NewGitHubSource(client *http.Client, repository string, sourceID string) *GitHubSource {
	return NewGitHubSourceWithTokenProvider(client, repository, sourceID, githubauth.NewDefaultTokenProvider(client, defaultGitHubAPIURL), true)
}

func NewGitHubSourceWithTokenProvider(client *http.Client, repository string, sourceID string, tokenProvider githubauth.TokenProvider, releasesOnly bool) *GitHubSource {
	return &GitHubSource{
		client:        client,
		repository:    repository,
		baseURL:       defaultGitHubAPIURL,
		sourceID:      sourceID,
		releasesOnly:  releasesOnly,
		tokenProvider: tokenProvider,
	}
}

func (s *GitHubSource) Name() string     { return "github" }
func (s *GitHubSource) SourceID() string { return s.sourceID }

func (s *GitHubSource) FetchNewReleases(ctx context.Context) ([]IngestionResult, error) {
	results, seen, err := s.fetchReleases(ctx)
	if err != nil {
		return nil, err
	}

	if s.releasesOnly {
		return results, nil
	}

	tags, err := s.fetchTags(ctx)
	if err != nil {
		return nil, err
	}
	for _, t := range tags {
		if t.Name == "" || seen[t.Name] {
			continue
		}
		seen[t.Name] = true
		results = append(results, IngestionResult{
			Repository: s.repository,
			RawVersion: t.Name,
			Metadata: map[string]string{
				"release_url": fmt.Sprintf("https://github.com/%s/releases/tag/%s", s.repository, t.Name),
				"source_kind": "tag",
			},
			Timestamp: s.tagCommitDate(ctx, t.Commit.SHA),
		})
	}
	return results, nil
}

// fetchReleases returns published releases and the set of tag names they cover,
// so tag discovery can skip versions that already have a release.
func (s *GitHubSource) fetchReleases(ctx context.Context) ([]IngestionResult, map[string]bool, error) {
	url := fmt.Sprintf("%s/repos/%s/releases?per_page=30", s.baseURL, s.repository)

	resp, err := s.doGitHubRequest(ctx, url)
	if err != nil {
		return nil, nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, nil, httpStatusError(resp)
	}

	var releases []ghRelease
	if err := json.NewDecoder(resp.Body).Decode(&releases); err != nil {
		return nil, nil, fmt.Errorf("decode releases: %w", err)
	}

	results := make([]IngestionResult, 0, len(releases))
	seen := make(map[string]bool, len(releases))
	for _, rel := range releases {
		if rel.Draft {
			continue
		}
		if rel.TagName == "" {
			continue
		}
		seen[rel.TagName] = true

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
	return results, seen, nil
}

// fetchTags returns the repository's git tags. Tags carry no changelog body or
// publish timestamp, so tag-derived releases have neither.
func (s *GitHubSource) fetchTags(ctx context.Context) ([]ghTag, error) {
	url := fmt.Sprintf("%s/repos/%s/tags?per_page=30", s.baseURL, s.repository)

	resp, err := s.doGitHubRequest(ctx, url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, httpStatusError(resp)
	}

	var tags []ghTag
	if err := json.NewDecoder(resp.Body).Decode(&tags); err != nil {
		return nil, fmt.Errorf("decode tags: %w", err)
	}
	return tags, nil
}

// tagCommitDate returns the committer date for a tag's commit. Tags carry no
// date of their own, so this backfills released_at. Failures are non-fatal:
// a zero time is returned and the release is stored without a date.
func (s *GitHubSource) tagCommitDate(ctx context.Context, sha string) time.Time {
	if sha == "" {
		return time.Time{}
	}
	url := fmt.Sprintf("%s/repos/%s/commits/%s", s.baseURL, s.repository, sha)
	resp, err := s.doGitHubRequest(ctx, url)
	if err != nil {
		return time.Time{}
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return time.Time{}
	}
	var c ghCommit
	if err := json.NewDecoder(resp.Body).Decode(&c); err != nil {
		return time.Time{}
	}
	ts, _ := time.Parse(time.RFC3339, c.Commit.Committer.Date)
	return ts
}

func (s *GitHubSource) doGitHubRequest(ctx context.Context, url string) (*http.Response, error) {
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
		return nil, fmt.Errorf("fetch: %w", err)
	}
	return resp, nil
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
