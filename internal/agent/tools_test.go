package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/sentioxyz/changelogue/internal/models"
)

// mockDataStore implements AgentDataStore for testing.
type mockDataStore struct {
	releases       []models.Release
	releasesTotal  int
	releaseByID    map[string]*models.Release
	contextSources []models.ContextSource
	contextTotal   int
	err            error
}

func (m *mockDataStore) ListReleasesByProject(_ context.Context, _ string, page, perPage int, includeExcluded bool, filter models.ReleaseFilter) ([]models.Release, int, error) {
	if m.err != nil {
		return nil, 0, m.err
	}
	return m.releases, m.releasesTotal, nil
}

func (m *mockDataStore) GetRelease(_ context.Context, id string) (*models.Release, error) {
	if m.err != nil {
		return nil, m.err
	}
	if r, ok := m.releaseByID[id]; ok {
		return r, nil
	}
	return nil, fmt.Errorf("not found")
}

func (m *mockDataStore) ListContextSources(_ context.Context, _ string, page, perPage int) ([]models.ContextSource, int, error) {
	if m.err != nil {
		return nil, 0, m.err
	}
	return m.contextSources, m.contextTotal, nil
}

func (m *mockDataStore) ListSourcesByProject(_ context.Context, _ string, page, perPage int) ([]models.Source, int, error) {
	return nil, 0, nil
}

func TestNewTools(t *testing.T) {
	store := &mockDataStore{}
	tools, err := NewTools(store, "proj-1")
	if err != nil {
		t.Fatalf("NewTools returned error: %v", err)
	}
	if len(tools) != 3 {
		t.Fatalf("expected 3 tools, got %d", len(tools))
	}

	expectedNames := map[string]bool{
		"get_releases":         false,
		"get_release_detail":   false,
		"list_context_sources": false,
	}
	for _, tool := range tools {
		if _, ok := expectedNames[tool.Name()]; !ok {
			t.Errorf("unexpected tool name: %s", tool.Name())
		}
		expectedNames[tool.Name()] = true
	}
	for name, found := range expectedNames {
		if !found {
			t.Errorf("expected tool %q not found", name)
		}
	}
}

func TestGetReleases(t *testing.T) {
	now := time.Now()
	releasedAt := now.Add(-24 * time.Hour)
	store := &mockDataStore{
		releases: []models.Release{
			{
				ID:         "rel-1",
				SourceID:   "src-1",
				Version:    "v1.2.0",
				RawData:    json.RawMessage(`{"notes": "bug fixes"}`),
				ReleasedAt: &releasedAt,
				CreatedAt:  now,
			},
			{
				ID:        "rel-2",
				SourceID:  "src-1",
				Version:   "v1.1.0",
				RawData:   json.RawMessage(`{}`),
				CreatedAt: now.Add(-48 * time.Hour),
			},
		},
		releasesTotal: 2,
	}

	f := &toolFactory{store: store, projectID: "proj-1"}

	t.Run("default pagination", func(t *testing.T) {
		out, err := f.getReleases(nil, GetReleasesInput{})
		if err != nil {
			t.Fatalf("getReleases error: %v", err)
		}
		if out.Total != 2 {
			t.Errorf("expected total=2, got %d", out.Total)
		}
		if out.Page != 1 {
			t.Errorf("expected page=1, got %d", out.Page)
		}
		if out.PerPage != 20 {
			t.Errorf("expected perPage=20, got %d", out.PerPage)
		}
		if len(out.Releases) != 2 {
			t.Fatalf("expected 2 releases, got %d", len(out.Releases))
		}
		if out.Releases[0].Version != "v1.2.0" {
			t.Errorf("expected version v1.2.0, got %s", out.Releases[0].Version)
		}
		if out.Releases[0].ReleasedAt == "" {
			t.Error("expected released_at to be set for first release")
		}
		if out.Releases[1].ReleasedAt != "" {
			t.Error("expected released_at to be empty for second release")
		}
	})

	t.Run("custom pagination", func(t *testing.T) {
		out, err := f.getReleases(nil, GetReleasesInput{Page: 2, PerPage: 10})
		if err != nil {
			t.Fatalf("getReleases error: %v", err)
		}
		if out.Page != 2 {
			t.Errorf("expected page=2, got %d", out.Page)
		}
		if out.PerPage != 10 {
			t.Errorf("expected perPage=10, got %d", out.PerPage)
		}
	})

	t.Run("clamps perPage", func(t *testing.T) {
		out, err := f.getReleases(nil, GetReleasesInput{PerPage: 100})
		if err != nil {
			t.Fatalf("getReleases error: %v", err)
		}
		if out.PerPage != 20 {
			t.Errorf("expected perPage clamped to 20, got %d", out.PerPage)
		}
	})

	t.Run("store error", func(t *testing.T) {
		errStore := &mockDataStore{err: fmt.Errorf("db down")}
		ef := &toolFactory{store: errStore, projectID: "proj-1"}
		_, err := ef.getReleases(nil, GetReleasesInput{})
		if err == nil {
			t.Fatal("expected error, got nil")
		}
	})
}

