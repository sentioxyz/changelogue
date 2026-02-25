# API Design — ReleaseBeacon (Pivot)

**Date:** 2026-02-24 (updated 2026-02-25)
**Status:** Approved

## Overview

RESTful HTTP API serving the Next.js dashboard, agent orchestration, and webhook ingestion endpoints. Pure Go stdlib `net/http` with Go 1.22+ enhanced `ServeMux` — zero routing dependencies.

**Key decisions:**
- API key authentication (bearer token)
- SSE real-time events backed by PostgreSQL LISTEN/NOTIFY
- Versioned prefix: `/api/v1`
- Webhooks live outside the versioned prefix at `/webhook/*`
- All entity IDs are UUIDs (string type)
- Sources and context sources are nested under projects
- Two subscription types: `source` (per-source) and `project` (covers all sources)
- Agent runs are triggered via API and processed asynchronously

---

## 1. Resource Endpoints

### Projects (the central entity)

A project represents a tracked piece of software. It owns sources, context sources, and semantic releases.

| Method   | Path                      | Description                                    |
|----------|---------------------------|------------------------------------------------|
| `GET`    | `/api/v1/projects`        | List projects (paginated)                      |
| `POST`   | `/api/v1/projects`        | Create a project                               |
| `GET`    | `/api/v1/projects/{id}`   | Get project details                            |
| `PUT`    | `/api/v1/projects/{id}`   | Update project metadata                        |
| `DELETE` | `/api/v1/projects/{id}`   | Delete project (cascades to sources + subscriptions) |

### Sources (nested under projects)

Sources belong to a project. A project can have multiple sources (e.g., GitHub + Docker Hub). List/create are nested under projects; get/update/delete use the source ID directly.

| Method   | Path                                      | Description                            |
|----------|-------------------------------------------|----------------------------------------|
| `GET`    | `/api/v1/projects/{projectId}/sources`    | List sources for a project             |
| `POST`   | `/api/v1/projects/{projectId}/sources`    | Register a new source for a project    |
| `GET`    | `/api/v1/sources/{id}`                    | Get source details + last poll status  |
| `PUT`    | `/api/v1/sources/{id}`                    | Update source config                   |
| `DELETE` | `/api/v1/sources/{id}`                    | Remove a source                        |

### Context Sources (nested under projects)

Context sources provide additional intelligence context for the LLM agent (docs, runbooks, etc.).

| Method   | Path                                              | Description                              |
|----------|----------------------------------------------------|------------------------------------------|
| `GET`    | `/api/v1/projects/{projectId}/context-sources`    | List context sources for a project       |
| `POST`   | `/api/v1/projects/{projectId}/context-sources`    | Create a context source for a project    |
| `GET`    | `/api/v1/context-sources/{id}`                    | Get context source details               |
| `PUT`    | `/api/v1/context-sources/{id}`                    | Update context source config             |
| `DELETE` | `/api/v1/context-sources/{id}`                    | Remove a context source                  |

### Releases (read-only)

Releases are created exclusively through the ingestion layer (pollers + webhooks). The API surface is read-only. Accessible by source or by project.

| Method | Path                                        | Description                            |
|--------|----------------------------------------------|----------------------------------------|
| `GET`  | `/api/v1/sources/{id}/releases`             | List releases for a specific source    |
| `GET`  | `/api/v1/projects/{projectId}/releases`     | List releases for all project sources  |
| `GET`  | `/api/v1/releases/{id}`                     | Get single release with full details   |

### Semantic Releases (read-only)

Semantic releases are produced by the LLM agent. They aggregate information from multiple raw releases into an actionable report.

| Method | Path                                                | Description                              |
|--------|------------------------------------------------------|------------------------------------------|
| `GET`  | `/api/v1/projects/{projectId}/semantic-releases`    | List semantic releases for a project     |
| `GET`  | `/api/v1/semantic-releases/{id}`                    | Get single semantic release with report  |

### Agent (trigger and track)

Agent runs execute the LLM agent to produce semantic releases.

| Method | Path                                          | Description                              |
|--------|------------------------------------------------|------------------------------------------|
| `POST` | `/api/v1/projects/{projectId}/agent/run`      | Trigger a new agent run                  |
| `GET`  | `/api/v1/projects/{projectId}/agent/runs`     | List agent runs for a project            |
| `GET`  | `/api/v1/agent-runs/{id}`                     | Get single agent run details             |

### Subscriptions (full CRUD)

Subscriptions link a notification channel to either a specific source or an entire project. Two types:
- `source` — triggers on releases from a specific source (requires `source_id`)
- `project` — triggers on releases from any source in the project (requires `project_id`)

