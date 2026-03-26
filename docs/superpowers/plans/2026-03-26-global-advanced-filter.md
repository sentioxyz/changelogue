# Global Advanced Filter Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace ad-hoc inline filters on the releases and todo pages with a shared chip-based `<FilterBar>` component, backed by extended API query params for provider, urgency, and date range filtering.

**Architecture:** A config-driven `<FilterBar>` component renders active filters as removable chips with an "Add filter" popover. A `useFilterParams()` hook syncs filter state to URL query params. The backend extends existing `GET /releases` and `GET /todos` endpoints with new optional query params that map to dynamic SQL WHERE clauses.

**Tech Stack:** Go (backend handlers + pgstore), Next.js/React + SWR (frontend), Tailwind CSS (styling)

---

## File Structure

### New files
| File | Responsibility |
|------|---------------|
| `web/components/filters/filter-bar.tsx` | `<FilterBar>` component — chip rendering, "Add filter" popover, "Clear all" |
| `web/components/filters/use-filter-params.ts` | `useFilterParams()` hook — URL read/write, state management |
| `internal/api/filters.go` | `ReleaseFilter` and `TodoFilter` structs + `ParseReleaseFilters()` / `ParseTodoFilters()` helper functions |

### Modified files
| File | Change |
|------|--------|
| `internal/api/releases.go` | Update `ReleasesStore` interface and handlers to accept filter struct |
| `internal/api/releases_test.go` | Update mock store signatures, add filter-passing tests |
| `internal/api/todos.go` | Update `TodosStore` interface and handler to accept filter struct |
| `internal/api/pgstore.go` | Extend `ListAllReleases`, `ListReleasesByProject`, `ListTodos` with dynamic WHERE clauses |
| `web/lib/api/client.ts` | Change `releases.list()` and `todos.list()` to accept filter objects |
| `web/components/dashboard/unified-feed.tsx` | Update `releases.list()` call to match new signature |
| `web/app/releases/page.tsx` | Replace inline filters with `<FilterBar>` |
| `web/app/todo/page.tsx` | Replace inline tabs/toggles with `<FilterBar>` |

---

## Task 1: Add backend filter structs and parse helpers

**Files:**
- Create: `internal/api/filters.go`

- [ ] **Step 1: Create filter structs and parse helpers**

```go
// internal/api/filters.go
package api

import (
	"net/http"
	"time"
)

// ReleaseFilter holds optional filter parameters for release listing endpoints.
type ReleaseFilter struct {
	Provider  string
	Urgency   string
	DateFrom  *time.Time
	DateTo    *time.Time
}

// TodoFilter holds optional filter parameters for the todo listing endpoint.
type TodoFilter struct {
	ProjectID string
	Provider  string
	Urgency   string
	DateFrom  *time.Time
	DateTo    *time.Time
}

// ParseReleaseFilters extracts release filter params from the request query string.
func ParseReleaseFilters(r *http.Request) ReleaseFilter {
	q := r.URL.Query()
	f := ReleaseFilter{
		Provider: q.Get("provider"),
		Urgency:  q.Get("urgency"),
	}
	if v := q.Get("date_from"); v != "" {
		if t, err := time.Parse("2006-01-02", v); err == nil {
			f.DateFrom = &t
		}
	}
	if v := q.Get("date_to"); v != "" {
		if t, err := time.Parse("2006-01-02", v); err == nil {
			// Set to end of day so the date is inclusive
			end := t.Add(24*time.Hour - time.Nanosecond)
			f.DateTo = &end
		}
	}
	return f
}

// ParseTodoFilters extracts todo filter params from the request query string.
func ParseTodoFilters(r *http.Request) TodoFilter {
	q := r.URL.Query()
	f := TodoFilter{
		ProjectID: q.Get("project"),
		Provider:  q.Get("provider"),
		Urgency:   q.Get("urgency"),
	}
	if v := q.Get("date_from"); v != "" {
		if t, err := time.Parse("2006-01-02", v); err == nil {
			f.DateFrom = &t
		}
	}
	if v := q.Get("date_to"); v != "" {
		if t, err := time.Parse("2006-01-02", v); err == nil {
			end := t.Add(24*time.Hour - time.Nanosecond)
			f.DateTo = &end
		}
	}
	return f
}
```

- [ ] **Step 2: Verify it compiles**

Run: `go vet ./internal/api/...`
Expected: No errors

- [ ] **Step 3: Commit**

```bash
git add internal/api/filters.go
git commit -m "feat(api): add ReleaseFilter and TodoFilter structs with parse helpers"
```

---

## Task 2: Update releases handler and store interface

**Files:**
- Modify: `internal/api/releases.go`
- Modify: `internal/api/releases_test.go`

- [ ] **Step 1: Update the ReleasesStore interface**

In `internal/api/releases.go`, change the interface (lines 11-16) to accept filter structs:

```go
// ReleasesStore defines the persistence operations for releases (read-only).
type ReleasesStore interface {
	ListAllReleases(ctx context.Context, page, perPage int, includeExcluded bool, filter ReleaseFilter) ([]models.Release, int, error)
	ListReleasesBySource(ctx context.Context, sourceID string, page, perPage int, includeExcluded bool, filter ReleaseFilter) ([]models.Release, int, error)
	ListReleasesByProject(ctx context.Context, projectID string, page, perPage int, includeExcluded bool, filter ReleaseFilter) ([]models.Release, int, error)
	GetRelease(ctx context.Context, id string) (*models.Release, error)
}
```

- [ ] **Step 2: Update the List handler**

In `internal/api/releases.go`, update the `List` method (lines 29-41):

```go
func (h *ReleasesHandler) List(w http.ResponseWriter, r *http.Request) {
	page, perPage := ParsePagination(r)
	includeExcluded := r.URL.Query().Get("include_excluded") == "true"
	filter := ParseReleaseFilters(r)
	releases, total, err := h.store.ListAllReleases(r.Context(), page, perPage, includeExcluded, filter)
	if err != nil {
		RespondError(w, r, http.StatusInternalServerError, "internal_error", "Failed to list releases")
		return
	}
	if releases == nil {
		releases = []models.Release{}
	}
	RespondList(w, r, http.StatusOK, releases, page, perPage, total)
}
```

