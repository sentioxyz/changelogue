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
	"github.com/sentioxyz/changelogue/internal/routing"
)

// mockChannelsStore implements ChannelsStore for testing.
type mockChannelsStore struct {
	channels  []models.NotificationChannel
	created   *models.NotificationChannel
	updated   *models.NotificationChannel
	deleted   string
	listErr   error
	createErr error
	getErr    error
	updateErr error
	deleteErr error
}

func (m *mockChannelsStore) ListChannels(_ context.Context, page, perPage int) ([]models.NotificationChannel, int, error) {
	if m.listErr != nil {
		return nil, 0, m.listErr
	}
	return m.channels, len(m.channels), nil
}

func (m *mockChannelsStore) CreateChannel(_ context.Context, ch *models.NotificationChannel) error {
	if m.createErr != nil {
		return m.createErr
	}
	ch.ID = "ch-001"
	ch.CreatedAt = time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	ch.UpdatedAt = ch.CreatedAt
	m.created = ch
	return nil
}

func (m *mockChannelsStore) GetChannel(_ context.Context, id string) (*models.NotificationChannel, error) {
	if m.getErr != nil {
		return nil, m.getErr
	}
	for i := range m.channels {
		if m.channels[i].ID == id {
			return &m.channels[i], nil
		}
	}
	return nil, fmt.Errorf("not found")
}

func (m *mockChannelsStore) UpdateChannel(_ context.Context, id string, ch *models.NotificationChannel) error {
	if m.updateErr != nil {
		return m.updateErr
	}
	for i := range m.channels {
		if m.channels[i].ID == id {
			m.updated = ch
			return nil
		}
	}
	return fmt.Errorf("not found")
}

func (m *mockChannelsStore) DeleteChannel(_ context.Context, id string) error {
	if m.deleteErr != nil {
		return m.deleteErr
	}
	for _, c := range m.channels {
		if c.ID == id {
			m.deleted = id
			return nil
		}
	}
	return fmt.Errorf("not found")
}

// mockSender implements routing.Sender for testing.
type mockSender struct {
	err  error
	sent *routing.Notification
}

func (m *mockSender) Send(_ context.Context, _ *models.NotificationChannel, msg routing.Notification) error {
	m.sent = &msg
	return m.err
}

func setupChannelsMux(store ChannelsStore) *http.ServeMux {
	return setupChannelsMuxWithSenders(store, nil)
}

func setupChannelsMuxWithSenders(store ChannelsStore, senders map[string]routing.Sender) *http.ServeMux {
	h := NewChannelsHandler(store, senders)
	mux := http.NewServeMux()
	mux.HandleFunc("GET /channels", h.List)
	mux.HandleFunc("POST /channels", h.Create)
	mux.HandleFunc("GET /channels/{id}", h.Get)
	mux.HandleFunc("PUT /channels/{id}", h.Update)
	mux.HandleFunc("DELETE /channels/{id}", h.Delete)
	mux.HandleFunc("POST /channels/{id}/test", h.Test)
	return mux
}

func TestChannelsHandlerList(t *testing.T) {
	store := &mockChannelsStore{
		channels: []models.NotificationChannel{
			{ID: "ch-1", Type: "slack", Name: "ops-alerts", Config: json.RawMessage(`{"webhook_url":"https://hooks.slack.com/xxx"}`), CreatedAt: time.Now(), UpdatedAt: time.Now()},
			{ID: "ch-2", Type: "pagerduty", Name: "on-call", Config: json.RawMessage(`{"routing_key":"abc123"}`), CreatedAt: time.Now(), UpdatedAt: time.Now()},
		},
	}
	mux := setupChannelsMux(store)

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/channels", nil)
	mux.ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", w.Code)
	}

	var got struct {
		Data []models.NotificationChannel `json:"data"`
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
		t.Fatalf("expected 2 channels, got %d", len(got.Data))
	}
	if got.Meta.Total != 2 {
		t.Fatalf("expected total=2, got %d", got.Meta.Total)
	}
}

func TestChannelsHandlerCreate(t *testing.T) {
	store := &mockChannelsStore{}
	mux := setupChannelsMux(store)

	body := `{"type":"slack","name":"ops-alerts","config":{"webhook_url":"https://hooks.slack.com/xxx"}}`
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/channels", strings.NewReader(body))
	r.Header.Set("Content-Type", "application/json")
	mux.ServeHTTP(w, r)

	if w.Code != http.StatusCreated {
		t.Fatalf("expected status 201, got %d; body: %s", w.Code, w.Body.String())
	}

	var got struct {
		Data models.NotificationChannel `json:"data"`
	}
	if err := json.NewDecoder(w.Body).Decode(&got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if got.Data.ID != "ch-001" {
		t.Fatalf("expected id=ch-001, got %s", got.Data.ID)
	}
	if got.Data.Name != "ops-alerts" {
		t.Fatalf("expected name=ops-alerts, got %s", got.Data.Name)
	}
	if store.created == nil {
		t.Fatal("expected store.created to be set")
	}
}

func TestChannelsHandlerCreateMissingType(t *testing.T) {
	store := &mockChannelsStore{}
	mux := setupChannelsMux(store)

	body := `{"name":"ops-alerts","config":{"webhook_url":"https://hooks.slack.com/xxx"}}`
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/channels", strings.NewReader(body))
	r.Header.Set("Content-Type", "application/json")
	mux.ServeHTTP(w, r)

	if w.Code != http.StatusUnprocessableEntity {
		t.Fatalf("expected status 422, got %d", w.Code)
	}
}

func TestChannelsHandlerCreateMissingName(t *testing.T) {
	store := &mockChannelsStore{}
	mux := setupChannelsMux(store)

	body := `{"type":"slack","config":{"webhook_url":"https://hooks.slack.com/xxx"}}`
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/channels", strings.NewReader(body))
	r.Header.Set("Content-Type", "application/json")
	mux.ServeHTTP(w, r)

	if w.Code != http.StatusUnprocessableEntity {
		t.Fatalf("expected status 422, got %d", w.Code)
	}
}

func TestChannelsHandlerCreateMissingConfig(t *testing.T) {
	store := &mockChannelsStore{}
	mux := setupChannelsMux(store)

	body := `{"type":"slack","name":"ops-alerts"}`
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/channels", strings.NewReader(body))
	r.Header.Set("Content-Type", "application/json")
	mux.ServeHTTP(w, r)

	if w.Code != http.StatusUnprocessableEntity {
		t.Fatalf("expected status 422, got %d", w.Code)
	}
}

func TestChannelsHandlerCreateInvalidJSON(t *testing.T) {
	store := &mockChannelsStore{}
	mux := setupChannelsMux(store)

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/channels", strings.NewReader(`{invalid`))
	r.Header.Set("Content-Type", "application/json")
	mux.ServeHTTP(w, r)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d", w.Code)
	}
}

