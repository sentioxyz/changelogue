package openai

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"iter"
	"net/http"

	"google.golang.org/adk/model"
	"google.golang.org/genai"
)

const defaultBaseURL = "https://api.openai.com/v1"

// Config holds connection parameters for an OpenAI-compatible API.
type Config struct {
	APIKey  string
	BaseURL string // defaults to https://api.openai.com/v1
}

// openaiModel implements model.LLM for OpenAI-compatible chat completion APIs.
type openaiModel struct {
	name    string
	apiKey  string
	baseURL string
	client  *http.Client
}

// NewModel creates an OpenAI-backed model.LLM.
func NewModel(_ context.Context, modelName string, cfg Config) (model.LLM, error) {
	if cfg.APIKey == "" {
		return nil, fmt.Errorf("openai: API key is required")
	}
	baseURL := cfg.BaseURL
	if baseURL == "" {
		baseURL = defaultBaseURL
	}
	return &openaiModel{
		name:    modelName,
		apiKey:  cfg.APIKey,
		baseURL: baseURL,
		client:  &http.Client{},
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

// --- OpenAI API types ---

type chatRequest struct {
	Model       string        `json:"model"`
	Messages    []chatMessage `json:"messages"`
	Tools       []chatTool    `json:"tools,omitempty"`
	Temperature *float32      `json:"temperature,omitempty"`
	TopP        *float32      `json:"top_p,omitempty"`
	MaxTokens   int32         `json:"max_tokens,omitempty"`
	Stop        []string      `json:"stop,omitempty"`
	Stream      bool          `json:"stream"`
}

type chatMessage struct {
	Role       string     `json:"role"`
	Content    string     `json:"content,omitempty"`
	ToolCalls  []toolCall `json:"tool_calls,omitempty"`
	ToolCallID string     `json:"tool_call_id,omitempty"`
}

type toolCall struct {
	ID       string       `json:"id"`
	Type     string       `json:"type"`
	Function functionCall `json:"function"`
}

type functionCall struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}

type chatTool struct {
	Type     string       `json:"type"`
	Function chatFunction `json:"function"`
}

type chatFunction struct {
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	Parameters  any    `json:"parameters,omitempty"`
}

type chatResponse struct {
	Choices []chatChoice `json:"choices"`
	Usage   *chatUsage   `json:"usage,omitempty"`
	Error   *apiError    `json:"error,omitempty"`
}

type chatChoice struct {
	Message      chatMessage `json:"message"`
	FinishReason string      `json:"finish_reason"`
}

type chatUsage struct {
	PromptTokens     int32 `json:"prompt_tokens"`
	CompletionTokens int32 `json:"completion_tokens"`
	TotalTokens      int32 `json:"total_tokens"`
}

type apiError struct {
	Message string `json:"message"`
	Type    string `json:"type"`
	Code    string `json:"code"`
}

// --- request/response conversion ---

func (m *openaiModel) doRequest(ctx context.Context, req *model.LLMRequest) (*model.LLMResponse, error) {
	msgs := convertContents(req)
	tools := convertTools(req)

	chatReq := chatRequest{
		Model:    m.name,
		Messages: msgs,
		Tools:    tools,
		Stream:   false,
	}

	if req.Config != nil {
		chatReq.Temperature = req.Config.Temperature
		chatReq.TopP = req.Config.TopP
		if req.Config.MaxOutputTokens > 0 {
			chatReq.MaxTokens = req.Config.MaxOutputTokens
		}
		if len(req.Config.StopSequences) > 0 {
			chatReq.Stop = req.Config.StopSequences
		}
	}

	body, err := json.Marshal(chatReq)
	if err != nil {
		return nil, fmt.Errorf("openai: marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, m.baseURL+"/chat/completions", bytes.NewReader(body))
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

	var chatResp chatResponse
	if err := json.Unmarshal(respBody, &chatResp); err != nil {
		return nil, fmt.Errorf("openai: unmarshal response: %w", err)
	}

	if chatResp.Error != nil {
		return nil, fmt.Errorf("openai: API error: %s (type=%s, code=%s)",
			chatResp.Error.Message, chatResp.Error.Type, chatResp.Error.Code)
	}

	return convertResponse(&chatResp), nil
}

// convertContents transforms ADK genai.Content messages into OpenAI chat messages.
func convertContents(req *model.LLMRequest) []chatMessage {
	var msgs []chatMessage

	// System instruction.
	if req.Config != nil && req.Config.SystemInstruction != nil {
		var text string
		for _, p := range req.Config.SystemInstruction.Parts {
			if p.Text != "" {
				text += p.Text
			}
		}
		if text != "" {
			msgs = append(msgs, chatMessage{Role: "system", Content: text})
		}
	}

	// Conversation contents.
	for _, c := range req.Contents {
		role := mapRole(c.Role)

		// Check each part: could be text, function_call, or function_response.
		for _, p := range c.Parts {
			switch {
			case p.FunctionCall != nil:
				argsJSON, _ := json.Marshal(p.FunctionCall.Args)
				msgs = append(msgs, chatMessage{
					Role: "assistant",
					ToolCalls: []toolCall{{
						ID:   p.FunctionCall.ID,
						Type: "function",
						Function: functionCall{
							Name:      p.FunctionCall.Name,
							Arguments: string(argsJSON),
						},
					}},
				})
			case p.FunctionResponse != nil:
				respJSON, _ := json.Marshal(p.FunctionResponse.Response)
				msgs = append(msgs, chatMessage{
					Role:       "tool",
					Content:    string(respJSON),
					ToolCallID: p.FunctionResponse.ID,
				})
			case p.Text != "":
				msgs = append(msgs, chatMessage{Role: role, Content: p.Text})
			}
		}
	}

	// Merge consecutive assistant messages with tool_calls.
	msgs = mergeAssistantToolCalls(msgs)

	return msgs
}

// mergeAssistantToolCalls combines adjacent assistant messages that each have
// tool_calls into a single message, as OpenAI expects multiple tool_calls in
// one message rather than separate messages.
func mergeAssistantToolCalls(msgs []chatMessage) []chatMessage {
	if len(msgs) == 0 {
		return msgs
	}
	var merged []chatMessage
	for _, msg := range msgs {
		if msg.Role == "assistant" && len(msg.ToolCalls) > 0 && len(merged) > 0 {
			prev := &merged[len(merged)-1]
			if prev.Role == "assistant" && len(prev.ToolCalls) > 0 {
				prev.ToolCalls = append(prev.ToolCalls, msg.ToolCalls...)
				continue
			}
		}
		merged = append(merged, msg)
	}
	return merged
}

// convertTools transforms ADK genai.Tool definitions into OpenAI tool format.
func convertTools(req *model.LLMRequest) []chatTool {
	if req.Config == nil || len(req.Config.Tools) == 0 {
		return nil
	}

	var tools []chatTool
	for _, t := range req.Config.Tools {
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

// schemaToMap converts a genai.Schema to a JSON-serialisable map that matches
// the OpenAI function parameters format (JSON Schema).
func schemaToMap(s *genai.Schema) map[string]any {
	m := map[string]any{}
	if s.Type != "" {
		m["type"] = string(s.Type)
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

// convertResponse transforms an OpenAI chat completion response into an ADK LLMResponse.
func convertResponse(resp *chatResponse) *model.LLMResponse {
	llmResp := &model.LLMResponse{
		TurnComplete: true,
	}

	if len(resp.Choices) == 0 {
		llmResp.Content = genai.NewContentFromText("", "model")
		return llmResp
	}

	choice := resp.Choices[0]
	llmResp.FinishReason = mapFinishReason(choice.FinishReason)

	content := &genai.Content{Role: "model"}

	// Text content.
	if choice.Message.Content != "" {
		content.Parts = append(content.Parts, &genai.Part{Text: choice.Message.Content})
	}

	// Tool calls.
	for _, tc := range choice.Message.ToolCalls {
		var args map[string]any
		if tc.Function.Arguments != "" {
			_ = json.Unmarshal([]byte(tc.Function.Arguments), &args)
		}
		content.Parts = append(content.Parts, &genai.Part{
			FunctionCall: &genai.FunctionCall{
				ID:   tc.ID,
				Name: tc.Function.Name,
				Args: args,
			},
		})
	}

	llmResp.Content = content

	if resp.Usage != nil {
		llmResp.UsageMetadata = &genai.GenerateContentResponseUsageMetadata{
			PromptTokenCount:     resp.Usage.PromptTokens,
			CandidatesTokenCount: resp.Usage.CompletionTokens,
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
