package routing

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/sentioxyz/changelogue/internal/models"
)

func TestWebhookSender_Send(t *testing.T) {
	var received []byte
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Content-Type") != "application/json" {
			t.Errorf("expected Content-Type application/json, got %s", r.Header.Get("Content-Type"))
		}
		received, _ = io.ReadAll(r.Body)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	sender := &WebhookSender{Client: srv.Client()}
	ch := &models.NotificationChannel{
		Type:   "webhook",
		Config: json.RawMessage(`{"url": "` + srv.URL + `"}`),
	}
	msg := Notification{
		Title:   "New release: geth v1.14.0",
		Body:    "Released on GitHub",
		Version: "v1.14.0",
	}

	err := sender.Send(context.Background(), ch, msg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(received) == 0 {
		t.Fatal("expected webhook to receive payload")
	}

	// Verify the payload is valid JSON containing our notification fields.
	var payload Notification
	if err := json.Unmarshal(received, &payload); err != nil {
		t.Fatalf("received invalid JSON: %v", err)
	}
	if payload.Title != msg.Title {
		t.Errorf("expected title %q, got %q", msg.Title, payload.Title)
	}
	if payload.Version != msg.Version {
		t.Errorf("expected version %q, got %q", msg.Version, payload.Version)
	}
}

func TestWebhookSender_SendError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	sender := &WebhookSender{Client: srv.Client()}
	ch := &models.NotificationChannel{
		Type:   "webhook",
		Config: json.RawMessage(`{"url": "` + srv.URL + `"}`),
	}
	msg := Notification{Title: "test", Body: "test"}

	err := sender.Send(context.Background(), ch, msg)
	if err == nil {
		t.Fatal("expected error for 500 response")
	}
}

func TestWebhookSender_InvalidConfig(t *testing.T) {
	sender := &WebhookSender{}
	ch := &models.NotificationChannel{
		Type:   "webhook",
		Config: json.RawMessage(`{invalid`),
	}

	err := sender.Send(context.Background(), ch, Notification{})
	if err == nil {
		t.Fatal("expected error for invalid config")
	}
}

func TestWebhookSender_NilClient(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	// A sender with nil Client should fall back to http.DefaultClient.
	sender := &WebhookSender{}
	ch := &models.NotificationChannel{
		Type:   "webhook",
		Config: json.RawMessage(`{"url": "` + srv.URL + `"}`),
	}

	err := sender.Send(context.Background(), ch, Notification{Title: "test"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}
