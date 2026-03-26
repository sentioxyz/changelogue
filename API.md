# API Reference — Changelogue

RESTful HTTP API serving the Next.js dashboard, agent orchestration, and external integrations. Pure Go stdlib `net/http` with Go 1.22+ enhanced `ServeMux` — zero routing dependencies.

**Key decisions:**
- Dual authentication: API key (bearer token) and GitHub OAuth (session cookie), `NO_AUTH=true` for development
- SSE real-time events backed by PostgreSQL LISTEN/NOTIFY
- Versioned prefix: `/api/v1`
- All entity IDs are UUIDs (string type)
- Sources and context sources are nested under projects
- Two subscription types: `source_release` (per-source) and `semantic_release` (covers all sources)
- Agent runs are triggered via API and processed asynchronously
- Release gates, TODOs, onboarding, discovery, and suggestions extend the core API

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
| `POST`   | `/api/v1/sources/{id}/poll`               | Manually trigger source polling        |

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

Releases are created exclusively through the ingestion layer (polling sources). The API surface is read-only.

| Method | Path                                        | Description                            |
|--------|----------------------------------------------|----------------------------------------|
| `GET`  | `/api/v1/releases`                          | List all releases across all projects  |
| `GET`  | `/api/v1/sources/{id}/releases`             | List releases for a specific source    |
| `GET`  | `/api/v1/projects/{projectId}/releases`     | List releases for all project sources  |
| `GET`  | `/api/v1/releases/{id}`                     | Get single release with full details   |

### Semantic Releases

Semantic releases are produced by the LLM agent. They aggregate information from multiple raw releases into an actionable report.

| Method   | Path                                                | Description                              |
|----------|------------------------------------------------------|------------------------------------------|
| `GET`    | `/api/v1/semantic-releases`                         | List all semantic releases               |
| `GET`    | `/api/v1/projects/{projectId}/semantic-releases`    | List semantic releases for a project     |
| `GET`    | `/api/v1/semantic-releases/{id}`                    | Get single semantic release with report  |
| `GET`    | `/api/v1/semantic-releases/{id}/sources`            | List source releases composing this semantic release |
| `DELETE` | `/api/v1/semantic-releases/{id}`                    | Delete a semantic release                |

### Agent (trigger and track)

Agent runs execute the LLM agent to produce semantic releases.

| Method | Path                                          | Description                              |
|--------|------------------------------------------------|------------------------------------------|
| `POST` | `/api/v1/projects/{projectId}/agent/run`      | Trigger a new agent run                  |
| `GET`  | `/api/v1/projects/{projectId}/agent/runs`     | List agent runs for a project            |
| `GET`  | `/api/v1/agent-runs/{id}`                     | Get single agent run details             |

### Subscriptions (full CRUD)

Subscriptions link a notification channel to either a specific source or an entire project. Two types:
- `source_release` — triggers on releases from a specific source (requires `source_id`)
- `semantic_release` — triggers on semantic releases for the project (requires `project_id`)

| Method   | Path                          | Description              |
|----------|-------------------------------|--------------------------|
| `GET`    | `/api/v1/subscriptions`       | List all subscriptions   |
| `POST`   | `/api/v1/subscriptions`       | Create a subscription    |
| `GET`    | `/api/v1/subscriptions/{id}`  | Get single subscription  |
| `PUT`    | `/api/v1/subscriptions/{id}`  | Update a subscription    |
| `DELETE` | `/api/v1/subscriptions/{id}`  | Delete a subscription    |
| `POST`   | `/api/v1/subscriptions/batch` | Batch-create subscriptions |
| `DELETE` | `/api/v1/subscriptions/batch` | Batch-delete subscriptions |

### Notification Channels

| Method   | Path                            | Description                              |
|----------|---------------------------------|------------------------------------------|
| `GET`    | `/api/v1/channels`              | List registered notification channels    |
| `POST`   | `/api/v1/channels`              | Register a new channel (Slack, Discord, email, webhook) |
| `GET`    | `/api/v1/channels/{id}`         | Get channel details                      |
| `PUT`    | `/api/v1/channels/{id}`         | Update channel config                    |
| `DELETE` | `/api/v1/channels/{id}`         | Remove a channel                         |
| `POST`   | `/api/v1/channels/{id}/test`    | Send a test notification to verify channel |

### Providers (metadata)

| Method | Path                 | Description                                      |
|--------|----------------------|--------------------------------------------------|
| `GET`  | `/api/v1/providers`  | List supported source types (`dockerhub`, `github`, `ecr-public`, `gitlab`, `pypi`, `npm`) |

