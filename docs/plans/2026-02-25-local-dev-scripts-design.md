# Local Dev Scripts & Integration Tests Design

**Date:** 2026-02-25

## Goal

Add scripts to easily build and launch the stack locally, plus comprehensive integration tests covering API + webhook end-to-end flows.

## Components

### 1. `docker-compose.yml` — Local infrastructure

Postgres 16 service with healthcheck. App runs natively for fast iteration.

### 2. `Makefile` — Unified command interface

| Target | Description |
|--------|-------------|
| `up` / `down` | Start/stop docker-compose services |
| `db-reset` | Drop and recreate database |
| `build` | `go build -o releaseguard ./cmd/server` |
| `run` | Build + run server (depends on `up`) |
| `dev` | Start Postgres + run server (single command) |
| `test` | `go test ./...` |
| `vet` | `go vet ./...` |
| `lint` | Vet + checks |
| `frontend-install` | `npm install` in `web/` |
| `frontend-dev` | `npm run dev` in `web/` |
| `frontend-build` | `npm run build` in `web/` |
| `integration-test` | Run full integration test suite |

### 3. `scripts/integration-test.sh` — Updated & expanded

- Fix outdated job kind (`pipeline_process` → `notify_release`)
- Add API CRUD tests (projects, sources, subscriptions, channels)
- Add release flow test (project → source → webhook → release + job)
- Use docker-compose Postgres when available, fall back to own container
- Structured pass/fail output with summary

## Decisions

- **Bash integration tests** over Go testcontainers: simpler, no new deps, good for HTTP-level testing
- **Native app execution** over containerized: faster iteration, better debugging
- **docker-compose for deps only**: Postgres is the only external dependency
