package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/sentioxyz/changelogue/internal/models"
)

// mockSemanticReleasesStore implements SemanticReleasesStore for testing.
type mockSemanticReleasesStore struct {
	releases   []models.SemanticRelease
	sources    []models.Release
	listErr    error
	getErr     error
	sourcesErr error
	deleteErr  error
}

func (m *mockSemanticReleasesStore) ListAllSemanticReleases(_ context.Context, page, perPage int) ([]models.SemanticRelease, int, error) {
	if m.listErr != nil {
		return nil, 0, m.listErr
	}
	return m.releases, len(m.releases), nil
}

func (m *mockSemanticReleasesStore) ListSemanticReleases(_ context.Context, projectID string, page, perPage int) ([]models.SemanticRelease, int, error) {
	if m.listErr != nil {
		return nil, 0, m.listErr
	}
	return m.releases, len(m.releases), nil
}

func (m *mockSemanticReleasesStore) GetSemanticRelease(_ context.Context, id string) (*models.SemanticRelease, error) {
	if m.getErr != nil {
		return nil, m.getErr
	}
	for i := range m.releases {
		if m.releases[i].ID == id {
			return &m.releases[i], nil
		}
	}
	return nil, fmt.Errorf("not found")
}

func (m *mockSemanticReleasesStore) GetSemanticReleaseSources(_ context.Context, id string) ([]models.Release, error) {
	if m.sourcesErr != nil {
		return nil, m.sourcesErr
	}
	return m.sources, nil
}

func (m *mockSemanticReleasesStore) DeleteSemanticRelease(_ context.Context, id string) error {
	if m.deleteErr != nil {
		return m.deleteErr
	}
	for i := range m.releases {
		if m.releases[i].ID == id {
			m.releases = append(m.releases[:i], m.releases[i+1:]...)
			return nil
		}
	}
	return fmt.Errorf("not found")
}

func setupSemanticReleasesMux(store SemanticReleasesStore) *http.ServeMux {
	h := NewSemanticReleasesHandler(store)
	mux := http.NewServeMux()
	mux.HandleFunc("GET /semantic-releases", h.ListAll)
	mux.HandleFunc("GET /projects/{projectId}/semantic-releases", h.List)
	mux.HandleFunc("GET /semantic-releases/{id}", h.Get)
	mux.HandleFunc("GET /semantic-releases/{id}/sources", h.ListSources)
	mux.HandleFunc("DELETE /semantic-releases/{id}", h.Delete)
	return mux
}

func TestSemanticReleasesHandlerList(t *testing.T) {
	store := &mockSemanticReleasesStore{
		releases: []models.SemanticRelease{
			{ID: "sr-1", ProjectID: "p1", Version: "1.0.0", Status: "completed", Report: json.RawMessage(`{"summary":"stable release"}`), CreatedAt: time.Now()},
			{ID: "sr-2", ProjectID: "p1", Version: "2.0.0", Status: "pending", CreatedAt: time.Now()},
		},
	}
	mux := setupSemanticReleasesMux(store)

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/projects/p1/semantic-releases", nil)
	mux.ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", w.Code)
	}

	var got struct {
		Data []models.SemanticRelease `json:"data"`
		Meta struct {
			Page    int `json:"page"`
			PerPage int `json:"per_page"`
			Total   int `json:"total"`
		} `json:"meta"`
	}
	if err := json.NewDecoder(w.Body).Decode(&got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(got.Data) != 2 {
		t.Fatalf("expected 2 semantic releases, got %d", len(got.Data))
	}
	if got.Meta.Total != 2 {
		t.Fatalf("expected total=2, got %d", got.Meta.Total)
	}
	if got.Meta.Page != 1 {
		t.Fatalf("expected page=1, got %d", got.Meta.Page)
	}
	if got.Meta.PerPage != 25 {
		t.Fatalf("expected per_page=25, got %d", got.Meta.PerPage)
	}
}

func TestSemanticReleasesHandlerListEmpty(t *testing.T) {
	store := &mockSemanticReleasesStore{}
	mux := setupSemanticReleasesMux(store)

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/projects/p1/semantic-releases", nil)
	mux.ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", w.Code)
	}

	// Verify data is an empty array, not null.
	var raw map[string]json.RawMessage
	if err := json.NewDecoder(w.Body).Decode(&raw); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if string(raw["data"]) != "[]" {
		t.Fatalf("expected data to be empty array [], got %s", string(raw["data"]))
	}
}

func TestSemanticReleasesHandlerListError(t *testing.T) {
	store := &mockSemanticReleasesStore{listErr: fmt.Errorf("db down")}
	mux := setupSemanticReleasesMux(store)

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/projects/p1/semantic-releases", nil)
	mux.ServeHTTP(w, r)

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected status 500, got %d", w.Code)
	}
}

