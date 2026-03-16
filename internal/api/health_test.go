package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// mockHealthChecker implements HealthChecker for testing.
type mockHealthChecker struct {
	pingErr  error
	stats    *DashboardStats
	statsErr error
	trend    []TrendBucket
	trendErr error
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

func (m *mockHealthChecker) GetTrend(_ context.Context, _ string, _, _ time.Time) ([]TrendBucket, error) {
	if m.trendErr != nil {
		return nil, m.trendErr
	}
	return m.trend, nil
}

func setupHealthMux(checker HealthChecker) *http.ServeMux {
	h := NewHealthHandler(checker)
	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", h.Check)
	mux.HandleFunc("GET /stats", h.Stats)
	mux.HandleFunc("GET /stats/trend", h.Trend)
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
			TotalReleases:    42,
			ActiveSources:    5,
			TotalProjects:    3,
			PendingAgentRuns: 1,
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
	if got.Data.TotalProjects != 3 {
		t.Fatalf("expected total_projects=3, got %d", got.Data.TotalProjects)
	}
	if got.Data.PendingAgentRuns != 1 {
		t.Fatalf("expected pending_agent_runs=1, got %d", got.Data.PendingAgentRuns)
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
	if len(got.Data) != 6 {
		t.Fatalf("expected 6 providers, got %d", len(got.Data))
	}
	if got.Data[0]["id"] != "dockerhub" {
		t.Fatalf("expected first provider id=dockerhub, got %s", got.Data[0]["id"])
	}
	if got.Data[1]["id"] != "github" {
		t.Fatalf("expected second provider id=github, got %s", got.Data[1]["id"])
	}
	if got.Data[2]["id"] != "ecr-public" {
		t.Fatalf("expected third provider id=ecr-public, got %s", got.Data[2]["id"])
	}
	if got.Data[3]["id"] != "gitlab" {
		t.Fatalf("expected fourth provider id=gitlab, got %s", got.Data[3]["id"])
	}
	if got.Data[4]["id"] != "pypi" {
		t.Fatalf("expected fifth provider id=pypi, got %s", got.Data[4]["id"])
	}
	if got.Data[0]["type"] != "polling" {
		t.Fatalf("expected dockerhub type=polling, got %s", got.Data[0]["type"])
	}
	if got.Data[1]["type"] != "webhook" {
		t.Fatalf("expected github type=webhook, got %s", got.Data[1]["type"])
	}
	if got.Data[2]["type"] != "polling" {
		t.Fatalf("expected ecr-public type=polling, got %s", got.Data[2]["type"])
	}
	if got.Data[3]["type"] != "polling" {
		t.Fatalf("expected gitlab type=polling, got %s", got.Data[3]["type"])
	}
	if got.Data[4]["type"] != "polling" {
		t.Fatalf("expected pypi type=polling, got %s", got.Data[4]["type"])
	}
	if got.Data[5]["id"] != "npm" {
		t.Fatalf("expected sixth provider id=npm, got %s", got.Data[5]["id"])
	}
	if got.Data[5]["type"] != "polling" {
		t.Fatalf("expected npm type=polling, got %s", got.Data[5]["type"])
	}
}

func TestTrendHandlerDaily(t *testing.T) {
	checker := &mockHealthChecker{
		trend: []TrendBucket{
			{Period: "2026-01-29", Releases: 3, SemanticReleases: 1},
			{Period: "2026-01-30", Releases: 0, SemanticReleases: 0},
		},
	}
	mux := setupHealthMux(checker)

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/stats/trend?granularity=daily", nil)
	mux.ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", w.Code)
	}

	var got struct {
		Data TrendResponse `json:"data"`
	}
	if err := json.NewDecoder(w.Body).Decode(&got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if got.Data.Granularity != "daily" {
		t.Fatalf("expected granularity=daily, got %s", got.Data.Granularity)
	}
	if len(got.Data.Buckets) != 2 {
		t.Fatalf("expected 2 buckets, got %d", len(got.Data.Buckets))
	}
	if got.Data.Buckets[0].Releases != 3 {
		t.Fatalf("expected first bucket releases=3, got %d", got.Data.Buckets[0].Releases)
	}
}

func TestTrendHandlerDefaultGranularity(t *testing.T) {
	checker := &mockHealthChecker{
		trend: []TrendBucket{},
	}
	mux := setupHealthMux(checker)

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/stats/trend", nil)
	mux.ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", w.Code)
	}

	var got struct {
		Data TrendResponse `json:"data"`
	}
	if err := json.NewDecoder(w.Body).Decode(&got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if got.Data.Granularity != "daily" {
		t.Fatalf("expected default granularity=daily, got %s", got.Data.Granularity)
	}
}

func TestTrendHandlerInvalidGranularity(t *testing.T) {
	checker := &mockHealthChecker{}
	mux := setupHealthMux(checker)

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/stats/trend?granularity=hourly", nil)
	mux.ServeHTTP(w, r)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d", w.Code)
	}
}

func TestTrendHandlerError(t *testing.T) {
	checker := &mockHealthChecker{
		trendErr: fmt.Errorf("database error"),
	}
	mux := setupHealthMux(checker)

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/stats/trend?granularity=weekly", nil)
	mux.ServeHTTP(w, r)

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected status 500, got %d", w.Code)
	}
}

func TestTrendHandlerCustomDays(t *testing.T) {
	checker := &mockHealthChecker{
		trend: []TrendBucket{
			{Period: "2026-02-01", Releases: 1, SemanticReleases: 0},
		},
	}
	mux := setupHealthMux(checker)

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/stats/trend?granularity=daily&days=30", nil)
	mux.ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", w.Code)
	}
}

func TestTrendHandlerInvalidDays(t *testing.T) {
	checker := &mockHealthChecker{}
	mux := setupHealthMux(checker)

	tests := []struct {
		name string
		url  string
	}{
		{"zero", "/stats/trend?days=0"},
		{"negative", "/stats/trend?days=-5"},
		{"too_large", "/stats/trend?days=999"},
		{"non_numeric", "/stats/trend?days=abc"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			r := httptest.NewRequest(http.MethodGet, tt.url, nil)
			mux.ServeHTTP(w, r)
			if w.Code != http.StatusBadRequest {
				t.Fatalf("expected status 400, got %d", w.Code)
			}
		})
	}
}
