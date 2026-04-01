package stealth

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/sentioxyz/changelogue/internal/ingestion"
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

// ─────────────────────────────────────────
// Helper: create a source for a given project
// ─────────────────────────────────────────

func makeSource(t *testing.T, s *Store, projectID string) *models.Source {
	t.Helper()
	src := &models.Source{
		ProjectID:           projectID,
		Provider:            "github",
		Repository:          fmt.Sprintf("org/repo-%d", time.Now().UnixNano()),
		PollIntervalSeconds: 3600,
		Enabled:             true,
	}
	if err := s.CreateSource(context.Background(), src); err != nil {
		t.Fatalf("makeSource: CreateSource: %v", err)
	}
	return src
}

// ─────────────────────────────────────────
// Releases Read
// ─────────────────────────────────────────

func TestReleases_Read(t *testing.T) {
	ctx := context.Background()
	s := testStore(t)

	proj := makeProject(t, s)
	src := makeSource(t, s, proj.ID)

	// Insert a release directly (IngestRelease not yet implemented).
	releaseID := "rel-001"
	releasedAt := time.Now().UTC().Add(-time.Hour).Format(time.RFC3339Nano)
	createdAt := time.Now().UTC().Format(time.RFC3339Nano)
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO releases (id, source_id, version, raw_data, released_at, created_at) VALUES (?, ?, ?, ?, ?, ?)`,
		releaseID, src.ID, "v1.0.0", `{"tag":"v1.0.0"}`, releasedAt, createdAt,
	)
	if err != nil {
		t.Fatalf("insert release: %v", err)
	}

	// ListAllReleases
	releases, total, err := s.ListAllReleases(ctx, 1, 10, false, models.ReleaseFilter{})
	if err != nil {
		t.Fatalf("ListAllReleases: %v", err)
	}
	if total != 1 {
		t.Fatalf("expected total=1, got %d", total)
	}
	if len(releases) != 1 {
		t.Fatalf("expected 1 release, got %d", len(releases))
	}
	r := releases[0]
	if r.ID != releaseID {
		t.Errorf("expected id %q, got %q", releaseID, r.ID)
	}
	if r.Version != "v1.0.0" {
		t.Errorf("expected version 'v1.0.0', got %q", r.Version)
	}
	if r.ProjectID != proj.ID {
		t.Errorf("expected project_id %q, got %q", proj.ID, r.ProjectID)
	}
	if r.ProjectName != proj.Name {
		t.Errorf("expected project_name %q, got %q", proj.Name, r.ProjectName)
	}
	if r.Provider != "github" {
		t.Errorf("expected provider 'github', got %q", r.Provider)
	}
	if r.ReleasedAt == nil {
		t.Error("expected ReleasedAt to be set")
	}

	// ListReleasesBySource
	releases, total, err = s.ListReleasesBySource(ctx, src.ID, 1, 10, false, models.ReleaseFilter{})
	if err != nil {
		t.Fatalf("ListReleasesBySource: %v", err)
	}
	if total != 1 || len(releases) != 1 {
		t.Fatalf("expected 1 release by source, got total=%d len=%d", total, len(releases))
	}

	// ListReleasesByProject
	releases, total, err = s.ListReleasesByProject(ctx, proj.ID, 1, 10, false, models.ReleaseFilter{})
	if err != nil {
		t.Fatalf("ListReleasesByProject: %v", err)
	}
	if total != 1 || len(releases) != 1 {
		t.Fatalf("expected 1 release by project, got total=%d len=%d", total, len(releases))
	}

	// GetRelease
	got, err := s.GetRelease(ctx, releaseID)
	if err != nil {
		t.Fatalf("GetRelease: %v", err)
	}
	if got.ID != releaseID {
		t.Errorf("GetRelease id mismatch: %q", got.ID)
	}
	if got.Version != "v1.0.0" {
		t.Errorf("GetRelease version mismatch: %q", got.Version)
	}

	// GetRelease — not found
	_, err = s.GetRelease(ctx, "nonexistent")
	if err == nil {
		t.Fatal("expected error for nonexistent release, got nil")
	}

	// Filter by provider — should match
	releases, total, err = s.ListAllReleases(ctx, 1, 10, false, models.ReleaseFilter{Provider: "github"})
	if err != nil {
		t.Fatalf("ListAllReleases with provider filter: %v", err)
	}
	if total != 1 {
		t.Errorf("expected 1 release with provider=github, got %d", total)
	}

	// Filter by provider — should not match
	releases, total, err = s.ListAllReleases(ctx, 1, 10, false, models.ReleaseFilter{Provider: "npm"})
	if err != nil {
		t.Fatalf("ListAllReleases with non-matching provider: %v", err)
	}
	if total != 0 {
		t.Errorf("expected 0 releases with provider=npm, got %d", total)
	}
	_ = releases
}

// ─────────────────────────────────────────
// Channels CRUD
// ─────────────────────────────────────────

func TestChannels_CRUD(t *testing.T) {
	ctx := context.Background()
	s := testStore(t)

	// Create
	ch := &models.NotificationChannel{
		Name:   "My Slack",
		Type:   "slack",
		Config: json.RawMessage(`{"webhook_url":"https://hooks.slack.com/test"}`),
	}
	if err := s.CreateChannel(ctx, ch); err != nil {
		t.Fatalf("CreateChannel: %v", err)
	}
	if ch.ID == "" {
		t.Fatal("expected ID to be populated")
	}
	if ch.CreatedAt.IsZero() {
		t.Fatal("expected CreatedAt to be populated")
	}

	// List
	channels, total, err := s.ListChannels(ctx, 1, 10)
	if err != nil {
		t.Fatalf("ListChannels: %v", err)
	}
	if total != 1 {
		t.Fatalf("expected total=1, got %d", total)
	}
	if len(channels) != 1 {
		t.Fatalf("expected 1 channel, got %d", len(channels))
	}
	if channels[0].Name != "My Slack" {
		t.Errorf("expected name 'My Slack', got %q", channels[0].Name)
	}
	if string(channels[0].Config) != `{"webhook_url":"https://hooks.slack.com/test"}` {
		t.Errorf("config mismatch: %s", string(channels[0].Config))
	}

	// Get
	gotCh, err := s.GetChannel(ctx, ch.ID)
	if err != nil {
		t.Fatalf("GetChannel: %v", err)
	}
	if gotCh.Type != "slack" {
		t.Errorf("GetChannel type mismatch: %q", gotCh.Type)
	}

	// Update
	ch.Name = "Updated Slack"
	ch.Config = json.RawMessage(`{"webhook_url":"https://hooks.slack.com/updated"}`)
	if err := s.UpdateChannel(ctx, ch.ID, ch); err != nil {
		t.Fatalf("UpdateChannel: %v", err)
	}
	gotCh, err = s.GetChannel(ctx, ch.ID)
	if err != nil {
		t.Fatalf("GetChannel after update: %v", err)
	}
	if gotCh.Name != "Updated Slack" {
		t.Errorf("expected updated name, got %q", gotCh.Name)
	}
	if string(gotCh.Config) != `{"webhook_url":"https://hooks.slack.com/updated"}` {
		t.Errorf("config after update mismatch: %s", string(gotCh.Config))
	}

	// Update non-existent
	if err := s.UpdateChannel(ctx, "nonexistent", ch); err == nil {
		t.Fatal("expected error updating non-existent channel")
	}

	// Delete
	if err := s.DeleteChannel(ctx, ch.ID); err != nil {
		t.Fatalf("DeleteChannel: %v", err)
	}
	_, err = s.GetChannel(ctx, ch.ID)
	if err == nil {
		t.Fatal("expected error after delete, got nil")
	}

	// Delete non-existent
	if err := s.DeleteChannel(ctx, ch.ID); err == nil {
		t.Fatal("expected error deleting non-existent channel")
	}

	// List after delete
	channels, total, err = s.ListChannels(ctx, 1, 10)
	if err != nil {
		t.Fatalf("ListChannels after delete: %v", err)
	}
	if total != 0 {
		t.Fatalf("expected total=0 after delete, got %d", total)
	}
	_ = channels
}

// ─────────────────────────────────────────
// Subscriptions CRUD
// ─────────────────────────────────────────

func makeChannel(t *testing.T, s *Store) *models.NotificationChannel {
	t.Helper()
	ch := &models.NotificationChannel{
		Name:   "Test Channel",
		Type:   "slack",
		Config: json.RawMessage(`{"webhook_url":"https://hooks.slack.com/x"}`),
	}
	if err := s.CreateChannel(context.Background(), ch); err != nil {
		t.Fatalf("makeChannel: CreateChannel: %v", err)
	}
	return ch
}

func TestSubscriptions_CRUD(t *testing.T) {
	ctx := context.Background()
	s := testStore(t)

	proj := makeProject(t, s)
	src := makeSource(t, s, proj.ID)
	ch := makeChannel(t, s)

	srcID := src.ID

	// Create (source_release)
	sub := &models.Subscription{
		ChannelID:     ch.ID,
		Type:          "source_release",
		SourceID:      &srcID,
		VersionFilter: "v1.*",
		Config:        json.RawMessage(`{"notify_on":"all"}`),
	}
	if err := s.CreateSubscription(ctx, sub); err != nil {
		t.Fatalf("CreateSubscription: %v", err)
	}
	if sub.ID == "" {
		t.Fatal("expected ID to be populated")
	}
	if sub.CreatedAt.IsZero() {
		t.Fatal("expected CreatedAt to be populated")
	}

	// List
	subs, total, err := s.ListSubscriptions(ctx, 1, 10)
	if err != nil {
		t.Fatalf("ListSubscriptions: %v", err)
	}
	if total != 1 {
		t.Fatalf("expected total=1, got %d", total)
	}
	if len(subs) != 1 {
		t.Fatalf("expected 1 subscription, got %d", len(subs))
	}
	if subs[0].ChannelID != ch.ID {
		t.Errorf("expected channel_id %q, got %q", ch.ID, subs[0].ChannelID)
	}
	if subs[0].SourceID == nil || *subs[0].SourceID != srcID {
		t.Errorf("expected source_id %q, got %v", srcID, subs[0].SourceID)
	}
	if subs[0].VersionFilter != "v1.*" {
		t.Errorf("expected version_filter 'v1.*', got %q", subs[0].VersionFilter)
	}
	if string(subs[0].Config) != `{"notify_on":"all"}` {
		t.Errorf("config mismatch: %s", string(subs[0].Config))
	}

	// Get
	gotSub, err := s.GetSubscription(ctx, sub.ID)
	if err != nil {
		t.Fatalf("GetSubscription: %v", err)
	}
	if gotSub.Type != "source_release" {
		t.Errorf("GetSubscription type mismatch: %q", gotSub.Type)
	}

	// Update
	sub.VersionFilter = "v2.*"
	sub.Config = json.RawMessage(`{"notify_on":"major"}`)
	if err := s.UpdateSubscription(ctx, sub.ID, sub); err != nil {
		t.Fatalf("UpdateSubscription: %v", err)
	}
	gotSub, err = s.GetSubscription(ctx, sub.ID)
	if err != nil {
		t.Fatalf("GetSubscription after update: %v", err)
	}
	if gotSub.VersionFilter != "v2.*" {
		t.Errorf("expected updated version_filter, got %q", gotSub.VersionFilter)
	}
	if string(gotSub.Config) != `{"notify_on":"major"}` {
		t.Errorf("config after update mismatch: %s", string(gotSub.Config))
	}

	// Update non-existent
	if err := s.UpdateSubscription(ctx, "nonexistent", sub); err == nil {
		t.Fatal("expected error updating non-existent subscription")
	}

	// Delete
	if err := s.DeleteSubscription(ctx, sub.ID); err != nil {
		t.Fatalf("DeleteSubscription: %v", err)
	}
	_, err = s.GetSubscription(ctx, sub.ID)
	if err == nil {
		t.Fatal("expected error after delete, got nil")
	}

	// Delete non-existent
	if err := s.DeleteSubscription(ctx, sub.ID); err == nil {
		t.Fatal("expected error deleting non-existent subscription")
	}

	// Batch create
	projID := proj.ID
	batchSubs := []models.Subscription{
		{ChannelID: ch.ID, Type: "semantic_release", ProjectID: &projID},
		{ChannelID: ch.ID, Type: "source_release", SourceID: &srcID},
	}
	created, err := s.CreateSubscriptionBatch(ctx, batchSubs)
	if err != nil {
		t.Fatalf("CreateSubscriptionBatch: %v", err)
	}
	if len(created) != 2 {
		t.Fatalf("expected 2 created subscriptions, got %d", len(created))
	}
	for _, c := range created {
		if c.ID == "" {
			t.Error("expected each batch subscription to have an ID")
		}
	}

	// Batch delete
	batchIDs := []string{created[0].ID, created[1].ID}
	if err := s.DeleteSubscriptionBatch(ctx, batchIDs); err != nil {
		t.Fatalf("DeleteSubscriptionBatch: %v", err)
	}
	// Verify both are gone
	for _, id := range batchIDs {
		_, err := s.GetSubscription(ctx, id)
		if err == nil {
			t.Errorf("expected subscription %q to be deleted", id)
		}
	}

	// List after all deletes
	subs, total, err = s.ListSubscriptions(ctx, 1, 10)
	if err != nil {
		t.Fatalf("ListSubscriptions after delete: %v", err)
	}
	if total != 0 {
		t.Fatalf("expected total=0, got %d", total)
	}
	_ = subs
}

// ─────────────────────────────────────────
// KeyStore
// ─────────────────────────────────────────

func TestKeyStore(t *testing.T) {
	ctx := context.Background()
	s := testStore(t)

	rawKey := "my-secret-api-key"
	hash := fmt.Sprintf("%x", sha256.Sum256([]byte(rawKey)))

	// Insert an API key manually.
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO api_keys (id, name, key_hash, key_prefix, created_at) VALUES (?, ?, ?, ?, ?)`,
		"key-001", "Test Key", hash, "my-s",
		time.Now().UTC().Format(time.RFC3339Nano),
	)
	if err != nil {
		t.Fatalf("insert api_key: %v", err)
	}

	// ValidateKey — valid key
	valid, err := s.ValidateKey(ctx, rawKey)
	if err != nil {
		t.Fatalf("ValidateKey: %v", err)
	}
	if !valid {
		t.Error("expected key to be valid")
	}

	// ValidateKey — invalid key
	valid, err = s.ValidateKey(ctx, "wrong-key")
	if err != nil {
		t.Fatalf("ValidateKey (wrong): %v", err)
	}
	if valid {
		t.Error("expected wrong key to be invalid")
	}

	// TouchKeyUsage — should update last_used_at without error
	s.TouchKeyUsage(ctx, rawKey)

	var lastUsedAt sql.NullString
	err = s.db.QueryRowContext(ctx, `SELECT last_used_at FROM api_keys WHERE id = 'key-001'`).Scan(&lastUsedAt)
	if err != nil {
		t.Fatalf("select last_used_at: %v", err)
	}
	if !lastUsedAt.Valid || lastUsedAt.String == "" {
		t.Error("expected last_used_at to be set after TouchKeyUsage")
	}
}

