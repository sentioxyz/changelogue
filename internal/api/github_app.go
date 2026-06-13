package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"strings"

	"github.com/sentioxyz/changelogue/internal/githubauth"
	"github.com/sentioxyz/changelogue/internal/models"
)

type GitHubAppStore interface {
	ListGitHubAppInstallations(ctx context.Context) ([]models.GitHubAppInstallation, error)
	UpsertGitHubAppInstallation(ctx context.Context, inst models.GitHubAppInstallation) error
	ReplaceGitHubAppRepositories(ctx context.Context, installationID int64, repos []models.GitHubAppRepository) error
	ListGitHubAppRepositories(ctx context.Context) ([]models.GitHubAppRepository, error)
}

type GitHubAppClient interface {
	Configured() bool
	AppID() string
	ListInstallations(ctx context.Context) ([]githubauth.Installation, error)
	ListInstallationRepositories(ctx context.Context, installationID int64) ([]githubauth.Repository, error)
}

type GitHubAppHandler struct {
	store  GitHubAppStore
	client GitHubAppClient
}

func NewGitHubAppHandler(store GitHubAppStore, client GitHubAppClient) *GitHubAppHandler {
	return &GitHubAppHandler{store: store, client: client}
}

func (h *GitHubAppHandler) Status(w http.ResponseWriter, r *http.Request) {
	installations, err := h.store.ListGitHubAppInstallations(r.Context())
	if err != nil {
		RespondError(w, r, http.StatusInternalServerError, "internal_error", "Failed to list GitHub App installations")
		return
	}
	status := models.GitHubAppStatus{
		Configured:    h.client != nil && h.client.Configured(),
		Installations: installations,
	}
	if h.client != nil {
		status.AppID = h.client.AppID()
	}
	if name := strings.TrimSpace(os.Getenv("GITHUB_APP_NAME")); name != "" {
		status.InstallURL = "https://github.com/apps/" + name + "/installations/new"
	}
	RespondJSON(w, r, http.StatusOK, status)
}

func (h *GitHubAppHandler) Sync(w http.ResponseWriter, r *http.Request) {
	if h.client == nil || !h.client.Configured() {
		RespondError(w, r, http.StatusUnprocessableEntity, "github_app_not_configured", "GitHub App credentials are not configured on the server")
		return
	}
	installations, err := h.client.ListInstallations(r.Context())
	if err != nil {
		h.respondGitHubError(w, r, err)
		return
	}
	for _, inst := range installations {
		permissions := json.RawMessage(`{}`)
		if len(inst.Permissions) > 0 {
			permissions = inst.Permissions
		}
		model := models.GitHubAppInstallation{
			InstallationID:      inst.ID,
			AccountLogin:        inst.AccountLogin,
			AccountType:         inst.AccountType,
			RepositorySelection: inst.RepositorySelection,
			Permissions:         permissions,
		}
		if err := h.store.UpsertGitHubAppInstallation(r.Context(), model); err != nil {
			RespondError(w, r, http.StatusInternalServerError, "internal_error", "Failed to save GitHub App installation")
			return
		}

		repos, err := h.client.ListInstallationRepositories(r.Context(), inst.ID)
		if err != nil {
			h.respondGitHubError(w, r, err)
			return
		}
		modelsRepos := make([]models.GitHubAppRepository, 0, len(repos))
		for _, repo := range repos {
			modelsRepos = append(modelsRepos, models.GitHubAppRepository{
				InstallationID: inst.ID,
				FullName:       repo.FullName,
				Private:        repo.Private,
				HTMLURL:        repo.HTMLURL,
			})
		}
		if err := h.store.ReplaceGitHubAppRepositories(r.Context(), inst.ID, modelsRepos); err != nil {
			RespondError(w, r, http.StatusInternalServerError, "internal_error", "Failed to save GitHub App repositories")
			return
		}
	}
	RespondJSON(w, r, http.StatusOK, map[string]int{"installations": len(installations)})
}

func (h *GitHubAppHandler) Repositories(w http.ResponseWriter, r *http.Request) {
	repos, err := h.store.ListGitHubAppRepositories(r.Context())
	if err != nil {
		RespondError(w, r, http.StatusInternalServerError, "internal_error", "Failed to list GitHub App repositories")
		return
	}
	if repos == nil {
		repos = []models.GitHubAppRepository{}
	}
	RespondJSON(w, r, http.StatusOK, repos)
}

func (h *GitHubAppHandler) respondGitHubError(w http.ResponseWriter, r *http.Request, err error) {
	if errors.Is(err, githubauth.ErrNotConfigured) {
		RespondError(w, r, http.StatusUnprocessableEntity, "github_app_not_configured", "GitHub App credentials are not configured on the server")
		return
	}
	if errors.Is(err, githubauth.ErrRepoUnauthorized) {
		RespondError(w, r, http.StatusBadGateway, "github_app_unauthorized", "GitHub App credentials were rejected by GitHub")
		return
	}
	if errors.Is(err, githubauth.ErrRateLimited) {
		RespondError(w, r, http.StatusTooManyRequests, "github_rate_limited", "GitHub API rate limit exceeded")
		return
	}
	RespondError(w, r, http.StatusBadGateway, "github_error", fmt.Sprintf("GitHub API error: %v", err))
}