func TestSemanticReleasesHandlerGet(t *testing.T) {
	store := &mockSemanticReleasesStore{
		releases: []models.SemanticRelease{
			{ID: "sr-42", ProjectID: "p1", Version: "3.0.0", Status: "completed", Report: json.RawMessage(`{"summary":"major upgrade"}`), CreatedAt: time.Now()},
		},
	}
	mux := setupSemanticReleasesMux(store)

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/semantic-releases/sr-42", nil)
	mux.ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", w.Code)
	}

	var got struct {
		Data models.SemanticRelease `json:"data"`
	}
	if err := json.NewDecoder(w.Body).Decode(&got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if got.Data.ID != "sr-42" {
		t.Fatalf("expected id=sr-42, got %s", got.Data.ID)
	}
	if got.Data.Version != "3.0.0" {
		t.Fatalf("expected version=3.0.0, got %s", got.Data.Version)
	}
	if got.Data.Status != "completed" {
		t.Fatalf("expected status=completed, got %s", got.Data.Status)
	}
}

func TestSemanticReleasesHandlerGetNotFound(t *testing.T) {
	store := &mockSemanticReleasesStore{}
	mux := setupSemanticReleasesMux(store)

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/semantic-releases/nonexistent", nil)
	mux.ServeHTTP(w, r)

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected status 404, got %d", w.Code)
	}

	var got struct {
		Error struct {
			Code string `json:"code"`
		} `json:"error"`
	}
	if err := json.NewDecoder(w.Body).Decode(&got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if got.Error.Code != "not_found" {
		t.Fatalf("expected error.code=not_found, got %s", got.Error.Code)
	}
}

func TestSemanticReleasesHandlerDelete(t *testing.T) {
	store := &mockSemanticReleasesStore{
		releases: []models.SemanticRelease{
			{ID: "sr-99", ProjectID: "p1", Version: "4.0.0", Status: "completed", CreatedAt: time.Now()},
		},
	}
	mux := setupSemanticReleasesMux(store)

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodDelete, "/semantic-releases/sr-99", nil)
	mux.ServeHTTP(w, r)

	if w.Code != http.StatusNoContent {
		t.Fatalf("expected status 204, got %d", w.Code)
	}
	if len(store.releases) != 0 {
		t.Fatalf("expected release to be removed from store, got %d remaining", len(store.releases))
	}
}

func TestSemanticReleasesHandlerDeleteNotFound(t *testing.T) {
	store := &mockSemanticReleasesStore{}
	mux := setupSemanticReleasesMux(store)

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodDelete, "/semantic-releases/nonexistent", nil)
	mux.ServeHTTP(w, r)

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected status 404, got %d", w.Code)
	}
}

func TestSemanticReleasesHandlerListAll(t *testing.T) {
	store := &mockSemanticReleasesStore{
		releases: []models.SemanticRelease{
			{ID: "sr-1", ProjectID: "p1", Version: "1.0.0", Status: "completed", Report: json.RawMessage(`{"summary":"stable release"}`), CreatedAt: time.Now()},
			{ID: "sr-2", ProjectID: "p2", Version: "2.0.0", Status: "pending", CreatedAt: time.Now()},
		},
	}
	mux := setupSemanticReleasesMux(store)

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/semantic-releases", nil)
	mux.ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", w.Code)
	}

	var got struct {
		Data []models.SemanticRelease `json:"data"`
		Meta struct {
			Page    int `json:"page"`
			PerPage int `json:"per_page"`
			Total   int `json:"total"`
		} `json:"meta"`
	}
	if err := json.NewDecoder(w.Body).Decode(&got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(got.Data) != 2 {
		t.Fatalf("expected 2 semantic releases, got %d", len(got.Data))
	}
	if got.Meta.Total != 2 {
		t.Fatalf("expected total=2, got %d", got.Meta.Total)
	}
	// Verify releases span different projects
	if got.Data[0].ProjectID == got.Data[1].ProjectID {
		t.Fatalf("expected releases from different projects")
	}
}

func TestSemanticReleasesHandlerListSources(t *testing.T) {
	now := time.Now()
	store := &mockSemanticReleasesStore{
		releases: []models.SemanticRelease{
			{ID: "sr-1", ProjectID: "p1", Version: "1.0.0", Status: "completed", CreatedAt: now},
		},
		sources: []models.Release{
			{ID: "r-1", SourceID: "s-1", Version: "v1.16.7", CreatedAt: now},
			{ID: "r-2", SourceID: "s-2", Version: "v1.16.7", CreatedAt: now},
		},
	}
	mux := setupSemanticReleasesMux(store)

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/semantic-releases/sr-1/sources", nil)
	mux.ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", w.Code)
	}

	var got struct {
		Data []models.Release `json:"data"`
	}
	if err := json.NewDecoder(w.Body).Decode(&got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(got.Data) != 2 {
		t.Fatalf("expected 2 source releases, got %d", len(got.Data))
	}
}

func TestSemanticReleasesHandlerListSourcesEmpty(t *testing.T) {
	store := &mockSemanticReleasesStore{
		releases: []models.SemanticRelease{
			{ID: "sr-1", ProjectID: "p1", Version: "1.0.0", Status: "completed", CreatedAt: time.Now()},
		},
	}
	mux := setupSemanticReleasesMux(store)

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/semantic-releases/sr-1/sources", nil)
	mux.ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", w.Code)
	}

	var raw map[string]json.RawMessage
	if err := json.NewDecoder(w.Body).Decode(&raw); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if string(raw["data"]) != "[]" {
		t.Fatalf("expected data to be empty array [], got %s", string(raw["data"]))
	}
}
