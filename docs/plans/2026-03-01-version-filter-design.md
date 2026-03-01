# Version Filter for Source Releases

**Date:** 2026-03-01
**Status:** Approved

## Problem

When fetching/viewing releases from a source, users need to filter out irrelevant version patterns (e.g., pre-releases, nightly builds, specific version ranges). Currently all releases are shown and all trigger notifications regardless of relevance.

## Design

### Data Model

Add two nullable TEXT columns to the `sources` table:

| Column | Type | Default | Description |
|--------|------|---------|-------------|
| `version_filter_include` | `TEXT` | `NULL` | Regex — only versions matching this are shown |
| `version_filter_exclude` | `TEXT` | `NULL` | Regex — versions matching this are hidden |

**Filter precedence:** Include is applied first (whitelist), then Exclude (blacklist). Both are optional — when NULL, no filtering is applied for that dimension.

### DB Query Changes

All release listing queries (`ListAllReleases`, `ListReleasesBySource`, `ListReleasesByProject`) JOIN with `sources` and apply conditional WHERE clauses using PostgreSQL `~` regex operator:

```sql
-- When source.version_filter_include IS NOT NULL:
AND r.version ~ s.version_filter_include
-- When source.version_filter_exclude IS NOT NULL:
AND r.version !~ s.version_filter_exclude
```

Both the count query and data query must include these filters for correct pagination.

### Notification Worker Changes

In `routing/worker.go`, after fetching the release and its source, check whether the release version passes the source's filters. If it doesn't, skip notification delivery and agent rule checks entirely.

### API Changes

Existing `POST/PUT /sources` endpoints accept new fields. `GET` responses include them.

### Frontend Changes

Add "Version Filters" section to source form (`source-form.tsx`) with:
- "Include Pattern" text input (regex, optional)
- "Exclude Pattern" text input (regex, optional)

### Migration

Add migration step to ALTER the sources table with the two new columns.

## Examples

- Exclude pre-releases: set `version_filter_exclude` = `-(alpha|beta|rc|dev|pre|snapshot|nightly)`
- Only semver: set `version_filter_include` = `^v?\d+\.\d+\.\d+$`
- Exclude v1.x: set `version_filter_exclude` = `^v?1\.`
