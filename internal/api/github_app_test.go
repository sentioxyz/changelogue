package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/sentioxyz/changelogue/internal/githubauth"
	"github.com/sentioxyz/changelogue/internal/models"
)

func TestGitHubAppSyncStoresInstallationsAndRepositories(t *testing.T) {
	store := &mockGitHubAppStore{}
	client := &mockGitHubAppClient{
		configured: true,
		appID:      "42",
		installations: []githubauth.Installation{{
			ID:                  123,
			AccountLogin:        "acme",
			AccountType:         "Organization",
			RepositorySelection: "selected",
			Permissions:         json.RawMessage(`{"contents":"read"}`),
		}},
		repos: map[int64][]githubauth.Repository{
			123: {{FullName: "acme/private", Private: true, HTMLURL: "https://github.com/acme/private"}},
		},
	}
	h := NewGitHubAppHandler(store, client)
	req := httptest.NewRequest(http.MethodPost, "/github-app/sync", nil)
	w := httptest.NewRecorder()

	h.Sync(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", w.Code, w.Body.String())
	}
	if len(store.installations) != 1 || store.installations[0].InstallationID != 123 {
		t.Fatalf("installations = %+v", store.installations)
	}
	if len(store.repositories[123]) != 1 || store.repositories[123][0].FullName != "acme/private" {
		t.Fatalf("repositories = %+v", store.repositories)
	}
}

type mockGitHubAppClient struct {
	configured    bool
	appID         string
	installations []githubauth.Installation
	repos         map[int64][]githubauth.Repository
}

func (m *mockGitHubAppClient) Configured() bool { return m.configured }
func (m *mockGitHubAppClient) AppID() string    { return m.appID }
func (m *mockGitHubAppClient) ListInstallations(context.Context) ([]githubauth.Installation, error) {
	return m.installations, nil
}
func (m *mockGitHubAppClient) ListInstallationRepositories(_ context.Context, installationID int64) ([]githubauth.Repository, error) {
	return m.repos[installationID], nil
}

type mockGitHubAppStore struct {
	installations []models.GitHubAppInstallation
	repositories  map[int64][]models.GitHubAppRepository
}

func (m *mockGitHubAppStore) ListGitHubAppInstallations(context.Context) ([]models.GitHubAppInstallation, error) {
	return m.installations, nil
}

func (m *mockGitHubAppStore) UpsertGitHubAppInstallation(_ context.Context, inst models.GitHubAppInstallation) error {
	inst.ID = "inst-id"
	inst.CreatedAt = time.Now()
	inst.UpdatedAt = time.Now()
	m.installations = append(m.installations, inst)
	return nil
}

func (m *mockGitHubAppStore) ReplaceGitHubAppRepositories(_ context.Context, installationID int64, repos []models.GitHubAppRepository) error {
	if m.repositories == nil {
		m.repositories = map[int64][]models.GitHubAppRepository{}
	}
	m.repositories[installationID] = repos
	return nil
}

func (m *mockGitHubAppStore) ListGitHubAppRepositories(context.Context) ([]models.GitHubAppRepository, error) {
	var repos []models.GitHubAppRepository
	for _, items := range m.repositories {
		repos = append(repos, items...)
	}
	return repos, nil
}
