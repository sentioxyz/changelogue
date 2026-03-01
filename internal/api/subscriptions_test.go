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

// mockSubscriptionsStore implements SubscriptionsStore for testing.
type mockSubscriptionsStore struct {
	subscriptions []models.Subscription
	created       *models.Subscription
	updated       *models.Subscription
	deleted       string
	batchCreated []models.Subscription
	batchErr     error
	listErr       error
	createErr     error
	getErr        error
	updateErr     error
	deleteErr     error
}

func (m *mockSubscriptionsStore) ListSubscriptions(_ context.Context, page, perPage int) ([]models.Subscription, int, error) {
	if m.listErr != nil {
		return nil, 0, m.listErr
	}
	return m.subscriptions, len(m.subscriptions), nil
}

func (m *mockSubscriptionsStore) CreateSubscription(_ context.Context, sub *models.Subscription) error {
	if m.createErr != nil {
		return m.createErr
	}
	sub.ID = "sub-001"
	sub.CreatedAt = time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	m.created = sub
	return nil
}

func (m *mockSubscriptionsStore) GetSubscription(_ context.Context, id string) (*models.Subscription, error) {
	if m.getErr != nil {
		return nil, m.getErr
	}
	for i := range m.subscriptions {
		if m.subscriptions[i].ID == id {
			return &m.subscriptions[i], nil
		}
	}
	return nil, fmt.Errorf("not found")
}

func (m *mockSubscriptionsStore) UpdateSubscription(_ context.Context, id string, sub *models.Subscription) error {
	if m.updateErr != nil {
		return m.updateErr
	}
	for i := range m.subscriptions {
		if m.subscriptions[i].ID == id {
			m.updated = sub
			return nil
		}
	}
	return fmt.Errorf("not found")
}

func (m *mockSubscriptionsStore) CreateSubscriptionBatch(_ context.Context, subs []models.Subscription) ([]models.Subscription, error) {
	if m.batchErr != nil {
		return nil, m.batchErr
	}
	var result []models.Subscription
	for i, sub := range subs {
		sub.ID = fmt.Sprintf("sub-batch-%d", i)
		sub.CreatedAt = time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
		result = append(result, sub)
	}
	m.batchCreated = result
	return result, nil
}

func (m *mockSubscriptionsStore) DeleteSubscription(_ context.Context, id string) error {
	if m.deleteErr != nil {
		return m.deleteErr
	}
	for _, s := range m.subscriptions {
		if s.ID == id {
			m.deleted = id
			return nil
		}
	}
	return fmt.Errorf("not found")
}

func setupSubscriptionsMux(store SubscriptionsStore) *http.ServeMux {
	h := NewSubscriptionsHandler(store)
	mux := http.NewServeMux()
	mux.HandleFunc("GET /subscriptions", h.List)
	mux.HandleFunc("POST /subscriptions", h.Create)
	mux.HandleFunc("POST /subscriptions/batch", h.BatchCreate)
	mux.HandleFunc("GET /subscriptions/{id}", h.Get)
	mux.HandleFunc("PUT /subscriptions/{id}", h.Update)
	mux.HandleFunc("DELETE /subscriptions/{id}", h.Delete)
	return mux
}

func TestSubscriptionsHandlerList(t *testing.T) {
	srcID := "s1"
	projID := "p1"
	store := &mockSubscriptionsStore{
		subscriptions: []models.Subscription{
			{ID: "sub-1", ChannelID: "ch1", Type: "source", SourceID: &srcID, CreatedAt: time.Now()},
			{ID: "sub-2", ChannelID: "ch2", Type: "project", ProjectID: &projID, CreatedAt: time.Now()},
		},
	}
	mux := setupSubscriptionsMux(store)

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/subscriptions", nil)
	mux.ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", w.Code)
	}

	var got struct {
		Data []models.Subscription `json:"data"`
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
		t.Fatalf("expected 2 subscriptions, got %d", len(got.Data))
	}
	if got.Meta.Total != 2 {
		t.Fatalf("expected total=2, got %d", got.Meta.Total)
	}
}

