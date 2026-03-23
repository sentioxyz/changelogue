# Personalized GitHub Suggestions Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add personalized repo/dependency suggestions to the dashboard — show authenticated users their GitHub stars and let them scan their own repos for dependencies to track.

**Architecture:** Two new backend endpoints proxy the public GitHub API (using the server-side `GITHUB_TOKEN`) to fetch a user's starred repos and public repos by `github_login`. A new frontend `SuggestionsSection` component with two tabs replaces the trending carousel for authenticated users. The dependencies tab reuses the existing onboard scan pipeline for LLM-powered dependency extraction.

**Tech Stack:** Go (backend handlers, GitHub API client, caching), Next.js/React (frontend components, SWR), existing onboard scan pipeline

**Spec:** `docs/superpowers/specs/2026-03-23-personalized-suggestions-design.md`

---

## File Structure

### New files
| File | Responsibility |
|------|---------------|
| `internal/api/suggestions.go` | `SuggestionsHandler` struct, constructor, `Stars()` and `Repos()` handlers, GitHub API fetching, per-user caching |
| `internal/api/suggestions_test.go` | Unit tests with mock GitHub API server and mock store |
| `web/components/dashboard/suggestions-section.tsx` | Top-level tabbed component ("Your Stars" / "Your Dependencies"), auth-gated |
| `web/components/dashboard/stars-tab.tsx` | Stars tab: fetch, render grid, one-click track |
| `web/components/dashboard/deps-tab.tsx` | Dependencies tab: repo picker → scan → results → apply |

### Modified files
| File | Change |
|------|--------|
| `internal/api/server.go` | Register `/api/v1/suggestions/stars` and `/api/v1/suggestions/repos` routes on auth chain |
| `internal/api/pgstore.go` | Add `ListAllSourceRepos()` method — returns set of `(provider, repository)` pairs |
| `internal/api/sources.go` | Add `ListAllSourceRepos()` to `SourcesStore` interface |
| `web/lib/api/client.ts` | Add `suggestions.stars()` and `suggestions.repos()` methods |
| `web/lib/api/types.ts` | Add `SuggestionItem` and `RepoItem` types |
| `internal/models/source.go` | Add `SourceRepo` struct |
| `web/app/page.tsx` | Conditionally render `SuggestionsSection` vs `DiscoverySection` based on `user.github_login` |

### Deferred from spec (simplifications for v1)
- **API key auth 403**: Plan returns empty array instead of 403 for API key callers (simpler, no behavioral difference for users)
- **"Show more" pagination**: Stars tab renders all items; can add client-side pagination in a follow-up
- **Project assignment dropdown**: DepsTab auto-creates projects from dependency names; dropdown for assigning to existing projects deferred
- **Already-tracked in DepsTab**: Deps results don't indicate already-tracked deps; deferred to follow-up
- **Empty state suggestions**: Simple messages without fallback links; can enhance later

---

## Task 1: Add `ListAllSourceRepos` to store layer

**Files:**
- Modify: `internal/api/sources.go:15-22` (SourcesStore interface)
- Modify: `internal/api/pgstore.go` (add new method after line ~205)
- Modify: `internal/api/sources_test.go` (add to mock)

- [ ] **Step 1: Add method to SourcesStore interface**

In `internal/api/sources.go`, add to the `SourcesStore` interface:

```go
ListAllSourceRepos(ctx context.Context) ([]models.SourceRepo, error)
```

And in `internal/models/source.go`, add:

```go
type SourceRepo struct {
	Provider   string `json:"provider"`
	Repository string `json:"repository"`
}
```

- [ ] **Step 2: Implement in PgStore**

In `internal/api/pgstore.go`, add after the `UpdateSourcePollStatus` method:

```go
func (s *PgStore) ListAllSourceRepos(ctx context.Context) ([]models.SourceRepo, error) {
	rows, err := s.pool.Query(ctx, `SELECT DISTINCT provider, repository FROM sources WHERE enabled = true`)
	if err != nil {
		return nil, fmt.Errorf("list all source repos: %w", err)
	}
	defer rows.Close()
	var repos []models.SourceRepo
	for rows.Next() {
		var r models.SourceRepo
		if err := rows.Scan(&r.Provider, &r.Repository); err != nil {
			return nil, fmt.Errorf("scan source repo: %w", err)
		}
		repos = append(repos, r)
	}
	return repos, rows.Err()
}
```

