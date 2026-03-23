# CLI Support for Changelogue Core Functionalities

**Date:** 2026-03-23
**Status:** Approved

## Summary

Add a separate CLI binary (`changelogue-cli`) that provides CRUD management of Changelogue resources by calling the existing REST API. The CLI uses Cobra for command routing, authenticates via API key, and outputs human-readable tables by default with a `--json` flag for scripting. Mistyped commands produce fuzzy-matched suggestions.

## Decisions

| Decision | Choice | Rationale |
|----------|--------|-----------|
| Scope | Projects, sources, releases, channels, subscriptions | Core management operations; ops triggers deferred |
| Communication | REST API client | Decoupled from server internals, works remotely |
| Framework | Cobra | Industry standard, built-in suggestions, completions |
| Binary | Separate `cmd/cli/main.go` | Clean separation from server binary |
| Output | Table + `--json` flag | Human-readable default, machine-readable option |
| Auth | API key via flag/env var | Matches existing `Authorization: Bearer` auth |

## Command Tree

```
changelogue-cli
├── projects
│   ├── list                    # GET /api/v1/projects
│   ├── get <id>                # GET /api/v1/projects/{id}
│   ├── create --name <n> ...   # POST /api/v1/projects
│   ├── update <id> ...         # PUT /api/v1/projects/{id}
│   └── delete <id>             # DELETE /api/v1/projects/{id}
├── sources
│   ├── list --project <id>     # GET /api/v1/projects/{id}/sources
│   ├── get <id>                # GET /api/v1/sources/{id}
│   ├── create --project <id> --provider <p> --repository <r> ...
│   │                           # POST /api/v1/projects/{id}/sources
│   ├── update <id> ...         # PUT /api/v1/sources/{id}
│   └── delete <id>             # DELETE /api/v1/sources/{id}
├── releases
│   ├── list [--source <id>] [--project <id>]
│   │                           # GET /api/v1/releases (or scoped variants)
│   └── get <id>                # GET /api/v1/releases/{id}
├── channels
│   ├── list                    # GET /api/v1/channels
│   ├── get <id>                # GET /api/v1/channels/{id}
│   ├── create --name <n> --type <t> --config <json>
│   │                           # POST /api/v1/channels
│   ├── update <id> ...         # PUT /api/v1/channels/{id}
│   ├── delete <id>             # DELETE /api/v1/channels/{id}
│   └── test <id>               # POST /api/v1/channels/{id}/test
├── subscriptions
│   ├── list                    # GET /api/v1/subscriptions
│   ├── create --channel <id> --type <t> [--source <id>] [--project <id>]
│   │                           # POST /api/v1/subscriptions
│   ├── delete <id>             # DELETE /api/v1/subscriptions/{id}
│   └── batch-create --channel <id> --type <t> --project-ids <ids...>
│                               # POST /api/v1/subscriptions/batch
└── version                     # Print CLI version
```

### Global Flags

| Flag | Env Var | Default | Description |
|------|---------|---------|-------------|
| `--server` | `CHANGELOGUE_SERVER` | `http://localhost:8080` | Server base URL |
| `--api-key` | `CHANGELOGUE_API_KEY` | (none) | API key for authentication |
| `--json` | — | `false` | Output raw JSON instead of table |
| `--page` | — | `1` | Page number (list commands) |
| `--per-page` | — | `25` | Items per page (list commands) |

## AI-Friendly Command Hints

When a user mistypes a command or flag, the CLI provides intelligent suggestions using Levenshtein distance fuzzy matching.

### Behaviors

1. **Unknown command** — suggest closest match from valid sibling commands:
   ```
   $ changelogue-cli projet list
   Error: unknown command "projet"

   Did you mean?
     changelogue-cli projects list
   ```

2. **Unknown subcommand** — suggest closest match within the resource group:
   ```
   $ changelogue-cli projects lst
   Error: unknown command "lst" for "changelogue-cli projects"

   Did you mean?
     changelogue-cli projects list
   ```

