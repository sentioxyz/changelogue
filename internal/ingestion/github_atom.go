package ingestion

import (
	"context"
	"encoding/xml"
	"fmt"
	"net/http"
	"strings"
	"time"
)

const defaultGitHubURL = "https://github.com"

// atomFeed represents a GitHub releases Atom feed.
type atomFeed struct {
	Entries []atomEntry `xml:"entry"`
}

type atomEntry struct {
	ID      string `xml:"id"`
	Title   string `xml:"title"`
	Updated string `xml:"updated"`
	Content string `xml:"content"`
	Link    struct {
		Href string `xml:"href,attr"`
	} `xml:"link"`
}

// GitHubAtomSource polls a GitHub repository's releases Atom feed.
type GitHubAtomSource struct {
	client     *http.Client
	repository string
	baseURL    string
	sourceID   string
}

func NewGitHubAtomSource(client *http.Client, repository string, sourceID string) *GitHubAtomSource {
	return &GitHubAtomSource{
		client:     client,
		repository: repository,
		baseURL:    defaultGitHubURL,
		sourceID:   sourceID,
	}
}

func (s *GitHubAtomSource) Name() string     { return "github" }
func (s *GitHubAtomSource) SourceID() string  { return s.sourceID }

func (s *GitHubAtomSource) FetchNewReleases(ctx context.Context) ([]IngestionResult, error) {
	url := fmt.Sprintf("%s/%s/releases.atom", s.baseURL, s.repository)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetch atom feed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status: %d", resp.StatusCode)
	}

	var feed atomFeed
	if err := xml.NewDecoder(resp.Body).Decode(&feed); err != nil {
		return nil, fmt.Errorf("decode atom feed: %w", err)
	}

	results := make([]IngestionResult, 0, len(feed.Entries))
	for _, entry := range feed.Entries {
		version := extractVersion(entry.ID)
		if version == "" {
			continue
		}
		ts, _ := time.Parse(time.RFC3339, entry.Updated)
		results = append(results, IngestionResult{
			Repository: s.repository,
			RawVersion: version,
			Changelog:  entry.Content,
			Timestamp:  ts,
		})
	}
	return results, nil
}

// extractVersion extracts the tag name from an Atom entry ID.
// Format: "tag:github.com,2008:Repository/15452919/v1.17.0" -> "v1.17.0"
func extractVersion(id string) string {
	idx := strings.LastIndex(id, "/")
	if idx < 0 || idx == len(id)-1 {
		return ""
	}
	return id[idx+1:]
}