func TestSubscriptionsHandlerCreateSource(t *testing.T) {
	store := &mockSubscriptionsStore{}
	mux := setupSubscriptionsMux(store)

	body := `{"channel_id":"ch1","type":"source","source_id":"s1"}`
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/subscriptions", strings.NewReader(body))
	r.Header.Set("Content-Type", "application/json")
	mux.ServeHTTP(w, r)

	if w.Code != http.StatusCreated {
		t.Fatalf("expected status 201, got %d; body: %s", w.Code, w.Body.String())
	}

	var got struct {
		Data models.Subscription `json:"data"`
	}
	if err := json.NewDecoder(w.Body).Decode(&got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if got.Data.ID != "sub-001" {
		t.Fatalf("expected id=sub-001, got %s", got.Data.ID)
	}
	if got.Data.Type != "source" {
		t.Fatalf("expected type=source, got %s", got.Data.Type)
	}
	if store.created == nil {
		t.Fatal("expected store.created to be set")
	}
}

func TestSubscriptionsHandlerCreateProject(t *testing.T) {
	store := &mockSubscriptionsStore{}
	mux := setupSubscriptionsMux(store)

	body := `{"channel_id":"ch1","type":"project","project_id":"p1"}`
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/subscriptions", strings.NewReader(body))
	r.Header.Set("Content-Type", "application/json")
	mux.ServeHTTP(w, r)

	if w.Code != http.StatusCreated {
		t.Fatalf("expected status 201, got %d; body: %s", w.Code, w.Body.String())
	}
}

func TestSubscriptionsHandlerCreateInvalidType(t *testing.T) {
	store := &mockSubscriptionsStore{}
	mux := setupSubscriptionsMux(store)

	body := `{"channel_id":"ch1","type":"invalid","source_id":"s1"}`
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/subscriptions", strings.NewReader(body))
	r.Header.Set("Content-Type", "application/json")
	mux.ServeHTTP(w, r)

	if w.Code != http.StatusUnprocessableEntity {
		t.Fatalf("expected status 422, got %d", w.Code)
	}
}

func TestSubscriptionsHandlerCreateMissingChannelID(t *testing.T) {
	store := &mockSubscriptionsStore{}
	mux := setupSubscriptionsMux(store)

	body := `{"type":"source","source_id":"s1"}`
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/subscriptions", strings.NewReader(body))
	r.Header.Set("Content-Type", "application/json")
	mux.ServeHTTP(w, r)

	if w.Code != http.StatusUnprocessableEntity {
		t.Fatalf("expected status 422, got %d", w.Code)
	}
}

func TestSubscriptionsHandlerCreateSourceMissingSourceID(t *testing.T) {
	store := &mockSubscriptionsStore{}
	mux := setupSubscriptionsMux(store)

	body := `{"channel_id":"ch1","type":"source"}`
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/subscriptions", strings.NewReader(body))
	r.Header.Set("Content-Type", "application/json")
	mux.ServeHTTP(w, r)

	if w.Code != http.StatusUnprocessableEntity {
		t.Fatalf("expected status 422, got %d", w.Code)
	}
}

func TestSubscriptionsHandlerCreateProjectMissingProjectID(t *testing.T) {
	store := &mockSubscriptionsStore{}
	mux := setupSubscriptionsMux(store)

	body := `{"channel_id":"ch1","type":"project"}`
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/subscriptions", strings.NewReader(body))
	r.Header.Set("Content-Type", "application/json")
	mux.ServeHTTP(w, r)

	if w.Code != http.StatusUnprocessableEntity {
		t.Fatalf("expected status 422, got %d", w.Code)
	}
}

func TestSubscriptionsHandlerCreateInvalidJSON(t *testing.T) {
	store := &mockSubscriptionsStore{}
	mux := setupSubscriptionsMux(store)

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/subscriptions", strings.NewReader(`{invalid`))
	r.Header.Set("Content-Type", "application/json")
	mux.ServeHTTP(w, r)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d", w.Code)
	}
}

func TestSubscriptionsHandlerGet(t *testing.T) {
	srcID := "s1"
	store := &mockSubscriptionsStore{
		subscriptions: []models.Subscription{
			{ID: "sub-42", ChannelID: "ch1", Type: "source", SourceID: &srcID, CreatedAt: time.Now()},
		},
	}
	mux := setupSubscriptionsMux(store)

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/subscriptions/sub-42", nil)
	mux.ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", w.Code)
	}

	var got struct {
		Data models.Subscription `json:"data"`
	}
	if err := json.NewDecoder(w.Body).Decode(&got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if got.Data.ID != "sub-42" {
		t.Fatalf("expected id=sub-42, got %s", got.Data.ID)
	}
}

