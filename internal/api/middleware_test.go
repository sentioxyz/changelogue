package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestRequestIDMiddleware(t *testing.T) {
	var capturedID string
	handler := RequestID(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedID = getRequestID(r.Context())
		w.WriteHeader(http.StatusOK)
	}))

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	handler.ServeHTTP(w, r)

	headerID := w.Header().Get("X-Request-ID")
	if headerID == "" {
		t.Fatal("expected X-Request-ID header to be set")
	}
	if capturedID == "" {
		t.Fatal("expected request_id in context")
	}
	if headerID != capturedID {
		t.Fatalf("header ID %q != context ID %q", headerID, capturedID)
	}
}

func TestRecoveryMiddleware(t *testing.T) {
	handler := Recovery(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		panic("test panic")
	}))

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	handler.ServeHTTP(w, r)

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected status 500, got %d", w.Code)
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
	if got.Error.Code != "internal_error" {
		t.Fatalf("expected error.code=internal_error, got %s", got.Error.Code)
	}
}

func TestCORSMiddleware(t *testing.T) {
	handler := CORS(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	// Test preflight OPTIONS
	t.Run("preflight", func(t *testing.T) {
		w := httptest.NewRecorder()
		r := httptest.NewRequest(http.MethodOptions, "/", nil)
		handler.ServeHTTP(w, r)

		if w.Code != http.StatusNoContent {
			t.Fatalf("expected status 204, got %d", w.Code)
		}
		if v := w.Header().Get("Access-Control-Allow-Origin"); v != "*" {
			t.Fatalf("expected Allow-Origin=*, got %s", v)
		}
		if v := w.Header().Get("Access-Control-Allow-Methods"); v == "" {
			t.Fatal("expected Allow-Methods header")
		}
		if v := w.Header().Get("Access-Control-Allow-Headers"); v == "" {
			t.Fatal("expected Allow-Headers header")
		}
		if v := w.Header().Get("Access-Control-Max-Age"); v != "86400" {
			t.Fatalf("expected Max-Age=86400, got %s", v)
		}
	})

	// Test normal request passes through with CORS headers
	t.Run("normal request", func(t *testing.T) {
		w := httptest.NewRecorder()
		r := httptest.NewRequest(http.MethodGet, "/", nil)
		handler.ServeHTTP(w, r)

		if w.Code != http.StatusOK {
			t.Fatalf("expected status 200, got %d", w.Code)
		}
		if v := w.Header().Get("Access-Control-Allow-Origin"); v != "*" {
			t.Fatalf("expected Allow-Origin=*, got %s", v)
		}
	})
}

func TestChain(t *testing.T) {
	var order []string

	mw1 := func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			order = append(order, "mw1-before")
			next.ServeHTTP(w, r)
			order = append(order, "mw1-after")
		})
	}
	mw2 := func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			order = append(order, "mw2-before")
			next.ServeHTTP(w, r)
			order = append(order, "mw2-after")
		})
	}

	chained := Chain(mw1, mw2)
	handler := chained(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		order = append(order, "handler")
		w.WriteHeader(http.StatusOK)
	}))

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	handler.ServeHTTP(w, r)

	expected := []string{"mw1-before", "mw2-before", "handler", "mw2-after", "mw1-after"}
	if len(order) != len(expected) {
		t.Fatalf("expected %d calls, got %d: %v", len(expected), len(order), order)
	}
	for i, v := range expected {
		if order[i] != v {
			t.Fatalf("order[%d]: expected %s, got %s (full: %v)", i, v, order[i], order)
		}
	}
}
