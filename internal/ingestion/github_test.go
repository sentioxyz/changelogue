package ingestion

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/sentioxyz/changelogue/internal/githubauth"
)

const sampleGitHubReleases = `[
  {
    "tag_name": "v1.17.0",
    "body": "## Changes\n\nBug fixes and improvements",
    "html_url": "https://github.com/ethereum/go-ethereum/releases/tag/v1.17.0",
    "published_at": "2026-02-17T16:31:00Z",
    "prerelease": false,
    "draft": false
  },
  {
    "tag_name": "v1.17.0-rc.1",
    "body": "Release candidate",
    "html_url": "https://github.com/ethereum/go-ethereum/releases/tag/v1.17.0-rc.1",
    "published_at": "2026-02-15T10:00:00Z",
    "prerelease": true,
    "draft": false
  },
  {
    "tag_name": "v1.18.0-draft",
    "body": "Work in progress",
    "html_url": "https://github.com/ethereum/go-ethereum/releases/tag/v1.18.0-draft",
    "published_at": "2026-02-20T12:00:00Z",
    "prerelease": false,
    "draft": true
  },
  {
    "tag_name": "v1.16.9",
    "body": "Patch release",
    "html_url": "https://github.com/ethereum/go-ethereum/releases/tag/v1.16.9",
    "published_at": "2026-02-17T16:06:58Z",
    "prerelease": false,
    "draft": false
  }
]`

func TestGitHubSourceName(t *testing.T) {
	src := NewGitHubSource(http.DefaultClient, "ethereum/go-ethereum", "src-id")
	if got := src.Name(); got != "github" {
		t.Errorf("Name() = %q, want %q", got, "github")
	}
}

func TestGitHubFetchNewReleases(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(sampleGitHubReleases))
	}))
	defer srv.Close()

	src := NewGitHubSource(srv.Client(), "ethereum/go-ethereum", "src-id")
	src.baseURL = srv.URL

	results, err := src.FetchNewReleases(context.Background())
	if err != nil {
		t.Fatalf("FetchNewReleases: %v", err)
	}

	// Draft should be excluded, so 3 results (v1.17.0, v1.17.0-rc.1, v1.16.9)
	if len(results) != 3 {
		t.Fatalf("got %d results, want 3", len(results))
	}

	// First entry — stable release
	if results[0].RawVersion != "v1.17.0" {
		t.Errorf("results[0].RawVersion = %q, want %q", results[0].RawVersion, "v1.17.0")
	}
	if results[0].Repository != "ethereum/go-ethereum" {
		t.Errorf("results[0].Repository = %q, want %q", results[0].Repository, "ethereum/go-ethereum")
	}
	if results[0].Changelog == "" {
		t.Error("results[0].Changelog should not be empty")
	}
	if results[0].Timestamp.IsZero() {
		t.Error("results[0].Timestamp should not be zero")
	}
	if results[0].Metadata["prerelease"] != "false" {
		t.Errorf("results[0].Metadata[prerelease] = %q, want %q", results[0].Metadata["prerelease"], "false")
	}
	if results[0].Metadata["release_url"] == "" {
		t.Error("results[0].Metadata[release_url] should not be empty")
	}

	// Second entry — prerelease
	if results[1].RawVersion != "v1.17.0-rc.1" {
		t.Errorf("results[1].RawVersion = %q, want %q", results[1].RawVersion, "v1.17.0-rc.1")
	}
	if results[1].Metadata["prerelease"] != "true" {
		t.Errorf("results[1].Metadata[prerelease] = %q, want %q", results[1].Metadata["prerelease"], "true")
	}

	// Third entry
	if results[2].RawVersion != "v1.16.9" {
		t.Errorf("results[2].RawVersion = %q, want %q", results[2].RawVersion, "v1.16.9")
	}
}

