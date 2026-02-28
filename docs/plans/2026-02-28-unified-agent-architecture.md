# Unified Agent Architecture Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Unify the Gemini and OpenAI agent flows to use the same sub-agent architecture via `agenttool`, with provider-specific search tools.

**Architecture:** Both providers use a 3-agent pattern: root → data sub-agent + search sub-agent. The search sub-agent uses `geminitool.GoogleSearch{}` for Gemini and a new `openai.WebSearch{}` for OpenAI. The OpenAI wrapper translates the `GoogleSearch` marker on `genai.Tool` into `web_search_options: {}` in the HTTP body.

**Tech Stack:** Go, ADK-Go v0.5.0, OpenAI Chat Completions API

---

### Task 1: Fix schema type casing in OpenAI wrapper

**Files:**
- Modify: `internal/agent/openai/openai.go:301-305` (schemaToMap function)
- Create: `internal/agent/openai/openai_test.go`

**Step 1: Write the failing test**

Create `internal/agent/openai/openai_test.go`:

```go
package openai

import (
	"testing"

	"google.golang.org/genai"
)

func TestSchemaToMap_LowercasesType(t *testing.T) {
	s := &genai.Schema{
		Type: "OBJECT",
		Properties: map[string]*genai.Schema{
			"request": {Type: "STRING"},
		},
		Required: []string{"request"},
	}
	m := schemaToMap(s)

	typ, ok := m["type"].(string)
	if !ok {
		t.Fatal("type field missing or wrong type")
	}
	if typ != "object" {
		t.Errorf("expected type 'object', got %q", typ)
	}

	props := m["properties"].(map[string]any)
	reqProp := props["request"].(map[string]any)
	reqType := reqProp["type"].(string)
	if reqType != "string" {
		t.Errorf("expected nested type 'string', got %q", reqType)
	}
}
```

**Step 2: Run test to verify it fails**

Run: `go test -v -run TestSchemaToMap_LowercasesType ./internal/agent/openai/...`
Expected: FAIL — `expected type 'object', got "OBJECT"`

**Step 3: Write minimal implementation**

In `internal/agent/openai/openai.go`, add `"strings"` to imports and change line 304:

```go
// Before:
m["type"] = string(s.Type)

// After:
m["type"] = strings.ToLower(string(s.Type))
```

**Step 4: Run test to verify it passes**

Run: `go test -v -run TestSchemaToMap_LowercasesType ./internal/agent/openai/...`
Expected: PASS

**Step 5: Commit**

```bash
git add internal/agent/openai/openai.go internal/agent/openai/openai_test.go
git commit -m "fix(openai): lowercase schema types for OpenAI compatibility"
```

---

### Task 2: Add web search options support to OpenAI wrapper

**Files:**
- Modify: `internal/agent/openai/openai.go` (chatRequest struct, convertTools, doRequest)
- Modify: `internal/agent/openai/openai_test.go`

**Step 1: Write the failing test**

Add to `internal/agent/openai/openai_test.go`:

```go
func TestConvertTools_SkipsGoogleSearchMarker(t *testing.T) {
	req := &model.LLMRequest{
		Config: &genai.GenerateContentConfig{
			Tools: []*genai.Tool{
				{
					FunctionDeclarations: []*genai.FunctionDeclaration{
						{Name: "get_releases", Description: "Fetch releases"},
					},
				},
				{
					GoogleSearch: &genai.GoogleSearch{},
				},
			},
		},
	}

	tools := convertTools(req)

	// Should only contain the function tool, not the GoogleSearch marker.
	if len(tools) != 1 {
		t.Fatalf("expected 1 tool, got %d", len(tools))
	}
	if tools[0].Function.Name != "get_releases" {
		t.Errorf("expected tool name 'get_releases', got %q", tools[0].Function.Name)
	}
}

func TestHasWebSearch(t *testing.T) {
	t.Run("with GoogleSearch marker", func(t *testing.T) {
		req := &model.LLMRequest{
			Config: &genai.GenerateContentConfig{
				Tools: []*genai.Tool{
					{GoogleSearch: &genai.GoogleSearch{}},
				},
			},
		}
		if !hasWebSearch(req) {
			t.Error("expected hasWebSearch to return true")
		}
	})

	t.Run("without GoogleSearch marker", func(t *testing.T) {
		req := &model.LLMRequest{
			Config: &genai.GenerateContentConfig{
				Tools: []*genai.Tool{
					{FunctionDeclarations: []*genai.FunctionDeclaration{{Name: "test"}}},
				},
			},
		}
		if hasWebSearch(req) {
			t.Error("expected hasWebSearch to return false")
		}
	})

	t.Run("nil config", func(t *testing.T) {
		req := &model.LLMRequest{}
		if hasWebSearch(req) {
			t.Error("expected hasWebSearch to return false for nil config")
		}
	})
}
```

