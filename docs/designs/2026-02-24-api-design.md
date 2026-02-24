# API Design — ReleaseBeacon

**Date:** 2026-02-24
**Status:** Approved

## Overview

RESTful HTTP API serving the Next.js dashboard and webhook ingestion endpoints. Pure Go stdlib `net/http` with Go 1.22+ enhanced `ServeMux` — zero routing dependencies.

**Key decisions:**
- API key authentication (bearer token)
- SSE real-time events backed by PostgreSQL LISTEN/NOTIFY
- Versioned prefix: `/api/v1`
- Webhooks live outside the versioned prefix at `/webhook/*`

---

## 1. Resource Endpoints

### Projects (the central entity)

A project represents a tracked piece of software. It can have multiple sources and subscriptions.

| Method   | Path                      | Description                                    |
|----------|---------------------------|------------------------------------------------|
| `GET`    | `/api/v1/projects`        | List projects (paginated)                      |
| `POST`   | `/api/v1/projects`        | Create a project                               |
| `GET`    | `/api/v1/projects/{id}`   | Get project with its sources and subscription count |
| `PUT`    | `/api/v1/projects/{id}`   | Update project metadata                        |
| `DELETE` | `/api/v1/projects/{id}`   | Delete project (cascades to sources + subscriptions) |

### Releases (read-only)

Releases are created exclusively through the ingestion layer (pollers + webhooks). The API surface is read-only.

| Method | Path                             | Description                            |
|--------|----------------------------------|----------------------------------------|
| `GET`  | `/api/v1/releases`               | List releases (paginated, filterable)  |
| `GET`  | `/api/v1/releases/{id}`          | Get single release with full details   |
| `GET`  | `/api/v1/releases/{id}/pipeline` | Get pipeline status for a release      |
| `GET`  | `/api/v1/releases/{id}/notes`    | Get release notes/changelog separately |

**Query parameters** for `GET /api/v1/releases`:

| Param         | Type    | Default       | Description                                |
|---------------|---------|---------------|--------------------------------------------|
| `project_id`  | int     | —             | Filter by project                          |
| `source_id`   | int     | —             | Filter by specific source                  |
| `pre_release` | bool    | —             | Filter by pre-release flag                 |
| `page`        | int     | `1`           | Page number                                |
| `per_page`    | int     | `25`          | Items per page (max 100)                   |
| `sort`        | string  | `created_at`  | Sort field (`created_at`, `version`)       |
| `order`       | string  | `desc`        | Sort order (`asc`, `desc`)                 |

### Subscriptions (full CRUD)

Subscriptions attach to projects, not individual sources. A subscription for a project covers all releases from any of its sources.

| Method   | Path                          | Description              |
|----------|-------------------------------|--------------------------|
| `GET`    | `/api/v1/subscriptions`       | List all subscriptions   |
| `POST`   | `/api/v1/subscriptions`       | Create a subscription    |
| `GET`    | `/api/v1/subscriptions/{id}`  | Get single subscription  |
| `PUT`    | `/api/v1/subscriptions/{id}`  | Update a subscription    |
| `DELETE` | `/api/v1/subscriptions/{id}`  | Delete a subscription    |

### Sources (configured ingestion sources)

Sources belong to a project. A project can have multiple sources (e.g., GitHub + Docker Hub).

| Method   | Path                               | Description                            |
|----------|------------------------------------|----------------------------------------|
| `GET`    | `/api/v1/sources`                  | List configured sources                |
| `POST`   | `/api/v1/sources`                  | Register a new source for a project    |
| `GET`    | `/api/v1/sources/{id}`             | Get source details + last poll status  |
| `PUT`    | `/api/v1/sources/{id}`             | Update source config                   |
| `DELETE` | `/api/v1/sources/{id}`             | Remove a source                        |
| `GET`    | `/api/v1/sources/{id}/latest-release` | Get the newest non-excluded release for a source |
| `GET`    | `/api/v1/sources/{id}/releases/{version}` | Get a specific release by version string |