- [ ] **Step 3: Update the ListBySource handler**

In `internal/api/releases.go`, update `ListBySource` (lines 44-61):

```go
func (h *ReleasesHandler) ListBySource(w http.ResponseWriter, r *http.Request) {
	sourceID := r.PathValue("id")
	if sourceID == "" {
		RespondError(w, r, http.StatusBadRequest, "bad_request", "Source ID is required")
		return
	}
	page, perPage := ParsePagination(r)
	includeExcluded := r.URL.Query().Get("include_excluded") == "true"
	filter := ParseReleaseFilters(r)
	releases, total, err := h.store.ListReleasesBySource(r.Context(), sourceID, page, perPage, includeExcluded, filter)
	if err != nil {
		RespondError(w, r, http.StatusInternalServerError, "internal_error", "Failed to list releases")
		return
	}
	if releases == nil {
		releases = []models.Release{}
	}
	RespondList(w, r, http.StatusOK, releases, page, perPage, total)
}
```

- [ ] **Step 4: Update the ListByProject handler**

In `internal/api/releases.go`, update `ListByProject` (lines 63-81):

```go
func (h *ReleasesHandler) ListByProject(w http.ResponseWriter, r *http.Request) {
	projectID := r.PathValue("projectId")
	if projectID == "" {
		RespondError(w, r, http.StatusBadRequest, "bad_request", "Project ID is required")
		return
	}
	page, perPage := ParsePagination(r)
	includeExcluded := r.URL.Query().Get("include_excluded") == "true"
	filter := ParseReleaseFilters(r)
	releases, total, err := h.store.ListReleasesByProject(r.Context(), projectID, page, perPage, includeExcluded, filter)
	if err != nil {
		RespondError(w, r, http.StatusInternalServerError, "internal_error", "Failed to list releases")
		return
	}
	if releases == nil {
		releases = []models.Release{}
	}
	RespondList(w, r, http.StatusOK, releases, page, perPage, total)
}
```

- [ ] **Step 5: Update mock store in tests**

In `internal/api/releases_test.go`, update the mock signatures (lines 25-44):

```go
func (m *mockReleasesStore) ListAllReleases(_ context.Context, page, perPage int, includeExcluded bool, filter ReleaseFilter) ([]models.Release, int, error) {
	if m.listErr != nil {
		return nil, 0, m.listErr
	}
	return m.allReleases, len(m.allReleases), nil
}

func (m *mockReleasesStore) ListReleasesBySource(_ context.Context, sourceID string, page, perPage int, includeExcluded bool, filter ReleaseFilter) ([]models.Release, int, error) {
	if m.listErr != nil {
		return nil, 0, m.listErr
	}
	return m.sourceReleases, len(m.sourceReleases), nil
}

func (m *mockReleasesStore) ListReleasesByProject(_ context.Context, projectID string, page, perPage int, includeExcluded bool, filter ReleaseFilter) ([]models.Release, int, error) {
	if m.listErr != nil {
		return nil, 0, m.listErr
	}
	return m.projectReleases, len(m.projectReleases), nil
}
```

- [ ] **Step 6: Verify it compiles and tests pass**

Run: `go test ./internal/api/... -run TestReleases -v`
Expected: All existing tests pass

- [ ] **Step 7: Commit**

```bash
git add internal/api/releases.go internal/api/releases_test.go
git commit -m "feat(api): add filter param to releases handler and store interface"
```

---

## Task 3: Update todos handler and store interface

**Files:**
- Modify: `internal/api/todos.go`

- [ ] **Step 1: Update TodosStore interface**

In `internal/api/todos.go`, change the interface (lines 11-17):

```go
type TodosStore interface {
	ListTodos(ctx context.Context, status string, page, perPage int, aggregated bool, filter TodoFilter) ([]models.Todo, int, error)
	GetTodo(ctx context.Context, id string) (*models.Todo, error)
	AcknowledgeTodo(ctx context.Context, id string, cascade bool) error
	ResolveTodo(ctx context.Context, id string, cascade bool) error
	ReopenTodo(ctx context.Context, id string) error
}
```

- [ ] **Step 2: Update List handler**

In `internal/api/todos.go`, update the `List` method (lines 31-45):

```go
func (h *TodosHandler) List(w http.ResponseWriter, r *http.Request) {
	page, perPage := ParsePagination(r)
	status := r.URL.Query().Get("status")
	aggregated := r.URL.Query().Get("aggregated") == "true"
	filter := ParseTodoFilters(r)

	todos, total, err := h.store.ListTodos(r.Context(), status, page, perPage, aggregated, filter)
	if err != nil {
		RespondError(w, r, http.StatusInternalServerError, "internal_error", err.Error())
		return
	}
	if todos == nil {
		todos = []models.Todo{}
	}
	RespondList(w, r, http.StatusOK, todos, page, perPage, total)
}
```

- [ ] **Step 3: Verify it compiles**

Run: `go vet ./internal/api/...`
Expected: Compilation errors from pgstore (expected — will be fixed in Task 4)

- [ ] **Step 4: Commit**

```bash
git add internal/api/todos.go
git commit -m "feat(api): add filter param to todos handler and store interface"
```

---

## Task 4: Extend pgstore with dynamic WHERE clauses

**Files:**
- Modify: `internal/api/pgstore.go`

This is the largest task. We need to add filter WHERE clause building to three functions.

- [ ] **Step 1: Add a filter clause builder helper at the top of pgstore.go**

Add this helper function near the top of `pgstore.go` (after imports):

```go
// appendFilterClause appends a parameterized condition to the WHERE clause builder.
func appendFilterClause(clauses *[]string, args *[]any, expr string, val any) {
	idx := len(*args) + 1
	*clauses = append(*clauses, fmt.Sprintf(expr, idx))
	*args = append(*args, val)
}
```

- [ ] **Step 2: Add release filter WHERE clause builder**

