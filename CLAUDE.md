# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

**Changelogue** is an agent-driven release intelligence platform that polls upstream registries (Docker Hub, GitHub) for new releases, sends source-level notifications, and uses LLM agents (ADK-Go) to produce semantic release reports.

Go module: `github.com/sentioxyz/changelogue`

## Tech Stack

- **Backend:** Go 1.25 — polling engine, notification routing, API server
- **Database/Queue/PubSub:** PostgreSQL + [River](https://github.com/riverqueue/river) v0.31.0 for job queue (`FOR UPDATE SKIP LOCKED`), native `LISTEN`/`NOTIFY` for real-time events
- **Frontend:** Next.js (React) + Tailwind CSS — dashboard (`web/`)
- **Agent Intelligence:** [Google ADK-Go](https://google.github.io/adk-go/) v0.5.0 for agent orchestration — Gemini-powered semantic release analysis
- **Deployment:** Single binary — Go `//go:embed` serves Next.js static export

## Build & Test Commands

```bash
# Build
go build -o changelogue ./cmd/server

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

## Local Development

```bash
# Start Postgres (docker-compose)
make up

# Run server (builds + starts with NO_AUTH, requires Postgres)
make run

# One command: start Postgres + run server
make dev

# Reset database
make db-reset

# Frontend
make frontend-install
make frontend-dev

# Integration tests (spins up its own Postgres on port 5433)
make integration-test

# Cleanup everything (binary + containers + volumes)
make clean
```

## Environment Variables

| Variable | Default | Purpose |
|----------|---------|---------|
| `DATABASE_URL` | `postgres://localhost:5432/changelogue?sslmode=disable` | PostgreSQL connection |
| `LISTEN_ADDR` | `:8080` | HTTP server bind address |
| `GOOGLE_API_KEY` | (empty) | Gemini API key for agent LLM (required when `LLM_PROVIDER=gemini`) |
| `LLM_PROVIDER` | `gemini` | LLM provider: `gemini` or `openai` |
| `LLM_MODEL` | `gemini-2.5-flash` (gemini) / `gpt-5.2` (openai) | Model name |
| `OPENAI_API_KEY` | (empty) | OpenAI API key (required when `LLM_PROVIDER=openai`) |
| `OPENAI_BASE_URL` | `https://api.openai.com/v1` | OpenAI-compatible API base URL |

## Architecture

Four decoupled layers communicate exclusively through PostgreSQL:

1. **Ingestion Layer** — Polling workers behind `IIngestionSource` interface discover new releases from upstream registries
2. **Notification Routing** — River `NotifyWorker` sends source-level notifications to subscribed channels on new releases
3. **Agent Layer** — ADK-Go agent (`internal/agent/`) researches releases via project-scoped tools and produces semantic release reports
4. **Routing & Output** — Notification channels (Slack, Discord, webhooks) behind `Sender` interface

### Transactional Outbox Pattern

The critical data integrity pattern: release insert + River job enqueue happen in the same SQL transaction (`internal/ingestion/pgstore.go`). If either fails, both rollback — zero-loss guarantee.

### Key Interfaces

- `IIngestionSource` (`internal/ingestion/source.go`) — Polling-based providers implement `FetchNewReleases(ctx) ([]IngestionResult, error)`
- `ReleaseStore` (`internal/ingestion/store.go`) — Persistence: `IngestRelease(ctx, sourceID string, result *IngestionResult) error`
- `Sender` (`internal/routing/sender.go`) — Notification channel output: `Send(ctx, channel, msg) error`
- `NotifyStore` (`internal/routing/worker.go`) — Data access for the notification worker
- `AgentDataStore` (`internal/agent/tools.go`) — Data access for agent tools (releases, context sources)
- `OrchestratorStore` (`internal/agent/orchestrator.go`) — Full data access for agent orchestrator (project, agent runs, semantic releases)

### Data Flow

```
IIngestionSource.FetchNewReleases() → IngestionResult
    → Service.ProcessResults() processes results
    → PgStore.IngestRelease() in single TX: INSERT release + River InsertTx(NotifyJobArgs)
    → NotifyWorker picks up job → sends source notifications + checks agent rules
    → If rules match → enqueue AgentJobArgs
    → AgentWorker picks up → runs ADK-Go agent → creates semantic release
    → Sends project notifications
```

## Database

Schema lives in `internal/db/migrations.go` (idempotent, runs on startup). Current tables:
- `projects` — tracked software projects (central entity)
- `sources` — configured ingestion sources per project (provider, repository, poll interval)
- `context_sources` — read-only references for agent research (runbooks, docs)
- `releases` — source-level releases, unique on `(source_id, version)`
- `semantic_releases` — AI-generated project-level reports, unique on `(project_id, version)`
- `semantic_release_sources` — join table linking semantic releases to source releases
- `notification_channels` — standalone channels (webhook, Slack, Discord)
- `subscriptions` — source-level and project-level notification subscriptions
- `agent_runs` — agent execution records scoped to a project
- `api_keys` — API authentication keys
- River's own tables (created by `rivermigrate`)

## Key Design References

- `ARCH.md` — Full architecture: system diagram, data flow lifecycle
- `DESIGN.md` — Component design: database schema, agent workflow, error handling

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