// ─────────────────────────────────────────
// HealthChecker
// ─────────────────────────────────────────

func TestHealthChecker(t *testing.T) {
	ctx := context.Background()
	s := testStore(t)

	proj := makeProject(t, s)
	src := makeSource(t, s, proj.ID)

	// Insert 2 releases.
	for i := 0; i < 2; i++ {
		_, err := s.db.ExecContext(ctx,
			`INSERT INTO releases (id, source_id, version, created_at) VALUES (?, ?, ?, ?)`,
			fmt.Sprintf("rel-%d", i), src.ID, fmt.Sprintf("v1.%d.0", i),
			time.Now().UTC().Format(time.RFC3339Nano),
		)
		if err != nil {
			t.Fatalf("insert release %d: %v", i, err)
		}
	}

	stats, err := s.GetStats(ctx)
	if err != nil {
		t.Fatalf("GetStats: %v", err)
	}
	if stats.TotalReleases != 2 {
		t.Errorf("expected TotalReleases=2, got %d", stats.TotalReleases)
	}
	if stats.ActiveSources != 1 {
		t.Errorf("expected ActiveSources=1, got %d", stats.ActiveSources)
	}
	if stats.TotalProjects != 1 {
		t.Errorf("expected TotalProjects=1, got %d", stats.TotalProjects)
	}

	// GetTrend — stealth mode returns empty
	buckets, err := s.GetTrend(ctx, "daily", time.Now().Add(-7*24*time.Hour), time.Now())
	if err != nil {
		t.Fatalf("GetTrend: %v", err)
	}
	if len(buckets) != 0 {
		t.Errorf("expected 0 trend buckets in stealth mode, got %d", len(buckets))
	}
}