```go
// buildReleaseFilterClauses returns SQL WHERE conditions and args for ReleaseFilter fields.
// startIdx is the next available placeholder index (e.g. 1 if no prior args, 3 if $1/$2 are taken).
func buildReleaseFilterClauses(filter ReleaseFilter, startIdx int) ([]string, []any) {
	var clauses []string
	var args []any
	idx := startIdx
	if filter.Provider != "" {
		clauses = append(clauses, fmt.Sprintf("s.provider = $%d", idx))
		args = append(args, filter.Provider)
		idx++
	}
	if filter.Urgency != "" {
		clauses = append(clauses, fmt.Sprintf("sr_info.urgency ILIKE $%d", idx))
		args = append(args, filter.Urgency)
		idx++
	}
	if filter.DateFrom != nil {
		clauses = append(clauses, fmt.Sprintf("COALESCE(r.released_at, r.created_at) >= $%d", idx))
		args = append(args, *filter.DateFrom)
		idx++
	}
	if filter.DateTo != nil {
		clauses = append(clauses, fmt.Sprintf("COALESCE(r.released_at, r.created_at) <= $%d", idx))
		args = append(args, *filter.DateTo)
		idx++
	}
	return clauses, args
}
```

- [ ] **Step 3: Update ListAllReleases signature and inject filter clauses**

Change the function signature on line 237:

```go
func (s *PgStore) ListAllReleases(ctx context.Context, page, perPage int, includeExcluded bool, filter ReleaseFilter) ([]models.Release, int, error) {
```

The approach: after the existing WHERE clauses for version filtering, append the new filter clauses. Since this function has two code paths (includeExcluded true/false) with separate SQL strings, we need to inject filter conditions into both.

For the **count query** (both paths): build extra WHERE clauses from the filter and append them. Note that the urgency filter references `sr_info` from the LATERAL join, which is NOT available in the count query. For count, we skip the urgency filter.

For the **data query** (both paths): append all filter clauses including urgency.

The simplest approach: refactor both code paths to use dynamic query building instead of two static SQL strings. Replace lines 238-313 with:

```go
	var total int
	offset := (page - 1) * perPage

	// Base count WHERE conditions
	countWhere := []string{}
	countArgs := []any{}
	if !includeExcluded {
		countWhere = append(countWhere,
			"(s.version_filter_include IS NULL OR r.version ~ s.version_filter_include)",
			"(s.version_filter_exclude IS NULL OR r.version !~ s.version_filter_exclude)",
			"(s.exclude_prereleases = false OR r.raw_data->>'prerelease' IS NULL OR r.raw_data->>'prerelease' != 'true')",
		)
	}
	// Add filter conditions (skip urgency for count — it requires the LATERAL join)
	countFilterIdx := len(countArgs) + 1
	if filter.Provider != "" {
		countWhere = append(countWhere, fmt.Sprintf("s.provider = $%d", countFilterIdx))
		countArgs = append(countArgs, filter.Provider)
		countFilterIdx++
	}
	if filter.DateFrom != nil {
		countWhere = append(countWhere, fmt.Sprintf("COALESCE(r.released_at, r.created_at) >= $%d", countFilterIdx))
		countArgs = append(countArgs, *filter.DateFrom)
		countFilterIdx++
	}
	if filter.DateTo != nil {
		countWhere = append(countWhere, fmt.Sprintf("COALESCE(r.released_at, r.created_at) <= $%d", countFilterIdx))
		countArgs = append(countArgs, *filter.DateTo)
		countFilterIdx++
	}

	countSQL := `SELECT COUNT(*) FROM releases r LEFT JOIN sources s ON r.source_id = s.id`
	if len(countWhere) > 0 {
		countSQL += " WHERE " + strings.Join(countWhere, " AND ")
	}
	if err := s.pool.QueryRow(ctx, countSQL, countArgs...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count releases: %w", err)
	}

	// Build data query WHERE conditions
	dataWhere := []string{}
	dataArgs := []any{}
	if includeExcluded {
		// "excluded" flag is computed in SELECT, no WHERE filter needed for version exclusions
	} else {
		dataWhere = append(dataWhere,
			"(s.version_filter_include IS NULL OR r.version ~ s.version_filter_include)",
			"(s.version_filter_exclude IS NULL OR r.version !~ s.version_filter_exclude)",
			"(s.exclude_prereleases = false OR r.raw_data->>'prerelease' IS NULL OR r.raw_data->>'prerelease' != 'true')",
		)
	}
	dataFilterIdx := len(dataArgs) + 1
	if filter.Provider != "" {
		dataWhere = append(dataWhere, fmt.Sprintf("s.provider = $%d", dataFilterIdx))
		dataArgs = append(dataArgs, filter.Provider)
		dataFilterIdx++
	}
	if filter.Urgency != "" {
		dataWhere = append(dataWhere, fmt.Sprintf("sr_info.urgency ILIKE $%d", dataFilterIdx))
		dataArgs = append(dataArgs, filter.Urgency)
		dataFilterIdx++
	}
	if filter.DateFrom != nil {
		dataWhere = append(dataWhere, fmt.Sprintf("COALESCE(r.released_at, r.created_at) >= $%d", dataFilterIdx))
		dataArgs = append(dataArgs, *filter.DateFrom)
		dataFilterIdx++
	}
	if filter.DateTo != nil {
		dataWhere = append(dataWhere, fmt.Sprintf("COALESCE(r.released_at, r.created_at) <= $%d", dataFilterIdx))
		dataArgs = append(dataArgs, *filter.DateTo)
		dataFilterIdx++
	}

	excludedExpr := "false"
	if includeExcluded {
		excludedExpr = `CASE WHEN
			(s.version_filter_include IS NOT NULL AND r.version !~ s.version_filter_include)
			OR (s.version_filter_exclude IS NOT NULL AND r.version ~ s.version_filter_exclude)
			OR (s.exclude_prereleases = true AND r.raw_data->>'prerelease' = 'true')
		THEN true ELSE false END`
	}

	dataSQL := fmt.Sprintf(`SELECT r.id, r.source_id, r.version, COALESCE(r.raw_data,'{}'), r.released_at, r.created_at,
		COALESCE(p.id::text,''), COALESCE(p.name,''), COALESCE(s.provider,''), COALESCE(s.repository,''),
		%s,
		COALESCE(sr_info.id::text,''), COALESCE(sr_info.status,''), COALESCE(sr_info.urgency,'')
	FROM releases r
	LEFT JOIN sources s ON r.source_id = s.id
	LEFT JOIN projects p ON s.project_id = p.id
	LEFT JOIN LATERAL (
		(SELECT sr.id, sr.status, sr.report->>'urgency' AS urgency, 0 AS priority
		 FROM semantic_releases sr
		 WHERE sr.project_id = p.id AND sr.version = r.version
		 ORDER BY sr.created_at DESC LIMIT 1)
		UNION ALL
		(SELECT NULL::uuid, 'processing', '', 1
		 FROM agent_runs ar
		 WHERE ar.project_id = p.id AND ar.version = r.version
		   AND ar.status IN ('pending', 'running')
		 LIMIT 1)
		ORDER BY priority LIMIT 1
	) sr_info ON true`, excludedExpr)

	if len(dataWhere) > 0 {
		dataSQL += " WHERE " + strings.Join(dataWhere, " AND ")
	}
	limitIdx := len(dataArgs) + 1
	dataSQL += fmt.Sprintf(" ORDER BY COALESCE(r.released_at, r.created_at) DESC LIMIT $%d OFFSET $%d", limitIdx, limitIdx+1)
	dataArgs = append(dataArgs, perPage, offset)

	rows, err := s.pool.Query(ctx, dataSQL, dataArgs...)
```

