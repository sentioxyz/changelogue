# GitHub Repo Onboarding — Dependency Scanner

**Issue**: [#7 — Be able to connect to a GitHub repo and detect dependencies it has](https://github.com/sentioxyz/changelogue/issues/7)

**Date**: 2026-03-14

## Overview

Add a "Quick Onboard" feature that lets users point Changelogue at a GitHub repository, automatically detect its dependencies using an LLM, and selectively create tracked projects/sources for those dependencies.

## Decisions

| Decision | Choice | Rationale |
|----------|--------|-----------|
| Ecosystem scope | Multi-ecosystem via LLM | Gemini parses any manifest format without per-ecosystem parsers |
| Repo access | GitHub API | Lighter than cloning; fetches only needed files |
| Project mapping | User picks per dependency | Flexible — user chooses existing project or creates new |
| LLM integration | Direct Gemini `genai.GenerateContent` call | Single-turn structured output — ADK-Go runner/session overhead is unnecessary |
| UX entry point | Dedicated `/onboard` page | Clear onboarding flow, separate from project management |
| Backend pattern | Async River job | Handles large repos; matches existing architecture |

## Architecture

### Data Flow

```
User enters repo URL
  → POST /api/v1/onboard/scan
    → River job: scan_dependencies
      → GitHub API: fetch repo tree (using GITHUB_TOKEN from env if set)
      → GitHub API: fetch dependency file contents
      → Gemini genai.GenerateContent: parse dependencies (structured JSON output)
      → Store results in onboard_scans table
      → pg NOTIFY on release_events channel with scan_complete event
    → Frontend polls GET /api/v1/onboard/scans/{id} for results
  → User selects dependencies to track
  → POST /api/v1/onboard/scans/{id}/apply
    → In a single DB transaction: create projects + sources for selections
    → Trigger first poll for each source via IngestionService
```

### New Database Table

```sql
CREATE TABLE onboard_scans (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    repo_url TEXT NOT NULL,
    status VARCHAR(50) NOT NULL DEFAULT 'pending',
    results JSONB,
    error TEXT,
    created_at TIMESTAMPTZ DEFAULT NOW(),
    started_at TIMESTAMPTZ,
    completed_at TIMESTAMPTZ
);
```

`results` schema (JSONB array):
```json
[
  {
    "name": "gorilla/mux",
    "version": "v1.8.1",
    "ecosystem": "go",
    "upstream_repo": "github.com/gorilla/mux",
    "provider": "github"
  }
]
```

### New API Endpoints

**`POST /api/v1/onboard/scan`**
- Request: `{ "repo_url": "https://github.com/org/repo" }` (or `"org/repo"`)
- Validates: repo URL format, checks no active scan for same repo exists (status = pending/processing)
- Response: `{ "data": { "id": "uuid", "status": "pending", "repo_url": "org/repo" } }`

**`GET /api/v1/onboard/scans/{id}`**
- Response: `{ "data": { "id": "uuid", "status": "completed", "repo_url": "...", "results": [...], "completed_at": "..." } }`

**`POST /api/v1/onboard/scans/{id}/apply`**
- Request:
```json
{
  "selections": [
    {
      "dep_name": "gorilla/mux",
      "upstream_repo": "github.com/gorilla/mux",
      "provider": "github",
      "project_id": "existing-uuid-or-null",
      "new_project_name": "gorilla-mux"
    }
  ]
}
```
- Validation: exactly one of `project_id` or `new_project_name` must be set per selection. If `new_project_name` conflicts with an existing project name, return 409 with the conflicting name.
- All project/source creation happens in a **single database transaction** — if any creation fails, the entire batch rolls back.
- After successful commit, triggers first poll for each created source via `IngestionService.PollSource()`.
- Response: `{ "data": { "created_projects": [...], "created_sources": [...], "skipped": [...] } }`
- `skipped` includes entries where source already exists (UNIQUE constraint on provider+repository).

### OnboardStore Interface

```go
type OnboardStore interface {
    CreateOnboardScan(ctx context.Context, repoURL string) (*models.OnboardScan, error)
    GetOnboardScan(ctx context.Context, id string) (*models.OnboardScan, error)
    UpdateOnboardScanStatus(ctx context.Context, id string, status string, results json.RawMessage, scanErr string) error
    ApplyOnboardScan(ctx context.Context, scanID string, selections []OnboardSelection) (*OnboardApplyResult, error)
}
```

`ApplyOnboardScan` operates within a single transaction.

### New River Job

Added to existing `internal/queue/jobs.go`:

```go
type ScanDependenciesJobArgs struct {
    ScanID string `json:"scan_id"`
}

func (ScanDependenciesJobArgs) Kind() string { return "scan_dependencies" }
```

Worker steps:
1. Load scan record from DB to get `repo_url`
2. Update scan status to `processing`, set `started_at`
3. Parse repo owner/name from `repo_url`
4. Fetch repo tree via `GET /repos/{owner}/{repo}/git/trees/{default_branch}?recursive=1` (uses `GITHUB_TOKEN` from env for auth, same pattern as `internal/ingestion/github.go`)
5. Filter tree entries for known dependency file patterns (go.mod, package.json, requirements.txt, Cargo.toml, pyproject.toml, Gemfile, pom.xml, build.gradle, etc.)
6. Fetch contents of matched files via `GET /repos/{owner}/{repo}/contents/{path}` (same `GITHUB_TOKEN` auth)
7. Call Gemini via direct `genai.GenerateContent` with file contents (see LLM section below)
8. Parse structured JSON response, store in `onboard_scans.results`
9. Update status to `completed`, set `completed_at`
10. Send `pg_notify('release_events', ...)` with scan_complete event

### GitHub Authentication

Uses the same `GITHUB_TOKEN` environment variable as the existing `GitHubSource` in `internal/ingestion/github.go`. All GitHub API requests (tree, contents) set `Authorization: Bearer $GITHUB_TOKEN` when the env var is present. No per-scan token support — uses server-wide token.

### LLM Integration — Direct Gemini Call

Uses `genai.GenerateContent` directly (not ADK-Go runner/session). This is a single-turn structured output extraction — ADK-Go's agent orchestration adds unnecessary overhead.

Reuses model configuration from `internal/agent/model.go` for client setup and model selection.

**Prompt**:
```
You are a dependency extraction agent. Given the contents of dependency/manifest
files from a software project, extract all dependencies.

For each dependency, return:
- name: the package/library name
- version: the version constraint or pinned version
- ecosystem: one of "go", "npm", "pypi", "cargo", "rubygems", "maven", "gradle", "docker", "other"
- upstream_repo: your best guess at the canonical GitHub repository URL (e.g., "github.com/gorilla/mux")
- provider: the Changelogue provider to use for release tracking. Use "github" for repos
  with GitHub releases, "dockerhub" for Docker images, "gitlab" for GitLab repos,
  "ecr_public" for AWS ECR images. When unsure, default to "github".

Return ONLY a JSON array. No explanations.
```

### SSE / Real-Time Updates

Reuses the existing `release_events` PostgreSQL NOTIFY channel. The scan worker sends a notification with event type `scan_complete` and the scan ID as payload. The frontend on the onboard page polls `GET /api/v1/onboard/scans/{id}` on an interval (2s) while status is `pending` or `processing` — simpler than wiring up SSE for a one-off wait.

## Frontend

### New Page: `/onboard`

**Sidebar entry**: "Quick Onboard" — placed after "Dashboard" and before "Projects" in the sidebar.

**Step 1 — Input**: Text field for GitHub repo URL + "Scan" button.

**Step 2 — Scanning**: Loading state with spinner. Polls `GET /api/v1/onboard/scans/{id}` every 2 seconds until status changes.

**Step 3 — Results**: Table with columns:
- Checkbox (select/deselect, all selected by default)
- Dependency name
- Version (detected)
- Ecosystem badge (Go, npm, Python, etc.)
- Upstream source (GitHub repo URL)
- Project assignment: dropdown — "Create new project" (default, uses dep name) or select existing project

**Step 4 — Apply**: "Track Selected" button. Shows success summary with links to created projects. Shows skipped items (duplicates) with explanation.

### Frontend API Client Additions

```typescript
export const onboard = {
  scan: (repoUrl: string) =>
    request<ApiResponse<OnboardScan>>("/onboard/scan", {
      method: "POST",
      body: JSON.stringify({ repo_url: repoUrl }),
    }),
  getScan: (id: string) =>
    request<ApiResponse<OnboardScan>>(`/onboard/scans/${id}`),
  apply: (id: string, selections: OnboardSelection[]) =>
    request<ApiResponse<OnboardApplyResult>>(`/onboard/scans/${id}/apply`, {
      method: "POST",
      body: JSON.stringify({ selections }),
    }),
};
```

## Error Handling

| Scenario | Behavior |
|----------|----------|
| Private repo without token | Scan fails: "GitHub API returned 404. Set GITHUB_TOKEN for private repos." |
| No dependency files found | LLM returns empty array → UI shows "No dependencies detected" |
| LLM returns malformed JSON | River retries (max 3 attempts); if all fail, status = `failed` with error |
| Duplicate source (provider+repo) | Apply endpoint skips, includes in `skipped` array with reason |
| Duplicate active scan (same repo) | POST /scan returns 409 with existing scan ID |
| GitHub rate limit (429) | Worker retries with River's backoff; surfaces error after max retries |
| Large repo tree | Only fetches files matching dependency file name patterns |
| Apply validation: both/neither project_id and new_project_name | Returns 400 with field-level error |
| Apply: new_project_name conflicts with existing | Returns 409 with conflicting name |
| Apply: partial failure | Entire batch rolls back (single transaction) |

## Testing

- **Unit tests**: Scan worker with mocked GitHub API + mocked Gemini client
- **Unit tests**: Apply endpoint — creates projects/sources correctly, handles duplicates, validates selections
- **Unit tests**: Apply endpoint — transaction rollback on partial failure
- **Unit tests**: OnboardStore — CreateOnboardScan, GetOnboardScan, UpdateOnboardScanStatus, ApplyOnboardScan
- **Integration test**: End-to-end scan of a known public repo
- **Frontend**: Manual testing of the onboard flow

## Files to Create/Modify

### New Files
- `internal/onboard/scanner.go` — GitHub API file fetcher + tree walker
- `internal/onboard/gemini.go` — Direct Gemini call for dependency extraction
- `internal/onboard/worker.go` — River job worker
- `internal/api/onboard.go` — API handlers (scan, get scan, apply) + OnboardStore interface
- `web/app/onboard/page.tsx` — Onboarding page

### Modified Files
- `internal/db/migrations.go` — Add `onboard_scans` table
- `internal/api/server.go` — Register new routes
- `internal/api/pgstore.go` — Add OnboardStore implementation methods
- `internal/queue/jobs.go` — Add `ScanDependenciesJobArgs`
- `cmd/server/main.go` — Register scan worker with River
- `web/lib/api/client.ts` — Add onboard API functions
- `web/lib/api/types.ts` — Add OnboardScan, OnboardSelection, OnboardApplyResult types
- `web/components/layout/sidebar.tsx` — Add "Quick Onboard" entry after Dashboard
