package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestRespondJSON(t *testing.T) {
	w := httptest.NewRecorder()
	ctx := withRequestID(context.Background(), "req-123")
	r := httptest.NewRequest(http.MethodGet, "/", nil).WithContext(ctx)

	type payload struct {
		Name string `json:"name"`
	}
	RespondJSON(w, r, http.StatusOK, payload{Name: "test"})

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", w.Code)
	}
	if ct := w.Header().Get("Content-Type"); ct != "application/json" {
		t.Fatalf("expected Content-Type application/json, got %s", ct)
	}

	var got struct {
		Data struct {
			Name string `json:"name"`
		} `json:"data"`
		Meta struct {
			RequestID string `json:"request_id"`
		} `json:"meta"`
	}
	if err := json.NewDecoder(w.Body).Decode(&got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if got.Data.Name != "test" {
		t.Fatalf("expected data.name=test, got %s", got.Data.Name)
	}
	if got.Meta.RequestID != "req-123" {
		t.Fatalf("expected meta.request_id=req-123, got %s", got.Meta.RequestID)
	}
}

func TestRespondError(t *testing.T) {
	w := httptest.NewRecorder()
	ctx := withRequestID(context.Background(), "req-456")
	r := httptest.NewRequest(http.MethodGet, "/", nil).WithContext(ctx)

	RespondError(w, r, http.StatusBadRequest, "bad_request", "Something went wrong")

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d", w.Code)
	}

	var got struct {
		Error struct {
			Code    string `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
		Meta struct {
			RequestID string `json:"request_id"`
		} `json:"meta"`
	}
	if err := json.NewDecoder(w.Body).Decode(&got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if got.Error.Code != "bad_request" {
		t.Fatalf("expected error.code=bad_request, got %s", got.Error.Code)
	}
	if got.Error.Message != "Something went wrong" {
		t.Fatalf("expected error.message=Something went wrong, got %s", got.Error.Message)
	}
	if got.Meta.RequestID != "req-456" {
		t.Fatalf("expected meta.request_id=req-456, got %s", got.Meta.RequestID)
	}
}

func TestRespondList(t *testing.T) {
	w := httptest.NewRecorder()
	ctx := withRequestID(context.Background(), "req-789")
	r := httptest.NewRequest(http.MethodGet, "/", nil).WithContext(ctx)

	items := []string{"a", "b", "c"}
	RespondList(w, r, http.StatusOK, items, 2, 10, 30)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", w.Code)
	}

	var got struct {
		Data []string `json:"data"`
		Meta struct {
			RequestID string `json:"request_id"`
			Page      int    `json:"page"`
			PerPage   int    `json:"per_page"`
			Total     int    `json:"total"`
		} `json:"meta"`
	}
	if err := json.NewDecoder(w.Body).Decode(&got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(got.Data) != 3 {
		t.Fatalf("expected 3 items, got %d", len(got.Data))
	}
	if got.Meta.RequestID != "req-789" {
		t.Fatalf("expected meta.request_id=req-789, got %s", got.Meta.RequestID)
	}
	if got.Meta.Page != 2 {
		t.Fatalf("expected meta.page=2, got %d", got.Meta.Page)
	}
	if got.Meta.PerPage != 10 {
		t.Fatalf("expected meta.per_page=10, got %d", got.Meta.PerPage)
	}
	if got.Meta.Total != 30 {
		t.Fatalf("expected meta.total=30, got %d", got.Meta.Total)
	}
}

func TestParsePagination(t *testing.T) {
	tests := []struct {
		name            string
		query           string
		wantPage        int
		wantPerPage     int
	}{
		{"defaults", "", 1, 25},
		{"custom page", "?page=3", 3, 25},
		{"custom per_page", "?per_page=50", 1, 50},
		{"both", "?page=2&per_page=10", 2, 10},
		{"invalid page", "?page=-1", 1, 25},
		{"invalid per_page zero", "?per_page=0", 1, 25},
		{"per_page exceeds max", "?per_page=200", 1, 25},
		{"non-numeric", "?page=abc&per_page=xyz", 1, 25},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := httptest.NewRequest(http.MethodGet, "/"+tt.query, nil)
			page, perPage := ParsePagination(r)
			if page != tt.wantPage {
				t.Errorf("page: got %d, want %d", page, tt.wantPage)
			}
			if perPage != tt.wantPerPage {
				t.Errorf("perPage: got %d, want %d", perPage, tt.wantPerPage)
			}
		})
	}
}

func TestDecodeJSON(t *testing.T) {
	body := strings.NewReader(`{"name":"test"}`)
	r := httptest.NewRequest(http.MethodPost, "/", body)

	var dst struct {
		Name string `json:"name"`
	}
	if err := DecodeJSON(r, &dst); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if dst.Name != "test" {
		t.Fatalf("expected name=test, got %s", dst.Name)
	}
}

func TestGetRequestIDEmpty(t *testing.T) {
	ctx := withRequestID(context.Background(), "")
	if id := getRequestID(ctx); id != "" {
		t.Fatalf("expected empty request_id, got %s", id)
	}
}

func TestGetRequestIDMissing(t *testing.T) {
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	if id := getRequestID(r.Context()); id != "" {
		t.Fatalf("expected empty request_id from bare context, got %s", id)
	}
}
