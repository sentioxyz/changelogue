---
name: adding-a-provider
description: Use when adding a new ingestion provider (e.g. GitLab, Bitbucket, Artifactory). Checklist of every file that needs a new provider entry.
---

# Adding a Provider

## Overview

Every provider must be registered in **10 locations** across backend, frontend, and tests. Missing any one causes silent failures or missing UI elements.

## Checklist

### Backend (Go)

1. **`internal/ingestion/<provider>.go`** — Create. Implement `IIngestionSource` interface (`Name()`, `SourceID()`, `FetchNewReleases()`). Follow `github.go` as template.

2. **`internal/ingestion/<provider>_test.go`** — Create. Mock HTTP server tests: Name, FetchNewReleases, FetchEmpty, FetchHTTPError. Follow `github_test.go` as template.

3. **`internal/ingestion/loader.go`** — Add `case "<provider>"` to `BuildSource()` switch.

4. **`internal/api/providers.go`** — Add `{"id": "<provider>", "name": "<Display Name>", "type": "polling"}` to the providers slice.

5. **`internal/api/health_test.go`** — Update `TestProvidersHandlerList`: bump expected count and add assertions for new provider.

### Frontend (TypeScript/React)

6. **`web/components/sources/source-form.tsx`** — Add `<SelectItem>` to provider dropdown. If provider supports prereleases, add to the `exclude_prereleases` condition.

7. **`web/components/projects/project-form.tsx`** — **EASY TO MISS.** Same changes as source-form.tsx — this is a *separate inline form* used during project creation.

8. **`web/components/ui/provider-badge.tsx`** — Add entry to `PROVIDER_STYLES` with brand color and icon (check `react-icons/fa` for available icons).

9. **`web/components/releases/release-detail.tsx`** — Add cases to both `getProviderUrl()` and `getProviderLabel()`.

10. **`web/lib/format.ts`** — Add URL validation rule to `validateRepository()` rejecting full URLs for the new provider.

### Verification

- `go test ./...` — all pass
- `go vet ./...` — clean
- Check both the "New Project" dialog source form AND the standalone "Add Source" form show the new provider
