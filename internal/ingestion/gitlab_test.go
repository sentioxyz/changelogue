package ingestion

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

const sampleGitLabReleases = `[
  {
    "tag_name": "v1.5.0",
    "description": "## What's New\n\nStability improvements",
    "released_at": "2026-02-20T14:00:00Z",
    "upcoming_release": false,
    "_links": {
      "self": "https://gitlab.com/api/v4/projects/inkscape%2Finkscape/releases/v1.5.0"
    }
  },
  {
    "tag_name": "v1.5.0-rc.1",
    "description": "Release candidate",
    "released_at": "2026-02-18T10:00:00Z",
    "upcoming_release": true,
    "_links": {
      "self": "https://gitlab.com/api/v4/projects/inkscape%2Finkscape/releases/v1.5.0-rc.1"
    }
  },
  {
    "tag_name": "",
    "description": "Empty tag",
    "released_at": "2026-02-17T10:00:00Z",
    "upcoming_release": false,
    "_links": {
      "self": "https://gitlab.com/api/v4/projects/inkscape%2Finkscape/releases/"
    }
  },
  {
    "tag_name": "v1.4.0",
    "description": "Patch release",
    "released_at": "2026-02-15T16:06:58Z",
    "upcoming_release": false,
    "_links": {
      "self": "https://gitlab.com/api/v4/projects/inkscape%2Finkscape/releases/v1.4.0"
    }
  }
]`

func TestGitLabSourceName(t *testing.T) {
	src := NewGitLabSource(http.DefaultClient, "inkscape/inkscape", "src-id")
	if got := src.Name(); got != "gitlab" {
		t.Errorf("Name() = %q, want %q", got, "gitlab")
	}
}

func TestGitLabFetchNewReleases(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(sampleGitLabReleases))
	}))
	defer srv.Close()

	src := NewGitLabSource(srv.Client(), "inkscape/inkscape", "src-id")
	src.baseURL = srv.URL

	results, err := src.FetchNewReleases(context.Background())
	if err != nil {
		t.Fatalf("FetchNewReleases: %v", err)
	}

	// Empty tag_name should be excluded, so 3 results
	if len(results) != 3 {
		t.Fatalf("got %d results, want 3", len(results))
	}

	// First entry — stable release
	if results[0].RawVersion != "v1.5.0" {
		t.Errorf("results[0].RawVersion = %q, want %q", results[0].RawVersion, "v1.5.0")
	}
	if results[0].Repository != "inkscape/inkscape" {
		t.Errorf("results[0].Repository = %q, want %q", results[0].Repository, "inkscape/inkscape")
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

	// Second entry — upcoming/prerelease
	if results[1].RawVersion != "v1.5.0-rc.1" {
		t.Errorf("results[1].RawVersion = %q, want %q", results[1].RawVersion, "v1.5.0-rc.1")
	}
	if results[1].Metadata["prerelease"] != "true" {
		t.Errorf("results[1].Metadata[prerelease] = %q, want %q", results[1].Metadata["prerelease"], "true")
	}

	// Third entry
	if results[2].RawVersion != "v1.4.0" {
		t.Errorf("results[2].RawVersion = %q, want %q", results[2].RawVersion, "v1.4.0")
	}
}

func TestGitLabFetchEmpty(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte("[]"))
	}))
	defer srv.Close()

	src := NewGitLabSource(srv.Client(), "org/new-repo", "src-id")
	src.baseURL = srv.URL

	results, err := src.FetchNewReleases(context.Background())
	if err != nil {
		t.Fatalf("FetchNewReleases: %v", err)
	}
	if len(results) != 0 {
		t.Fatalf("got %d results, want 0", len(results))
	}
}

func TestGitLabFetchHTTPError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	src := NewGitLabSource(srv.Client(), "org/missing", "src-id")
	src.baseURL = srv.URL

	_, err := src.FetchNewReleases(context.Background())
	if err == nil {
		t.Fatal("expected error for 404 response")
	}
}
