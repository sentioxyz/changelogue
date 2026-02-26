package ingestion

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

const sampleAtomFeed = `<?xml version="1.0" encoding="UTF-8"?>
<feed xmlns="http://www.w3.org/2005/Atom">
  <title>Release notes from go-ethereum</title>
  <entry>
    <id>tag:github.com,2008:Repository/15452919/v1.17.0</id>
    <updated>2026-02-17T16:31:00Z</updated>
    <title>Eezo-Inlaid Circuitry (v1.17.0)</title>
    <content type="html">&lt;h2&gt;Changes&lt;/h2&gt;&lt;p&gt;Bug fixes&lt;/p&gt;</content>
    <link rel="alternate" type="text/html" href="https://github.com/ethereum/go-ethereum/releases/tag/v1.17.0"/>
  </entry>
  <entry>
    <id>tag:github.com,2008:Repository/15452919/v1.16.9</id>
    <updated>2026-02-17T16:06:58Z</updated>
    <title>Shield Focusing Module (v1.16.9)</title>
    <content type="html">&lt;p&gt;Patch release&lt;/p&gt;</content>
    <link rel="alternate" type="text/html" href="https://github.com/ethereum/go-ethereum/releases/tag/v1.16.9"/>
  </entry>
</feed>`

func TestGitHubAtomSourceName(t *testing.T) {
	src := NewGitHubAtomSource(http.DefaultClient, "ethereum/go-ethereum", "src-id")
	if got := src.Name(); got != "github" {
		t.Errorf("Name() = %q, want %q", got, "github")
	}
}

func TestGitHubAtomFetchNewReleases(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/atom+xml")
		w.Write([]byte(sampleAtomFeed))
	}))
	defer srv.Close()

	src := NewGitHubAtomSource(srv.Client(), "ethereum/go-ethereum", "src-id")
	src.baseURL = srv.URL

	results, err := src.FetchNewReleases(context.Background())
	if err != nil {
		t.Fatalf("FetchNewReleases: %v", err)
	}

	if len(results) != 2 {
		t.Fatalf("got %d results, want 2", len(results))
	}

	// First entry
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

	// Second entry
	if results[1].RawVersion != "v1.16.9" {
		t.Errorf("results[1].RawVersion = %q, want %q", results[1].RawVersion, "v1.16.9")
	}
}

func TestGitHubAtomFetchEmpty(t *testing.T) {
	emptyFeed := `<?xml version="1.0" encoding="UTF-8"?>
<feed xmlns="http://www.w3.org/2005/Atom">
  <title>Release notes from new-repo</title>
</feed>`

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/atom+xml")
		w.Write([]byte(emptyFeed))
	}))
	defer srv.Close()

	src := NewGitHubAtomSource(srv.Client(), "org/new-repo", "src-id")
	src.baseURL = srv.URL

	results, err := src.FetchNewReleases(context.Background())
	if err != nil {
		t.Fatalf("FetchNewReleases: %v", err)
	}
	if len(results) != 0 {
		t.Fatalf("got %d results, want 0", len(results))
	}
}

// TestGitHubAtomLiveGoEthereum hits the real GitHub Atom feed for ethereum/go-ethereum.
// Skipped in CI — run with: go test -v -run TestGitHubAtomLive ./internal/ingestion/...
func TestGitHubAtomLiveGoEthereum(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping live test in short mode")
	}

	src := NewGitHubAtomSource(http.DefaultClient, "ethereum/go-ethereum", "live-test")
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
		t.Logf("  [%d] %s  released=%s  changelog_len=%d  url=%s",
			i, r.RawVersion, r.Timestamp.Format("2006-01-02"), len(r.Changelog), r.Metadata["release_url"])
	}
}

func TestGitHubAtomFetchHTTPError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	src := NewGitHubAtomSource(srv.Client(), "org/missing", "src-id")
	src.baseURL = srv.URL

	_, err := src.FetchNewReleases(context.Background())
	if err == nil {
		t.Fatal("expected error for 404 response")
	}
}
