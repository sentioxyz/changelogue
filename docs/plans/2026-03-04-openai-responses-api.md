# OpenAI Responses API Migration — Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Migrate the OpenAI adapter from Chat Completions API to Responses API for native web search tool support with domain filtering and search context control.

**Architecture:** Replace `POST /v1/chat/completions` with `POST /v1/responses` in the OpenAI adapter. The Responses API uses a different wire format (`input` items instead of `messages`, flat tool definitions instead of nested `function` wrappers, `function_call`/`function_call_output` input items instead of `tool` role messages). The WebSearch tool gains config fields (SearchContextSize, AllowedDomains). The search sub-agent no longer needs a separate model.

**Tech Stack:** Go, `net/http`, Google ADK-Go v0.5.0 (`model.LLM` interface)

**Design doc:** `docs/plans/2026-03-04-openai-responses-api-design.md`

---

### Task 1: Rewrite WebSearch tool with config fields

**Files:**
- Modify: `internal/agent/openai/websearch.go`
- Modify: `internal/agent/openai/websearch_test.go`

**Step 1: Write the failing tests**

Update `websearch_test.go` to test that WebSearch carries config and ProcessRequest adds a `webSearchConfig` marker to the request (instead of GoogleSearch marker):

```go
package openai

import (
	"testing"

	"google.golang.org/adk/model"
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

func TestWebSearch_ProcessRequest_SetsMarker(t *testing.T) {
	ws := WebSearch{SearchContextSize: "high"}
	req := &model.LLMRequest{}

	if err := ws.ProcessRequest(nil, req); err != nil {
		t.Fatalf("ProcessRequest error: %v", err)
	}

	if req.Config == nil {
		t.Fatal("expected Config to be set")
	}
	// The marker is still a GoogleSearch tool (ADK convention), but the
	// OpenAI adapter reads our webSearchConfig from the model wrapper.
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

**Step 2: Run tests to verify they pass (marker behavior is unchanged)**

Run: `go test -v -run TestWebSearch ./internal/agent/openai/...`
Expected: PASS — the marker behavior hasn't changed yet, we just added config fields.

**Step 3: Update WebSearch struct with config fields**

In `websearch.go`, add the configurable fields:

```go
package openai

import (
	"fmt"

	"google.golang.org/genai"

	"google.golang.org/adk/model"
	"google.golang.org/adk/tool"
)

