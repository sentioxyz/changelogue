package ingestion

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestDockerHubSourceName(t *testing.T) {
	src := NewDockerHubSource(http.DefaultClient, "library/golang", 0)
	if got := src.Name(); got != "dockerhub" {
		t.Errorf("Name() = %q, want %q", got, "dockerhub")
	}
}

func TestDockerHubFetchNewReleases(t *testing.T) {
	response := `{
		"results": [
			{"name": "1.21.0", "last_updated": "2024-01-15T10:00:00.000000Z"},
			{"name": "1.21.0-rc.1", "last_updated": "2024-01-10T10:00:00.000000Z"}
		]
	}`

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(response))
	}))
	defer srv.Close()

	src := NewDockerHubSource(srv.Client(), "library/golang", 0)
	src.baseURL = srv.URL

	results, err := src.FetchNewReleases(context.Background())
	if err != nil {
		t.Fatalf("FetchNewReleases: %v", err)
	}

	if len(results) != 2 {
		t.Fatalf("got %d results, want 2", len(results))
	}
	if results[0].RawVersion != "1.21.0" {
		t.Errorf("results[0].RawVersion = %q, want %q", results[0].RawVersion, "1.21.0")
	}
	if results[0].Repository != "library/golang" {
		t.Errorf("results[0].Repository = %q, want %q", results[0].Repository, "library/golang")
	}
}

func TestDockerHubFetchEmpty(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"results": []}`))
	}))
	defer srv.Close()

	src := NewDockerHubSource(srv.Client(), "library/golang", 0)
	src.baseURL = srv.URL

	results, err := src.FetchNewReleases(context.Background())
	if err != nil {
		t.Fatalf("FetchNewReleases: %v", err)
	}
	if len(results) != 0 {
		t.Fatalf("got %d results, want 0", len(results))
	}
}
