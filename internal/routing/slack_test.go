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

// slackTestPayload mirrors slackPayload but with typed Blocks for test assertions.
type slackTestPayload struct {
	Blocks      []slackBlock      `json:"blocks,omitempty"`
	Attachments []slackAttachment `json:"attachments,omitempty"`
}

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
		Title:      "geth",
		Body:       `{"changelog":"Security fixes and performance improvements"}`,
		Version:    "v1.14.0",
		Provider:   "github",
		Repository: "ethereum/go-ethereum",
	}

	err := sender.Send(context.Background(), ch, msg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(received) == 0 {
		t.Fatal("expected Slack to receive payload")
	}

	// Verify it's a valid Slack payload with attachments (has changelog).
	var payload map[string]interface{}
	if err := json.Unmarshal(received, &payload); err != nil {
		t.Fatalf("received invalid JSON: %v", err)
	}
	attachments, ok := payload["attachments"]
	if !ok {
		t.Fatal("expected 'attachments' key in Slack payload")
	}
	attList, ok := attachments.([]interface{})
	if !ok || len(attList) < 1 {
		t.Fatalf("expected at least 1 attachment, got %v", attachments)
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

	var payload slackTestPayload
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

	var payload slackTestPayload
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
		Title:   "geth",
		Body:    "Released on GitHub with security fixes",
		Version: "v1.14.0",
	}

	err := sender.Send(context.Background(), ch, msg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var payload slackTestPayload
	if err := json.Unmarshal(received, &payload); err != nil {
		t.Fatalf("received invalid JSON: %v", err)
	}

	// Fallback with no changelog should use attachment format (consistent with changelog path)
	if len(payload.Attachments) < 1 {
		t.Fatal("expected at least 1 attachment for non-report fallback")
	}
	att := payload.Attachments[0]
	if !strings.Contains(att.Text, "geth") {
		t.Fatalf("attachment text should contain project name, got %q", att.Text)
	}
	if !strings.Contains(att.Text, "v1.14.0") {
		t.Fatalf("attachment text should contain version, got %q", att.Text)
	}
	// Should NOT contain raw body text
	if strings.Contains(att.Text, "Released on GitHub with security fixes") {
		t.Fatal("should not dump raw body text in attachment")
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
		Title:       "zkSync Era",
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

	var payload slackTestPayload
	if err := json.Unmarshal(received, &payload); err != nil {
		t.Fatalf("received invalid JSON: %v", err)
	}

	// Should use attachments for changelog (auto-collapse with "Show more")
	if len(payload.Attachments) != 1 {
		t.Fatalf("expected 1 attachment, got %d", len(payload.Attachments))
	}

	att := payload.Attachments[0]

	// Attachment title should show repo on provider
	if !strings.Contains(att.Title, "matter-labs/zksync-era") {
		t.Fatalf("expected repo in attachment title, got %q", att.Title)
	}
	if !strings.Contains(att.Title, "GitHub") {
		t.Fatalf("expected provider label in attachment title, got %q", att.Title)
	}

	// Attachment text should contain changelog in a code block
	if !strings.Contains(att.Text, "Fixed wrong genesis commit") {
		t.Fatal("expected changelog text in attachment")
	}
	if !strings.Contains(att.Text, "```") {
		t.Fatal("changelog should be wrapped in code block")
	}
	if strings.Contains(att.Text, "release_url") {
		t.Fatal("raw JSON keys should not appear in formatted output")
	}

	// Title link should point to source
	if att.TitleLink != "https://github.com/matter-labs/zksync-era/releases/tag/zkos-0.29.4-rc1" {
		t.Fatalf("unexpected title_link: %s", att.TitleLink)
	}

	// Links should be in the attachment text
	if !strings.Contains(att.Text, "View on GitHub") {
		t.Fatal("expected 'View on GitHub' link in attachment text")
	}
	if !strings.Contains(att.Text, "View in Changelogue") {
		t.Fatal("expected 'View in Changelogue' link in attachment text")
	}

	// No separate blocks needed
	if len(payload.Blocks) != 0 {
		t.Fatalf("expected no blocks (all in attachment), got %d", len(payload.Blocks))
	}
}

func TestSlackSender_EmptyChangelog_NoRawJSON(t *testing.T) {
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

	tests := []struct {
		name string
		msg  Notification
	}{
		{
			name: "github release with metadata but no changelog",
			msg: Notification{
				Title:      "zkSync Era",
				Body:       `{"prerelease":"false","release_url":"https://github.com/matter-labs/zksync-era/releases/tag/test-release-aba"}`,
				Version:    "test-release-aba",
				Provider:   "github",
				Repository: "matter-labs/zksync-era",
				SourceURL:  "https://github.com/matter-labs/zksync-era/releases/tag/test-release-aba",
				ReleaseURL: "https://changelogue.example.com/releases/rel-1",
			},
		},
		{
			name: "dockerhub release with empty metadata",
			msg: Notification{
				Title:      "external-node",
				Body:       `{}`,
				Version:    "v25.1.0",
				Provider:   "dockerhub",
				Repository: "matterlabs/external-node",
				SourceURL:  "https://hub.docker.com/r/matterlabs/external-node/tags?name=v25.1.0",
				ReleaseURL: "https://changelogue.example.com/releases/rel-2",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := sender.Send(context.Background(), ch, tt.msg)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			var payload slackTestPayload
			if err := json.Unmarshal(received, &payload); err != nil {
				t.Fatalf("received invalid JSON: %v", err)
			}

			// Should use attachments (consistent with changelog path)
			if len(payload.Attachments) < 1 {
				t.Fatal("expected at least 1 attachment")
			}

			att := payload.Attachments[0]

			// Attachment text should include project name and version
			if !strings.Contains(att.Text, tt.msg.Title) {
				t.Fatalf("attachment text should contain project name %q, got %q", tt.msg.Title, att.Text)
			}
			if !strings.Contains(att.Text, tt.msg.Version) {
				t.Fatalf("attachment text should contain version %q, got %q", tt.msg.Version, att.Text)
			}

			// Attachment title should show repo on provider
			if tt.msg.Repository != "" && tt.msg.Provider != "" {
				if !strings.Contains(att.Title, tt.msg.Repository) {
					t.Fatalf("expected repo in attachment title, got %q", att.Title)
				}
			}

			// Should NOT contain raw JSON in attachment text
			if strings.Contains(att.Text, "prerelease") {
				t.Fatalf("should not contain raw JSON key 'prerelease' in text: %q", att.Text)
			}

			// Should have links
			if tt.msg.SourceURL != "" && !strings.Contains(att.Text, "View on") {
				t.Fatal("expected 'View on' link in attachment text")
			}
			if tt.msg.ReleaseURL != "" && !strings.Contains(att.Text, "View in Changelogue") {
				t.Fatal("expected 'View in Changelogue' link in attachment text")
			}
		})
	}
}

func TestMarkdownToASCII(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		contains []string
		excludes []string
	}{
		{
			name:     "heading converted to ASCII art",
			input:    "## Protocol",
			contains: []string{"--------\nProtocol\n--------"},
			excludes: []string{"##"},
		},
		{
			name:     "h1 heading",
			input:    "# Release Notes",
			contains: []string{"-------------\nRelease Notes\n-------------"},
			excludes: []string{"# "},
		},
		{
			name:     "GitHub PR URL converted",
			input:    "Fixed bug https://github.com/MystenLabs/sui/pull/25364 today",
			contains: []string{"#25364 ( https://github.com/MystenLabs/sui/pull/25364 )"},
		},
		{
			name:     "GitHub issue URL converted",
			input:    "See https://github.com/org/repo/issues/123",
			contains: []string{"#123 ( https://github.com/org/repo/issues/123 )"},
		},
		{
			name:     "markdown link converted",
			input:    "Check [the docs](https://example.com/docs) for details",
			contains: []string{"the docs ( https://example.com/docs )"},
			excludes: []string{"[the docs]"},
		},
		{
			name:     "bold stripped",
			input:    "This is **important** text",
			contains: []string{"important"},
			excludes: []string{"**"},
		},
		{
			name:     "inline code stripped",
			input:    "Run `npm install` to start",
			contains: []string{"npm install"},
			excludes: []string{"`"},
		},
		{
			name:     "image stripped",
			input:    "![screenshot](https://example.com/img.png)",
			contains: []string{"screenshot"},
			excludes: []string{"![", "https://example.com/img.png"},
		},
		{
			name:     "HTML tags stripped",
			input:    "Text <br/> more <b>bold</b> text",
			contains: []string{"Text", "more", "bold", "text"},
			excludes: []string{"<br/>", "<b>", "</b>"},
		},
		{
			name:     "setext h2 heading (dashes)",
			input:    "Summary of Changes\n------------------",
			contains: []string{"------------------\nSummary of Changes\n------------------"},
		},
		{
			name:     "setext h1 heading (equals)",
			input:    "Release Notes\n=============",
			contains: []string{"-------------\nRelease Notes\n-------------"},
		},
		{
			name:  "cilium-style changelog with setext heading",
			input: "Summary of Changes\n------------------\n\n* Fixed a bug in endpoint routing\n* Updated Hubble version to v1.2.3",
			contains: []string{
				"------------------\nSummary of Changes\n------------------",
				"Fixed a bug in endpoint routing",
			},
		},
		{
			name:  "full changelog example",
			input: "## What's Changed\n\n* feat: add monitoring by @dev in https://github.com/org/repo/pull/42\n* fix: **memory leak** in `worker`\n\n**Full Changelog**: [v0.9...v1.0](https://github.com/org/repo/compare/v0.9.0...v1.0.0)",
			contains: []string{
				"What's Changed",
				"#42 ( https://github.com/org/repo/pull/42 )",
				"memory leak",
				"worker",
			},
			excludes: []string{"##", "**", "`worker`", "[v0.9...v1.0]"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := markdownToASCII(tt.input)
			for _, want := range tt.contains {
				if !strings.Contains(result, want) {
					t.Errorf("expected result to contain %q, got:\n%s", want, result)
				}
			}
			for _, exclude := range tt.excludes {
				if strings.Contains(result, exclude) {
					t.Errorf("expected result NOT to contain %q, got:\n%s", exclude, result)
				}
			}
		})
	}
}