**Step 2: Run tests to verify they fail**

Run: `go test -v -run "TestConvertTools_SkipsGoogleSearchMarker|TestHasWebSearch" ./internal/agent/openai/...`
Expected: FAIL — `hasWebSearch` undefined, `convertTools` includes GoogleSearch

**Step 3: Write minimal implementation**

In `internal/agent/openai/openai.go`:

1. Add `webSearchOptions` type and update `chatRequest`:

```go
type webSearchOptions struct{}

type chatRequest struct {
	Model            string            `json:"model"`
	Messages         []chatMessage     `json:"messages"`
	Tools            []chatTool        `json:"tools,omitempty"`
	Temperature      *float32          `json:"temperature,omitempty"`
	TopP             *float32          `json:"top_p,omitempty"`
	MaxTokens        int32             `json:"max_tokens,omitempty"`
	Stop             []string          `json:"stop,omitempty"`
	Stream           bool              `json:"stream"`
	WebSearchOptions *webSearchOptions `json:"web_search_options,omitempty"`
}
```

2. Add `hasWebSearch` helper:

```go
// hasWebSearch checks if any genai.Tool in the request has GoogleSearch set.
func hasWebSearch(req *model.LLMRequest) bool {
	if req.Config == nil {
		return false
	}
	for _, t := range req.Config.Tools {
		if t != nil && t.GoogleSearch != nil {
			return true
		}
	}
	return false
}
```

3. Update `convertTools` to skip GoogleSearch entries:

```go
func convertTools(req *model.LLMRequest) []chatTool {
	if req.Config == nil || len(req.Config.Tools) == 0 {
		return nil
	}

	var tools []chatTool
	for _, t := range req.Config.Tools {
		// Skip GoogleSearch markers — handled via web_search_options.
		if t.GoogleSearch != nil {
			continue
		}
		for _, fd := range t.FunctionDeclarations {
			ct := chatTool{
				Type: "function",
				Function: chatFunction{
					Name:        fd.Name,
					Description: fd.Description,
				},
			}
			if fd.Parameters != nil {
				ct.Function.Parameters = schemaToMap(fd.Parameters)
			}
			tools = append(tools, ct)
		}
	}
	return tools
}
```

4. Update `doRequest` to set `WebSearchOptions` when detected:

```go
// In doRequest, after building chatReq, before marshalling:
if hasWebSearch(req) {
	chatReq.WebSearchOptions = &webSearchOptions{}
}
```

**Step 4: Run tests to verify they pass**

Run: `go test -v -run "TestConvertTools_SkipsGoogleSearchMarker|TestHasWebSearch" ./internal/agent/openai/...`
Expected: PASS

**Step 5: Commit**

```bash
git add internal/agent/openai/openai.go internal/agent/openai/openai_test.go
git commit -m "feat(openai): add web_search_options support via GoogleSearch marker"
```

---

### Task 3: Create WebSearch tool for OpenAI

**Files:**
- Create: `internal/agent/openai/websearch.go`
- Create: `internal/agent/openai/websearch_test.go`

**Step 1: Write the failing test**

Create `internal/agent/openai/websearch_test.go`:

```go
package openai

import (
	"testing"

	"google.golang.org/adk/model"
	"google.golang.org/genai"
)

func TestWebSearch_Name(t *testing.T) {
	ws := WebSearch{}
	if ws.Name() != "web_search" {
		t.Errorf("expected name 'web_search', got %q", ws.Name())
	}
}

func TestWebSearch_IsLongRunning(t *testing.T) {
	ws := WebSearch{}
	if ws.IsLongRunning() {
		t.Error("expected IsLongRunning to return false")
	}
}

func TestWebSearch_ProcessRequest(t *testing.T) {
	ws := WebSearch{}
	req := &model.LLMRequest{}

	if err := ws.ProcessRequest(nil, req); err != nil {
		t.Fatalf("ProcessRequest error: %v", err)
	}

	if req.Config == nil {
		t.Fatal("expected Config to be set")
	}
	if len(req.Config.Tools) != 1 {
		t.Fatalf("expected 1 tool, got %d", len(req.Config.Tools))
	}
	if req.Config.Tools[0].GoogleSearch == nil {
		t.Error("expected GoogleSearch marker to be set")
	}
}

func TestWebSearch_ProcessRequest_NilRequest(t *testing.T) {
	ws := WebSearch{}
	err := ws.ProcessRequest(nil, nil)
	if err == nil {
		t.Error("expected error for nil request")
	}
}
```

**Step 2: Run tests to verify they fail**

Run: `go test -v -run "TestWebSearch" ./internal/agent/openai/...`
Expected: FAIL — `WebSearch` type undefined

**Step 3: Write minimal implementation**

