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

// mockProjectsStore implements ProjectsStore for testing.
type mockProjectsStore struct {
	projects []models.Project
	created  *models.Project
	updated  *models.Project
	deleted  int
	listErr  error
	createErr error
	getErr   error
	updateErr error
	deleteErr error
}

func (m *mockProjectsStore) ListProjects(_ context.Context, page, perPage int) ([]models.Project, int, error) {
	if m.listErr != nil {
		return nil, 0, m.listErr
	}
	return m.projects, len(m.projects), nil
}

func (m *mockProjectsStore) CreateProject(_ context.Context, p *models.Project) error {
	if m.createErr != nil {
		return m.createErr
	}
	p.ID = 1
	p.CreatedAt = time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	p.UpdatedAt = p.CreatedAt
	m.created = p
	return nil
}

func (m *mockProjectsStore) GetProject(_ context.Context, id int) (*models.Project, error) {
	if m.getErr != nil {
		return nil, m.getErr
	}
	for i := range m.projects {
		if m.projects[i].ID == id {
			return &m.projects[i], nil
		}
	}
	return nil, fmt.Errorf("not found")
}

func (m *mockProjectsStore) UpdateProject(_ context.Context, id int, p *models.Project) error {
	if m.updateErr != nil {
		return m.updateErr
	}
	for i := range m.projects {
		if m.projects[i].ID == id {
			p.UpdatedAt = time.Date(2026, 2, 1, 0, 0, 0, 0, time.UTC)
			m.updated = p
			return nil
		}
	}
	return fmt.Errorf("not found")
}

func (m *mockProjectsStore) DeleteProject(_ context.Context, id int) error {
	if m.deleteErr != nil {
		return m.deleteErr
	}
	for _, p := range m.projects {
		if p.ID == id {
			m.deleted = id
			return nil
		}
	}
	return fmt.Errorf("not found")
}

func setupProjectsMux(store ProjectsStore) *http.ServeMux {
	h := NewProjectsHandler(store)
	mux := http.NewServeMux()
	mux.HandleFunc("GET /projects", h.List)
	mux.HandleFunc("POST /projects", h.Create)
	mux.HandleFunc("GET /projects/{id}", h.Get)
	mux.HandleFunc("PUT /projects/{id}", h.Update)
	mux.HandleFunc("DELETE /projects/{id}", h.Delete)
	return mux
}