- [ ] **Step 3: Add to mock in sources_test.go**

Add to the `mockSourcesStore` struct:

```go
func (m *mockSourcesStore) ListAllSourceRepos(_ context.Context) ([]models.SourceRepo, error) {
	return nil, nil
}
```

- [ ] **Step 4: Verify compilation**

Run: `go build ./internal/...`
Expected: Success, no errors

- [ ] **Step 5: Commit**

```bash
git add internal/api/sources.go internal/api/pgstore.go internal/api/sources_test.go internal/models/source.go
git commit -m "feat(store): add ListAllSourceRepos for tracked-repo filtering"
```

---

## Task 2: Backend suggestions handler

**Files:**
- Create: `internal/api/suggestions.go`
- Create: `internal/api/suggestions_test.go`

- [ ] **Step 1: Write failing test for Stars endpoint**

Create `internal/api/suggestions_test.go`:

```go
package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
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
	// Inject user into context
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
	// First item should not be tracked
	if got.Data[0].Tracked {
		t.Error("expected org/cool-repo to not be tracked")
	}
	// Second item should be tracked
	if !got.Data[1].Tracked {
		t.Error("expected already/tracked to be tracked")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test -v -run TestSuggestionsStars ./internal/api/...`
Expected: FAIL — `NewSuggestionsHandler` undefined

- [ ] **Step 3: Write failing test for Stars with no user context**

Add to `suggestions_test.go`:

```go
func TestSuggestionsStarsNoUser(t *testing.T) {
	h := NewSuggestionsHandler(http.DefaultClient, &mockSuggestionsStore{}, "", "")

	mux := http.NewServeMux()
	mux.HandleFunc("GET /suggestions/stars", h.Stars)

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/suggestions/stars", nil)
	// No user in context

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
```

- [ ] **Step 4: Write failing test for Repos endpoint**

Add to `suggestions_test.go`:

```go
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
```

- [ ] **Step 5: Write failing test for caching**

Add to `suggestions_test.go`:

```go
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
```

(Add `"sync/atomic"` to imports.)

- [ ] **Step 6: Implement SuggestionsHandler**

Create `internal/api/suggestions.go`:

```go
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

	// Fetch from GitHub (or cache)
	raw, ok := h.getCached(cacheKey)
	if !ok {
		var err error
		// Fetch up to 4 pages (120 stars)
		var allStars []json.RawMessage
		for page := 1; page <= 4; page++ {
			path := fmt.Sprintf("/users/%s/starred?per_page=30&page=%d", login, page)
			pageRaw, fetchErr := h.githubGet(r.Context(), path)
			if fetchErr != nil {
				if page == 1 {
					RespondError(w, r, http.StatusBadGateway, "upstream_error", fetchErr.Error())
					return
				}
				break // partial results OK
			}
			var pageItems []json.RawMessage
			if err = json.Unmarshal(pageRaw, &pageItems); err != nil {
				RespondError(w, r, http.StatusBadGateway, "upstream_error", "failed to parse GitHub response")
				return
			}
			allStars = append(allStars, pageItems...)
			if len(pageItems) < 30 {
				break // last page
			}
		}
		raw, err = json.Marshal(allStars)
		if err != nil {
			RespondError(w, r, http.StatusInternalServerError, "internal_error", "failed to marshal stars")
			return
		}
		h.setCache(cacheKey, raw)
	}

	// Parse raw GitHub response
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

	// Get tracked repos for filtering
	tracked := make(map[string]bool)
	if repos, err := h.store.ListAllSourceRepos(r.Context()); err == nil {
		for _, repo := range repos {
			tracked[repo.Provider+":"+repo.Repository] = true
		}
	}

	// Map to response
	items := make([]SuggestionItem, 0, len(ghStars))
	for _, s := range ghStars {
		items = append(items, SuggestionItem{
			Name:      s.Name,
			FullName:  s.FullName,
			Description: s.Desc,
			Stars:     s.Stars,
			Language:  s.Language,
			URL:       s.URL,
			AvatarURL: s.Owner.AvatarURL,
			Provider:  "github",
			Tracked:   tracked["github:"+s.FullName],
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
```

- [ ] **Step 7: Run all tests**