// ─────────────────────────────────────────
// IngestRelease
// ─────────────────────────────────────────

func TestIngestRelease(t *testing.T) {
	ctx := context.Background()
	store := testStore(t)

	// Setup: project + source
	p := &models.Project{Name: "proj", AgentRules: json.RawMessage(`{}`)}
	if err := store.CreateProject(ctx, p); err != nil {
		t.Fatalf("CreateProject: %v", err)
	}
	src := &models.Source{
		ProjectID: p.ID, Provider: "github", Repository: "owner/repo",
		PollIntervalSeconds: 300, Enabled: true,
	}
	if err := store.CreateSource(ctx, src); err != nil {
		t.Fatalf("CreateSource: %v", err)
	}

	// Ingest
	result := &ingestion.IngestionResult{
		Repository: "owner/repo",
		RawVersion: "v1.0.0",
		Timestamp:  time.Now(),
		Metadata:   map[string]string{"tag": "v1.0.0"},
	}
	if err := store.IngestRelease(ctx, src.ID, result); err != nil {
		t.Fatalf("IngestRelease: %v", err)
	}

	// Verify release exists
	releases, total, err := store.ListReleasesBySource(ctx, src.ID, 1, 50, false, models.ReleaseFilter{})
	if err != nil {
		t.Fatalf("ListReleasesBySource: %v", err)
	}
	if total != 1 || len(releases) != 1 {
		t.Fatalf("got %d/%d, want 1/1", len(releases), total)
	}
	if releases[0].Version != "v1.0.0" {
		t.Errorf("got version %q, want %q", releases[0].Version, "v1.0.0")
	}

	// Duplicate should return a unique constraint error
	err = store.IngestRelease(ctx, src.ID, result)
	if err == nil || !strings.Contains(err.Error(), "UNIQUE constraint failed") {
		t.Fatalf("expected unique constraint error on duplicate, got: %v", err)
	}
}

