package onboard

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestParseRepoURL(t *testing.T) {
	tests := []struct {
		input     string
		owner     string
		repo      string
		expectErr bool
	}{
		{"owner/repo", "owner", "repo", false},
		{"https://github.com/owner/repo", "owner", "repo", false},
		{"https://github.com/owner/repo.git", "owner", "repo", false},
		{"http://github.com/owner/repo", "owner", "repo", false},
		{"github.com/owner/repo", "owner", "repo", false},
		{"", "", "", true},
		{"justowner", "", "", true},
	}
	for _, tc := range tests {
		t.Run(tc.input, func(t *testing.T) {
			owner, repo, err := ParseRepoURL(tc.input)
			if tc.expectErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if owner != tc.owner {
				t.Errorf("owner = %q, want %q", owner, tc.owner)
			}
			if repo != tc.repo {
				t.Errorf("repo = %q, want %q", repo, tc.repo)
			}
		})
	}
}

func TestFetchDependencyFiles(t *testing.T) {
	// Mock GitHub API
	mux := http.NewServeMux()

	// Repo metadata (default branch)
	mux.HandleFunc("GET /repos/test/repo", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{"default_branch": "main"})
	})

	// Tree
	mux.HandleFunc("GET /repos/test/repo/git/trees/main", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{
			"tree": []map[string]any{
				{"path": "go.mod", "type": "blob"},
				{"path": "README.md", "type": "blob"},
				{"path": "package.json", "type": "blob"},
				{"path": "src/main.go", "type": "blob"},
			},
		})
	})

	// File contents
	mux.HandleFunc("GET /repos/test/repo/contents/go.mod", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{
			"content":  "bW9kdWxlIGV4YW1wbGUuY29t", // base64 of "module example.com"
			"encoding": "base64",
		})
	})
	mux.HandleFunc("GET /repos/test/repo/contents/package.json", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{
			"content":  "eyJuYW1lIjogInRlc3QifQ==", // base64 of `{"name": "test"}`
			"encoding": "base64",
		})
	})

	server := httptest.NewServer(mux)
	defer server.Close()

	scanner := NewScanner(server.Client(), server.URL, "test-token")
	files, err := scanner.FetchDependencyFiles(context.Background(), "test", "repo")
	if err != nil {
		t.Fatalf("FetchDependencyFiles: %v", err)
	}
	if len(files) != 2 {
		t.Fatalf("expected 2 dep files, got %d", len(files))
	}
	if files[0].Path != "go.mod" {
		t.Errorf("files[0].Path = %q, want go.mod", files[0].Path)
	}
	if files[0].Content != "module example.com" {
		t.Errorf("files[0].Content = %q, want 'module example.com'", files[0].Content)
	}
}