Run: `go test -v -run TestSuggestions ./internal/api/...`
Expected: All 4 tests PASS

- [ ] **Step 8: Commit**

```bash
git add internal/api/suggestions.go internal/api/suggestions_test.go
git commit -m "feat(api): add suggestions handler for stars and repos"
```

---

## Task 3: Register routes in server.go

**Files:**
- Modify: `internal/api/server.go`

- [ ] **Step 1: Add suggestions routes**

In `internal/api/server.go`, after the discover routes block (around line 136), add:

```go
// Personalized suggestions (auth required — needs github_login from session)
suggestions := NewSuggestionsHandler(deps.HTTPClient, deps.SourcesStore, "", "")
mux.Handle("GET /api/v1/suggestions/stars", chain(http.HandlerFunc(suggestions.Stars)))
mux.Handle("GET /api/v1/suggestions/repos", chain(http.HandlerFunc(suggestions.Repos)))
```

Note: Uses `chain` (auth-required) not `publicChain`. The handlers themselves gracefully handle missing user context (return empty array).

- [ ] **Step 2: Verify compilation and existing tests still pass**

Run: `go build ./cmd/server && go test ./internal/api/...`
Expected: Build succeeds, all tests pass

- [ ] **Step 3: Commit**

```bash
git add internal/api/server.go
git commit -m "feat(api): register suggestions routes on auth chain"
```

---

## Task 4: Frontend types and API client

**Files:**
- Modify: `web/lib/api/types.ts`
- Modify: `web/lib/api/client.ts`

- [ ] **Step 1: Add types**

In `web/lib/api/types.ts`, add:

```typescript
export interface SuggestionItem {
  name: string;
  full_name: string;
  description: string;
  stars: number;
  language?: string;
  url: string;
  avatar_url?: string;
  provider: string;
  tracked: boolean;
}

export interface RepoItem {
  name: string;
  full_name: string;
  description: string;
  language?: string;
  url: string;
  pushed_at: string;
}
```

- [ ] **Step 2: Add API client methods**

In `web/lib/api/client.ts`, add after the `discover` export:

```typescript
export const suggestions = {
  stars: () =>
    request<ApiResponse<SuggestionItem[]>>("/suggestions/stars"),
  repos: () =>
    request<ApiResponse<RepoItem[]>>("/suggestions/repos"),
};
```

Add `SuggestionItem` and `RepoItem` to the imports from `./types`.

- [ ] **Step 3: Commit**

```bash
git add web/lib/api/types.ts web/lib/api/client.ts
git commit -m "feat(web): add suggestions API types and client methods"
```

---

## Task 5: Stars tab component

**Files:**
- Create: `web/components/dashboard/stars-tab.tsx`

- [ ] **Step 1: Create StarsTab component**

Create `web/components/dashboard/stars-tab.tsx`:

