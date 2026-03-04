package openai

import (
	"testing"

	"google.golang.org/adk/model"
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

func TestConvertInputs_SystemInstruction(t *testing.T) {
	req := &model.LLMRequest{
		Config: &genai.GenerateContentConfig{
			SystemInstruction: genai.NewContentFromText("You are helpful.", "system"),
		},
	}
	_, instructions := convertInputs(req)
	if instructions != "You are helpful." {
		t.Errorf("expected 'You are helpful.', got %q", instructions)
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

	if funcTool == nil || funcTool.Name != "get_releases" {
		t.Errorf("missing or wrong function tool")
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
	if len(llmResp.Content.Parts) != 2 {
		t.Fatalf("expected 2 parts, got %d", len(llmResp.Content.Parts))
	}

	fc := llmResp.Content.Parts[0].FunctionCall
	if fc == nil || fc.Name != "get_releases" || fc.ID != "call_1" {
		t.Errorf("wrong function call: %+v", fc)
	}
	if llmResp.Content.Parts[1].Text != "Here are the releases." {
		t.Errorf("unexpected text: %q", llmResp.Content.Parts[1].Text)
	}
}
