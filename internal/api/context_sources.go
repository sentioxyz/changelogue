package api

import (
	"context"
	"net/http"
	"strings"

	"github.com/sentioxyz/releaseguard/internal/models"
)

// ContextSourcesStore defines the persistence operations for context sources.
type ContextSourcesStore interface {
	ListContextSources(ctx context.Context, projectID string, page, perPage int) ([]models.ContextSource, int, error)
	CreateContextSource(ctx context.Context, cs *models.ContextSource) error
	GetContextSource(ctx context.Context, id string) (*models.ContextSource, error)
	UpdateContextSource(ctx context.Context, id string, cs *models.ContextSource) error
	DeleteContextSource(ctx context.Context, id string) error
}

// ContextSourcesHandler implements HTTP handlers for the /context-sources resource.
type ContextSourcesHandler struct {
	store ContextSourcesStore
}

// NewContextSourcesHandler returns a new ContextSourcesHandler.
func NewContextSourcesHandler(store ContextSourcesStore) *ContextSourcesHandler {
	return &ContextSourcesHandler{store: store}
}

// List handles GET /projects/{projectId}/context-sources — returns a paginated list of context sources for a project.
func (h *ContextSourcesHandler) List(w http.ResponseWriter, r *http.Request) {
	projectID := r.PathValue("projectId")
	if projectID == "" {
		RespondError(w, r, http.StatusBadRequest, "bad_request", "Project ID is required")
		return
	}
	page, perPage := ParsePagination(r)
	sources, total, err := h.store.ListContextSources(r.Context(), projectID, page, perPage)
	if err != nil {
		RespondError(w, r, http.StatusInternalServerError, "internal_error", "Failed to list context sources")
		return
	}
	if sources == nil {
		sources = []models.ContextSource{}
	}
	RespondList(w, r, http.StatusOK, sources, page, perPage, total)
}

// Create handles POST /projects/{projectId}/context-sources — creates a new context source under a project.
func (h *ContextSourcesHandler) Create(w http.ResponseWriter, r *http.Request) {
	projectID := r.PathValue("projectId")
	if projectID == "" {
		RespondError(w, r, http.StatusBadRequest, "bad_request", "Project ID is required")
		return
	}
	var cs models.ContextSource
	if err := DecodeJSON(r, &cs); err != nil {
		RespondError(w, r, http.StatusBadRequest, "bad_request", "Invalid JSON body")
		return
	}
	cs.ProjectID = projectID
	cs.Type = strings.TrimSpace(cs.Type)
	cs.Name = strings.TrimSpace(cs.Name)
	if cs.Type == "" {
		RespondError(w, r, http.StatusUnprocessableEntity, "validation_error", "type is required")
		return
	}
	if cs.Name == "" {
		RespondError(w, r, http.StatusUnprocessableEntity, "validation_error", "name is required")
		return
	}
	if cs.Config == nil || string(cs.Config) == "null" {
		RespondError(w, r, http.StatusUnprocessableEntity, "validation_error", "config is required")
		return
	}
	if err := h.store.CreateContextSource(r.Context(), &cs); err != nil {
		RespondError(w, r, http.StatusInternalServerError, "internal_error", "Failed to create context source")
		return
	}
	RespondJSON(w, r, http.StatusCreated, cs)
}

// Get handles GET /context-sources/{id} — returns a single context source.
func (h *ContextSourcesHandler) Get(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		RespondError(w, r, http.StatusBadRequest, "bad_request", "Invalid context source ID")
		return
	}
	cs, err := h.store.GetContextSource(r.Context(), id)
	if err != nil {
		RespondError(w, r, http.StatusNotFound, "not_found", "Context source not found")
		return
	}
	RespondJSON(w, r, http.StatusOK, cs)
}

// Update handles PUT /context-sources/{id} — updates an existing context source.
func (h *ContextSourcesHandler) Update(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		RespondError(w, r, http.StatusBadRequest, "bad_request", "Invalid context source ID")
		return
	}
	var cs models.ContextSource
	if err := DecodeJSON(r, &cs); err != nil {
		RespondError(w, r, http.StatusBadRequest, "bad_request", "Invalid JSON body")
		return
	}
	cs.Type = strings.TrimSpace(cs.Type)
	cs.Name = strings.TrimSpace(cs.Name)
	if cs.Type == "" {
		RespondError(w, r, http.StatusUnprocessableEntity, "validation_error", "type is required")
		return
	}
	if cs.Name == "" {
		RespondError(w, r, http.StatusUnprocessableEntity, "validation_error", "name is required")
		return
	}
	if cs.Config == nil || string(cs.Config) == "null" {
		RespondError(w, r, http.StatusUnprocessableEntity, "validation_error", "config is required")
		return
	}
	if err := h.store.UpdateContextSource(r.Context(), id, &cs); err != nil {
		RespondError(w, r, http.StatusNotFound, "not_found", "Context source not found")
		return
	}
	cs.ID = id
	RespondJSON(w, r, http.StatusOK, cs)
}

// Delete handles DELETE /context-sources/{id} — deletes a context source.
func (h *ContextSourcesHandler) Delete(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		RespondError(w, r, http.StatusBadRequest, "bad_request", "Invalid context source ID")
		return
	}
	if err := h.store.DeleteContextSource(r.Context(), id); err != nil {
		RespondError(w, r, http.StatusNotFound, "not_found", "Context source not found")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