### Notification Channels

| Method   | Path                       | Description                              |
|----------|----------------------------|------------------------------------------|
| `GET`    | `/api/v1/channels`         | List registered notification channels    |
| `POST`   | `/api/v1/channels`         | Register a new channel (Slack, PagerDuty, webhook) |
| `GET`    | `/api/v1/channels/{id}`    | Get channel details                      |
| `PUT`    | `/api/v1/channels/{id}`    | Update channel config                    |
| `DELETE` | `/api/v1/channels/{id}`    | Remove a channel                         |

### Providers (metadata)

| Method | Path                 | Description                                      |
|--------|----------------------|--------------------------------------------------|
| `GET`  | `/api/v1/providers`  | List supported source types (`dockerhub`, `github`, etc.) |

### Webhooks (ingestion — outside `/api/v1`)

| Method | Path               | Description                              |
|--------|--------------------|------------------------------------------|
| `POST` | `/webhook/github`  | GitHub release webhook (HMAC-SHA256 auth) |

### System

| Method | Path               | Description                                              |
|--------|--------------------|----------------------------------------------------------|
| `GET`  | `/api/v1/health`   | Health check (DB connectivity, queue status) — **public** |
| `GET`  | `/api/v1/stats`    | Dashboard stats (total releases, active sources, pending jobs) |

---

## 2. Response Envelope

Every API response uses a consistent JSON envelope.

### Success — single resource

```json
{
  "data": { ... },
  "meta": { "request_id": "550e8400-e29b-41d4-a716-446655440000" }
}
```

### Success — list

```json
{
  "data": [ ... ],
  "meta": {
    "request_id": "550e8400-e29b-41d4-a716-446655440000",
    "page": 1,
    "per_page": 25,
    "total": 142
  }
}
```

### Error

```json
{
  "error": {
    "code": "not_found",
    "message": "Release with ID abc-123 not found"
  },
  "meta": { "request_id": "550e8400-e29b-41d4-a716-446655440000" }
}
```

---

## 3. Resource Shapes

### Project (request body for POST/PUT)

```json
{
  "name": "Geth",
  "description": "Go Ethereum - Official Go implementation of the Ethereum protocol",
  "url": "https://geth.ethereum.org",
  "pipeline_config": {
    "availability_checker": true,
    "risk_assessor": true,
    "adoption_tracker": true,
    "changelog_summarizer": true,
    "urgency_scorer": true,
    "validation_trigger": false
  }
}
```

The `pipeline_config` controls which DAG nodes run for this project. Disabled nodes are skipped during processing and their sections are omitted from notifications. Two structural nodes (regex normalizer, subscription router) are always-on and not configurable. If `pipeline_config` is omitted on creation, defaults are applied (all enabled except `adoption_tracker` and `validation_trigger`).

Response includes `id`, `created_at`, `updated_at` fields. `GET /projects/{id}` also includes `sources` (array of attached sources) and `subscription_count`.

### Release (response)

```json
{
  "id": "550e8400-e29b-41d4-a716-446655440000",
  "source_id": 2,
  "source_type": "dockerhub",
  "repository": "ethereum/client-go",
  "project_id": 1,
  "project_name": "Geth",
  "raw_version": "v1.10.15",
  "semantic_version": {
    "major": 1,
    "minor": 10,
    "patch": 15,
    "pre_release": ""
  },
  "is_pre_release": false,
  "metadata": { "digest": "sha256:abc..." },
  "pipeline_status": "completed",
  "created_at": "2026-02-24T10:30:00Z"
}
```

The `source_type`, `repository`, `project_id`, and `project_name` fields are denormalized from the source/project join for convenience. List responses omit `changelog` to keep payloads small — use `GET /releases/{id}/notes` for the full text.

`GET /releases/{id}/pipeline` returns the per-node results that map to notification sections:

```json
{
  "release_id": "550e8400...",
  "state": "completed",
  "current_node": null,
  "node_results": {
    "regex_normalizer": {
      "semantic_version": { "major": 1, "minor": 10, "patch": 15 },
      "is_pre_release": false
    },
    "availability_checker": {
      "docker_image": "verified",
      "binaries": "available"
    },
    "risk_assessor": {
      "level": "critical",
      "keywords": ["Hard Fork"],
      "signal_source": "Discord #announcements"
    },
    "adoption_tracker": {
      "percentage": 12,
      "recommendation": "Wait recommended if not urgent"
    },
    "changelog_summarizer": {
      "summary": "Fixes sync bug in block 14,000,000."
    },
    "urgency_scorer": {
      "score": "high",
      "factors": ["critical_risk_level", "low_adoption"]
    }
  },
  "attempt": 1,
  "completed_at": "2026-02-24T10:30:05Z"
}
```

### Subscription (request body for POST/PUT)

```json
{
  "project_id": 1,
  "channel_type": "stable",
  "channel_id": 1,
  "version_pattern": "^\\d+\\.\\d+\\.0$",
  "frequency": "instant",
  "enabled": true
}
```

Response includes `id`, `created_at`, `updated_at` fields. The `project_id` links the subscription to a project — it covers releases from all of that project's sources. The `channel_id` references a registered notification channel. The `version_pattern` is an optional regex — only matching versions trigger notifications. The `frequency` controls digest batching: `instant` (default), `hourly`, `daily`, or `weekly`.

### Source (request body for POST/PUT)

```json
{
  "project_id": 1,
  "type": "dockerhub",
  "repository": "library/golang",
  "poll_interval_seconds": 300,
  "enabled": true,
  "exclude_version_regexp": "-(alpha|beta|nightly)",
  "exclude_prereleases": false
}
```

Response includes `id`, `last_polled_at`, `last_error`, `created_at`, `updated_at` fields. The `project_id` links this source to its parent project. Exclusion filters are applied at ingestion time — matching versions are never inserted into the releases table.

### Notification Channel (request body for POST/PUT)

```json
{
  "type": "slack",
  "name": "Engineering Releases",
  "config": {
    "webhook_url": "https://hooks.slack.com/services/...",
    "channel": "#releases"
  },
  "enabled": true
}
```

Response includes `id`, `created_at` fields. The `config` object is provider-specific — Slack needs `webhook_url`, PagerDuty needs `routing_key`, custom webhooks need `url` and optional `headers`.

### Health (response)

```json
{
  "status": "healthy",
  "checks": {
    "database": "ok",
    "queue": "ok"
  }
}
```

### Stats (response)

```json
{
  "total_releases": 1423,
  "active_sources": 12,
  "pending_jobs": 3,
  "failed_jobs": 0
}
```

---

## 4. SSE Real-Time Events

### Endpoint

| Method | Path               | Description                           |
|--------|--------------------|---------------------------------------|
| `GET`  | `/api/v1/events`   | SSE stream of real-time events        |

**Query parameters:**

| Param    | Type   | Default | Description                              |
|----------|--------|---------|------------------------------------------|
| `topics` | string | all     | Comma-separated filter (`releases,pipeline`) |

### Architecture

```
PostgreSQL LISTEN/NOTIFY  →  Go listener goroutine  →  SSE broadcaster  →  connected clients
```

1. When a release is ingested (transactional outbox commits), a PostgreSQL trigger fires `NOTIFY release_events, '{release_id}'`
2. A Go goroutine holds a persistent `LISTEN release_events` connection
3. On notification, it fetches the full resource and broadcasts to all connected SSE clients
4. Clients filter by topic if specified

### Event Types

| Event                      | Triggered When                          |
|----------------------------|-----------------------------------------|
| `release.created`          | New release ingested                    |
| `pipeline.node_completed`  | A DAG node finishes (includes node name + result) |
| `pipeline.completed`       | Pipeline finishes for a release         |
| `pipeline.failed`          | Pipeline job goes to dead-letter queue  |
| `source.error`             | Polling source encounters an error      |
| `source.polled`            | Successful poll completed               |

### Event Format

