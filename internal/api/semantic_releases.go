package api

import (
	"context"
	"net/http"

	"github.com/sentioxyz/releaseguard/internal/models"
)

// SemanticReleasesStore defines the persistence operations for semantic releases (read-only).
type SemanticReleasesStore interface {
	ListSemanticReleases(ctx context.Context, projectID string, page, perPage int) ([]models.SemanticRelease, int, error)
	GetSemanticRelease(ctx context.Context, id string) (*models.SemanticRelease, error)
	GetSemanticReleaseSources(ctx context.Context, id string) ([]models.Release, error)
}

// SemanticReleasesHandler implements HTTP handlers for the /semantic-releases resource.
type SemanticReleasesHandler struct {
	store SemanticReleasesStore
}

// NewSemanticReleasesHandler returns a new SemanticReleasesHandler.
func NewSemanticReleasesHandler(store SemanticReleasesStore) *SemanticReleasesHandler {
	return &SemanticReleasesHandler{store: store}
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