| Method   | Path                          | Description              |
|----------|-------------------------------|--------------------------|
| `GET`    | `/api/v1/subscriptions`       | List all subscriptions   |
| `POST`   | `/api/v1/subscriptions`       | Create a subscription    |
| `GET`    | `/api/v1/subscriptions/{id}`  | Get single subscription  |
| `PUT`    | `/api/v1/subscriptions/{id}`  | Update a subscription    |
| `DELETE` | `/api/v1/subscriptions/{id}`  | Delete a subscription    |

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
| `GET`  | `/api/v1/health`   | Health check (DB connectivity) — **public**              |
| `GET`  | `/api/v1/stats`    | Dashboard stats (total releases, active sources, projects, pending agent runs) |

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
  "agent_prompt": "You are an SRE evaluating Ethereum client releases...",
  "agent_rules": {
    "on_major_release": true,
    "on_minor_release": false,
    "on_security_patch": true,
    "version_pattern": "^v1\\."
  }
}
```

The `agent_prompt` is a custom system prompt for the LLM agent when evaluating releases for this project. The `agent_rules` control when the agent should automatically run. Both default to empty if not provided.

Response includes `id` (UUID), `created_at`, `updated_at` fields.

### Source (request body for POST/PUT)

```json
{
  "provider": "dockerhub",
  "repository": "library/golang",
  "poll_interval_seconds": 300,
  "enabled": true,
  "config": {}
}
```

The `project_id` is set from the URL path parameter. Response includes `id` (UUID), `last_polled_at`, `last_error`, `created_at`, `updated_at` fields. The `config` is provider-specific optional configuration.

### Context Source (request body for POST/PUT)

```json
{
  "type": "documentation",
  "name": "Geth Release Notes",
  "config": {
    "url": "https://github.com/ethereum/go-ethereum/blob/master/CHANGELOG.md",
    "format": "markdown"
  }
}
```

The `project_id` is set from the URL path parameter. Response includes `id` (UUID), `created_at`, `updated_at` fields.

### Release (response — read-only)

```json
{
  "id": "550e8400-e29b-41d4-a716-446655440000",
  "source_id": "660e8400-e29b-41d4-a716-446655440001",
  "version": "v1.10.15",
  "raw_data": { "digest": "sha256:abc..." },
  "released_at": "2026-02-24T10:30:00Z",
  "created_at": "2026-02-24T10:30:00Z"
}
```

### Semantic Release (response — read-only)

```json
{
  "id": "770e8400-e29b-41d4-a716-446655440002",
  "project_id": "880e8400-e29b-41d4-a716-446655440003",
  "version": "v1.10.15",
  "report": {
    "summary": "Critical security patch for block sync bug",
    "availability": "Docker image verified, binaries available",
    "adoption": "12% of nodes running this version",
    "urgency": "High — security patch with low adoption",
    "recommendation": "Upgrade immediately for security-critical deployments"
  },
  "status": "completed",
  "error": "",
  "created_at": "2026-02-24T10:30:00Z",
  "completed_at": "2026-02-24T10:30:05Z"
}
```

### Agent Run (response)

```json
{
  "id": "990e8400-e29b-41d4-a716-446655440004",
  "project_id": "880e8400-e29b-41d4-a716-446655440003",
  "semantic_release_id": "770e8400-e29b-41d4-a716-446655440002",
  "trigger": "manual",
  "status": "completed",
  "prompt_used": "You are an SRE evaluating...",
  "error": "",
  "started_at": "2026-02-24T10:30:00Z",
  "completed_at": "2026-02-24T10:30:05Z",
  "created_at": "2026-02-24T10:30:00Z"
}
```

**Trigger request body** for `POST /projects/{projectId}/agent/run`:

```json
{
  "trigger": "manual"
}
```

If `trigger` is omitted, it defaults to `"manual"`.

### Subscription (request body for POST/PUT)

```json
{
  "channel_id": "aa0e8400-e29b-41d4-a716-446655440005",
  "type": "source",
  "source_id": "660e8400-e29b-41d4-a716-446655440001",
  "version_filter": "^\\d+\\.\\d+\\.0$"
}
```

Or for project-level subscriptions:

```json
{
  "channel_id": "aa0e8400-e29b-41d4-a716-446655440005",
  "type": "project",
  "project_id": "880e8400-e29b-41d4-a716-446655440003",
  "version_filter": ""
}
```

The `type` must be either `"source"` or `"project"`. When `type` is `"source"`, `source_id` is required. When `type` is `"project"`, `project_id` is required. Response includes `id` (UUID), `created_at` fields.

### Notification Channel (request body for POST/PUT)

```json
{
  "type": "slack",
  "name": "Engineering Releases",
  "config": {
    "webhook_url": "https://hooks.slack.com/services/...",
    "channel": "#releases"
  }
}
```

Response includes `id` (UUID), `created_at`, `updated_at` fields. The `config` object is provider-specific — Slack needs `webhook_url`, PagerDuty needs `routing_key`, custom webhooks need `url` and optional `headers`.

### Health (response)

```json
{
  "status": "healthy",
  "checks": {
    "database": "ok"
  }
}
```

### Stats (response)

```json
{
  "total_releases": 1423,
  "active_sources": 12,
  "total_projects": 8,
  "pending_agent_runs": 2
}
```

---

## 4. SSE Real-Time Events

### Endpoint

| Method | Path               | Description                           |
|--------|--------------------|---------------------------------------|
| `GET`  | `/api/v1/events`   | SSE stream of real-time events        |

### Architecture

```
PostgreSQL LISTEN/NOTIFY  ->  Go listener goroutine  ->  SSE broadcaster  ->  connected clients
```

1. When a release is ingested (transactional outbox commits), a PostgreSQL trigger fires `NOTIFY release_events, '{release_id}'`
2. A Go goroutine holds a persistent `LISTEN release_events` connection
3. On notification, it broadcasts to all connected SSE clients
4. Clients filter by topic if specified

### Event Types

| Event                      | Triggered When                          |
|----------------------------|-----------------------------------------|
| `release.created`          | New release ingested                    |
| `agent.started`            | Agent run begins processing             |
| `agent.completed`          | Agent run produces a semantic release   |
| `agent.failed`             | Agent run encounters an error           |
| `source.error`             | Polling source encounters an error      |
| `source.polled`            | Successful poll completed               |

### Event Format

Standard SSE with JSON payloads:

```
event: release.created
data: {"id":"550e8400...","source_id":"660e8400...","version":"1.21.0","created_at":"2026-02-24T10:30:00Z"}