Create `internal/agent/openai/websearch.go`:

```go
package openai

import (
	"fmt"

	"google.golang.org/genai"

	"google.golang.org/adk/model"
	"google.golang.org/adk/tool"
)

// WebSearch is a tool that enables OpenAI's built-in web search via
// web_search_options. It mirrors geminitool.GoogleSearch by adding a
// GoogleSearch marker to the request config, which the OpenAI wrapper
// translates into the web_search_options HTTP body field.
type WebSearch struct{}

// Name implements tool.Tool.
func (WebSearch) Name() string { return "web_search" }

// Description implements tool.Tool.
func (WebSearch) Description() string {
	return "Search the web for additional context about a release."
}

// IsLongRunning implements tool.Tool.
func (WebSearch) IsLongRunning() bool { return false }

// ProcessRequest implements toolinternal.RequestProcessor by adding a
// GoogleSearch marker to the LLM request. The OpenAI model wrapper
// detects this marker and emits web_search_options in the HTTP body.
func (WebSearch) ProcessRequest(_ tool.Context, req *model.LLMRequest) error {
	if req == nil {
		return fmt.Errorf("llm request is nil")
	}
	if req.Config == nil {
		req.Config = &genai.GenerateContentConfig{}
	}
	req.Config.Tools = append(req.Config.Tools, &genai.Tool{
		GoogleSearch: &genai.GoogleSearch{},
	})
	return nil
}
```

**Step 4: Run tests to verify they pass**

Run: `go test -v -run "TestWebSearch" ./internal/agent/openai/...`
Expected: PASS

**Step 5: Commit**

```bash
git add internal/agent/openai/websearch.go internal/agent/openai/websearch_test.go
git commit -m "feat(openai): add WebSearch tool for OpenAI web search"
```

---

### Task 4: Add OpenAISearchModel to LLMConfig

**Files:**
- Modify: `internal/agent/model.go:15-25` (LLMConfig struct)
- Modify: `cmd/server/main.go:66-77` (env var parsing)
- Modify: `cmd/agent/main.go:62-73` (env var parsing)

**Step 1: Add field to LLMConfig**

In `internal/agent/model.go`, add after `OpenAIBaseURL`:

```go
type LLMConfig struct {
	Provider string // "gemini" or "openai"
	Model    string // e.g. "gemini-2.5-flash", "gpt-5.2"

	// Gemini
	GoogleAPIKey string

	// OpenAI
	OpenAIAPIKey      string
	OpenAIBaseURL     string // defaults to https://api.openai.com/v1
	OpenAISearchModel string // search-capable model for web search sub-agent
}
```

**Step 2: Wire env var in cmd/server/main.go**

Add after `OpenAIBaseURL` line (around line 76):

```go
llmConfig := agentpkg.LLMConfig{
	Provider:          llmProvider,
	Model:             envOr("LLM_MODEL", llmModelDefault),
	GoogleAPIKey:      os.Getenv("GOOGLE_API_KEY"),
	OpenAIAPIKey:      os.Getenv("OPENAI_API_KEY"),
	OpenAIBaseURL:     os.Getenv("OPENAI_BASE_URL"),
	OpenAISearchModel: envOr("OPENAI_SEARCH_MODEL", "gpt-5-search-api"),
}
```

**Step 3: Wire env var in cmd/agent/main.go**

Same change — add `OpenAISearchModel` to the `LLMConfig` literal (around line 67):

```go
llmConfig := agentpkg.LLMConfig{
	Provider:          llmProvider,
	Model:             envOr("LLM_MODEL", llmModelDefault),
	GoogleAPIKey:      os.Getenv("GOOGLE_API_KEY"),
	OpenAIAPIKey:      os.Getenv("OPENAI_API_KEY"),
	OpenAIBaseURL:     os.Getenv("OPENAI_BASE_URL"),
	OpenAISearchModel: envOr("OPENAI_SEARCH_MODEL", "gpt-5-search-api"),
}
```

**Step 4: Verify it compiles**

Run: `go build ./...`
Expected: SUCCESS

**Step 5: Commit**

```bash
git add internal/agent/model.go cmd/server/main.go cmd/agent/main.go
git commit -m "feat(agent): add OpenAISearchModel config for search sub-agent"
```

---

### Task 5: Unify BuildAgent to use sub-agent pattern for both providers

**Files:**
- Modify: `internal/agent/orchestrator.go:77-147` (BuildAgent function)
- Modify: `internal/agent/orchestrator.go:1-26` (imports)

**Step 1: Rewrite BuildAgent**

Replace the entire `BuildAgent` function (lines 77-147) with unified logic:

```go
func BuildAgent(ctx context.Context, store AgentDataStore, project *models.Project, llmConfig LLMConfig, version string) (agent.Agent, error) {
	instruction := DefaultInstruction
	if project.AgentPrompt != "" {
		instruction = project.AgentPrompt + "\n\n" + instruction
	}
	instruction = strings.ReplaceAll(instruction, "{{VERSION}}", version)

	llmModel, err := NewLLMModel(ctx, llmConfig)
	if err != nil {
		return nil, fmt.Errorf("create LLM model: %w", err)
	}

	// Create project-scoped function tools.
	functionTools, err := NewTools(store, project.ID)
	if err != nil {
		return nil, fmt.Errorf("create agent tools: %w", err)
	}

	// Data sub-agent: handles DB queries for releases and context sources.
	dataAgent, err := llmagent.New(llmagent.Config{
		Name:        "data_agent",
		Description: "Query project releases and context sources from the database. Use this to fetch release lists, release details, and context sources like runbooks and documentation.",
		Model:       llmModel,
		Tools:       functionTools,
	})
	if err != nil {
		return nil, fmt.Errorf("create data sub-agent: %w", err)
	}

	// Search sub-agent: provider-specific web search.
	var searchTool tool.Tool
	var searchModel model.LLM
	switch llmConfig.Provider {
	case "openai":
		searchTool = oaimodel.WebSearch{}
		// OpenAI web search requires a search-capable model.
		searchModelName := llmConfig.OpenAISearchModel
		if searchModelName == "" {
			searchModelName = "gpt-5-search-api"
		}
		searchModel, err = NewLLMModel(ctx, LLMConfig{
			Provider:      "openai",
			Model:         searchModelName,
			OpenAIAPIKey:  llmConfig.OpenAIAPIKey,
			OpenAIBaseURL: llmConfig.OpenAIBaseURL,
		})
		if err != nil {
			return nil, fmt.Errorf("create OpenAI search model: %w", err)
		}
	default: // gemini
		searchTool = geminitool.GoogleSearch{}
		searchModel = llmModel
	}

	searchAgent, err := llmagent.New(llmagent.Config{
		Name:        "search_agent",
		Description: "Search the web for additional context about a release. Use this ONLY when you need information not available from the project's sources, such as community sentiment, security advisories, network adoption statistics, or known issues.",
		Model:       searchModel,
		Tools:       []tool.Tool{searchTool},
	})
	if err != nil {
		return nil, fmt.Errorf("create search sub-agent: %w", err)
	}

	// Root agent orchestrates data lookup and web search via sub-agents.
	return llmagent.New(llmagent.Config{
		Name:        "release_analyst",
		Description: "Analyzes upstream releases and produces semantic release reports.",
		Model:       llmModel,
		Instruction: instruction,
		Tools: []tool.Tool{
			agenttool.New(dataAgent, nil),
			agenttool.New(searchAgent, nil),
		},
	})
}
```

Update imports — add the `oaimodel` import and `"google.golang.org/adk/model"`:

```go
import (
	// ... existing imports ...
	"google.golang.org/adk/model"

	oaimodel "github.com/sentioxyz/changelogue/internal/agent/openai"
	// ... rest ...
)
```

Note: `"google.golang.org/adk/model"` is needed for the `model.LLM` type used in `searchModel`. Check if the import alias conflicts — there shouldn't be one since the existing code refers to `model.LLM` via the `NewLLMModel` return type. If there's a conflict with the package name, use an alias like `adkmodel`.

**Step 2: Verify it compiles**

Run: `go build ./...`
Expected: SUCCESS

**Step 3: Run existing tests**

Run: `go test ./internal/agent/...`
Expected: PASS — existing tests don't exercise `BuildAgent` with a real LLM, so the unified path should be fine.

**Step 4: Commit**

```bash
git add internal/agent/orchestrator.go
git commit -m "feat(agent): unify BuildAgent to use sub-agent pattern for both providers"
```

---

### Task 6: Run all tests and verify

**Step 1: Run full test suite**

Run: `go test ./...`
Expected: All PASS

**Step 2: Run vet**

Run: `go vet ./...`
Expected: No issues

**Step 3: Build binary**

Run: `go build -o changelogue ./cmd/server`
Expected: SUCCESS

**Step 4: Commit if any fixes needed**

Only commit if there were compilation or test fixes needed.

---

### Task 7: Update documentation

**Files:**
- Modify: `README.md` (add `OPENAI_SEARCH_MODEL` to env var table, if env var table exists there)

**Step 1: Check if README has env var table**

Read `README.md` and look for the environment variables section.

**Step 2: Add new env var if table exists**

Add row:
```
| `OPENAI_SEARCH_MODEL` | `gpt-5-search-api` | Search-capable model for OpenAI web search sub-agent |
```

**Step 3: Commit**

```bash
git add README.md
git commit -m "docs: add OPENAI_SEARCH_MODEL env var"
```
