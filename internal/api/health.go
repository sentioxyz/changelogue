package api

import (
	"context"
	"net/http"
)

// DashboardStats holds aggregate statistics for the dashboard.
type DashboardStats struct {
	TotalReleases    int `json:"total_releases"`
	ActiveSources    int `json:"active_sources"`
	TotalProjects    int `json:"total_projects"`
	PendingAgentRuns int `json:"pending_agent_runs"`
}

// HealthChecker defines operations for health checks and dashboard statistics.
type HealthChecker interface {
	PingDB(ctx context.Context) error
	GetStats(ctx context.Context) (*DashboardStats, error)
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
