# GitLab Provider Design

**Date:** 2026-03-01

## Summary

Add GitLab as a new source provider, polling the GitLab REST API v4 for repository releases. Follows the same pattern as the existing GitHub provider.

## Scope

- **gitlab.com only** — no self-hosted support initially
- **Auth**: `GITLAB_TOKEN` env var → `PRIVATE-TOKEN` header (optional, public repos work without it)
- **Exclude prereleases**: supported via GitLab's `upcoming_release` field

## API

- **Endpoint**: `GET https://gitlab.com/api/v4/projects/:id/releases`
- **Repository format**: `owner/repo` — URL-encoded as `owner%2Frepo` in the API path
- **Pagination**: `per_page=20`, offset-based
- **Auth header**: `PRIVATE-TOKEN: <token>`

## Field Mapping

| GitLab API field   | IngestionResult field        |
|--------------------|------------------------------|
| `tag_name`         | `RawVersion`                 |
| `description`      | `Changelog`                  |
| `released_at`      | `Timestamp`                  |
| `_links.self`      | `Metadata["release_url"]`    |
| `upcoming_release` | `Metadata["prerelease"]`     |

## Files Changed

1. `internal/ingestion/gitlab.go` — New `GitLabSource` implementing `IIngestionSource`
2. `internal/ingestion/gitlab_test.go` — Unit tests with mock HTTP server
3. `internal/ingestion/loader.go` — Add `"gitlab"` case to `BuildSource()` switch
4. `internal/api/providers.go` — Add `"gitlab"` to static provider list
5. `web/components/sources/source-form.tsx` — Add "GitLab" to provider dropdown, show `exclude_prereleases` for gitlab
6. `web/lib/format.ts` — Add GitLab to repository validation

## What Doesn't Change

- Database schema (provider is a string column, no migration needed)
- API endpoints (sources API is provider-agnostic)
- Orchestrator/polling logic (loads sources dynamically)
- Notification flow (works on any source type)