func TestGetReleaseDetail(t *testing.T) {
	now := time.Now()
	releasedAt := now.Add(-24 * time.Hour)
	store := &mockDataStore{
		releaseByID: map[string]*models.Release{
			"rel-1": {
				ID:         "rel-1",
				SourceID:   "src-1",
				Version:    "v2.0.0",
				RawData:    json.RawMessage(`{"body": "major release"}`),
				ReleasedAt: &releasedAt,
				CreatedAt:  now,
			},
		},
	}
	f := &toolFactory{store: store, projectID: "proj-1"}

	t.Run("success", func(t *testing.T) {
		out, err := f.getReleaseDetail(nil, GetReleaseDetailInput{ReleaseID: "rel-1"})
		if err != nil {
			t.Fatalf("getReleaseDetail error: %v", err)
		}
		if out.ID != "rel-1" {
			t.Errorf("expected id=rel-1, got %s", out.ID)
		}
		if out.Version != "v2.0.0" {
			t.Errorf("expected version=v2.0.0, got %s", out.Version)
		}
		if out.ReleasedAt == "" {
			t.Error("expected released_at to be set")
		}
	})

	t.Run("empty release_id", func(t *testing.T) {
		_, err := f.getReleaseDetail(nil, GetReleaseDetailInput{})
		if err == nil {
			t.Fatal("expected error for empty release_id")
		}
	})

	t.Run("not found", func(t *testing.T) {
		_, err := f.getReleaseDetail(nil, GetReleaseDetailInput{ReleaseID: "nonexistent"})
		if err == nil {
			t.Fatal("expected error for nonexistent release")
		}
	})
}

func TestListContextSources(t *testing.T) {
	store := &mockDataStore{
		contextSources: []models.ContextSource{
			{
				ID:     "cs-1",
				Type:   "url",
				Name:   "Runbook",
				Config: json.RawMessage(`{"url": "https://wiki.example.com/runbook"}`),
			},
			{
				ID:     "cs-2",
				Type:   "github_repo",
				Name:   "Source Code",
				Config: json.RawMessage(`{"repo": "org/repo"}`),
			},
		},
		contextTotal: 2,
	}
	f := &toolFactory{store: store, projectID: "proj-1"}

	t.Run("success", func(t *testing.T) {
		out, err := f.listContextSources(nil, ListContextSourcesInput{})
		if err != nil {
			t.Fatalf("listContextSources error: %v", err)
		}
		if out.Total != 2 {
			t.Errorf("expected total=2, got %d", out.Total)
		}
		if len(out.Sources) != 2 {
			t.Fatalf("expected 2 sources, got %d", len(out.Sources))
		}
		if out.Sources[0].Name != "Runbook" {
			t.Errorf("expected name=Runbook, got %s", out.Sources[0].Name)
		}
		if out.Sources[1].Type != "github_repo" {
			t.Errorf("expected type=github_repo, got %s", out.Sources[1].Type)
		}
	})

	t.Run("store error", func(t *testing.T) {
		errStore := &mockDataStore{err: fmt.Errorf("db down")}
		ef := &toolFactory{store: errStore, projectID: "proj-1"}
		_, err := ef.listContextSources(nil, ListContextSourcesInput{})
		if err == nil {
			t.Fatal("expected error, got nil")
		}
	})
}
