# Personalized GitHub Suggestions — Design Spec

## Overview

Add personalized repo/dependency suggestions to the dashboard for GitHub-authenticated users. Replaces the generic trending discovery carousel with two tabs: "Your Stars" (user's starred repos) and "Your Dependencies" (scan user-selected repos for dependencies). Uses public GitHub API only — no OAuth changes needed.

## Decisions

- **Data access**: Public GitHub API via server-side `GITHUB_TOKEN` (no per-user token storage, no OAuth scope changes)
- **Placement**: Replaces trending carousel for authenticated users; unauthenticated users see existing trending carousel
- **Trigger**: On demand — user clicks to fetch each tab's data
- **Layout**: Two tabs — "Your Stars" | "Your Dependencies"
- **Dependencies flow**: User picks repos from checklist → scan selected → review & track results
- **Dependency scanning**: Reuses existing onboard scan pipeline (LLM extraction via `POST /api/v1/onboard/scan`)

## Backend API

### `GET /api/v1/suggestions/stars`

- Auth required (session cookie)
- Fetches `GET https://api.github.com/users/{github_login}/starred` using server-side `GITHUB_TOKEN`
- Paginates up to ~100 results (GitHub returns 30/page)
- Caches per-user for 1 hour (keyed by `github_login`)
- Filters out repos already tracked as sources
- Response: array of `{ name, full_name, description, stars, language, url, avatar_url, provider: "github" }`

### `GET /api/v1/suggestions/repos`

- Auth required (session cookie)
- Fetches `GET https://api.github.com/users/{github_login}/repos?sort=pushed&per_page=100`
- Caches per-user for 1 hour
- Response: array of `{ name, full_name, description, language, url, pushed_at }`

### Dependency scanning

Reuses existing endpoints — no new endpoints needed:
- `POST /api/v1/onboard/scan` — start scan for a repo
- `GET /api/v1/onboard/scans/{id}` — poll scan status
- `POST /api/v1/onboard/scans/{id}/apply` — apply selections (create projects + sources)

## Frontend Components

### SuggestionsSection

Top-level component that replaces `DiscoverySection` for authenticated users. Manages tab state ("stars" | "deps"). Falls back to `DiscoverySection` for unauthenticated users.

### StarsTab

- Initial state: "Load your starred repos" button
- After fetch: grid of repo cards with name, description, star count, language badge
- Already-tracked repos dimmed with checkmark
- One-click "Track" button per repo (creates project + source + auto-polls, same logic as discovery carousel)
- "Show more" pagination

### DepsTab

Multi-step flow:

1. **Repo picker**: "Load your repos" button → fetches repo list → checkboxes sorted by `pushed_at` desc → "Scan Selected (N)" button
2. **Scanning**: Progress indicators per repo (reuses onboard progress UI patterns). Cap at 5 concurrent scans.
3. **Results**: Dependencies grouped by source repo (e.g., "From myapp → express, lodash, pg"). Checkboxes to select which to track. Dropdown to assign to existing project or create new one.
4. **Applied**: Success confirmation with counts (reuses onboard applied state)

## Edge Cases

- **GitHub rate limiting**: 5000 req/hr with server-side token. Cache 1hr per user. Show friendly error if rate limited.
- **No stars / no repos**: Empty state messages with suggestion to try trending discovery or quick onboard instead.
- **Scan failures**: Per-repo status (success/failed/no deps). Failed scans don't block others.
- **Already tracked**: Both tabs filter/dim repos and dependencies already tracked as sources.
- **Concurrent scans**: Cap at 5 concurrent per user. Existing 409 conflict handling for duplicate repo scans.

## Files to Create/Modify

### New files
- `internal/api/suggestions.go` — two handlers + GitHub API client functions + caching
- `web/components/dashboard/suggestions-section.tsx` — top-level tabbed component
- `web/components/dashboard/stars-tab.tsx` — stars tab with repo cards
- `web/components/dashboard/deps-tab.tsx` — dependencies tab with multi-step flow

### Modified files
- `internal/api/server.go` — register new `/api/v1/suggestions/` routes
- `web/lib/api/client.ts` — add `suggestions.stars()` and `suggestions.repos()` client methods
- `web/app/page.tsx` — conditionally render `SuggestionsSection` vs `DiscoverySection` based on auth state

## Non-goals (future work)

- Private repo access (requires storing per-user OAuth tokens, expanded scopes)
- Auto-fetch on first login
- Recommendation ranking / ML-based suggestions
- Non-GitHub providers (GitLab, Bitbucket stars/repos)
