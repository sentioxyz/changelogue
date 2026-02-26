package api

import (
	"context"
	"net/http"

	"github.com/sentioxyz/changelogue/internal/models"
)

// SemanticReleasesStore defines the persistence operations for semantic releases.
type SemanticReleasesStore interface {
	ListAllSemanticReleases(ctx context.Context, page, perPage int) ([]models.SemanticRelease, int, error)
	ListSemanticReleases(ctx context.Context, projectID string, page, perPage int) ([]models.SemanticRelease, int, error)
	GetSemanticRelease(ctx context.Context, id string) (*models.SemanticRelease, error)
	GetSemanticReleaseSources(ctx context.Context, id string) ([]models.Release, error)
	DeleteSemanticRelease(ctx context.Context, id string) error
}

// SemanticReleasesHandler implements HTTP handlers for the /semantic-releases resource.
type SemanticReleasesHandler struct {
	store SemanticReleasesStore
}

// NewSemanticReleasesHandler returns a new SemanticReleasesHandler.
func NewSemanticReleasesHandler(store SemanticReleasesStore) *SemanticReleasesHandler {
	return &SemanticReleasesHandler{store: store}
}

// ListAll handles GET /semantic-releases — returns all semantic releases across all projects.
func (h *SemanticReleasesHandler) ListAll(w http.ResponseWriter, r *http.Request) {
	page, perPage := ParsePagination(r)
	releases, total, err := h.store.ListAllSemanticReleases(r.Context(), page, perPage)
	if err != nil {
		RespondError(w, r, http.StatusInternalServerError, "internal_error", "Failed to list semantic releases")
		return
	}
	if releases == nil {
		releases = []models.SemanticRelease{}
	}
	RespondList(w, r, http.StatusOK, releases, page, perPage, total)
}

// List handles GET /projects/{projectId}/semantic-releases — returns a paginated list of semantic releases for a project.
func (h *SemanticReleasesHandler) List(w http.ResponseWriter, r *http.Request) {
	projectID := r.PathValue("projectId")
	if projectID == "" {
		RespondError(w, r, http.StatusBadRequest, "bad_request", "Project ID is required")
		return
	}
	page, perPage := ParsePagination(r)
	releases, total, err := h.store.ListSemanticReleases(r.Context(), projectID, page, perPage)
	if err != nil {
		RespondError(w, r, http.StatusInternalServerError, "internal_error", "Failed to list semantic releases")
		return
	}
	if releases == nil {
		releases = []models.SemanticRelease{}
	}
	RespondList(w, r, http.StatusOK, releases, page, perPage, total)
}

// Get handles GET /semantic-releases/{id} — returns a single semantic release.
func (h *SemanticReleasesHandler) Get(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		RespondError(w, r, http.StatusBadRequest, "bad_request", "Semantic release ID is required")
		return
	}
	sr, err := h.store.GetSemanticRelease(r.Context(), id)
	if err != nil {
		RespondError(w, r, http.StatusNotFound, "not_found", "Semantic release not found")
		return
	}
	RespondJSON(w, r, http.StatusOK, sr)
}

// Delete handles DELETE /semantic-releases/{id} — deletes a single semantic release.
func (h *SemanticReleasesHandler) Delete(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		RespondError(w, r, http.StatusBadRequest, "bad_request", "Semantic release ID is required")
		return
	}
	if err := h.store.DeleteSemanticRelease(r.Context(), id); err != nil {
		RespondError(w, r, http.StatusNotFound, "not_found", "Semantic release not found")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
