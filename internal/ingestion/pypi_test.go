package ingestion

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

const samplePyPIResponse = `{
  "info": {
    "name": "requests",
    "summary": "Python HTTP for Humans."
  },
  "releases": {
    "2.31.0": [
      {
        "upload_time_iso_8601": "2023-05-22T15:12:44Z"
      }
    ],
    "2.32.0": [
      {
        "upload_time_iso_8601": "2024-05-20T12:30:00Z"
      },
      {
        "upload_time_iso_8601": "2024-05-20T12:35:00Z"
      }
    ],
    "2.33.0a1": [
      {
        "upload_time_iso_8601": "2024-06-01T10:00:00Z"
      }
    ],
    "0.0.0": []
  }
}`

func TestPyPISourceName(t *testing.T) {
	src := NewPyPISource(http.DefaultClient, "requests", "src-id")
	if got := src.Name(); got != "pypi" {
		t.Errorf("Name() = %q, want %q", got, "pypi")
	}
}

func TestPyPIFetchNewReleases(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(samplePyPIResponse))
	}))
	defer srv.Close()

	src := NewPyPISource(srv.Client(), "requests", "src-id")
	src.baseURL = srv.URL

	results, err := src.FetchNewReleases(context.Background())
	if err != nil {
		t.Fatalf("FetchNewReleases: %v", err)
	}

	// 3 results: 2.31.0, 2.32.0, 2.33.0a1 — 0.0.0 has no files so it's skipped
	if len(results) != 3 {
		t.Fatalf("got %d results, want 3", len(results))
	}

	// Build a map for stable assertions (map iteration order is random)
	byVersion := map[string]IngestionResult{}
	for _, r := range results {
		byVersion[r.RawVersion] = r
	}

	// Check 2.31.0
	r, ok := byVersion["2.31.0"]
	if !ok {
		t.Fatal("missing version 2.31.0")
	}
	if r.Repository != "requests" {
		t.Errorf("Repository = %q, want %q", r.Repository, "requests")
	}
	if r.Timestamp.IsZero() {
		t.Error("Timestamp should not be zero for 2.31.0")
	}
	if r.Metadata["release_url"] == "" {
		t.Error("Metadata[release_url] should not be empty")
	}

	// Check 2.32.0 — should pick earliest upload time
	r2 := byVersion["2.32.0"]
	if r2.Timestamp.IsZero() {
		t.Error("Timestamp should not be zero for 2.32.0")
	}

	// Check prerelease present
	if _, ok := byVersion["2.33.0a1"]; !ok {
		t.Error("missing prerelease version 2.33.0a1")
	}

	// Yanked/empty version should be excluded
	if _, ok := byVersion["0.0.0"]; ok {
		t.Error("version 0.0.0 with no files should be excluded")
	}
}

func TestPyPIFetchEmpty(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"info":{"name":"empty","summary":""},"releases":{}}`))
	}))
	defer srv.Close()

	src := NewPyPISource(srv.Client(), "empty", "src-id")
	src.baseURL = srv.URL

	results, err := src.FetchNewReleases(context.Background())
	if err != nil {
		t.Fatalf("FetchNewReleases: %v", err)
	}
	if len(results) != 0 {
		t.Fatalf("got %d results, want 0", len(results))
	}
}

func TestPyPIFetchHTTPError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	src := NewPyPISource(srv.Client(), "nonexistent", "src-id")
	src.baseURL = srv.URL

	_, err := src.FetchNewReleases(context.Background())
	if err == nil {
		t.Fatal("expected error for 404 response")
	}
}

// TestPyPILiveRequests hits the real PyPI API for the requests package.
// Skipped in short mode — run with: go test -v -run TestPyPILive ./internal/ingestion/...
func TestPyPILiveRequests(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping live test in short mode")
	}

	src := NewPyPISource(http.DefaultClient, "requests", "live-test")
	results, err := src.FetchNewReleases(context.Background())
	if err != nil {
		t.Fatalf("FetchNewReleases: %v", err)
	}

	if len(results) == 0 {
		t.Fatal("expected at least one release from requests")
	}

	t.Logf("got %d releases from requests", len(results))
	for i, r := range results {
		if r.RawVersion == "" {
			t.Errorf("results[%d].RawVersion is empty", i)
		}
		if r.Repository != "requests" {
			t.Errorf("results[%d].Repository = %q, want %q", i, r.Repository, "requests")
		}
		if r.Metadata["release_url"] == "" {
			t.Errorf("results[%d].Metadata[release_url] is empty for version %s", i, r.RawVersion)
		}
	}
}
