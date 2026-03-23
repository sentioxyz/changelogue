package cli

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/sentioxyz/changelogue/internal/models"
)

func TestChannelsList(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/channels" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		json.NewEncoder(w).Encode(map[string]any{
			"data": []models.NotificationChannel{{ID: "ch1", Name: "my-slack", Type: "slack"}},
			"meta": map[string]any{"request_id": "r1", "page": 1, "per_page": 25, "total": 1},
		})
	}))
	defer srv.Close()

	c := NewClient(srv.URL, "key")
	channels, _, err := ListChannels(c, 1, 25)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(channels) != 1 || channels[0].Type != "slack" {
		t.Errorf("unexpected channels: %+v", channels)
	}
}

func TestChannelsTest(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/api/v1/channels/ch1/test" {
			t.Errorf("unexpected %s %s", r.Method, r.URL.Path)
		}
		json.NewEncoder(w).Encode(map[string]any{
			"data": map[string]string{"status": "ok"},
			"meta": map[string]any{"request_id": "r2"},
		})
	}))
	defer srv.Close()

	c := NewClient(srv.URL, "key")
	err := TestChannel(c, "ch1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}
