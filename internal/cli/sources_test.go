package cli

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/sentioxyz/changelogue/internal/models"
)

func TestSourcesList(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/projects/proj-1/sources" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		json.NewEncoder(w).Encode(map[string]any{
			"data": []models.Source{{ID: "s1", ProjectID: "proj-1", Provider: "dockerhub", Repository: "library/postgres"}},
			"meta": map[string]any{"request_id": "r1", "page": 1, "per_page": 25, "total": 1},
		})
	}))
	defer srv.Close()

	c := NewClient(srv.URL, "key")
	sources, _, err := ListSources(c, "proj-1", 1, 25)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(sources) != 1 || sources[0].Provider != "dockerhub" {
		t.Errorf("unexpected sources: %+v", sources)
	}
}

func TestSourcesCreate(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/api/v1/projects/proj-1/sources" {
			t.Errorf("unexpected %s %s", r.Method, r.URL.Path)
		}
		var body map[string]any
		json.NewDecoder(r.Body).Decode(&body)
		if body["provider"] != "dockerhub" {
			t.Errorf("expected provider=dockerhub, got %v", body["provider"])
		}
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(map[string]any{
			"data": models.Source{ID: "s2", Provider: "dockerhub", Repository: "library/postgres"},
			"meta": map[string]any{"request_id": "r2"},
		})
	}))
	defer srv.Close()

	c := NewClient(srv.URL, "key")
	src, err := CreateSource(c, "proj-1", map[string]any{"provider": "dockerhub", "repository": "library/postgres"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if src.ID != "s2" {
		t.Errorf("expected s2, got %s", src.ID)
	}
}
