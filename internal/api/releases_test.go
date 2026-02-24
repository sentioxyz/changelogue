package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// mockReleasesStore implements ReleasesStore for testing.
type mockReleasesStore struct {
	releases   []ReleaseView
	release    *ReleaseView
	notes      string
	pipeline   *PipelineStatus
	listErr    error
	getErr     error
	notesErr   error
	pipeErr    error
}

func (m *mockReleasesStore) ListReleases(_ context.Context, opts ListReleasesOpts) ([]ReleaseView, int, error) {
	if m.listErr != nil {
		return nil, 0, m.listErr
	}
	return m.releases, len(m.releases), nil
}

func (m *mockReleasesStore) GetRelease(_ context.Context, id string) (*ReleaseView, error) {
	if m.getErr != nil {
		return nil, m.getErr
	}
	if m.release != nil && m.release.ID == id {
		return m.release, nil
	}
	return nil, fmt.Errorf("not found")
}

func (m *mockReleasesStore) GetReleaseNotes(_ context.Context, id string) (string, error) {
	if m.notesErr != nil {
		return "", m.notesErr
	}
	return m.notes, nil
}

func (m *mockReleasesStore) GetPipelineStatus(_ context.Context, releaseID string) (*PipelineStatus, error) {
	if m.pipeErr != nil {
		return nil, m.pipeErr
	}
	if m.pipeline != nil {
		return m.pipeline, nil
	}
	return nil, fmt.Errorf("not found")
}

func setupReleasesMux(store ReleasesStore) *http.ServeMux {
	h := NewReleasesHandler(store)
	mux := http.NewServeMux()
	mux.HandleFunc("GET /releases", h.List)
	mux.HandleFunc("GET /releases/{id}", h.Get)
	mux.HandleFunc("GET /releases/{id}/pipeline", h.Pipeline)
	mux.HandleFunc("GET /releases/{id}/notes", h.Notes)
	return mux
}

func TestReleasesHandlerList(t *testing.T) {
	store := &mockReleasesStore{
		releases: []ReleaseView{
			{ID: "aaa-111", SourceID: 1, SourceType: "docker_hub", Repository: "library/nginx", ProjectID: 1, ProjectName: "infra", RawVersion: "1.25.0", PipelineStatus: "completed", CreatedAt: "2026-01-15T00:00:00Z"},
			{ID: "bbb-222", SourceID: 2, SourceType: "github", Repository: "golang/go", ProjectID: 1, ProjectName: "infra", RawVersion: "1.22.0", PipelineStatus: "pending", CreatedAt: "2026-01-14T00:00:00Z"},
		},
	}
	mux := setupReleasesMux(store)

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/releases", nil)
	mux.ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", w.Code)
	}

	var got struct {
		Data []ReleaseView `json:"data"`
		Meta struct {
			Total int `json:"total"`
		} `json:"meta"`
	}
	if err := json.NewDecoder(w.Body).Decode(&got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(got.Data) != 2 {
		t.Fatalf("expected 2 releases, got %d", len(got.Data))
	}
	if got.Meta.Total != 2 {
		t.Fatalf("expected total=2, got %d", got.Meta.Total)
	}
}

func TestReleasesHandlerListWithFilters(t *testing.T) {
	store := &mockReleasesStore{
		releases: []ReleaseView{
			{ID: "aaa-111", SourceID: 1, ProjectID: 2, RawVersion: "1.0.0", PipelineStatus: "completed", CreatedAt: "2026-01-15T00:00:00Z"},
		},
	}
	mux := setupReleasesMux(store)

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/releases?project_id=2&source_id=1&sort=created_at&order=asc", nil)
	mux.ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", w.Code)
	}
}

func TestReleasesHandlerGet(t *testing.T) {
	store := &mockReleasesStore{
		release: &ReleaseView{
			ID:             "aaa-111",
			SourceID:       1,
			SourceType:     "docker_hub",
			Repository:     "library/nginx",
			ProjectID:      1,
			ProjectName:    "infra",
			RawVersion:     "1.25.0",
			PipelineStatus: "completed",
			CreatedAt:      "2026-01-15T00:00:00Z",
		},
	}
	mux := setupReleasesMux(store)

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/releases/aaa-111", nil)
	mux.ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", w.Code)
	}

	var got struct {
		Data ReleaseView `json:"data"`
	}
	if err := json.NewDecoder(w.Body).Decode(&got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if got.Data.ID != "aaa-111" {
		t.Fatalf("expected id=aaa-111, got %s", got.Data.ID)
	}
	if got.Data.RawVersion != "1.25.0" {
		t.Fatalf("expected version=1.25.0, got %s", got.Data.RawVersion)
	}
}

func TestReleasesHandlerGetNotFound(t *testing.T) {
	store := &mockReleasesStore{}
	mux := setupReleasesMux(store)

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/releases/nonexistent-id", nil)
	mux.ServeHTTP(w, r)

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected status 404, got %d", w.Code)
	}
}

func TestReleasesHandlerPipeline(t *testing.T) {
	now := time.Now()
	node := "urgency_scorer"
	store := &mockReleasesStore{
		pipeline: &PipelineStatus{
			ReleaseID:   "aaa-111",
			State:       "running",
			CurrentNode: &node,
			NodeResults: json.RawMessage(`{"regex_normalizer":"passed"}`),
			Attempt:     1,
			CompletedAt: &now,
		},
	}
	mux := setupReleasesMux(store)

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/releases/aaa-111/pipeline", nil)
	mux.ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", w.Code)
	}

	var got struct {
		Data PipelineStatus `json:"data"`
	}
	if err := json.NewDecoder(w.Body).Decode(&got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if got.Data.State != "running" {
		t.Fatalf("expected state=running, got %s", got.Data.State)
	}
	if *got.Data.CurrentNode != "urgency_scorer" {
		t.Fatalf("expected current_node=urgency_scorer, got %s", *got.Data.CurrentNode)
	}
}

func TestReleasesHandlerPipelineNotFound(t *testing.T) {
	store := &mockReleasesStore{}
	mux := setupReleasesMux(store)

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/releases/nonexistent/pipeline", nil)
	mux.ServeHTTP(w, r)

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected status 404, got %d", w.Code)
	}
}

func TestReleasesHandlerNotes(t *testing.T) {
	store := &mockReleasesStore{
		notes: "## Changelog\n- Fixed bug #123\n- Added feature X",
	}
	mux := setupReleasesMux(store)

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/releases/aaa-111/notes", nil)
	mux.ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", w.Code)
	}

	var got struct {
		Data struct {
			Changelog string `json:"changelog"`
		} `json:"data"`
	}
	if err := json.NewDecoder(w.Body).Decode(&got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if got.Data.Changelog != "## Changelog\n- Fixed bug #123\n- Added feature X" {
		t.Fatalf("unexpected changelog: %s", got.Data.Changelog)
	}
}
