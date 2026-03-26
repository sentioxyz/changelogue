# Global Advanced Filter Component

## Overview

Replace ad-hoc inline filters on the releases and todo pages with a shared, chip-based `<FilterBar>` component. Each active filter displays as a removable chip. Clicking "+ Add filter" opens a two-step popover (pick type → pick value). All filter state syncs to URL query params for shareability and back/forward navigation.

## Filter Fields

### Shared filters (available on both pages)

| Key | Label | Type | Options |
|-----|-------|------|---------|
| `project` | Project | `select` | Fetched from `GET /projects` |
| `provider` | Provider | `select` | Static: github, dockerhub, ecr-public, gitlab, pypi, npm |
| `urgency` | Urgency | `select` | Static: critical, high, medium, low |
| `date` | Date | `date-range` | Presets: 7d, 30d, 90d, 1y; Custom: date_from/date_to |

### Releases-only filters

| Key | Label | Type | Default |
|-----|-------|------|---------|
| `excluded` | Excluded | `boolean` | `"false"` |

### Todos-only filters

| Key | Label | Type | Default |
|-----|-------|------|---------|
| `status` | Status | `select` | `"pending"` |
| `aggregated` | Latest Only | `boolean` | `"false"` |

## Component API

### `<FilterBar>`

```tsx
interface FilterConfig {
  key: string;
  label: string;
  type: "select" | "boolean" | "date-range";
  options?: { value: string; label: string }[];  // for "select" type
  defaultValue?: string;                          // value when chip is not present
}

interface FilterBarProps {
  filters: FilterConfig[];
  value: Record<string, string>;
  onChange: (value: Record<string, string>) => void;
}
```

**Rendering rules:**
- Each entry in `value` with a non-empty string renders as a chip: `[Label: Value] ×`
- Boolean filters render as `[Label: shown]` / `[Label: hidden]` or just `[Label]` when active
- Date range presets render as `[Date: Last 30 days]`; custom renders as `[Date: Jan 1 – Mar 26]`
- Filters not yet in `value` appear in the "+ Add filter" popover
- "Clear all" removes all entries from `value` (resets to no filters)

### `useFilterParams()`

```tsx
function useFilterParams(
  defaults?: Record<string, string>
): [Record<string, string>, (next: Record<string, string>) => void]
```

- Reads initial values from `window.location.search` on mount
- Merges with `defaults` (URL values take precedence)
- On change, writes to URL via `window.history.replaceState`
- Resets `page` param to `1` whenever any filter value changes

### URL query format

```
/releases?project=abc123&provider=github&date_from=2026-03-01&date_to=2026-03-26&excluded=true
/todo?status=pending&project=abc123&urgency=high&date_from=2026-03-19
```

Date preset conversion (frontend-only, backend receives absolute dates):
- `7d` → `date_from=<today-7d>` (no `date_to`)
- `30d` → `date_from=<today-30d>`
- `90d` → `date_from=<today-90d>`
- `1y` → `date_from=<today-1y>`
- Custom → `date_from=YYYY-MM-DD&date_to=YYYY-MM-DD`

## Backend Changes

### Releases endpoint — new query params

Extend `GET /api/v1/releases` and `GET /api/v1/projects/{projectId}/releases`:

| Param | Type | SQL effect |
|-------|------|------------|
| `provider` | string | `WHERE s.provider = $N` |
| `urgency` | string | `WHERE sr_info.urgency ILIKE $N` |
| `date_from` | RFC 3339 date | `WHERE COALESCE(r.released_at, r.created_at) >= $N` |
| `date_to` | RFC 3339 date | `WHERE COALESCE(r.released_at, r.created_at) <= $N` |

All optional. Omitting returns unfiltered results (backward compatible).

### Todos endpoint — new query params

Extend `GET /api/v1/todos`:

| Param | Type | SQL effect |
|-------|------|------------|
| `project` | UUID | `WHERE p.id = $N` (via join to projects) |
| `provider` | string | `WHERE s.provider = $N` (via join to sources) |
| `urgency` | string | `WHERE sr.report->>'urgency' ILIKE $N` |
| `date_from` | RFC 3339 date | `WHERE t.created_at >= $N` |
| `date_to` | RFC 3339 date | `WHERE t.created_at <= $N` |

All optional. Existing `status` and `aggregated` params remain unchanged.

### API client changes

Update `web/lib/api/client.ts`:

```tsx
// Before: positional args
releases.list(page, perPage, includeExcluded)
todos.list(status, page, perPage, aggregated)

// After: filter object
interface ReleaseFilters {
  project?: string;
  provider?: string;
  urgency?: string;
  date_from?: string;
  date_to?: string;
  include_excluded?: boolean;
}
releases.list(page, perPage, filters: ReleaseFilters)

interface TodoFilters {
  status?: string;
  project?: string;
  provider?: string;
  urgency?: string;
  date_from?: string;
  date_to?: string;
  aggregated?: boolean;
}
todos.list(page, perPage, filters: TodoFilters)
```

## File Structure

### New files
- `web/components/filters/filter-bar.tsx` — `<FilterBar>` component
- `web/components/filters/use-filter-params.ts` — `useFilterParams()` hook

### Modified files
- `web/app/releases/page.tsx` — replace inline filters with `<FilterBar>`
- `web/app/todo/page.tsx` — replace inline tabs/toggles with `<FilterBar>`
- `web/lib/api/client.ts` — update `releases.list()` and `todos.list()` signatures
- `web/components/dashboard/unified-feed.tsx` — update `releases.list()` call to match new signature
- `internal/api/releases.go` — parse new query params, pass to store
- `internal/api/todos.go` — parse new query params, pass to store
- `internal/api/pgstore.go` — extend queries with dynamic WHERE clauses

### No new files needed
- No new database migrations (all filtering uses existing columns)
- No new API endpoints

## Data Flow

```
URL query string
  ↓ useFilterParams() reads on mount
Filter state (Record<string, string>)
  ↓ FilterBar renders chips + popover
  ↓ onChange → updates state + replaceState URL
  ↓ page passes filter values to API call
API client builds query string from filter object
  ↓ GET /releases?project=x&provider=github&date_from=...
Backend handler parses query params
  ↓ passes to PgStore method
PgStore builds dynamic WHERE clauses
  ↓ returns filtered, paginated results
```

## UI Behavior

### Add filter popover
1. Click "+ Add filter" → popover opens with list of available filter types (those not already active)
2. Click a filter type → second panel shows value picker
   - `select`: searchable list of options
   - `boolean`: immediate toggle (adds chip, closes popover)
   - `date-range`: preset buttons (7d, 30d, 90d, 1y) + custom date inputs
3. Select a value → chip appears, popover closes

### Chip interactions
- Click `×` on a chip → removes that filter
- "Clear all" → removes all filters, resets to defaults
- Clicking an existing chip → re-opens the value picker for that filter type

### Pagination reset
- Any filter change resets page to 1
- Page number is also synced to URL (`?page=2`)

## Testing

- `useFilterParams` hook: unit test URL read/write and default merging
- `FilterBar` component: test chip rendering, add/remove, clear all
- Backend: test each new query param individually and in combination
- Integration: verify URL → API → filtered results round-trip
