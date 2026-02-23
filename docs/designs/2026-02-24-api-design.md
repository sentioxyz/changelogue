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

### Releases (read-only)

Releases are created exclusively through the ingestion layer (pollers + webhooks). The API surface is read-only.

| Method | Path                             | Description                            |
|--------|----------------------------------|----------------------------------------|
| `GET`  | `/api/v1/releases`               | List releases (paginated, filterable)  |
| `GET`  | `/api/v1/releases/{id}`          | Get single release with full details   |
| `GET`  | `/api/v1/releases/{id}/pipeline` | Get pipeline status for a release      |

**Query parameters** for `GET /api/v1/releases`:

| Param         | Type    | Default       | Description                                |
|---------------|---------|---------------|--------------------------------------------|
| `source`      | string  | —             | Filter by source (`dockerhub`, `github`)   |
| `repository`  | string  | —             | Filter by repository name                  |
| `pre_release` | bool    | —             | Filter by pre-release flag                 |
| `page`        | int     | `1`           | Page number                                |
| `per_page`    | int     | `25`          | Items per page (max 100)                   |
| `sort`        | string  | `created_at`  | Sort field (`created_at`, `version`)       |
| `order`       | string  | `desc`        | Sort order (`asc`, `desc`)                 |

### Subscriptions (full CRUD)

| Method   | Path                          | Description              |
|----------|-------------------------------|--------------------------|
| `GET`    | `/api/v1/subscriptions`       | List all subscriptions   |
| `POST`   | `/api/v1/subscriptions`       | Create a subscription    |
| `GET`    | `/api/v1/subscriptions/{id}`  | Get single subscription  |
| `PUT`    | `/api/v1/subscriptions/{id}`  | Update a subscription    |
| `DELETE` | `/api/v1/subscriptions/{id}`  | Delete a subscription    |

### Sources (configured ingestion sources)

| Method   | Path                    | Description                            |
|----------|-------------------------|----------------------------------------|
| `GET`    | `/api/v1/sources`       | List configured sources                |
| `POST`   | `/api/v1/sources`       | Register a new source to poll          |
| `GET`    | `/api/v1/sources/{id}`  | Get source details + last poll status  |
| `PUT`    | `/api/v1/sources/{id}`  | Update source config                   |
| `DELETE` | `/api/v1/sources/{id}`  | Remove a source                        |

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

### Release (response)

```json
{
  "id": "550e8400-e29b-41d4-a716-446655440000",
  "source": "dockerhub",
  "repository": "library/golang",
  "raw_version": "1.21.0",
  "semantic_version": {
    "major": 1,
    "minor": 21,
    "patch": 0,
    "pre_release": ""
  },
  "is_pre_release": false,
  "changelog": "## What's Changed\n- Performance improvements...",
  "metadata": { "digest": "sha256:abc..." },
  "pipeline_status": "completed",
  "created_at": "2026-02-24T10:30:00Z"
}
```

### Subscription (request body for POST/PUT)

```json
{
  "repository": "library/golang",
  "channel_type": "stable",
  "notification_target": "slack:#releases"
}
```

Response includes `id` field.

### Source (request body for POST/PUT)

```json
{
  "type": "dockerhub",
  "repository": "library/golang",
  "poll_interval_seconds": 300,
  "enabled": true
}
```

Response includes `id`, `last_polled_at`, `last_error` fields.

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
| `pipeline.status_changed`  | Pipeline job transitions state          |
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
| 4     | Auth         | Validate API key, reject 401 if missing/invalid (skip for `/health`)  |
| 5     | CORS         | Allow frontend origin, standard headers                               |

All middleware uses the stdlib `func(http.Handler) http.Handler` pattern.

---

## 7. Error Codes

