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

## Known Limitations

- **Public data only**: `GET /users/{username}/starred` and `GET /users/{username}/repos` with a server-side token only return publicly visible stars and public repos. Private stars and private repos are not accessible. This is by design for the current scope — private repo support is a future goal requiring per-user token storage.
- **Repo cap**: GitHub API caps `per_page` at 100. Users with 100+ repos see only the most recently pushed 100. Acceptable for now.

## Backend API

### Handler Construction

`NewSuggestionsHandler(httpClient *http.Client, sourcesStore db.SourcesStore, githubToken string)` — reads `GITHUB_TOKEN` from env at initialization (same pattern as onboard scanner). Needs `SourcesStore` dependency to check which repos are already tracked.

### `GET /api/v1/suggestions/stars`

- Auth required: extracts user via `auth.UserFromContext(r.Context())`, reads `user.GitHubLogin`
- In `NO_AUTH` mode or when `github_login` is empty/`"dev"`: returns empty array (not an error)
- API key auth (Bearer token): returns 403 — these endpoints require a session with a GitHub identity
- Fetches `GET https://api.github.com/users/{github_login}/starred` using server-side `GITHUB_TOKEN`
- Paginates up to ~100 results (GitHub returns 30/page, fetch up to 4 pages)
- Caches raw GitHub response per-user for 1 hour (keyed by `github_login`). Tracked-repo filtering applied post-cache so tracking a repo is reflected immediately without cache invalidation.
- Returns already-tracked repos with a `tracked: true` flag (not filtered out) so the frontend can dim them with a checkmark
- Response: `ApiResponse<[]SuggestionItem>` where `SuggestionItem` is `{ name, full_name, description, stars, language, url, avatar_url, provider: "github", tracked: bool }`
- Rate limit error: returns `502 Bad Gateway` with `{ error: "upstream_error", message: "GitHub API rate limit exceeded, try again later" }`

### `GET /api/v1/suggestions/repos`

- Same auth requirements as `/stars`
- Fetches `GET https://api.github.com/users/{github_login}/repos?sort=pushed&per_page=100`
- Caches per-user for 1 hour
- Response: `ApiResponse<[]RepoItem>` where `RepoItem` is `{ name, full_name, description, language, url, pushed_at }`

### Dependency scanning

Reuses existing endpoints — no new endpoints needed:
- `POST /api/v1/onboard/scan` — start scan for a repo
- `GET /api/v1/onboard/scans/{id}` — poll scan status
- `POST /api/v1/onboard/scans/{id}/apply` — apply selections (create projects + sources)

## Frontend Components

### SuggestionsSection

Top-level component that replaces `DiscoverySection` for users with a `github_login`. Manages tab state ("stars" | "deps"). Falls back to `DiscoverySection` when `github_login` is absent (dev mode, API key auth).

### StarsTab

- Initial state: "Load your starred repos" button
- After fetch: grid of repo cards with name, description, star count, language badge
- Already-tracked repos dimmed with checkmark (using `tracked` flag from API)
- One-click "Track" button per repo (creates project + source + auto-polls, same logic as discovery carousel)
- "Show more" pagination (client-side, data already fetched)

### DepsTab

Multi-step flow:

1. **Repo picker**: "Load your repos" button → fetches repo list → checkboxes sorted by `pushed_at` desc → "Scan Selected (N)" button
2. **Scanning**: Progress indicators per repo (reuses onboard progress UI patterns). Cap at 5 concurrent scans.
3. **Results**: Dependencies grouped by source repo (e.g., "From myapp → express, lodash, pg"). Checkboxes to select which to track. Dropdown to assign to existing project or create new one.
4. **Applied**: Success confirmation with counts (reuses onboard applied state)

## Edge Cases

- **GitHub rate limiting**: 5000 req/hr with server-side token. Cache 1hr per user. Return 502 with descriptive error if rate limited.
- **No stars / no repos**: Empty state messages with suggestion to try trending discovery or quick onboard instead.
- **Scan failures**: Per-repo status (success/failed/no deps). Failed scans don't block others.
- **Already tracked**: Stars tab returns `tracked: true` flag; frontend dims with checkmark. Deps tab results similarly indicate already-tracked dependencies.
- **Concurrent scans**: Cap at 5 concurrent per user. Existing 409 conflict handling for duplicate repo scans.
- **NO_AUTH / dev mode**: Suggestions endpoints return empty arrays when `github_login` is empty or `"dev"`. Frontend shows trending carousel fallback.
- **Cache vs. tracking**: GitHub API response is cached; tracked-repo filtering is applied post-cache so it reflects immediately.

## Files to Create/Modify

### New files
- `internal/api/suggestions.go` — `SuggestionsHandler` with `handleStars`, `handleRepos` + GitHub API fetching + per-user caching
- `internal/api/suggestions_test.go` — unit tests for handlers (mock GitHub API, mock SourcesStore)
- `web/components/dashboard/suggestions-section.tsx` — top-level tabbed component
- `web/components/dashboard/stars-tab.tsx` — stars tab with repo cards
- `web/components/dashboard/deps-tab.tsx` — dependencies tab with multi-step flow

### Modified files
- `internal/api/server.go` — register new `/api/v1/suggestions/` routes on auth-required chain, construct `SuggestionsHandler` with `deps.SourcesStore` and `GITHUB_TOKEN`
- `web/lib/api/client.ts` — add `suggestions.stars()` and `suggestions.repos()` client methods
- `web/app/page.tsx` — conditionally render `SuggestionsSection` vs `DiscoverySection` based on `github_login` presence

## Non-goals (future work)

- Private repo/star access (requires storing per-user OAuth tokens, expanded scopes)
- Auto-fetch on first login
- Recommendation ranking / ML-based suggestions
- Non-GitHub providers (GitLab, Bitbucket stars/repos)