func TestGitHubFetchEmpty(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte("[]"))
	}))
	defer srv.Close()

	src := NewGitHubSource(srv.Client(), "org/new-repo", "src-id")
	src.baseURL = srv.URL

	results, err := src.FetchNewReleases(context.Background())
	if err != nil {
		t.Fatalf("FetchNewReleases: %v", err)
	}
	if len(results) != 0 {
		t.Fatalf("got %d results, want 0", len(results))
	}
}

func TestGitHubFetchHTTPError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	src := NewGitHubSource(srv.Client(), "org/missing", "src-id")
	src.baseURL = srv.URL

	_, err := src.FetchNewReleases(context.Background())
	if err == nil {
		t.Fatal("expected error for 404 response")
	}
}

func TestGitHubFetchUsesTokenProvider(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Authorization"); got != "Bearer test-token" {
			t.Fatalf("Authorization = %q, want Bearer test-token", got)
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte("[]"))
	}))
	defer srv.Close()

	src := NewGitHubSourceWithTokenProvider(srv.Client(), "org/private", "src-id", githubauth.NewStaticTokenProvider("test-token"), true)
	src.baseURL = srv.URL

	if _, err := src.FetchNewReleases(context.Background()); err != nil {
		t.Fatalf("FetchNewReleases: %v", err)
	}
}

// TestGitHubLiveGoEthereum hits the real GitHub API for ethereum/go-ethereum.
// Skipped in short mode — run with: go test -v -run TestGitHubLive ./internal/ingestion/...
func TestGitHubLiveGoEthereum(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping live test in short mode")
	}
	if os.Getenv("GITHUB_TOKEN") == "" {
		t.Skip("skipping live test: GITHUB_TOKEN not set")
	}

	src := NewGitHubSource(http.DefaultClient, "ethereum/go-ethereum", "live-test")
	results, err := src.FetchNewReleases(context.Background())
	if err != nil {
		t.Fatalf("FetchNewReleases: %v", err)
	}

	if len(results) == 0 {
		t.Fatal("expected at least one release from go-ethereum")
	}

	t.Logf("got %d releases from ethereum/go-ethereum", len(results))
	for i, r := range results {
		if r.RawVersion == "" {
			t.Errorf("results[%d].RawVersion is empty", i)
		}
		if r.Repository != "ethereum/go-ethereum" {
			t.Errorf("results[%d].Repository = %q, want %q", i, r.Repository, "ethereum/go-ethereum")
		}
		if r.Timestamp.IsZero() {
			t.Errorf("results[%d].Timestamp is zero for version %s", i, r.RawVersion)
		}
		if r.Metadata["release_url"] == "" {
			t.Errorf("results[%d].Metadata[release_url] is empty for version %s", i, r.RawVersion)
		}
		if r.Metadata["prerelease"] == "" {
			t.Errorf("results[%d].Metadata[prerelease] is empty for version %s", i, r.RawVersion)
		}
		t.Logf("  [%d] %s  released=%s  prerelease=%s  changelog_len=%d  url=%s",
			i, r.RawVersion, r.Timestamp.Format("2006-01-02"), r.Metadata["prerelease"], len(r.Changelog), r.Metadata["release_url"])
	}
}

const sampleGitHubTags = `[
  { "name": "v1.17.0", "commit": { "sha": "aaa" } },
  { "name": "v1.16.9", "commit": { "sha": "bbb" } },
  { "name": "v1.16.8", "commit": { "sha": "ccc" } }
]`

// githubMux routes /releases, /tags, and /commits to separate bodies and
// records whether the tags endpoint was hit.
func githubMux(t *testing.T, releasesBody, tagsBody string, tagsHit *bool) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case strings.Contains(r.URL.Path, "/commits/"):
			w.Write([]byte(`{"commit":{"committer":{"date":"2026-06-08T19:15:15Z"}}}`))
		case strings.Contains(r.URL.Path, "/tags"):
			if tagsHit != nil {
				*tagsHit = true
			}
			w.Write([]byte(tagsBody))
		case strings.Contains(r.URL.Path, "/releases"):
			w.Write([]byte(releasesBody))
		default:
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
	}))
}

