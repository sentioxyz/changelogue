# CLAUDE.md

## Core Principles

- **Simplicity First**: Make every change as simple as possible. Impact minimal code.
- **No Laziness**: Find root causes. No temporary fixes. Senior developer standards.
- **Minimal Impact**: Changes should only touch what's necessary. Avoid introducing bugs.

## Project Overview

**Changelogue** is an agent-driven release intelligence platform that polls upstream registries (Docker Hub, GitHub, ECR Public, GitLab, PyPI, npm) for new releases, sends source-level notifications, and uses LLM agents (ADK-Go) to produce semantic release reports.

Go module: `github.com/sentioxyz/changelogue`

## Tech Stack

- **Backend:** Go 1.25 — polling engine, notification routing, API server
- **Database/Queue/PubSub:** PostgreSQL + [River](https://github.com/riverqueue/river) v0.31.0 for job queue, native `LISTEN`/`NOTIFY` for real-time events
- **Frontend:** Next.js (React) + Tailwind CSS — dashboard (`web/`)
- **Agent Intelligence:** [Google ADK-Go](https://google.github.io/adk-go/) v0.5.0 — Gemini/OpenAI-powered semantic release analysis
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
make up                  # Start Postgres (docker-compose)
make run                 # Run server (builds + starts with NO_AUTH, requires Postgres)
make run-auth            # Run server with GitHub OAuth enabled
make dev                 # One command: start Postgres + run server
make db-reset            # Reset database
make cli                 # Build CLI binary (clog)
make agent-dev           # Run agent CLI for a specific project
make frontend-install    # Install frontend deps
make frontend-dev        # Run frontend dev server
make integration-test    # Integration tests (own Postgres on port 5433)
make release VERSION=x   # Tag and push a release (triggers GoReleaser in CI)
make release-dry-run     # Test GoReleaser locally without publishing
make clean               # Cleanup everything (binary + containers + volumes)
```

## Key Docs

- `ARCH.md` — Architecture, data flow, key interfaces, system diagram
- `DESIGN.md` — Database schema, agent workflow, error handling
- `API.md` — REST API endpoints and examples
- `README.md` — Environment variables, deployment, setup
- `docs/plans/lessons.md` — Accumulated lessons from past corrections

## Workflow

### Planning & Execution
- Enter plan mode for ANY non-trivial task (3+ steps or architectural decisions)
- Write plan to `docs/plans/todo.md` with checkable items; check in before starting
- If something goes sideways, STOP and re-plan immediately
- Mark items complete as you go; add review section when done

### Subagents
- Use subagents liberally to keep main context window clean
- Offload research, exploration, and parallel analysis to subagents
- One task per subagent for focused execution

### Verification
- Never mark a task complete without proving it works
- Run tests, check logs, demonstrate correctness
- Ask yourself: "Would a staff engineer approve this?"

### Self-Improvement
- After ANY correction: update `docs/plans/lessons.md` with the pattern
- Review lessons at session start

### Bug Fixing
- When given a bug report: just fix it autonomously
- Point at logs, errors, failing tests — then resolve them
