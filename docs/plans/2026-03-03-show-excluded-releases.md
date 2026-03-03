# Show Excluded Releases Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Show all releases including filtered-out ones (marked as gray/excluded) on the releases page and projects page.

**Architecture:** Add `Excluded` field to Release model, computed via SQL CASE expression. API gets `include_excluded` query param. Frontend gets `show_excluded` URL toggle on releases page; projects page always shows all with excluded grayed out.

**Tech Stack:** Go (backend model + SQL + handlers), Next.js/React (frontend pages), PostgreSQL (CASE expression)

---

### Task 1: Add Excluded field to Release model

**Files:**
- Modify: `internal/models/release.go:8-21`

**Step 1: Add the field**

Add `Excluded bool` to the Release struct, after the Repository field:

```go
Excluded bool `json:"excluded"` // true when filtered out by source version filters
```

**Step 2: Run tests to verify no breakage**

Run: `go test ./internal/...`
Expected: PASS (field is zero-valued by default, no scan changes yet)

**Step 3: Commit**

```bash
git add internal/models/release.go
git commit -m "feat(models): add Excluded field to Release struct"
```

---

### Task 2: Update ReleasesStore interface and mock

**Files:**
- Modify: `internal/api/releases.go:11-16` (interface)
- Modify: `internal/api/releases_test.go:15-54` (mock)

**Step 1: Update the interface**

Change the three list method signatures to add `includeExcluded bool`:

```go
type ReleasesStore interface {
	ListAllReleases(ctx context.Context, page, perPage int, includeExcluded bool) ([]models.Release, int, error)
	ListReleasesBySource(ctx context.Context, sourceID string, page, perPage int, includeExcluded bool) ([]models.Release, int, error)
	ListReleasesByProject(ctx context.Context, projectID string, page, perPage int, includeExcluded bool) ([]models.Release, int, error)
	GetRelease(ctx context.Context, id string) (*models.Release, error)
}
```

**Step 2: Update the mock**

Add `includeExcluded bool` param to all three mock methods (ignore the value — mock returns static data):

```go
func (m *mockReleasesStore) ListAllReleases(_ context.Context, page, perPage int, includeExcluded bool) ([]models.Release, int, error) {
func (m *mockReleasesStore) ListReleasesBySource(_ context.Context, sourceID string, page, perPage int, includeExcluded bool) ([]models.Release, int, error) {
func (m *mockReleasesStore) ListReleasesByProject(_ context.Context, projectID string, page, perPage int, includeExcluded bool) ([]models.Release, int, error) {
```

**Step 3: Update handlers to pass the new param**

In `releases.go`, update each handler to parse `include_excluded` and pass it:

```go
// In List():
includeExcluded := r.URL.Query().Get("include_excluded") == "true"
releases, total, err := h.store.ListAllReleases(r.Context(), page, perPage, includeExcluded)

// In ListBySource():
includeExcluded := r.URL.Query().Get("include_excluded") == "true"
releases, total, err := h.store.ListReleasesBySource(r.Context(), sourceID, page, perPage, includeExcluded)

// In ListByProject():
includeExcluded := r.URL.Query().Get("include_excluded") == "true"
releases, total, err := h.store.ListReleasesByProject(r.Context(), projectID, page, perPage, includeExcluded)
```

**Step 4: Run tests**

Run: `go test ./internal/api/...`
Expected: PASS (mock updated, handlers updated)

**Step 5: Commit**

```bash
git add internal/api/releases.go internal/api/releases_test.go
git commit -m "feat(api): add includeExcluded param to ReleasesStore interface and handlers"
```

---

### Task 3: Update AgentDataStore interface and mock

The agent's `AgentDataStore` also declares `ListReleasesByProject`. It needs the same signature change but always passes `false`.

**Files:**
- Modify: `internal/agent/tools.go:15-19` (interface) and line 135 (caller)
- Modify: `internal/agent/tools_test.go:13-28` (mock)

**Step 1: Update the interface**

```go
type AgentDataStore interface {
	ListReleasesByProject(ctx context.Context, projectID string, page, perPage int, includeExcluded bool) ([]models.Release, int, error)
	GetRelease(ctx context.Context, id string) (*models.Release, error)
	ListContextSources(ctx context.Context, projectID string, page, perPage int) ([]models.ContextSource, int, error)
}
```

