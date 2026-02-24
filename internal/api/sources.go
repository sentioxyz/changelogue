package api

import (
	"context"
	"net/http"
	"strconv"
	"strings"

	"github.com/sentioxyz/releaseguard/internal/models"
)

// ReleaseView is a denormalized read model joining releases, sources, projects, and pipeline_jobs.
type ReleaseView struct {
	ID             string `json:"id"`
	SourceID       int    `json:"source_id"`
	SourceType     string `json:"source_type"`
	Repository     string `json:"repository"`
	ProjectID      int    `json:"project_id"`
	ProjectName    string `json:"project_name"`
	RawVersion     string `json:"raw_version"`
	IsPreRelease   bool   `json:"is_pre_release"`
	PipelineStatus string `json:"pipeline_status"`
	CreatedAt      string `json:"created_at"`
}

// releaseViewQuery is the shared base query for constructing ReleaseView records.
const releaseViewQuery = `SELECT r.id, s.id, s.source_type, s.repository, p.id, p.name, r.version,
       COALESCE(r.payload->>'is_pre_release', 'false'),
       COALESCE(pj.state, 'pending'), r.created_at
FROM releases r
JOIN sources s ON r.source_id = s.id
JOIN projects p ON s.project_id = p.id
LEFT JOIN pipeline_jobs pj ON pj.release_id = r.id`

// SourcesStore defines the persistence operations for sources.
type SourcesStore interface {
	ListSources(ctx context.Context, page, perPage int) ([]models.Source, int, error)
	CreateSource(ctx context.Context, src *models.Source) error
	GetSource(ctx context.Context, id int) (*models.Source, error)
	UpdateSource(ctx context.Context, id int, src *models.Source) error
	DeleteSource(ctx context.Context, id int) error
	GetLatestRelease(ctx context.Context, sourceID int) (*ReleaseView, error)
	GetReleaseByVersion(ctx context.Context, sourceID int, version string) (*ReleaseView, error)
}

// SourcesHandler implements HTTP handlers for the /sources resource.
type SourcesHandler struct {
	store SourcesStore
}

// NewSourcesHandler returns a new SourcesHandler.
func NewSourcesHandler(store SourcesStore) *SourcesHandler {
	return &SourcesHandler{store: store}
}

// List handles GET /sources — returns a paginated list of sources.
func (h *SourcesHandler) List(w http.ResponseWriter, r *http.Request) {
	page, perPage := ParsePagination(r)
	sources, total, err := h.store.ListSources(r.Context(), page, perPage)
	if err != nil {
		RespondError(w, r, http.StatusInternalServerError, "internal_error", "Failed to list sources")
		return
	}
	if sources == nil {
		sources = []models.Source{}
	}
	RespondList(w, r, http.StatusOK, sources, page, perPage, total)
}

// Create handles POST /sources — creates a new source.
func (h *SourcesHandler) Create(w http.ResponseWriter, r *http.Request) {
	var src models.Source
	if err := DecodeJSON(r, &src); err != nil {
		RespondError(w, r, http.StatusBadRequest, "bad_request", "Invalid JSON body")
		return
	}
	src.SourceType = strings.TrimSpace(src.SourceType)
	src.Repository = strings.TrimSpace(src.Repository)
	if src.ProjectID == 0 {
		RespondError(w, r, http.StatusUnprocessableEntity, "validation_error", "project_id is required")
		return
	}
	if src.SourceType == "" {
		RespondError(w, r, http.StatusUnprocessableEntity, "validation_error", "type is required")
		return
	}
	if src.Repository == "" {
		RespondError(w, r, http.StatusUnprocessableEntity, "validation_error", "repository is required")
		return
	}
	if err := h.store.CreateSource(r.Context(), &src); err != nil {
		RespondError(w, r, http.StatusInternalServerError, "internal_error", "Failed to create source")
		return
	}
	RespondJSON(w, r, http.StatusCreated, src)
}

// Get handles GET /sources/{id} — returns a single source.
func (h *SourcesHandler) Get(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(r.PathValue("id"))
	if err != nil {
		RespondError(w, r, http.StatusBadRequest, "bad_request", "Invalid source ID")
		return
	}
	src, err := h.store.GetSource(r.Context(), id)
	if err != nil {
		RespondError(w, r, http.StatusNotFound, "not_found", "Source not found")
		return
	}
	RespondJSON(w, r, http.StatusOK, src)
}

// Update handles PUT /sources/{id} — updates an existing source.
func (h *SourcesHandler) Update(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(r.PathValue("id"))
	if err != nil {
		RespondError(w, r, http.StatusBadRequest, "bad_request", "Invalid source ID")
		return
	}
	var src models.Source
	if err := DecodeJSON(r, &src); err != nil {
		RespondError(w, r, http.StatusBadRequest, "bad_request", "Invalid JSON body")
		return
	}
	src.SourceType = strings.TrimSpace(src.SourceType)
	src.Repository = strings.TrimSpace(src.Repository)
	if src.SourceType == "" {
		RespondError(w, r, http.StatusUnprocessableEntity, "validation_error", "type is required")
		return
	}
	if src.Repository == "" {
		RespondError(w, r, http.StatusUnprocessableEntity, "validation_error", "repository is required")
		return
	}
	if err := h.store.UpdateSource(r.Context(), id, &src); err != nil {
		RespondError(w, r, http.StatusNotFound, "not_found", "Source not found")
		return
	}
	src.ID = id
	RespondJSON(w, r, http.StatusOK, src)
}

// Delete handles DELETE /sources/{id} — deletes a source.
func (h *SourcesHandler) Delete(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(r.PathValue("id"))
	if err != nil {
		RespondError(w, r, http.StatusBadRequest, "bad_request", "Invalid source ID")
		return
	}
	if err := h.store.DeleteSource(r.Context(), id); err != nil {
		RespondError(w, r, http.StatusNotFound, "not_found", "Source not found")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// LatestRelease handles GET /sources/{id}/latest-release — returns the latest release for a source.
func (h *SourcesHandler) LatestRelease(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(r.PathValue("id"))
	if err != nil {
		RespondError(w, r, http.StatusBadRequest, "bad_request", "Invalid source ID")
		return
	}
	rv, err := h.store.GetLatestRelease(r.Context(), id)
	if err != nil {
		RespondError(w, r, http.StatusNotFound, "not_found", "No releases found for this source")
		return
	}
	RespondJSON(w, r, http.StatusOK, rv)
}

// ReleaseByVersion handles GET /sources/{id}/releases/{version} — returns a specific release by version.
func (h *SourcesHandler) ReleaseByVersion(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(r.PathValue("id"))
	if err != nil {
		RespondError(w, r, http.StatusBadRequest, "bad_request", "Invalid source ID")
		return
	}
	version := r.PathValue("version")
	if version == "" {
		RespondError(w, r, http.StatusBadRequest, "bad_request", "Version is required")
		return
	}
	rv, err := h.store.GetReleaseByVersion(r.Context(), id, version)
	if err != nil {
		RespondError(w, r, http.StatusNotFound, "not_found", "Release not found")
		return
	}
	RespondJSON(w, r, http.StatusOK, rv)
}
