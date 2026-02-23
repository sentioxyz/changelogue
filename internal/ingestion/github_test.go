package ingestion

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"testing"
)

func signPayload(payload []byte, secret string) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(payload)
	return hex.EncodeToString(mac.Sum(nil))
}

func TestGitHubWebhookHandler(t *testing.T) {
	var received []IngestionResult
	handler := NewGitHubWebhookHandler("test-secret", func(results []IngestionResult) {
		received = append(received, results...)
	})

	payload := []byte(`{
		"action": "published",
		"release": {
			"tag_name": "v1.5.0",
			"body": "## Changes\n* Fix login bug",
			"prerelease": false,
			"published_at": "2024-01-15T10:00:00Z"
		},
		"repository": {
			"full_name": "org/myapp"
		}
	}`)

	sig := signPayload(payload, "test-secret")
	req := httptest.NewRequest(http.MethodPost, "/webhook/github", bytes.NewReader(payload))
	req.Header.Set("X-Hub-Signature-256", "sha256="+sig)
	req.Header.Set("X-GitHub-Event", "release")

	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body = %s", w.Code, http.StatusOK, w.Body.String())
	}
	if len(received) != 1 {
		t.Fatalf("received %d results, want 1", len(received))
	}
	if received[0].Repository != "org/myapp" {
		t.Errorf("Repository = %q, want %q", received[0].Repository, "org/myapp")
	}
	if received[0].RawVersion != "v1.5.0" {
		t.Errorf("RawVersion = %q, want %q", received[0].RawVersion, "v1.5.0")
	}
	if received[0].Changelog != "## Changes\n* Fix login bug" {
		t.Errorf("Changelog = %q", received[0].Changelog)
	}
}

func TestGitHubWebhookInvalidSignature(t *testing.T) {
	handler := NewGitHubWebhookHandler("real-secret", func(results []IngestionResult) {
		t.Fatal("callback should not be called")
	})

	payload := []byte(`{"action": "published"}`)
	req := httptest.NewRequest(http.MethodPost, "/webhook/github", bytes.NewReader(payload))
	req.Header.Set("X-Hub-Signature-256", "sha256=invalidsignature")
	req.Header.Set("X-GitHub-Event", "release")

	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Errorf("status = %d, want %d", w.Code, http.StatusForbidden)
	}
}

func TestGitHubWebhookIgnoresNonReleaseEvents(t *testing.T) {
	handler := NewGitHubWebhookHandler("secret", func(results []IngestionResult) {
		t.Fatal("callback should not be called")
	})

	body := []byte(`{}`)
	req := httptest.NewRequest(http.MethodPost, "/webhook/github", bytes.NewReader(body))
	req.Header.Set("X-GitHub-Event", "push")
	req.Header.Set("X-Hub-Signature-256", "sha256="+signPayload(body, "secret"))

	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", w.Code, http.StatusOK)
	}
}

func TestGitHubWebhookIgnoresNonPublishedActions(t *testing.T) {
	handler := NewGitHubWebhookHandler("secret", func(results []IngestionResult) {
		t.Fatal("callback should not be called for non-published actions")
	})

	payload := []byte(`{"action": "created", "release": {"tag_name": "v1.0.0"}, "repository": {"full_name": "a/b"}}`)
	req := httptest.NewRequest(http.MethodPost, "/webhook/github", bytes.NewReader(payload))
	req.Header.Set("X-GitHub-Event", "release")
	req.Header.Set("X-Hub-Signature-256", "sha256="+signPayload(payload, "secret"))

	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", w.Code, http.StatusOK)
	}
}