func TestProjectsHandlerList(t *testing.T) {
	store := &mockProjectsStore{
		projects: []models.Project{
			{ID: 1, Name: "alpha", PipelineConfig: json.RawMessage(`{}`), CreatedAt: time.Now(), UpdatedAt: time.Now()},
			{ID: 2, Name: "beta", PipelineConfig: json.RawMessage(`{}`), CreatedAt: time.Now(), UpdatedAt: time.Now()},
		},
	}
	mux := setupProjectsMux(store)

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/projects", nil)
	mux.ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", w.Code)
	}

	var got struct {
		Data []models.Project `json:"data"`
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
		t.Fatalf("expected 2 projects, got %d", len(got.Data))
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

func TestProjectsHandlerListEmpty(t *testing.T) {
	store := &mockProjectsStore{}
	mux := setupProjectsMux(store)

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/projects", nil)
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

func TestProjectsHandlerListError(t *testing.T) {
	store := &mockProjectsStore{listErr: fmt.Errorf("db down")}
	mux := setupProjectsMux(store)

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/projects", nil)
	mux.ServeHTTP(w, r)

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected status 500, got %d", w.Code)
	}
}

func TestProjectsHandlerCreate(t *testing.T) {
	store := &mockProjectsStore{}
	mux := setupProjectsMux(store)

	body := `{"name":"my-project","description":"A project","url":"https://example.com"}`
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/projects", strings.NewReader(body))
	r.Header.Set("Content-Type", "application/json")
	mux.ServeHTTP(w, r)

	if w.Code != http.StatusCreated {
		t.Fatalf("expected status 201, got %d", w.Code)
	}

	var got struct {
		Data models.Project `json:"data"`
	}
	if err := json.NewDecoder(w.Body).Decode(&got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if got.Data.ID != 1 {
		t.Fatalf("expected id=1, got %d", got.Data.ID)
	}
	if got.Data.Name != "my-project" {
		t.Fatalf("expected name=my-project, got %s", got.Data.Name)
	}
	if got.Data.Description != "A project" {
		t.Fatalf("expected description=A project, got %s", got.Data.Description)
	}
	if store.created == nil {
		t.Fatal("expected store.created to be set")
	}
}

func TestProjectsHandlerCreateMissingName(t *testing.T) {
	store := &mockProjectsStore{}
	mux := setupProjectsMux(store)

	body := `{"description":"no name provided"}`
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/projects", strings.NewReader(body))
	r.Header.Set("Content-Type", "application/json")
	mux.ServeHTTP(w, r)

	if w.Code != http.StatusUnprocessableEntity {
		t.Fatalf("expected status 422, got %d", w.Code)
	}

	var got struct {
		Error struct {
			Code    string `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.NewDecoder(w.Body).Decode(&got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if got.Error.Code != "validation_error" {
		t.Fatalf("expected error.code=validation_error, got %s", got.Error.Code)
	}
	if got.Error.Message != "Name is required" {
		t.Fatalf("expected message=Name is required, got %s", got.Error.Message)
	}
}

func TestProjectsHandlerCreateWhitespaceName(t *testing.T) {
	store := &mockProjectsStore{}
	mux := setupProjectsMux(store)

	body := `{"name":"   "}`
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/projects", strings.NewReader(body))
	r.Header.Set("Content-Type", "application/json")
	mux.ServeHTTP(w, r)

	if w.Code != http.StatusUnprocessableEntity {
		t.Fatalf("expected status 422, got %d", w.Code)
	}
}

func TestProjectsHandlerCreateInvalidJSON(t *testing.T) {
	store := &mockProjectsStore{}
	mux := setupProjectsMux(store)

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/projects", strings.NewReader(`{invalid`))
	r.Header.Set("Content-Type", "application/json")
	mux.ServeHTTP(w, r)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d", w.Code)
	}
}

func TestProjectsHandlerCreateDefaultPipelineConfig(t *testing.T) {
	store := &mockProjectsStore{}
	mux := setupProjectsMux(store)

	body := `{"name":"no-config"}`
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/projects", strings.NewReader(body))
	r.Header.Set("Content-Type", "application/json")
	mux.ServeHTTP(w, r)

	if w.Code != http.StatusCreated {
		t.Fatalf("expected status 201, got %d", w.Code)
	}
	if store.created == nil {
		t.Fatal("expected store.created to be set")
	}
	if string(store.created.PipelineConfig) != "{}" {
		t.Fatalf("expected default pipeline_config={}, got %s", string(store.created.PipelineConfig))
	}
}

func TestProjectsHandlerGet(t *testing.T) {
	store := &mockProjectsStore{
		projects: []models.Project{
			{ID: 42, Name: "found-project", PipelineConfig: json.RawMessage(`{"key":"val"}`), CreatedAt: time.Now(), UpdatedAt: time.Now()},
		},
	}
	mux := setupProjectsMux(store)

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/projects/42", nil)
	mux.ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", w.Code)
	}

	var got struct {
		Data models.Project `json:"data"`
	}
	if err := json.NewDecoder(w.Body).Decode(&got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if got.Data.ID != 42 {
		t.Fatalf("expected id=42, got %d", got.Data.ID)
	}
	if got.Data.Name != "found-project" {
		t.Fatalf("expected name=found-project, got %s", got.Data.Name)
	}
}

func TestProjectsHandlerGetNotFound(t *testing.T) {
	store := &mockProjectsStore{}
	mux := setupProjectsMux(store)

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/projects/999", nil)
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

func TestProjectsHandlerGetInvalidID(t *testing.T) {
	store := &mockProjectsStore{}
	mux := setupProjectsMux(store)

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/projects/abc", nil)
	mux.ServeHTTP(w, r)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d", w.Code)
	}
}

func TestProjectsHandlerUpdate(t *testing.T) {
	store := &mockProjectsStore{
		projects: []models.Project{
			{ID: 10, Name: "old-name", PipelineConfig: json.RawMessage(`{}`), CreatedAt: time.Now(), UpdatedAt: time.Now()},
		},
	}
	mux := setupProjectsMux(store)

	body := `{"name":"new-name","description":"updated","pipeline_config":{"nodes":["a"]}}`
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPut, "/projects/10", strings.NewReader(body))
	r.Header.Set("Content-Type", "application/json")
	mux.ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", w.Code)
	}

	var got struct {
		Data models.Project `json:"data"`
	}
	if err := json.NewDecoder(w.Body).Decode(&got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if got.Data.Name != "new-name" {
		t.Fatalf("expected name=new-name, got %s", got.Data.Name)
	}
	if got.Data.ID != 10 {
		t.Fatalf("expected id=10, got %d", got.Data.ID)
	}
}

func TestProjectsHandlerUpdateNotFound(t *testing.T) {
	store := &mockProjectsStore{}
	mux := setupProjectsMux(store)

	body := `{"name":"update-missing"}`
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPut, "/projects/999", strings.NewReader(body))
	r.Header.Set("Content-Type", "application/json")
	mux.ServeHTTP(w, r)

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected status 404, got %d", w.Code)
	}
}

func TestProjectsHandlerDelete(t *testing.T) {
	store := &mockProjectsStore{
		projects: []models.Project{
			{ID: 5, Name: "to-delete", PipelineConfig: json.RawMessage(`{}`), CreatedAt: time.Now(), UpdatedAt: time.Now()},
		},
	}
	mux := setupProjectsMux(store)

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodDelete, "/projects/5", nil)
	mux.ServeHTTP(w, r)

	if w.Code != http.StatusNoContent {
		t.Fatalf("expected status 204, got %d", w.Code)
	}
	if w.Body.Len() != 0 {
		t.Fatalf("expected empty body, got %d bytes", w.Body.Len())
	}
	if store.deleted != 5 {
		t.Fatalf("expected deleted=5, got %d", store.deleted)
	}
}

func TestProjectsHandlerDeleteNotFound(t *testing.T) {
	store := &mockProjectsStore{}
	mux := setupProjectsMux(store)

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodDelete, "/projects/999", nil)
	mux.ServeHTTP(w, r)

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected status 404, got %d", w.Code)
	}
}