Make sure to add `"strings"` to the imports at the top of pgstore.go if not already present.

- [ ] **Step 4: Update ListReleasesByProject similarly**

Change the function signature on line 429:

```go
func (s *PgStore) ListReleasesByProject(ctx context.Context, projectID string, page, perPage int, includeExcluded bool, filter ReleaseFilter) ([]models.Release, int, error) {
```

Apply the same dynamic WHERE clause building pattern. The key difference: this function already has `WHERE s.project_id = $1`, so the project filter is always the first arg.

- [ ] **Step 5: Update ListReleasesBySource similarly**

Change the function signature and inject filter clauses. Same pattern as above but with `WHERE r.source_id = $1`.

- [ ] **Step 6: Update ListTodos**

Change the function signature on line 1279:

```go
func (s *PgStore) ListTodos(ctx context.Context, status string, page, perPage int, aggregated bool, filter TodoFilter) ([]models.Todo, int, error) {
```

After the existing status WHERE clause building (lines 1281-1286), append the new filter conditions:

```go
	whereClause := ""
	countArgs := []any{}
	if status != "" {
		whereClause = ` WHERE t.status = $1`
		countArgs = append(countArgs, status)
	}

	// Append advanced filter conditions
	filterClauses := []string{}
	nextIdx := len(countArgs) + 1
	if filter.ProjectID != "" {
		filterClauses = append(filterClauses, fmt.Sprintf("COALESCE(p1.id, p2.id)::text = $%d", nextIdx))
		countArgs = append(countArgs, filter.ProjectID)
		nextIdx++
	}
	if filter.Provider != "" {
		filterClauses = append(filterClauses, fmt.Sprintf("COALESCE(src.provider, '') = $%d", nextIdx))
		countArgs = append(countArgs, filter.Provider)
		nextIdx++
	}
	if filter.Urgency != "" {
		filterClauses = append(filterClauses, fmt.Sprintf("COALESCE(sr.report->>'urgency', '') ILIKE $%d", nextIdx))
		countArgs = append(countArgs, filter.Urgency)
		nextIdx++
	}
	if filter.DateFrom != nil {
		filterClauses = append(filterClauses, fmt.Sprintf("t.created_at >= $%d", nextIdx))
		countArgs = append(countArgs, *filter.DateFrom)
		nextIdx++
	}
	if filter.DateTo != nil {
		filterClauses = append(filterClauses, fmt.Sprintf("t.created_at <= $%d", nextIdx))
		countArgs = append(countArgs, *filter.DateTo)
		nextIdx++
	}
	if len(filterClauses) > 0 {
		if whereClause == "" {
			whereClause = " WHERE " + strings.Join(filterClauses, " AND ")
		} else {
			whereClause += " AND " + strings.Join(filterClauses, " AND ")
		}
	}
```

The rest of the function (countQuery, query, pagination, scan) remains unchanged — it already uses the `whereClause` and `countArgs` variables.

- [ ] **Step 7: Verify everything compiles and existing tests pass**

Run: `go test ./internal/api/... -v`
Expected: All existing tests pass

- [ ] **Step 8: Commit**

```bash
git add internal/api/pgstore.go
git commit -m "feat(api): extend pgstore queries with dynamic filter WHERE clauses"
```

---

## Task 5: Update frontend API client

**Files:**
- Modify: `web/lib/api/client.ts`
- Modify: `web/components/dashboard/unified-feed.tsx`

- [ ] **Step 1: Add filter type and update releases.list()**

In `web/lib/api/client.ts`, add a filter interface above the `releases` object and update the methods:

```typescript
// --- Releases ---

export interface ReleaseFilters {
  provider?: string;
  urgency?: string;
  date_from?: string;
  date_to?: string;
  include_excluded?: boolean;
}

function buildReleaseParams(page: number, perPage: number, filters?: ReleaseFilters): string {
  const p = new URLSearchParams({ page: String(page), per_page: String(perPage) });
  if (filters?.include_excluded) p.set("include_excluded", "true");
  if (filters?.provider) p.set("provider", filters.provider);
  if (filters?.urgency) p.set("urgency", filters.urgency);
  if (filters?.date_from) p.set("date_from", filters.date_from);
  if (filters?.date_to) p.set("date_to", filters.date_to);
  return p.toString();
}

export const releases = {
  list: (page = 1, perPage = 25, filters?: ReleaseFilters) =>
    request<ApiResponse<Release[]>>(`/releases?${buildReleaseParams(page, perPage, filters)}`),
  listBySource: (sourceId: string, page = 1, perPage = 25, filters?: ReleaseFilters) =>
    request<ApiResponse<Release[]>>(`/sources/${sourceId}/releases?${buildReleaseParams(page, perPage, filters)}`),
  listByProject: (projectId: string, page = 1, perPage = 25, filters?: ReleaseFilters) =>
    request<ApiResponse<Release[]>>(`/projects/${projectId}/releases?${buildReleaseParams(page, perPage, filters)}`),
  get: (id: string) =>
    request<ApiResponse<Release>>(`/releases/${id}`),
};
```

