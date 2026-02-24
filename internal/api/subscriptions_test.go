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

// mockSubscriptionsStore implements SubscriptionsStore for testing.
type mockSubscriptionsStore struct {
	subscriptions []models.Subscription
	created       *models.Subscription
	updated       *models.Subscription
	deleted       int
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
	sub.ID = 1
	sub.Enabled = true
	sub.CreatedAt = time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	sub.UpdatedAt = sub.CreatedAt
	m.created = sub
	return nil
}

func (m *mockSubscriptionsStore) GetSubscription(_ context.Context, id int) (*models.Subscription, error) {
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

func (m *mockSubscriptionsStore) UpdateSubscription(_ context.Context, id int, sub *models.Subscription) error {
	if m.updateErr != nil {
		return m.updateErr
	}
	for i := range m.subscriptions {
		if m.subscriptions[i].ID == id {
			sub.UpdatedAt = time.Date(2026, 2, 1, 0, 0, 0, 0, time.UTC)
			m.updated = sub
			return nil
		}
	}
	return fmt.Errorf("not found")
}

func (m *mockSubscriptionsStore) DeleteSubscription(_ context.Context, id int) error {
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
	mux.HandleFunc("GET /subscriptions/{id}", h.Get)
	mux.HandleFunc("PUT /subscriptions/{id}", h.Update)
	mux.HandleFunc("DELETE /subscriptions/{id}", h.Delete)
	return mux
}

func TestSubscriptionsHandlerList(t *testing.T) {
	store := &mockSubscriptionsStore{
		subscriptions: []models.Subscription{
			{ID: 1, ProjectID: 1, ChannelType: "slack", Frequency: "instant", Enabled: true, CreatedAt: time.Now(), UpdatedAt: time.Now()},
			{ID: 2, ProjectID: 2, ChannelType: "pagerduty", Frequency: "daily", Enabled: true, CreatedAt: time.Now(), UpdatedAt: time.Now()},
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

func TestSubscriptionsHandlerCreate(t *testing.T) {
	store := &mockSubscriptionsStore{}
	mux := setupSubscriptionsMux(store)

	body := `{"project_id":1,"channel_type":"slack","channel_id":5}`
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
	if got.Data.ID != 1 {
		t.Fatalf("expected id=1, got %d", got.Data.ID)
	}
	if got.Data.Frequency != "instant" {
		t.Fatalf("expected default frequency=instant, got %s", got.Data.Frequency)
	}
	if store.created == nil {
		t.Fatal("expected store.created to be set")
	}
}

func TestSubscriptionsHandlerCreateMissingProjectID(t *testing.T) {
	store := &mockSubscriptionsStore{}
	mux := setupSubscriptionsMux(store)

	body := `{"channel_type":"slack"}`
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/subscriptions", strings.NewReader(body))
	r.Header.Set("Content-Type", "application/json")
	mux.ServeHTTP(w, r)

	if w.Code != http.StatusUnprocessableEntity {
		t.Fatalf("expected status 422, got %d", w.Code)
	}
}

func TestSubscriptionsHandlerCreateMissingChannelType(t *testing.T) {
	store := &mockSubscriptionsStore{}
	mux := setupSubscriptionsMux(store)

	body := `{"project_id":1}`
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
	store := &mockSubscriptionsStore{
		subscriptions: []models.Subscription{
			{ID: 42, ProjectID: 1, ChannelType: "slack", Frequency: "instant", Enabled: true, CreatedAt: time.Now(), UpdatedAt: time.Now()},
		},
	}
	mux := setupSubscriptionsMux(store)

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/subscriptions/42", nil)
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
	if got.Data.ID != 42 {
		t.Fatalf("expected id=42, got %d", got.Data.ID)
	}
}

func TestSubscriptionsHandlerGetNotFound(t *testing.T) {
	store := &mockSubscriptionsStore{}
	mux := setupSubscriptionsMux(store)

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/subscriptions/999", nil)
	mux.ServeHTTP(w, r)

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected status 404, got %d", w.Code)
	}
}

func TestSubscriptionsHandlerDelete(t *testing.T) {
	store := &mockSubscriptionsStore{
		subscriptions: []models.Subscription{
			{ID: 5, ProjectID: 1, ChannelType: "slack", Frequency: "instant", Enabled: true, CreatedAt: time.Now(), UpdatedAt: time.Now()},
		},
	}
	mux := setupSubscriptionsMux(store)

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodDelete, "/subscriptions/5", nil)
	mux.ServeHTTP(w, r)

	if w.Code != http.StatusNoContent {
		t.Fatalf("expected status 204, got %d", w.Code)
	}
	if store.deleted != 5 {
		t.Fatalf("expected deleted=5, got %d", store.deleted)
	}
}

func TestSubscriptionsHandlerDeleteNotFound(t *testing.T) {
	store := &mockSubscriptionsStore{}
	mux := setupSubscriptionsMux(store)

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodDelete, "/subscriptions/999", nil)
	mux.ServeHTTP(w, r)

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected status 404, got %d", w.Code)
	}
}

func TestSubscriptionsHandlerCreateDefaultFrequency(t *testing.T) {
	store := &mockSubscriptionsStore{}
	mux := setupSubscriptionsMux(store)

	body := `{"project_id":1,"channel_type":"slack"}`
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/subscriptions", strings.NewReader(body))
	r.Header.Set("Content-Type", "application/json")
	mux.ServeHTTP(w, r)

	if w.Code != http.StatusCreated {
		t.Fatalf("expected status 201, got %d", w.Code)
	}
	if store.created == nil {
		t.Fatal("expected store.created to be set")
	}
	if store.created.Frequency != "instant" {
		t.Fatalf("expected default frequency=instant, got %s", store.created.Frequency)
	}
}
