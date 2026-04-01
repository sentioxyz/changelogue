package stealth

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/sentioxyz/changelogue/internal/models"
)

func testStore(t *testing.T) *Store {
	t.Helper()
	dbPath := filepath.Join(t.TempDir(), "test.db")
	store, err := NewStore(dbPath)
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}
	t.Cleanup(func() { store.Close() })
	return store
}

func TestNewStore(t *testing.T) {
	store := testStore(t)
	if err := store.PingDB(context.Background()); err != nil {
		t.Fatalf("PingDB: %v", err)
	}
}

func TestNewStore_CreatesDirectory(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "sub", "dir")
	dbPath := filepath.Join(dir, "test.db")
	store, err := NewStore(dbPath)
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}
	defer store.Close()
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		t.Fatal("expected directory to be created")
	}
}

// ─────────────────────────────────────────
// Projects CRUD
// ─────────────────────────────────────────

func TestProjects_CRUD(t *testing.T) {
	ctx := context.Background()
	s := testStore(t)

	// Create
	p := &models.Project{
		Name:        "Test Project",
		Description: "A description",
		AgentPrompt: "Summarise releases",
		AgentRules:  json.RawMessage(`{"on_major_release":true}`),
	}
	if err := s.CreateProject(ctx, p); err != nil {
		t.Fatalf("CreateProject: %v", err)
	}
	if p.ID == "" {
		t.Fatal("expected ID to be populated")
	}
	if p.CreatedAt.IsZero() {
		t.Fatal("expected CreatedAt to be populated")
	}

	// List
	projects, total, err := s.ListProjects(ctx, 1, 10)
	if err != nil {
		t.Fatalf("ListProjects: %v", err)
	}
	if total != 1 {
		t.Fatalf("expected total=1, got %d", total)
	}
	if len(projects) != 1 {
		t.Fatalf("expected 1 project, got %d", len(projects))
	}
	if projects[0].Name != "Test Project" {
		t.Errorf("expected name 'Test Project', got %q", projects[0].Name)
	}

	// Get
	got, err := s.GetProject(ctx, p.ID)
	if err != nil {
		t.Fatalf("GetProject: %v", err)
	}
	if got.Name != p.Name {
		t.Errorf("GetProject name mismatch: %q vs %q", got.Name, p.Name)
	}
	if got.Description != "A description" {
		t.Errorf("GetProject description mismatch: %q", got.Description)
	}
	if string(got.AgentRules) != `{"on_major_release":true}` {
		t.Errorf("GetProject agent_rules mismatch: %s", string(got.AgentRules))
	}

	// Update
	p.Name = "Updated Project"
	p.Description = "Updated desc"
	p.AgentRules = json.RawMessage(`{"on_minor_release":true}`)
	if err := s.UpdateProject(ctx, p.ID, p); err != nil {
		t.Fatalf("UpdateProject: %v", err)
	}
	got, err = s.GetProject(ctx, p.ID)
	if err != nil {
		t.Fatalf("GetProject after update: %v", err)
	}
	if got.Name != "Updated Project" {
		t.Errorf("expected updated name, got %q", got.Name)
	}
	if got.Description != "Updated desc" {
		t.Errorf("expected updated description, got %q", got.Description)
	}

	// Delete
	if err := s.DeleteProject(ctx, p.ID); err != nil {
		t.Fatalf("DeleteProject: %v", err)
	}
	_, err = s.GetProject(ctx, p.ID)
	if err == nil {
		t.Fatal("expected error after delete, got nil")
	}

	// Delete non-existent should error
	if err := s.DeleteProject(ctx, p.ID); err == nil {
		t.Fatal("expected error deleting non-existent project")
	}

	// Update non-existent should error
	if err := s.UpdateProject(ctx, "nonexistent", p); err == nil {
		t.Fatal("expected error updating non-existent project")
	}

	// List after delete
	projects, total, err = s.ListProjects(ctx, 1, 10)
	if err != nil {
		t.Fatalf("ListProjects after delete: %v", err)
	}
	if total != 0 {
		t.Fatalf("expected total=0 after delete, got %d", total)
	}
	if len(projects) != 0 {
		t.Fatalf("expected 0 projects after delete, got %d", len(projects))
	}
}

// ─────────────────────────────────────────
// Sources CRUD + ListEnabledSources + UpdateSourcePollStatus
// ─────────────────────────────────────────

func makeProject(t *testing.T, s *Store) *models.Project {
	t.Helper()
	p := &models.Project{
		Name:       "Proj for Sources",
		AgentRules: json.RawMessage(`{}`),
	}
	if err := s.CreateProject(context.Background(), p); err != nil {
		t.Fatalf("makeProject: CreateProject: %v", err)
	}
	return p
}