```tsx
"use client";

import { useState, useCallback } from "react";
import useSWR, { mutate } from "swr";
import { suggestions, projects, sources } from "@/lib/api/client";
import type { SuggestionItem } from "@/lib/api/types";

export function StarsTab() {
  const [loaded, setLoaded] = useState(false);
  const [trackingIds, setTrackingIds] = useState<Set<string>>(new Set());

  const { data, isLoading, error } = useSWR(
    loaded ? "suggestions-stars" : null,
    () => suggestions.stars(),
    { revalidateOnFocus: false }
  );

  const items = data?.data ?? [];

  const handleTrack = useCallback(async (item: SuggestionItem) => {
    const key = item.full_name;
    setTrackingIds((prev) => new Set(prev).add(key));
    try {
      const projRes = await projects.create({
        name: item.full_name,
        description: item.description,
      });
      const project = projRes.data;
      const sourceRes = await sources.create(project.id, {
        provider: "github",
        repository: item.full_name,
        poll_interval_seconds: 86400,
        enabled: true,
      });
      await sources.poll(sourceRes.data.id);
      await mutate("suggestions-stars");
      await mutate("projects-list");
      await mutate("projects-for-dashboard");
    } catch (err: unknown) {
      const msg = err instanceof Error ? err.message : "Failed to track";
      alert(msg);
    } finally {
      setTrackingIds((prev) => {
        const next = new Set(prev);
        next.delete(key);
        return next;
      });
    }
  }, []);

  if (!loaded) {
    return (
      <div className="flex items-center justify-center py-12">
        <button
          onClick={() => setLoaded(true)}
          className="rounded-lg bg-purple-600 px-6 py-3 text-sm font-semibold text-white hover:bg-purple-700 transition-colors"
        >
          Load your starred repos
        </button>
      </div>
    );
  }

  if (isLoading) {
    return (
      <div className="flex items-center justify-center py-12">
        <div className="text-sm text-zinc-400">Loading your starred repos...</div>
      </div>
    );
  }

  if (error) {
    return (
      <div className="flex items-center justify-center py-12">
        <div className="text-sm text-red-400">Failed to load stars. Try again later.</div>
      </div>
    );
  }

  if (items.length === 0) {
    return (
      <div className="flex items-center justify-center py-12">
        <div className="text-sm text-zinc-400">
          You haven&apos;t starred any public repos on GitHub yet.
        </div>
      </div>
    );
  }

  return (
    <div className="grid grid-cols-1 gap-3 sm:grid-cols-2 lg:grid-cols-3">
      {items.map((item) => (
        <div
          key={item.full_name}
          className={`rounded-lg border p-3 ${
            item.tracked
              ? "border-zinc-700 bg-zinc-800/50 opacity-60"
              : "border-zinc-700 bg-zinc-800"
          }`}
        >
          <div className="flex items-start justify-between gap-2 mb-2">
            <span className="text-sm font-semibold text-zinc-200 truncate">
              {item.full_name}
            </span>
            {item.tracked ? (
              <span className="shrink-0 text-xs text-green-400">✓ Tracked</span>
            ) : (
              <button
                onClick={() => handleTrack(item)}
                disabled={trackingIds.has(item.full_name)}
                className="shrink-0 rounded bg-purple-600 px-2.5 py-1 text-xs font-medium text-white hover:bg-purple-700 disabled:opacity-50 transition-colors"
              >
                {trackingIds.has(item.full_name) ? "Tracking..." : "Track"}
              </button>
            )}
          </div>
          {item.description && (
            <p className="text-xs text-zinc-400 mb-2 line-clamp-2">
              {item.description}
            </p>
          )}
          <div className="flex items-center gap-3 text-xs text-zinc-500">
            <span>⭐ {item.stars.toLocaleString()}</span>
            {item.language && <span>● {item.language}</span>}
          </div>
        </div>
      ))}
    </div>
  );
}
```

- [ ] **Step 2: Verify no TypeScript errors**

Run: `cd web && npx tsc --noEmit`
Expected: No errors (or only pre-existing errors)

- [ ] **Step 3: Commit**

```bash
git add web/components/dashboard/stars-tab.tsx
git commit -m "feat(web): add StarsTab component for GitHub starred repos"
```

---

## Task 6: Dependencies tab component

**Files:**
- Create: `web/components/dashboard/deps-tab.tsx`

- [ ] **Step 1: Create DepsTab component**

Create `web/components/dashboard/deps-tab.tsx`:

```tsx
"use client";

import { useState, useCallback } from "react";
import useSWR from "swr";
import { suggestions, onboard } from "@/lib/api/client";
import type { RepoItem, OnboardScan, ScannedDependency, OnboardSelection } from "@/lib/api/types";

type Step = "pick" | "scanning" | "results" | "applied";

interface ScanState {
  repoName: string;
  scanId: string;
  status: string;
  results?: ScannedDependency[];
  error?: string;
}

export function DepsTab() {
  const [loaded, setLoaded] = useState(false);
  const [step, setStep] = useState<Step>("pick");
  const [selectedRepos, setSelectedRepos] = useState<Set<string>>(new Set());
  const [scans, setScans] = useState<ScanState[]>([]);
  const [appliedCount, setAppliedCount] = useState(0);

  const { data, isLoading, error } = useSWR(
    loaded ? "suggestions-repos" : null,
    () => suggestions.repos(),
    { revalidateOnFocus: false }
  );

  const repos = data?.data ?? [];

  const toggleRepo = useCallback((fullName: string) => {
    setSelectedRepos((prev) => {
      const next = new Set(prev);
      if (next.has(fullName)) next.delete(fullName);
      else next.add(fullName);
      return next;
    });
  }, []);

  const handleScan = useCallback(async () => {
    const repoNames = Array.from(selectedRepos);
    setStep("scanning");

    const scanStates: ScanState[] = repoNames.map((name) => ({
      repoName: name,
      scanId: "",
      status: "queued",
    }));
    setScans([...scanStates]);

    // Start scans (max 5 concurrent)
    const batchSize = 5;
    for (let i = 0; i < repoNames.length; i += batchSize) {
      const batch = repoNames.slice(i, i + batchSize);
      await Promise.all(
        batch.map(async (repoName, batchIdx) => {
          const idx = i + batchIdx;
          try {
            const repoUrl = `https://github.com/${repoName}`;
            const scanRes = await onboard.scan(repoUrl);
            const scanId = scanRes.data.id;
            scanStates[idx] = { ...scanStates[idx], scanId, status: "processing" };
            setScans([...scanStates]);

            // Poll until complete
            let scan: OnboardScan;
            do {
              await new Promise((r) => setTimeout(r, 2000));
              const pollRes = await onboard.getScan(scanId);
              scan = pollRes.data;
            } while (scan.status === "pending" || scan.status === "processing");

            if (scan.status === "completed" && scan.results) {
              scanStates[idx] = { ...scanStates[idx], status: "completed", results: scan.results };
            } else {
              scanStates[idx] = { ...scanStates[idx], status: "failed", error: scan.error || "Unknown error" };
            }
          } catch (err: unknown) {
            const msg = err instanceof Error ? err.message : "Scan failed";
            scanStates[idx] = { ...scanStates[idx], status: "failed", error: msg };
          }
          setScans([...scanStates]);
        })
      );
    }

    setStep("results");
  }, [selectedRepos]);

  const handleApply = useCallback(
    async (selections: { scanId: string; items: OnboardSelection[] }[]) => {
      let total = 0;
      for (const { scanId, items } of selections) {
        if (items.length === 0) continue;
        try {
          const res = await onboard.apply(scanId, items);
          total += res.data.created_sources.length;
        } catch {
          // continue with other scans
        }
      }
      setAppliedCount(total);
      setStep("applied");
    },
    []
  );

  if (!loaded) {
    return (
      <div className="flex items-center justify-center py-12">
        <button
          onClick={() => setLoaded(true)}
          className="rounded-lg bg-purple-600 px-6 py-3 text-sm font-semibold text-white hover:bg-purple-700 transition-colors"
        >
          Load your repos
        </button>
      </div>
    );
  }

  if (isLoading) {
    return (
      <div className="flex items-center justify-center py-12">
        <div className="text-sm text-zinc-400">Loading your repos...</div>
      </div>
    );
  }

  if (error) {
    return (
      <div className="flex items-center justify-center py-12">
        <div className="text-sm text-red-400">Failed to load repos. Try again later.</div>
      </div>
    );
  }

  if (step === "pick") {
    return <RepoPicker repos={repos} selected={selectedRepos} onToggle={toggleRepo} onScan={handleScan} />;
  }

  if (step === "scanning") {
    return <ScanProgress scans={scans} />;
  }

  if (step === "results") {
    return <ScanResults scans={scans.filter((s) => s.status === "completed")} onApply={handleApply} />;
  }

  return (
    <div className="flex flex-col items-center justify-center py-12 gap-3">
      <div className="text-sm text-green-400">
        Successfully started tracking {appliedCount} dependencies.
      </div>
      <button
        onClick={() => {
          setStep("pick");
          setScans([]);
          setSelectedRepos(new Set());
        }}
        className="text-xs text-zinc-400 hover:text-zinc-200 underline"
      >
        Scan more repos
      </button>
    </div>
  );
}