func TestGitHubReleasesOnlySkipsTags(t *testing.T) {
	tagsHit := false
	srv := githubMux(t, sampleGitHubReleases, sampleGitHubTags, &tagsHit)
	defer srv.Close()

	src := NewGitHubSource(srv.Client(), "ethereum/go-ethereum", "src-id") // releasesOnly = true
	src.baseURL = srv.URL

	results, err := src.FetchNewReleases(context.Background())
	if err != nil {
		t.Fatalf("FetchNewReleases: %v", err)
	}
	if tagsHit {
		t.Error("tags endpoint should not be called when releasesOnly is true")
	}
	if len(results) != 3 { // draft excluded
		t.Fatalf("got %d results, want 3", len(results))
	}
}

func TestGitHubTagDiscoveryMergesAndDedups(t *testing.T) {
	srv := githubMux(t, sampleGitHubReleases, sampleGitHubTags, nil)
	defer srv.Close()

	src := NewGitHubSourceWithTokenProvider(srv.Client(), "ethereum/go-ethereum", "src-id", githubauth.NewStaticTokenProvider(""), false)
	src.baseURL = srv.URL

	results, err := src.FetchNewReleases(context.Background())
	if err != nil {
		t.Fatalf("FetchNewReleases: %v", err)
	}

	// 3 releases (draft excluded) + only v1.16.8 from tags (v1.17.0/v1.16.9 already releases) = 4
	if len(results) != 4 {
		t.Fatalf("got %d results, want 4", len(results))
	}

	byVersion := map[string]IngestionResult{}
	for _, r := range results {
		byVersion[r.RawVersion] = r
	}

	// The release-backed version keeps its changelog and timestamp.
	rel := byVersion["v1.17.0"]
	if rel.Changelog == "" {
		t.Error("release v1.17.0 should keep its changelog (not clobbered by tag)")
	}
	if rel.Timestamp.IsZero() {
		t.Error("release v1.17.0 should keep its timestamp")
	}
	if _, ok := rel.Metadata["source_kind"]; ok {
		t.Error("release v1.17.0 should not be marked source_kind=tag")
	}

	// The tag-only version has no changelog, a zero timestamp, and tag metadata.
	tag, ok := byVersion["v1.16.8"]
	if !ok {
		t.Fatal("expected tag-only version v1.16.8 in results")
	}
	if tag.Changelog != "" {
		t.Errorf("tag v1.16.8 changelog = %q, want empty", tag.Changelog)
	}
	if tag.Timestamp.IsZero() {
		t.Error("tag v1.16.8 timestamp should be backfilled from its commit date")
	}
	if tag.Metadata["source_kind"] != "tag" {
		t.Errorf("tag v1.16.8 source_kind = %q, want tag", tag.Metadata["source_kind"])
	}
	if tag.Metadata["release_url"] != "https://github.com/ethereum/go-ethereum/releases/tag/v1.16.8" {
		t.Errorf("tag v1.16.8 release_url = %q", tag.Metadata["release_url"])
	}
}

func TestGitHubTagDiscoveryTagsOnlyRepo(t *testing.T) {
	srv := githubMux(t, "[]", sampleGitHubTags, nil)
	defer srv.Close()

	src := NewGitHubSourceWithTokenProvider(srv.Client(), "FraxFinance/fraxtal-op-reth", "src-id", githubauth.NewStaticTokenProvider(""), false)
	src.baseURL = srv.URL

	results, err := src.FetchNewReleases(context.Background())
	if err != nil {
		t.Fatalf("FetchNewReleases: %v", err)
	}

	// No releases, so all 3 tags surface.
	if len(results) != 3 {
		t.Fatalf("got %d results, want 3", len(results))
	}
	for _, r := range results {
		if r.Metadata["source_kind"] != "tag" {
			t.Errorf("%s source_kind = %q, want tag", r.RawVersion, r.Metadata["source_kind"])
		}
		if r.Timestamp.IsZero() {
			t.Errorf("%s timestamp should be backfilled from its commit date", r.RawVersion)
		}
	}
}