func TestSubscriptionsHandlerGetNotFound(t *testing.T) {
	store := &mockSubscriptionsStore{}
	mux := setupSubscriptionsMux(store)

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/subscriptions/nonexistent", nil)
	mux.ServeHTTP(w, r)

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected status 404, got %d", w.Code)
	}
}

func TestSubscriptionsHandlerDelete(t *testing.T) {
	srcID := "s1"
	store := &mockSubscriptionsStore{
		subscriptions: []models.Subscription{
			{ID: "sub-5", ChannelID: "ch1", Type: "source", SourceID: &srcID, CreatedAt: time.Now()},
		},
	}
	mux := setupSubscriptionsMux(store)

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodDelete, "/subscriptions/sub-5", nil)
	mux.ServeHTTP(w, r)

	if w.Code != http.StatusNoContent {
		t.Fatalf("expected status 204, got %d", w.Code)
	}
	if store.deleted != "sub-5" {
		t.Fatalf("expected deleted=sub-5, got %s", store.deleted)
	}
}

func TestSubscriptionsHandlerDeleteNotFound(t *testing.T) {
	store := &mockSubscriptionsStore{}
	mux := setupSubscriptionsMux(store)

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodDelete, "/subscriptions/nonexistent", nil)
	mux.ServeHTTP(w, r)

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected status 404, got %d", w.Code)
	}
}

func TestSubscriptionsHandlerBatchCreateProjects(t *testing.T) {
	store := &mockSubscriptionsStore{}
	mux := setupSubscriptionsMux(store)

	body := `{"channel_id":"ch1","type":"project","project_ids":["p1","p2","p3"]}`
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/subscriptions/batch", strings.NewReader(body))
	r.Header.Set("Content-Type", "application/json")
	mux.ServeHTTP(w, r)

	if w.Code != http.StatusCreated {
		t.Fatalf("expected status 201, got %d; body: %s", w.Code, w.Body.String())
	}
	var got struct {
		Data []models.Subscription `json:"data"`
	}
	if err := json.NewDecoder(w.Body).Decode(&got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(got.Data) != 3 {
		t.Fatalf("expected 3 subscriptions, got %d", len(got.Data))
	}
	if len(store.batchCreated) != 3 {
		t.Fatalf("expected 3 in store, got %d", len(store.batchCreated))
	}
}

func TestSubscriptionsHandlerBatchCreateSources(t *testing.T) {
	store := &mockSubscriptionsStore{}
	mux := setupSubscriptionsMux(store)

	body := `{"channel_id":"ch1","type":"source","source_ids":["s1","s2"]}`
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/subscriptions/batch", strings.NewReader(body))
	r.Header.Set("Content-Type", "application/json")
	mux.ServeHTTP(w, r)

	if w.Code != http.StatusCreated {
		t.Fatalf("expected status 201, got %d; body: %s", w.Code, w.Body.String())
	}
	var got struct {
		Data []models.Subscription `json:"data"`
	}
	if err := json.NewDecoder(w.Body).Decode(&got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(got.Data) != 2 {
		t.Fatalf("expected 2 subscriptions, got %d", len(got.Data))
	}
}

func TestSubscriptionsHandlerBatchCreateEmptyIDs(t *testing.T) {
	store := &mockSubscriptionsStore{}
	mux := setupSubscriptionsMux(store)

	body := `{"channel_id":"ch1","type":"project","project_ids":[]}`
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/subscriptions/batch", strings.NewReader(body))
	r.Header.Set("Content-Type", "application/json")
	mux.ServeHTTP(w, r)

	if w.Code != http.StatusUnprocessableEntity {
		t.Fatalf("expected status 422, got %d", w.Code)
	}
}

func TestSubscriptionsHandlerBatchCreateMissingChannelID(t *testing.T) {
	store := &mockSubscriptionsStore{}
	mux := setupSubscriptionsMux(store)

	body := `{"type":"project","project_ids":["p1"]}`
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/subscriptions/batch", strings.NewReader(body))
	r.Header.Set("Content-Type", "application/json")
	mux.ServeHTTP(w, r)

	if w.Code != http.StatusUnprocessableEntity {
		t.Fatalf("expected status 422, got %d", w.Code)
	}
}