function RepoPicker({
  repos,
  selected,
  onToggle,
  onScan,
}: {
  repos: RepoItem[];
  selected: Set<string>;
  onToggle: (name: string) => void;
  onScan: () => void;
}) {
  if (repos.length === 0) {
    return (
      <div className="flex items-center justify-center py-12">
        <div className="text-sm text-zinc-400">No public repos found.</div>
      </div>
    );
  }

  return (
    <div>
      <p className="text-sm text-zinc-400 mb-3">Select repos to scan for dependencies:</p>
      <div className="flex flex-col gap-2 max-h-80 overflow-y-auto">
        {repos.map((repo) => (
          <label
            key={repo.full_name}
            className="flex items-center gap-3 rounded-lg border border-zinc-700 bg-zinc-800 p-3 cursor-pointer hover:border-zinc-600 transition-colors"
          >
            <input
              type="checkbox"
              checked={selected.has(repo.full_name)}
              onChange={() => onToggle(repo.full_name)}
              className="accent-purple-600"
            />
            <div className="flex-1 min-w-0">
              <div className="text-sm font-semibold text-zinc-200 truncate">{repo.full_name}</div>
              <div className="text-xs text-zinc-500">
                {repo.pushed_at && `Pushed ${new Date(repo.pushed_at).toLocaleDateString()}`}
                {repo.language && ` · ${repo.language}`}
              </div>
            </div>
          </label>
        ))}
      </div>
      <div className="flex justify-end mt-3">
        <button
          onClick={onScan}
          disabled={selected.size === 0}
          className="rounded-lg bg-purple-600 px-5 py-2 text-sm font-semibold text-white hover:bg-purple-700 disabled:opacity-50 transition-colors"
        >
          Scan Selected ({selected.size})
        </button>
      </div>
    </div>
  );
}

function ScanProgress({ scans }: { scans: ScanState[] }) {
  return (
    <div className="space-y-2 py-4">
      <p className="text-sm text-zinc-400 mb-3">Scanning repositories for dependencies...</p>
      {scans.map((scan) => (
        <div key={scan.repoName} className="flex items-center gap-3 text-sm">
          <span className="w-5 text-center">
            {scan.status === "completed" && "✓"}
            {scan.status === "failed" && "✗"}
            {(scan.status === "queued" || scan.status === "processing") && "⏳"}
          </span>
          <span className={scan.status === "failed" ? "text-red-400" : "text-zinc-300"}>
            {scan.repoName}
          </span>
          {scan.status === "failed" && scan.error && (
            <span className="text-xs text-red-400/70">({scan.error})</span>
          )}
        </div>
      ))}
    </div>
  );
}

function ScanResults({
  scans,
  onApply,
}: {
  scans: ScanState[];
  onApply: (selections: { scanId: string; items: OnboardSelection[] }[]) => void;
}) {
  const [selected, setSelected] = useState<Set<string>>(() => {
    const all = new Set<string>();
    scans.forEach((scan) =>
      scan.results?.forEach((dep) => all.add(`${scan.scanId}:${dep.name}`))
    );
    return all;
  });

  const toggle = (key: string) => {
    setSelected((prev) => {
      const next = new Set(prev);
      if (next.has(key)) next.delete(key);
      else next.add(key);
      return next;
    });
  };

  const handleApply = () => {
    const selections = scans.map((scan) => ({
      scanId: scan.scanId,
      items: (scan.results ?? [])
        .filter((dep) => selected.has(`${scan.scanId}:${dep.name}`))
        .map((dep) => ({
          dep_name: dep.name,
          upstream_repo: dep.upstream_repo,
          provider: dep.provider,
          new_project_name: dep.upstream_repo || dep.name,
        })),
    }));
    onApply(selections);
  };

  const totalSelected = selected.size;

  if (scans.length === 0 || scans.every((s) => !s.results?.length)) {
    return (
      <div className="flex items-center justify-center py-12">
        <div className="text-sm text-zinc-400">No dependencies found in the selected repos.</div>
      </div>
    );
  }

  return (
    <div>
      {scans.map((scan) => {
        if (!scan.results?.length) return null;
        return (
          <div key={scan.scanId} className="mb-4">
            <h4 className="text-sm font-semibold text-zinc-300 mb-2">
              From {scan.repoName}
            </h4>
            <div className="flex flex-col gap-1">
              {scan.results.map((dep) => {
                const key = `${scan.scanId}:${dep.name}`;
                return (
                  <label
                    key={key}
                    className="flex items-center gap-3 rounded border border-zinc-700 bg-zinc-800/50 px-3 py-2 cursor-pointer text-sm"
                  >
                    <input
                      type="checkbox"
                      checked={selected.has(key)}
                      onChange={() => toggle(key)}
                      className="accent-purple-600"
                    />
                    <span className="text-zinc-200">{dep.name}</span>
                    {dep.version && (
                      <span className="text-xs text-zinc-500">{dep.version}</span>
                    )}
                    <span className="ml-auto text-xs text-zinc-500">{dep.ecosystem}</span>
                  </label>
                );
              })}
            </div>
          </div>
        );
      })}
      <div className="flex justify-end mt-3">
        <button
          onClick={handleApply}
          disabled={totalSelected === 0}
          className="rounded-lg bg-purple-600 px-5 py-2 text-sm font-semibold text-white hover:bg-purple-700 disabled:opacity-50 transition-colors"
        >
          Track Selected ({totalSelected})
        </button>
      </div>
    </div>
  );
}
```

- [ ] **Step 2: Verify no TypeScript errors**

Run: `cd web && npx tsc --noEmit`
Expected: No errors (or only pre-existing errors)

- [ ] **Step 3: Commit**

```bash
git add web/components/dashboard/deps-tab.tsx
git commit -m "feat(web): add DepsTab component for dependency scanning"
```

---

## Task 7: SuggestionsSection and dashboard integration

**Files:**
- Create: `web/components/dashboard/suggestions-section.tsx`
- Modify: `web/app/page.tsx`

- [ ] **Step 1: Create SuggestionsSection component**

Create `web/components/dashboard/suggestions-section.tsx`:

```tsx
"use client";

