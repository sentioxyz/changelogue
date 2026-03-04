package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"sync"
	"time"
)

const discoverCacheTTL = 1 * time.Hour

// DiscoverItem represents a single repository/image result from a discovery search.
type DiscoverItem struct {
	Name        string `json:"name"`
	FullName    string `json:"full_name"`
	Description string `json:"description"`
	Stars       int    `json:"stars"`
	Language    string `json:"language,omitempty"`
	URL         string `json:"url"`
	AvatarURL   string `json:"avatar_url,omitempty"`
	Provider    string `json:"provider"`
}

type cachedResult struct {
	items     []DiscoverItem
	fetchedAt time.Time
}

// DiscoverHandler implements HTTP handlers for the /discover resource.
type DiscoverHandler struct {
	client       *http.Client
	githubURL    string
	dockerHubURL string
	mu           sync.Mutex
	cache        map[string]cachedResult
}

// NewDiscoverHandler returns a new DiscoverHandler. Empty URL strings default
// to the real GitHub / Docker Hub API base URLs.
func NewDiscoverHandler(client *http.Client, githubURL, dockerHubURL string) *DiscoverHandler {
	if githubURL == "" {
		githubURL = "https://api.github.com"
	}
	if dockerHubURL == "" {
		dockerHubURL = "https://hub.docker.com"
	}
	return &DiscoverHandler{
		client:       client,
		githubURL:    githubURL,
		dockerHubURL: dockerHubURL,
		cache:        make(map[string]cachedResult),
	}
}

func (h *DiscoverHandler) getCached(key string) ([]DiscoverItem, bool) {
	h.mu.Lock()
	defer h.mu.Unlock()
	c, ok := h.cache[key]
	if !ok {
		return nil, false
	}
	if time.Since(c.fetchedAt) > discoverCacheTTL {
		delete(h.cache, key)
		return nil, false
	}
	return c.items, true
}

func (h *DiscoverHandler) setCache(key string, items []DiscoverItem) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.cache[key] = cachedResult{items: items, fetchedAt: time.Now()}
}

// GitHub handles GET /discover/github — searches GitHub repositories.
func (h *DiscoverHandler) GitHub(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query().Get("q")
	language := r.URL.Query().Get("language")

	cacheKey := fmt.Sprintf("github:%s:%s", q, language)
	if items, ok := h.getCached(cacheKey); ok {
		RespondJSON(w, r, http.StatusOK, items)
		return
	}

	// Approximate GitHub Trending: search for repos with recent star activity.
	// Default query filters to repos created in the last 7 days, sorted by stars.
	since := time.Now().AddDate(0, 0, -7).Format("2006-01-02")
	searchQ := q
	if searchQ == "" {
		searchQ = fmt.Sprintf("created:>%s stars:>10", since)
	}
	if language != "" {
		searchQ = fmt.Sprintf("%s language:%s", searchQ, language)
	}

	u, _ := url.Parse(h.githubURL + "/search/repositories")
	params := url.Values{}
	params.Set("q", searchQ)
	params.Set("sort", "stars")
	params.Set("order", "desc")
	params.Set("per_page", "25")
	u.RawQuery = params.Encode()

	req, err := http.NewRequestWithContext(r.Context(), http.MethodGet, u.String(), nil)
	if err != nil {
		RespondError(w, r, http.StatusInternalServerError, "request_error", "failed to build GitHub request")
		return
	}
	resp, err := h.client.Do(req)
	if err != nil {
		RespondError(w, r, http.StatusBadGateway, "upstream_error", "failed to reach GitHub API")
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		RespondError(w, r, http.StatusBadGateway, "upstream_error", fmt.Sprintf("GitHub API returned %d", resp.StatusCode))
		return
	}

	var ghResp struct {
		Items []struct {
			Name        string `json:"name"`
			FullName    string `json:"full_name"`
			Description string `json:"description"`
			Stars       int    `json:"stargazers_count"`
			Language    string `json:"language"`
			HTMLURL     string `json:"html_url"`
			Owner       struct {
				AvatarURL string `json:"avatar_url"`
			} `json:"owner"`
		} `json:"items"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&ghResp); err != nil {
		RespondError(w, r, http.StatusInternalServerError, "decode_error", "failed to parse GitHub response")
		return
	}

	items := make([]DiscoverItem, 0, len(ghResp.Items))
	for _, repo := range ghResp.Items {
		items = append(items, DiscoverItem{
			Name:        repo.Name,
			FullName:    repo.FullName,
			Description: repo.Description,
			Stars:       repo.Stars,
			Language:    repo.Language,
			URL:         repo.HTMLURL,
			AvatarURL:   repo.Owner.AvatarURL,
			Provider:    "github",
		})
	}

	h.setCache(cacheKey, items)
	RespondJSON(w, r, http.StatusOK, items)
}

// DockerHub handles GET /discover/dockerhub — searches Docker Hub repositories.
func (h *DiscoverHandler) DockerHub(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query().Get("q")
	if q == "" {
		q = "nginx"
	}

	cacheKey := fmt.Sprintf("dockerhub:%s", q)
	if items, ok := h.getCached(cacheKey); ok {
		RespondJSON(w, r, http.StatusOK, items)
		return
	}

	u, _ := url.Parse(h.dockerHubURL + "/v2/search/repositories/")
	params := url.Values{}
	params.Set("query", q)
	params.Set("page_size", "25")
	u.RawQuery = params.Encode()

	req, err := http.NewRequestWithContext(r.Context(), http.MethodGet, u.String(), nil)
	if err != nil {
		RespondError(w, r, http.StatusInternalServerError, "request_error", "failed to build Docker Hub request")
		return
	}
	resp, err := h.client.Do(req)
	if err != nil {
		RespondError(w, r, http.StatusBadGateway, "upstream_error", "failed to reach Docker Hub API")
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		RespondError(w, r, http.StatusBadGateway, "upstream_error", fmt.Sprintf("Docker Hub API returned %d", resp.StatusCode))
		return
	}

	var dhResp struct {
		Results []struct {
			RepoName         string `json:"repo_name"`
			ShortDescription string `json:"short_description"`
			StarCount        int    `json:"star_count"`
			IsOfficial       bool   `json:"is_official"`
		} `json:"results"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&dhResp); err != nil {
		RespondError(w, r, http.StatusInternalServerError, "decode_error", "failed to parse Docker Hub response")
		return
	}

	items := make([]DiscoverItem, 0, len(dhResp.Results))
	for _, repo := range dhResp.Results {
		name := repo.RepoName
		fullName := repo.RepoName
		if repo.IsOfficial {
			fullName = "library/" + repo.RepoName
		}
		hubURL := fmt.Sprintf("https://hub.docker.com/r/%s", repo.RepoName)
		if repo.IsOfficial {
			hubURL = fmt.Sprintf("https://hub.docker.com/_/%s", repo.RepoName)
		}
		items = append(items, DiscoverItem{
			Name:        name,
			FullName:    fullName,
			Description: repo.ShortDescription,
			Stars:       repo.StarCount,
			URL:         hubURL,
			Provider:    "dockerhub",
		})
	}

	h.setCache(cacheKey, items)
	RespondJSON(w, r, http.StatusOK, items)
}