event: agent.completed
data: {"agent_run_id":"990e8400...","project_id":"880e8400...","semantic_release_id":"770e8400..."}

event: source.error
data: {"source_id":"660e8400...","error":"rate limited","timestamp":"2026-02-24T10:31:00Z"}
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
| 4     | Rate Limit   | Token bucket per API key; returns `Retry-After` header                |
| 5     | Auth         | Validate API key, reject 401 if missing/invalid (skip for `/health`)  |

All middleware uses the stdlib `func(http.Handler) http.Handler` pattern. Rate limiting uses Go's `golang.org/x/time/rate` — in-process token bucket keyed by API key, no external dependencies. CORS is applied at the server level.

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

### Package: `internal/api/`

```
internal/api/
  server.go             -- HTTP server setup, route registration
  middleware.go         -- Request ID, logger, recovery, rate limit, CORS
  auth.go              -- API key validation, key store interface
  response.go          -- Envelope helpers (JSON, Error, Paginated)
  projects.go          -- Project CRUD handlers
  sources.go           -- Source CRUD handlers (nested under projects)
  releases.go          -- Releases read-only handlers (by source and project)
  subscriptions.go     -- Subscription CRUD handlers
  channels.go          -- Notification channel CRUD handlers
  context_sources.go   -- Context source CRUD handlers (nested under projects)
  semantic_releases.go -- Semantic release read-only handlers
  agent.go             -- Agent run trigger and listing handlers
  providers.go         -- Provider metadata handler
  health.go            -- Health check + stats
  events.go            -- SSE broadcaster, LISTEN/NOTIFY integration
  pgstore.go           -- PostgreSQL implementation of all store interfaces
```

### Handler Pattern

Each resource gets a handler struct with an injected store interface:

```go
type ReleasesHandler struct {
    store ReleasesStore
}

func (h *ReleasesHandler) ListBySource(w http.ResponseWriter, r *http.Request) { ... }
func (h *ReleasesHandler) ListByProject(w http.ResponseWriter, r *http.Request) { ... }
func (h *ReleasesHandler) Get(w http.ResponseWriter, r *http.Request) { ... }
```

### Store Interfaces

Each handler defines its own store interface (what it needs from the DB). All IDs are `string` (UUID):

