package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"sync"
	"time"

	"github.com/sentioxyz/changelogue/internal/auth"
	"github.com/sentioxyz/changelogue/internal/models"
)

const suggestionsCacheTTL = 1 * time.Hour

type SuggestionItem struct {
	Name        string `json:"name"`
	FullName    string `json:"full_name"`
	Description string `json:"description"`
	Stars       int    `json:"stars"`
	Language    string `json:"language,omitempty"`
	URL         string `json:"url"`
	AvatarURL   string `json:"avatar_url,omitempty"`
	Provider    string `json:"provider"`
	Tracked     bool   `json:"tracked"`
}

type RepoItem struct {
	Name        string `json:"name"`
	FullName    string `json:"full_name"`
	Description string `json:"description"`
	Language    string `json:"language,omitempty"`
	URL         string `json:"url"`
	PushedAt    string `json:"pushed_at"`
}

type SuggestionsSourceStore interface {
	ListAllSourceRepos(ctx context.Context) ([]models.SourceRepo, error)
}

type suggestionsCache struct {
	data      json.RawMessage
	fetchedAt time.Time
}

type SuggestionsHandler struct {
	client    *http.Client
	store     SuggestionsSourceStore
	token     string
	githubURL string
	mu        sync.Mutex
	cache     map[string]suggestionsCache
}

func NewSuggestionsHandler(client *http.Client, store SuggestionsSourceStore, token, githubURL string) *SuggestionsHandler {
	if token == "" {
		token = os.Getenv("GITHUB_TOKEN")
	}
	if githubURL == "" {
		githubURL = "https://api.github.com"
	}
	return &SuggestionsHandler{
		client:    client,
		store:     store,
		token:     token,
		githubURL: githubURL,
		cache:     make(map[string]suggestionsCache),
	}
}

func (h *SuggestionsHandler) getCached(key string) (json.RawMessage, bool) {
	h.mu.Lock()
	defer h.mu.Unlock()
	c, ok := h.cache[key]
	if !ok {
		return nil, false
	}
	if time.Since(c.fetchedAt) > suggestionsCacheTTL {
		delete(h.cache, key)
		return nil, false
	}
	return c.data, true
}

func (h *SuggestionsHandler) setCache(key string, data json.RawMessage) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.cache[key] = suggestionsCache{data: data, fetchedAt: time.Now()}
}

func (h *SuggestionsHandler) githubGet(ctx context.Context, path string) (json.RawMessage, error) {
	url := h.githubURL + path
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}
	if h.token != "" {
		req.Header.Set("Authorization", "Bearer "+h.token)
	}
	req.Header.Set("Accept", "application/vnd.github+json")

	resp, err := h.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusForbidden || resp.StatusCode == http.StatusTooManyRequests {
		return nil, fmt.Errorf("github rate limit exceeded")
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("GitHub API returned %d", resp.StatusCode)
	}

	var raw json.RawMessage
	if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
		return nil, fmt.Errorf("decode GitHub response: %w", err)
	}
	return raw, nil
}

func (h *SuggestionsHandler) Stars(w http.ResponseWriter, r *http.Request) {
	user := auth.UserFromContext(r.Context())
	if user == nil || user.GitHubLogin == "" || user.GitHubLogin == "dev" {
		RespondJSON(w, r, http.StatusOK, []SuggestionItem{})
		return
	}

	login := user.GitHubLogin
	cacheKey := "stars:" + login

	raw, ok := h.getCached(cacheKey)
	if !ok {
		var err error
		var allStars []json.RawMessage
		for page := 1; page <= 4; page++ {
			path := fmt.Sprintf("/users/%s/starred?per_page=30&page=%d", login, page)
			pageRaw, fetchErr := h.githubGet(r.Context(), path)
			if fetchErr != nil {
				if page == 1 {
					RespondError(w, r, http.StatusBadGateway, "upstream_error", fetchErr.Error())
					return
				}
				break
			}
			var pageItems []json.RawMessage
			if err = json.Unmarshal(pageRaw, &pageItems); err != nil {
				RespondError(w, r, http.StatusBadGateway, "upstream_error", "failed to parse GitHub response")
				return
			}
			allStars = append(allStars, pageItems...)
			if len(pageItems) < 30 {
				break
			}
		}
		raw, err = json.Marshal(allStars)
		if err != nil {
			RespondError(w, r, http.StatusInternalServerError, "internal_error", "failed to marshal stars")
			return
		}
		h.setCache(cacheKey, raw)
	}

	var ghStars []struct {
		Name     string `json:"name"`
		FullName string `json:"full_name"`
		Desc     string `json:"description"`
		Stars    int    `json:"stargazers_count"`
		Language string `json:"language"`
		URL      string `json:"html_url"`
		Owner    struct {
			AvatarURL string `json:"avatar_url"`
		} `json:"owner"`
	}
	if err := json.Unmarshal(raw, &ghStars); err != nil {
		RespondError(w, r, http.StatusInternalServerError, "internal_error", "failed to parse cached stars")
		return
	}

	tracked := make(map[string]bool)
	if repos, err := h.store.ListAllSourceRepos(r.Context()); err == nil {
		for _, repo := range repos {
			tracked[repo.Provider+":"+repo.Repository] = true
		}
	}

	items := make([]SuggestionItem, 0, len(ghStars))
	for _, s := range ghStars {
		items = append(items, SuggestionItem{
			Name:        s.Name,
			FullName:    s.FullName,
			Description: s.Desc,
			Stars:       s.Stars,
			Language:    s.Language,
			URL:         s.URL,
			AvatarURL:   s.Owner.AvatarURL,
			Provider:    "github",
			Tracked:     tracked["github:"+s.FullName],
		})
	}

	RespondJSON(w, r, http.StatusOK, items)
}

func (h *SuggestionsHandler) Repos(w http.ResponseWriter, r *http.Request) {
	user := auth.UserFromContext(r.Context())
	if user == nil || user.GitHubLogin == "" || user.GitHubLogin == "dev" {
		RespondJSON(w, r, http.StatusOK, []RepoItem{})
		return
	}

	login := user.GitHubLogin
	cacheKey := "repos:" + login

	raw, ok := h.getCached(cacheKey)
	if !ok {
		var err error
		raw, err = h.githubGet(r.Context(), fmt.Sprintf("/users/%s/repos?sort=pushed&per_page=100", login))
		if err != nil {
			RespondError(w, r, http.StatusBadGateway, "upstream_error", err.Error())
			return
		}
		h.setCache(cacheKey, raw)
	}

	var ghRepos []struct {
		Name     string `json:"name"`
		FullName string `json:"full_name"`
		Desc     string `json:"description"`
		Language string `json:"language"`
		URL      string `json:"html_url"`
		PushedAt string `json:"pushed_at"`
	}
	if err := json.Unmarshal(raw, &ghRepos); err != nil {
		RespondError(w, r, http.StatusInternalServerError, "internal_error", "failed to parse cached repos")
		return
	}

	items := make([]RepoItem, 0, len(ghRepos))
	for _, repo := range ghRepos {
		items = append(items, RepoItem{
			Name:        repo.Name,
			FullName:    repo.FullName,
			Description: repo.Desc,
			Language:    repo.Language,
			URL:         repo.URL,
			PushedAt:    repo.PushedAt,
		})
	}

	RespondJSON(w, r, http.StatusOK, items)
}
