# ADK-Web Dev Entrypoint Design

## Goal

Add a dev/debug entrypoint that serves the ADK-Web UI for interactive agent development. The existing production agent flow (River worker) is untouched.

## Approach

New standalone binary at `cmd/agent/main.go` using ADK-Go's `launcher/full` package.

## Architecture

```
cmd/agent/main.go
    ├── Parse --project-id flag
    ├── Connect to Postgres (DATABASE_URL)
    ├── Validate project exists in DB
    ├── Read LLM config from env vars
    ├── Create LLM model via existing NewLLMModel()
    ├── Create project-scoped tools via existing NewTools()
    ├── Create agent via llmagent.New()
    ├── Wrap in agent.NewSingleLoader()
    └── full.NewLauncher().Execute() → serves adk-web UI
```

## Data Flow

```
User ──browser──▶ adk-web UI (localhost:8080)
                      │
                      ▼
              ADK Launcher (API server)
                      │
                      ▼
              llmagent "release_analyst"
                 ├── get_releases        ──▶ PgStore ──▶ PostgreSQL
                 ├── get_release_detail   ──▶ PgStore ──▶ PostgreSQL
                 └── list_context_sources ──▶ PgStore ──▶ PostgreSQL
                      │
                      ▼
              LLM (Gemini / OpenAI)
```

## New File: `cmd/agent/main.go`

Responsibilities:
1. `--project-id` flag (required, validated against DB)
2. `DATABASE_URL` env var → `db.NewPool()`
3. `PgStore` with `river=nil` (no queue needed)
4. Load project to build instruction (custom prompt + default)
5. `NewLLMModel()` with same env var config as server
6. `NewTools(store, projectID)` for project-scoped tools
7. `llmagent.New()` with same config as orchestrator
8. `agent.NewSingleLoader()` → `launcher.Config`
9. `full.NewLauncher().Execute(ctx, config, remainingArgs)`

## Makefile Target

```makefile
agent-dev:
	DATABASE_URL=$(DATABASE_URL) go run ./cmd/agent \
		--project-id=$(PROJECT_ID) web api webui
```

Usage: `make agent-dev PROJECT_ID=<uuid>`

## Dependencies

- `google.golang.org/adk/cmd/launcher` (new import)
- `google.golang.org/adk/cmd/launcher/full` (new import)
- `google.golang.org/adk/agent` (already used)

## What's NOT Changing

- `cmd/server/main.go` — untouched
- `internal/agent/*` — untouched (reused as-is)
- Production agent flow via River — untouched
