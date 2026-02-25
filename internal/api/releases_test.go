package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/sentioxyz/releaseguard/internal/models"
)

// mockReleasesStore implements ReleasesStore for testing.
type mockReleasesStore struct {
	sourceReleases  []models.Release
	projectReleases []models.Release
	release         *models.Release
	listErr         error
	getErr          error
}

func (m *mockReleasesStore) ListReleasesBySource(_ context.Context, sourceID string, page, perPage int) ([]models.Release, int, error) {
	if m.listErr != nil {
		return nil, 0, m.listErr
	}
	return m.sourceReleases, len(m.sourceReleases), nil
}

func (m *mockReleasesStore) ListReleasesByProject(_ context.Context, projectID string, page, perPage int) ([]models.Release, int, error) {
	if m.listErr != nil {
		return nil, 0, m.listErr
	}
	return m.projectReleases, len(m.projectReleases), nil
}

func (m *mockReleasesStore) GetRelease(_ context.Context, id string) (*models.Release, error) {
	if m.getErr != nil {
		return nil, m.getErr
	}
	if m.release != nil && m.release.ID == id {
		return m.release, nil
	}
	return nil, fmt.Errorf("not found")
}

func setupReleasesMux(store ReleasesStore) *http.ServeMux {
	h := NewReleasesHandler(store)
	mux := http.NewServeMux()
	mux.HandleFunc("GET /sources/{id}/releases", h.ListBySource)
	mux.HandleFunc("GET /projects/{projectId}/releases", h.ListByProject)
	mux.HandleFunc("GET /releases/{id}", h.Get)
	return mux
}

func TestReleasesHandlerListBySource(t *testing.T) {
	now := time.Now()
	store := &mockReleasesStore{
		sourceReleases: []models.Release{
			{ID: "aaa-111", SourceID: "s1", Version: "1.25.0", RawData: json.RawMessage(`{}`), CreatedAt: now},
			{ID: "bbb-222", SourceID: "s1", Version: "1.24.0", RawData: json.RawMessage(`{}`), CreatedAt: now},
		},
	}
	mux := setupReleasesMux(store)

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/sources/s1/releases", nil)
	mux.ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", w.Code)
	}

	var got struct {
		Data []models.Release `json:"data"`
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

func TestReleasesHandlerListByProject(t *testing.T) {
	now := time.Now()
	store := &mockReleasesStore{
		projectReleases: []models.Release{
			{ID: "ccc-333", SourceID: "s1", Version: "2.0.0", RawData: json.RawMessage(`{}`), CreatedAt: now},
		},
	}
	mux := setupReleasesMux(store)

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/projects/p1/releases", nil)
	mux.ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", w.Code)
	}

	var got struct {
		Data []models.Release `json:"data"`
		Meta struct {
			Total int `json:"total"`
		} `json:"meta"`
	}
	if err := json.NewDecoder(w.Body).Decode(&got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(got.Data) != 1 {
		t.Fatalf("expected 1 release, got %d", len(got.Data))
	}
}

func TestReleasesHandlerGet(t *testing.T) {
	now := time.Now()
	store := &mockReleasesStore{
		release: &models.Release{
			ID:        "aaa-111",
			SourceID:  "s1",
			Version:   "1.25.0",
			RawData:   json.RawMessage(`{"digest":"sha256:abc"}`),
			CreatedAt: now,
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
		Data models.Release `json:"data"`
	}
	if err := json.NewDecoder(w.Body).Decode(&got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if got.Data.ID != "aaa-111" {
		t.Fatalf("expected id=aaa-111, got %s", got.Data.ID)
	}
	if got.Data.Version != "1.25.0" {
		t.Fatalf("expected version=1.25.0, got %s", got.Data.Version)
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
