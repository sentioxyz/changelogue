package cli

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/sentioxyz/changelogue/internal/models"
)

func TestProjectsList(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/projects" {
			t.Errorf("expected /api/v1/projects, got %s", r.URL.Path)
		}
		if r.URL.Query().Get("page") != "1" {
			t.Errorf("expected page=1, got %s", r.URL.Query().Get("page"))
		}
		json.NewEncoder(w).Encode(map[string]any{
			"data": []models.Project{{ID: "p1", Name: "proj1"}},
			"meta": map[string]any{"request_id": "r1", "page": 1, "per_page": 25, "total": 1},
		})
	}))
	defer srv.Close()

	c := NewClient(srv.URL, "key")
	projects, meta, err := ListProjects(c, 1, 25)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(projects) != 1 {
		t.Fatalf("expected 1 project, got %d", len(projects))
	}
	if projects[0].Name != "proj1" {
		t.Errorf("expected proj1, got %s", projects[0].Name)
	}
	if meta.Total != 1 {
		t.Errorf("expected total=1, got %d", meta.Total)
	}
}

func TestProjectsCreate(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", r.Method)
		}
		var body map[string]string
		json.NewDecoder(r.Body).Decode(&body)
		if body["name"] != "new-project" {
			t.Errorf("expected name=new-project, got %s", body["name"])
		}
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(map[string]any{
			"data": models.Project{ID: "p2", Name: "new-project"},
			"meta": map[string]any{"request_id": "r2"},
		})
	}))
	defer srv.Close()

	c := NewClient(srv.URL, "key")
	proj, err := CreateProject(c, "new-project", "desc", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if proj.ID != "p2" {
		t.Errorf("expected p2, got %s", proj.ID)
	}
}

func TestProjectsDelete(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete {
			t.Errorf("expected DELETE, got %s", r.Method)
		}
		if r.URL.Path != "/api/v1/projects/p1" {
			t.Errorf("expected /api/v1/projects/p1, got %s", r.URL.Path)
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()

	c := NewClient(srv.URL, "key")
	err := DeleteProject(c, "p1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestProjectsGet(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/projects/p1" {
			t.Errorf("expected /api/v1/projects/p1, got %s", r.URL.Path)
		}
		json.NewEncoder(w).Encode(map[string]any{
			"data": models.Project{ID: "p1", Name: "my-project"},
			"meta": map[string]any{"request_id": "r3"},
		})
	}))
	defer srv.Close()

	c := NewClient(srv.URL, "key")
	proj, err := GetProject(c, "p1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if proj.Name != "my-project" {
		t.Errorf("expected my-project, got %s", proj.Name)
	}
}

func TestProjectsUpdate(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPut || r.URL.Path != "/api/v1/projects/p1" {
			t.Errorf("unexpected %s %s", r.Method, r.URL.Path)
		}
		var body map[string]any
		json.NewDecoder(r.Body).Decode(&body)
		if body["name"] != "updated-name" {
			t.Errorf("expected name=updated-name, got %v", body["name"])
		}
		json.NewEncoder(w).Encode(map[string]any{
			"data": models.Project{ID: "p1", Name: "updated-name"},
			"meta": map[string]any{"request_id": "r4"},
		})
	}))
	defer srv.Close()

	c := NewClient(srv.URL, "key")
	proj, err := UpdateProject(c, "p1", map[string]any{"name": "updated-name"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if proj.Name != "updated-name" {
		t.Errorf("expected updated-name, got %s", proj.Name)
	}
}
