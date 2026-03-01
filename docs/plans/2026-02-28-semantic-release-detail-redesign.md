# Semantic Release Detail Page Redesign — Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Redesign the semantic release detail page to display all 12 report fields (risk level, status checks, download links, changelog summary, etc.) and show only linked source releases via a new API endpoint.

**Architecture:** Backend adds one new GET endpoint for linked source releases. Frontend updates the TypeScript types to match the full Go model, adds an API client method, and rewrites the detail component with a top-down briefing layout prioritizing actionable intelligence.

**Tech Stack:** Go (net/http), Next.js (React), Tailwind CSS, SWR, Lucide icons

---

### Task 1: Add backend endpoint — GET /semantic-releases/{id}/sources

**Files:**
- Modify: `internal/api/semantic_releases.go:62-75` (add ListSources handler after Get)
- Modify: `internal/api/server.go:78` (register new route)

**Step 1: Add the ListSources handler**

In `internal/api/semantic_releases.go`, add after the `Get` handler (line 75):

```go
// ListSources handles GET /semantic-releases/{id}/sources — returns source releases linked to this semantic release.
func (h *SemanticReleasesHandler) ListSources(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		RespondError(w, r, http.StatusBadRequest, "bad_request", "Semantic release ID is required")
		return
	}
	releases, err := h.store.GetSemanticReleaseSources(r.Context(), id)
	if err != nil {
		RespondError(w, r, http.StatusNotFound, "not_found", "Semantic release not found")
		return
	}
	if releases == nil {
		releases = []models.Release{}
	}
	RespondJSON(w, r, http.StatusOK, releases)
}
```

**Step 2: Register the route**

In `internal/api/server.go`, after line 78 (the DELETE route), add:

```go
mux.Handle("GET /api/v1/semantic-releases/{id}/sources", chain(http.HandlerFunc(semanticReleases.ListSources)))
```

**Step 3: Run existing tests to verify no breakage**

Run: `go test ./internal/api/... -run TestSemanticReleases -v`
Expected: All existing tests PASS

**Step 4: Add test for the new endpoint**

In `internal/api/semantic_releases_test.go`, update `setupSemanticReleasesMux` to register the new route, then add a test:

Update `setupSemanticReleasesMux`:
```go
func setupSemanticReleasesMux(store SemanticReleasesStore) *http.ServeMux {
	h := NewSemanticReleasesHandler(store)
	mux := http.NewServeMux()
	mux.HandleFunc("GET /semantic-releases", h.ListAll)
	mux.HandleFunc("GET /projects/{projectId}/semantic-releases", h.List)
	mux.HandleFunc("GET /semantic-releases/{id}", h.Get)
	mux.HandleFunc("GET /semantic-releases/{id}/sources", h.ListSources)
	mux.HandleFunc("DELETE /semantic-releases/{id}", h.Delete)
	return mux
}
```

Add test:
```go
func TestSemanticReleasesHandlerListSources(t *testing.T) {
	now := time.Now()
	store := &mockSemanticReleasesStore{
		releases: []models.SemanticRelease{
			{ID: "sr-1", ProjectID: "p1", Version: "1.0.0", Status: "completed", CreatedAt: now},
		},
		sources: []models.Release{
			{ID: "r-1", SourceID: "s-1", Version: "v1.16.7", CreatedAt: now},
			{ID: "r-2", SourceID: "s-2", Version: "v1.16.7", CreatedAt: now},
		},
	}
	mux := setupSemanticReleasesMux(store)

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/semantic-releases/sr-1/sources", nil)
	mux.ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", w.Code)
	}

	var got struct {
		Data []models.Release `json:"data"`
	}
	if err := json.NewDecoder(w.Body).Decode(&got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(got.Data) != 2 {
		t.Fatalf("expected 2 source releases, got %d", len(got.Data))
	}
}

func TestSemanticReleasesHandlerListSourcesEmpty(t *testing.T) {
	store := &mockSemanticReleasesStore{
		releases: []models.SemanticRelease{
			{ID: "sr-1", ProjectID: "p1", Version: "1.0.0", Status: "completed", CreatedAt: time.Now()},
		},
	}
	mux := setupSemanticReleasesMux(store)

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/semantic-releases/sr-1/sources", nil)
	mux.ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", w.Code)
	}

	var raw map[string]json.RawMessage
	if err := json.NewDecoder(w.Body).Decode(&raw); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if string(raw["data"]) != "[]" {
		t.Fatalf("expected data to be empty array [], got %s", string(raw["data"]))
	}
}
```

**Step 5: Run all tests**

Run: `go test ./internal/api/... -run TestSemanticReleases -v`
Expected: All tests PASS including the two new ones

**Step 6: Commit**

```bash
git add internal/api/semantic_releases.go internal/api/server.go internal/api/semantic_releases_test.go
git commit -m "feat(api): add GET /semantic-releases/{id}/sources endpoint"
```

---

### Task 2: Update frontend TypeScript types

**Files:**
- Modify: `web/lib/api/types.ts:106-112`

**Step 1: Update SemanticReport interface**

Replace the `SemanticReport` interface at `web/lib/api/types.ts:106-112` with:

```typescript
export interface SemanticReport {
  subject?: string;
  risk_level?: string;
  risk_reason?: string;
  status_checks?: string[];
  changelog_summary?: string;
  download_commands?: string[];
  download_links?: string[];
  summary?: string;
  availability?: string;
  adoption?: string;
  urgency?: string;
  recommendation?: string;
}
```

All fields are optional since older reports may not have every field.

**Step 2: Commit**

```bash
git add web/lib/api/types.ts
git commit -m "feat(web): add missing SemanticReport fields to TypeScript types"
```

---

### Task 3: Add frontend API client method

