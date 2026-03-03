# Show Excluded Releases Design

## Goal

Show all releases for sources in projects, even when filtered out by source version filters. Excluded releases appear grayed out. The releases page has a URL-persisted toggle to show/hide excluded versions.

## Scope

- **Releases page (`/releases`)**: Toggle (default on) to show excluded versions as gray rows. State persisted in URL as `?show_excluded=true|false`.
- **Projects page**: Recent releases in flow cards always show all releases, with excluded ones grayed out. No toggle.

## Backend Changes

### Model

Add `Excluded bool` field to `Release` struct:

```go
Excluded bool `json:"excluded"` // true when filtered by source version filters
```

### SQL Queries

All three list methods (`ListAllReleases`, `ListReleasesBySource`, `ListReleasesByProject`) gain an `includeExcluded bool` parameter.

When `includeExcluded=true`:
- Remove version filter WHERE clauses
- Add CASE expression to compute `excluded`:

```sql
CASE WHEN
  (s.version_filter_include IS NOT NULL AND r.version !~ s.version_filter_include)
  OR (s.version_filter_exclude IS NOT NULL AND r.version ~ s.version_filter_exclude)
  OR (s.exclude_prereleases = true AND r.raw_data->>'prerelease' = 'true')
THEN true ELSE false END AS excluded
```

When `includeExcluded=false` (default): existing behavior, no `excluded` field computation.

### API

Query parameter `include_excluded=true` on:
- `GET /releases`
- `GET /sources/{id}/releases`
- `GET /projects/{projectId}/releases`

Backward compatible — omitting the param gives current behavior.

### Store Interface

```go
ListAllReleases(ctx context.Context, page, perPage int, includeExcluded bool) ([]models.Release, int, error)
ListReleasesBySource(ctx context.Context, sourceID string, page, perPage int, includeExcluded bool) ([]models.Release, int, error)
ListReleasesByProject(ctx context.Context, projectID string, page, perPage int, includeExcluded bool) ([]models.Release, int, error)
```

## Frontend Changes

### TypeScript Types

```typescript
export interface Release {
  // ... existing fields ...
  excluded?: boolean;
}
```

### API Client

Add optional `includeExcluded` parameter to list methods.

### Releases Page

- URL param: `show_excluded` (default `true`)
- Toggle switch next to project filter dropdown
- When `show_excluded=true`: fetch with `include_excluded=true`, render excluded rows with muted gray styling
- When `show_excluded=false`: fetch without the param (backend filters as today)
- Gray styling: muted text color (`#c4c4c0`), muted version chip, no hover highlight

### Projects Page

- Always fetch releases with `include_excluded=true`
- Excluded version chips rendered in muted gray (`#c4c4c0`) instead of normal blue
- No toggle — excluded always visible as gray
