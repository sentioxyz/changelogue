package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/sentioxyz/releaseguard/internal/models"
)

// mockSourcesStore implements SourcesStore for testing.
type mockSourcesStore struct {
	sources        []models.Source
	created        *models.Source
	updated        *models.Source
	deleted        int
	latestRelease  *ReleaseView
	versionRelease *ReleaseView
	listErr        error
	createErr      error
	getErr         error
	updateErr      error
	deleteErr      error
	latestErr      error
	versionErr     error
}

func (m *mockSourcesStore) ListSources(_ context.Context, page, perPage int) ([]models.Source, int, error) {
	if m.listErr != nil {
		return nil, 0, m.listErr
	}
	return m.sources, len(m.sources), nil
}

func (m *mockSourcesStore) CreateSource(_ context.Context, src *models.Source) error {
	if m.createErr != nil {
		return m.createErr
	}
	src.ID = 1
	src.Enabled = true
	src.CreatedAt = time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	src.UpdatedAt = src.CreatedAt
	m.created = src
	return nil
}

func (m *mockSourcesStore) GetSource(_ context.Context, id int) (*models.Source, error) {
	if m.getErr != nil {
		return nil, m.getErr
	}
	for i := range m.sources {
		if m.sources[i].ID == id {
			return &m.sources[i], nil
		}
	}
	return nil, fmt.Errorf("not found")
}

func (m *mockSourcesStore) UpdateSource(_ context.Context, id int, src *models.Source) error {
	if m.updateErr != nil {
		return m.updateErr
	}
	for i := range m.sources {
		if m.sources[i].ID == id {
			src.UpdatedAt = time.Date(2026, 2, 1, 0, 0, 0, 0, time.UTC)
			m.updated = src
			return nil
		}
	}
	return fmt.Errorf("not found")
}

func (m *mockSourcesStore) DeleteSource(_ context.Context, id int) error {
	if m.deleteErr != nil {
		return m.deleteErr
	}
	for _, s := range m.sources {
		if s.ID == id {
			m.deleted = id
			return nil
		}
	}
	return fmt.Errorf("not found")
}

func (m *mockSourcesStore) GetLatestRelease(_ context.Context, sourceID int) (*ReleaseView, error) {
	if m.latestErr != nil {
		return nil, m.latestErr
	}
	if m.latestRelease != nil {
		return m.latestRelease, nil
	}
	return nil, fmt.Errorf("not found")
}

func (m *mockSourcesStore) GetReleaseByVersion(_ context.Context, sourceID int, version string) (*ReleaseView, error) {
	if m.versionErr != nil {
		return nil, m.versionErr
	}
	if m.versionRelease != nil {
		return m.versionRelease, nil
	}
	return nil, fmt.Errorf("not found")
}

func setupSourcesMux(store SourcesStore) *http.ServeMux {
	h := NewSourcesHandler(store)
	mux := http.NewServeMux()
	mux.HandleFunc("GET /sources", h.List)
	mux.HandleFunc("POST /sources", h.Create)
	mux.HandleFunc("GET /sources/{id}", h.Get)
	mux.HandleFunc("PUT /sources/{id}", h.Update)
	mux.HandleFunc("DELETE /sources/{id}", h.Delete)
	mux.HandleFunc("GET /sources/{id}/latest-release", h.LatestRelease)
	mux.HandleFunc("GET /sources/{id}/releases/{version}", h.ReleaseByVersion)
	return mux
}

func TestSourcesHandlerList(t *testing.T) {
	store := &mockSourcesStore{
		sources: []models.Source{
			{ID: 1, ProjectID: 1, SourceType: "docker_hub", Repository: "library/nginx", Enabled: true, CreatedAt: time.Now(), UpdatedAt: time.Now()},
			{ID: 2, ProjectID: 1, SourceType: "github", Repository: "golang/go", Enabled: true, CreatedAt: time.Now(), UpdatedAt: time.Now()},
		},
	}
	mux := setupSourcesMux(store)

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/sources", nil)
	mux.ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", w.Code)
	}

	var got struct {
		Data []models.Source `json:"data"`
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
		t.Fatalf("expected 2 sources, got %d", len(got.Data))
	}
	if got.Meta.Total != 2 {
		t.Fatalf("expected total=2, got %d", got.Meta.Total)
	}
}

