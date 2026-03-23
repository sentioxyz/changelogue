package cli

import (
	"bytes"
	"strings"
	"testing"
)

func TestRenderTable(t *testing.T) {
	var buf bytes.Buffer
	headers := []string{"ID", "NAME", "CREATED"}
	rows := [][]string{
		{"abc-123", "my-project", "2026-03-23"},
		{"def-456", "other-proj", "2026-03-22"},
	}
	RenderTableTo(&buf, headers, rows)
	out := buf.String()

	if !strings.Contains(out, "ID") {
		t.Error("expected header ID in output")
	}
	if !strings.Contains(out, "my-project") {
		t.Error("expected my-project in output")
	}
	if !strings.Contains(out, "other-proj") {
		t.Error("expected other-proj in output")
	}
}

func TestRenderTableEmpty(t *testing.T) {
	var buf bytes.Buffer
	RenderTableTo(&buf, []string{"ID", "NAME"}, nil)
	out := buf.String()
	if !strings.Contains(out, "No results") {
		t.Error("expected 'No results' message for empty table")
	}
}

func TestRenderJSON(t *testing.T) {
	var buf bytes.Buffer
	data := map[string]string{"id": "abc", "name": "test"}
	RenderJSONTo(&buf, data)
	out := buf.String()
	if !strings.Contains(out, `"id": "abc"`) {
		t.Error("expected JSON with id field")
	}
}
