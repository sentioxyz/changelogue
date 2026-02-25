package api

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"

	"github.com/sentioxyz/releaseguard/internal/models"
)

// ProjectsStore defines the persistence operations for projects.
type ProjectsStore interface {
	ListProjects(ctx context.Context, page, perPage int) ([]models.Project, int, error)
	CreateProject(ctx context.Context, p *models.Project) error
	GetProject(ctx context.Context, id string) (*models.Project, error)
	UpdateProject(ctx context.Context, id string, p *models.Project) error
	DeleteProject(ctx context.Context, id string) error
}

// ProjectsHandler implements HTTP handlers for the /projects resource.
type ProjectsHandler struct {
	store ProjectsStore
}

// NewProjectsHandler returns a new ProjectsHandler.
func NewProjectsHandler(store ProjectsStore) *ProjectsHandler {
	return &ProjectsHandler{store: store}
}

// List handles GET /projects — returns a paginated list of projects.
func (h *ProjectsHandler) List(w http.ResponseWriter, r *http.Request) {
	page, perPage := ParsePagination(r)
	projects, total, err := h.store.ListProjects(r.Context(), page, perPage)
	if err != nil {
		RespondError(w, r, http.StatusInternalServerError, "internal_error", "Failed to list projects")
		return
	}
	// Return empty array, not null, when no results.
	if projects == nil {
		projects = []models.Project{}
	}
	RespondList(w, r, http.StatusOK, projects, page, perPage, total)
}

// Create handles POST /projects — creates a new project.
func (h *ProjectsHandler) Create(w http.ResponseWriter, r *http.Request) {
	var p models.Project
	if err := DecodeJSON(r, &p); err != nil {
		RespondError(w, r, http.StatusBadRequest, "bad_request", "Invalid JSON body")
		return
	}
	p.Name = strings.TrimSpace(p.Name)
	if p.Name == "" {
		RespondError(w, r, http.StatusUnprocessableEntity, "validation_error", "Name is required")
		return
	}
	// Default agent_rules to empty object if not provided.
	if p.AgentRules == nil {
		p.AgentRules = json.RawMessage(`{}`)
	}
	if err := h.store.CreateProject(r.Context(), &p); err != nil {
		RespondError(w, r, http.StatusInternalServerError, "internal_error", "Failed to create project")
		return
	}
	RespondJSON(w, r, http.StatusCreated, p)
}

// Get handles GET /projects/{id} — returns a single project.
func (h *ProjectsHandler) Get(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		RespondError(w, r, http.StatusBadRequest, "bad_request", "Invalid project ID")
		return
	}
	p, err := h.store.GetProject(r.Context(), id)
	if err != nil {
		RespondError(w, r, http.StatusNotFound, "not_found", "Project not found")
		return
	}
	RespondJSON(w, r, http.StatusOK, p)
}

// Update handles PUT /projects/{id} — updates an existing project.
func (h *ProjectsHandler) Update(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		RespondError(w, r, http.StatusBadRequest, "bad_request", "Invalid project ID")
		return
	}
	var p models.Project
	if err := DecodeJSON(r, &p); err != nil {
		RespondError(w, r, http.StatusBadRequest, "bad_request", "Invalid JSON body")
		return
	}
	p.Name = strings.TrimSpace(p.Name)
	if p.Name == "" {
		RespondError(w, r, http.StatusUnprocessableEntity, "validation_error", "Name is required")
		return
	}
	if p.AgentRules == nil {
		p.AgentRules = json.RawMessage(`{}`)
	}
	if err := h.store.UpdateProject(r.Context(), id, &p); err != nil {
		RespondError(w, r, http.StatusNotFound, "not_found", "Project not found")
		return
	}
	p.ID = id
	RespondJSON(w, r, http.StatusOK, p)
}

// Delete handles DELETE /projects/{id} — deletes a project.
func (h *ProjectsHandler) Delete(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		RespondError(w, r, http.StatusBadRequest, "bad_request", "Invalid project ID")
		return
	}
	if err := h.store.DeleteProject(r.Context(), id); err != nil {
		RespondError(w, r, http.StatusNotFound, "not_found", "Project not found")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
