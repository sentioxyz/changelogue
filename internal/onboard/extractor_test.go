package onboard

import (
	"strings"
	"testing"
)

func TestParseDependencies_ValidJSON(t *testing.T) {
	raw := `[
		{"name": "gorilla/mux", "version": "v1.8.1", "ecosystem": "go", "upstream_repo": "github.com/gorilla/mux", "provider": "github"},
		{"name": "react", "version": "^18.2.0", "ecosystem": "npm", "upstream_repo": "github.com/facebook/react", "provider": "github"}
	]`

	deps, err := ParseDependencies([]byte(raw))
	if err != nil {
		t.Fatalf("ParseDependencies: %v", err)
	}
	if len(deps) != 2 {
		t.Fatalf("expected 2 deps, got %d", len(deps))
	}
	if deps[0].Name != "gorilla/mux" {
		t.Errorf("deps[0].Name = %q, want %q", deps[0].Name, "gorilla/mux")
	}
	if deps[1].Ecosystem != "npm" {
		t.Errorf("deps[1].Ecosystem = %q, want %q", deps[1].Ecosystem, "npm")
	}
}

func TestParseDependencies_ExtractsJSON(t *testing.T) {
	// LLM sometimes wraps JSON in markdown code fences
	raw := "```json\n[{\"name\": \"test\", \"version\": \"1.0\", \"ecosystem\": \"go\", \"upstream_repo\": \"github.com/test/test\", \"provider\": \"github\"}]\n```"

	deps, err := ParseDependencies([]byte(raw))
	if err != nil {
		t.Fatalf("ParseDependencies: %v", err)
	}
	if len(deps) != 1 {
		t.Fatalf("expected 1 dep, got %d", len(deps))
	}
	if deps[0].Name != "test" {
		t.Errorf("deps[0].Name = %q, want %q", deps[0].Name, "test")
	}
}

func TestParseDependencies_EmptyArray(t *testing.T) {
	deps, err := ParseDependencies([]byte("[]"))
	if err != nil {
		t.Fatalf("ParseDependencies: %v", err)
	}
	if len(deps) != 0 {
		t.Fatalf("expected 0 deps, got %d", len(deps))
	}
}

func TestBuildPrompt(t *testing.T) {
	files := []DependencyFile{
		{Path: "go.mod", Content: "module example.com"},
	}
	prompt := BuildExtractionPrompt(files)
	if prompt == "" {
		t.Fatal("prompt is empty")
	}
	if !strings.Contains(prompt, "go.mod") {
		t.Error("prompt should contain file path")
	}
	if !strings.Contains(prompt, "module example.com") {
		t.Error("prompt should contain file content")
	}
}
