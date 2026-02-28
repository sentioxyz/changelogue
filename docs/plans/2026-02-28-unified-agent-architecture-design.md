# Unified Agent Architecture Design

## Goal

Unify the agent flow so both Gemini and OpenAI providers use the same sub-agent architecture via `agenttool`. Currently Gemini uses a 3-agent hierarchy (root + data sub-agent + search sub-agent) while OpenAI uses a flat architecture with all tools on the root agent. After this change, both providers share the same structure — the only difference is which search tool the search sub-agent receives.

## Architecture

```
Root Agent (release_analyst)
├── Data Sub-Agent (data_agent) — via agenttool
│   └── functiontools: get_releases, get_release_detail, list_context_sources
└── Search Sub-Agent (search_agent) — via agenttool
    └── Gemini: geminitool.GoogleSearch{}
        OpenAI: openaitool.WebSearch{}  ← new
```

`BuildAgent()` becomes a single code path. It builds a provider-specific `searchTool` and passes it to the same sub-agent structure.

## OpenAI Web Search

OpenAI's Chat Completions web search uses `web_search_options: {}` as a top-level request parameter (not a tool in the `tools` array). This mirrors how Gemini's `GoogleSearch` is a server-side grounding feature.

### New type: `WebSearch` in `internal/agent/openai/websearch.go`

Implements `tool.Tool`. Key behaviors:

- `Declaration()` returns `nil` (not a callable function)
- `ProcessRequest()` adds `genai.Tool{GoogleSearch: &genai.GoogleSearch{}}` to the request config — reusing the same marker that `geminitool.GoogleSearch{}` uses
- `Run()` returns an error (never called directly; search happens inside the LLM call)

The OpenAI wrapper detects `GoogleSearch != nil` on any `genai.Tool` entry and translates it to `web_search_options: {}` in the HTTP body.

### Model constraint

`web_search_options` only works with search-capable models (`gpt-4o-search-preview`, `gpt-4o-mini-search-preview`, `gpt-5-search-api`). The search sub-agent gets its own model instance with a search-capable model name.

## OpenAI Wrapper Changes (`internal/agent/openai/openai.go`)

### 1. Schema type lowercasing in `schemaToMap()`

`agenttool` generates schemas with uppercase types (`"OBJECT"`, `"STRING"`). OpenAI expects lowercase. Fix: `strings.ToLower(string(s.Type))`.

### 2. Web search options in `chatRequest`

```go
type chatRequest struct {
    // ... existing fields ...
    WebSearchOptions *webSearchOptions `json:"web_search_options,omitempty"`
}

type webSearchOptions struct{}
```

### 3. Detect GoogleSearch marker in `doRequest()`

Before building the HTTP request, check if any `genai.Tool` has `GoogleSearch` set. If found, set `chatReq.WebSearchOptions = &webSearchOptions{}` and skip that tool entry in `convertTools()`.

## Config Changes

New env var:

| Variable | Default | Purpose |
|----------|---------|---------|
| `OPENAI_SEARCH_MODEL` | `gpt-5-search-api` | Search-capable model for OpenAI search sub-agent |

New field in `LLMConfig`:

```go
OpenAISearchModel string
```

Wired in `cmd/server/main.go` and `cmd/agent/main.go`. For Gemini, this field is ignored.

## Test Strategy

### Unit tests

1. `schemaToMap()` — verify `"OBJECT"` → `"object"`, `"STRING"` → `"string"`
2. `convertTools()` with GoogleSearch marker — verify `web_search_options` emitted, marker skipped
3. `WebSearch.ProcessRequest()` — verify correct `genai.Tool` entry added
4. `WebSearch.Declaration()` — verify returns `nil`

### Manual integration test

Run `cmd/agent` with `LLM_PROVIDER=openai` and verify:
- Data sub-agent tool calls work (agenttool round-trip)
- Search sub-agent triggers web search
- Final semantic report produced correctly

## Non-goals

- No new external dependencies
- No database schema changes
- No changes to Gemini flow behavior
