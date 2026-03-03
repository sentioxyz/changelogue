package routing

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/sentioxyz/changelogue/internal/models"
)

func TestDiscordSender_Send(t *testing.T) {
	var received []byte
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Content-Type") != "application/json" {
			t.Errorf("expected Content-Type application/json, got %s", r.Header.Get("Content-Type"))
		}
		received, _ = io.ReadAll(r.Body)
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()

	sender := &DiscordSender{Client: srv.Client()}
	ch := &models.NotificationChannel{
		Type:   "discord",
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
		t.Fatal("expected Discord to receive payload")
	}

	// Verify it's a valid Discord webhook payload with embeds.
	var payload map[string]interface{}
	if err := json.Unmarshal(received, &payload); err != nil {
		t.Fatalf("received invalid JSON: %v", err)
	}
	embeds, ok := payload["embeds"]
	if !ok {
		t.Fatal("expected 'embeds' key in Discord payload")
	}
	embedList, ok := embeds.([]interface{})
	if !ok || len(embedList) < 1 {
		t.Fatalf("expected at least 1 embed, got %v", embeds)
	}

	// Verify the embed contains our title.
	embed := embedList[0].(map[string]interface{})
	if title, ok := embed["title"].(string); !ok || title != msg.Title {
		t.Errorf("expected embed title %q, got %q", msg.Title, title)
	}
}

func TestDiscordSender_SendError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	sender := &DiscordSender{Client: srv.Client()}
	ch := &models.NotificationChannel{
		Type:   "discord",
		Config: json.RawMessage(`{"webhook_url": "` + srv.URL + `"}`),
	}

	err := sender.Send(context.Background(), ch, Notification{Title: "test", Body: "test"})
	if err == nil {
		t.Fatal("expected error for 500 response")
	}
}

func TestDiscordSender_InvalidConfig(t *testing.T) {
	sender := &DiscordSender{}
	ch := &models.NotificationChannel{
		Type:   "discord",
		Config: json.RawMessage(`{invalid`),
	}

	err := sender.Send(context.Background(), ch, Notification{})
	if err == nil {
		t.Fatal("expected error for invalid config")
	}
}

func TestDiscordSender_BodyTruncation(t *testing.T) {
	var received []byte
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		received, _ = io.ReadAll(r.Body)
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()

	// Create a body longer than Discord's embed description limit (4096 chars).
	longBody := make([]byte, 5000)
	for i := range longBody {
		longBody[i] = 'a'
	}

	sender := &DiscordSender{Client: srv.Client()}
	ch := &models.NotificationChannel{
		Type:   "discord",
		Config: json.RawMessage(`{"webhook_url": "` + srv.URL + `"}`),
	}
	msg := Notification{
		Title:   "test",
		Body:    string(longBody),
		Version: "v1.0.0",
	}

	err := sender.Send(context.Background(), ch, msg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify the body was truncated in the embed description.
	var payload map[string]interface{}
	if err := json.Unmarshal(received, &payload); err != nil {
		t.Fatalf("received invalid JSON: %v", err)
	}
	embeds := payload["embeds"].([]interface{})
	embed := embeds[0].(map[string]interface{})
	desc := embed["description"].(string)
	if len(desc) > 4096 {
		t.Errorf("expected description to be at most 4096 chars, got %d", len(desc))
	}
}

func TestDiscordSender_SemanticReport(t *testing.T) {
	var received []byte
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		received, _ = io.ReadAll(r.Body)
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()

	sender := &DiscordSender{Client: srv.Client()}
	ch := &models.NotificationChannel{
		Type:   "discord",
		Config: json.RawMessage(`{"webhook_url": "` + srv.URL + `"}`),
	}

	reportJSON := `{
		"subject": "Ready to Deploy: go-ethereum v1.16.4",
		"urgency": "Critical",
		"urgency_reason": "Hard fork — deploy before block 18M",
		"changelog_summary": "Consensus-critical update",
		"download_commands": ["docker pull ethereum/client-go:v1.16.4"],
		"availability": "GA"
	}`

	msg := Notification{
		Title:       "go-ethereum v1.16.4",
		Body:        reportJSON,
		Version:     "v1.16.4",
		ProjectName: "go-ethereum",
		Provider:    "github",
		Repository:  "ethereum/go-ethereum",
		SourceURL:   "https://github.com/ethereum/go-ethereum/releases/tag/v1.16.4",
		ReleaseURL:  "https://changelogue.example.com/sr/1",
	}

	err := sender.Send(context.Background(), ch, msg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var payload discordPayload
	if err := json.Unmarshal(received, &payload); err != nil {
		t.Fatalf("received invalid JSON: %v", err)
	}

	if len(payload.Embeds) != 1 {
		t.Fatalf("expected 1 embed, got %d", len(payload.Embeds))
	}

	embed := payload.Embeds[0]

	// Should contain urgency reason for Critical
	if !strings.Contains(embed.Description, "Hard fork") {
		t.Fatal("critical urgency should include urgency_reason in description")
	}

	// Should contain changelog
	if !strings.Contains(embed.Description, "Consensus-critical update") {
		t.Fatal("should include changelog summary")
	}

	// Should NOT have inline fields (compact format removes them)
	if len(embed.Fields) > 0 {
		t.Fatalf("compact format should not have embed fields, got %d", len(embed.Fields))
	}

	// Color should be CRITICAL near-black
	if embed.Color != 0x111113 {
		t.Fatalf("expected critical color 0x111113, got 0x%06X", embed.Color)
	}
}
