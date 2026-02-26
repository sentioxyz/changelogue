package api

import (
	"context"
	"net/http"
	"strings"

	"github.com/sentioxyz/changelogue/internal/models"
)

// AgentStore defines the persistence operations for agent runs.
type AgentStore interface {
	TriggerAgentRun(ctx context.Context, projectID, trigger string) (*models.AgentRun, error)
	ListAgentRuns(ctx context.Context, projectID string, page, perPage int) ([]models.AgentRun, int, error)
	GetAgentRun(ctx context.Context, id string) (*models.AgentRun, error)
}

// AgentHandler implements HTTP handlers for the /agent resource.
type AgentHandler struct {
	store AgentStore
}

// NewAgentHandler returns a new AgentHandler.
func NewAgentHandler(store AgentStore) *AgentHandler {
	return &AgentHandler{store: store}
}

// triggerRequest is the request body for POST /projects/{projectId}/agent/run.
type triggerRequest struct {
	Trigger string `json:"trigger"`
}

// TriggerRun handles POST /projects/{projectId}/agent/run — triggers a new agent run for a project.
func (h *AgentHandler) TriggerRun(w http.ResponseWriter, r *http.Request) {
	projectID := r.PathValue("projectId")
	if projectID == "" {
		RespondError(w, r, http.StatusBadRequest, "bad_request", "Project ID is required")
		return
	}
	var req triggerRequest
	if err := DecodeJSON(r, &req); err != nil {
		RespondError(w, r, http.StatusBadRequest, "bad_request", "Invalid JSON body")
		return
	}
	trigger := strings.TrimSpace(req.Trigger)
	if trigger == "" {
		trigger = "manual"
	}
	run, err := h.store.TriggerAgentRun(r.Context(), projectID, trigger)
	if err != nil {
		RespondError(w, r, http.StatusInternalServerError, "internal_error", "Failed to trigger agent run")
		return
	}
	RespondJSON(w, r, http.StatusCreated, run)
}

// ListRuns handles GET /projects/{projectId}/agent/runs — returns a paginated list of agent runs for a project.
func (h *AgentHandler) ListRuns(w http.ResponseWriter, r *http.Request) {
	projectID := r.PathValue("projectId")
	if projectID == "" {
		RespondError(w, r, http.StatusBadRequest, "bad_request", "Project ID is required")
		return
	}
	page, perPage := ParsePagination(r)
	runs, total, err := h.store.ListAgentRuns(r.Context(), projectID, page, perPage)
	if err != nil {
		RespondError(w, r, http.StatusInternalServerError, "internal_error", "Failed to list agent runs")
		return
	}
	if runs == nil {
		runs = []models.AgentRun{}
	}
	RespondList(w, r, http.StatusOK, runs, page, perPage, total)
}

// GetRun handles GET /agent-runs/{id} — returns a single agent run.
func (h *AgentHandler) GetRun(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		RespondError(w, r, http.StatusBadRequest, "bad_request", "Agent run ID is required")
		return
	}
	run, err := h.store.GetAgentRun(r.Context(), id)
	if err != nil {
		RespondError(w, r, http.StatusNotFound, "not_found", "Agent run not found")
		return
	}
	RespondJSON(w, r, http.StatusOK, run)
}
