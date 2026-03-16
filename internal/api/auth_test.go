package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/sentioxyz/changelogue/internal/auth"
)

type mockKeyStore struct{ valid bool }

func (m *mockKeyStore) ValidateKey(_ context.Context, _ string) (bool, error) { return m.valid, nil }
func (m *mockKeyStore) TouchKeyUsage(_ context.Context, _ string)            {}

func TestAuthMiddlewareValidKey(t *testing.T) {
	store := &mockKeyStore{valid: true}
	handler := Auth(store, nil)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r.Header.Set("Authorization", "Bearer valid-key-123")
	handler.ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", w.Code)
	}
}

func TestAuthMiddlewareMissingKey(t *testing.T) {
	store := &mockKeyStore{valid: true}
	handler := Auth(store, nil)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	// No Authorization header
	handler.ServeHTTP(w, r)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected status 401, got %d", w.Code)
	}

	var got struct {
		Error struct {
			Code    string `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.NewDecoder(w.Body).Decode(&got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if got.Error.Code != "unauthorized" {
		t.Fatalf("expected error.code=unauthorized, got %s", got.Error.Code)
	}
	if got.Error.Message != "Missing API key" {
		t.Fatalf("expected message=Missing API key, got %s", got.Error.Message)
	}
}

func TestAuthMiddlewareInvalidKey(t *testing.T) {
	store := &mockKeyStore{valid: false}
	handler := Auth(store, nil)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r.Header.Set("Authorization", "Bearer bad-key")
	handler.ServeHTTP(w, r)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected status 401, got %d", w.Code)
	}

	var got struct {
		Error struct {
			Code    string `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.NewDecoder(w.Body).Decode(&got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if got.Error.Code != "unauthorized" {
		t.Fatalf("expected error.code=unauthorized, got %s", got.Error.Code)
	}
	if got.Error.Message != "Invalid API key" {
		t.Fatalf("expected message=Invalid API key, got %s", got.Error.Message)
	}
}

func TestRateLimitMiddleware(t *testing.T) {
	handler := RateLimit(1, 2)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	// First two requests should succeed (burst=2)
	for i := range 2 {
		w := httptest.NewRecorder()
		r := httptest.NewRequest(http.MethodGet, "/", nil)
		r.RemoteAddr = "192.168.1.1:12345"
		handler.ServeHTTP(w, r)
		if w.Code != http.StatusOK {
			t.Fatalf("request %d: expected status 200, got %d", i+1, w.Code)
		}
	}

	// Third request should be rate limited
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r.RemoteAddr = "192.168.1.1:12345"
	handler.ServeHTTP(w, r)

	if w.Code != http.StatusTooManyRequests {
		t.Fatalf("third request: expected status 429, got %d", w.Code)
	}
	if v := w.Header().Get("Retry-After"); v != "1" {
		t.Fatalf("expected Retry-After=1, got %s", v)
	}

	var got struct {
		Error struct {
			Code string `json:"code"`
		} `json:"error"`
	}
	if err := json.NewDecoder(w.Body).Decode(&got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if got.Error.Code != "rate_limited" {
		t.Fatalf("expected error.code=rate_limited, got %s", got.Error.Code)
	}
}

func TestRateLimitKeysByBearer(t *testing.T) {
	handler := RateLimit(1, 1)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	// First request with key A should succeed
	w1 := httptest.NewRecorder()
	r1 := httptest.NewRequest(http.MethodGet, "/", nil)
	r1.Header.Set("Authorization", "Bearer key-a")
	handler.ServeHTTP(w1, r1)
	if w1.Code != http.StatusOK {
		t.Fatalf("key-a first request: expected 200, got %d", w1.Code)
	}

	// First request with key B should also succeed (separate limiter)
	w2 := httptest.NewRecorder()
	r2 := httptest.NewRequest(http.MethodGet, "/", nil)
	r2.Header.Set("Authorization", "Bearer key-b")
	handler.ServeHTTP(w2, r2)
	if w2.Code != http.StatusOK {
		t.Fatalf("key-b first request: expected 200, got %d", w2.Code)
	}

	// Second request with key A should be rate limited
	w3 := httptest.NewRecorder()
	r3 := httptest.NewRequest(http.MethodGet, "/", nil)
	r3.Header.Set("Authorization", "Bearer key-a")
	handler.ServeHTTP(w3, r3)
	if w3.Code != http.StatusTooManyRequests {
		t.Fatalf("key-a second request: expected 429, got %d", w3.Code)
	}
}

type mockSessionValidator struct {
	user *auth.User
}

func (m *mockSessionValidator) ValidateSession(_ context.Context, _ string) (*auth.User, error) {
	if m.user != nil {
		return m.user, nil
	}
	return nil, fmt.Errorf("invalid session")
}

func TestAuthMiddlewareSessionCookie(t *testing.T) {
	keys := &mockKeyStore{valid: false}
	sessions := &mockSessionValidator{user: &auth.User{ID: "u1", GitHubLogin: "alice"}}
	handler := Auth(keys, sessions)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		u := auth.UserFromContext(r.Context())
		if u == nil {
			t.Fatal("expected user in context")
		}
		if u.GitHubLogin != "alice" {
			t.Fatalf("expected alice, got %s", u.GitHubLogin)
		}
		w.WriteHeader(http.StatusOK)
	}))

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r.AddCookie(&http.Cookie{Name: "session", Value: "test-cookie"})
	handler.ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
}

func TestAuthMiddlewareBearerTakesPrecedence(t *testing.T) {
	keys := &mockKeyStore{valid: true}
	sessions := &mockSessionValidator{user: nil} // would fail if reached
	handler := Auth(keys, sessions)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r.Header.Set("Authorization", "Bearer valid-key")
	r.AddCookie(&http.Cookie{Name: "session", Value: "test-cookie"})
	handler.ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
}
