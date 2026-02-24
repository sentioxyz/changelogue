package api

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"
	"time"
)

// ListReleasesOpts holds filter and sort parameters for listing releases.
type ListReleasesOpts struct {
	Page, PerPage int
	ProjectID     *int
	SourceID      *int
	Sort, Order   string
}

// PipelineStatus represents the current state of a release's pipeline processing.
type PipelineStatus struct {
	ReleaseID   string          `json:"release_id"`
	State       string          `json:"state"`
	CurrentNode *string         `json:"current_node"`
	NodeResults json.RawMessage `json:"node_results"`
	Attempt     int             `json:"attempt"`
	CompletedAt *time.Time      `json:"completed_at,omitempty"`
}

// ReleasesStore defines the persistence operations for releases (read-only).
type ReleasesStore interface {
	ListReleases(ctx context.Context, opts ListReleasesOpts) ([]ReleaseView, int, error)
	GetRelease(ctx context.Context, id string) (*ReleaseView, error)
	GetReleaseNotes(ctx context.Context, id string) (string, error)
	GetPipelineStatus(ctx context.Context, releaseID string) (*PipelineStatus, error)
}

// ReleasesHandler implements HTTP handlers for the /releases resource.
type ReleasesHandler struct {
	store ReleasesStore
}

// NewReleasesHandler returns a new ReleasesHandler.
func NewReleasesHandler(store ReleasesStore) *ReleasesHandler {
	return &ReleasesHandler{store: store}
}

// List handles GET /releases — returns a paginated, filterable list of releases.
func (h *ReleasesHandler) List(w http.ResponseWriter, r *http.Request) {
	page, perPage := ParsePagination(r)
	opts := ListReleasesOpts{
		Page:    page,
		PerPage: perPage,
		Sort:    r.URL.Query().Get("sort"),
		Order:   r.URL.Query().Get("order"),
	}
	if v := r.URL.Query().Get("project_id"); v != "" {
		if pid, err := strconv.Atoi(v); err == nil {
			opts.ProjectID = &pid
		}
	}
	if v := r.URL.Query().Get("source_id"); v != "" {
		if sid, err := strconv.Atoi(v); err == nil {
			opts.SourceID = &sid
		}
	}
	releases, total, err := h.store.ListReleases(r.Context(), opts)
	if err != nil {
		RespondError(w, r, http.StatusInternalServerError, "internal_error", "Failed to list releases")
		return
	}
	if releases == nil {
		releases = []ReleaseView{}
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
	rv, err := h.store.GetRelease(r.Context(), id)
	if err != nil {
		RespondError(w, r, http.StatusNotFound, "not_found", "Release not found")
		return
	}
	RespondJSON(w, r, http.StatusOK, rv)
}

// Pipeline handles GET /releases/{id}/pipeline — returns pipeline status for a release.
func (h *ReleasesHandler) Pipeline(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		RespondError(w, r, http.StatusBadRequest, "bad_request", "Release ID is required")
		return
	}
	ps, err := h.store.GetPipelineStatus(r.Context(), id)
	if err != nil {
		RespondError(w, r, http.StatusNotFound, "not_found", "Pipeline status not found")
		return
	}
	RespondJSON(w, r, http.StatusOK, ps)
}

// Notes handles GET /releases/{id}/notes — returns changelog notes for a release.
func (h *ReleasesHandler) Notes(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		RespondError(w, r, http.StatusBadRequest, "bad_request", "Release ID is required")
		return
	}
	changelog, err := h.store.GetReleaseNotes(r.Context(), id)
	if err != nil {
		RespondError(w, r, http.StatusNotFound, "not_found", "Release not found")
		return
	}
	RespondJSON(w, r, http.StatusOK, map[string]string{"changelog": changelog})
}
