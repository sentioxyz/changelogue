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
