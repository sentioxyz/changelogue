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
	if len(req.Config.Tools) != 1 {
		t.Fatalf("expected 1 tool, got %d", len(req.Config.Tools))
	}
	if req.Config.Tools[0].GoogleSearch == nil {
		t.Error("expected GoogleSearch marker to be set")
	}
	if ws.SearchContextSize != "high" {
		t.Errorf("expected SearchContextSize 'high', got %q", ws.SearchContextSize)
	}
}

func TestWebSearch_ProcessRequest_NilRequest(t *testing.T) {
	ws := WebSearch{}
	err := ws.ProcessRequest(nil, nil)
	if err == nil {
		t.Error("expected error for nil request")
	}
}