3. **Unknown flag** — suggest closest valid flag:
   ```
   $ changelogue-cli sources create --project abc --repo myimage
   Error: unknown flag "--repo"

   Did you mean?
     --repository    The source repository (e.g., "library/postgres")
   ```

4. **Missing required flag** — show usage with example:
   ```
   $ changelogue-cli projects create
   Error: required flag "--name" not set

   Usage:
     changelogue-cli projects create --name <name> [--description <desc>]

   Example:
     changelogue-cli projects create --name "My Project" --description "Tracks postgres"
   ```

5. **Resource group with no subcommand** — list available subcommands:
   ```
   $ changelogue-cli channels
   Available commands:
     list      List all notification channels
     get       Get channel details by ID
     create    Create a new notification channel
     ...
   ```

### Implementation

- Cobra's `SuggestionsMinimumDistance` set to 2
- Each command has `Short` (one-line) and `Long` (with examples) descriptions
- `RunE` on group commands prints subcommand list
- Custom `FlagErrorFunc` on root to show flag suggestions

## Architecture

### File Layout

```
cmd/cli/
  main.go                # Root Cobra command, global flags, version cmd

internal/cli/
  client.go              # HTTP client wrapper (base URL, auth, JSON helpers)
  output.go              # Table (tabwriter) and JSON rendering
  projects.go            # projects subcommand tree
  sources.go             # sources subcommand tree
  releases.go            # releases subcommand tree
  channels.go            # channels subcommand tree
  subscriptions.go       # subscriptions subcommand tree
```

### HTTP Client

`client.go` wraps `net/http.Client`:

```go
type Client struct {
    BaseURL    string
    APIKey     string
    HTTPClient *http.Client
}
```

- Methods: `Get(path)`, `Post(path, body)`, `Put(path, body)`, `Delete(path)`
- All methods return `(*http.Response, error)` — callers decode the envelope
- Sets `Authorization: Bearer <key>` header on all requests
- Sets `Content-Type: application/json` on POST/PUT
- Handles HTTP error responses with user-friendly messages:
  - 401 → "Authentication failed. Check your --api-key or CHANGELOGUE_API_KEY."
  - 404 → "Resource not found."
  - 429 → "Rate limited. Retry after N seconds."
  - 5xx → "Server error: <message>"

### Response Decoding

Reuse `internal/models` types. Define envelope types in `client.go`:

```go
type APIResponse[T any] struct {
    Data T    `json:"data"`
    Meta Meta `json:"meta"`
}

type Meta struct {
    RequestID string `json:"request_id"`
    Page      int    `json:"page,omitempty"`
    PerPage   int    `json:"per_page,omitempty"`
    Total     int    `json:"total,omitempty"`
}

type APIError struct {
    Error struct {
        Code    string `json:"code"`
        Message string `json:"message"`
    } `json:"error"`
}
```

### Output Rendering

`output.go` provides:

- `RenderJSON(data any)` — marshals to indented JSON, prints to stdout
- `RenderTable(headers []string, rows [][]string)` — tabwriter-based table
- Each resource file provides a `toTableRows` function mapping model → string columns

### Build

Add to Makefile:
```makefile
cli:
	go build -o changelogue-cli ./cmd/cli
```

## Testing Strategy

- **Unit tests** for `client.go` using `httptest.NewServer` to mock API responses
- **Unit tests** for each subcommand — verify correct HTTP method/path/body sent
- **Unit tests** for `output.go` — verify table and JSON formatting
- **Integration test** (optional) — start real server, run CLI commands against it

## Dependencies

- `github.com/spf13/cobra` — CLI framework
- No other new dependencies. Uses stdlib `text/tabwriter`, `net/http`, `encoding/json`.

## Out of Scope

- Operational triggers (poll, agent run, scan)
- Health/stats endpoints
- Shell completions (can be added later via `cobra.GenBashCompletion`)
- Config file (~/.changelogue/config.yaml)
- Interactive prompts