- [ ] **Step 2: Add TodoFilters and update todos.list()**

```typescript
export interface TodoFilters {
  status?: string;
  project?: string;
  provider?: string;
  urgency?: string;
  date_from?: string;
  date_to?: string;
  aggregated?: boolean;
}

export const todos = {
  list: (page = 1, perPage = 25, filters?: TodoFilters) => {
    const params = new URLSearchParams({ page: String(page), per_page: String(perPage) });
    if (filters?.status) params.set("status", filters.status);
    if (filters?.aggregated) params.set("aggregated", "true");
    if (filters?.project) params.set("project", filters.project);
    if (filters?.provider) params.set("provider", filters.provider);
    if (filters?.urgency) params.set("urgency", filters.urgency);
    if (filters?.date_from) params.set("date_from", filters.date_from);
    if (filters?.date_to) params.set("date_to", filters.date_to);
    return request<ApiResponse<Todo[]>>(`/todos?${params}`);
  },
  // ... rest unchanged
```

- [ ] **Step 3: Update unified-feed.tsx**

In `web/components/dashboard/unified-feed.tsx`, the `releases.list(1, 15)` call (line 49) needs no change — the new signature defaults `filters` to `undefined`, which is equivalent to the old `includeExcluded = false` default.

Verify this by checking that `buildReleaseParams(1, 15, undefined)` produces `page=1&per_page=15` with no extra params.

- [ ] **Step 4: Update all other callers**

Search the codebase for other callers of `releasesApi.list(`, `releasesApi.listByProject(`, and `todosApi.list(` and update them to the new signature. The main callers are in:
- `web/app/releases/page.tsx` (will be fully rewritten in Task 7, but update temporarily for compilation)
- `web/app/todo/page.tsx` (will be fully rewritten in Task 8, but update temporarily for compilation)

For releases page, change:
```typescript
// Old:
releasesApi.listByProject(projectFilter, page, PER_PAGE, showExcluded)
releasesApi.list(page, PER_PAGE, showExcluded)
// New:
releasesApi.listByProject(projectFilter, page, PER_PAGE, { include_excluded: showExcluded })
releasesApi.list(page, PER_PAGE, { include_excluded: showExcluded })
```

For todo page, change:
```typescript
// Old:
todosApi.list(activeTab, page, PER_PAGE, aggregated)
todosApi.list("pending", 1, 1, aggregated)
// New:
todosApi.list(page, PER_PAGE, { status: activeTab, aggregated })
todosApi.list(1, 1, { status: "pending", aggregated })
```

- [ ] **Step 5: Type-check**

Run: `cd web && npx tsc --noEmit`
Expected: No errors

- [ ] **Step 6: Commit**

```bash
git add web/lib/api/client.ts web/components/dashboard/unified-feed.tsx web/app/releases/page.tsx web/app/todo/page.tsx
git commit -m "feat(web): update API client to accept filter objects"
```

---

## Task 6: Create useFilterParams hook

**Files:**
- Create: `web/components/filters/use-filter-params.ts`

- [ ] **Step 1: Create the hook**

```typescript
// web/components/filters/use-filter-params.ts
"use client";

import { useState, useCallback, useEffect } from "react";

/**
 * useFilterParams syncs a Record<string, string> of filter values with URL query params.
 * When filters change, page is reset to 1 and URL is updated via replaceState.
 */
export function useFilterParams(
  defaults?: Record<string, string>
): {
  filters: Record<string, string>;
  setFilters: (next: Record<string, string>) => void;
  page: number;
  setPage: (p: number) => void;
} {
  const [filters, setFiltersState] = useState<Record<string, string>>(() => {
    if (typeof window === "undefined") return defaults ?? {};
    const params = new URLSearchParams(window.location.search);
    const parsed: Record<string, string> = { ...(defaults ?? {}) };
    params.forEach((value, key) => {
      if (key !== "page") {
        parsed[key] = value;
      }
    });
    return parsed;
  });

  const [page, setPageState] = useState<number>(() => {
    if (typeof window === "undefined") return 1;
    const p = new URLSearchParams(window.location.search).get("page");
    return p ? Math.max(1, parseInt(p, 10) || 1) : 1;
  });

  // Sync to URL whenever filters or page change
  useEffect(() => {
    const url = new URL(window.location.href);
    // Clear all existing params
    Array.from(url.searchParams.keys()).forEach((k) =>
      url.searchParams.delete(k)
    );
    // Set filter params (skip empty values and defaults)
    for (const [key, value] of Object.entries(filters)) {
      if (value !== "" && value !== undefined) {
        url.searchParams.set(key, value);
      }
    }
    // Set page if > 1
    if (page > 1) {
      url.searchParams.set("page", String(page));
    }
    window.history.replaceState({}, "", url.toString());
  }, [filters, page]);

  const setFilters = useCallback((next: Record<string, string>) => {
    setFiltersState(next);
    setPageState(1); // reset page on filter change
  }, []);

  const setPage = useCallback((p: number) => {
    setPageState(p);
  }, []);

  return { filters, setFilters, page, setPage };
}
```

- [ ] **Step 2: Type-check**

Run: `cd web && npx tsc --noEmit`
Expected: No errors

- [ ] **Step 3: Commit**

```bash
git add web/components/filters/use-filter-params.ts
git commit -m "feat(web): add useFilterParams hook for URL-synced filter state"
```

---

## Task 7: Create FilterBar component

**Files:**
- Create: `web/components/filters/filter-bar.tsx`

- [ ] **Step 1: Create the component**

