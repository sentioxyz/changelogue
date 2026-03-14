package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/sentioxyz/changelogue/internal/models"
	"github.com/sentioxyz/changelogue/internal/onboard"
)

// OnboardSelection represents a user's choice for one dependency.
type OnboardSelection struct {
	DepName        string `json:"dep_name"`
	UpstreamRepo   string `json:"upstream_repo"`
	Provider       string `json:"provider"`
	ProjectID      string `json:"project_id,omitempty"`
	NewProjectName string `json:"new_project_name,omitempty"`
}

// OnboardApplyResult holds the outcome of applying scan selections.
type OnboardApplyResult struct {
	CreatedProjects []models.Project `json:"created_projects"`
	CreatedSources  []models.Source  `json:"created_sources"`
	Skipped         []string         `json:"skipped"`
}

// OnboardStore defines the data access interface for onboarding operations.
type OnboardStore interface {
	CreateOnboardScan(ctx context.Context, repoURL string) (*models.OnboardScan, error)
	GetOnboardScan(ctx context.Context, id string) (*models.OnboardScan, error)
	UpdateOnboardScanStatus(ctx context.Context, id, status string, results json.RawMessage, scanErr string) error
	ActiveScanForRepo(ctx context.Context, repoURL string) (*models.OnboardScan, error)
	ApplyOnboardScan(ctx context.Context, scanID string, selections []OnboardSelection) (*OnboardApplyResult, error)
}

// OnboardHandler handles HTTP requests for repo onboarding.
type OnboardHandler struct {
	store OnboardStore
}

// NewOnboardHandler creates a new OnboardHandler.
func NewOnboardHandler(store OnboardStore) *OnboardHandler {
	return &OnboardHandler{store: store}
}

// Scan creates a new dependency scan for a GitHub repo.
func (h *OnboardHandler) Scan(w http.ResponseWriter, r *http.Request) {
	var input struct {
		RepoURL string `json:"repo_url"`
	}
	if err := DecodeJSON(r, &input); err != nil {
		RespondError(w, r, http.StatusBadRequest, "bad_request", err.Error())
		return
	}

	// Normalize the repo URL
	owner, repo, err := onboard.ParseRepoURL(input.RepoURL)
	if err != nil || strings.TrimSpace(input.RepoURL) == "" {
		RespondError(w, r, http.StatusUnprocessableEntity, "validation_error", "Invalid or empty repo URL")
		return
	}
	normalizedURL := fmt.Sprintf("%s/%s", owner, repo)

	// Check for active scan
	active, err := h.store.ActiveScanForRepo(r.Context(), normalizedURL)
	if err != nil {
		RespondError(w, r, http.StatusInternalServerError, "internal_error", err.Error())
		return
	}
	if active != nil {
		RespondError(w, r, http.StatusConflict, "conflict", fmt.Sprintf("Active scan already exists: %s", active.ID))
		return
	}

	scan, err := h.store.CreateOnboardScan(r.Context(), normalizedURL)
	if err != nil {
		RespondError(w, r, http.StatusInternalServerError, "internal_error", err.Error())
		return
	}
	RespondJSON(w, r, http.StatusCreated, scan)
}

// GetScan returns a single scan by ID.
func (h *OnboardHandler) GetScan(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	scan, err := h.store.GetOnboardScan(r.Context(), id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			RespondError(w, r, http.StatusNotFound, "not_found", "Scan not found")
			return
		}
		RespondError(w, r, http.StatusInternalServerError, "internal_error", err.Error())
		return
	}
	RespondJSON(w, r, http.StatusOK, scan)
}

// Apply creates projects and sources from the user's selections.
func (h *OnboardHandler) Apply(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	var input struct {
		Selections []OnboardSelection `json:"selections"`
	}
	if err := DecodeJSON(r, &input); err != nil {
		RespondError(w, r, http.StatusBadRequest, "bad_request", err.Error())
		return
	}

	// Validate selections
	for i, sel := range input.Selections {
		hasProjectID := sel.ProjectID != ""
		hasNewName := sel.NewProjectName != ""
		if hasProjectID == hasNewName {
			RespondError(w, r, http.StatusBadRequest, "validation_error",
				fmt.Sprintf("selection[%d]: exactly one of project_id or new_project_name must be set", i))
			return
		}
	}

	result, err := h.store.ApplyOnboardScan(r.Context(), id, input.Selections)
	if err != nil {
		RespondError(w, r, http.StatusInternalServerError, "internal_error", err.Error())
		return
	}
	RespondJSON(w, r, http.StatusOK, result)
}
