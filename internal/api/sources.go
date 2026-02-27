package api

import (
	"context"
	"log/slog"
	"net/http"
	"strings"

	"github.com/sentioxyz/changelogue/internal/ingestion"
	"github.com/sentioxyz/changelogue/internal/models"
)

// SourcesStore defines the persistence operations for sources.
type SourcesStore interface {
	ListSourcesByProject(ctx context.Context, projectID string, page, perPage int) ([]models.Source, int, error)
	CreateSource(ctx context.Context, src *models.Source) error
	GetSource(ctx context.Context, id string) (*models.Source, error)
	UpdateSource(ctx context.Context, id string, src *models.Source) error
	DeleteSource(ctx context.Context, id string) error
}

// SourcesHandler implements HTTP handlers for the /sources resource.
type SourcesHandler struct {
	store            SourcesStore
	ingestionService *ingestion.Service
	httpClient       *http.Client
}

// NewSourcesHandler returns a new SourcesHandler.
func NewSourcesHandler(store SourcesStore, ingestionService *ingestion.Service, httpClient *http.Client) *SourcesHandler {
	return &SourcesHandler{store: store, ingestionService: ingestionService, httpClient: httpClient}
}

// List handles GET /projects/{projectId}/sources — returns a paginated list of sources for a project.
func (h *SourcesHandler) List(w http.ResponseWriter, r *http.Request) {
	projectID := r.PathValue("projectId")
	if projectID == "" {
		RespondError(w, r, http.StatusBadRequest, "bad_request", "Project ID is required")
		return
	}
	page, perPage := ParsePagination(r)
	sources, total, err := h.store.ListSourcesByProject(r.Context(), projectID, page, perPage)
	if err != nil {
		RespondError(w, r, http.StatusInternalServerError, "internal_error", "Failed to list sources")
		return
	}
	if sources == nil {
		sources = []models.Source{}
	}
	RespondList(w, r, http.StatusOK, sources, page, perPage, total)
}

// Create handles POST /projects/{projectId}/sources — creates a new source under a project.
func (h *SourcesHandler) Create(w http.ResponseWriter, r *http.Request) {
	projectID := r.PathValue("projectId")
	if projectID == "" {
		RespondError(w, r, http.StatusBadRequest, "bad_request", "Project ID is required")
		return
	}
	var src models.Source
	if err := DecodeJSON(r, &src); err != nil {
		RespondError(w, r, http.StatusBadRequest, "bad_request", "Invalid JSON body")
		return
	}
	src.ProjectID = projectID
	src.Provider = strings.TrimSpace(src.Provider)
	src.Repository = strings.TrimSpace(src.Repository)
	if src.Provider == "" {
		RespondError(w, r, http.StatusUnprocessableEntity, "validation_error", "provider is required")
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
	id := r.PathValue("id")
	if id == "" {
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
	id := r.PathValue("id")
	if id == "" {
		RespondError(w, r, http.StatusBadRequest, "bad_request", "Invalid source ID")
		return
	}
	var src models.Source
	if err := DecodeJSON(r, &src); err != nil {
		RespondError(w, r, http.StatusBadRequest, "bad_request", "Invalid JSON body")
		return
	}
	src.Provider = strings.TrimSpace(src.Provider)
	src.Repository = strings.TrimSpace(src.Repository)
	if src.Provider == "" {
		RespondError(w, r, http.StatusUnprocessableEntity, "validation_error", "provider is required")
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
	id := r.PathValue("id")
	if id == "" {
		RespondError(w, r, http.StatusBadRequest, "bad_request", "Invalid source ID")
		return
	}
	if err := h.store.DeleteSource(r.Context(), id); err != nil {
		RespondError(w, r, http.StatusNotFound, "not_found", "Source not found")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// FetchReleases handles POST /sources/{id}/releases — polls upstream for new
// releases and ingests them immediately.
func (h *SourcesHandler) FetchReleases(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		RespondError(w, r, http.StatusBadRequest, "bad_request", "Invalid source ID")
		return
	}
	src, err := h.store.GetSource(r.Context(), id)
	if err != nil {
		RespondError(w, r, http.StatusNotFound, "not_found", "Source not found")
		return
	}

	ingestionSrc := ingestion.BuildSource(h.httpClient, src.ID, src.Provider, src.Repository)
	if ingestionSrc == nil {
		RespondError(w, r, http.StatusUnprocessableEntity, "unsupported_provider", "Unsupported provider: "+src.Provider)
		return
	}

	results, err := ingestionSrc.FetchNewReleases(r.Context())
	if err != nil {
		slog.Error("fetch releases failed", "source", src.ID, "err", err)
		RespondError(w, r, http.StatusBadGateway, "upstream_error", "Failed to fetch releases from upstream")
		return
	}

	newCount := 0
	if len(results) > 0 && h.ingestionService != nil {
		for _, result := range results {
			if procErr := h.ingestionService.ProcessResults(r.Context(), src.ID, src.Provider, []ingestion.IngestionResult{result}); procErr != nil {
				slog.Error("process release failed", "source", src.ID, "version", result.RawVersion, "err", procErr)
				continue
			}
			newCount++
		}
	}

	RespondJSON(w, r, http.StatusOK, map[string]int{"new_releases": newCount})
}