```tsx
// web/components/filters/filter-bar.tsx
"use client";

import { useState, useRef, useEffect } from "react";
import { X, Plus, Search, Check } from "lucide-react";
import { useTranslation } from "@/lib/i18n/context";

/* ------------------------------------------------------------------ */
/*  Types                                                              */
/* ------------------------------------------------------------------ */

export interface FilterOption {
  value: string;
  label: string;
}

export interface FilterConfig {
  key: string;
  label: string;
  type: "select" | "boolean" | "date-range";
  options?: FilterOption[];
  defaultValue?: string;
}

export interface FilterBarProps {
  filters: FilterConfig[];
  value: Record<string, string>;
  onChange: (value: Record<string, string>) => void;
}

/* ------------------------------------------------------------------ */
/*  Date presets                                                       */
/* ------------------------------------------------------------------ */

const DATE_PRESETS: FilterOption[] = [
  { value: "7d", label: "Last 7 days" },
  { value: "30d", label: "Last 30 days" },
  { value: "90d", label: "Last 90 days" },
  { value: "1y", label: "Last year" },
];

/** Convert a date preset like "30d" to date_from/date_to params */
export function expandDatePreset(preset: string): {
  date_from: string;
  date_to?: string;
} {
  const now = new Date();
  let from: Date;
  switch (preset) {
    case "7d":
      from = new Date(now.getTime() - 7 * 86400000);
      break;
    case "30d":
      from = new Date(now.getTime() - 30 * 86400000);
      break;
    case "90d":
      from = new Date(now.getTime() - 90 * 86400000);
      break;
    case "1y":
      from = new Date(now.getTime() - 365 * 86400000);
      break;
    default:
      // If it's already a YYYY-MM-DD, treat as custom date_from
      return { date_from: preset };
  }
  return { date_from: from.toISOString().slice(0, 10) };
}

/* ------------------------------------------------------------------ */
/*  Chip                                                               */
/* ------------------------------------------------------------------ */

function Chip({
  label,
  displayValue,
  onRemove,
  onClick,
}: {
  label: string;
  displayValue: string;
  onRemove: () => void;
  onClick: () => void;
}) {
  return (
    <button
      type="button"
      onClick={onClick}
      className="inline-flex items-center gap-1.5 rounded-full border border-border bg-surface-secondary px-2.5 py-1 text-xs transition-colors hover:bg-surface-tertiary"
      style={{ fontFamily: "var(--font-dm-sans)" }}
    >
      <span className="text-text-muted">{label}:</span>
      <span className="text-text-primary">{displayValue}</span>
      <span
        role="button"
        tabIndex={0}
        className="ml-0.5 text-text-muted hover:text-text-primary"
        onClick={(e) => {
          e.stopPropagation();
          onRemove();
        }}
        onKeyDown={(e) => {
          if (e.key === "Enter") {
            e.stopPropagation();
            onRemove();
          }
        }}
      >
        <X size={12} />
      </span>
    </button>
  );
}

/* ------------------------------------------------------------------ */
/*  FilterBar                                                          */
/* ------------------------------------------------------------------ */

export function FilterBar({ filters, value, onChange }: FilterBarProps) {
  const { t } = useTranslation();
  const [popoverOpen, setPopoverOpen] = useState(false);
  const [selectedType, setSelectedType] = useState<string | null>(null);
  const [search, setSearch] = useState("");
  const popoverRef = useRef<HTMLDivElement>(null);

  // Close popover on outside click
  useEffect(() => {
    function handleClick(e: MouseEvent) {
      if (
        popoverRef.current &&
        !popoverRef.current.contains(e.target as Node)
      ) {
        setPopoverOpen(false);
        setSelectedType(null);
        setSearch("");
      }
    }
    if (popoverOpen) {
      document.addEventListener("mousedown", handleClick);
      return () => document.removeEventListener("mousedown", handleClick);
    }
  }, [popoverOpen]);

  // Active chips: filters that have a value set
  const activeFilters = filters.filter(
    (f) => value[f.key] !== undefined && value[f.key] !== ""
  );
  // Available filters for popover: those not currently active
  const availableFilters = filters.filter(
    (f) => value[f.key] === undefined || value[f.key] === ""
  );

  const getDisplayValue = (config: FilterConfig, val: string): string => {
    if (config.type === "boolean") {
      return val === "true" ? "Yes" : "No";
    }
    if (config.type === "date-range") {
      const preset = DATE_PRESETS.find((p) => p.value === val);
      if (preset) return preset.label;
      return val;
    }
    if (config.options) {
      const opt = config.options.find((o) => o.value === val);
      if (opt) return opt.label;
    }
    return val;
  };

  const removeFilter = (key: string) => {
    const next = { ...value };
    delete next[key];
    // Also remove date_from/date_to if removing the date preset
    if (key === "date") {
      delete next["date_from"];
      delete next["date_to"];
    }
    onChange(next);
  };

  const setFilter = (key: string, val: string) => {
    onChange({ ...value, [key]: val });
    setPopoverOpen(false);
    setSelectedType(null);
    setSearch("");
  };

  const openFilterEdit = (key: string) => {
    setSelectedType(key);
    setPopoverOpen(true);
    setSearch("");
  };

  const clearAll = () => {
    onChange({});
  };

  const hasActiveFilters = activeFilters.length > 0;

  // Get the selected config for the popover value picker
  const selectedConfig = selectedType
    ? filters.find((f) => f.key === selectedType)
    : null;

  return (
    <div className="flex flex-wrap items-center gap-2 rounded-lg border border-border bg-surface px-3 py-2">
      {/* Active chips */}
      {activeFilters.map((config) => (
        <Chip
          key={config.key}
          label={config.label}
          displayValue={getDisplayValue(config, value[config.key])}
          onRemove={() => removeFilter(config.key)}
          onClick={() => openFilterEdit(config.key)}
        />
      ))}

      {/* Add filter button + popover */}
      {availableFilters.length > 0 && (
        <div className="relative" ref={popoverRef}>
          <button
            type="button"
            onClick={() => {
              setPopoverOpen(!popoverOpen);
              setSelectedType(null);
              setSearch("");
            }}
            className="inline-flex items-center gap-1 rounded-full border border-dashed border-border px-2.5 py-1 text-xs text-text-muted transition-colors hover:border-border-strong hover:text-text-secondary"
            style={{ fontFamily: "var(--font-dm-sans)" }}
          >
            <Plus size={12} />
            Add filter
          </button>

          {popoverOpen && (
            <div className="absolute left-0 top-full z-50 mt-1 flex overflow-hidden rounded-lg border border-border bg-surface shadow-lg">
              {/* Step 1: Filter type list */}
              {!selectedType && (
                <div className="w-44 py-1">
                  <div className="px-3 py-1.5 text-[10px] uppercase tracking-wider text-text-muted">
                    Filter by
                  </div>
                  {availableFilters.map((config) => (
                    <button
                      key={config.key}
                      type="button"
                      onClick={() => {
                        if (config.type === "boolean") {
                          // Boolean: toggle immediately
                          setFilter(config.key, "true");
                        } else {
                          setSelectedType(config.key);
                        }
                      }}
                      className="flex w-full items-center px-3 py-1.5 text-left text-xs text-text-secondary transition-colors hover:bg-surface-secondary"
                      style={{ fontFamily: "var(--font-dm-sans)" }}
                    >
                      {config.label}
                    </button>
                  ))}
                </div>
              )}

              {/* Step 2: Value picker */}
              {selectedConfig && selectedConfig.type === "select" && (
                <div className="w-52 py-1">
                  {/* Search box */}
                  <div className="px-2 pb-1">
                    <div className="flex items-center gap-1.5 rounded border border-border bg-background px-2 py-1">
                      <Search size={12} className="text-text-muted" />
                      <input
                        type="text"
                        value={search}
                        onChange={(e) => setSearch(e.target.value)}
                        placeholder={`Search ${selectedConfig.label.toLowerCase()}...`}
                        className="w-full bg-transparent text-xs text-text-primary outline-none placeholder:text-text-muted"
                        autoFocus
                      />
                    </div>
                  </div>
                  {/* Options */}
                  <div className="max-h-48 overflow-y-auto">
                    {(selectedConfig.options ?? [])
                      .filter((opt) =>
                        opt.label.toLowerCase().includes(search.toLowerCase())
                      )
                      .map((opt) => {
                        const isActive = value[selectedConfig.key] === opt.value;
                        return (
                          <button
                            key={opt.value}
                            type="button"
                            onClick={() =>
                              setFilter(selectedConfig.key, opt.value)
                            }
                            className="flex w-full items-center gap-2 px-3 py-1.5 text-left text-xs transition-colors hover:bg-surface-secondary"
                            style={{ fontFamily: "var(--font-dm-sans)" }}
                          >
                            <span
                              className={
                                isActive
                                  ? "text-text-primary font-medium"
                                  : "text-text-secondary"
                              }
                            >
                              {opt.label}
                            </span>
                            {isActive && (
                              <Check
                                size={12}
                                className="ml-auto text-accent"
                              />
                            )}
                          </button>
                        );
                      })}
                  </div>
                </div>
              )}

              {/* Step 2: Date range picker */}
              {selectedConfig && selectedConfig.type === "date-range" && (
                <div className="w-52 py-1">
                  <div className="px-3 py-1.5 text-[10px] uppercase tracking-wider text-text-muted">
                    Date range
                  </div>
                  {DATE_PRESETS.map((preset) => (
                    <button
                      key={preset.value}
                      type="button"
                      onClick={() => setFilter("date", preset.value)}
                      className="flex w-full items-center px-3 py-1.5 text-left text-xs text-text-secondary transition-colors hover:bg-surface-secondary"
                      style={{ fontFamily: "var(--font-dm-sans)" }}
                    >
                      {preset.label}
                    </button>
                  ))}
                </div>
              )}
            </div>
          )}
        </div>
      )}

      {/* Clear all */}
      {hasActiveFilters && (
        <button
          type="button"
          onClick={clearAll}
          className="ml-auto text-[11px] text-text-muted transition-colors hover:text-text-secondary"
          style={{ fontFamily: "var(--font-dm-sans)" }}
        >
          Clear all
        </button>
      )}
    </div>
  );
}
```

