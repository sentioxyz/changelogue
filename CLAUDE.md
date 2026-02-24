# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

**ReleaseBeacon** (internal name: ReleaseGuard) is an event-driven release management system that polls upstream registries (Docker Hub, GitHub) for new releases, processes them through a configurable pipeline, and routes notifications to downstream systems (Slack, PagerDuty, Ops Opsack).

Go module: `github.com/sentioxyz/releaseguard`

## Tech Stack

- **Backend:** Go 1.25 — polling engine, configurable pipeline, API server
- **Database/Queue/PubSub:** PostgreSQL + [River](https://github.com/riverqueue/river) v0.31.0 for job queue (`FOR UPDATE SKIP LOCKED`), native `LISTEN`/`NOTIFY` for real-time events
- **Frontend:** Next.js (React) + Tailwind CSS — dashboard (not yet started, `web/` is empty)
- **Intelligence:** LLMs (Gemini/GPT-4o-mini) via agent frameworks for changelog analysis and SRE validation (planned)
- **Deployment:** Single binary — Go `//go:embed` serves Next.js static export

## Build & Test Commands

```bash
# Build
go build -o releaseguard ./cmd/server

# Test all packages
go test ./...

# Test a single package
go test ./internal/ingestion/...
go test -v -run TestDockerHubSource ./internal/ingestion/...

# Vet
go vet ./...

# Integration tests (requires Docker)
bash scripts/integration-test.sh
```

## Environment Variables

| Variable | Default | Purpose |
|----------|---------|---------|
| `DATABASE_URL` | `postgres://localhost:5432/releaseguard?sslmode=disable` | PostgreSQL connection |
| `GITHUB_WEBHOOK_SECRET` | (empty) | HMAC-SHA256 verification for GitHub webhooks |
| `LISTEN_ADDR` | `:8080` | HTTP server bind address |

## Architecture

Four decoupled layers communicate exclusively through PostgreSQL:

1. **Ingestion Layer** *(implemented)* — Polling workers and webhook handlers behind `IIngestionSource` interface
2. **Processing Pipeline** *(planned)* — Sequential configurable nodes processing a `ReleaseEvent` IR
3. **Agentic Validation** *(planned)* — SRE agent for autonomous sandbox testing
4. **Routing & Notification** *(planned)* — Notification Matrix behind `INotificationChannel` interface

### Transactional Outbox Pattern

The critical data integrity pattern: release insert + River job enqueue happen in the same SQL transaction (`internal/ingestion/pgstore.go`). If either fails, both rollback — zero-loss guarantee.

### Key Interfaces (implemented)

- `IIngestionSource` (`internal/ingestion/source.go`) — Polling-based providers must implement `FetchLatest(ctx) ([]IngestionResult, error)`
- `ReleaseStore` (`internal/ingestion/store.go`) — Persistence abstraction with `Save(ctx, source, results) error`

### Key Interfaces (planned)

- `PipelineNode` — Pipeline processing stages
- `INotificationChannel` — Output providers (Slack, PagerDuty, webhooks)

### Data Flow

```
IIngestionSource.FetchLatest() → IngestionResult
    → IngestionService.ProcessResults() normalizes to ReleaseEvent IR
    → PgStore.Save() in single TX: INSERT release + River InsertTx(PipelineJobArgs)
    → River worker picks up job → pipeline processes → notifications sent
```

## Database

Schema lives in `internal/db/migrations.go` (idempotent, runs on startup). Current tables:
- `releases` — unique on `(repository, version)` for idempotent ingestion
- `subscriptions` — maps repositories to notification channels
- River's own tables (created by `rivermigrate`)

## Key Design References

- `ARCH.md` — Full architecture: system diagram, data flow lifecycle, pipeline design
- `DESIGN.md` — Component design: ReleaseEvent IR struct, database schema, SRE agent workflow, error handling
- `docs/designs/` — Feature-specific design docs (API design, etc.)
- `docs/plans/` — Implementation plans and task tracking

## Workflow Orchestration

### 1. Plan Node Default
- Enter plan mode for ANY non-trivial task (3+ steps or architectural decisions)
- If something goes sideways, STOP and re-plan immediately — don't keep pushing
- Use plan mode for verification steps, not just building
- Write detailed specs upfront to reduce ambiguity

### 2. Subagent Strategy
- Use subagents liberally to keep main context window clean
- Offload research, exploration, and parallel analysis to subagents
- For complex problems, throw more compute at it via subagents
- One task per subagent for focused execution

### 3. Self-Improvement Loop
- After ANY correction from the user: update `docs/plans/lessons.md` with the pattern
- Write rules for yourself that prevent the same mistake
- Ruthlessly iterate on these lessons until mistake rate drops
- Review lessons at session start for relevant project

### 4. Verification Before Done
- Never mark a task complete without proving it works
- Diff behavior between main and your changes when relevant
- Ask yourself: "Would a staff engineer approve this?"
- Run tests, check logs, demonstrate correctness

### 5. Demand Elegance (Balanced)
- For non-trivial changes: pause and ask "is there a more elegant way?"
- If a fix feels hacky: "Knowing everything I know now, implement the elegant solution"
- Skip this for simple, obvious fixes — don't over-engineer
- Challenge your own work before presenting it

### 6. Autonomous Bug Fixing
- When given a bug report: just fix it. Don't ask for hand-holding
- Point at logs, errors, failing tests — then resolve them
- Zero context switching required from the user
- Go fix failing CI tests without being told how

## Task Management

1. **Plan First**: Write plan to `docs/plans/todo.md` with checkable items
2. **Verify Plan**: Check in before starting implementation
3. **Track Progress**: Mark items complete as you go
4. **Explain Changes**: High-level summary at each step
5. **Document Results**: Add review section to `docs/plans/todo.md`
6. **Capture Lessons**: Update `docs/plans/lessons.md` after corrections

## Core Principles

- **Simplicity First**: Make every change as simple as possible. Impact minimal code.
- **No Laziness**: Find root causes. No temporary fixes. Senior developer standards.
- **Minimal Impact**: Changes should only touch what's necessary. Avoid introducing bugs.