### System

| Method | Path                    | Description                                              |
|--------|-------------------------|----------------------------------------------------------|
| `GET`  | `/api/v1/health`        | Health check (DB connectivity) — **public**              |
| `GET`  | `/api/v1/stats`         | Dashboard stats (total releases, active sources, projects, pending agent runs) |
| `GET`  | `/api/v1/stats/trend`   | Time-bucketed release counts (configurable granularity)  |

### TODOs (release action tracking)

TODOs track operator acknowledgment and resolution of releases. Each TODO is linked to either a source release or a semantic release.

| Method  | Path                                    | Description                                    |
|---------|-----------------------------------------|------------------------------------------------|
| `GET`   | `/api/v1/todos`                         | List TODOs (filterable by status, paginated)   |
| `GET`   | `/api/v1/todos/{id}`                    | Get TODO details                               |
| `PATCH` | `/api/v1/todos/{id}/acknowledge`        | Mark TODO as acknowledged                      |
| `PATCH` | `/api/v1/todos/{id}/resolve`            | Mark TODO as resolved                          |
| `PATCH` | `/api/v1/todos/{id}/reopen`             | Reopen a resolved/acknowledged TODO            |
| `GET`   | `/api/v1/todos/{id}/acknowledge`        | One-click acknowledge (notification link)      |
| `GET`   | `/api/v1/todos/{id}/resolve`            | One-click resolve (notification link)          |

The GET variants of acknowledge/resolve support `?redirect=true` to redirect the user to the frontend TODO page after clicking from a notification.

### Onboarding (dependency scanning)

Scan a GitHub repository to auto-detect dependencies and suggest sources.

| Method | Path                                    | Description                                    |
|--------|-----------------------------------------|------------------------------------------------|
| `POST` | `/api/v1/onboard/scan`                  | Start a new dependency scan                    |
| `GET`  | `/api/v1/onboard/scans/{id}`            | Get scan status and results                    |
| `POST` | `/api/v1/onboard/scans/{id}/apply`      | Apply scan results (auto-create sources)       |

### Release Gates (version readiness)

Per-project gates that delay agent analysis until all required sources report a version.

| Method   | Path                                                           | Description                                    |
|----------|----------------------------------------------------------------|------------------------------------------------|
| `GET`    | `/api/v1/projects/{id}/release-gate`                           | Get gate configuration                         |
| `PUT`    | `/api/v1/projects/{id}/release-gate`                           | Create or update gate configuration            |
| `DELETE` | `/api/v1/projects/{id}/release-gate`                           | Delete gate configuration                      |
| `GET`    | `/api/v1/projects/{id}/version-readiness`                      | List all version readiness states              |
| `GET`    | `/api/v1/projects/{id}/version-readiness/{version}`            | Get readiness for a specific version           |
| `GET`    | `/api/v1/projects/{id}/version-readiness/{version}/events`     | Get gate events for a specific version         |
| `GET`    | `/api/v1/projects/{id}/gate-events`                            | List all gate events for project               |

### Discovery (public, no auth)

Search public registries for packages and repositories.

| Method | Path                             | Description                                    |
|--------|----------------------------------|------------------------------------------------|
| `GET`  | `/api/v1/discover/github`        | Search GitHub repositories (`?q=...&limit=N`)  |
| `GET`  | `/api/v1/discover/dockerhub`     | Search Docker Hub images (`?q=...&limit=N`)    |

### Suggestions (personalized, auth required)

Personalized source recommendations based on the authenticated user's GitHub activity.

| Method | Path                             | Description                                    |
|--------|----------------------------------|------------------------------------------------|
| `GET`  | `/api/v1/suggestions/stars`      | Repos starred by the authenticated user        |
| `GET`  | `/api/v1/suggestions/repos`      | Repos authored by the authenticated user       |

Both return items with a `tracked` boolean indicating whether the repo is already a configured source.

### Authentication

GitHub OAuth 2.0 login flow. These endpoints are **not** under the `/api/v1` prefix.

