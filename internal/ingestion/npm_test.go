package ingestion

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

const sampleNpmResponse = `{
  "name": "express",
  "time": {
    "created": "2010-12-29T19:38:25.450Z",
    "modified": "2024-10-08T01:09:08.012Z",
    "4.19.2": "2024-03-25T20:08:18.573Z",
    "4.20.0": "2024-09-10T18:30:00.000Z",
    "5.0.0-beta.1": "2024-06-01T12:00:00.000Z"
  }
}`

func TestNpmSourceName(t *testing.T) {
	src := NewNpmSource(http.DefaultClient, "express", "src-id")
	if got := src.Name(); got != "npm" {
		t.Errorf("Name() = %q, want %q", got, "npm")
	}
}

func TestNpmSourceID(t *testing.T) {
	src := NewNpmSource(http.DefaultClient, "express", "src-id")
	if got := src.SourceID(); got != "src-id" {
		t.Errorf("SourceID() = %q, want %q", got, "src-id")
	}
}

func TestNpmFetchNewReleases(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(sampleNpmResponse))
	}))
	defer srv.Close()

	src := NewNpmSource(srv.Client(), "express", "src-id")
	src.baseURL = srv.URL

	results, err := src.FetchNewReleases(context.Background())
	if err != nil {
		t.Fatalf("FetchNewReleases: %v", err)
	}

	// 3 versions: 4.19.2, 4.20.0, 5.0.0-beta.1 — "created" and "modified" are skipped
	if len(results) != 3 {
		t.Fatalf("got %d results, want 3", len(results))
	}

	byVersion := map[string]IngestionResult{}
	for _, r := range results {
		byVersion[r.RawVersion] = r
	}

	// Check 4.19.2
	r, ok := byVersion["4.19.2"]
	if !ok {
		t.Fatal("missing version 4.19.2")
	}
	if r.Repository != "express" {
		t.Errorf("Repository = %q, want %q", r.Repository, "express")
	}
	if r.Timestamp.IsZero() {
		t.Error("Timestamp should not be zero for 4.19.2")
	}
	if r.Metadata["release_url"] == "" {
		t.Error("Metadata[release_url] should not be empty")
	}

	// Check 4.20.0
	if _, ok := byVersion["4.20.0"]; !ok {
		t.Error("missing version 4.20.0")
	}

	// Check prerelease present
	if _, ok := byVersion["5.0.0-beta.1"]; !ok {
		t.Error("missing prerelease version 5.0.0-beta.1")
	}

	// Metadata keys should be skipped
	if _, ok := byVersion["created"]; ok {
		t.Error("'created' metadata key should be excluded")
	}
	if _, ok := byVersion["modified"]; ok {
		t.Error("'modified' metadata key should be excluded")
	}
}

func TestNpmFetchEmpty(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"name":"empty","time":{"created":"2020-01-01T00:00:00.000Z","modified":"2020-01-01T00:00:00.000Z"}}`))
	}))
	defer srv.Close()

	src := NewNpmSource(srv.Client(), "empty", "src-id")
	src.baseURL = srv.URL

	results, err := src.FetchNewReleases(context.Background())
	if err != nil {
		t.Fatalf("FetchNewReleases: %v", err)
	}
	if len(results) != 0 {
		t.Fatalf("got %d results, want 0", len(results))
	}
}

func TestNpmFetchHTTPError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	src := NewNpmSource(srv.Client(), "nonexistent", "src-id")
	src.baseURL = srv.URL

	_, err := src.FetchNewReleases(context.Background())
	if err == nil {
		t.Fatal("expected error for 404 response")
	}
}

// TestNpmLiveExpress hits the real npm registry for the express package.
// Skipped in short mode — run with: go test -v -run TestNpmLive ./internal/ingestion/...
func TestNpmLiveExpress(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping live test in short mode")
	}

	src := NewNpmSource(http.DefaultClient, "express", "live-test")
	results, err := src.FetchNewReleases(context.Background())
	if err != nil {
		t.Fatalf("FetchNewReleases: %v", err)
	}

	if len(results) == 0 {
		t.Fatal("expected at least one release from express")
	}

	t.Logf("got %d releases from express", len(results))
	for i, r := range results {
		if r.RawVersion == "" {
			t.Errorf("results[%d].RawVersion is empty", i)
		}
		if r.Repository != "express" {
			t.Errorf("results[%d].Repository = %q, want %q", i, r.Repository, "express")
		}
		if r.Metadata["release_url"] == "" {
			t.Errorf("results[%d].Metadata[release_url] is empty for version %s", i, r.RawVersion)
		}
	}
}
