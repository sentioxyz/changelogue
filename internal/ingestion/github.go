package ingestion

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"time"
)

// GitHubWebhookHandler handles incoming GitHub release webhook events.
// It validates the HMAC signature, parses the release payload, and
// forwards results via the onResult callback.
type GitHubWebhookHandler struct {
	secret   string
	onResult func([]IngestionResult)
}

func NewGitHubWebhookHandler(secret string, onResult func([]IngestionResult)) *GitHubWebhookHandler {
	return &GitHubWebhookHandler{secret: secret, onResult: onResult}
}

func (h *GitHubWebhookHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "read body", http.StatusBadRequest)
		return
	}

	if !h.verifySignature(body, r.Header.Get("X-Hub-Signature-256")) {
		http.Error(w, "invalid signature", http.StatusForbidden)
		return
	}

	if r.Header.Get("X-GitHub-Event") != "release" {
		w.WriteHeader(http.StatusOK)
		return
	}

	var payload struct {
		Action  string `json:"action"`
		Release struct {
			TagName     string `json:"tag_name"`
			Body        string `json:"body"`
			PreRelease  bool   `json:"prerelease"`
			PublishedAt string `json:"published_at"`
		} `json:"release"`
		Repository struct {
			FullName string `json:"full_name"`
		} `json:"repository"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		http.Error(w, "invalid json", http.StatusBadRequest)
		return
	}

	if payload.Action != "published" {
		w.WriteHeader(http.StatusOK)
		return
	}

	ts, _ := time.Parse(time.RFC3339, payload.Release.PublishedAt)
	result := IngestionResult{
		Repository: payload.Repository.FullName,
		RawVersion: payload.Release.TagName,
		Changelog:  payload.Release.Body,
		Timestamp:  ts,
	}

	h.onResult([]IngestionResult{result})
	w.WriteHeader(http.StatusOK)
}

func (h *GitHubWebhookHandler) verifySignature(body []byte, header string) bool {
	sig := strings.TrimPrefix(header, "sha256=")
	mac := hmac.New(sha256.New, []byte(h.secret))
	mac.Write(body)
	expected := hex.EncodeToString(mac.Sum(nil))
	return hmac.Equal([]byte(sig), []byte(expected))
}
