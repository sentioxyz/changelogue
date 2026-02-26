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

// mockAgentStore implements AgentStore for testing.
type mockAgentStore struct {
	runs       []models.AgentRun
	triggered  *models.AgentRun
	triggerErr error
	listErr    error
	getErr     error
}

func (m *mockAgentStore) TriggerAgentRun(_ context.Context, projectID, trigger string) (*models.AgentRun, error) {
	if m.triggerErr != nil {
		return nil, m.triggerErr
	}
	run := &models.AgentRun{
		ID:        "run-001",
		ProjectID: projectID,
		Trigger:   trigger,
		Status:    "pending",
		CreatedAt: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
	}
	m.triggered = run
	return run, nil
}

func (m *mockAgentStore) ListAgentRuns(_ context.Context, projectID string, page, perPage int) ([]models.AgentRun, int, error) {
	if m.listErr != nil {
		return nil, 0, m.listErr
	}
	return m.runs, len(m.runs), nil
}

func (m *mockAgentStore) GetAgentRun(_ context.Context, id string) (*models.AgentRun, error) {
	if m.getErr != nil {
		return nil, m.getErr
	}
	for i := range m.runs {
		if m.runs[i].ID == id {
			return &m.runs[i], nil
		}
	}
	return nil, fmt.Errorf("not found")
}

func setupAgentMux(store AgentStore) *http.ServeMux {
	h := NewAgentHandler(store)
	mux := http.NewServeMux()
	mux.HandleFunc("POST /projects/{projectId}/agent/run", h.TriggerRun)
	mux.HandleFunc("GET /projects/{projectId}/agent/runs", h.ListRuns)
	mux.HandleFunc("GET /agent-runs/{id}", h.GetRun)
	return mux
}

func TestAgentHandlerTriggerRun(t *testing.T) {
	store := &mockAgentStore{}
	mux := setupAgentMux(store)

	body := `{"trigger":"release_detected"}`
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/projects/p1/agent/run", strings.NewReader(body))
	r.Header.Set("Content-Type", "application/json")
	mux.ServeHTTP(w, r)

	if w.Code != http.StatusCreated {
		t.Fatalf("expected status 201, got %d; body: %s", w.Code, w.Body.String())
	}

	var got struct {
		Data models.AgentRun `json:"data"`
	}
	if err := json.NewDecoder(w.Body).Decode(&got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if got.Data.ID != "run-001" {
		t.Fatalf("expected id=run-001, got %s", got.Data.ID)
	}
	if got.Data.ProjectID != "p1" {
		t.Fatalf("expected project_id=p1, got %s", got.Data.ProjectID)
	}
	if got.Data.Trigger != "release_detected" {
		t.Fatalf("expected trigger=release_detected, got %s", got.Data.Trigger)
	}
	if got.Data.Status != "pending" {
		t.Fatalf("expected status=pending, got %s", got.Data.Status)
	}
	if store.triggered == nil {
		t.Fatal("expected store.triggered to be set")
	}
}

func TestAgentHandlerTriggerRunDefaultTrigger(t *testing.T) {
	store := &mockAgentStore{}
	mux := setupAgentMux(store)

	body := `{}`
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/projects/p1/agent/run", strings.NewReader(body))
	r.Header.Set("Content-Type", "application/json")
	mux.ServeHTTP(w, r)

	if w.Code != http.StatusCreated {
		t.Fatalf("expected status 201, got %d; body: %s", w.Code, w.Body.String())
	}

	var got struct {
		Data models.AgentRun `json:"data"`
	}
	if err := json.NewDecoder(w.Body).Decode(&got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if got.Data.Trigger != "manual" {
		t.Fatalf("expected trigger=manual (default), got %s", got.Data.Trigger)
	}
}

func TestAgentHandlerTriggerRunInvalidJSON(t *testing.T) {
	store := &mockAgentStore{}
	mux := setupAgentMux(store)

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/projects/p1/agent/run", strings.NewReader(`{invalid`))
	r.Header.Set("Content-Type", "application/json")
	mux.ServeHTTP(w, r)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d", w.Code)
	}
}

func TestAgentHandlerTriggerRunError(t *testing.T) {
	store := &mockAgentStore{triggerErr: fmt.Errorf("agent unavailable")}
	mux := setupAgentMux(store)

	body := `{"trigger":"manual"}`
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/projects/p1/agent/run", strings.NewReader(body))
	r.Header.Set("Content-Type", "application/json")
	mux.ServeHTTP(w, r)

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected status 500, got %d", w.Code)
	}
}

func TestAgentHandlerListRuns(t *testing.T) {
	store := &mockAgentStore{
		runs: []models.AgentRun{
			{ID: "run-1", ProjectID: "p1", Trigger: "manual", Status: "completed", CreatedAt: time.Now()},
			{ID: "run-2", ProjectID: "p1", Trigger: "release_detected", Status: "pending", CreatedAt: time.Now()},
		},
	}
	mux := setupAgentMux(store)

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/projects/p1/agent/runs", nil)
	mux.ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", w.Code)
	}

	var got struct {
		Data []models.AgentRun `json:"data"`
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
		t.Fatalf("expected 2 agent runs, got %d", len(got.Data))
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

func TestAgentHandlerListRunsEmpty(t *testing.T) {
	store := &mockAgentStore{}
	mux := setupAgentMux(store)

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/projects/p1/agent/runs", nil)
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

func TestAgentHandlerListRunsError(t *testing.T) {
	store := &mockAgentStore{listErr: fmt.Errorf("db down")}
	mux := setupAgentMux(store)

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/projects/p1/agent/runs", nil)
	mux.ServeHTTP(w, r)

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected status 500, got %d", w.Code)
	}
}

func TestAgentHandlerGetRun(t *testing.T) {
	store := &mockAgentStore{
		runs: []models.AgentRun{
			{ID: "run-42", ProjectID: "p1", Trigger: "release_detected", Status: "completed", CreatedAt: time.Now()},
		},
	}
	mux := setupAgentMux(store)

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/agent-runs/run-42", nil)
	mux.ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", w.Code)
	}

	var got struct {
		Data models.AgentRun `json:"data"`
	}
	if err := json.NewDecoder(w.Body).Decode(&got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if got.Data.ID != "run-42" {
		t.Fatalf("expected id=run-42, got %s", got.Data.ID)
	}
	if got.Data.Trigger != "release_detected" {
		t.Fatalf("expected trigger=release_detected, got %s", got.Data.Trigger)
	}
	if got.Data.Status != "completed" {
		t.Fatalf("expected status=completed, got %s", got.Data.Status)
	}
}

func TestAgentHandlerGetRunNotFound(t *testing.T) {
	store := &mockAgentStore{}
	mux := setupAgentMux(store)

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/agent-runs/nonexistent", nil)
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