// ─────────────────────────────────────────
// NotifyStore methods
// ─────────────────────────────────────────

func TestNotifyStoreMethods(t *testing.T) {
	ctx := context.Background()
	store := testStore(t)

	proj := makeProject(t, store)
	src := makeSource(t, store, proj.ID)
	ch := makeChannel(t, store)

	srcID := src.ID

	// Create a source_release subscription
	sub := &models.Subscription{
		ChannelID: ch.ID,
		Type:      "source_release",
		SourceID:  &srcID,
		Config:    json.RawMessage(`{"command":"echo test"}`),
	}
	if err := store.CreateSubscription(ctx, sub); err != nil {
		t.Fatalf("CreateSubscription: %v", err)
	}

	// ListSourceSubscriptions
	subs, err := store.ListSourceSubscriptions(ctx, srcID)
	if err != nil {
		t.Fatalf("ListSourceSubscriptions: %v", err)
	}
	if len(subs) != 1 {
		t.Fatalf("expected 1 subscription, got %d", len(subs))
	}
	if subs[0].ID != sub.ID {
		t.Errorf("subscription ID mismatch: got %q, want %q", subs[0].ID, sub.ID)
	}

	// GetPreviousRelease — no releases yet
	prev, err := store.GetPreviousRelease(ctx, srcID, "v1.0.0")
	if err != nil {
		t.Fatalf("GetPreviousRelease: %v", err)
	}
	if prev != nil {
		t.Error("expected nil previous release")
	}

	// Ingest two releases
	r1 := &ingestion.IngestionResult{RawVersion: "v1.0.0", Timestamp: time.Now().Add(-time.Hour)}
	if err := store.IngestRelease(ctx, srcID, r1); err != nil {
		t.Fatalf("IngestRelease v1.0.0: %v", err)
	}
	r2 := &ingestion.IngestionResult{RawVersion: "v2.0.0", Timestamp: time.Now()}
	if err := store.IngestRelease(ctx, srcID, r2); err != nil {
		t.Fatalf("IngestRelease v2.0.0: %v", err)
	}

	// GetPreviousRelease — should find v1.0.0
	prev, err = store.GetPreviousRelease(ctx, srcID, "v2.0.0")
	if err != nil {
		t.Fatalf("GetPreviousRelease: %v", err)
	}
	if prev == nil {
		t.Fatal("expected non-nil previous release")
	}
	if prev.Version != "v1.0.0" {
		t.Errorf("expected previous version v1.0.0, got %q", prev.Version)
	}

	// No-op methods
	if err := store.EnqueueAgentRun(ctx, proj.ID, "test", "v1.0.0"); err != nil {
		t.Fatalf("EnqueueAgentRun: %v", err)
	}
	todoID, err := store.CreateReleaseTodo(ctx, "some-release-id")
	if err != nil {
		t.Fatalf("CreateReleaseTodo: %v", err)
	}
	if todoID != "" {
		t.Errorf("expected empty todoID, got %q", todoID)
	}
	hasGate, err := store.HasReleaseGate(ctx, proj.ID)
	if err != nil {
		t.Fatalf("HasReleaseGate: %v", err)
	}
	if hasGate {
		t.Error("expected HasReleaseGate=false")
	}
}