**Files:**
- Modify: `web/lib/api/client.ts:117-126`

**Step 1: Add getSources method**

In `web/lib/api/client.ts`, add to the `semanticReleases` object (after the `delete` method):

```typescript
export const semanticReleases = {
  listAll: (page = 1, perPage = 25) =>
    request<ApiResponse<SemanticRelease[]>>(`/semantic-releases?page=${page}&per_page=${perPage}`),
  list: (projectId: string, page = 1, perPage = 25) =>
    request<ApiResponse<SemanticRelease[]>>(`/projects/${projectId}/semantic-releases?page=${page}&per_page=${perPage}`),
  get: (id: string) =>
    request<ApiResponse<SemanticRelease>>(`/semantic-releases/${id}`),
  delete: (id: string) =>
    request<ApiResponse<null>>(`/semantic-releases/${id}`, { method: "DELETE" }),
  getSources: (id: string) =>
    request<ApiResponse<Release[]>>(`/semantic-releases/${id}/sources`),
};
```

**Step 2: Commit**

```bash
git add web/lib/api/client.ts
git commit -m "feat(web): add getSources API client method for semantic releases"
```

---

### Task 4: Rewrite semantic release detail component

**Files:**
- Modify: `web/components/semantic-releases/semantic-release-detail.tsx` (full rewrite)

**Step 1: Rewrite the component**

Replace the entire contents of `web/components/semantic-releases/semantic-release-detail.tsx`. The new component should:

1. **Data fetching** — Use SWR to fetch:
   - `srApi.get(srId)` — the semantic release
   - `projectsApi.get(projectId)` — project metadata
   - `srApi.getSources(srId)` — linked source releases (NEW — replaces `releasesApi.listByProject`)
   - `sourcesApi.listByProject(projectId)` — source definitions (for provider badges)
   - `contextSources.list(projectId)` — context sources (NEW)

2. **Layout sections** (in order):
   - **Back link** — unchanged
   - **Byline** — project name in italic Fraunces
   - **Version heading** — h1 at 42px
   - **Subject** — `sr.report.subject` rendered as a subtitle (20px, DM Sans, `text-[#374151]`)
   - **Meta line** — status dot + age + delete button (unchanged)
   - **Divider**
   - **Error state** (if applicable)
   - **Risk & Urgency Banner** — Full-width colored card. Color-coded by `risk_level`: CRITICAL=`#dc2626`/`#fff1f2`, HIGH=`#d97706`/`#fff8f0`, MEDIUM=`#ca8a04`/`#fefce8`, LOW=`#16a34a`/`#f0fdf4`. Shows risk level as uppercase badge, urgency level, and `risk_reason` as body text. Replaces the old `<UrgencyCallout>`.
   - **Status Checks & Downloads** — Section with:
     - Status check pills: green checkmark icon + text (e.g., "Docker Image Verified")
     - Download links: pills with external-link icon, wrapped in `<a>` tags
     - Download commands: inline `<code>` blocks with a copy button (uses `navigator.clipboard.writeText`)
   - **Adoption** — Only shown if `sr.report.adoption` exists. Rendered as a callout card with `SectionLabel`.
   - **Source Releases** — Table from `srApi.getSources(srId)`, same layout as current but scoped to linked releases. Version chips are clickable — for GitHub sources link to `https://github.com/{repository}/releases/tag/{version}`, for Docker sources link to `https://hub.docker.com/r/{repository}/tags?name={version}`.
   - **Context Sources** — List of context source cards from `contextSources.list(projectId)`. Each shows name, type badge, and a link extracted from `config.url` if present. Only shown if context sources exist.
   - **Changelog Summary** — `sr.report.changelog_summary` as prose text.
   - **Recommendation** — Pull-quote blockquote with orange left border (unchanged).

3. **Styling** — Follow existing patterns:
   - Fonts: `var(--font-fraunces)` for headings, `var(--font-dm-sans)` for body, `'JetBrains Mono', monospace` for code
   - Colors: `#111113` primary, `#6b7280` secondary, `#9ca3af` muted, `#e8601a` accent
   - Container: `max-w-[760px]` centered
   - Cards: `border: 1px solid #e8e8e5`, `backgroundColor: #ffffff`
   - Section spacing: `space-y-10` or `mt-10`

**Step 2: Verify the build compiles**

Run: `cd web && npm run build`
Expected: Build succeeds with no TypeScript errors

**Step 3: Commit**

```bash
git add web/components/semantic-releases/semantic-release-detail.tsx
git commit -m "feat(web): redesign semantic release detail page with full report fields"
```

---

### Task 5: Visual verification

**Step 1: Start the dev server**

Run: `make dev` (or `make run` if Postgres is already running)

**Step 2: Navigate to a semantic release detail page**

Open: `http://localhost:3000/projects/{projectId}/semantic-releases/{srId}` with a real semantic release that has a full report.

**Step 3: Verify all sections render correctly**

Check:
- Subject line appears below version
- Risk & urgency banner is color-coded and shows risk_reason
- Status checks show as green pills
- Download links are clickable and open in new tab
- Download commands have copy buttons that work
- Source releases table shows ONLY linked releases (not all project releases)
- Context sources show with links
- Changelog summary renders as prose
- Recommendation blockquote still has orange accent border

**Step 4: Final commit if any tweaks needed**

```bash
git add -A
git commit -m "fix(web): polish semantic release detail page layout"
```

---

### Task 6: Run all tests

**Step 1: Run Go tests**

Run: `go test ./internal/api/...`
Expected: All PASS

**Step 2: Run frontend build**

Run: `cd web && npm run build`
Expected: Build succeeds

**Step 3: Commit all remaining changes**

Ensure all changes are committed. Create a final summary commit if needed.
