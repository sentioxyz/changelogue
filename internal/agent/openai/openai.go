package openai

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"iter"
	"net/http"
	"strings"

	"google.golang.org/adk/model"
	"google.golang.org/genai"
)

const defaultBaseURL = "https://api.openai.com/v1"

// Config holds connection parameters for an OpenAI-compatible API.
type Config struct {
	APIKey  string
	BaseURL string // defaults to https://api.openai.com/v1
}

// openaiModel implements model.LLM for OpenAI-compatible Responses APIs.
type openaiModel struct {
	name     string
	apiKey   string
	baseURL  string
	client   *http.Client
	wsConfig *WebSearch
}

// NewModel creates an OpenAI-backed model.LLM. wsConfig may be nil if web
// search is not needed.
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

func (m *openaiModel) Name() string { return m.name }

// GenerateContent converts the ADK request to an OpenAI chat completion request,
// calls the API, and converts the response back to an LLMResponse.
// Streaming is not implemented; the iterator always yields a single response.
func (m *openaiModel) GenerateContent(ctx context.Context, req *model.LLMRequest, _ bool) iter.Seq2[*model.LLMResponse, error] {
	return func(yield func(*model.LLMResponse, error) bool) {
		resp, err := m.doRequest(ctx, req)
		if err != nil {
			yield(nil, err)
			return
		}
		yield(resp, nil)
	}
}

// Close is a no-op; the HTTP client has no persistent state.
func (m *openaiModel) Close() {}

// --- Responses API types ---

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

type inputItem struct {
	Type      string `json:"type"`               // "message", "function_call", "function_call_output"
	Role      string `json:"role,omitempty"`
	Content   string `json:"content,omitempty"`
	ID        string `json:"id,omitempty"`
	CallID    string `json:"call_id,omitempty"`
	Name      string `json:"name,omitempty"`
	Arguments string `json:"arguments,omitempty"`
	Output    string `json:"output,omitempty"`
}

type responseTool struct {
	Type              string         `json:"type"` // "function" or "web_search"
	Name              string         `json:"name,omitempty"`
	Description       string         `json:"description,omitempty"`
	Parameters        any            `json:"parameters,omitempty"`
	SearchContextSize string         `json:"search_context_size,omitempty"`
	UserLocation      any            `json:"user_location,omitempty"`
	Filters           *searchFilters `json:"filters,omitempty"`
}

type searchFilters struct {
	AllowedDomains []string `json:"allowed_domains,omitempty"`
}

type responsesResponse struct {
	ID         string       `json:"id"`
	Status     string       `json:"status"`
	Output     []outputItem `json:"output"`
	OutputText string       `json:"output_text"`
	Usage      *respUsage   `json:"usage,omitempty"`
	Error      *apiError    `json:"error,omitempty"`
}

type outputItem struct {
	Type      string          `json:"type"` // "message", "function_call", "web_search_call"
	ID        string          `json:"id,omitempty"`
	Status    string          `json:"status,omitempty"`
	CallID    string          `json:"call_id,omitempty"`
	Name      string          `json:"name,omitempty"`
	Arguments string          `json:"arguments,omitempty"`
	Role      string          `json:"role,omitempty"`
	Content   []outputContent `json:"content,omitempty"`
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

// --- request/response conversion ---

func (m *openaiModel) doRequest(ctx context.Context, req *model.LLMRequest) (*model.LLMResponse, error) {
	items, instructions := convertInputs(req)
	tools := convertResponseTools(req, m.wsConfig)

	respReq := responsesRequest{
		Model:        m.name,
		Input:        items,
		Instructions: instructions,
		Tools:        tools,
		Store:        false,
	}

	if req.Config != nil {
		respReq.Temperature = req.Config.Temperature
		respReq.TopP = req.Config.TopP
		if req.Config.MaxOutputTokens > 0 {
			respReq.MaxTokens = req.Config.MaxOutputTokens
		}
	}

	body, err := json.Marshal(respReq)
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

	var respOut responsesResponse
	if err := json.Unmarshal(respBody, &respOut); err != nil {
		return nil, fmt.Errorf("openai: unmarshal response: %w", err)
	}

	if respOut.Error != nil {
		return nil, fmt.Errorf("openai: API error: %s (type=%s, code=%s)",
			respOut.Error.Message, respOut.Error.Type, respOut.Error.Code)
	}

	return convertResponseOutput(&respOut), nil
}

// convertInputs transforms ADK Contents into Responses API input items and
// extracts the system instruction as a separate instructions string.
func convertInputs(req *model.LLMRequest) ([]inputItem, string) {
	var items []inputItem
	var instructions string

	// System instruction.
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
					Role:    mapRole(c.Role),
					Content: p.Text,
				})
			}
		}
	}

	return items, instructions
}

// convertResponseTools transforms ADK tools to Responses API flat format.
// Function tools become {type: "function", name, description, parameters}.
// If a GoogleSearch marker is present, a {type: "web_search"} tool is added
// using wsConfig for search_context_size and allowed_domains.
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
			wsTool.SearchContextSize = wsConfig.SearchContextSize
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

// schemaToMap converts a genai.Schema to a JSON-serialisable map that matches
// the OpenAI function parameters format (JSON Schema).
func schemaToMap(s *genai.Schema) map[string]any {
	m := map[string]any{}
	if s.Type != "" {
		m["type"] = strings.ToLower(string(s.Type))
	}
	if s.Description != "" {
		m["description"] = s.Description
	}
	if len(s.Enum) > 0 {
		m["enum"] = s.Enum
	}
	if s.Items != nil {
		m["items"] = schemaToMap(s.Items)
	}
	if len(s.Properties) > 0 {
		props := map[string]any{}
		for k, v := range s.Properties {
			props[k] = schemaToMap(v)
		}
		m["properties"] = props
	}
	if len(s.Required) > 0 {
		m["required"] = s.Required
	}
	return m
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
		case "message":
			for _, c := range item.Content {
				if c.Text != "" {
					content.Parts = append(content.Parts, &genai.Part{Text: c.Text})
				}
			}
		case "web_search_call":
			// Skipped — handled internally by the API.
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

func mapRole(role string) string {
	switch role {
	case "model":
		return "assistant"
	default:
		return role
	}
}

func mapFinishReason(reason string) genai.FinishReason {
	switch reason {
	case "stop":
		return genai.FinishReasonStop
	case "length":
		return genai.FinishReasonMaxTokens
	case "tool_calls":
		return genai.FinishReasonStop
	case "content_filter":
		return genai.FinishReasonSafety
	default:
		return genai.FinishReasonOther
	}
}
