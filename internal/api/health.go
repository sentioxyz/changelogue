package api

import (
	"context"
	"net/http"
	"strconv"
	"time"
)

// DashboardStats holds aggregate statistics for the dashboard.
type DashboardStats struct {
	TotalReleases    int `json:"total_releases"`
	ActiveSources    int `json:"active_sources"`
	TotalProjects    int `json:"total_projects"`
	PendingAgentRuns int `json:"pending_agent_runs"`
	ReleasesThisWeek int `json:"releases_this_week"`
	AttentionNeeded  int `json:"attention_needed"`
}

// TrendBucket represents a single time bucket in the release trend.
type TrendBucket struct {
	Period           string `json:"period"`
	Releases         int    `json:"releases"`
	SemanticReleases int    `json:"semantic_releases"`
}

// TrendResponse is the response payload for the trend endpoint.
type TrendResponse struct {
	Granularity string        `json:"granularity"`
	Buckets     []TrendBucket `json:"buckets"`
}

// HealthChecker defines operations for health checks and dashboard statistics.
type HealthChecker interface {
	PingDB(ctx context.Context) error
	GetStats(ctx context.Context) (*DashboardStats, error)
	GetTrend(ctx context.Context, granularity string, start, end time.Time) ([]TrendBucket, error)
}

// HealthHandler implements HTTP handlers for health and stats endpoints.
type HealthHandler struct {
	checker HealthChecker
}

// NewHealthHandler returns a new HealthHandler.
func NewHealthHandler(checker HealthChecker) *HealthHandler {
	return &HealthHandler{checker: checker}
}

// Check handles GET /health — pings the database and returns health status.
func (h *HealthHandler) Check(w http.ResponseWriter, r *http.Request) {
	err := h.checker.PingDB(r.Context())
	if err != nil {
		RespondJSON(w, r, http.StatusServiceUnavailable, map[string]any{
			"status": "unhealthy",
			"checks": map[string]string{
				"database": "error",
			},
		})
		return
	}
	RespondJSON(w, r, http.StatusOK, map[string]any{
		"status": "healthy",
		"checks": map[string]string{
			"database": "ok",
		},
	})
}

// Stats handles GET /stats — returns aggregate dashboard statistics.
func (h *HealthHandler) Stats(w http.ResponseWriter, r *http.Request) {
	stats, err := h.checker.GetStats(r.Context())
	if err != nil {
		RespondError(w, r, http.StatusInternalServerError, "internal_error", "Failed to retrieve stats")
		return
	}
	RespondJSON(w, r, http.StatusOK, stats)
}

// Trend handles GET /api/v1/stats/trend — returns time-bucketed release counts.
// Query params:
//   - granularity: "daily" (default), "weekly", or "monthly"
//   - days: number of days to look back (default 7, max 365)
func (h *HealthHandler) Trend(w http.ResponseWriter, r *http.Request) {
	granularity := r.URL.Query().Get("granularity")
	if granularity == "" {
		granularity = "daily"
	}

	switch granularity {
	case "daily", "weekly", "monthly":
	default:
		RespondError(w, r, http.StatusBadRequest, "invalid_granularity", "granularity must be daily, weekly, or monthly")
		return
	}

	days := 7
	if d := r.URL.Query().Get("days"); d != "" {
		parsed, err := strconv.Atoi(d)
		if err != nil || parsed < 1 || parsed > 365 {
			RespondError(w, r, http.StatusBadRequest, "invalid_days", "days must be an integer between 1 and 365")
			return
		}
		days = parsed
	}

	now := time.Now().UTC()
	start := now.AddDate(0, 0, -days)

	buckets, err := h.checker.GetTrend(r.Context(), granularity, start, now)
	if err != nil {
		RespondError(w, r, http.StatusInternalServerError, "internal_error", "Failed to retrieve trend data")
		return
	}

	RespondJSON(w, r, http.StatusOK, TrendResponse{
		Granularity: granularity,
		Buckets:     buckets,
	})
}
