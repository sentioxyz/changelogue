package api

import (
	"context"
	"net/http"

	"github.com/sentioxyz/changelogue/internal/models"
)

// ReleasesStore defines the persistence operations for releases (read-only).
type ReleasesStore interface {
	ListAllReleases(ctx context.Context, page, perPage int) ([]models.Release, int, error)
	ListReleasesBySource(ctx context.Context, sourceID string, page, perPage int) ([]models.Release, int, error)
	ListReleasesByProject(ctx context.Context, projectID string, page, perPage int) ([]models.Release, int, error)
	GetRelease(ctx context.Context, id string) (*models.Release, error)
}

// ReleasesHandler implements HTTP handlers for the /releases resource.
type ReleasesHandler struct {
	store ReleasesStore
}

// NewReleasesHandler returns a new ReleasesHandler.
func NewReleasesHandler(store ReleasesStore) *ReleasesHandler {
	return &ReleasesHandler{store: store}
}

// List handles GET /releases — returns all releases across all projects.
func (h *ReleasesHandler) List(w http.ResponseWriter, r *http.Request) {
	page, perPage := ParsePagination(r)
	releases, total, err := h.store.ListAllReleases(r.Context(), page, perPage)
	if err != nil {
		RespondError(w, r, http.StatusInternalServerError, "internal_error", "Failed to list releases")
		return
	}
	if releases == nil {
		releases = []models.Release{}
	}
	RespondList(w, r, http.StatusOK, releases, page, perPage, total)
}

// ListBySource handles GET /sources/{id}/releases — returns releases for a specific source.
func (h *ReleasesHandler) ListBySource(w http.ResponseWriter, r *http.Request) {
	sourceID := r.PathValue("id")
	if sourceID == "" {
		RespondError(w, r, http.StatusBadRequest, "bad_request", "Source ID is required")
		return
	}
	page, perPage := ParsePagination(r)
	releases, total, err := h.store.ListReleasesBySource(r.Context(), sourceID, page, perPage)
	if err != nil {
		RespondError(w, r, http.StatusInternalServerError, "internal_error", "Failed to list releases")
		return
	}
	if releases == nil {
		releases = []models.Release{}
	}
	RespondList(w, r, http.StatusOK, releases, page, perPage, total)
}

// ListByProject handles GET /projects/{projectId}/releases — returns releases for all sources in a project.
func (h *ReleasesHandler) ListByProject(w http.ResponseWriter, r *http.Request) {
	projectID := r.PathValue("projectId")
	if projectID == "" {
		RespondError(w, r, http.StatusBadRequest, "bad_request", "Project ID is required")
		return
	}
	page, perPage := ParsePagination(r)
	releases, total, err := h.store.ListReleasesByProject(r.Context(), projectID, page, perPage)
	if err != nil {
		RespondError(w, r, http.StatusInternalServerError, "internal_error", "Failed to list releases")
		return
	}
	if releases == nil {
		releases = []models.Release{}
	}
	RespondList(w, r, http.StatusOK, releases, page, perPage, total)
}

// Get handles GET /releases/{id} — returns a single release.
func (h *ReleasesHandler) Get(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		RespondError(w, r, http.StatusBadRequest, "bad_request", "Release ID is required")
		return
	}
	rel, err := h.store.GetRelease(r.Context(), id)
	if err != nil {
		RespondError(w, r, http.StatusNotFound, "not_found", "Release not found")
		return
	}
	RespondJSON(w, r, http.StatusOK, rel)
}
