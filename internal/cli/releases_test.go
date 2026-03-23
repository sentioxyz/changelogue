package cli

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/sentioxyz/changelogue/internal/models"
)

func TestReleasesListAll(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/releases" {
			t.Errorf("expected /api/v1/releases, got %s", r.URL.Path)
		}
		json.NewEncoder(w).Encode(map[string]any{
			"data": []models.Release{{ID: "r1", Version: "1.0.0"}},
			"meta": map[string]any{"request_id": "r1", "page": 1, "per_page": 25, "total": 1},
		})
	}))
	defer srv.Close()

	c := NewClient(srv.URL, "key")
	releases, _, err := ListReleases(c, "", "", false, 1, 25)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(releases) != 1 || releases[0].Version != "1.0.0" {
		t.Errorf("unexpected releases: %+v", releases)
	}
}

func TestReleasesListBySource(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/sources/s1/releases" {
			t.Errorf("expected /api/v1/sources/s1/releases, got %s", r.URL.Path)
		}
		json.NewEncoder(w).Encode(map[string]any{
			"data": []models.Release{{ID: "r2", Version: "2.0.0", SourceID: "s1"}},
			"meta": map[string]any{"request_id": "r2", "page": 1, "per_page": 25, "total": 1},
		})
	}))
	defer srv.Close()

	c := NewClient(srv.URL, "key")
	releases, _, err := ListReleases(c, "s1", "", false, 1, 25)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(releases) != 1 || releases[0].SourceID != "s1" {
		t.Errorf("unexpected releases: %+v", releases)
	}
}

func TestReleasesListByProject(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/projects/p1/releases" {
			t.Errorf("expected /api/v1/projects/p1/releases, got %s", r.URL.Path)
		}
		json.NewEncoder(w).Encode(map[string]any{
			"data": []models.Release{},
			"meta": map[string]any{"request_id": "r3", "page": 1, "per_page": 25, "total": 0},
		})
	}))
	defer srv.Close()

	c := NewClient(srv.URL, "key")
	releases, _, err := ListReleases(c, "", "p1", false, 1, 25)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(releases) != 0 {
		t.Errorf("expected 0 releases, got %d", len(releases))
	}
}

func TestReleasesGet(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/releases/r1" {
			t.Errorf("expected /api/v1/releases/r1, got %s", r.URL.Path)
		}
		json.NewEncoder(w).Encode(map[string]any{
			"data": models.Release{ID: "r1", Version: "3.0.0"},
			"meta": map[string]any{"request_id": "r4"},
		})
	}))
	defer srv.Close()

	c := NewClient(srv.URL, "key")
	rel, err := GetRelease(c, "r1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rel.Version != "3.0.0" {
		t.Errorf("expected 3.0.0, got %s", rel.Version)
	}
}