**Step 2: Update the caller in tools.go**

Line 135: add `false` as the last argument:

```go
releases, total, err := f.store.ListReleasesByProject(ctx, f.projectID, page, perPage, false)
```

**Step 3: Update mock in tools_test.go**

```go
func (m *mockDataStore) ListReleasesByProject(_ context.Context, _ string, page, perPage int, includeExcluded bool) ([]models.Release, int, error) {
```

**Step 4: Run tests**

Run: `go test ./internal/agent/...`
Expected: PASS

**Step 5: Commit**

```bash
git add internal/agent/tools.go internal/agent/tools_test.go
git commit -m "feat(agent): update AgentDataStore for includeExcluded param"
```

---

### Task 4: Update PgStore SQL queries

**Files:**
- Modify: `internal/api/pgstore.go:205-320`

**Step 1: Update ListAllReleases**

Add `includeExcluded bool` param. When true:
- Count query: remove version filter WHERE clauses
- Select query: remove version filter WHERE clauses, add CASE expression as `excluded` column
- Scan the new `Excluded` field

When false: keep existing behavior, scan `false` for excluded.

The CASE expression:
```sql
CASE WHEN
  (s.version_filter_include IS NOT NULL AND r.version !~ s.version_filter_include)
  OR (s.version_filter_exclude IS NOT NULL AND r.version ~ s.version_filter_exclude)
  OR (s.exclude_prereleases = true AND r.raw_data->>'prerelease' = 'true')
THEN true ELSE false END AS excluded
```

Implementation approach: use two code paths (if/else on `includeExcluded`) for each method. The "include excluded" path has no WHERE filter + CASE, the "exclude" path has the WHERE filter and scans a constant `false`.

**Step 2: Update ListReleasesBySource** — same pattern.

**Step 3: Update ListReleasesByProject** — same pattern.

**Step 4: Run tests**

Run: `go test ./internal/...`
Expected: PASS

Run: `go vet ./...`
Expected: clean

**Step 5: Commit**

```bash
git add internal/api/pgstore.go
git commit -m "feat(store): compute excluded flag in SQL queries"
```

---

### Task 5: Update frontend types and API client

**Files:**
- Modify: `web/lib/api/types.ts:76-87` (Release interface)
- Modify: `web/lib/api/client.ts:85-94` (releases API)

**Step 1: Add excluded to Release type**

```typescript
export interface Release {
  // ... existing fields ...
  excluded?: boolean;
}
```

**Step 2: Update API client methods**

Add `includeExcluded` parameter to `list` and `listByProject`:

```typescript
export const releases = {
  list: (page = 1, perPage = 25, includeExcluded = false) =>
    request<ApiResponse<Release[]>>(
      `/releases?page=${page}&per_page=${perPage}${includeExcluded ? '&include_excluded=true' : ''}`
    ),
  listBySource: (sourceId: string, page = 1) =>
    request<ApiResponse<Release[]>>(`/sources/${sourceId}/releases?page=${page}`),
  listByProject: (projectId: string, page = 1, perPage = 25, includeExcluded = false) =>
    request<ApiResponse<Release[]>>(
      `/projects/${projectId}/releases?page=${page}&per_page=${perPage}${includeExcluded ? '&include_excluded=true' : ''}`
    ),
  get: (id: string) =>
    request<ApiResponse<Release>>(`/releases/${id}`),
};
```

**Step 3: Commit**

```bash
git add web/lib/api/types.ts web/lib/api/client.ts
git commit -m "feat(web): add excluded field to Release type and API client"
```

---

### Task 6: Add show_excluded toggle to releases page

**Files:**
- Modify: `web/app/releases/page.tsx`

**Step 1: Add URL state for show_excluded**

Read `show_excluded` from search params (default `"true"`). Add toggle state:

```typescript
const initialShowExcluded = searchParams.get("show_excluded") !== "false"; // default true
const [showExcluded, setShowExcluded] = useState(initialShowExcluded);
```

**Step 2: Update API calls to pass includeExcluded**

