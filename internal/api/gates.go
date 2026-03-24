package api

import (
	"context"
	"net/http"

	"github.com/sentioxyz/changelogue/internal/models"
)

// GatesStore defines data access for gate API handlers.
type GatesStore interface {
	GetReleaseGate(ctx context.Context, projectID string) (*models.ReleaseGate, error)
	CreateReleaseGate(ctx context.Context, g *models.ReleaseGate) error
	UpdateReleaseGate(ctx context.Context, g *models.ReleaseGate) error
	DeleteReleaseGate(ctx context.Context, projectID string) error
	ListVersionReadiness(ctx context.Context, projectID string, page, perPage int) ([]models.VersionReadiness, int, error)
	GetVersionReadinessByVersion(ctx context.Context, projectID, version string) (*models.VersionReadiness, error)
	ListGateEvents(ctx context.Context, projectID string, page, perPage int) ([]models.GateEvent, int, error)
	ListGateEventsByVersion(ctx context.Context, projectID, version string, page, perPage int) ([]models.GateEvent, int, error)
}

// GatesHandler implements HTTP handlers for the release gate resources.
type GatesHandler struct {
	store GatesStore
}

// NewGatesHandler returns a new GatesHandler.
func NewGatesHandler(store GatesStore) *GatesHandler {
	return &GatesHandler{store: store}
}

// GetGate handles GET /api/v1/projects/{id}/release-gate.
func (h *GatesHandler) GetGate(w http.ResponseWriter, r *http.Request) {
	projectID := r.PathValue("id")
	if projectID == "" {
		RespondError(w, r, http.StatusBadRequest, "bad_request", "Invalid project ID")
		return
	}
	g, err := h.store.GetReleaseGate(r.Context(), projectID)
	if err != nil || g == nil {
		RespondError(w, r, http.StatusNotFound, "not_found", "Release gate not found")
		return
	}
	RespondJSON(w, r, http.StatusOK, g)
}

// UpsertGate handles PUT /api/v1/projects/{id}/release-gate.
func (h *GatesHandler) UpsertGate(w http.ResponseWriter, r *http.Request) {
	projectID := r.PathValue("id")
	if projectID == "" {
		RespondError(w, r, http.StatusBadRequest, "bad_request", "Invalid project ID")
		return
	}
	var g models.ReleaseGate
	if err := DecodeJSON(r, &g); err != nil {
		RespondError(w, r, http.StatusBadRequest, "bad_request", "Invalid JSON body")
		return
	}
	g.ProjectID = projectID

	existing, err := h.store.GetReleaseGate(r.Context(), projectID)
	if err != nil {
		RespondError(w, r, http.StatusInternalServerError, "internal_error", "Failed to check release gate")
		return
	}
	if existing != nil {
		if err := h.store.UpdateReleaseGate(r.Context(), &g); err != nil {
			RespondError(w, r, http.StatusInternalServerError, "internal_error", "Failed to update release gate")
			return
		}
	} else {
		if err := h.store.CreateReleaseGate(r.Context(), &g); err != nil {
			RespondError(w, r, http.StatusInternalServerError, "internal_error", "Failed to create release gate")
			return
		}
	}

	updated, err := h.store.GetReleaseGate(r.Context(), projectID)
	if err != nil || updated == nil {
		RespondError(w, r, http.StatusInternalServerError, "internal_error", "Failed to fetch release gate after upsert")
		return
	}
	RespondJSON(w, r, http.StatusOK, updated)
}

// DeleteGate handles DELETE /api/v1/projects/{id}/release-gate.
func (h *GatesHandler) DeleteGate(w http.ResponseWriter, r *http.Request) {
	projectID := r.PathValue("id")
	if projectID == "" {
		RespondError(w, r, http.StatusBadRequest, "bad_request", "Invalid project ID")
		return
	}
	if err := h.store.DeleteReleaseGate(r.Context(), projectID); err != nil {
		RespondError(w, r, http.StatusNotFound, "not_found", "Release gate not found")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// ListReadiness handles GET /api/v1/projects/{id}/version-readiness.
func (h *GatesHandler) ListReadiness(w http.ResponseWriter, r *http.Request) {
	projectID := r.PathValue("id")
	if projectID == "" {
		RespondError(w, r, http.StatusBadRequest, "bad_request", "Invalid project ID")
		return
	}
	page, perPage := ParsePagination(r)
	items, total, err := h.store.ListVersionReadiness(r.Context(), projectID, page, perPage)
	if err != nil {
		RespondError(w, r, http.StatusInternalServerError, "internal_error", "Failed to list version readiness")
		return
	}
	if items == nil {
		items = []models.VersionReadiness{}
	}
	RespondList(w, r, http.StatusOK, items, page, perPage, total)
}

// GetReadiness handles GET /api/v1/projects/{id}/version-readiness/{version}.
func (h *GatesHandler) GetReadiness(w http.ResponseWriter, r *http.Request) {
	projectID := r.PathValue("id")
	version := r.PathValue("version")
	if projectID == "" || version == "" {
		RespondError(w, r, http.StatusBadRequest, "bad_request", "Invalid project ID or version")
		return
	}
	vr, err := h.store.GetVersionReadinessByVersion(r.Context(), projectID, version)
	if err != nil || vr == nil {
		RespondError(w, r, http.StatusNotFound, "not_found", "Version readiness not found")
		return
	}
	RespondJSON(w, r, http.StatusOK, vr)
}

// ListEvents handles GET /api/v1/projects/{id}/gate-events.
func (h *GatesHandler) ListEvents(w http.ResponseWriter, r *http.Request) {
	projectID := r.PathValue("id")
	if projectID == "" {
		RespondError(w, r, http.StatusBadRequest, "bad_request", "Invalid project ID")
		return
	}
	page, perPage := ParsePagination(r)
	items, total, err := h.store.ListGateEvents(r.Context(), projectID, page, perPage)
	if err != nil {
		RespondError(w, r, http.StatusInternalServerError, "internal_error", "Failed to list gate events")
		return
	}
	if items == nil {
		items = []models.GateEvent{}
	}
	RespondList(w, r, http.StatusOK, items, page, perPage, total)
}

// ListEventsByVersion handles GET /api/v1/projects/{id}/version-readiness/{version}/events.
func (h *GatesHandler) ListEventsByVersion(w http.ResponseWriter, r *http.Request) {
	projectID := r.PathValue("id")
	version := r.PathValue("version")
	if projectID == "" || version == "" {
		RespondError(w, r, http.StatusBadRequest, "bad_request", "Invalid project ID or version")
		return
	}
	page, perPage := ParsePagination(r)
	items, total, err := h.store.ListGateEventsByVersion(r.Context(), projectID, version, page, perPage)
	if err != nil {
		RespondError(w, r, http.StatusInternalServerError, "internal_error", "Failed to list gate events by version")
		return
	}
	if items == nil {
		items = []models.GateEvent{}
	}
	RespondList(w, r, http.StatusOK, items, page, perPage, total)
}
