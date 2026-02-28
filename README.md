<p align="center">
  <img src="docs/banner.svg" alt="Changelogue — Track what changed, understand why it matters" width="800" />
</p>

![Build](https://github.com/sentioxyz/releaseguard/actions/workflows/ci.yml/badge.svg)
![AI Co-Authored](https://img.shields.io/endpoint?url=https://gist.githubusercontent.com/Poytr1/94cc8f0ddf90bb1d04dafbb76102786d/raw/ai-commits.json)

## What it does

- **Discovers releases** by polling Docker Hub and GitHub on configurable intervals
- **Routes notifications** to Slack, Discord, and webhooks the moment a new version lands
- **Generates AI reports** via Google Gemini agents that research changelogs, assess risk, and summarize what changed
- **Serves a dashboard** (Next.js) for managing projects, sources, subscriptions, and browsing releases in real time

## Architecture

```mermaid
graph LR
    A["Ingestion<br/>(polling)"] --> B["PostgreSQL +<br/>River Queue"]
    B --> C["Agent<br/>(ADK-Go)"]
    C --> D["Routing<br/>(channels)"]

    A -.- A1["Docker Hub<br/>GitHub"]
    B -.- B1["Transactional Outbox<br/>LISTEN/NOTIFY → SSE"]
    C -.- C1["Gemini LLM<br/>research tools"]
    D -.- D1["Slack · Discord<br/>Webhooks"]
```

Release insert and job enqueue happen in a single SQL transaction — zero-loss guarantee.

See [ARCH.md](ARCH.md), [API.md](API.md), and [DESIGN.md](DESIGN.md) for the full design.

## Quick start

**Prerequisites:** Go 1.25+, Docker, Node.js 20+

```bash
# Start Postgres and the server (NO_AUTH mode)
make dev

# In another terminal — start the frontend
make frontend-install
make frontend-dev
```

The API runs on `localhost:8080`, the dashboard on `localhost:3000`.

## Environment variables

| Variable | Default | Purpose |
|----------|---------|---------|
| `DATABASE_URL` | `postgres://localhost:5432/changelogue?sslmode=disable` | PostgreSQL connection |
| `LISTEN_ADDR` | `:8080` | HTTP server bind address |
| `NO_AUTH` | _(unset)_ | Set to `true` to disable API key auth (development) |
| `LOG_LEVEL` | `info` | Log level: `debug`, `info`, `warn`, `error` |
| `LLM_PROVIDER` | `gemini` | LLM provider: `gemini` or `openai` |
| `LLM_MODEL` | `gemini-2.5-flash` / `gpt-5.2` | Model name (default depends on provider) |
| `GOOGLE_API_KEY` | _(empty)_ | Gemini API key (required when `LLM_PROVIDER=gemini`) |
| `OPENAI_API_KEY` | _(empty)_ | OpenAI API key (required when `LLM_PROVIDER=openai`) |
| `OPENAI_BASE_URL` | `https://api.openai.com/v1` | OpenAI-compatible API base URL |
| `OPENAI_SEARCH_MODEL` | `gpt-5-search-api` | Search-capable model for OpenAI web search sub-agent |

## Project structure

```
cmd/
  server/              Entry point — wires all layers together
  agent/               Agent CLI — run agent analysis for a project
internal/
  agent/               ADK-Go agent orchestrator, tools, and worker
    openai/            OpenAI-compatible LLM provider
  api/                 REST API, SSE, middleware, auth
  db/                  Connection pool and migrations
  ingestion/           Polling sources (Docker Hub, GitHub Atom)
  models/              Shared domain types
  queue/               River job definitions and client
  routing/             Notification channels and delivery worker
web/                   Next.js dashboard (React + Tailwind + shadcn)
scripts/               Integration test harness
```

## Extending

More providers (npm, PyPI, Helm, etc.) and channels (PagerDuty, email, etc.) are planned. Adding one is a single-interface implementation:

**Add a registry provider** — implement `IIngestionSource` in `internal/ingestion/source.go`:

```go
type IIngestionSource interface {
    FetchNewReleases(ctx context.Context) ([]IngestionResult, error)
}
```

**Add a notification channel** — implement `Sender` in `internal/routing/sender.go`:

```go
type Sender interface {
    Send(ctx context.Context, channel models.NotificationChannel, msg Message) error
}
```

## Useful commands

```bash
make build              # go build -o changelogue ./cmd/server
make test               # go test ./...
make vet                # go vet ./...
make lint               # alias for vet
make integration-test   # full integration test (spins up its own Postgres)
make db-reset           # drop and recreate the local database
make frontend-build     # build Next.js static export
make agent-dev          # run agent CLI for a specific project
make clean              # remove binary, stop containers, delete volumes
```