- [ ] **Step 2: Type-check**

Run: `cd web && npx tsc --noEmit`
Expected: No errors

- [ ] **Step 3: Commit**

```bash
git add web/components/filters/filter-bar.tsx
git commit -m "feat(web): add FilterBar component with chip-based UI and popover"
```

---

## Task 8: Integrate FilterBar into releases page

**Files:**
- Modify: `web/app/releases/page.tsx`

- [ ] **Step 1: Replace inline filters with FilterBar**

In `web/app/releases/page.tsx`, make these changes:

1. Add imports:
```typescript
import { FilterBar, FilterConfig, expandDatePreset } from "@/components/filters/filter-bar";
import { useFilterParams } from "@/components/filters/use-filter-params";
import type { ReleaseFilters } from "@/lib/api/client";
```

2. Replace the filter state variables (lines 57-62) and URL sync logic with:
```typescript
const { filters, setFilters, page, setPage } = useFilterParams();
```

3. Build the filter config:
```typescript
const filterConfig: FilterConfig[] = [
  {
    key: "project",
    label: "Project",
    type: "select",
    options: (projectsData?.data ?? []).map((p) => ({
      value: p.id,
      label: p.name,
    })),
  },
  {
    key: "provider",
    label: "Provider",
    type: "select",
    options: [
      { value: "github", label: "GitHub" },
      { value: "dockerhub", label: "Docker Hub" },
      { value: "ecr-public", label: "ECR Public" },
      { value: "gitlab", label: "GitLab" },
      { value: "pypi", label: "PyPI" },
      { value: "npm", label: "npm" },
    ],
  },
  {
    key: "urgency",
    label: "Urgency",
    type: "select",
    options: [
      { value: "critical", label: "Critical" },
      { value: "high", label: "High" },
      { value: "medium", label: "Medium" },
      { value: "low", label: "Low" },
    ],
  },
  { key: "date", label: "Date", type: "date-range" },
  { key: "excluded", label: "Show excluded", type: "boolean" },
];
```