func TestSourcesHandlerCreate(t *testing.T) {
	store := &mockSourcesStore{}
	mux := setupSourcesMux(store)

	body := `{"project_id":1,"type":"docker_hub","repository":"library/nginx"}`
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/sources", strings.NewReader(body))
	r.Header.Set("Content-Type", "application/json")
	mux.ServeHTTP(w, r)

	if w.Code != http.StatusCreated {
		t.Fatalf("expected status 201, got %d; body: %s", w.Code, w.Body.String())
	}

	var got struct {
		Data models.Source `json:"data"`
	}
	if err := json.NewDecoder(w.Body).Decode(&got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if got.Data.ID != 1 {
		t.Fatalf("expected id=1, got %d", got.Data.ID)
	}
	if got.Data.Repository != "library/nginx" {
		t.Fatalf("expected repository=library/nginx, got %s", got.Data.Repository)
	}
	if store.created == nil {
		t.Fatal("expected store.created to be set")
	}
}

func TestSourcesHandlerCreateMissingProjectID(t *testing.T) {
	store := &mockSourcesStore{}
	mux := setupSourcesMux(store)

	body := `{"type":"docker_hub","repository":"library/nginx"}`
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/sources", strings.NewReader(body))
	r.Header.Set("Content-Type", "application/json")
	mux.ServeHTTP(w, r)

	if w.Code != http.StatusUnprocessableEntity {
		t.Fatalf("expected status 422, got %d", w.Code)
	}
}

func TestSourcesHandlerCreateMissingType(t *testing.T) {
	store := &mockSourcesStore{}
	mux := setupSourcesMux(store)

	body := `{"project_id":1,"repository":"library/nginx"}`
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/sources", strings.NewReader(body))
	r.Header.Set("Content-Type", "application/json")
	mux.ServeHTTP(w, r)

	if w.Code != http.StatusUnprocessableEntity {
		t.Fatalf("expected status 422, got %d", w.Code)
	}
}

func TestSourcesHandlerCreateMissingRepository(t *testing.T) {
	store := &mockSourcesStore{}
	mux := setupSourcesMux(store)

	body := `{"project_id":1,"type":"docker_hub"}`
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/sources", strings.NewReader(body))
	r.Header.Set("Content-Type", "application/json")
	mux.ServeHTTP(w, r)

	if w.Code != http.StatusUnprocessableEntity {
		t.Fatalf("expected status 422, got %d", w.Code)
	}
}

func TestSourcesHandlerCreateInvalidJSON(t *testing.T) {
	store := &mockSourcesStore{}
	mux := setupSourcesMux(store)

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/sources", strings.NewReader(`{invalid`))
	r.Header.Set("Content-Type", "application/json")
	mux.ServeHTTP(w, r)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d", w.Code)
	}
}

func TestSourcesHandlerGet(t *testing.T) {
	store := &mockSourcesStore{
		sources: []models.Source{
			{ID: 42, ProjectID: 1, SourceType: "github", Repository: "golang/go", Enabled: true, CreatedAt: time.Now(), UpdatedAt: time.Now()},
		},
	}
	mux := setupSourcesMux(store)

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/sources/42", nil)
	mux.ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", w.Code)
	}

	var got struct {
		Data models.Source `json:"data"`
	}
	if err := json.NewDecoder(w.Body).Decode(&got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if got.Data.ID != 42 {
		t.Fatalf("expected id=42, got %d", got.Data.ID)
	}
}

func TestSourcesHandlerGetNotFound(t *testing.T) {
	store := &mockSourcesStore{}
	mux := setupSourcesMux(store)

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/sources/999", nil)
	mux.ServeHTTP(w, r)

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected status 404, got %d", w.Code)
	}
}

func TestSourcesHandlerDelete(t *testing.T) {
	store := &mockSourcesStore{
		sources: []models.Source{
			{ID: 5, ProjectID: 1, SourceType: "docker_hub", Repository: "library/redis", Enabled: true, CreatedAt: time.Now(), UpdatedAt: time.Now()},
		},
	}
	mux := setupSourcesMux(store)

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodDelete, "/sources/5", nil)
	mux.ServeHTTP(w, r)

	if w.Code != http.StatusNoContent {
		t.Fatalf("expected status 204, got %d", w.Code)
	}
	if store.deleted != 5 {
		t.Fatalf("expected deleted=5, got %d", store.deleted)
	}
}

func TestSourcesHandlerLatestRelease(t *testing.T) {
	store := &mockSourcesStore{
		latestRelease: &ReleaseView{
			ID:             "aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee",
			SourceID:       1,
			SourceType:     "docker_hub",
			Repository:     "library/nginx",
			ProjectID:      1,
			ProjectName:    "infra",
			RawVersion:     "1.25.0",
			IsPreRelease:   false,
			PipelineStatus: "completed",
			CreatedAt:      "2026-01-15T00:00:00Z",
		},
	}
	mux := setupSourcesMux(store)

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/sources/1/latest-release", nil)
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
	if got.Data.RawVersion != "1.25.0" {
		t.Fatalf("expected version=1.25.0, got %s", got.Data.RawVersion)
	}
}

func TestSourcesHandlerLatestReleaseNotFound(t *testing.T) {
	store := &mockSourcesStore{}
	mux := setupSourcesMux(store)

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/sources/999/latest-release", nil)
	mux.ServeHTTP(w, r)

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected status 404, got %d", w.Code)
	}
}

func TestSourcesHandlerReleaseByVersion(t *testing.T) {
	store := &mockSourcesStore{
		versionRelease: &ReleaseView{
			ID:             "aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee",
			SourceID:       1,
			SourceType:     "docker_hub",
			Repository:     "library/nginx",
			ProjectID:      1,
			ProjectName:    "infra",
			RawVersion:     "1.24.0",
			IsPreRelease:   false,
			PipelineStatus: "completed",
			CreatedAt:      "2026-01-10T00:00:00Z",
		},
	}
	mux := setupSourcesMux(store)

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/sources/1/releases/1.24.0", nil)
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
	if got.Data.RawVersion != "1.24.0" {
		t.Fatalf("expected version=1.24.0, got %s", got.Data.RawVersion)
	}
}

func TestSourcesHandlerReleaseByVersionNotFound(t *testing.T) {
	store := &mockSourcesStore{}
	mux := setupSourcesMux(store)

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/sources/1/releases/99.0.0", nil)
	mux.ServeHTTP(w, r)

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected status 404, got %d", w.Code)
	}
}
