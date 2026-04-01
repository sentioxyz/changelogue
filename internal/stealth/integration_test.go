package stealth

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/sentioxyz/changelogue/internal/api"
	"github.com/sentioxyz/changelogue/internal/ingestion"
	"github.com/sentioxyz/changelogue/internal/routing"
)

// ─────────────────────────────────────────
// HTTP helpers
// ─────────────────────────────────────────

func get(t *testing.T, ts *httptest.Server, path string) *http.Response {
	t.Helper()
	resp, err := http.Get(ts.URL + path)
	if err != nil {
		t.Fatalf("GET %s: %v", path, err)
	}
	return resp
}

func post(t *testing.T, ts *httptest.Server, path, body string) *http.Response {
	t.Helper()
	resp, err := http.Post(ts.URL+path, "application/json", strings.NewReader(body))
	if err != nil {
		t.Fatalf("POST %s: %v", path, err)
	}
	return resp
}

func readBody(t *testing.T, resp *http.Response) []byte {
	t.Helper()
	defer resp.Body.Close()
	b, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read body: %v", err)
	}
	return b
}

// extractID extracts the "id" field from a {"data": {"id": "..."}} response.
func extractID(t *testing.T, body []byte) string {
	t.Helper()
	var env struct {
		Data struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &env); err != nil {
		t.Fatalf("unmarshal envelope: %v (body: %s)", err, string(body))
	}
	if env.Data.ID == "" {
		t.Fatalf("expected non-empty id in response: %s", string(body))
	}
	return env.Data.ID
}

// ─────────────────────────────────────────
// Integration Test
// ─────────────────────────────────────────

