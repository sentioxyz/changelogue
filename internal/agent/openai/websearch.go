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