func TestSources_CRUD(t *testing.T) {
	ctx := context.Background()
	s := testStore(t)
	p := makeProject(t, s)

	vfi := "v1.*"
	// Create
	src := &models.Source{
		ProjectID:            p.ID,
		Provider:             "github",
		Repository:           "org/repo",
		PollIntervalSeconds:  3600,
		Enabled:              true,
		VersionFilterInclude: &vfi,
		ExcludePrereleases:   true,
	}
	if err := s.CreateSource(ctx, src); err != nil {
		t.Fatalf("CreateSource: %v", err)
	}
	if src.ID == "" {
		t.Fatal("expected ID to be populated")
	}
	if src.CreatedAt.IsZero() {
		t.Fatal("expected CreatedAt to be populated")
	}

	// List
	sources, total, err := s.ListSourcesByProject(ctx, p.ID, 1, 10)
	if err != nil {
		t.Fatalf("ListSourcesByProject: %v", err)
	}
	if total != 1 {
		t.Fatalf("expected total=1, got %d", total)
	}
	if len(sources) != 1 {
		t.Fatalf("expected 1 source, got %d", len(sources))
	}
	if sources[0].Provider != "github" {
		t.Errorf("expected provider 'github', got %q", sources[0].Provider)
	}
	if !sources[0].Enabled {
		t.Error("expected Enabled=true")
	}
	if !sources[0].ExcludePrereleases {
		t.Error("expected ExcludePrereleases=true")
	}
	if sources[0].VersionFilterInclude == nil || *sources[0].VersionFilterInclude != "v1.*" {
		t.Errorf("expected VersionFilterInclude='v1.*', got %v", sources[0].VersionFilterInclude)
	}

	// Get
	got, err := s.GetSource(ctx, src.ID)
	if err != nil {
		t.Fatalf("GetSource: %v", err)
	}
	if got.Repository != "org/repo" {
		t.Errorf("GetSource repository mismatch: %q", got.Repository)
	}

	// Update
	src.Repository = "org/other-repo"
	src.Enabled = false
	src.ExcludePrereleases = false
	src.VersionFilterInclude = nil
	if err := s.UpdateSource(ctx, src.ID, src); err != nil {
		t.Fatalf("UpdateSource: %v", err)
	}
	got, err = s.GetSource(ctx, src.ID)
	if err != nil {
		t.Fatalf("GetSource after update: %v", err)
	}
	if got.Repository != "org/other-repo" {
		t.Errorf("expected updated repository, got %q", got.Repository)
	}
	if got.Enabled {
		t.Error("expected Enabled=false after update")
	}
	if got.VersionFilterInclude != nil {
		t.Errorf("expected VersionFilterInclude=nil after update, got %v", got.VersionFilterInclude)
	}

	// UpdateSourcePollStatus — success
	if err := s.UpdateSourcePollStatus(ctx, src.ID, nil); err != nil {
		t.Fatalf("UpdateSourcePollStatus (nil): %v", err)
	}
	got, err = s.GetSource(ctx, src.ID)
	if err != nil {
		t.Fatalf("GetSource after poll status update: %v", err)
	}
	if got.LastPolledAt == nil {
		t.Error("expected LastPolledAt to be set")
	}
	if got.LastError != nil {
		t.Errorf("expected LastError=nil, got %v", got.LastError)
	}

	// UpdateSourcePollStatus — with error
	pollErr := errors.New("upstream timeout")
	if err := s.UpdateSourcePollStatus(ctx, src.ID, pollErr); err != nil {
		t.Fatalf("UpdateSourcePollStatus (err): %v", err)
	}
	got, err = s.GetSource(ctx, src.ID)
	if err != nil {
		t.Fatalf("GetSource after error poll status: %v", err)
	}
	if got.LastError == nil || *got.LastError != "upstream timeout" {
		t.Errorf("expected LastError='upstream timeout', got %v", got.LastError)
	}

	// ListEnabledSources — source is disabled, should not appear
	enabled, err := s.ListEnabledSources(ctx)
	if err != nil {
		t.Fatalf("ListEnabledSources: %v", err)
	}
	if len(enabled) != 0 {
		t.Fatalf("expected 0 enabled sources (disabled), got %d", len(enabled))
	}

	// Re-enable and check ListEnabledSources
	src.Enabled = true
	if err := s.UpdateSource(ctx, src.ID, src); err != nil {
		t.Fatalf("re-enable source: %v", err)
	}
	enabled, err = s.ListEnabledSources(ctx)
	if err != nil {
		t.Fatalf("ListEnabledSources after re-enable: %v", err)
	}
	if len(enabled) != 1 {
		t.Fatalf("expected 1 enabled source, got %d", len(enabled))
	}
	if enabled[0].Provider != "github" {
		t.Errorf("expected provider 'github', got %q", enabled[0].Provider)
	}
	if enabled[0].LastPolledAt == nil {
		t.Error("expected LastPolledAt to be set on enabled source")
	}

	// ListAllSourceRepos
	repos, err := s.ListAllSourceRepos(ctx)
	if err != nil {
		t.Fatalf("ListAllSourceRepos: %v", err)
	}
	if len(repos) != 1 {
		t.Fatalf("expected 1 repo, got %d", len(repos))
	}
	if repos[0].Provider != "github" || repos[0].Repository != "org/other-repo" {
		t.Errorf("unexpected repo: %+v", repos[0])
	}

	// Delete
	if err := s.DeleteSource(ctx, src.ID); err != nil {
		t.Fatalf("DeleteSource: %v", err)
	}
	_, err = s.GetSource(ctx, src.ID)
	if err == nil {
		t.Fatal("expected error after delete, got nil")
	}

	// Delete non-existent
	if err := s.DeleteSource(ctx, src.ID); err == nil {
		t.Fatal("expected error deleting non-existent source")
	}

	// Update non-existent
	if err := s.UpdateSource(ctx, "nonexistent", src); err == nil {
		t.Fatal("expected error updating non-existent source")
	}

	// List after delete
	sources, total, err = s.ListSourcesByProject(ctx, p.ID, 1, 10)
	if err != nil {
		t.Fatalf("ListSourcesByProject after delete: %v", err)
	}
	if total != 0 {
		t.Fatalf("expected total=0 after delete, got %d", total)
	}
	if len(sources) != 0 {
		t.Fatalf("expected 0 sources after delete, got %d", len(sources))
	}
}

