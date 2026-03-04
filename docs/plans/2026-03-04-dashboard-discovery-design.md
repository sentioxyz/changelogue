# Dashboard Discovery Section — Design

## Goal

Add a discovery section to the dashboard so users can quickly onboard popular repositories from GitHub trending and Docker Hub without manual project creation.

## Approach

Backend-proxied discovery (Approach A): new Go API endpoints fetch trending/popular repos from GitHub and Docker Hub, cache results in-memory, and return normalized data. The frontend renders a tabbed discovery section at the top of the dashboard with one-click "Track" buttons.

## Backend: Discovery API

### Endpoints

**`GET /api/v1/discover/github`**
- Proxies GitHub Search API: `https://api.github.com/search/repositories?sort=stars&order=desc&q=stars:>1000`
- Query params: `language`, `topic`, `q` (free text search)
- In-memory cache, 1-hour TTL, keyed by query params
- Returns normalized items

**`GET /api/v1/discover/dockerhub`**
- Proxies Docker Hub search API: `https://hub.docker.com/v2/search/repositories/?query=*&page_size=25&ordering=-star_count`
- Query param: `q` (search text)
- Same cache pattern, 1-hour TTL
- Returns normalized items

### Response Shape

```json
{
  "data": [
    {
      "name": "kubernetes",
      "full_name": "kubernetes/kubernetes",
      "description": "Production-Grade Container Scheduling and Management",
      "stars": 112000,
      "language": "Go",
      "url": "https://github.com/kubernetes/kubernetes",
      "avatar_url": "https://avatars.githubusercontent.com/u/13629408",
      "provider": "github"
    }
  ],
  "meta": { "request_id": "uuid" }
}
```

### Implementation

- New file: `internal/api/discover.go`
- `DiscoverHandler` struct with `*http.Client` and cache map (`sync.Mutex` + `time.Time` + `[]DiscoverItem`)
- Follows existing handler patterns (RespondJSON, RespondError)
- Registered on public middleware chain (no auth required — public data)

## Frontend: Discovery Section

### Layout

- Position: top of dashboard, above stats cards
- Section heading: "Discover Projects"
- Two tabs: GitHub | Docker Hub
- Horizontal scrollable row of compact cards (5-6 visible)
- Each card: repo name, description (truncated), star count, language badge (GitHub only)
- "Track" button per card — one-click onboarding

### Behavior

- Data via SWR (`refreshInterval: 0`, `revalidateOnFocus: false`)
- Loading skeleton while fetching
- Already-tracked repos show checkmark/"Tracked" state (matched by `source.repository == discoverItem.full_name`)
- Collapsible/dismissible section

### Files

- `web/components/dashboard/discovery-section.tsx` — main component
- `web/lib/api/client.ts` — add `discoverGitHub()` and `discoverDockerHub()`
- `web/lib/api/types.ts` — add `DiscoverItem` type
- `web/app/page.tsx` — mount above existing content

## One-Click Track Flow

When user clicks "Track":

1. `POST /api/v1/projects` — name = full_name, description = repo description
2. `POST /api/v1/projects/{id}/sources` — provider + repository, poll_interval = `24h`
3. `POST /sources/{id}/poll` — trigger immediate first poll

All sequential. Track button shows spinner. On success: optimistic "Tracked" state. On failure: error toast + cleanup.

### Edge Cases

- Duplicate project name: show "Already tracked" toast
- Partial failure: delete project if source creation fails
- "Already tracked" detection: compare source repositories against discovered items

## Caching

### Backend

- In-memory per endpoint, no external deps
- Cache key = serialized query params
- TTL = 1 hour
- Cache miss: fetch upstream, store result + timestamp
- Cache hit: return immediately

### Frontend

- SWR with no auto-refresh (discovery data is stable suggestions)
- Projects list uses existing 30s refresh for "tracked" matching

## Not In Scope

- GitHub user authentication / personal starred repos (future)
- Curated suggestion lists
- URL-based quick import
- New ingestion providers (uses existing github + dockerhub providers)
