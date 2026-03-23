package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/sentioxyz/changelogue/internal/auth"
	"github.com/sentioxyz/changelogue/internal/models"
)

type mockSuggestionsStore struct{}

func (m *mockSuggestionsStore) ListAllSourceRepos(_ context.Context) ([]models.SourceRepo, error) {
	return []models.SourceRepo{
		{Provider: "github", Repository: "already/tracked"},
	}, nil
}

func TestSuggestionsStars(t *testing.T) {
	mockGH := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/users/testuser/starred" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `[
			{
				"name": "cool-repo",
				"full_name": "org/cool-repo",
				"description": "A cool repo",
				"stargazers_count": 500,
				"language": "Go",
				"html_url": "https://github.com/org/cool-repo",
				"owner": {"avatar_url": "https://example.com/avatar.png"}
			},
			{
				"name": "tracked",
				"full_name": "already/tracked",
				"description": "Already tracked",
				"stargazers_count": 100,
				"language": "Go",
				"html_url": "https://github.com/already/tracked",
				"owner": {"avatar_url": "https://example.com/avatar2.png"}
			}
		]`)
	}))
	defer mockGH.Close()

	h := NewSuggestionsHandler(mockGH.Client(), &mockSuggestionsStore{}, "", mockGH.URL)

	mux := http.NewServeMux()
	mux.HandleFunc("GET /suggestions/stars", h.Stars)

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/suggestions/stars", nil)
	ctx := auth.WithUser(r.Context(), &auth.User{GitHubLogin: "testuser"})
	r = r.WithContext(ctx)

	mux.ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var got struct {
		Data []SuggestionItem `json:"data"`
	}
	if err := json.NewDecoder(w.Body).Decode(&got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(got.Data) != 2 {
		t.Fatalf("expected 2 items, got %d", len(got.Data))
	}
	if got.Data[0].Tracked {
		t.Error("expected org/cool-repo to not be tracked")
	}
	if !got.Data[1].Tracked {
		t.Error("expected already/tracked to be tracked")
	}
}

func TestSuggestionsStarsNoUser(t *testing.T) {
	h := NewSuggestionsHandler(http.DefaultClient, &mockSuggestionsStore{}, "", "")

	mux := http.NewServeMux()
	mux.HandleFunc("GET /suggestions/stars", h.Stars)

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/suggestions/stars", nil)

	mux.ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var got struct {
		Data []SuggestionItem `json:"data"`
	}
	if err := json.NewDecoder(w.Body).Decode(&got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(got.Data) != 0 {
		t.Fatalf("expected 0 items for no user, got %d", len(got.Data))
	}
}

func TestSuggestionsRepos(t *testing.T) {
	mockGH := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/users/testuser/repos" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `[
			{
				"name": "my-app",
				"full_name": "testuser/my-app",
				"description": "My application",
				"language": "TypeScript",
				"html_url": "https://github.com/testuser/my-app",
				"pushed_at": "2026-03-20T10:00:00Z"
			}
		]`)
	}))
	defer mockGH.Close()

	h := NewSuggestionsHandler(mockGH.Client(), &mockSuggestionsStore{}, "", mockGH.URL)

	mux := http.NewServeMux()
	mux.HandleFunc("GET /suggestions/repos", h.Repos)

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/suggestions/repos", nil)
	ctx := auth.WithUser(r.Context(), &auth.User{GitHubLogin: "testuser"})
	r = r.WithContext(ctx)

	mux.ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var got struct {
		Data []RepoItem `json:"data"`
	}
	if err := json.NewDecoder(w.Body).Decode(&got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(got.Data) != 1 {
		t.Fatalf("expected 1 repo, got %d", len(got.Data))
	}
	if got.Data[0].FullName != "testuser/my-app" {
		t.Errorf("expected testuser/my-app, got %s", got.Data[0].FullName)
	}
}

func TestSuggestionsStarsCaching(t *testing.T) {
	var callCount atomic.Int32
	mockGH := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount.Add(1)
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `[]`)
	}))
	defer mockGH.Close()

	h := NewSuggestionsHandler(mockGH.Client(), &mockSuggestionsStore{}, "", mockGH.URL)

	mux := http.NewServeMux()
	mux.HandleFunc("GET /suggestions/stars", h.Stars)

	for i := 0; i < 3; i++ {
		w := httptest.NewRecorder()
		r := httptest.NewRequest(http.MethodGet, "/suggestions/stars", nil)
		ctx := auth.WithUser(r.Context(), &auth.User{GitHubLogin: "cacheuser"})
		r = r.WithContext(ctx)
		mux.ServeHTTP(w, r)
	}

	if calls := callCount.Load(); calls != 1 {
		t.Fatalf("expected 1 GitHub API call (rest cached), got %d", calls)
	}
}
