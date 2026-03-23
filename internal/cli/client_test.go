package cli

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestClientGet(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("expected GET, got %s", r.Method)
		}
		if r.URL.Path != "/api/v1/projects" {
			t.Errorf("expected /api/v1/projects, got %s", r.URL.Path)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer test-key" {
			t.Errorf("expected Bearer test-key, got %s", got)
		}
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]any{"data": []any{}, "meta": map[string]any{"request_id": "r1"}})
	}))
	defer srv.Close()

	c := NewClient(srv.URL, "test-key")
	resp, err := c.Get("/api/v1/projects")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
}

func TestClientPost(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", r.Method)
		}
		if ct := r.Header.Get("Content-Type"); ct != "application/json" {
			t.Errorf("expected application/json, got %s", ct)
		}
		var body map[string]string
		json.NewDecoder(r.Body).Decode(&body)
		if body["name"] != "test-project" {
			t.Errorf("expected name=test-project, got %s", body["name"])
		}
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(map[string]any{"data": body, "meta": map[string]any{"request_id": "r2"}})
	}))
	defer srv.Close()

	c := NewClient(srv.URL, "test-key")
	resp, err := c.Post("/api/v1/projects", map[string]string{"name": "test-project"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("expected 201, got %d", resp.StatusCode)
	}
}

func TestClientHandlesErrorResponse(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		json.NewEncoder(w).Encode(map[string]any{
			"error": map[string]string{"code": "unauthorized", "message": "Invalid API key"},
			"meta":  map[string]any{"request_id": "r3"},
		})
	}))
	defer srv.Close()

	c := NewClient(srv.URL, "bad-key")
	resp, err := c.Get("/api/v1/projects")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer resp.Body.Close()

	apiErr, err := DecodeError(resp)
	if err != nil {
		t.Fatalf("failed to decode error: %v", err)
	}
	if apiErr.Err.Code != "unauthorized" {
		t.Errorf("expected code unauthorized, got %s", apiErr.Err.Code)
	}
}

func TestDecodeResponse(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]any{
			"data": map[string]string{"id": "abc", "name": "proj"},
			"meta": map[string]any{"request_id": "r4"},
		})
	}))
	defer srv.Close()

	c := NewClient(srv.URL, "key")
	resp, _ := c.Get("/test")
	defer resp.Body.Close()

	type item struct {
		ID   string `json:"id"`
		Name string `json:"name"`
	}
	var result APIResponse[item]
	if err := DecodeJSON(resp, &result); err != nil {
		t.Fatalf("decode failed: %v", err)
	}
	if result.Data.ID != "abc" {
		t.Errorf("expected id abc, got %s", result.Data.ID)
	}
}
