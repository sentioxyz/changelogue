# Source UX Improvements Design

## Summary

Three improvements to the source creation flow:
1. Auto-strip URL prefixes from repository input (e.g. `github.com/ethereum/go-ethereum` → `ethereum/go-ethereum`)
2. Change default poll interval to 1 day (86400s)
3. Trigger immediate poll after source creation via `POST /api/v1/sources/{id}/poll`

## 1. Auto-strip Repository URL Prefixes

**Location**: `web/lib/format.ts` (shared), consumed by all source forms

Normalize repository input on submit based on selected provider:
- **GitHub**: Strip `https://github.com/`, `http://github.com/`, `github.com/`; strip trailing `.git`
- **Docker Hub**: Strip `https://hub.docker.com/r/`, `hub.docker.com/r/`, `hub.docker.com/_/`
- Strip trailing slashes from result

Silent normalization — no error shown, input value updated to cleaned form.

## 2. Default Poll Interval → 1 Day

- **Frontend**: Default `pollInterval` state: `300` → `86400` (in all forms)
- **Database migration**: `poll_interval_seconds` default: `900` → `86400`

## 3. Poll on Source Creation

### Backend: `POST /api/v1/sources/{id}/poll`

Loads the source, polls upstream, processes results via ingestion service, returns count of new releases.

Response: `{ "data": { "new_releases": 5 } }`

**Wiring**: `SourcesHandler` gains references to `ingestion.Service` and `*http.Client`. These are added to `Dependencies` struct and passed through `RegisterRoutes`.

### Frontend

After `sources.create()` succeeds, call `sources.poll(id)` fire-and-forget. Add `poll` method to the API client.

## Files Changed

- `web/lib/format.ts` — shared `normalizeRepository` function
- `web/components/sources/source-form.tsx` — import normalization + default interval
- `web/components/projects/project-form.tsx` — normalization + default interval
- `web/app/projects/page.tsx` — normalization + poll trigger + default interval
- `web/components/projects/project-detail.tsx` — poll trigger after source creation
- `web/lib/api/client.ts` — `poll` method
- `internal/api/sources.go` — `FetchReleases` handler + updated constructor
- `internal/api/sources_test.go` — updated constructor call
- `internal/api/server.go` — route + dependencies
- `internal/ingestion/loader.go` — exported `BuildSource`
- `internal/db/migrations.go` — default interval
- `cmd/server/main.go` — pass ingestion deps