func TestChannelsHandlerGet(t *testing.T) {
	store := &mockChannelsStore{
		channels: []models.NotificationChannel{
			{ID: "ch-42", Type: "slack", Name: "ops-alerts", Config: json.RawMessage(`{"webhook_url":"https://hooks.slack.com/xxx"}`), CreatedAt: time.Now(), UpdatedAt: time.Now()},
		},
	}
	mux := setupChannelsMux(store)

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/channels/ch-42", nil)
	mux.ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", w.Code)
	}

	var got struct {
		Data models.NotificationChannel `json:"data"`
	}
	if err := json.NewDecoder(w.Body).Decode(&got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if got.Data.ID != "ch-42" {
		t.Fatalf("expected id=ch-42, got %s", got.Data.ID)
	}
}

func TestChannelsHandlerGetNotFound(t *testing.T) {
	store := &mockChannelsStore{}
	mux := setupChannelsMux(store)

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/channels/nonexistent", nil)
	mux.ServeHTTP(w, r)

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected status 404, got %d", w.Code)
	}
}

func TestChannelsHandlerDelete(t *testing.T) {
	store := &mockChannelsStore{
		channels: []models.NotificationChannel{
			{ID: "ch-5", Type: "slack", Name: "to-delete", Config: json.RawMessage(`{}`), CreatedAt: time.Now(), UpdatedAt: time.Now()},
		},
	}
	mux := setupChannelsMux(store)

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodDelete, "/channels/ch-5", nil)
	mux.ServeHTTP(w, r)

	if w.Code != http.StatusNoContent {
		t.Fatalf("expected status 204, got %d", w.Code)
	}
	if store.deleted != "ch-5" {
		t.Fatalf("expected deleted=ch-5, got %s", store.deleted)
	}
}

func TestChannelsHandlerDeleteNotFound(t *testing.T) {
	store := &mockChannelsStore{}
	mux := setupChannelsMux(store)

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodDelete, "/channels/nonexistent", nil)
	mux.ServeHTTP(w, r)

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected status 404, got %d", w.Code)
	}
}

func TestChannelsHandlerTestSuccess(t *testing.T) {
	sender := &mockSender{}
	store := &mockChannelsStore{
		channels: []models.NotificationChannel{
			{ID: "ch-1", Type: "webhook", Name: "test-hook", Config: json.RawMessage(`{"url":"https://example.com/hook"}`)},
		},
	}
	mux := setupChannelsMuxWithSenders(store, map[string]routing.Sender{"webhook": sender})

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/channels/ch-1/test", nil)
	mux.ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d; body: %s", w.Code, w.Body.String())
	}
	if sender.sent == nil {
		t.Fatal("expected sender.Send to be called")
	}
	if sender.sent.Title != "Test notification from Changelogue" {
		t.Fatalf("unexpected title: %s", sender.sent.Title)
	}
}

func TestChannelsHandlerTestNotFound(t *testing.T) {
	store := &mockChannelsStore{}
	mux := setupChannelsMuxWithSenders(store, map[string]routing.Sender{"webhook": &mockSender{}})

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/channels/nonexistent/test", nil)
	mux.ServeHTTP(w, r)

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected status 404, got %d", w.Code)
	}
}

func TestChannelsHandlerTestUnsupportedType(t *testing.T) {
	store := &mockChannelsStore{
		channels: []models.NotificationChannel{
			{ID: "ch-1", Type: "telegram", Name: "tg-chan", Config: json.RawMessage(`{}`)},
		},
	}
	mux := setupChannelsMuxWithSenders(store, map[string]routing.Sender{"webhook": &mockSender{}})

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/channels/ch-1/test", nil)
	mux.ServeHTTP(w, r)

	if w.Code != http.StatusUnprocessableEntity {
		t.Fatalf("expected status 422, got %d", w.Code)
	}
}

func TestChannelsHandlerTestSendError(t *testing.T) {
	sender := &mockSender{err: fmt.Errorf("connection refused")}
	store := &mockChannelsStore{
		channels: []models.NotificationChannel{
			{ID: "ch-1", Type: "webhook", Name: "broken-hook", Config: json.RawMessage(`{"url":"https://example.com/hook"}`)},
		},
	}
	mux := setupChannelsMuxWithSenders(store, map[string]routing.Sender{"webhook": sender})

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/channels/ch-1/test", nil)
	mux.ServeHTTP(w, r)

	if w.Code != http.StatusBadGateway {
		t.Fatalf("expected status 502, got %d", w.Code)
	}
	if !strings.Contains(w.Body.String(), "connection refused") {
		t.Fatalf("expected error message in body, got: %s", w.Body.String())
	}
}