| Method | Path                        | Description                                    |
|--------|-----------------------------|------------------------------------------------|
| `GET`  | `/auth/github`              | Redirect to GitHub OAuth authorization         |
| `GET`  | `/auth/github/callback`     | Handle OAuth callback                          |
| `GET`  | `/auth/me`                  | Get authenticated user info                    |
| `POST` | `/auth/logout`              | Logout (clears session cookie)                 |

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
    "version_pattern": "^v1\\.",
    "wait_for_all_sources": false
  }
}
```

The `agent_prompt` is a custom system prompt for the LLM agent when evaluating releases for this project. The `agent_rules` control when the agent should automatically run (including `wait_for_all_sources` for multi-source synchronization). Both default to empty if not provided.

Response includes `id` (UUID), `created_at`, `updated_at` fields.

### Source (request body for POST/PUT)

```json
{
  "provider": "dockerhub",
  "repository": "library/golang",
  "poll_interval_seconds": 300,
  "enabled": true,
  "config": {},
  "version_filter_include": "^v?\\d+\\.\\d+\\.\\d+$",
  "version_filter_exclude": "-(alpha|beta|rc|nightly)",
  "exclude_prereleases": false
}
```

The `project_id` is set from the URL path parameter. Response includes `id` (UUID), `last_polled_at`, `last_error`, `created_at`, `updated_at` fields. The `config` is provider-specific optional configuration.

The `version_filter_include` and `version_filter_exclude` are optional regex patterns applied when listing releases and sending notifications. When `version_filter_include` is set, only versions matching the pattern are shown. When `version_filter_exclude` is set, versions matching the pattern are hidden. Both use PostgreSQL regex syntax.

The `exclude_prereleases` boolean (default `false`) filters out pre-release versions from listings and notifications. Applies to GitHub and GitLab sources (which have an explicit `prerelease` flag) and PyPI sources (where pre-release versions use PEP 440 suffixes like `a`, `b`, `rc`).

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

### Semantic Release (response)

```json
{
  "id": "770e8400-e29b-41d4-a716-446655440002",
  "project_id": "880e8400-e29b-41d4-a716-446655440003",
  "version": "v1.10.15",
  "report": {
    "subject": "Go 1.10.15 — Critical security patch",
    "risk_level": "HIGH",
    "risk_reason": "Security vulnerability in net/http",
    "status_checks": ["Docker image available", "Binary checksums verified"],
    "changelog_summary": "Fixes CVE-2026-XXXX in net/http TLS handling",
    "download_commands": ["docker pull golang:1.10.15"],
    "download_links": ["https://go.dev/dl/go1.10.15.linux-amd64.tar.gz"],
    "summary": "Critical security patch for TLS handling bug",
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
  "version": "v1.10.15",
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
  "type": "source_release",
  "source_id": "660e8400-e29b-41d4-a716-446655440001",
  "version_filter": "^\\d+\\.\\d+\\.0$"
}
```

Or for project-level subscriptions:

```json
{
  "channel_id": "aa0e8400-e29b-41d4-a716-446655440005",
  "type": "semantic_release",
  "project_id": "880e8400-e29b-41d4-a716-446655440003",
  "version_filter": ""
}
```

The `type` must be either `"source_release"` or `"semantic_release"`. When `type` is `"source_release"`, `source_id` is required. When `type` is `"semantic_release"`, `project_id` is required. Response includes `id` (UUID), `created_at` fields.

### Batch Subscription (request body for POST /subscriptions/batch)

Create multiple subscriptions at once for a single channel. All subscriptions are created atomically in one transaction.

For project-level subscriptions:

```json
{
  "channel_id": "aa0e8400-e29b-41d4-a716-446655440005",
  "type": "semantic_release",
  "project_ids": [
    "880e8400-e29b-41d4-a716-446655440003",
    "990e8400-e29b-41d4-a716-446655440004"
  ],
  "version_filter": ""
}
```

For source-level subscriptions:

```json
{
  "channel_id": "aa0e8400-e29b-41d4-a716-446655440005",
  "type": "source_release",
  "source_ids": [
    "660e8400-e29b-41d4-a716-446655440001",
    "770e8400-e29b-41d4-a716-446655440002"
  ],
  "version_filter": "^\\d+\\.\\d+\\.0$"
}
```

Response: `201 Created` with `data` containing an array of created subscriptions (same fields as single subscription).

### Batch Delete Subscriptions (request body for DELETE /subscriptions/batch)

Delete multiple subscriptions at once by their IDs.

```json
{
  "ids": [
    "550e8400-e29b-41d4-a716-446655440000",
    "660e8400-e29b-41d4-a716-446655440001"
  ]
}
```

Response: `204 No Content`.

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

Response includes `id` (UUID), `created_at`, `updated_at` fields. The `config` object is provider-specific:

| Type | Config Fields |
|------|---------------|
| `slack` | `webhook_url` |
| `discord` | `webhook_url` |
| `webhook` | `url` |
| `email` | `smtp_host`, `smtp_port`, `username`, `password`, `from_address`, `to_addresses` |

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

### TODO (response)

```json
{
  "id": "aa0e8400-e29b-41d4-a716-446655440010",
  "release_id": "550e8400-e29b-41d4-a716-446655440000",
  "semantic_release_id": null,
  "status": "pending",
  "acknowledged_at": null,
  "resolved_at": null,
  "created_at": "2026-02-24T10:30:00Z"
}
```

A TODO is linked to either `release_id` (source-level) or `semantic_release_id` (project-level), never both.

### Release Gate (request body for PUT)

```json
{
  "required_sources": ["660e8400-e29b-41d4-a716-446655440001", "770e8400-e29b-41d4-a716-446655440002"],
  "timeout_hours": 168,
  "version_mapping": {
    "660e8400-e29b-41d4-a716-446655440001": {
      "regex": "^v(.*)$",
      "template": "$1"
    }
  },
  "nl_rule": "All Docker images must have verified signatures",
  "enabled": true
}
```

Response includes `id` (UUID), `project_id`, `created_at`, `updated_at` fields.

### Version Readiness (response)

```json
{
  "id": "bb0e8400-e29b-41d4-a716-446655440011",
  "project_id": "880e8400-e29b-41d4-a716-446655440003",
  "version": "1.21.0",
  "status": "pending",
  "sources_met": ["660e8400-e29b-41d4-a716-446655440001"],
  "sources_missing": ["770e8400-e29b-41d4-a716-446655440002"],
  "nl_rule_passed": null,
  "timeout_at": "2026-03-03T10:30:00Z",
  "opened_at": null,
  "agent_triggered": false,
  "created_at": "2026-02-24T10:30:00Z"
}
```

### Onboard Scan (response)

```json
{
  "id": "cc0e8400-e29b-41d4-a716-446655440012",
  "repo_url": "https://github.com/ethereum/go-ethereum",
  "status": "completed",
  "results": [
    {
      "name": "golang",
      "version": "1.21",
      "provider": "dockerhub",
      "repository": "library/golang"
    }
  ],
  "error": "",
  "started_at": "2026-02-24T10:30:00Z",
  "completed_at": "2026-02-24T10:30:05Z",
  "created_at": "2026-02-24T10:30:00Z"
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
PostgreSQL LISTEN/NOTIFY  →  Go listener goroutine  →  SSE broadcaster  →  connected clients
```

1. When a release is ingested (transactional outbox commits), a PostgreSQL trigger fires `NOTIFY release_events, '{...}'`
2. A Go goroutine holds a persistent `LISTEN release_events` connection
3. On notification, it broadcasts to all connected SSE clients via the `Broadcaster` (64-buffered channels, non-blocking send skips slow clients)

### Event Types

| Event Type | Payload | Triggered When |
|------------|---------|----------------|
| `release` | `{"type": "release", "id": "<uuid>"}` | New release ingested |
| `semantic_release` | `{"type": "semantic_release", "id": "<uuid>"}` | Semantic release completed |
| `connected` | — | Initial SSE handshake |

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
- Set `NO_AUTH=true` to disable authentication entirely (development mode)

### GitHub OAuth

Browser-based authentication via GitHub OAuth 2.0:

1. Frontend redirects to `GET /auth/github`
2. User authorizes → GitHub redirects to `GET /auth/github/callback`
3. Server validates user against allowlist (`ALLOWED_GITHUB_USERS` / `ALLOWED_GITHUB_ORGS`)
4. Session cookie set (HMAC-signed, HttpOnly, configurable Secure flag)
5. Subsequent API requests authenticated via session cookie

`GET /auth/me` returns the authenticated user's profile. `POST /auth/logout` clears the session.

---

## 6. Middleware Stack

Applied in order to every `/api/v1/*` request:

| Order | Middleware    | Responsibility                                                        |
|-------|--------------|-----------------------------------------------------------------------|
| 1     | Request ID   | Generate UUID, set `X-Request-ID` header, inject into `context`       |
| 2     | Logger       | Structured JSON log: request ID, method, path, duration, status code  |
| 3     | Recovery     | Catch panics, return 500 with request ID                              |
| 4     | Rate Limit   | Token bucket per API key (10 rps, 20 burst); returns `Retry-After` header |
| 5     | Auth         | Validate API key or session cookie, reject 401 if missing/invalid (skip for `/health`, skip entirely when `NO_AUTH=true`) |

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
