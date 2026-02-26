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

	"github.com/sentioxyz/changelogue/internal/models"
)

// mockSourcesStore implements SourcesStore for testing.
type mockSourcesStore struct {
	sources   []models.Source
	created   *models.Source
	updated   *models.Source
	deleted   string
	listErr   error
	createErr error
	getErr    error
	updateErr error
	deleteErr error
}

func (m *mockSourcesStore) ListSourcesByProject(_ context.Context, projectID string, page, perPage int) ([]models.Source, int, error) {
	if m.listErr != nil {
		return nil, 0, m.listErr
	}
	return m.sources, len(m.sources), nil
}

func (m *mockSourcesStore) CreateSource(_ context.Context, src *models.Source) error {
	if m.createErr != nil {
		return m.createErr
	}
	src.ID = "src-001"
	src.Enabled = true
	src.CreatedAt = time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	src.UpdatedAt = src.CreatedAt
	m.created = src
	return nil
}

func (m *mockSourcesStore) GetSource(_ context.Context, id string) (*models.Source, error) {
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

func (m *mockSourcesStore) UpdateSource(_ context.Context, id string, src *models.Source) error {
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

func (m *mockSourcesStore) DeleteSource(_ context.Context, id string) error {
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

func setupSourcesMux(store SourcesStore) *http.ServeMux {
	h := NewSourcesHandler(store)
	mux := http.NewServeMux()
	mux.HandleFunc("GET /projects/{projectId}/sources", h.List)
	mux.HandleFunc("POST /projects/{projectId}/sources", h.Create)
	mux.HandleFunc("GET /sources/{id}", h.Get)
	mux.HandleFunc("PUT /sources/{id}", h.Update)
	mux.HandleFunc("DELETE /sources/{id}", h.Delete)
	return mux
}

func TestSourcesHandlerList(t *testing.T) {
	store := &mockSourcesStore{
		sources: []models.Source{
			{ID: "s1", ProjectID: "p1", Provider: "docker_hub", Repository: "library/nginx", Enabled: true, CreatedAt: time.Now(), UpdatedAt: time.Now()},
			{ID: "s2", ProjectID: "p1", Provider: "github", Repository: "golang/go", Enabled: true, CreatedAt: time.Now(), UpdatedAt: time.Now()},
		},
	}
	mux := setupSourcesMux(store)

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/projects/p1/sources", nil)
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

	body := `{"provider":"docker_hub","repository":"library/nginx"}`
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/projects/p1/sources", strings.NewReader(body))
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
	if got.Data.ID != "src-001" {
		t.Fatalf("expected id=src-001, got %s", got.Data.ID)
	}
	if got.Data.Repository != "library/nginx" {
		t.Fatalf("expected repository=library/nginx, got %s", got.Data.Repository)
	}
	if got.Data.ProjectID != "p1" {
		t.Fatalf("expected project_id=p1, got %s", got.Data.ProjectID)
	}
	if store.created == nil {
		t.Fatal("expected store.created to be set")
	}
}

func TestSourcesHandlerCreateMissingProvider(t *testing.T) {
	store := &mockSourcesStore{}
	mux := setupSourcesMux(store)

	body := `{"repository":"library/nginx"}`
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/projects/p1/sources", strings.NewReader(body))
	r.Header.Set("Content-Type", "application/json")
	mux.ServeHTTP(w, r)

	if w.Code != http.StatusUnprocessableEntity {
		t.Fatalf("expected status 422, got %d", w.Code)
	}
}

func TestSourcesHandlerCreateMissingRepository(t *testing.T) {
	store := &mockSourcesStore{}
	mux := setupSourcesMux(store)

	body := `{"provider":"docker_hub"}`
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/projects/p1/sources", strings.NewReader(body))
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
	r := httptest.NewRequest(http.MethodPost, "/projects/p1/sources", strings.NewReader(`{invalid`))
	r.Header.Set("Content-Type", "application/json")
	mux.ServeHTTP(w, r)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d", w.Code)
	}
}

func TestSourcesHandlerGet(t *testing.T) {
	store := &mockSourcesStore{
		sources: []models.Source{
			{ID: "src-42", ProjectID: "p1", Provider: "github", Repository: "golang/go", Enabled: true, CreatedAt: time.Now(), UpdatedAt: time.Now()},
		},
	}
	mux := setupSourcesMux(store)

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/sources/src-42", nil)
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
	if got.Data.ID != "src-42" {
		t.Fatalf("expected id=src-42, got %s", got.Data.ID)
	}
}

func TestSourcesHandlerGetNotFound(t *testing.T) {
	store := &mockSourcesStore{}
	mux := setupSourcesMux(store)

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/sources/nonexistent", nil)
	mux.ServeHTTP(w, r)

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected status 404, got %d", w.Code)
	}
}

func TestSourcesHandlerDelete(t *testing.T) {
	store := &mockSourcesStore{
		sources: []models.Source{
			{ID: "src-5", ProjectID: "p1", Provider: "docker_hub", Repository: "library/redis", Enabled: true, CreatedAt: time.Now(), UpdatedAt: time.Now()},
		},
	}
	mux := setupSourcesMux(store)

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodDelete, "/sources/src-5", nil)
	mux.ServeHTTP(w, r)

	if w.Code != http.StatusNoContent {
		t.Fatalf("expected status 204, got %d", w.Code)
	}
	if store.deleted != "src-5" {
		t.Fatalf("expected deleted=src-5, got %s", store.deleted)
	}
}