```typescript
const { data: scopedData, isLoading: scopedLoading } = useSWR(
  projectFilter !== "all" ? ["releases", page, projectFilter, showExcluded] : null,
  () => releasesApi.listByProject(projectFilter, page, PER_PAGE, showExcluded)
);

const { data: allReleasesData, isLoading: allLoading } = useSWR(
  projectFilter === "all" ? ["all-releases", page, showExcluded] : null,
  () => releasesApi.list(page, PER_PAGE, showExcluded)
);
```

**Step 3: Add toggle UI**

Next to the project filter select, add a toggle button:

```tsx
<label className="inline-flex items-center gap-2 ml-3 cursor-pointer select-none">
  <span style={{
    fontFamily: "var(--font-dm-sans)",
    fontSize: "13px",
    color: "#6b7280",
  }}>
    Show excluded
  </span>
  <button
    role="switch"
    aria-checked={showExcluded}
    onClick={() => {
      const next = !showExcluded;
      setShowExcluded(next);
      setPage(1);
      const params = new URLSearchParams(window.location.search);
      if (next) params.delete("show_excluded");
      else params.set("show_excluded", "false");
      window.history.replaceState({}, "", `?${params.toString()}`);
    }}
    className="relative inline-flex h-5 w-9 items-center rounded-full transition-colors"
    style={{ backgroundColor: showExcluded ? "#e8601a" : "#d1d5db" }}
  >
    <span
      className="inline-block h-3.5 w-3.5 rounded-full bg-white transition-transform"
      style={{ transform: showExcluded ? "translateX(18px)" : "translateX(3px)" }}
    />
  </button>
</label>
```

**Step 4: Gray out excluded rows**

On the `<tr>` for each release, apply conditional styling:

```tsx
<tr
  key={release.id}
  className={`transition-colors ${release.excluded ? '' : 'hover:bg-[#fafaf9]'}`}
  style={{
    borderBottom: "1px solid #e8e8e5",
    opacity: release.excluded ? 0.45 : 1,
  }}
>
```

**Step 5: Commit**

```bash
git add web/app/releases/page.tsx
git commit -m "feat(web): add show_excluded toggle to releases page"
```

---

### Task 7: Show excluded releases on projects page

**Files:**
- Modify: `web/app/projects/page.tsx`

**Step 1: Update API calls to pass includeExcluded=true**

In `ProjectFlowCard` (line 157):
```typescript
const { data: relData } = useSWR(
  `project-${project.id}-card-releases`,
  () => releasesApi.listByProject(project.id, 1, 25, true),
);
```

In `ProjectCompactRow` (line 377):
```typescript
const { data: relData } = useSWR(`project-${project.id}-card-releases`, () =>
  releasesApi.listByProject(project.id, 1, 5, true)
);
```

**Step 2: Gray out excluded releases in FlowSection**

In `ProjectFlowCard` releases flow (around line 255-283), apply muted color for excluded releases:

```tsx
{releases.map((r) => {
  const src = sourceMap.get(r.source_id);
  return (
    <span key={r.id} className="inline-flex items-baseline mr-2.5">
      <Link
        href={`/releases/${r.id}`}
        className={r.excluded ? "" : "text-[#2563eb] hover:underline"}
        style={{
          fontFamily: "'JetBrains Mono', monospace",
          fontSize: "12px",
          ...(r.excluded ? { color: "#c4c4c0" } : {}),
        }}
      >
        {r.version}
      </Link>
      {/* ... rest unchanged but age/source spans inherit muted style if excluded */}
    </span>
  );
})}
```

**Step 3: Commit**

```bash
git add web/app/projects/page.tsx
git commit -m "feat(web): show excluded releases as gray on projects page"
```

---

### Task 8: Verify everything works end-to-end

**Step 1: Run all Go tests**

Run: `go test ./...`
Expected: PASS

**Step 2: Run go vet**

Run: `go vet ./...`
Expected: clean

**Step 3: Build frontend**

Run: `cd web && npm run build`
Expected: clean build

**Step 4: Manual verification**

Start the dev server. Verify:
1. `/releases` page shows excluded releases grayed out by default
2. Toggle to hide excluded — they disappear
3. URL updates with `show_excluded` param
4. Projects page shows excluded releases in gray in the flow section
5. API without `include_excluded` behaves exactly as before

**Step 5: Final commit if any fixups needed**