4. Convert the `filters` record into API call params:
```typescript
const apiFilters: ReleaseFilters = {
  include_excluded: filters.excluded === "true",
  provider: filters.provider,
  urgency: filters.urgency,
};
if (filters.date) {
  const expanded = expandDatePreset(filters.date);
  apiFilters.date_from = expanded.date_from;
  apiFilters.date_to = expanded.date_to;
}
```

5. Update SWR call to use new params:
```typescript
const { data, isLoading } = useSWR(
  filters.project
    ? ["releases", page, filters]
    : ["all-releases", page, filters],
  () =>
    filters.project
      ? releasesApi.listByProject(filters.project, page, PER_PAGE, apiFilters)
      : releasesApi.list(page, PER_PAGE, apiFilters),
  { refreshInterval: 30_000 }
);
```

6. Replace the inline filter UI (the project dropdown and show_excluded toggle) with:
```tsx
<FilterBar filters={filterConfig} value={filters} onChange={setFilters} />
```

7. Remove the old URL sync `useEffect` and manual `setProject`/`setShowExcluded` handlers.

8. Update pagination to use `page`/`setPage` from the hook.

- [ ] **Step 2: Type-check and test in browser**

Run: `cd web && npx tsc --noEmit`
Expected: No errors

- [ ] **Step 3: Commit**

```bash
git add web/app/releases/page.tsx
git commit -m "feat(web): integrate FilterBar into releases page"
```

---

## Task 9: Integrate FilterBar into todo page

**Files:**
- Modify: `web/app/todo/page.tsx`

- [ ] **Step 1: Replace inline filters with FilterBar**

Same pattern as Task 8 but for the todo page:

1. Add imports:
```typescript
import { FilterBar, FilterConfig, expandDatePreset } from "@/components/filters/filter-bar";
import { useFilterParams } from "@/components/filters/use-filter-params";
import type { TodoFilters } from "@/lib/api/client";
```

2. Replace state with:
```typescript
const { filters, setFilters, page, setPage } = useFilterParams({
  status: "pending",
});
```

3. Build filter config:
```typescript
const filterConfig: FilterConfig[] = [
  {
    key: "status",
    label: "Status",
    type: "select",
    options: [
      { value: "pending", label: "Pending" },
      { value: "acknowledged", label: "Acknowledged" },
      { value: "resolved", label: "Resolved" },
    ],
  },
  {
    key: "project",
    label: "Project",
    type: "select",
    options: (projectsData?.data ?? []).map((p) => ({
      value: p.id,
      label: p.name,
    })),
  },
  {
    key: "provider",
    label: "Provider",
    type: "select",
    options: [
      { value: "github", label: "GitHub" },
      { value: "dockerhub", label: "Docker Hub" },
      { value: "ecr-public", label: "ECR Public" },
      { value: "gitlab", label: "GitLab" },
      { value: "pypi", label: "PyPI" },
      { value: "npm", label: "npm" },
    ],
  },
  {
    key: "urgency",
    label: "Urgency",
    type: "select",
    options: [
      { value: "critical", label: "Critical" },
      { value: "high", label: "High" },
      { value: "medium", label: "Medium" },
      { value: "low", label: "Low" },
    ],
  },
  { key: "date", label: "Date", type: "date-range" },
  { key: "aggregated", label: "Latest Only", type: "boolean" },
];
```

4. Convert to API call params:
```typescript
const apiFilters: TodoFilters = {
  status: filters.status,
  aggregated: filters.aggregated === "true",
  project: filters.project,
  provider: filters.provider,
  urgency: filters.urgency,
};
if (filters.date) {
  const expanded = expandDatePreset(filters.date);
  apiFilters.date_from = expanded.date_from;
  apiFilters.date_to = expanded.date_to;
}
```

5. Update SWR call:
```typescript
const { data, isLoading } = useSWR(
  ["todos", page, filters],
  () => todosApi.list(page, PER_PAGE, apiFilters),
  { refreshInterval: 30_000 }
);
```

6. For tab count fetches, update to use the new signature:
```typescript
const { data: pendingCount } = useSWR(
  ["todos-count", "pending", filters],
  () => todosApi.list(1, 1, { ...apiFilters, status: "pending" })
);
const { data: ackCount } = useSWR(
  ["todos-count", "acknowledged", filters],
  () => todosApi.list(1, 1, { ...apiFilters, status: "acknowledged" })
);
const { data: resolvedCount } = useSWR(
  ["todos-count", "resolved", filters],
  () => todosApi.list(1, 1, { ...apiFilters, status: "resolved" })
);
```

7. Replace the inline status tabs and aggregated toggle with `<FilterBar>`.

8. Update pagination to use `page`/`setPage` from the hook.

- [ ] **Step 2: Type-check**

Run: `cd web && npx tsc --noEmit`
Expected: No errors

- [ ] **Step 3: Commit**

```bash
git add web/app/todo/page.tsx
git commit -m "feat(web): integrate FilterBar into todo page"
```

---

## Task 10: End-to-end verification

**Files:** None (verification only)

- [ ] **Step 1: Run Go tests**

Run: `go test ./internal/api/... -v`
Expected: All tests pass

- [ ] **Step 2: Run Go vet**

Run: `go vet ./...`
Expected: No issues

- [ ] **Step 3: Type-check frontend**

Run: `cd web && npx tsc --noEmit`
Expected: No errors

- [ ] **Step 4: Manual smoke test (if dev server available)**

Start dev: `make dev` (in one terminal) and `make frontend-dev` (in another)

Test these scenarios:
1. Open `/releases` — filter bar shows, add a project filter chip, verify URL updates
2. Add provider filter — verify releases list narrows
3. Add date range (30d) — verify URL shows `date_from`
4. Clear all — verify all chips removed and full list returns
5. Open `/todo` — filter bar shows with status=pending default chip
6. Add urgency filter — verify list narrows
7. Refresh page — verify filters persist from URL

- [ ] **Step 5: Commit any fixes**

```bash
git add -A
git commit -m "fix: address issues found during e2e verification"
```
