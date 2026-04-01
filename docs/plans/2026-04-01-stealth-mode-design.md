# Stealth Mode Design

**Date:** 2026-04-01
**Status:** Approved

## Overview

Stealth mode is a headless, agent-native operation mode for Changelogue. It runs as a local system service with SQLite storage, no UI, and shell command callbacks — designed for integration with agent harnesses like Claude Code.

An agent interacts with stealth mode through the existing `clog` CLI pointed at a local HTTP server. When new releases are detected, the daemon executes configurable shell commands to notify or spawn agent instances.

## Architecture

Separate binary (`cmd/stealth/main.go`) that maximizes reuse of existing packages:

### Reused Components (no changes)
- **All ingestion sources** (`internal/ingestion/` — GitHub, Docker Hub, ECR, GitLab, PyPI, npm)
- **All models** (`internal/models/`)
- **API router** (`api.RegisterRoutes`) — same REST endpoints, same CLI compatibility
- **Notification senders** (`internal/routing/` — Slack, Discord, webhook, email)
- **CLI** (`clog`) — unchanged, just point at `localhost:PORT`

### Replaced Components

| Component | Normal Mode | Stealth Mode |
|-----------|------------|--------------|
| Database | PostgreSQL (`pgxpool`) | SQLite (`database/sql` + `modernc.org/sqlite`) |
| Store | `api.PgStore` | `internal/stealth/store.go` (same interfaces) |
| Job queue | River (async) | Synchronous in-process (no queue) |
| Pub/Sub | PostgreSQL LISTEN/NOTIFY | Dropped (no UI, no SSE) |
| Auth | API key + OAuth | API key only (or `NO_AUTH`) |
| Storage path | Remote DB URL | `~/.changelogue/stealth.db` |

### New Components
- **Shell sender** (`internal/routing/shell_sender.go`) — new `Sender` implementation for shell command callbacks
- **Stealth store** (`internal/stealth/store.go`) — SQLite-backed store implementing core interfaces
- **Stealth binary** (`cmd/stealth/main.go`) — service lifecycle, startup, install/uninstall

## SQLite Store

### Location
`~/.changelogue/stealth.db`

### Pragmas
```sql
PRAGMA journal_mode=WAL;
PRAGMA busy_timeout=5000;
PRAGMA foreign_keys=ON;
```

### Interfaces Implemented (core subset)
- `api.ProjectsStore` — full CRUD
- `api.SourcesStore` — full CRUD + poll status
- `api.ReleasesStore` — list/get
- `api.SubscriptionsStore` — full CRUD
- `api.ChannelsStore` — full CRUD
- `api.KeyStore` — API key validation
- `api.HealthChecker` — ping + basic stats
- `ingestion.ReleaseStore` — `IngestRelease()` (write path)
- `routing.NotifyStore` — release/subscription/channel lookups for notification dispatch

### Interfaces NOT Implemented (return 501)
- `SemanticReleasesStore`, `AgentStore`, `TodosStore`, `OnboardStore`, `GatesStore`, `ContextSourcesStore`, `SessionValidator`

### Schema Adaptations
- UUIDs generated in Go (`github.com/google/uuid`) instead of `gen_random_uuid()`
- `TIMESTAMPTZ` → `TEXT` (ISO 8601 strings)
- `JSONB` → `TEXT` (JSON as text, parsed in Go)
- `$1` positional params → `?` placeholders

## Shell Command Callbacks

New `ShellSender` implements `routing.Sender` interface.

### Channel Type
`"shell"`

### Channel Config (defaults only)
The channel holds shared defaults. The actual command lives on the subscription.
```json
{
  "timeout_seconds": 30,
  "working_dir": "~"
}
```

### Per-Subscription Command Templates
Each subscription carries its own `config.command` — this is where the real customization lives. Different projects/sources get different agent workflows:

**Example: Breaking change analyzer**
```bash
clog sources create --project-id $PID --provider github --repository owner/repo
clog subscriptions create \
  --source-id $SID \
  --channel-id $SHELL_CH \
  --config '{"command": "cd /projects/myapp && claude --message \"Analyze breaking changes in ${CHANGELOGUE_REPOSITORY} ${CHANGELOGUE_VERSION}. Check our usage and report impact.\""}'
```

**Example: Auto-update dependency**
```bash
clog subscriptions create \
  --source-id $SID \
  --channel-id $SHELL_CH \
  --config '{"command": "cd /projects/myapp && claude --message \"Update ${CHANGELOGUE_REPOSITORY} to ${CHANGELOGUE_VERSION} in go.mod, run tests, and commit if green.\"", "working_dir": "/projects/myapp"}'
```

**Example: Notify and log only**
```bash
clog subscriptions create \
  --source-id $SID \
  --channel-id $SHELL_CH \
  --config '{"command": "echo \"[${CHANGELOGUE_PROVIDER}] ${CHANGELOGUE_REPOSITORY} ${CHANGELOGUE_VERSION}\" >> ~/releases.log"}'
```

### Subscription Config Schema
```json
{
  "command": "...",
  "working_dir": "/override/channel/default",
  "timeout_seconds": 60
}
```
Fields on the subscription override the channel defaults. `command` is required on the subscription (no channel-level default — forces explicit intent per workflow).

**Model change:** Add a `Config json.RawMessage` field to `models.Subscription` and a `config TEXT` column to the `subscriptions` table (both PostgreSQL and SQLite schemas). This is a general-purpose extension point — non-shell subscriptions can ignore it.

