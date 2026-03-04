package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestDiscoverGitHub(t *testing.T) {
	// Mock GitHub Search API responding with one repo item.
	mockGH := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/search/repositories" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{
			"total_count": 1,
			"items": [{
				"name": "kubernetes",
				"full_name": "kubernetes/kubernetes",
				"description": "Production-Grade Container Scheduling and Management",
				"stargazers_count": 100000,
				"language": "Go",
				"html_url": "https://github.com/kubernetes/kubernetes",
				"owner": {
					"avatar_url": "https://avatars.githubusercontent.com/u/13629408"
				}
			}]
		}`)
	}))
	defer mockGH.Close()

	h := NewDiscoverHandler(mockGH.Client(), mockGH.URL, "")

	mux := http.NewServeMux()
	mux.HandleFunc("GET /discover/github", h.GitHub)

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/discover/github?q=kubernetes", nil)
	mux.ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", w.Code)
	}

	var got struct {
		Data []DiscoverItem `json:"data"`
	}
	if err := json.NewDecoder(w.Body).Decode(&got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(got.Data) != 1 {
		t.Fatalf("expected 1 item, got %d", len(got.Data))
	}
	item := got.Data[0]
	if item.FullName != "kubernetes/kubernetes" {
		t.Fatalf("expected full_name=kubernetes/kubernetes, got %s", item.FullName)
	}
	if item.Stars != 100000 {
		t.Fatalf("expected stars=100000, got %d", item.Stars)
	}
	if item.Provider != "github" {
		t.Fatalf("expected provider=github, got %s", item.Provider)
	}
}
