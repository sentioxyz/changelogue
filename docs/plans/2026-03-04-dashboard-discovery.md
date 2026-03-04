# Dashboard Discovery Section Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Add a discovery section to the dashboard that shows trending GitHub repos and popular Docker Hub images, with one-click project tracking.

**Architecture:** Two new backend GET endpoints (`/discover/github`, `/discover/dockerhub`) proxy external APIs with in-memory caching. A new React component renders the results as a horizontally scrollable card row with tabs, mounted at the top of the dashboard page.

**Tech Stack:** Go (backend handler + HTTP client), React + SWR + Tailwind (frontend), GitHub Search API, Docker Hub Search API.

---

### Task 1: Backend — DiscoverItem model and handler scaffold

**Files:**
- Create: `internal/api/discover.go`
- Test: `internal/api/discover_test.go`

**Step 1: Write the failing test for GitHub discovery**

```go
// internal/api/discover_test.go
package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestDiscoverGitHub(t *testing.T) {
	// Mock GitHub Search API
	ghServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/search/repositories" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"items": []map[string]any{
				{
					"full_name":   "kubernetes/kubernetes",
					"name":        "kubernetes",
					"description": "Production-Grade Container Scheduling",
					"stargazers_count": 112000,
					"language":    "Go",
					"html_url":    "https://github.com/kubernetes/kubernetes",
					"owner": map[string]any{
						"avatar_url": "https://avatars.githubusercontent.com/u/13629408",
					},
				},
			},
		})
	}))
	defer ghServer.Close()

	h := NewDiscoverHandler(&http.Client{}, ghServer.URL, "")
	mux := http.NewServeMux()
	mux.HandleFunc("GET /discover/github", h.GitHub)

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/discover/github", nil)
	mux.ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var got struct {
		Data []DiscoverItem `json:"data"`
	}
	if err := json.NewDecoder(w.Body).Decode(&got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(got.Data) != 1 {
		t.Fatalf("expected 1 item, got %d", len(got.Data))
	}
	item := got.Data[0]
	if item.FullName != "kubernetes/kubernetes" {
		t.Fatalf("expected full_name=kubernetes/kubernetes, got %s", item.FullName)
	}
	if item.Stars != 112000 {
		t.Fatalf("expected stars=112000, got %d", item.Stars)
	}
	if item.Provider != "github" {
		t.Fatalf("expected provider=github, got %s", item.Provider)
	}
}
```

**Step 2: Run test to verify it fails**

Run: `go test -v -run TestDiscoverGitHub ./internal/api/...`
Expected: FAIL — `NewDiscoverHandler` and `DiscoverItem` undefined

**Step 3: Write the handler implementation**