| HTTP Status | Error Code         | When                                      |
|-------------|--------------------|-------------------------------------------|
| 400         | `bad_request`      | Invalid query params, malformed JSON       |
| 401         | `unauthorized`     | Missing or invalid API key                 |
| 404         | `not_found`        | Resource doesn't exist                     |
| 409         | `conflict`         | Duplicate resource (e.g. duplicate subscription) |
| 422         | `validation_error` | Valid JSON but invalid field values        |
| 500         | `internal_error`   | Unexpected server error                    |

---

## 8. Go Package Architecture

### New Package: `internal/api/`

```
internal/api/
  server.go          — HTTP server setup, route registration, graceful shutdown
  middleware.go       — Request ID, logger, recovery, auth, CORS
  auth.go            — API key validation, key store interface
  response.go        — Envelope helpers (JSON, Error, Paginated)
  releases.go        — GET /releases, GET /releases/{id}, GET /releases/{id}/pipeline
  subscriptions.go   — Full CRUD handlers
  sources.go         — Full CRUD handlers
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
type ReleasesStore interface {
    ListReleases(ctx context.Context, opts ListReleasesOpts) ([]models.ReleaseEvent, int, error)
    GetRelease(ctx context.Context, id string) (*models.ReleaseEvent, error)
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
}
```

### Route Registration

A single function wires everything, called from `main.go`:

```go
func RegisterRoutes(mux *http.ServeMux, deps Dependencies) {
    // Middleware chain
    chain := Chain(RequestID, Logger, Recovery, Auth(deps.KeyStore), CORS)

    // Releases (read-only)
    releases := &ReleasesHandler{store: deps.ReleasesStore}
    mux.Handle("GET /api/v1/releases", chain(http.HandlerFunc(releases.List)))
    mux.Handle("GET /api/v1/releases/{id}", chain(http.HandlerFunc(releases.Get)))
    mux.Handle("GET /api/v1/releases/{id}/pipeline", chain(http.HandlerFunc(releases.Pipeline)))

    // Subscriptions (CRUD)
    subs := &SubscriptionsHandler{store: deps.SubscriptionsStore}
    mux.Handle("GET /api/v1/subscriptions", chain(http.HandlerFunc(subs.List)))
    mux.Handle("POST /api/v1/subscriptions", chain(http.HandlerFunc(subs.Create)))
    mux.Handle("GET /api/v1/subscriptions/{id}", chain(http.HandlerFunc(subs.Get)))
    mux.Handle("PUT /api/v1/subscriptions/{id}", chain(http.HandlerFunc(subs.Update)))
    mux.Handle("DELETE /api/v1/subscriptions/{id}", chain(http.HandlerFunc(subs.Delete)))

    // Sources (CRUD)
    sources := &SourcesHandler{store: deps.SourcesStore}
    mux.Handle("GET /api/v1/sources", chain(http.HandlerFunc(sources.List)))
    mux.Handle("POST /api/v1/sources", chain(http.HandlerFunc(sources.Create)))
    mux.Handle("GET /api/v1/sources/{id}", chain(http.HandlerFunc(sources.Get)))
    mux.Handle("PUT /api/v1/sources/{id}", chain(http.HandlerFunc(sources.Update)))
    mux.Handle("DELETE /api/v1/sources/{id}", chain(http.HandlerFunc(sources.Delete)))

    // SSE events
    events := &EventsHandler{broadcaster: deps.Broadcaster}
    mux.Handle("GET /api/v1/events", chain(http.HandlerFunc(events.Stream)))

    // Health (public — no auth middleware)
    health := &HealthHandler{db: deps.DB, river: deps.RiverClient}
    publicChain := Chain(RequestID, Logger, Recovery, CORS)
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
        ReleasesStore:      pgStores,
        SubscriptionsStore: pgStores,
        SourcesStore:       pgStores,
        KeyStore:           pgStores,
        Broadcaster:        broadcaster,
    })

    // Webhook routes (outside /api/v1, separate auth)
    mux.Handle("POST /webhook/github", githubHandler)

    srv := &http.Server{Addr: ":8080", Handler: mux}
    // ... graceful shutdown with signal handling ...
}
```
