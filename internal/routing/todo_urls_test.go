package routing

import "testing"

func TestTodoAcknowledgeURL(t *testing.T) {
	got := TodoAcknowledgeURL("https://example.com", "abc-123")
	want := "https://example.com/api/v1/todos/abc-123/acknowledge?redirect=true"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestTodoResolveURL(t *testing.T) {
	got := TodoResolveURL("https://example.com", "abc-123")
	want := "https://example.com/api/v1/todos/abc-123/resolve?redirect=true"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestTodoURLsEmptyInputs(t *testing.T) {
	if got := TodoAcknowledgeURL("", "abc"); got != "" {
		t.Errorf("expected empty, got %q", got)
	}
	if got := TodoResolveURL("https://example.com", ""); got != "" {
		t.Errorf("expected empty, got %q", got)
	}
}
