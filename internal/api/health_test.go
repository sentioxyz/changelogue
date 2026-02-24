package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
)

// mockHealthChecker implements HealthChecker for testing.
type mockHealthChecker struct {
	pingErr  error
	stats    *DashboardStats
	statsErr error
}

func (m *mockHealthChecker) PingDB(_ context.Context) error {
	return m.pingErr
}

func (m *mockHealthChecker) GetStats(_ context.Context) (*DashboardStats, error) {
	if m.statsErr != nil {
		return nil, m.statsErr
	}
	return m.stats, nil
}

func setupHealthMux(checker HealthChecker) *http.ServeMux {
	h := NewHealthHandler(checker)
	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", h.Check)
	mux.HandleFunc("GET /stats", h.Stats)
	return mux
}

func TestHealthHandlerCheckHealthy(t *testing.T) {
	checker := &mockHealthChecker{}
	mux := setupHealthMux(checker)

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/health", nil)
	mux.ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", w.Code)
	}

	var got struct {
		Data struct {
			Status string            `json:"status"`
			Checks map[string]string `json:"checks"`
		} `json:"data"`
	}
	if err := json.NewDecoder(w.Body).Decode(&got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if got.Data.Status != "healthy" {
		t.Fatalf("expected status=healthy, got %s", got.Data.Status)
	}
	if got.Data.Checks["database"] != "ok" {
		t.Fatalf("expected database=ok, got %s", got.Data.Checks["database"])
	}
}

func TestHealthHandlerCheckUnhealthy(t *testing.T) {
	checker := &mockHealthChecker{
		pingErr: fmt.Errorf("connection refused"),
	}
	mux := setupHealthMux(checker)

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/health", nil)
	mux.ServeHTTP(w, r)

	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected status 503, got %d", w.Code)
	}

	var got struct {
		Data struct {
			Status string            `json:"status"`
			Checks map[string]string `json:"checks"`
		} `json:"data"`
	}
	if err := json.NewDecoder(w.Body).Decode(&got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if got.Data.Status != "unhealthy" {
		t.Fatalf("expected status=unhealthy, got %s", got.Data.Status)
	}
	if got.Data.Checks["database"] != "error" {
		t.Fatalf("expected database=error, got %s", got.Data.Checks["database"])
	}
}

func TestHealthHandlerStats(t *testing.T) {
	checker := &mockHealthChecker{
		stats: &DashboardStats{
			TotalReleases: 42,
			ActiveSources: 5,
			PendingJobs:   3,
			FailedJobs:    1,
		},
	}
	mux := setupHealthMux(checker)

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/stats", nil)
	mux.ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", w.Code)
	}

	var got struct {
		Data DashboardStats `json:"data"`
	}
	if err := json.NewDecoder(w.Body).Decode(&got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if got.Data.TotalReleases != 42 {
		t.Fatalf("expected total_releases=42, got %d", got.Data.TotalReleases)
	}
	if got.Data.ActiveSources != 5 {
		t.Fatalf("expected active_sources=5, got %d", got.Data.ActiveSources)
	}
	if got.Data.PendingJobs != 3 {
		t.Fatalf("expected pending_jobs=3, got %d", got.Data.PendingJobs)
	}
	if got.Data.FailedJobs != 1 {
		t.Fatalf("expected failed_jobs=1, got %d", got.Data.FailedJobs)
	}
}

func TestHealthHandlerStatsError(t *testing.T) {
	checker := &mockHealthChecker{
		statsErr: fmt.Errorf("database error"),
	}
	mux := setupHealthMux(checker)

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/stats", nil)
	mux.ServeHTTP(w, r)

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected status 500, got %d", w.Code)
	}
}

func TestProvidersHandlerList(t *testing.T) {
	h := NewProvidersHandler()
	mux := http.NewServeMux()
	mux.HandleFunc("GET /providers", h.List)

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/providers", nil)
	mux.ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", w.Code)
	}

	var got struct {
		Data []map[string]string `json:"data"`
	}
	if err := json.NewDecoder(w.Body).Decode(&got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(got.Data) != 2 {
		t.Fatalf("expected 2 providers, got %d", len(got.Data))
	}
	if got.Data[0]["id"] != "dockerhub" {
		t.Fatalf("expected first provider id=dockerhub, got %s", got.Data[0]["id"])
	}
	if got.Data[1]["id"] != "github" {
		t.Fatalf("expected second provider id=github, got %s", got.Data[1]["id"])
	}
	if got.Data[0]["type"] != "polling" {
		t.Fatalf("expected dockerhub type=polling, got %s", got.Data[0]["type"])
	}
	if got.Data[1]["type"] != "webhook" {
		t.Fatalf("expected github type=webhook, got %s", got.Data[1]["type"])
	}
}