### Environment Variables
Injected into the shell command's environment and available for `${VAR}` substitution in the command string:

- `CHANGELOGUE_VERSION` — release version string
- `CHANGELOGUE_REPOSITORY` — source repository (e.g., `library/nginx`)
- `CHANGELOGUE_PROVIDER` — provider name (e.g., `dockerhub`, `github`)
- `CHANGELOGUE_PROJECT` — project name
- `CHANGELOGUE_RELEASE_ID` — release UUID
- `CHANGELOGUE_SOURCE_ID` — source UUID
- `CHANGELOGUE_RAW_DATA` — full JSON payload

### Execution
- `exec.CommandContext` with configured timeout (subscription overrides channel default)
- Runs in a goroutine (fire-and-forget, non-blocking)
- Exit code and stderr logged, don't fail the notification
- Lives at `internal/routing/shell_sender.go`

## Synchronous Job Processing

Replaces River's async transactional outbox with synchronous in-process execution.

### Ingestion Flow
```
IngestRelease():
  1. Begin SQLite transaction
  2. INSERT INTO releases
  3. Commit
  4. Synchronously call notifyRelease()
```

### Notification Flow
`notifyRelease()` is a new function in `internal/stealth/notify.go` that reuses the `routing.Sender` implementations but not `routing.NotifyWorker` (which has River-specific types). It directly calls into the stealth store (which implements `routing.NotifyStore`) and dispatches via senders:

```
notifyRelease():
  1. Fetch release from SQLite (via stealth store)
  2. List subscriptions for the source
  3. For each subscription: resolve channel → dispatch via Sender
```

No River dependency. No retry queues. If a notification send fails, it's logged but doesn't roll back the release insertion.

## Ingestion Orchestrator Refactor

**Only change to existing code.** Extract a `SourceLister` interface so the orchestrator works with both PostgreSQL and SQLite:

```go
// internal/ingestion/orchestrator.go
type SourceLister interface {
    ListEnabledSources(ctx context.Context) ([]models.Source, error)
}
```

`SourceLoader` accepts a `SourceLister` instead of `*pgxpool.Pool`. Constructor changes:

```go
// Before
func NewOrchestrator(service *Service, loader *SourceLoader, pool *pgxpool.Pool, interval time.Duration) *Orchestrator

// After
func NewOrchestrator(service *Service, loader *SourceLoader, interval time.Duration) *Orchestrator
```

Both `PgStore` and `stealth.Store` implement `SourceLister`.

## Binary & Lifecycle

### Binary
`cmd/stealth/main.go` → builds to `clog-stealth`

### Startup Sequence
1. Open/create SQLite at `~/.changelogue/stealth.db`
2. Run migrations
3. Instantiate `stealth.Store`
4. Build `api.Dependencies` (unsupported stores get 501 stubs)
5. Register API routes via `api.RegisterRoutes(mux, deps)`
6. Start ingestion orchestrator in background goroutine
7. Listen on `localhost:9876` (configurable via `--port` / `CHANGELOGUE_STEALTH_PORT`)
8. Write PID file to `~/.changelogue/stealth.pid`

### Service Management Commands
- `clog-stealth install` — writes `launchd` plist (macOS) or `systemd` unit (Linux)
- `clog-stealth uninstall` — removes service file, stops process
- `clog-stealth status` — shows running state, port, DB path, source count
- `clog-stealth serve` — runs in foreground (for manual testing / debugging)

### CLI Integration
Existing `clog` CLI works unchanged:
```bash
export CHANGELOGUE_SERVER=http://localhost:9876
clog projects list
clog sources create --project-id ... --provider github --repository owner/repo
clog releases list
```

### Makefile
```makefile
stealth:         go build -o clog-stealth ./cmd/stealth
stealth-install: ./clog-stealth install
stealth-run:     ./clog-stealth serve
```

## Data Flow Summary

```
                 ┌─────────────┐
                 │  clog CLI   │ (agent uses this)
                 └──────┬──────┘
                        │ HTTP (localhost:9876)
                        ▼
              ┌─────────────────┐
              │  Stealth Server │
              │  (REST API)     │
              └────────┬────────┘
                       │
          ┌────────────┼────────────┐
          ▼            ▼            ▼
   ┌────────────┐ ┌────────┐ ┌──────────┐
   │ Orchestrator│ │ SQLite │ │ Senders  │
   │ (polling)   │ │  Store │ │ (shell,  │
   └──────┬─────┘ └────────┘ │  webhook)│
          │                   └────┬─────┘
          ▼                        ▼
   ┌────────────┐          ┌────────────┐
   │ Ingestion  │          │ Shell Cmd  │
   │ Sources    │          │ (callback) │
   │ (GitHub,   │          └────────────┘
   │  Docker,   │
   │  npm, etc) │
   └────────────┘
```

## File Structure

```
cmd/stealth/
  main.go              — binary entrypoint, service lifecycle

internal/stealth/
  store.go             — SQLite store (implements core interfaces)
  migrations.go        — SQLite schema
  notify.go            — synchronous notification dispatch (reuses Senders)
  stubs.go             — 501 stub implementations for unsupported interfaces

internal/routing/
  shell_sender.go      — ShellSender (new, implements Sender)

internal/ingestion/
  orchestrator.go      — extract SourceLister interface (refactor)
```
