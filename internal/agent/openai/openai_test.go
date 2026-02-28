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