// WebSearch is a tool that enables OpenAI's built-in web search via the
// Responses API tools array. It adds a GoogleSearch marker to the ADK
// request config, which the OpenAI adapter translates into a
// {"type": "web_search", ...} tool entry.
type WebSearch struct {
	SearchContextSize string   // "low", "medium" (default), "high"
	AllowedDomains    []string // optional domain filter (up to 100)
}

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
// detects this marker and emits a web_search tool in the Responses API body.
func (ws WebSearch) ProcessRequest(_ tool.Context, req *model.LLMRequest) error {
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

**Step 4: Run tests**

Run: `go test -v -run TestWebSearch ./internal/agent/openai/...`
Expected: PASS

**Step 5: Commit**

```bash
git add internal/agent/openai/websearch.go internal/agent/openai/websearch_test.go
git commit -m "feat(agent/openai): add config fields to WebSearch tool"
```

---

### Task 2: Rewrite OpenAI adapter request types for Responses API

**Files:**
- Modify: `internal/agent/openai/openai.go`

**Step 1: Replace Chat Completions types with Responses API types**

Replace the type definitions in `openai.go`. Remove `chatRequest`, `chatMessage`, `chatTool`, `chatFunction`, `chatResponse`, `chatChoice`, `chatUsage`, `apiError`, `webSearchOptions`, `toolCall`, `functionCall`.

Add new types:

```go
// --- Responses API types ---

// responsesRequest is the body for POST /v1/responses.
type responsesRequest struct {
	Model        string         `json:"model"`
	Input        []inputItem    `json:"input"`
	Instructions string         `json:"instructions,omitempty"`
	Tools        []responseTool `json:"tools,omitempty"`
	Temperature  *float32       `json:"temperature,omitempty"`
	TopP         *float32       `json:"top_p,omitempty"`
	MaxTokens    int32          `json:"max_output_tokens,omitempty"`
	Store        bool           `json:"store"`
}

// inputItem is a union type for input array entries.
// Only one of the "type" variants is populated per item.
type inputItem struct {
	// Message input (role + content)
	Type    string `json:"type"`              // "message", "function_call", "function_call_output"
	Role    string `json:"role,omitempty"`     // "user", "assistant", "system", "developer"
	Content string `json:"content,omitempty"` // for message and function_call_output

	// function_call echo fields
	ID        string `json:"id,omitempty"`
	CallID    string `json:"call_id,omitempty"`
	Name      string `json:"name,omitempty"`
	Arguments string `json:"arguments,omitempty"`

	// function_call_output fields
	// CallID and Content are reused
	Output string `json:"output,omitempty"` // function result
}

// responseTool represents a tool in the Responses API tools array.
type responseTool struct {
	Type string `json:"type"` // "function" or "web_search"

	// Function tool fields (type=function)
	Name        string `json:"name,omitempty"`
	Description string `json:"description,omitempty"`
	Parameters  any    `json:"parameters,omitempty"`

	// Web search fields (type=web_search)
	SearchContextSize string         `json:"search_context_size,omitempty"`
	UserLocation      any            `json:"user_location,omitempty"`
	Filters           *searchFilters `json:"filters,omitempty"`
}

type searchFilters struct {
	AllowedDomains []string `json:"allowed_domains,omitempty"`
}

// responsesResponse is the top-level response from POST /v1/responses.
type responsesResponse struct {
	ID         string       `json:"id"`
	Status     string       `json:"status"`
	Output     []outputItem `json:"output"`
	OutputText string       `json:"output_text"`
	Usage      *respUsage   `json:"usage,omitempty"`
	Error      *apiError    `json:"error,omitempty"`
}

// outputItem is a union type for output array entries.
type outputItem struct {
	Type   string `json:"type"` // "message", "function_call", "web_search_call"
	ID     string `json:"id,omitempty"`
	Status string `json:"status,omitempty"`

	// function_call fields
	CallID    string `json:"call_id,omitempty"`
	Name      string `json:"name,omitempty"`
	Arguments string `json:"arguments,omitempty"`

	// message fields
	Role    string          `json:"role,omitempty"`
	Content []outputContent `json:"content,omitempty"`
}

type outputContent struct {
	Type        string       `json:"type"` // "output_text"
	Text        string       `json:"text"`
	Annotations []annotation `json:"annotations,omitempty"`
}

type annotation struct {
	Type       string `json:"type"` // "url_citation"
	URL        string `json:"url"`
	Title      string `json:"title"`
	StartIndex int    `json:"start_index"`
	EndIndex   int    `json:"end_index"`
}

type respUsage struct {
	InputTokens  int32 `json:"input_tokens"`
	OutputTokens int32 `json:"output_tokens"`
	TotalTokens  int32 `json:"total_tokens"`
}

type apiError struct {
	Message string `json:"message"`
	Type    string `json:"type"`
	Code    string `json:"code"`
}
```

**Step 2: Verify compilation**

Run: `go build ./internal/agent/openai/...`
Expected: compilation errors because `doRequest`, `convertContents`, `convertTools`, `convertResponse` still reference old types. This is expected — we fix them in Task 3.

---

### Task 3: Rewrite conversion functions for Responses API

**Files:**
- Modify: `internal/agent/openai/openai.go`

**Step 1: Write tests for the new conversion functions**

Add to `openai_test.go`:

```go
func TestConvertInputs_SystemInstruction(t *testing.T) {
	req := &model.LLMRequest{
		Config: &genai.GenerateContentConfig{
			SystemInstruction: genai.NewContentFromText("You are helpful.", "system"),
		},
	}
	_, instructions := convertInputs(req)
	if instructions != "You are helpful." {
		t.Errorf("expected instructions 'You are helpful.', got %q", instructions)
	}
}

func TestConvertInputs_FunctionCallAndResponse(t *testing.T) {
	req := &model.LLMRequest{
		Contents: []*genai.Content{
			genai.NewContentFromText("What releases?", "user"),
			{
				Role: "model",
				Parts: []*genai.Part{{
					FunctionCall: &genai.FunctionCall{
						ID:   "fc_1",
						Name: "get_releases",
						Args: map[string]any{"page": float64(1)},
					},
				}},
			},
			{
				Role: "model",
				Parts: []*genai.Part{{
					FunctionResponse: &genai.FunctionResponse{
						ID:       "fc_1",
						Name:     "get_releases",
						Response: map[string]any{"releases": []any{}},
					},
				}},
			},
		},
	}
	items, _ := convertInputs(req)

	// Should have: user message, function_call echo, function_call_output
	if len(items) != 3 {
		t.Fatalf("expected 3 input items, got %d", len(items))
	}
	if items[0].Type != "message" || items[0].Role != "user" {
		t.Errorf("item 0: expected message/user, got %s/%s", items[0].Type, items[0].Role)
	}
	if items[1].Type != "function_call" || items[1].Name != "get_releases" {
		t.Errorf("item 1: expected function_call/get_releases, got %s/%s", items[1].Type, items[1].Name)
	}
	if items[2].Type != "function_call_output" {
		t.Errorf("item 2: expected function_call_output, got %s", items[2].Type)
	}
}

func TestConvertResponseTools_WebSearch(t *testing.T) {
	ws := &WebSearch{SearchContextSize: "high", AllowedDomains: []string{"github.com"}}
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

	tools := convertResponseTools(req, ws)

	// Should have: 1 function tool + 1 web_search tool
	if len(tools) != 2 {
		t.Fatalf("expected 2 tools, got %d", len(tools))
	}

	var funcTool, searchTool *responseTool
	for i := range tools {
		switch tools[i].Type {
		case "function":
			funcTool = &tools[i]
		case "web_search":
			searchTool = &tools[i]
		}
	}

	if funcTool == nil {
		t.Fatal("no function tool found")
	}
	if funcTool.Name != "get_releases" {
		t.Errorf("expected function name 'get_releases', got %q", funcTool.Name)
	}
	if searchTool == nil {
		t.Fatal("no web_search tool found")
	}
	if searchTool.SearchContextSize != "high" {
		t.Errorf("expected search_context_size 'high', got %q", searchTool.SearchContextSize)
	}
	if searchTool.Filters == nil || len(searchTool.Filters.AllowedDomains) != 1 {
		t.Fatal("expected 1 allowed domain")
	}
}

func TestConvertResponseOutput_TextAndFunctionCalls(t *testing.T) {
	resp := &responsesResponse{
		Output: []outputItem{
			{
				Type:      "function_call",
				ID:        "fc_1",
				CallID:    "call_1",
				Name:      "get_releases",
				Arguments: `{"page":1}`,
			},
			{
				Type: "message",
				Role: "assistant",
				Content: []outputContent{{
					Type: "output_text",
					Text: "Here are the releases.",
				}},
			},
		},
		Usage: &respUsage{
			InputTokens:  100,
			OutputTokens: 50,
			TotalTokens:  150,
		},
	}

	llmResp := convertResponseOutput(resp)

	if llmResp.Content == nil {
		t.Fatal("expected content")
	}
	// Should have function_call part + text part
	if len(llmResp.Content.Parts) != 2 {
		t.Fatalf("expected 2 parts, got %d", len(llmResp.Content.Parts))
	}

	fc := llmResp.Content.Parts[0].FunctionCall
	if fc == nil {
		t.Fatal("expected first part to be FunctionCall")
	}
	if fc.Name != "get_releases" {
		t.Errorf("expected function name 'get_releases', got %q", fc.Name)
	}
	if fc.ID != "call_1" {
		t.Errorf("expected function ID 'call_1', got %q", fc.ID)
	}
	if llmResp.Content.Parts[1].Text != "Here are the releases." {
		t.Errorf("unexpected text: %q", llmResp.Content.Parts[1].Text)
	}
}
```

**Step 2: Run tests to verify they fail**

Run: `go test -v -run "TestConvertInputs|TestConvertResponseTools|TestConvertResponseOutput" ./internal/agent/openai/...`
Expected: FAIL — functions don't exist yet.

**Step 3: Implement conversion functions**

Replace `convertContents`, `convertTools`, `convertResponse`, `hasWebSearch`, `mergeAssistantToolCalls` with:

```go
// convertInputs transforms ADK genai.Content messages into Responses API input items.
// Returns the input items and extracted system instructions.
func convertInputs(req *model.LLMRequest) ([]inputItem, string) {
	var items []inputItem
	var instructions string

	// Extract system instruction separately (Responses API uses "instructions" field).
	if req.Config != nil && req.Config.SystemInstruction != nil {
		var text string
		for _, p := range req.Config.SystemInstruction.Parts {
			if p.Text != "" {
				text += p.Text
			}
		}
		instructions = text
	}

	// Conversation contents.
	for _, c := range req.Contents {
		role := mapRole(c.Role)

		for _, p := range c.Parts {
			switch {
			case p.FunctionCall != nil:
				argsJSON, _ := json.Marshal(p.FunctionCall.Args)
				items = append(items, inputItem{
					Type:      "function_call",
					CallID:    p.FunctionCall.ID,
					Name:      p.FunctionCall.Name,
					Arguments: string(argsJSON),
				})
			case p.FunctionResponse != nil:
				respJSON, _ := json.Marshal(p.FunctionResponse.Response)
				items = append(items, inputItem{
					Type:   "function_call_output",
					CallID: p.FunctionResponse.ID,
					Output: string(respJSON),
				})
			case p.Text != "":
				items = append(items, inputItem{
					Type:    "message",
					Role:    role,
					Content: p.Text,
				})
			}
		}
	}

	return items, instructions
}

// webSearchConfig extracts web search parameters from the WebSearch tool
// registered on the model. Returns nil if no WebSearch is configured.
func (m *openaiModel) webSearchConfig() *WebSearch {
	return m.wsConfig
}

// convertResponseTools transforms ADK genai.Tool definitions into Responses API tools.
// If wsConfig is non-nil and a GoogleSearch marker is present, it emits a web_search
// tool with the configured parameters.
func convertResponseTools(req *model.LLMRequest, wsConfig *WebSearch) []responseTool {
	if req.Config == nil || len(req.Config.Tools) == 0 {
		return nil
	}

	var tools []responseTool
	hasSearch := false

	for _, t := range req.Config.Tools {
		if t.GoogleSearch != nil {
			hasSearch = true
			continue
		}
		for _, fd := range t.FunctionDeclarations {
			rt := responseTool{
				Type:        "function",
				Name:        fd.Name,
				Description: fd.Description,
			}
			if fd.Parameters != nil {
				rt.Parameters = schemaToMap(fd.Parameters)
			}
			tools = append(tools, rt)
		}
	}

	if hasSearch {
		wsTool := responseTool{Type: "web_search"}
		if wsConfig != nil {
			if wsConfig.SearchContextSize != "" {
				wsTool.SearchContextSize = wsConfig.SearchContextSize
			}
			if len(wsConfig.AllowedDomains) > 0 {
				wsTool.Filters = &searchFilters{
					AllowedDomains: wsConfig.AllowedDomains,
				}
			}
		}
		tools = append(tools, wsTool)
	}

	return tools
}

// convertResponseOutput transforms a Responses API response into an ADK LLMResponse.
func convertResponseOutput(resp *responsesResponse) *model.LLMResponse {
	llmResp := &model.LLMResponse{
		TurnComplete: true,
	}

	content := &genai.Content{Role: "model"}

	for _, item := range resp.Output {
		switch item.Type {
		case "function_call":
			var args map[string]any
			if item.Arguments != "" {
				_ = json.Unmarshal([]byte(item.Arguments), &args)
			}
			content.Parts = append(content.Parts, &genai.Part{
				FunctionCall: &genai.FunctionCall{
					ID:   item.CallID,
					Name: item.Name,
					Args: args,
				},
			})
			llmResp.FinishReason = genai.FinishReasonStop

		case "message":
			for _, c := range item.Content {
				if c.Text != "" {
					content.Parts = append(content.Parts, &genai.Part{Text: c.Text})
				}
			}
			llmResp.FinishReason = genai.FinishReasonStop

		case "web_search_call":
			// Informational; no ADK equivalent. Skip.
		}
	}

	if len(content.Parts) == 0 {
		content.Parts = append(content.Parts, &genai.Part{Text: ""})
	}

	llmResp.Content = content

	if resp.Usage != nil {
		llmResp.UsageMetadata = &genai.GenerateContentResponseUsageMetadata{
			PromptTokenCount:     resp.Usage.InputTokens,
			CandidatesTokenCount: resp.Usage.OutputTokens,
			TotalTokenCount:      resp.Usage.TotalTokens,
		}
	}

	return llmResp
}
```

**Step 4: Run tests**

Run: `go test -v -run "TestConvertInputs|TestConvertResponseTools|TestConvertResponseOutput|TestSchemaToMap" ./internal/agent/openai/...`
Expected: PASS

**Step 5: Commit**

```bash
git add internal/agent/openai/openai.go internal/agent/openai/openai_test.go
git commit -m "feat(agent/openai): add Responses API conversion functions"
```

---

### Task 4: Rewrite doRequest and model struct for Responses API

**Files:**
- Modify: `internal/agent/openai/openai.go`

**Step 1: Update openaiModel struct and NewModel to accept WebSearch config**

```go
type openaiModel struct {
	name     string
	apiKey   string
	baseURL  string
	client   *http.Client
	wsConfig *WebSearch // optional web search config
}

// NewModel creates an OpenAI-backed model.LLM.
// wsConfig is optional; pass nil to disable web search.
func NewModel(_ context.Context, modelName string, cfg Config, wsConfig *WebSearch) (model.LLM, error) {
	if cfg.APIKey == "" {
		return nil, fmt.Errorf("openai: API key is required")
	}
	baseURL := cfg.BaseURL
	if baseURL == "" {
		baseURL = defaultBaseURL
	}
	return &openaiModel{
		name:     modelName,
		apiKey:   cfg.APIKey,
		baseURL:  baseURL,
		client:   &http.Client{},
		wsConfig: wsConfig,
	}, nil
}
```

**Step 2: Rewrite doRequest for Responses API**

```go
func (m *openaiModel) doRequest(ctx context.Context, req *model.LLMRequest) (*model.LLMResponse, error) {
	items, instructions := convertInputs(req)
	tools := convertResponseTools(req, m.wsConfig)

	apiReq := responsesRequest{
		Model:        m.name,
		Input:        items,
		Instructions: instructions,
		Tools:        tools,
		Store:        false,
	}

	if req.Config != nil {
		apiReq.Temperature = req.Config.Temperature
		apiReq.TopP = req.Config.TopP
		if req.Config.MaxOutputTokens > 0 {
			apiReq.MaxTokens = req.Config.MaxOutputTokens
		}
	}

	body, err := json.Marshal(apiReq)
	if err != nil {
		return nil, fmt.Errorf("openai: marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, m.baseURL+"/responses", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("openai: create http request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+m.apiKey)

	httpResp, err := m.client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("openai: http request: %w", err)
	}
	defer httpResp.Body.Close()

	respBody, err := io.ReadAll(httpResp.Body)
	if err != nil {
		return nil, fmt.Errorf("openai: read response: %w", err)
	}

	if httpResp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("openai: API returned status %d: %s", httpResp.StatusCode, string(respBody))
	}

	var apiResp responsesResponse
	if err := json.Unmarshal(respBody, &apiResp); err != nil {
		return nil, fmt.Errorf("openai: unmarshal response: %w", err)
	}

	if apiResp.Error != nil {
		return nil, fmt.Errorf("openai: API error: %s (type=%s, code=%s)",
			apiResp.Error.Message, apiResp.Error.Type, apiResp.Error.Code)
	}

	return convertResponseOutput(&apiResp), nil
}
```

**Step 3: Remove old dead code**

Delete these functions that are no longer used:
- `convertContents`
- `convertTools` (renamed to `convertResponseTools`)
- `convertResponse` (renamed to `convertResponseOutput`)
- `hasWebSearch`
- `mergeAssistantToolCalls`
- Old types: `chatRequest`, `chatMessage`, `chatTool`, `chatFunction`, `chatResponse`, `chatChoice`, `chatUsage`, `webSearchOptions`, `toolCall`, `functionCall`

**Step 4: Run tests and fix old tests**

Update `TestConvertTools_SkipsGoogleSearchMarker` → `TestConvertResponseTools_SkipsGoogleSearchMarker`.
Update `TestHasWebSearch` → remove (no longer needed, tested indirectly via convertResponseTools).

Run: `go test -v ./internal/agent/openai/...`
Expected: PASS

**Step 5: Commit**

```bash
git add internal/agent/openai/openai.go internal/agent/openai/openai_test.go
git commit -m "feat(agent/openai): migrate doRequest to Responses API"
```

---

### Task 5: Update model.go — remove OpenAISearchModel, pass WebSearch config

**Files:**
- Modify: `internal/agent/model.go`

**Step 1: Update LLMConfig and NewLLMModel**

Remove `OpenAISearchModel` from `LLMConfig`. Update `NewLLMModel` signature to accept optional `*WebSearch`:

```go
type LLMConfig struct {
	Provider string // "gemini" or "openai"
	Model    string // e.g. "gemini-2.5-flash", "gpt-5.2"

	// Gemini
	GoogleAPIKey string

	// OpenAI
	OpenAIAPIKey  string
	OpenAIBaseURL string // defaults to https://api.openai.com/v1
}

// NewLLMModel creates a model.LLM based on the configured provider.
// wsConfig is optional OpenAI web search configuration (ignored for Gemini).
func NewLLMModel(ctx context.Context, cfg LLMConfig, wsConfig *oaimodel.WebSearch) (model.LLM, error) {
	switch cfg.Provider {
	case "gemini", "":
		if cfg.GoogleAPIKey == "" {
			return nil, fmt.Errorf("GOOGLE_API_KEY is required for gemini provider")
		}
		return gemini.NewModel(ctx, cfg.Model, &genai.ClientConfig{
			APIKey: cfg.GoogleAPIKey,
		})

	case "openai":
		return oaimodel.NewModel(ctx, cfg.Model, oaimodel.Config{
			APIKey:  cfg.OpenAIAPIKey,
			BaseURL: cfg.OpenAIBaseURL,
		}, wsConfig)

	default:
		return nil, fmt.Errorf("unknown LLM provider: %q (supported: gemini, openai)", cfg.Provider)
	}
}
```

**Step 2: Verify compilation**

Run: `go build ./internal/agent/...`
Expected: compilation errors in orchestrator.go because callers pass wrong args. Fixed in Task 6.

---

### Task 6: Update orchestrator.go — simplify search sub-agent

**Files:**
- Modify: `internal/agent/orchestrator.go`

**Step 1: Update BuildAgent to use same model for search**

Update `BuildAgent` in orchestrator.go:

```go
func BuildAgent(ctx context.Context, store AgentDataStore, project *models.Project, llmConfig LLMConfig, version string) (agent.Agent, error) {
	instruction := DefaultInstruction
	if project.AgentPrompt != "" {
		instruction = project.AgentPrompt + "\n\n" + instruction
	}
	instruction = strings.ReplaceAll(instruction, "{{VERSION}}", version)

	// Create main model (no web search).
	llmModel, err := NewLLMModel(ctx, llmConfig, nil)
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
		wsConfig := &oaimodel.WebSearch{SearchContextSize: "medium"}
		searchTool = *wsConfig
		// Same model, web search is a native tool in the Responses API.
		searchModel, err = NewLLMModel(ctx, llmConfig, wsConfig)
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

Remove the `geminitool` import if no longer needed after this change. Remove the `oaimodel` import only if no longer needed (it will still be needed for `oaimodel.WebSearch`).

**Step 2: Run build**

Run: `go build ./internal/agent/...`
Expected: PASS

**Step 3: Run tests**

Run: `go test ./internal/agent/...`
Expected: PASS

**Step 4: Commit**

```bash
git add internal/agent/model.go internal/agent/orchestrator.go
git commit -m "feat(agent): simplify search agent — same model, Responses API web search"
```

---

### Task 7: Update cmd/server/main.go — remove OPENAI_SEARCH_MODEL

**Files:**
- Modify: `cmd/server/main.go`

**Step 1: Remove OPENAI_SEARCH_MODEL from LLMConfig initialization**

Find the block that creates `llmConfig` and remove the `OpenAISearchModel` line:

```go
llmConfig := agentpkg.LLMConfig{
	Provider:     llmProvider,
	Model:        envOr("LLM_MODEL", llmModelDefault),
	GoogleAPIKey: os.Getenv("GOOGLE_API_KEY"),
	OpenAIAPIKey: os.Getenv("OPENAI_API_KEY"),
	OpenAIBaseURL: os.Getenv("OPENAI_BASE_URL"),
}
```

**Step 2: Check cmd/agent/main.go for same field**

Read `cmd/agent/main.go` and remove `OpenAISearchModel` references if present.

**Step 3: Run build and test**

Run: `go build ./cmd/server/... && go build ./cmd/agent/... && go test ./...`
Expected: PASS

**Step 4: Commit**

```bash
git add cmd/server/main.go cmd/agent/main.go
git commit -m "chore: remove OPENAI_SEARCH_MODEL env var"
```

---

### Task 8: Run full test suite and verify

**Step 1: Run all tests**

Run: `go test ./...`
Expected: PASS

**Step 2: Run vet**

Run: `go vet ./...`
Expected: PASS

**Step 3: Build binary**

Run: `go build -o changelogue ./cmd/server`
Expected: PASS

---

### Task 9: Update design doc with completion status

**Files:**
- Modify: `docs/plans/2026-03-04-openai-responses-api-design.md`

**Step 1: Add completion note**

Add to the top of the design doc: `**Status:** Implemented`

**Step 2: Final commit**

```bash
git add docs/plans/2026-03-04-openai-responses-api-design.md
git commit -m "docs: mark Responses API migration as implemented"
```
