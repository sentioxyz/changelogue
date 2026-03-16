package auth

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestHandleGitHubRedirect(t *testing.T) {
	h := &OAuthHandler{
		ClientID: "test-client-id",
		States:   NewStateStore(100, 10*time.Minute),
	}

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/auth/github", nil)
	h.HandleGitHubRedirect(w, r)

	if w.Code != http.StatusFound {
		t.Fatalf("expected 302, got %d", w.Code)
	}
	loc := w.Header().Get("Location")
	if loc == "" {
		t.Fatal("expected Location header")
	}
	if !strings.Contains(loc, "github.com/login/oauth/authorize") {
		t.Fatalf("expected GitHub authorize URL, got %s", loc)
	}
	if !strings.Contains(loc, "client_id=test-client-id") {
		t.Fatalf("expected client_id in URL, got %s", loc)
	}
}

func TestHandleGitHubRedirectWhenFull(t *testing.T) {
	h := &OAuthHandler{
		ClientID: "test-client-id",
		States:   NewStateStore(0, 10*time.Minute), // zero capacity
	}

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/auth/github", nil)
	h.HandleGitHubRedirect(w, r)

	if w.Code != http.StatusTooManyRequests {
		t.Fatalf("expected 429, got %d", w.Code)
	}
}

func TestHandleMeNoSession(t *testing.T) {
	h := &OAuthHandler{}

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/auth/me", nil)
	h.HandleMe(w, r)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", w.Code)
	}
}

func TestHandleMeWithUser(t *testing.T) {
	h := &OAuthHandler{}

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/auth/me", nil)
	ctx := WithUser(r.Context(), &User{
		ID:          "user-1",
		GitHubID:    12345,
		GitHubLogin: "alice",
		Name:        "Alice",
		AvatarURL:   "https://github.com/alice.png",
	})
	r = r.WithContext(ctx)
	h.HandleMe(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
}
