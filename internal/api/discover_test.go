package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
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

func TestDiscoverDockerHub(t *testing.T) {
	// Mock Docker Hub Search API responding with one result.
	mockDH := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v2/search/repositories/" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{
			"results": [{
				"repo_name": "nginx",
				"short_description": "Official NGINX image",
				"star_count": 19000,
				"is_official": true
			}]
		}`)
	}))
	defer mockDH.Close()

	h := NewDiscoverHandler(mockDH.Client(), "", mockDH.URL)

	mux := http.NewServeMux()
	mux.HandleFunc("GET /discover/dockerhub", h.DockerHub)

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/discover/dockerhub?q=nginx", nil)
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
	if item.Name != "nginx" {
		t.Fatalf("expected name=nginx, got %s", item.Name)
	}
	if item.Provider != "dockerhub" {
		t.Fatalf("expected provider=dockerhub, got %s", item.Provider)
	}
}

func TestDiscoverGitHubUpstreamError(t *testing.T) {
	// Mock GitHub returning 503 Service Unavailable.
	mockGH := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer mockGH.Close()

	h := NewDiscoverHandler(mockGH.Client(), mockGH.URL, "")

	mux := http.NewServeMux()
	mux.HandleFunc("GET /discover/github", h.GitHub)

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/discover/github?q=test", nil)
	mux.ServeHTTP(w, r)

	if w.Code != http.StatusBadGateway {
		t.Fatalf("expected status 502, got %d", w.Code)
	}
}

func TestDiscoverCaching(t *testing.T) {
	// Mock GitHub that counts how many times it is called.
	var callCount atomic.Int32
	mockGH := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount.Add(1)
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{
			"total_count": 1,
			"items": [{
				"name": "cached-repo",
				"full_name": "org/cached-repo",
				"description": "A repo for cache testing",
				"stargazers_count": 42,
				"language": "Go",
				"html_url": "https://github.com/org/cached-repo",
				"owner": {
					"avatar_url": "https://avatars.githubusercontent.com/u/1"
				}
			}]
		}`)
	}))
	defer mockGH.Close()

	h := NewDiscoverHandler(mockGH.Client(), mockGH.URL, "")

	mux := http.NewServeMux()
	mux.HandleFunc("GET /discover/github", h.GitHub)

	// First request — should hit the mock.
	w1 := httptest.NewRecorder()
	r1 := httptest.NewRequest(http.MethodGet, "/discover/github?q=cached", nil)
	mux.ServeHTTP(w1, r1)
	if w1.Code != http.StatusOK {
		t.Fatalf("first request: expected status 200, got %d", w1.Code)
	}

	// Second request with the same params — should be served from cache.
	w2 := httptest.NewRecorder()
	r2 := httptest.NewRequest(http.MethodGet, "/discover/github?q=cached", nil)
	mux.ServeHTTP(w2, r2)
	if w2.Code != http.StatusOK {
		t.Fatalf("second request: expected status 200, got %d", w2.Code)
	}

	if calls := callCount.Load(); calls != 1 {
		t.Fatalf("expected mock to be called once (second request cached), got %d calls", calls)
	}
}