import { useState } from "react";
import { StarsTab } from "./stars-tab";
import { DepsTab } from "./deps-tab";

type Tab = "stars" | "deps";

export function SuggestionsSection() {
  const [tab, setTab] = useState<Tab>("stars");

  return (
    <div className="rounded-xl border border-zinc-800 bg-zinc-900 p-4">
      <div className="flex gap-0 mb-4 border-b border-zinc-700">
        <button
          onClick={() => setTab("stars")}
          className={`px-5 py-2 text-sm font-medium transition-colors ${
            tab === "stars"
              ? "border-b-2 border-purple-500 text-purple-400"
              : "text-zinc-400 hover:text-zinc-200"
          }`}
        >
          ⭐ Your Stars
        </button>
        <button
          onClick={() => setTab("deps")}
          className={`px-5 py-2 text-sm font-medium transition-colors ${
            tab === "deps"
              ? "border-b-2 border-purple-500 text-purple-400"
              : "text-zinc-400 hover:text-zinc-200"
          }`}
        >
          📦 Your Dependencies
        </button>
      </div>
      {tab === "stars" ? <StarsTab /> : <DepsTab />}
    </div>
  );
}
```

- [ ] **Step 2: Update dashboard page to conditionally render**

In `web/app/page.tsx`, replace the `<DiscoverySection />` usage with conditional rendering:

```tsx
import { SuggestionsSection } from "@/components/dashboard/suggestions-section";
import { DiscoverySection } from "@/components/dashboard/discovery-section";
import { useAuth } from "@/lib/auth/context";

// Inside the component:
const { user } = useAuth();

// In the JSX, replace <DiscoverySection /> with:
{user?.github_login && user.github_login !== "dev" ? (
  <SuggestionsSection />
) : (
  <DiscoverySection />
)}
```

- [ ] **Step 3: Verify no TypeScript errors**

Run: `cd web && npx tsc --noEmit`
Expected: No errors (or only pre-existing errors)

- [ ] **Step 4: Commit**

```bash
git add web/components/dashboard/suggestions-section.tsx web/app/page.tsx
git commit -m "feat(web): integrate SuggestionsSection on dashboard for GitHub users"
```

---

## Task 8: Manual verification

- [ ] **Step 1: Run full backend test suite**

Run: `go test ./...`
Expected: All tests pass

- [ ] **Step 2: Build backend**

Run: `go build -o changelogue ./cmd/server`
Expected: Successful build

- [ ] **Step 3: Run frontend type check**

Run: `cd web && npx tsc --noEmit`
Expected: No new errors

- [ ] **Step 4: Start dev environment and manually verify**

Run: `make dev` in one terminal, `make frontend-dev` in another.

Verify:
1. Login with GitHub
2. Dashboard shows "Your Stars" / "Your Dependencies" tabs instead of trending carousel
3. Click "Load your starred repos" — starred repos appear with Track buttons
4. Click "Load your repos" — repo list appears with checkboxes
5. Select repos and click "Scan Selected" — scanning progress shows
6. Results appear grouped by repo — can select and track dependencies
7. Logout → dashboard shows trending carousel again
