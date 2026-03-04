# Design: Migrate OpenAI Adapter to Responses API

**Date:** 2026-03-04
**Status:** Approved

## Problem

The current OpenAI adapter uses the Chat Completions API (`/v1/chat/completions`) with `web_search_options: {}` for web search. This has limitations:

- Requires a dedicated search-capable model (`gpt-5-search-api`) — separate from the main model
- No control over search parameters (domain filtering, search context size, user location)
- Chat Completions search models are being superseded by the Responses API

## Solution

Migrate the OpenAI adapter from Chat Completions to the Responses API (`/v1/responses`), which:

- Supports `web_search` as a native tool alongside function tools
- Works with any supported model (gpt-5, gpt-4.1, o4-mini, etc.)
- Provides domain filtering (`allowed_domains`), `search_context_size`, and `user_location`
- Eliminates the need for a separate search model

## Scope

### Files Changed

| File | Change |
|------|--------|
| `internal/agent/openai/openai.go` | Rewrite request/response types and `doRequest()` for Responses API |
| `internal/agent/openai/openai_test.go` | Update tests for new wire format |
| `internal/agent/openai/websearch.go` | Add configurable fields (SearchContextSize, AllowedDomains) |
| `internal/agent/model.go` | Remove `OpenAISearchModel` from `LLMConfig` |
| `internal/agent/orchestrator.go` | Simplify search sub-agent (same model, no separate model creation) |
| `cmd/server/main.go` | Remove `OPENAI_SEARCH_MODEL` env var |

### Request Format (Before → After)

**Before (Chat Completions):**
```json
{
  "model": "gpt-5.2",
  "messages": [{"role": "user", "content": "..."}],
  "tools": [{"type": "function", "function": {"name": "get_releases", ...}}],
  "web_search_options": {}
}
```

**After (Responses):**
```json
{
  "model": "gpt-5.2",
  "input": [{"role": "user", "content": "..."}],
  "tools": [
    {"type": "function", "name": "get_releases", "parameters": {...}},
    {"type": "web_search", "search_context_size": "medium"}
  ]
}
```

### Response Format (Before → After)

**Before:** `choices[0].message.content` + `choices[0].message.tool_calls`

**After:** `output` array containing:
- `{type: "web_search_call", status: "completed"}` — search invocation record
- `{type: "function_call", id, name, arguments}` — function tool calls
- `{type: "message", content: [{type: "output_text", text, annotations}]}` — final text with citations

### WebSearch Tool Enhancement

```go
type WebSearch struct {
    SearchContextSize string   // "low", "medium" (default), "high"
    AllowedDomains    []string // optional domain filter
}
```

The `ProcessRequest` method will encode these fields into the tool config rather than using the GoogleSearch marker hack.

### Config Simplification

**Removed from LLMConfig:**
- `OpenAISearchModel` — no longer needed; any model supports web search via tools

**Removed env vars:**
- `OPENAI_SEARCH_MODEL` — search model is the same as the main model

### Orchestrator Simplification

The search sub-agent no longer needs a separate model. In `BuildAgent()`:

```go
// Before: separate model for search
searchModel, err = NewLLMModel(ctx, LLMConfig{
    Provider: "openai", Model: "gpt-5-search-api", ...
})

// After: same model, web search is just a tool
searchAgent = llmagent.New(llmagent.Config{
    Model: llmModel,  // same model as root/data agent
    Tools: []tool.Tool{oaimodel.WebSearch{SearchContextSize: "medium"}},
})
```

## Non-Goals

- Streaming support (not implemented today, stays that way)
- Deep research models (o3-deep-research etc.)
- Gemini adapter changes (unaffected)
