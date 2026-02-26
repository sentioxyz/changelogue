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

func TestSlackSender_Send(t *testing.T) {
	var received []byte
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Content-Type") != "application/json" {
			t.Errorf("expected Content-Type application/json, got %s", r.Header.Get("Content-Type"))
		}
		received, _ = io.ReadAll(r.Body)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	sender := &SlackSender{Client: srv.Client()}
	ch := &models.NotificationChannel{
		Type:   "slack",
		Config: json.RawMessage(`{"webhook_url": "` + srv.URL + `"}`),
	}
	msg := Notification{
		Title:   "New release: geth v1.14.0",
		Body:    "Released on GitHub with security fixes",
		Version: "v1.14.0",
	}

	err := sender.Send(context.Background(), ch, msg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(received) == 0 {
		t.Fatal("expected Slack to receive payload")
	}

	// Verify it's a valid Slack Block Kit payload with blocks.
	var payload map[string]interface{}
	if err := json.Unmarshal(received, &payload); err != nil {
		t.Fatalf("received invalid JSON: %v", err)
	}
	blocks, ok := payload["blocks"]
	if !ok {
		t.Fatal("expected 'blocks' key in Slack payload")
	}
	blockList, ok := blocks.([]interface{})
	if !ok || len(blockList) < 2 {
		t.Fatalf("expected at least 2 blocks, got %v", blocks)
	}
}

func TestSlackSender_SendError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	sender := &SlackSender{Client: srv.Client()}
	ch := &models.NotificationChannel{
		Type:   "slack",
		Config: json.RawMessage(`{"webhook_url": "` + srv.URL + `"}`),
	}

	err := sender.Send(context.Background(), ch, Notification{Title: "test", Body: "test"})
	if err == nil {
		t.Fatal("expected error for 500 response")
	}
}

func TestSlackSender_InvalidConfig(t *testing.T) {
	sender := &SlackSender{}
	ch := &models.NotificationChannel{
		Type:   "slack",
		Config: json.RawMessage(`{invalid`),
	}

	err := sender.Send(context.Background(), ch, Notification{})
	if err == nil {
		t.Fatal("expected error for invalid config")
	}
}