func TestStealthIntegration(t *testing.T) {
	// 1. Create store
	store := testStore(t)

	// 2. Set up API server with httptest
	broadcaster := api.NewBroadcaster()
	mux := http.NewServeMux()
	api.RegisterRoutes(mux, api.Dependencies{
		ProjectsStore:         store,
		ReleasesStore:         store,
		SubscriptionsStore:    store,
		SourcesStore:          store,
		ChannelsStore:         store,
		ContextSourcesStore:   ContextSourcesStub{},
		SemanticReleasesStore: SemanticReleasesStub{},
		AgentStore:            AgentStub{},
		TodosStore:            TodosStub{},
		OnboardStore:          OnboardStub{},
		GatesStore:            GatesStub{},
		KeyStore:              store,
		SessionValidator:      SessionValidatorStub{},
		HealthChecker:         store,
		Broadcaster:           broadcaster,
		NoAuth:                true,
		IngestionService:      ingestion.NewService(store),
		HTTPClient:            http.DefaultClient,
	})
	ts := httptest.NewServer(mux)
	defer ts.Close()

	// 3. Create a project via the API
	resp := post(t, ts, "/api/v1/projects", `{"name":"Integration Project","description":"test project"}`)
	if resp.StatusCode != http.StatusCreated {
		body := readBody(t, resp)
		t.Fatalf("create project: expected 201, got %d: %s", resp.StatusCode, body)
	}
	body := readBody(t, resp)
	projectID := extractID(t, body)
	t.Logf("created project: %s", projectID)

	// 4. Create a source via the API
	sourceBody := fmt.Sprintf(`{
		"provider": "github",
		"repository": "test-org/test-repo",
		"poll_interval_seconds": 3600,
		"enabled": true
	}`)
	resp = post(t, ts, "/api/v1/projects/"+projectID+"/sources", sourceBody)
	if resp.StatusCode != http.StatusCreated {
		body := readBody(t, resp)
		t.Fatalf("create source: expected 201, got %d: %s", resp.StatusCode, body)
	}
	body = readBody(t, resp)
	sourceID := extractID(t, body)
	t.Logf("created source: %s", sourceID)

	// 5. Create a shell channel via the API
	channelBody := `{
		"name": "Test Shell Channel",
		"type": "shell",
		"config": {"description": "integration test channel"}
	}`
	resp = post(t, ts, "/api/v1/channels", channelBody)
	if resp.StatusCode != http.StatusCreated {
		body := readBody(t, resp)
		t.Fatalf("create channel: expected 201, got %d: %s", resp.StatusCode, body)
	}
	body = readBody(t, resp)
	channelID := extractID(t, body)
	t.Logf("created channel: %s", channelID)

	// 6. Create a subscription with command config via the API
	subBody := fmt.Sprintf(`{
		"channel_id": %q,
		"type": "source_release",
		"source_id": %q,
		"config": {"command": "echo release ${VERSION}"}
	}`, channelID, sourceID)
	resp = post(t, ts, "/api/v1/subscriptions", subBody)
	if resp.StatusCode != http.StatusCreated {
		body := readBody(t, resp)
		t.Fatalf("create subscription: expected 201, got %d: %s", resp.StatusCode, body)
	}
	body = readBody(t, resp)
	subID := extractID(t, body)
	t.Logf("created subscription: %s", subID)

	// 7. Verify the store has the data
	ctx := context.Background()

	proj, err := store.GetProject(ctx, projectID)
	if err != nil {
		t.Fatalf("GetProject: %v", err)
	}
	if proj.Name != "Integration Project" {
		t.Errorf("project name: got %q, want %q", proj.Name, "Integration Project")
	}

	src, err := store.GetSource(ctx, sourceID)
	if err != nil {
		t.Fatalf("GetSource: %v", err)
	}
	if src.Provider != "github" || src.Repository != "test-org/test-repo" {
		t.Errorf("source: got provider=%q repo=%q, want github/test-org/test-repo", src.Provider, src.Repository)
	}

	ch, err := store.GetChannel(ctx, channelID)
	if err != nil {
		t.Fatalf("GetChannel: %v", err)
	}
	if ch.Type != "shell" {
		t.Errorf("channel type: got %q, want %q", ch.Type, "shell")
	}

	sub, err := store.GetSubscription(ctx, subID)
	if err != nil {
		t.Fatalf("GetSubscription: %v", err)
	}
	if sub.ChannelID != channelID {
		t.Errorf("subscription channel_id: got %q, want %q", sub.ChannelID, channelID)
	}
	if sub.SourceID == nil || *sub.SourceID != sourceID {
		t.Errorf("subscription source_id: got %v, want %q", sub.SourceID, sourceID)
	}

	// 8. Wire up the NotifyHook and ingest a release
	senders := routing.NewSenders()
	store.NotifyHook = func(ctx context.Context, releaseID, srcID string) {
		store.NotifyRelease(ctx, releaseID, srcID, senders)
	}

	result := &ingestion.IngestionResult{
		Repository: "test-org/test-repo",
		RawVersion: "v1.0.0",
		Timestamp:  time.Now(),
		Metadata:   map[string]string{"tag": "v1.0.0"},
	}
	if err := store.IngestRelease(ctx, sourceID, result); err != nil {
		t.Fatalf("IngestRelease: %v", err)
	}

	// 9. Wait briefly for async shell execution (shell sender runs command asynchronously)
	time.Sleep(500 * time.Millisecond)

	// 10. Verify the release via the API
	resp = get(t, ts, "/api/v1/sources/"+sourceID+"/releases")
	if resp.StatusCode != http.StatusOK {
		body := readBody(t, resp)
		t.Fatalf("list releases: expected 200, got %d: %s", resp.StatusCode, body)
	}
	body = readBody(t, resp)

	var listEnv struct {
		Data []struct {
			ID         string `json:"id"`
			Version    string `json:"version"`
			SourceID   string `json:"source_id"`
			Provider   string `json:"provider"`
			Repository string `json:"repository"`
		} `json:"data"`
		Meta struct {
			Total int `json:"total"`
		} `json:"meta"`
	}
	if err := json.Unmarshal(body, &listEnv); err != nil {
		t.Fatalf("unmarshal releases list: %v (body: %s)", err, string(body))
	}
	if listEnv.Meta.Total != 1 {
		t.Fatalf("expected 1 release, got total=%d", listEnv.Meta.Total)
	}
	if len(listEnv.Data) != 1 {
		t.Fatalf("expected 1 release in data, got %d", len(listEnv.Data))
	}
	rel := listEnv.Data[0]
	if rel.Version != "v1.0.0" {
		t.Errorf("release version: got %q, want %q", rel.Version, "v1.0.0")
	}
	if rel.SourceID != sourceID {
		t.Errorf("release source_id: got %q, want %q", rel.SourceID, sourceID)
	}
	if rel.Provider != "github" {
		t.Errorf("release provider: got %q, want %q", rel.Provider, "github")
	}
	if rel.Repository != "test-org/test-repo" {
		t.Errorf("release repository: got %q, want %q", rel.Repository, "test-org/test-repo")
	}

	t.Log("integration test passed: full stealth flow verified")
}