```go
// internal/api/discover.go
package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"
)

// DiscoverItem represents a discoverable repository from an external registry.
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

// DiscoverHandler handles discovery endpoints for trending/popular repositories.
type DiscoverHandler struct {
	client       *http.Client
	githubURL    string
	dockerHubURL string

	mu    sync.Mutex
	cache map[string]cachedResult
}

const discoverCacheTTL = 1 * time.Hour

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
	if !ok || time.Since(c.fetchedAt) > discoverCacheTTL {
		return nil, false
	}
	return c.items, true
}

func (h *DiscoverHandler) setCache(key string, items []DiscoverItem) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.cache[key] = cachedResult{items: items, fetchedAt: time.Now()}
}

// GitHub handles GET /discover/github — returns trending GitHub repositories.
func (h *DiscoverHandler) GitHub(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query().Get("q")
	language := r.URL.Query().Get("language")

	cacheKey := fmt.Sprintf("github:%s:%s", q, language)
	if items, ok := h.getCached(cacheKey); ok {
		RespondJSON(w, r, http.StatusOK, items)
		return
	}

	query := "stars:>1000"
	if q != "" {
		query = q + " " + query
	}
	if language != "" {
		query += " language:" + language
	}

	url := fmt.Sprintf("%s/search/repositories?q=%s&sort=stars&order=desc&per_page=25",
		h.githubURL, query)

	req, err := http.NewRequestWithContext(r.Context(), http.MethodGet, url, nil)
	if err != nil {
		RespondError(w, r, http.StatusInternalServerError, "internal_error", "Failed to build request")
		return
	}
	req.Header.Set("Accept", "application/vnd.github+json")

	resp, err := h.client.Do(req)
	if err != nil {
		RespondError(w, r, http.StatusBadGateway, "upstream_error", "Failed to reach GitHub")
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		RespondError(w, r, http.StatusBadGateway, "upstream_error",
			fmt.Sprintf("GitHub returned status %d", resp.StatusCode))
		return
	}

	var body struct {
		Items []struct {
			FullName    string `json:"full_name"`
			Name        string `json:"name"`
			Description string `json:"description"`
			Stars       int    `json:"stargazers_count"`
			Language    string `json:"language"`
			HTMLURL     string `json:"html_url"`
			Owner       struct {
				AvatarURL string `json:"avatar_url"`
			} `json:"owner"`
		} `json:"items"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		RespondError(w, r, http.StatusBadGateway, "upstream_error", "Failed to parse GitHub response")
		return
	}

	items := make([]DiscoverItem, 0, len(body.Items))
	for _, repo := range body.Items {
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

// DockerHub handles GET /discover/dockerhub — returns popular Docker Hub repositories.
func (h *DiscoverHandler) DockerHub(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query().Get("q")
	if q == "" {
		q = "nginx" // Docker Hub search requires a query
	}

	cacheKey := fmt.Sprintf("dockerhub:%s", q)
	if items, ok := h.getCached(cacheKey); ok {
		RespondJSON(w, r, http.StatusOK, items)
		return
	}

	url := fmt.Sprintf("%s/v2/search/repositories/?query=%s&page_size=25",
		h.dockerHubURL, q)

	req, err := http.NewRequestWithContext(r.Context(), http.MethodGet, url, nil)
	if err != nil {
		RespondError(w, r, http.StatusInternalServerError, "internal_error", "Failed to build request")
		return
	}

	resp, err := h.client.Do(req)
	if err != nil {
		RespondError(w, r, http.StatusBadGateway, "upstream_error", "Failed to reach Docker Hub")
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		RespondError(w, r, http.StatusBadGateway, "upstream_error",
			fmt.Sprintf("Docker Hub returned status %d", resp.StatusCode))
		return
	}

	var body struct {
		Results []struct {
			RepoName    string `json:"repo_name"`
			ShortDesc   string `json:"short_description"`
			StarCount   int    `json:"star_count"`
			IsOfficial  bool   `json:"is_official"`
		} `json:"results"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		RespondError(w, r, http.StatusBadGateway, "upstream_error", "Failed to parse Docker Hub response")
		return
	}

	items := make([]DiscoverItem, 0, len(body.Results))
	for _, repo := range body.Results {
		fullName := "library/" + repo.RepoName
		if repo.IsOfficial {
			fullName = "library/" + repo.RepoName
		}
		items = append(items, DiscoverItem{
			Name:        repo.RepoName,
			FullName:    fullName,
			Description: repo.ShortDesc,
			Stars:       repo.StarCount,
			URL:         fmt.Sprintf("https://hub.docker.com/_/%s", repo.RepoName),
			Provider:    "dockerhub",
		})
	}

	h.setCache(cacheKey, items)
	RespondJSON(w, r, http.StatusOK, items)
}
```

**Step 4: Run test to verify it passes**

Run: `go test -v -run TestDiscoverGitHub ./internal/api/...`
Expected: PASS

**Step 5: Commit**

```bash
git add internal/api/discover.go internal/api/discover_test.go
git commit -m "feat(api): add discover handler for GitHub trending repos"
```

---

### Task 2: Backend — Docker Hub discover test + upstream error test

**Files:**
- Modify: `internal/api/discover_test.go`

**Step 1: Add tests for Docker Hub and error handling**

Append to `internal/api/discover_test.go`:

```go
func TestDiscoverDockerHub(t *testing.T) {
	dhServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v2/search/repositories/" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"results": []map[string]any{
				{
					"repo_name":         "nginx",
					"short_description": "Official NGINX image",
					"star_count":        19000,
					"is_official":       true,
				},
			},
		})
	}))
	defer dhServer.Close()

	h := NewDiscoverHandler(&http.Client{}, "", dhServer.URL)
	mux := http.NewServeMux()
	mux.HandleFunc("GET /discover/dockerhub", h.DockerHub)

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/discover/dockerhub?q=nginx", nil)
	mux.ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var got struct {
		Data []DiscoverItem `json:"data"`
	}
	if err := json.NewDecoder(w.Body).Decode(&got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(got.Data) != 1 {
		t.Fatalf("expected 1 item, got %d", len(got.Data))
	}
	if got.Data[0].Name != "nginx" {
		t.Fatalf("expected name=nginx, got %s", got.Data[0].Name)
	}
	if got.Data[0].Provider != "dockerhub" {
		t.Fatalf("expected provider=dockerhub, got %s", got.Data[0].Provider)
	}
}

func TestDiscoverGitHubUpstreamError(t *testing.T) {
	ghServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer ghServer.Close()

	h := NewDiscoverHandler(&http.Client{}, ghServer.URL, "")
	mux := http.NewServeMux()
	mux.HandleFunc("GET /discover/github", h.GitHub)

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/discover/github", nil)
	mux.ServeHTTP(w, r)

	if w.Code != http.StatusBadGateway {
		t.Fatalf("expected 502, got %d", w.Code)
	}
}

func TestDiscoverCaching(t *testing.T) {
	callCount := 0
	ghServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{"items": []map[string]any{}})
	}))
	defer ghServer.Close()

	h := NewDiscoverHandler(&http.Client{}, ghServer.URL, "")
	mux := http.NewServeMux()
	mux.HandleFunc("GET /discover/github", h.GitHub)

	// First request — cache miss
	w1 := httptest.NewRecorder()
	r1 := httptest.NewRequest(http.MethodGet, "/discover/github", nil)
	mux.ServeHTTP(w1, r1)

	// Second request — cache hit
	w2 := httptest.NewRecorder()
	r2 := httptest.NewRequest(http.MethodGet, "/discover/github", nil)
	mux.ServeHTTP(w2, r2)

	if callCount != 1 {
		t.Fatalf("expected 1 upstream call (cached), got %d", callCount)
	}
}
```

**Step 2: Run all discover tests**

Run: `go test -v -run TestDiscover ./internal/api/...`
Expected: PASS (all 4 tests)

**Step 3: Commit**

```bash
git add internal/api/discover_test.go
git commit -m "test(api): add Docker Hub, upstream error, and caching tests for discover"
```

---

### Task 3: Backend — Register discover routes

**Files:**
- Modify: `internal/api/server.go` (line 108, after providers)

**Step 1: Add routes to server.go**

After line 108 (`mux.Handle("GET /api/v1/providers", ...)`), add:

```go
	// Discovery (public — no auth, proxies external APIs)
	discover := NewDiscoverHandler(deps.HTTPClient, "", "")
	mux.Handle("GET /api/v1/discover/github", publicChain(http.HandlerFunc(discover.GitHub)))
	mux.Handle("GET /api/v1/discover/dockerhub", publicChain(http.HandlerFunc(discover.DockerHub)))
```

**Step 2: Run vet and existing tests**

Run: `go vet ./internal/api/... && go test ./internal/api/...`
Expected: PASS

**Step 3: Commit**

```bash
git add internal/api/server.go
git commit -m "feat(api): register discover routes on public chain"
```

---

### Task 4: Frontend — Add DiscoverItem type and API client methods

**Files:**
- Modify: `web/lib/api/types.ts` (after line 218, TrendData)
- Modify: `web/lib/api/client.ts` (after line 211, system namespace)

**Step 1: Add DiscoverItem type to types.ts**

After the `TrendData` interface (line 218), add:

```typescript
// --- Discovery Types ---

export interface DiscoverItem {
  name: string;
  full_name: string;
  description: string;
  stars: number;
  language?: string;
  url: string;
  avatar_url?: string;
  provider: "github" | "dockerhub";
}
```

**Step 2: Add discover namespace to client.ts**

Add `DiscoverItem` to the import in `client.ts` (line 2), then after the `system` namespace (line 211), add:

```typescript
// --- Discovery ---

export const discover = {
  github: (params?: { q?: string; language?: string }) => {
    const search = new URLSearchParams();
    if (params?.q) search.set("q", params.q);
    if (params?.language) search.set("language", params.language);
    const qs = search.toString();
    return request<ApiResponse<DiscoverItem[]>>(`/discover/github${qs ? `?${qs}` : ""}`);
  },
  dockerhub: (params?: { q?: string }) => {
    const search = new URLSearchParams();
    if (params?.q) search.set("q", params.q);
    const qs = search.toString();
    return request<ApiResponse<DiscoverItem[]>>(`/discover/dockerhub${qs ? `?${qs}` : ""}`);
  },
};
```

**Step 3: Verify frontend compiles**

Run: `cd web && npx tsc --noEmit`
Expected: No errors

**Step 4: Commit**

```bash
git add web/lib/api/types.ts web/lib/api/client.ts
git commit -m "feat(web): add discover API client and DiscoverItem type"
```

---

### Task 5: Frontend — Create DiscoverySection component

**Files:**
- Create: `web/components/dashboard/discovery-section.tsx`

**Step 1: Create the component**

```tsx
// web/components/dashboard/discovery-section.tsx
"use client";

import { useState } from "react";
import useSWR from "swr";
import { Star, Plus, Check, Loader2, ChevronLeft, ChevronRight } from "lucide-react";
import { FaGithub, FaDocker } from "react-icons/fa";
import { discover, projects as projectsApi, sources, type DiscoverItem } from "@/lib/api/client";

// NOTE: DiscoverItem is exported from types.ts but we import via client.ts re-export.
// If that doesn't work, import directly from @/lib/api/types.

type Tab = "github" | "dockerhub";

export function DiscoverySection() {
  const [tab, setTab] = useState<Tab>("github");
  const [trackingId, setTrackingId] = useState<string | null>(null);
  const [trackedNames, setTrackedNames] = useState<Set<string>>(new Set());

  const { data: ghData, isLoading: ghLoading } = useSWR(
    "discover-github",
    () => discover.github(),
    { revalidateOnFocus: false }
  );

  const { data: dhData, isLoading: dhLoading } = useSWR(
    "discover-dockerhub",
    () => discover.dockerhub(),
    { revalidateOnFocus: false }
  );

  // Fetch existing projects to detect already-tracked repos
  const { data: existingProjects, mutate: mutateProjects } = useSWR(
    "projects-all",
    () => projectsApi.list(1, 100),
    { revalidateOnFocus: false }
  );

  const trackedRepos = new Set<string>([
    ...trackedNames,
    ...(existingProjects?.data?.flatMap((p) =>
      // We don't have sources inline on project list, so match by project name
      [p.name]
    ) ?? []),
  ]);

  const items = tab === "github" ? ghData?.data : dhData?.data;
  const isLoading = tab === "github" ? ghLoading : dhLoading;

  async function handleTrack(item: DiscoverItem) {
    setTrackingId(item.full_name);
    try {
      // 1. Create project
      const projectRes = await projectsApi.create({
        name: item.full_name,
        description: item.description,
      });
      const projectId = projectRes.data.id;

      // 2. Create source
      const sourceRes = await sources.create(projectId, {
        provider: item.provider,
        repository: item.full_name,
        poll_interval_seconds: 86400, // daily
        enabled: true,
      });

      // 3. Trigger initial poll
      await sources.poll(sourceRes.data.id);

      // Mark as tracked
      setTrackedNames((prev) => new Set(prev).add(item.full_name));
      mutateProjects();
    } catch (err: any) {
      alert(err.message || "Failed to track project");
    } finally {
      setTrackingId(null);
    }
  }

  const isTracked = (item: DiscoverItem) =>
    trackedRepos.has(item.full_name) || trackedRepos.has(item.name);

  return (
    <div
      className="rounded-lg bg-white p-5"
      style={{ border: "1px solid #e8e8e5" }}
    >
      {/* Header */}
      <div className="mb-4 flex items-center justify-between">
        <div>
          <h2
            style={{
              fontFamily: "var(--font-fraunces)",
              fontSize: "16px",
              fontWeight: 600,
              color: "#111113",
            }}
          >
            Discover Projects
          </h2>
          <p
            className="mt-0.5"
            style={{
              fontFamily: "var(--font-dm-sans)",
              fontSize: "12px",
              color: "#6b7280",
            }}
          >
            Track popular repositories with one click
          </p>
        </div>

        {/* Tabs */}
        <div
          className="flex gap-1 rounded-lg p-1"
          style={{ backgroundColor: "#f5f5f3" }}
        >
          <button
            onClick={() => setTab("github")}
            className="flex items-center gap-1.5 rounded-md px-3 py-1.5 text-xs font-medium transition-colors"
            style={{
              fontFamily: "var(--font-dm-sans)",
              backgroundColor: tab === "github" ? "#fff" : "transparent",
              color: tab === "github" ? "#111113" : "#6b7280",
              boxShadow: tab === "github" ? "0 1px 2px rgba(0,0,0,0.05)" : "none",
            }}
          >
            <FaGithub className="h-3.5 w-3.5" />
            GitHub
          </button>
          <button
            onClick={() => setTab("dockerhub")}
            className="flex items-center gap-1.5 rounded-md px-3 py-1.5 text-xs font-medium transition-colors"
            style={{
              fontFamily: "var(--font-dm-sans)",
              backgroundColor: tab === "dockerhub" ? "#fff" : "transparent",
              color: tab === "dockerhub" ? "#111113" : "#6b7280",
              boxShadow: tab === "dockerhub" ? "0 1px 2px rgba(0,0,0,0.05)" : "none",
            }}
          >
            <FaDocker className="h-3.5 w-3.5" />
            Docker Hub
          </button>
        </div>
      </div>

      {/* Cards */}
      {isLoading ? (
        <div className="flex gap-3 overflow-hidden">
          {Array.from({ length: 5 }).map((_, i) => (
            <div
              key={i}
              className="h-[120px] w-[220px] flex-shrink-0 animate-pulse rounded-lg"
              style={{ backgroundColor: "#f5f5f3" }}
            />
          ))}
        </div>
      ) : (
        <div className="flex gap-3 overflow-x-auto pb-2" style={{ scrollbarWidth: "thin" }}>
          {items?.map((item) => {
            const tracked = isTracked(item);
            const isTracking = trackingId === item.full_name;

            return (
              <div
                key={item.full_name}
                className="flex w-[220px] flex-shrink-0 flex-col justify-between rounded-lg p-3"
                style={{ border: "1px solid #e8e8e5", minHeight: "120px" }}
              >
                <div>
                  <div className="flex items-start justify-between gap-2">
                    <p
                      className="truncate text-sm font-medium"
                      style={{
                        fontFamily: "var(--font-dm-sans)",
                        color: "#111113",
                      }}
                      title={item.full_name}
                    >
                      {item.full_name}
                    </p>
                  </div>
                  <p
                    className="mt-1 line-clamp-2 text-xs"
                    style={{
                      fontFamily: "var(--font-dm-sans)",
                      color: "#6b7280",
                    }}
                  >
                    {item.description || "No description"}
                  </p>
                </div>

                <div className="mt-2 flex items-center justify-between">
                  <div className="flex items-center gap-2">
                    <span
                      className="flex items-center gap-1 text-xs"
                      style={{ color: "#6b7280", fontFamily: "var(--font-dm-sans)" }}
                    >
                      <Star className="h-3 w-3" style={{ color: "#e8601a" }} />
                      {item.stars >= 1000
                        ? `${(item.stars / 1000).toFixed(item.stars >= 10000 ? 0 : 1)}k`
                        : item.stars}
                    </span>
                    {item.language && (
                      <span
                        className="text-xs"
                        style={{ color: "#9ca3af", fontFamily: "var(--font-dm-sans)" }}
                      >
                        {item.language}
                      </span>
                    )}
                  </div>

                  {tracked ? (
                    <span
                      className="flex items-center gap-1 text-xs"
                      style={{ color: "#16a34a", fontFamily: "var(--font-dm-sans)" }}
                    >
                      <Check className="h-3 w-3" />
                      Tracked
                    </span>
                  ) : (
                    <button
                      onClick={() => handleTrack(item)}
                      disabled={isTracking}
                      className="flex items-center gap-1 rounded-md px-2 py-1 text-xs font-medium text-white transition-colors hover:opacity-90 disabled:opacity-50"
                      style={{
                        fontFamily: "var(--font-dm-sans)",
                        backgroundColor: "#e8601a",
                      }}
                    >
                      {isTracking ? (
                        <Loader2 className="h-3 w-3 animate-spin" />
                      ) : (
                        <Plus className="h-3 w-3" />
                      )}
                      Track
                    </button>
                  )}
                </div>
              </div>
            );
          })}

          {items?.length === 0 && (
            <p
              className="py-8 text-center text-sm w-full"
              style={{ color: "#6b7280", fontFamily: "var(--font-dm-sans)" }}
            >
              No repositories found.
            </p>
          )}
        </div>
      )}
    </div>
  );
}
```

**Step 2: Verify it compiles**

Run: `cd web && npx tsc --noEmit`
Expected: No errors (may need to adjust imports — see note in code)

**Step 3: Commit**

```bash
git add web/components/dashboard/discovery-section.tsx
git commit -m "feat(web): create DiscoverySection component with tabs and one-click tracking"
```

---

### Task 6: Frontend — Mount DiscoverySection on dashboard

**Files:**
- Modify: `web/app/page.tsx` (lines 8-9, 66)

**Step 1: Import and mount the component**

Add import after line 9:
```typescript
import { DiscoverySection } from "@/components/dashboard/discovery-section";
```

Replace lines 66-87 (the conditional rendering) with:

```tsx
      <DiscoverySection />

      {hasProjects ? (
        <>
          <StatsCards />
          <div className="grid gap-4 lg:grid-cols-2">
            <ReleaseTrendChart />
            <UnifiedFeed />
          </div>
        </>
      ) : isLoading ? (
        <div
          className="py-16 text-center"
          style={{
            fontFamily: "var(--font-dm-sans)",
            fontSize: "13px",
            color: "#6b7280",
          }}
        >
          Loading...
        </div>
      ) : (
        <DashboardEmptyState />
      )}
```

This places DiscoverySection at the top unconditionally — visible whether or not the user has projects.

**Step 2: Verify it compiles**

Run: `cd web && npx tsc --noEmit`
Expected: No errors

**Step 3: Commit**

```bash
git add web/app/page.tsx
git commit -m "feat(web): mount DiscoverySection at top of dashboard"
```

---

### Task 7: Manual verification

**Step 1: Start the backend**

Run: `make dev`
Expected: Server starts, discover endpoints accessible

**Step 2: Test backend endpoints**

Run: `curl http://localhost:8080/api/v1/discover/github | jq '.data | length'`
Expected: 25 (or close, depending on GitHub API)

Run: `curl 'http://localhost:8080/api/v1/discover/dockerhub?q=nginx' | jq '.data[0].name'`
Expected: `"nginx"`

**Step 3: Test frontend**

Run: `make frontend-dev` (in another terminal)
Visit `http://localhost:3000` — verify:
- Discovery section appears at top of dashboard
- GitHub tab shows trending repos with star counts
- Docker Hub tab shows popular images
- Clicking "Track" creates project + source + triggers poll
- Already-tracked repos show green "Tracked" badge

**Step 4: Run all tests**

Run: `go test ./... && cd web && npx tsc --noEmit`
Expected: All pass

**Step 5: Final commit (if any tweaks needed)**

```bash
git add -A
git commit -m "fix(web): polish discovery section after manual testing"
```