Standard SSE with JSON payloads:

```
event: release.created
data: {"id":"550e8400...","source":"dockerhub","repository":"library/golang","raw_version":"1.21.0","created_at":"2026-02-24T10:30:00Z"}

event: pipeline.status_changed
data: {"release_id":"550e8400...","status":"completed","node":"ai_urgency_scorer"}

event: source.error
data: {"source_id":1,"repository":"library/golang","error":"rate limited","timestamp":"2026-02-24T10:31:00Z"}
```

---

## 5. Authentication

### API Key

Bearer token in the `Authorization` header:

```
Authorization: Bearer rg_live_abc123def456
```

- Keys prefixed with `rg_live_` (production) or `rg_test_` (development)
- Stored hashed (SHA-256) in the database — raw key shown only once at creation
- `/api/v1/health` is public (no auth required) for load balancer health checks
- Webhook endpoints use their own auth (HMAC signatures), not API keys

### Database Table

```sql
CREATE TABLE api_keys (
    id SERIAL PRIMARY KEY,
    name VARCHAR(100) NOT NULL,
    key_hash VARCHAR(64) NOT NULL UNIQUE,
    key_prefix VARCHAR(12) NOT NULL,
    created_at TIMESTAMPTZ DEFAULT NOW(),
    last_used_at TIMESTAMPTZ
);
```

---

## 6. Middleware Stack

Applied in order to every `/api/v1/*` request:

| Order | Middleware    | Responsibility                                                        |
|-------|--------------|-----------------------------------------------------------------------|
| 1     | Request ID   | Generate UUID, set `X-Request-ID` header, inject into `context`       |
| 2     | Logger       | Structured JSON log: request ID, method, path, duration, status code  |
| 3     | Recovery     | Catch panics, return 500 with request ID                              |
| 4     | Rate Limit   | Token bucket per API key; returns `X-Ratelimit-Limit`, `X-Ratelimit-Remaining`, `X-Ratelimit-Reset`, `Retry-After` headers |
| 5     | Auth         | Validate API key, reject 401 if missing/invalid (skip for `/health`)  |
| 6     | CORS         | Allow frontend origin, standard headers                               |

All middleware uses the stdlib `func(http.Handler) http.Handler` pattern. Rate limiting uses Go's `golang.org/x/time/rate` — in-process token bucket keyed by API key, no external dependencies.

---

## 7. Error Codes

| HTTP Status | Error Code         | When                                      |
|-------------|--------------------|-------------------------------------------|
| 400         | `bad_request`      | Invalid query params, malformed JSON       |
| 401         | `unauthorized`     | Missing or invalid API key                 |
| 404         | `not_found`        | Resource doesn't exist                     |
| 405         | `method_not_allowed` | HTTP method not supported for this path  |
| 409         | `conflict`         | Duplicate resource (e.g. duplicate subscription) |
| 422         | `validation_error` | Valid JSON but invalid field values        |
| 429         | `rate_limited`     | Too many requests — check `Retry-After` header |
| 500         | `internal_error`   | Unexpected server error                    |
| 503         | `unavailable`      | Service in maintenance / graceful shutdown |

---

## 8. Go Package Architecture

### New Package: `internal/api/`

```
internal/api/
  server.go          — HTTP server setup, route registration, graceful shutdown
  middleware.go       — Request ID, logger, recovery, rate limit, auth, CORS
  auth.go            — API key validation, key store interface
  response.go        — Envelope helpers (JSON, Error, Paginated)
  projects.go        — Project CRUD handlers
  releases.go        — Releases + release notes handlers
  subscriptions.go   — Full CRUD handlers
  sources.go         — Full CRUD + latest-release + version lookup handlers
  channels.go        — Notification channel CRUD handlers
  providers.go       — Provider metadata handler
  health.go          — Health check + stats
  events.go          — SSE broadcaster, LISTEN/NOTIFY integration
```

### Handler Pattern

Each resource gets a handler struct with injected dependencies:

```go
type ReleasesHandler struct {
    store ReleasesStore
}

func (h *ReleasesHandler) List(w http.ResponseWriter, r *http.Request) { ... }
func (h *ReleasesHandler) Get(w http.ResponseWriter, r *http.Request)  { ... }
func (h *ReleasesHandler) Pipeline(w http.ResponseWriter, r *http.Request) { ... }
```

### Store Interfaces

Each handler defines its own store interface (what it needs from the DB):

```go
type ProjectsStore interface {
    ListProjects(ctx context.Context, opts ListProjectsOpts) ([]Project, int, error)
    CreateProject(ctx context.Context, p *Project) error
    GetProject(ctx context.Context, id int) (*Project, error)  // includes sources
    UpdateProject(ctx context.Context, id int, p *Project) error
    DeleteProject(ctx context.Context, id int) error
}

type ReleasesStore interface {
    ListReleases(ctx context.Context, opts ListReleasesOpts) ([]ReleaseView, int, error)
    GetRelease(ctx context.Context, id string) (*ReleaseView, error)
    GetReleaseNotes(ctx context.Context, id string) (string, error)
    GetPipelineStatus(ctx context.Context, releaseID string) (*PipelineStatus, error)
}

type SubscriptionsStore interface {
    ListSubscriptions(ctx context.Context) ([]Subscription, error)
    CreateSubscription(ctx context.Context, sub *Subscription) error
    GetSubscription(ctx context.Context, id int) (*Subscription, error)
    UpdateSubscription(ctx context.Context, id int, sub *Subscription) error
    DeleteSubscription(ctx context.Context, id int) error
}

type SourcesStore interface {
    ListSources(ctx context.Context) ([]Source, error)
    CreateSource(ctx context.Context, src *Source) error
    GetSource(ctx context.Context, id int) (*Source, error)
    UpdateSource(ctx context.Context, id int, src *Source) error
    DeleteSource(ctx context.Context, id int) error
    GetLatestRelease(ctx context.Context, sourceID int) (*ReleaseView, error)
    GetReleaseByVersion(ctx context.Context, sourceID int, version string) (*ReleaseView, error)
}

type ChannelsStore interface {
    ListChannels(ctx context.Context) ([]NotificationChannel, error)
    CreateChannel(ctx context.Context, ch *NotificationChannel) error
    GetChannel(ctx context.Context, id int) (*NotificationChannel, error)
    UpdateChannel(ctx context.Context, id int, ch *NotificationChannel) error
    DeleteChannel(ctx context.Context, id int) error
}
```

Note: `ReleaseView` is a read model that joins `releases`, `sources`, and `projects` to include denormalized fields (`source_type`, `repository`, `project_id`, `project_name`) in a single response object.

### Route Registration

A single function wires everything, called from `main.go`:

```go
func RegisterRoutes(mux *http.ServeMux, deps Dependencies) {
    // Middleware chains
    chain := Chain(RequestID, Logger, Recovery, RateLimit, Auth(deps.KeyStore), CORS)
    publicChain := Chain(RequestID, Logger, Recovery, CORS)

    // Projects (CRUD)
    projects := &ProjectsHandler{store: deps.ProjectsStore}
    mux.Handle("GET /api/v1/projects", chain(http.HandlerFunc(projects.List)))
    mux.Handle("POST /api/v1/projects", chain(http.HandlerFunc(projects.Create)))
    mux.Handle("GET /api/v1/projects/{id}", chain(http.HandlerFunc(projects.Get)))
    mux.Handle("PUT /api/v1/projects/{id}", chain(http.HandlerFunc(projects.Update)))
    mux.Handle("DELETE /api/v1/projects/{id}", chain(http.HandlerFunc(projects.Delete)))

    // Releases (read-only)
    releases := &ReleasesHandler{store: deps.ReleasesStore}
    mux.Handle("GET /api/v1/releases", chain(http.HandlerFunc(releases.List)))
    mux.Handle("GET /api/v1/releases/{id}", chain(http.HandlerFunc(releases.Get)))
    mux.Handle("GET /api/v1/releases/{id}/pipeline", chain(http.HandlerFunc(releases.Pipeline)))
    mux.Handle("GET /api/v1/releases/{id}/notes", chain(http.HandlerFunc(releases.Notes)))

    // Subscriptions (CRUD)
    subs := &SubscriptionsHandler{store: deps.SubscriptionsStore}
    mux.Handle("GET /api/v1/subscriptions", chain(http.HandlerFunc(subs.List)))
    mux.Handle("POST /api/v1/subscriptions", chain(http.HandlerFunc(subs.Create)))
    mux.Handle("GET /api/v1/subscriptions/{id}", chain(http.HandlerFunc(subs.Get)))
    mux.Handle("PUT /api/v1/subscriptions/{id}", chain(http.HandlerFunc(subs.Update)))
    mux.Handle("DELETE /api/v1/subscriptions/{id}", chain(http.HandlerFunc(subs.Delete)))

    // Sources (CRUD + release lookups)
    sources := &SourcesHandler{store: deps.SourcesStore}
    mux.Handle("GET /api/v1/sources", chain(http.HandlerFunc(sources.List)))
    mux.Handle("POST /api/v1/sources", chain(http.HandlerFunc(sources.Create)))
    mux.Handle("GET /api/v1/sources/{id}", chain(http.HandlerFunc(sources.Get)))
    mux.Handle("PUT /api/v1/sources/{id}", chain(http.HandlerFunc(sources.Update)))
    mux.Handle("DELETE /api/v1/sources/{id}", chain(http.HandlerFunc(sources.Delete)))
    mux.Handle("GET /api/v1/sources/{id}/latest-release", chain(http.HandlerFunc(sources.LatestRelease)))
    mux.Handle("GET /api/v1/sources/{id}/releases/{version}", chain(http.HandlerFunc(sources.ReleaseByVersion)))

    // Notification channels (CRUD)
    channels := &ChannelsHandler{store: deps.ChannelsStore}
    mux.Handle("GET /api/v1/channels", chain(http.HandlerFunc(channels.List)))
    mux.Handle("POST /api/v1/channels", chain(http.HandlerFunc(channels.Create)))
    mux.Handle("GET /api/v1/channels/{id}", chain(http.HandlerFunc(channels.Get)))
    mux.Handle("PUT /api/v1/channels/{id}", chain(http.HandlerFunc(channels.Update)))
    mux.Handle("DELETE /api/v1/channels/{id}", chain(http.HandlerFunc(channels.Delete)))

    // Providers (metadata)
    providers := &ProvidersHandler{}
    mux.Handle("GET /api/v1/providers", chain(http.HandlerFunc(providers.List)))

    // SSE events
    events := &EventsHandler{broadcaster: deps.Broadcaster}
    mux.Handle("GET /api/v1/events", chain(http.HandlerFunc(events.Stream)))

    // Health (public — no auth middleware)
    health := &HealthHandler{db: deps.DB, river: deps.RiverClient}
    mux.Handle("GET /api/v1/health", publicChain(http.HandlerFunc(health.Check)))
    mux.Handle("GET /api/v1/stats", chain(http.HandlerFunc(health.Stats)))
}
```

### Integration with main.go

```go
func main() {
    // ... existing DB pool, River client setup ...

    mux := http.NewServeMux()
    api.RegisterRoutes(mux, api.Dependencies{
        DB:                 pool,
        RiverClient:        riverClient,
        ProjectsStore:      pgStores,
        ReleasesStore:      pgStores,
        SubscriptionsStore: pgStores,
        SourcesStore:       pgStores,
        ChannelsStore:      pgStores,
        KeyStore:           pgStores,
        Broadcaster:        broadcaster,
    })

    // Webhook routes (outside /api/v1, separate auth)
    mux.Handle("POST /webhook/github", githubHandler)

    srv := &http.Server{Addr: ":8080", Handler: mux}
    // ... graceful shutdown with signal handling ...
}
```
