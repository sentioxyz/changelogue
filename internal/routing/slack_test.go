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

func TestSlackSender_SemanticReport(t *testing.T) {
	var received []byte
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		received, _ = io.ReadAll(r.Body)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	sender := &SlackSender{Client: srv.Client()}
	ch := &models.NotificationChannel{
		Type:   "slack",
		Config: json.RawMessage(`{"webhook_url": "` + srv.URL + `"}`),
	}

	reportJSON := `{
		"subject": "Ready to Deploy: go-ethereum v1.16.4",
		"urgency": "High",
		"urgency_reason": "Security vulnerability patched",
		"status_checks": ["Binaries Unverified", "Docker Image Unverified"],
		"changelog_summary": "Security fixes and performance improvements",
		"availability": "GA",
		"adoption": "Recommended for production",
		"recommendation": "Deploy after verifying checksums",
		"download_commands": ["docker pull ethereum/client-go:v1.16.4"],
		"download_links": ["https://github.com/ethereum/go-ethereum/releases/tag/v1.16.4"]
	}`

	msg := Notification{
		Title:       "Semantic Release Report: go-ethereum v1.16.4",
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

	var payload slackPayload
	if err := json.Unmarshal(received, &payload); err != nil {
		t.Fatalf("received invalid JSON: %v", err)
	}

	// Compact format: header + urgency_reason (High) + changelog + download cmd + context = 5 blocks
	if len(payload.Blocks) < 3 || len(payload.Blocks) > 6 {
		t.Fatalf("expected 3-6 blocks for compact semantic report, got %d", len(payload.Blocks))
	}

	// First block should be header containing project name, version, and urgency
	if payload.Blocks[0].Type != "header" {
		t.Fatalf("expected first block to be header, got %s", payload.Blocks[0].Type)
	}
	headerText := payload.Blocks[0].Text.Text
	if !strings.Contains(headerText, "go-ethereum") || !strings.Contains(headerText, "v1.16.4") {
		t.Fatalf("header should contain project name and version, got %q", headerText)
	}
	if !strings.Contains(headerText, "High") {
		t.Fatalf("header should contain urgency level, got %q", headerText)
	}

	// Should NOT have a fields section (no separate risk/urgency/availability fields)
	for _, b := range payload.Blocks {
		if len(b.Fields) > 0 {
			t.Fatal("compact format should not have field sections")
		}
	}

	// Should have download command
	hasCode := false
	for _, b := range payload.Blocks {
		if b.Text != nil && strings.Contains(b.Text.Text, "docker pull") {
			hasCode = true
		}
	}
	if !hasCode {
		t.Fatal("expected download command")
	}

	// Last block should be context footer
	lastBlock := payload.Blocks[len(payload.Blocks)-1]
	if lastBlock.Type != "context" {
		t.Fatalf("expected last block to be context, got %s", lastBlock.Type)
	}
}

func TestSlackSender_SemanticReport_LowUrgency(t *testing.T) {
	var received []byte
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		received, _ = io.ReadAll(r.Body)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	sender := &SlackSender{Client: srv.Client()}
	ch := &models.NotificationChannel{
		Type:   "slack",
		Config: json.RawMessage(`{"webhook_url": "` + srv.URL + `"}`),
	}

	reportJSON := `{
		"subject": "go-ethereum v1.16.5",
		"urgency": "Low",
		"urgency_reason": "Minor dependency update",
		"changelog_summary": "Bumped go-libp2p to v0.30.0",
		"availability": "GA",
		"download_commands": ["docker pull ethereum/client-go:v1.16.5"]
	}`

	msg := Notification{
		Title:       "Semantic Release Report: go-ethereum v1.16.5",
		Body:        reportJSON,
		Version:     "v1.16.5",
		ProjectName: "go-ethereum",
		Provider:    "github",
		Repository:  "ethereum/go-ethereum",
	}

	err := sender.Send(context.Background(), ch, msg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var payload slackPayload
	if err := json.Unmarshal(received, &payload); err != nil {
		t.Fatalf("received invalid JSON: %v", err)
	}

	// Low urgency should NOT include urgency_reason block
	for _, b := range payload.Blocks {
		if b.Text != nil && strings.Contains(b.Text.Text, "Minor dependency update") {
			t.Fatal("low urgency should not include urgency_reason in notification")
		}
	}
}

func TestSlackSender_NonReportFallback(t *testing.T) {
	var received []byte
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		received, _ = io.ReadAll(r.Body)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	sender := &SlackSender{Client: srv.Client()}
	ch := &models.NotificationChannel{
		Type:   "slack",
		Config: json.RawMessage(`{"webhook_url": "` + srv.URL + `"}`),
	}

	// Plain text body (not a semantic report) should use simple fallback
	msg := Notification{
		Title:   "New release: geth v1.14.0",
		Body:    "Released on GitHub with security fixes",
		Version: "v1.14.0",
	}

	err := sender.Send(context.Background(), ch, msg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var payload slackPayload
	if err := json.Unmarshal(received, &payload); err != nil {
		t.Fatalf("received invalid JSON: %v", err)
	}

	// Fallback should produce exactly 2 blocks (header + section)
	if len(payload.Blocks) != 2 {
		t.Fatalf("expected 2 blocks for non-report, got %d", len(payload.Blocks))
	}
	if payload.Blocks[0].Type != "header" {
		t.Fatalf("expected header block, got %s", payload.Blocks[0].Type)
	}
	if payload.Blocks[1].Type != "section" {
		t.Fatalf("expected section block, got %s", payload.Blocks[1].Type)
	}
}

func TestSlackSender_RawJSONFallback(t *testing.T) {
	var received []byte
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		received, _ = io.ReadAll(r.Body)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	sender := &SlackSender{Client: srv.Client()}
	ch := &models.NotificationChannel{
		Type:   "slack",
		Config: json.RawMessage(`{"webhook_url": "` + srv.URL + `"}`),
	}

	// Raw release JSON with metadata on the Notification struct (as worker now provides)
	msg := Notification{
		Title:       "zkSync Era — New release: zkos-0.29.4-rc1",
		Body:        `{"changelog":"Fixed wrong genesis commit","prerelease":"false","release_url":"https://github.com/matter-labs/zksync-era/releases/tag/zkos-0.29.4-rc1"}`,
		Version:     "zkos-0.29.4-rc1",
		ProjectName: "zkSync Era",
		Provider:    "github",
		Repository:  "matter-labs/zksync-era",
		SourceURL:   "https://github.com/matter-labs/zksync-era/releases/tag/zkos-0.29.4-rc1",
		ReleaseURL:  "https://changelogue.example.com/releases/rel-1",
	}

	err := sender.Send(context.Background(), ch, msg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var payload slackPayload
	if err := json.Unmarshal(received, &payload); err != nil {
		t.Fatalf("received invalid JSON: %v", err)
	}

	// Should have header + changelog section + context with links = 3 blocks
	if len(payload.Blocks) != 3 {
		t.Fatalf("expected 3 blocks, got %d", len(payload.Blocks))
	}

	// Changelog text should appear in a code block
	changelogBlock := payload.Blocks[1]
	if changelogBlock.Text == nil || !strings.Contains(changelogBlock.Text.Text, "Fixed wrong genesis commit") {
		t.Fatal("expected changelog text in second block")
	}
	if !strings.Contains(changelogBlock.Text.Text, "```") {
		t.Fatal("changelog should be wrapped in code block")
	}
	if strings.Contains(changelogBlock.Text.Text, "release_url") {
		t.Fatal("raw JSON keys should not appear in formatted output")
	}

	// Context block should have source info and links
	contextBlock := payload.Blocks[2]
	if contextBlock.Type != "context" {
		t.Fatalf("expected context block, got %s", contextBlock.Type)
	}
	if len(contextBlock.Elements) == 0 {
		t.Fatal("expected context elements")
	}
	contextText := contextBlock.Elements[0].Text
	if !strings.Contains(contextText, "View on GitHub") {
		t.Fatal("expected 'View on GitHub' link in context")
	}
	if !strings.Contains(contextText, "View in Changelogue") {
		t.Fatal("expected 'View in Changelogue' link in context")
	}
	if !strings.Contains(contextText, "matter-labs/zksync-era") {
		t.Fatal("expected repository name in context")
	}
}
