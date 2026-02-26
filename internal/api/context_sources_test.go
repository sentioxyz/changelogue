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

// mockContextSourcesStore implements ContextSourcesStore for testing.
type mockContextSourcesStore struct {
	sources   []models.ContextSource
	created   *models.ContextSource
	updated   *models.ContextSource
	deleted   string
	listErr   error
	createErr error
	getErr    error
	updateErr error
	deleteErr error
}

func (m *mockContextSourcesStore) ListContextSources(_ context.Context, projectID string, page, perPage int) ([]models.ContextSource, int, error) {
	if m.listErr != nil {
		return nil, 0, m.listErr
	}
	return m.sources, len(m.sources), nil
}

func (m *mockContextSourcesStore) CreateContextSource(_ context.Context, cs *models.ContextSource) error {
	if m.createErr != nil {
		return m.createErr
	}
	cs.ID = "cs-001"
	cs.CreatedAt = time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	cs.UpdatedAt = cs.CreatedAt
	m.created = cs
	return nil
}

func (m *mockContextSourcesStore) GetContextSource(_ context.Context, id string) (*models.ContextSource, error) {
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

func (m *mockContextSourcesStore) UpdateContextSource(_ context.Context, id string, cs *models.ContextSource) error {
	if m.updateErr != nil {
		return m.updateErr
	}
	for i := range m.sources {
		if m.sources[i].ID == id {
			cs.UpdatedAt = time.Date(2026, 2, 1, 0, 0, 0, 0, time.UTC)
			m.updated = cs
			return nil
		}
	}
	return fmt.Errorf("not found")
}

func (m *mockContextSourcesStore) DeleteContextSource(_ context.Context, id string) error {
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

func setupContextSourcesMux(store ContextSourcesStore) *http.ServeMux {
	h := NewContextSourcesHandler(store)
	mux := http.NewServeMux()
	mux.HandleFunc("GET /projects/{projectId}/context-sources", h.List)
	mux.HandleFunc("POST /projects/{projectId}/context-sources", h.Create)
	mux.HandleFunc("GET /context-sources/{id}", h.Get)
	mux.HandleFunc("PUT /context-sources/{id}", h.Update)
	mux.HandleFunc("DELETE /context-sources/{id}", h.Delete)
	return mux
}

func TestContextSourcesHandlerList(t *testing.T) {
	store := &mockContextSourcesStore{
		sources: []models.ContextSource{
			{ID: "cs-1", ProjectID: "p1", Type: "changelog_url", Name: "nginx changelog", Config: json.RawMessage(`{"url":"https://example.com/changelog"}`), CreatedAt: time.Now(), UpdatedAt: time.Now()},
			{ID: "cs-2", ProjectID: "p1", Type: "git_repo", Name: "nginx repo", Config: json.RawMessage(`{"repo":"nginx/nginx"}`), CreatedAt: time.Now(), UpdatedAt: time.Now()},
		},
	}
	mux := setupContextSourcesMux(store)

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/projects/p1/context-sources", nil)
	mux.ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", w.Code)
	}

	var got struct {
		Data []models.ContextSource `json:"data"`
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
		t.Fatalf("expected 2 context sources, got %d", len(got.Data))
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

func TestContextSourcesHandlerListEmpty(t *testing.T) {
	store := &mockContextSourcesStore{}
	mux := setupContextSourcesMux(store)

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/projects/p1/context-sources", nil)
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

func TestContextSourcesHandlerListError(t *testing.T) {
	store := &mockContextSourcesStore{listErr: fmt.Errorf("db down")}
	mux := setupContextSourcesMux(store)

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/projects/p1/context-sources", nil)
	mux.ServeHTTP(w, r)

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected status 500, got %d", w.Code)
	}
}

func TestContextSourcesHandlerCreate(t *testing.T) {
	store := &mockContextSourcesStore{}
	mux := setupContextSourcesMux(store)

	body := `{"type":"changelog_url","name":"nginx changelog","config":{"url":"https://example.com/changelog"}}`
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/projects/p1/context-sources", strings.NewReader(body))
	r.Header.Set("Content-Type", "application/json")
	mux.ServeHTTP(w, r)

	if w.Code != http.StatusCreated {
		t.Fatalf("expected status 201, got %d; body: %s", w.Code, w.Body.String())
	}

	var got struct {
		Data models.ContextSource `json:"data"`
	}
	if err := json.NewDecoder(w.Body).Decode(&got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if got.Data.ID != "cs-001" {
		t.Fatalf("expected id=cs-001, got %s", got.Data.ID)
	}
	if got.Data.Name != "nginx changelog" {
		t.Fatalf("expected name=nginx changelog, got %s", got.Data.Name)
	}
	if got.Data.Type != "changelog_url" {
		t.Fatalf("expected type=changelog_url, got %s", got.Data.Type)
	}
	if got.Data.ProjectID != "p1" {
		t.Fatalf("expected project_id=p1, got %s", got.Data.ProjectID)
	}
	if store.created == nil {
		t.Fatal("expected store.created to be set")
	}
}

func TestContextSourcesHandlerCreateMissingType(t *testing.T) {
	store := &mockContextSourcesStore{}
	mux := setupContextSourcesMux(store)

	body := `{"name":"nginx changelog","config":{"url":"https://example.com/changelog"}}`
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/projects/p1/context-sources", strings.NewReader(body))
	r.Header.Set("Content-Type", "application/json")
	mux.ServeHTTP(w, r)

	if w.Code != http.StatusUnprocessableEntity {
		t.Fatalf("expected status 422, got %d", w.Code)
	}
}

func TestContextSourcesHandlerCreateMissingName(t *testing.T) {
	store := &mockContextSourcesStore{}
	mux := setupContextSourcesMux(store)

	body := `{"type":"changelog_url","config":{"url":"https://example.com/changelog"}}`
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/projects/p1/context-sources", strings.NewReader(body))
	r.Header.Set("Content-Type", "application/json")
	mux.ServeHTTP(w, r)

	if w.Code != http.StatusUnprocessableEntity {
		t.Fatalf("expected status 422, got %d", w.Code)
	}
}

func TestContextSourcesHandlerCreateMissingConfig(t *testing.T) {
	store := &mockContextSourcesStore{}
	mux := setupContextSourcesMux(store)

	body := `{"type":"changelog_url","name":"nginx changelog"}`
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/projects/p1/context-sources", strings.NewReader(body))
	r.Header.Set("Content-Type", "application/json")
	mux.ServeHTTP(w, r)

	if w.Code != http.StatusUnprocessableEntity {
		t.Fatalf("expected status 422, got %d", w.Code)
	}
}

func TestContextSourcesHandlerCreateInvalidJSON(t *testing.T) {
	store := &mockContextSourcesStore{}
	mux := setupContextSourcesMux(store)

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/projects/p1/context-sources", strings.NewReader(`{invalid`))
	r.Header.Set("Content-Type", "application/json")
	mux.ServeHTTP(w, r)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d", w.Code)
	}
}

func TestContextSourcesHandlerGet(t *testing.T) {
	store := &mockContextSourcesStore{
		sources: []models.ContextSource{
			{ID: "cs-42", ProjectID: "p1", Type: "changelog_url", Name: "nginx changelog", Config: json.RawMessage(`{"url":"https://example.com/changelog"}`), CreatedAt: time.Now(), UpdatedAt: time.Now()},
		},
	}
	mux := setupContextSourcesMux(store)

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/context-sources/cs-42", nil)
	mux.ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", w.Code)
	}

	var got struct {
		Data models.ContextSource `json:"data"`
	}
	if err := json.NewDecoder(w.Body).Decode(&got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if got.Data.ID != "cs-42" {
		t.Fatalf("expected id=cs-42, got %s", got.Data.ID)
	}
	if got.Data.Name != "nginx changelog" {
		t.Fatalf("expected name=nginx changelog, got %s", got.Data.Name)
	}
}

func TestContextSourcesHandlerGetNotFound(t *testing.T) {
	store := &mockContextSourcesStore{}
	mux := setupContextSourcesMux(store)

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/context-sources/nonexistent", nil)
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

func TestContextSourcesHandlerUpdate(t *testing.T) {
	store := &mockContextSourcesStore{
		sources: []models.ContextSource{
			{ID: "cs-10", ProjectID: "p1", Type: "changelog_url", Name: "old-name", Config: json.RawMessage(`{"url":"https://old.com"}`), CreatedAt: time.Now(), UpdatedAt: time.Now()},
		},
	}
	mux := setupContextSourcesMux(store)

	body := `{"type":"git_repo","name":"new-name","config":{"repo":"nginx/nginx"}}`
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPut, "/context-sources/cs-10", strings.NewReader(body))
	r.Header.Set("Content-Type", "application/json")
	mux.ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", w.Code)
	}

	var got struct {
		Data models.ContextSource `json:"data"`
	}
	if err := json.NewDecoder(w.Body).Decode(&got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if got.Data.Name != "new-name" {
		t.Fatalf("expected name=new-name, got %s", got.Data.Name)
	}
	if got.Data.ID != "cs-10" {
		t.Fatalf("expected id=cs-10, got %s", got.Data.ID)
	}
	if got.Data.Type != "git_repo" {
		t.Fatalf("expected type=git_repo, got %s", got.Data.Type)
	}
}

func TestContextSourcesHandlerUpdateNotFound(t *testing.T) {
	store := &mockContextSourcesStore{}
	mux := setupContextSourcesMux(store)

	body := `{"type":"changelog_url","name":"update-missing","config":{"url":"https://example.com"}}`
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPut, "/context-sources/nonexistent", strings.NewReader(body))
	r.Header.Set("Content-Type", "application/json")
	mux.ServeHTTP(w, r)

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected status 404, got %d", w.Code)
	}
}

func TestContextSourcesHandlerDelete(t *testing.T) {
	store := &mockContextSourcesStore{
		sources: []models.ContextSource{
			{ID: "cs-5", ProjectID: "p1", Type: "changelog_url", Name: "to-delete", Config: json.RawMessage(`{}`), CreatedAt: time.Now(), UpdatedAt: time.Now()},
		},
	}
	mux := setupContextSourcesMux(store)

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodDelete, "/context-sources/cs-5", nil)
	mux.ServeHTTP(w, r)

	if w.Code != http.StatusNoContent {
		t.Fatalf("expected status 204, got %d", w.Code)
	}
	if w.Body.Len() != 0 {
		t.Fatalf("expected empty body, got %d bytes", w.Body.Len())
	}
	if store.deleted != "cs-5" {
		t.Fatalf("expected deleted=cs-5, got %s", store.deleted)
	}
}

func TestContextSourcesHandlerDeleteNotFound(t *testing.T) {
	store := &mockContextSourcesStore{}
	mux := setupContextSourcesMux(store)

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodDelete, "/context-sources/nonexistent", nil)
	mux.ServeHTTP(w, r)

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected status 404, got %d", w.Code)
	}
}
