# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

**ReleaseBeacon** (internal name: ReleaseGuard) is an event-driven release management system that polls upstream registries (Docker Hub, GitHub) for new releases, processes them through an intelligent pipeline, and routes notifications to downstream systems (Slack, PagerDuty, Ops Opsack).

The project is in its initial phase — `ARCH.md` and `DESIGN.md` contain the full specification. No source code exists yet.

## Planned Tech Stack

- **Backend:** Go — polling engine, DAG pipeline, API server
- **Frontend:** Next.js (React) + Tailwind CSS — dashboard for release streams and configuration
- **Database/Queue/PubSub:** PostgreSQL — River library for job queue (`FOR UPDATE SKIP LOCKED`), native `LISTEN`/`NOTIFY` for real-time events
- **Intelligence:** LLMs (Gemini/GPT-4o-mini) via agent frameworks for changelog analysis and SRE validation
- **Deployment:** Single binary — Go `//go:embed` serves Next.js static export

## Architecture (Key Concepts)

Four decoupled layers communicate exclusively through PostgreSQL:

1. **Ingestion Layer** — Polling workers and webhook handlers behind `IIngestionSource` interface
2. **DAG Processing Pipeline** — Sequential nodes (Regex Normalizer → Subscription Router → AI Urgency Scorer → Validation Trigger) processing a `ReleaseEvent` IR
3. **Agentic Validation** — SRE agent with tools (`GetBaseABoxConfig`, `DraftConfigUpgrade`, `DeploySandbox`, `QueryAgentStatus`) for autonomous sandbox testing
4. **Routing & Notification** — Notification Matrix behind `INotificationChannel` interface

Critical patterns:
- **Transactional Outbox** — release insert + job queue within same SQL transaction (zero-loss)
- **Idempotent ingestion** — unique constraint on `(repository, version)`
- **Dead-Letter Queue** — failed jobs after 3 attempts go to `discarded` state

## Planned Directory Structure

```
cmd/server/          — Main Go entry point
internal/
  ingestion/         — Polling workers, webhook handlers (IIngestionSource)
  pipeline/          — DAG node implementations
  agents/            — LLM orchestration and validation tools
  routing/           — Notification matrix, output providers (INotificationChannel)
  queue/             — River queue setup and job definitions
  models/            — Shared domain structs (ReleaseEvent, SemanticData, etc.)
web/                 — Next.js frontend
deployments/         — Dockerfiles, Base A Box integration scripts
```

## Build Commands (to be established)

```bash
# Go backend
go build -o releaseguard ./cmd/server
go test ./...
go vet ./...

# Next.js frontend
cd web && npm install && npm run build

# Single binary with embedded frontend
# Build web first, then go build (embeds web/out/)
```

## Key Design References

- `ARCH.md` — Full architecture: system diagram, data flow lifecycle, DAG pipeline design
- `DESIGN.md` — Component design: ReleaseEvent IR struct, database schema, SRE agent workflow, error handling

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
- After ANY correction from the user: update `tasks/lessons.md` with the pattern
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

1. **Plan First**: Write plan to `tasks/todo.md` with checkable items
2. **Verify Plan**: Check in before starting implementation
3. **Track Progress**: Mark items complete as you go
4. **Explain Changes**: High-level summary at each step
5. **Document Results**: Add review section to `tasks/todo.md`
6. **Capture Lessons**: Update `tasks/lessons.md` after corrections

## Core Principles

- **Simplicity First**: Make every change as simple as possible. Impact minimal code.
- **No Laziness**: Find root causes. No temporary fixes. Senior developer standards.
- **Minimal Impact**: Changes should only touch what's necessary. Avoid introducing bugs.