```go
type ProjectsStore interface {
    ListProjects(ctx context.Context, page, perPage int) ([]models.Project, int, error)
    CreateProject(ctx context.Context, p *models.Project) error
    GetProject(ctx context.Context, id string) (*models.Project, error)
    UpdateProject(ctx context.Context, id string, p *models.Project) error
    DeleteProject(ctx context.Context, id string) error
}

type SourcesStore interface {
    ListSourcesByProject(ctx context.Context, projectID string, page, perPage int) ([]models.Source, int, error)
    CreateSource(ctx context.Context, src *models.Source) error
    GetSource(ctx context.Context, id string) (*models.Source, error)
    UpdateSource(ctx context.Context, id string, src *models.Source) error
    DeleteSource(ctx context.Context, id string) error
}

type ReleasesStore interface {
    ListReleasesBySource(ctx context.Context, sourceID string, page, perPage int) ([]models.Release, int, error)
    ListReleasesByProject(ctx context.Context, projectID string, page, perPage int) ([]models.Release, int, error)
    GetRelease(ctx context.Context, id string) (*models.Release, error)
}

type SubscriptionsStore interface {
    ListSubscriptions(ctx context.Context, page, perPage int) ([]models.Subscription, int, error)
    CreateSubscription(ctx context.Context, sub *models.Subscription) error
    GetSubscription(ctx context.Context, id string) (*models.Subscription, error)
    UpdateSubscription(ctx context.Context, id string, sub *models.Subscription) error
    DeleteSubscription(ctx context.Context, id string) error
}

type ChannelsStore interface {
    ListChannels(ctx context.Context, page, perPage int) ([]models.NotificationChannel, int, error)
    CreateChannel(ctx context.Context, ch *models.NotificationChannel) error
    GetChannel(ctx context.Context, id string) (*models.NotificationChannel, error)
    UpdateChannel(ctx context.Context, id string, ch *models.NotificationChannel) error
    DeleteChannel(ctx context.Context, id string) error
}

type ContextSourcesStore interface {
    ListContextSources(ctx context.Context, projectID string, page, perPage int) ([]models.ContextSource, int, error)
    CreateContextSource(ctx context.Context, cs *models.ContextSource) error
    GetContextSource(ctx context.Context, id string) (*models.ContextSource, error)
    UpdateContextSource(ctx context.Context, id string, cs *models.ContextSource) error
    DeleteContextSource(ctx context.Context, id string) error
}

type SemanticReleasesStore interface {
    ListSemanticReleases(ctx context.Context, projectID string, page, perPage int) ([]models.SemanticRelease, int, error)
    GetSemanticRelease(ctx context.Context, id string) (*models.SemanticRelease, error)
    GetSemanticReleaseSources(ctx context.Context, id string) ([]models.Release, error)
}

type AgentStore interface {
    TriggerAgentRun(ctx context.Context, projectID, trigger string) (*models.AgentRun, error)
    ListAgentRuns(ctx context.Context, projectID string, page, perPage int) ([]models.AgentRun, int, error)
    GetAgentRun(ctx context.Context, id string) (*models.AgentRun, error)
}
```

### PgStore

A single `PgStore` struct implements all store interfaces using a PostgreSQL connection pool and an optional River client for agent job enqueuing:

```go
type PgStore struct {
    pool  *pgxpool.Pool
    river *river.Client[pgx.Tx]
}
```

### Dependencies

```go
type Dependencies struct {
    DB                    *pgxpool.Pool
    ProjectsStore         ProjectsStore
    ReleasesStore         ReleasesStore
    SubscriptionsStore    SubscriptionsStore
    SourcesStore          SourcesStore
    ChannelsStore         ChannelsStore
    ContextSourcesStore   ContextSourcesStore
    SemanticReleasesStore SemanticReleasesStore
    AgentStore            AgentStore
    KeyStore              KeyStore
    HealthChecker         HealthChecker
    Broadcaster           *Broadcaster
    NoAuth                bool
}
```

### Integration with main.go

```go
func main() {
    // ... existing DB pool, River client setup ...

    pgStore := api.NewPgStore(pool, riverClient)
    broadcaster := api.NewBroadcaster()

    mux := http.NewServeMux()
    api.RegisterRoutes(mux, api.Dependencies{
        DB:                    pool,
        ProjectsStore:         pgStore,
        ReleasesStore:         pgStore,
        SubscriptionsStore:    pgStore,
        SourcesStore:          pgStore,
        ChannelsStore:         pgStore,
        ContextSourcesStore:   pgStore,
        SemanticReleasesStore: pgStore,
        AgentStore:            pgStore,
        KeyStore:              pgStore,
        HealthChecker:         pgStore,
        Broadcaster:           broadcaster,
        NoAuth:                noAuth,
    })

    // Webhook routes (outside /api/v1, separate auth)
    mux.Handle("POST /webhook/github", githubHandler)

    srv := &http.Server{Addr: ":8080", Handler: api.CORS(mux)}
    // ... graceful shutdown with signal handling ...
}
```
