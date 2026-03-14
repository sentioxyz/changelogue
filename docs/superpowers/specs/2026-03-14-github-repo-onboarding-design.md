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
| LLM provider | Gemini via ADK-Go | Already integrated for semantic releases |
| UX entry point | Dedicated `/onboard` page | Clear onboarding flow, separate from project management |
| Backend pattern | Async River job | Handles large repos; matches existing architecture |

## Architecture

### Data Flow

```
User enters repo URL
  → POST /api/v1/onboard/scan
    → River job: scan_dependencies
      → GitHub API: fetch repo tree
      → GitHub API: fetch dependency file contents
      → ADK-Go agent: parse dependencies via Gemini
      → Store results in onboard_scans table
      → SSE broadcast: scan_complete
  → Frontend polls/SSE for results
  → User selects dependencies to track
  → POST /api/v1/onboard/scans/{id}/apply
    → Create projects + sources
    → Trigger first poll for each source
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
- Response: `{ "data": { "created_projects": [...], "created_sources": [...], "skipped": [...] } }`

### New River Job

```go
type ScanDependenciesJobArgs struct {
    ScanID string `json:"scan_id"`
}

func (ScanDependenciesJobArgs) Kind() string { return "scan_dependencies" }
```

Worker steps:
1. Parse repo owner/name from `repo_url`
2. Fetch repo tree via `GET /repos/{owner}/{repo}/git/trees/{default_branch}?recursive=1`
3. Filter tree entries for known dependency file patterns (go.mod, package.json, requirements.txt, Cargo.toml, pyproject.toml, Gemfile, pom.xml, build.gradle, etc.)
4. Fetch contents of matched files via `GET /repos/{owner}/{repo}/contents/{path}`
5. Invoke ADK-Go dependency extraction agent with file contents
6. Parse structured JSON response, store in `onboard_scans.results`
7. Update status to `completed`, broadcast SSE event

### ADK-Go Dependency Extraction Agent

A lightweight agent — single LLM call, no sub-agents or tools.

**Prompt**:
```
You are a dependency extraction agent. Given the contents of dependency/manifest
files from a software project, extract all dependencies.

For each dependency, return:
- name: the package/library name
- version: the version constraint or pinned version
- ecosystem: one of "go", "npm", "pypi", "cargo", "rubygems", "maven", "gradle", "docker", "other"
- upstream_repo: your best guess at the canonical GitHub repository URL (e.g., "github.com/gorilla/mux")
- provider: the Changelogue provider to use for tracking — "github" for GitHub repos, "dockerhub" for Docker images

Return ONLY a JSON array. No explanations.
```

Reuses model configuration from `internal/agent/model.go`.

## Frontend

### New Page: `/onboard`

**Sidebar entry**: "Quick Onboard" — positioned near the top for discoverability.

**Step 1 — Input**: Text field for GitHub repo URL + "Scan" button.

**Step 2 — Scanning**: Loading state with spinner. Uses SSE or polling (`GET /api/v1/onboard/scans/{id}`) to wait for results.

**Step 3 — Results**: Table with columns:
- Checkbox (select/deselect, all selected by default)
- Dependency name
- Version (detected)
- Ecosystem badge (Go, npm, Python, etc.)
- Upstream source (GitHub repo URL)
- Project assignment: dropdown — "Create new project" or select existing project

**Step 4 — Apply**: "Track Selected" button. Shows success summary with links to created projects.

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
| Private repo without token | Scan fails with "GitHub API returned 404. Set GITHUB_TOKEN for private repos." |
| No dependency files found | LLM returns empty array → UI shows "No dependencies detected" |
| LLM returns malformed JSON | River retries (max 3 attempts); if all fail, status = `failed` with error |
| Duplicate source (provider+repo) | Apply endpoint skips with warning in `skipped` array |
| GitHub rate limit (429) | Worker retries with River's backoff; surfaces error after max retries |
| Large repo tree | Only fetches files matching dependency file name patterns |

## Testing

- **Unit tests**: Scan worker with mocked GitHub API + mocked ADK-Go agent
- **Unit tests**: Apply endpoint — creates projects/sources correctly, handles duplicates
- **Integration test**: End-to-end scan of a known public repo (e.g., `sentioxyz/changelogue` itself)
- **Frontend**: Manual testing of the onboard flow

## Files to Create/Modify

### New Files
- `internal/onboard/scanner.go` — GitHub API file fetcher + tree walker
- `internal/onboard/agent.go` — ADK-Go dependency extraction agent
- `internal/onboard/worker.go` — River job worker
- `internal/queue/scan_job.go` — Job args definition (or add to existing `jobs.go`)
- `internal/api/onboard.go` — API handlers (scan, get scan, apply)
- `web/app/onboard/page.tsx` — Onboarding page
- `web/lib/api/types.ts` — New types (OnboardScan, OnboardSelection, etc.)

### Modified Files
- `internal/db/migrations.go` — Add `onboard_scans` table
- `internal/api/server.go` — Register new routes
- `internal/api/pgstore.go` — Add store methods for onboard_scans
- `cmd/server/main.go` — Register scan worker with River
- `web/lib/api/client.ts` — Add onboard API functions
- `web/components/layout/sidebar.tsx` — Add "Quick Onboard" entry
