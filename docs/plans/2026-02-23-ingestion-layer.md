# Ingestion Layer Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Build the complete ingestion layer — polling Docker Hub and handling GitHub webhooks — that normalizes releases into a standard IR and persists them via the transactional outbox pattern (release INSERT + River job enqueue in one transaction).

**Architecture:** Polling sources implement `IIngestionSource` and are driven by an `Orchestrator` on configurable intervals. GitHub webhooks are handled by a dedicated HTTP handler. Both paths feed into an `IngestionService` that delegates to a `ReleaseStore` interface, whose PostgreSQL implementation performs the transactional outbox. Dependency injection via interfaces makes everything unit-testable without a real database.

**Tech Stack:** Go 1.22, PostgreSQL (pgx/v5), River job queue, net/http, crypto/hmac

---

## Task 1: Domain Models (ReleaseEvent IR)

**Files:**
- Create: `internal/models/release.go`
- Test: `internal/models/release_test.go`

**Step 1: Write the failing test**

```go
// internal/models/release_test.go
package models

import (
	"encoding/json"
	"testing"
	"time"
)

func TestReleaseEventJSON(t *testing.T) {
	event := ReleaseEvent{
		ID:         "550e8400-e29b-41d4-a716-446655440000",
		Source:     "dockerhub",
		Repository: "library/golang",
		RawVersion: "1.21.0",
		Changelog:  "Bug fixes and improvements",
		Metadata:   map[string]string{"digest": "sha256:abc123"},
		Timestamp:  time.Date(2024, 1, 15, 10, 0, 0, 0, time.UTC),
	}

	data, err := json.Marshal(event)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var got ReleaseEvent
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if got.ID != event.ID {
		t.Errorf("ID = %q, want %q", got.ID, event.ID)
	}
	if got.Source != event.Source {
		t.Errorf("Source = %q, want %q", got.Source, event.Source)
	}
	if got.Repository != event.Repository {
		t.Errorf("Repository = %q, want %q", got.Repository, event.Repository)
	}
	if got.RawVersion != event.RawVersion {
		t.Errorf("RawVersion = %q, want %q", got.RawVersion, event.RawVersion)
	}
}

func TestSemanticDataString(t *testing.T) {
	tests := []struct {
		name string
		data SemanticData
		want string
	}{
		{"stable", SemanticData{Major: 1, Minor: 21, Patch: 0}, "1.21.0"},
		{"prerelease", SemanticData{Major: 2, Minor: 0, Patch: 0, PreRelease: "rc.1"}, "2.0.0-rc.1"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.data.String(); got != tt.want {
				t.Errorf("String() = %q, want %q", got, tt.want)
			}
		})
	}
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/models/ -v`
Expected: FAIL — types not defined

**Step 3: Write minimal implementation**

```go
// internal/models/release.go
package models

import (
	"fmt"
	"time"
)

// SemanticData holds parsed semantic version components.
// Populated by the pipeline's Regex Normalizer node, not by ingestion.
type SemanticData struct {
	Major      int    `json:"major"`
	Minor      int    `json:"minor"`
	Patch      int    `json:"patch"`
	PreRelease string `json:"pre_release,omitempty"`
}

func (s SemanticData) String() string {
	v := fmt.Sprintf("%d.%d.%d", s.Major, s.Minor, s.Patch)
	if s.PreRelease != "" {
		v += "-" + s.PreRelease
	}
	return v
}

// ReleaseEvent is the Intermediate Representation (IR) for a detected release.
// See DESIGN.md Section 2.1 for the canonical definition.
type ReleaseEvent struct {
	ID              string            `json:"id"`
	Source          string            `json:"source"`
	Repository      string            `json:"repository"`
	RawVersion      string            `json:"raw_version"`
	SemanticVersion SemanticData      `json:"semantic_version"`
	Changelog       string            `json:"changelog"`
	IsPreRelease    bool              `json:"is_pre_release"`
	Metadata        map[string]string `json:"metadata"`
	Timestamp       time.Time         `json:"timestamp"`
}
```

**Step 4: Run test to verify it passes**

Run: `go test ./internal/models/ -v`
Expected: PASS

**Step 5: Commit**

```bash
git add internal/models/release.go internal/models/release_test.go
git commit -m "feat: add ReleaseEvent and SemanticData domain models"
```

---

## Task 2: Add Go Dependencies

**Files:**
- Modify: `go.mod` (via `go get`)

**Step 1: Install dependencies**

```bash
go get github.com/jackc/pgx/v5
go get github.com/riverqueue/river
go get github.com/riverqueue/river/riverdriver/riverpgxv5
go get github.com/google/uuid
```

**Step 2: Tidy modules**

```bash
go mod tidy
```

**Step 3: Verify build**

Run: `go build ./...`
Expected: Success

**Step 4: Commit**

```bash
git add go.mod go.sum
git commit -m "chore: add pgx, river, and uuid dependencies"
```

---

## Task 3: Database Connection & Schema Migrations

**Files:**
- Create: `internal/db/db.go`
- Create: `internal/db/migrations.go`

The `internal/db/` package is a natural addition not explicitly in the planned structure but needed since multiple packages (ingestion, queue, server) share the connection pool.

**Step 1: Create database connection helper**

```go
// internal/db/db.go
package db

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
)

// NewPool creates a PostgreSQL connection pool and verifies connectivity.
func NewPool(ctx context.Context, databaseURL string) (*pgxpool.Pool, error) {
	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		return nil, fmt.Errorf("create pool: %w", err)
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("ping: %w", err)
	}
	return pool, nil
}
```

**Step 2: Create migration runner**

Schema from DESIGN.md Section 3. Uses `CREATE TABLE IF NOT EXISTS` for idempotent execution — no migration framework needed yet (YAGNI).

```go
// internal/db/migrations.go
package db

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
)

const schema = `
CREATE TABLE IF NOT EXISTS releases (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    source VARCHAR(50) NOT NULL,
    repository VARCHAR(255) NOT NULL,
    version VARCHAR(100) NOT NULL,
    payload JSONB NOT NULL,
    created_at TIMESTAMPTZ DEFAULT NOW(),
    UNIQUE(repository, version)
);

CREATE TABLE IF NOT EXISTS subscriptions (
    id SERIAL PRIMARY KEY,
    repository VARCHAR(255) NOT NULL,
    channel_type VARCHAR(50) NOT NULL,
    notification_target VARCHAR(255) NOT NULL
);
`

// RunMigrations applies the database schema. Idempotent — safe to call on every startup.
func RunMigrations(ctx context.Context, pool *pgxpool.Pool) error {
	_, err := pool.Exec(ctx, schema)
	if err != nil {
		return fmt.Errorf("run migrations: %w", err)
	}
	return nil
}
```

**Step 3: Verify build**

Run: `go build ./internal/db/`
Expected: Success

**Step 4: Commit**

```bash
git add internal/db/db.go internal/db/migrations.go
git commit -m "feat: add database connection pool and schema migrations"
```

---

## Task 4: River Queue Job Definition

**Files:**
- Create: `internal/queue/jobs.go`
- Create: `internal/queue/jobs_test.go`
- Create: `internal/queue/client.go`

**Step 1: Write the failing test**

```go
// internal/queue/jobs_test.go
package queue

import "testing"

func TestPipelineJobArgsKind(t *testing.T) {
	args := PipelineJobArgs{ReleaseID: "test-id"}
	if got := args.Kind(); got != "pipeline_process" {
		t.Errorf("Kind() = %q, want %q", got, "pipeline_process")
	}
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/queue/ -v`
Expected: FAIL — PipelineJobArgs not defined

**Step 3: Write job definition**

```go
// internal/queue/jobs.go
package queue

import "github.com/riverqueue/river"

// PipelineJobArgs defines the payload for a DAG pipeline processing job.
type PipelineJobArgs struct {
	ReleaseID string `json:"release_id"`
}

func (PipelineJobArgs) Kind() string { return "pipeline_process" }

// Compile-time check that PipelineJobArgs satisfies river.JobArgs.
var _ river.JobArgs = PipelineJobArgs{}
```

**Step 4: Run test to verify it passes**

Run: `go test ./internal/queue/ -v`
Expected: PASS

**Step 5: Create River client factory**

```go
// internal/queue/client.go
package queue

import (
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/riverqueue/river"
	"github.com/riverqueue/river/riverdriver/riverpgxv5"
)

// NewRiverClient creates a River client backed by the given pgx pool.
// Pass nil workers to create an insert-only client (no job processing).
func NewRiverClient(pool *pgxpool.Pool, workers *river.Workers) (*river.Client[pgx.Tx], error) {
	config := &river.Config{
		Queues: map[string]river.QueueConfig{
			river.QueueDefault: {MaxWorkers: 100},
		},
	}
	if workers != nil {
		config.Workers = workers
	}
	return river.NewClient(riverpgxv5.New(pool), config)
}
```

**Step 6: Verify build**

Run: `go build ./internal/queue/`
Expected: Success

**Step 7: Commit**

```bash
git add internal/queue/jobs.go internal/queue/jobs_test.go internal/queue/client.go
git commit -m "feat: add River queue job definition and client factory"
```

---

## Task 5: Ingestion Interface & Source Contract

**Files:**
- Create: `internal/ingestion/source.go`

**Step 1: Define the interface and types**

```go
// internal/ingestion/source.go
package ingestion

import (
	"context"
	"time"
)

// IngestionResult is raw release data returned by an ingestion source.
type IngestionResult struct {
	Repository string
	RawVersion string
	Changelog  string
	Metadata   map[string]string
	Timestamp  time.Time
}

// IIngestionSource abstracts a polling-based release data provider.
// Each implementation fetches the latest releases from a specific registry.
type IIngestionSource interface {
	// Name returns the source identifier (e.g., "dockerhub", "github").
	Name() string
	// FetchNewReleases polls the upstream registry and returns discovered releases.
	FetchNewReleases(ctx context.Context) ([]IngestionResult, error)
}
```

**Step 2: Verify build**

Run: `go build ./internal/ingestion/`
Expected: Success

**Step 3: Commit**

```bash
git add internal/ingestion/source.go
git commit -m "feat: add IIngestionSource interface and IngestionResult type"
```

---

## Task 6: Ingestion Service (Transactional Outbox Orchestration)

**Files:**
- Create: `internal/ingestion/store.go`
- Create: `internal/ingestion/service.go`
- Test: `internal/ingestion/service_test.go`

**Step 1: Define the ReleaseStore interface**

```go
// internal/ingestion/store.go
package ingestion

import (
	"context"

	"github.com/sentioxyz/releaseguard/internal/models"
)

// ReleaseStore persists release events using the transactional outbox pattern.
// The implementation inserts the release and enqueues a pipeline job atomically.
type ReleaseStore interface {
	IngestRelease(ctx context.Context, event *models.ReleaseEvent) error
}
```

**Step 2: Write the failing test**

```go
// internal/ingestion/service_test.go
package ingestion

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/sentioxyz/releaseguard/internal/models"
)

type mockStore struct {
	ingested []*models.ReleaseEvent
	err      error
}

func (m *mockStore) IngestRelease(_ context.Context, event *models.ReleaseEvent) error {
	if m.err != nil {
		return m.err
	}
	m.ingested = append(m.ingested, event)
	return nil
}

func TestServiceProcessResults(t *testing.T) {
	store := &mockStore{}
	svc := NewService(store)

	results := []IngestionResult{
		{
			Repository: "library/golang",
			RawVersion: "1.21.0",
			Changelog:  "Bug fixes",
			Timestamp:  time.Date(2024, 1, 15, 0, 0, 0, 0, time.UTC),
		},
	}

	err := svc.ProcessResults(context.Background(), "dockerhub", results)
	if err != nil {
		t.Fatalf("ProcessResults: %v", err)
	}

	if len(store.ingested) != 1 {
		t.Fatalf("ingested %d events, want 1", len(store.ingested))
	}

	event := store.ingested[0]
	if event.Source != "dockerhub" {
		t.Errorf("Source = %q, want %q", event.Source, "dockerhub")
	}
	if event.Repository != "library/golang" {
		t.Errorf("Repository = %q, want %q", event.Repository, "library/golang")
	}
	if event.RawVersion != "1.21.0" {
		t.Errorf("RawVersion = %q, want %q", event.RawVersion, "1.21.0")
	}
	if event.ID == "" {
		t.Error("ID should not be empty")
	}
}

func TestServiceProcessResultsDuplicateSkipped(t *testing.T) {
	store := &mockStore{err: errors.New("unique_violation")}
	svc := NewService(store)

	results := []IngestionResult{
		{Repository: "lib/go", RawVersion: "1.0.0", Timestamp: time.Now()},
	}

	// Duplicates should not cause a top-level error — they're expected.
	err := svc.ProcessResults(context.Background(), "dockerhub", results)
	if err != nil {
		t.Fatalf("ProcessResults should not fail on duplicates: %v", err)
	}
}
```

**Step 3: Run test to verify it fails**

Run: `go test ./internal/ingestion/ -v -run TestService`
Expected: FAIL — `NewService` not defined

**Step 4: Implement the service**

```go
// internal/ingestion/service.go
package ingestion

import (
	"context"
	"log/slog"

	"github.com/google/uuid"
	"github.com/sentioxyz/releaseguard/internal/models"
)

// Service orchestrates ingestion sources and persists normalized results.
type Service struct {
	store ReleaseStore
}

func NewService(store ReleaseStore) *Service {
	return &Service{store: store}
}

// ProcessResults normalizes raw ingestion results into ReleaseEvents and persists them.
// Duplicate releases (unique constraint violations) are logged and skipped, not fatal.
func (s *Service) ProcessResults(ctx context.Context, sourceName string, results []IngestionResult) error {
	for _, r := range results {
		event := &models.ReleaseEvent{
			ID:         uuid.New().String(),
			Source:     sourceName,
			Repository: r.Repository,
			RawVersion: r.RawVersion,
			Changelog:  r.Changelog,
			Metadata:   r.Metadata,
			Timestamp:  r.Timestamp,
		}

		if err := s.store.IngestRelease(ctx, event); err != nil {
			slog.Warn("ingest failed (may be duplicate)",
				"repo", r.Repository, "version", r.RawVersion, "err", err)
			continue
		}
	}
	return nil
}
```

**Step 5: Run test to verify it passes**

Run: `go test ./internal/ingestion/ -v -run TestService`
Expected: PASS

**Step 6: Commit**

```bash
git add internal/ingestion/store.go internal/ingestion/service.go internal/ingestion/service_test.go
git commit -m "feat: add ingestion service with transactional outbox orchestration"
```

---

## Task 7: PostgreSQL Release Store (Transactional Outbox Implementation)

**Files:**
- Create: `internal/ingestion/pgstore.go`

This is the real `ReleaseStore` implementation. It performs the transactional outbox: BEGIN → INSERT release → River InsertTx job → COMMIT. No unit test here — it's a thin infrastructure adapter best validated by integration tests. The interface mock in Task 6 covers the service logic.

**Step 1: Implement PgStore**

```go
// internal/ingestion/pgstore.go
package ingestion

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/riverqueue/river"
	"github.com/sentioxyz/releaseguard/internal/models"
	"github.com/sentioxyz/releaseguard/internal/queue"
)

// PgStore implements ReleaseStore using PostgreSQL + River for the transactional outbox.
type PgStore struct {
	pool  *pgxpool.Pool
	river *river.Client[pgx.Tx]
}

func NewPgStore(pool *pgxpool.Pool, riverClient *river.Client[pgx.Tx]) *PgStore {
	return &PgStore{pool: pool, river: riverClient}
}

// IngestRelease inserts a release and enqueues a pipeline job in a single transaction.
// Returns an error on unique constraint violation (caller treats as idempotent skip).
func (s *PgStore) IngestRelease(ctx context.Context, event *models.ReleaseEvent) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback(ctx)

	payload, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("marshal payload: %w", err)
	}

	_, err = tx.Exec(ctx,
		`INSERT INTO releases (id, source, repository, version, payload) VALUES ($1, $2, $3, $4, $5)`,
		event.ID, event.Source, event.Repository, event.RawVersion, payload,
	)
	if err != nil {
		return fmt.Errorf("insert release: %w", err)
	}

	_, err = s.river.InsertTx(ctx, tx, queue.PipelineJobArgs{ReleaseID: event.ID}, nil)
	if err != nil {
		return fmt.Errorf("enqueue job: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit: %w", err)
	}
	return nil
}
```

**Step 2: Verify build**

Run: `go build ./internal/ingestion/`
Expected: Success

**Step 3: Commit**

```bash
git add internal/ingestion/pgstore.go
git commit -m "feat: add PostgreSQL release store with transactional outbox"
```

---

## Task 8: Docker Hub Poller

**Files:**
- Create: `internal/ingestion/dockerhub.go`
- Test: `internal/ingestion/dockerhub_test.go`

**Step 1: Write the failing test**

Uses `httptest.NewServer` to mock the Docker Hub API. The `baseURL` field is exported for test injection.

```go
// internal/ingestion/dockerhub_test.go
package ingestion

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestDockerHubSourceName(t *testing.T) {
	src := NewDockerHubSource(http.DefaultClient, "library/golang")
	if got := src.Name(); got != "dockerhub" {
		t.Errorf("Name() = %q, want %q", got, "dockerhub")
	}
}

func TestDockerHubFetchNewReleases(t *testing.T) {
	response := `{
		"results": [
			{"name": "1.21.0", "last_updated": "2024-01-15T10:00:00.000000Z"},
			{"name": "1.21.0-rc.1", "last_updated": "2024-01-10T10:00:00.000000Z"}
		]
	}`

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(response))
	}))
	defer srv.Close()

	src := NewDockerHubSource(srv.Client(), "library/golang")
	src.baseURL = srv.URL

	results, err := src.FetchNewReleases(context.Background())
	if err != nil {
		t.Fatalf("FetchNewReleases: %v", err)
	}

	if len(results) != 2 {
		t.Fatalf("got %d results, want 2", len(results))
	}
	if results[0].RawVersion != "1.21.0" {
		t.Errorf("results[0].RawVersion = %q, want %q", results[0].RawVersion, "1.21.0")
	}
	if results[0].Repository != "library/golang" {
		t.Errorf("results[0].Repository = %q, want %q", results[0].Repository, "library/golang")
	}
}

func TestDockerHubFetchEmpty(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"results": []}`))
	}))
	defer srv.Close()

	src := NewDockerHubSource(srv.Client(), "library/golang")
	src.baseURL = srv.URL

	results, err := src.FetchNewReleases(context.Background())
	if err != nil {
		t.Fatalf("FetchNewReleases: %v", err)
	}
	if len(results) != 0 {
		t.Fatalf("got %d results, want 0", len(results))
	}
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/ingestion/ -v -run TestDockerHub`
Expected: FAIL — `NewDockerHubSource` not defined

**Step 3: Implement Docker Hub source**

Polls `GET /v2/repositories/{repo}/tags/?page_size=25&ordering=last_updated`. Relies on the DB unique constraint for deduplication — no cursor tracking needed.

```go
// internal/ingestion/dockerhub.go
package ingestion

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

const defaultDockerHubURL = "https://hub.docker.com"

// DockerHubSource polls Docker Hub for new image tags.
type DockerHubSource struct {
	client     *http.Client
	repository string
	baseURL    string
}

func NewDockerHubSource(client *http.Client, repository string) *DockerHubSource {
	return &DockerHubSource{
		client:     client,
		repository: repository,
		baseURL:    defaultDockerHubURL,
	}
}

func (s *DockerHubSource) Name() string { return "dockerhub" }

func (s *DockerHubSource) FetchNewReleases(ctx context.Context) ([]IngestionResult, error) {
	url := fmt.Sprintf("%s/v2/repositories/%s/tags/?page_size=25&ordering=last_updated",
		s.baseURL, s.repository)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetch tags: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status: %d", resp.StatusCode)
	}

	var body struct {
		Results []struct {
			Name        string `json:"name"`
			LastUpdated string `json:"last_updated"`
		} `json:"results"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	results := make([]IngestionResult, 0, len(body.Results))
	for _, tag := range body.Results {
		ts, _ := time.Parse(time.RFC3339Nano, tag.LastUpdated)
		results = append(results, IngestionResult{
			Repository: s.repository,
			RawVersion: tag.Name,
			Timestamp:  ts,
		})
	}
	return results, nil
}
```

**Step 4: Run test to verify it passes**

Run: `go test ./internal/ingestion/ -v -run TestDockerHub`
Expected: PASS

**Step 5: Commit**

```bash
git add internal/ingestion/dockerhub.go internal/ingestion/dockerhub_test.go
git commit -m "feat: add Docker Hub ingestion source"
```

---

## Task 9: GitHub Webhook Handler

**Files:**
- Create: `internal/ingestion/github.go`
- Test: `internal/ingestion/github_test.go`

The webhook handler is an `http.Handler`, not an `IIngestionSource` (push-based, not poll-based). It validates `X-Hub-Signature-256`, parses the release payload, and calls a callback with results.

**Step 1: Write the failing test**

```go
// internal/ingestion/github_test.go
package ingestion

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"testing"
)

func signPayload(payload []byte, secret string) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(payload)
	return hex.EncodeToString(mac.Sum(nil))
}

func TestGitHubWebhookHandler(t *testing.T) {
	var received []IngestionResult
	handler := NewGitHubWebhookHandler("test-secret", func(results []IngestionResult) {
		received = append(received, results...)
	})

	payload := []byte(`{
		"action": "published",
		"release": {
			"tag_name": "v1.5.0",
			"body": "## Changes\n* Fix login bug",
			"prerelease": false,
			"published_at": "2024-01-15T10:00:00Z"
		},
		"repository": {
			"full_name": "org/myapp"
		}
	}`)

	sig := signPayload(payload, "test-secret")
	req := httptest.NewRequest(http.MethodPost, "/webhook/github", bytes.NewReader(payload))
	req.Header.Set("X-Hub-Signature-256", "sha256="+sig)
	req.Header.Set("X-GitHub-Event", "release")

	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body = %s", w.Code, http.StatusOK, w.Body.String())
	}
	if len(received) != 1 {
		t.Fatalf("received %d results, want 1", len(received))
	}
	if received[0].Repository != "org/myapp" {
		t.Errorf("Repository = %q, want %q", received[0].Repository, "org/myapp")
	}
	if received[0].RawVersion != "v1.5.0" {
		t.Errorf("RawVersion = %q, want %q", received[0].RawVersion, "v1.5.0")
	}
	if received[0].Changelog != "## Changes\n* Fix login bug" {
		t.Errorf("Changelog = %q", received[0].Changelog)
	}
}

func TestGitHubWebhookInvalidSignature(t *testing.T) {
	handler := NewGitHubWebhookHandler("real-secret", func(results []IngestionResult) {
		t.Fatal("callback should not be called")
	})

	payload := []byte(`{"action": "published"}`)
	req := httptest.NewRequest(http.MethodPost, "/webhook/github", bytes.NewReader(payload))
	req.Header.Set("X-Hub-Signature-256", "sha256=invalidsignature")
	req.Header.Set("X-GitHub-Event", "release")

	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Errorf("status = %d, want %d", w.Code, http.StatusForbidden)
	}
}

func TestGitHubWebhookIgnoresNonReleaseEvents(t *testing.T) {
	handler := NewGitHubWebhookHandler("secret", func(results []IngestionResult) {
		t.Fatal("callback should not be called")
	})

	body := []byte(`{}`)
	req := httptest.NewRequest(http.MethodPost, "/webhook/github", bytes.NewReader(body))
	req.Header.Set("X-GitHub-Event", "push")
	req.Header.Set("X-Hub-Signature-256", "sha256="+signPayload(body, "secret"))

	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", w.Code, http.StatusOK)
	}
}

func TestGitHubWebhookIgnoresNonPublishedActions(t *testing.T) {
	handler := NewGitHubWebhookHandler("secret", func(results []IngestionResult) {
		t.Fatal("callback should not be called for non-published actions")
	})

	payload := []byte(`{"action": "created", "release": {"tag_name": "v1.0.0"}, "repository": {"full_name": "a/b"}}`)
	req := httptest.NewRequest(http.MethodPost, "/webhook/github", bytes.NewReader(payload))
	req.Header.Set("X-GitHub-Event", "release")
	req.Header.Set("X-Hub-Signature-256", "sha256="+signPayload(payload, "secret"))

	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", w.Code, http.StatusOK)
	}
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/ingestion/ -v -run TestGitHub`
Expected: FAIL — `NewGitHubWebhookHandler` not defined

**Step 3: Implement GitHub webhook handler**

```go
// internal/ingestion/github.go
package ingestion

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"time"
)

// GitHubWebhookHandler handles incoming GitHub release webhook events.
// It validates the HMAC signature, parses the release payload, and
// forwards results via the onResult callback.
type GitHubWebhookHandler struct {
	secret   string
	onResult func([]IngestionResult)
}

func NewGitHubWebhookHandler(secret string, onResult func([]IngestionResult)) *GitHubWebhookHandler {
	return &GitHubWebhookHandler{secret: secret, onResult: onResult}
}

func (h *GitHubWebhookHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "read body", http.StatusBadRequest)
		return
	}

	if !h.verifySignature(body, r.Header.Get("X-Hub-Signature-256")) {
		http.Error(w, "invalid signature", http.StatusForbidden)
		return
	}

	if r.Header.Get("X-GitHub-Event") != "release" {
		w.WriteHeader(http.StatusOK)
		return
	}

	var payload struct {
		Action  string `json:"action"`
		Release struct {
			TagName     string `json:"tag_name"`
			Body        string `json:"body"`
			PreRelease  bool   `json:"prerelease"`
			PublishedAt string `json:"published_at"`
		} `json:"release"`
		Repository struct {
			FullName string `json:"full_name"`
		} `json:"repository"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		http.Error(w, "invalid json", http.StatusBadRequest)
		return
	}

	if payload.Action != "published" {
		w.WriteHeader(http.StatusOK)
		return
	}

	ts, _ := time.Parse(time.RFC3339, payload.Release.PublishedAt)
	result := IngestionResult{
		Repository: payload.Repository.FullName,
		RawVersion: payload.Release.TagName,
		Changelog:  payload.Release.Body,
		Timestamp:  ts,
	}

	h.onResult([]IngestionResult{result})
	w.WriteHeader(http.StatusOK)
}

func (h *GitHubWebhookHandler) verifySignature(body []byte, header string) bool {
	sig := strings.TrimPrefix(header, "sha256=")
	mac := hmac.New(sha256.New, []byte(h.secret))
	mac.Write(body)
	expected := hex.EncodeToString(mac.Sum(nil))
	return hmac.Equal([]byte(sig), []byte(expected))
}
```

**Step 4: Run test to verify it passes**

Run: `go test ./internal/ingestion/ -v -run TestGitHub`
Expected: PASS

**Step 5: Commit**

```bash
git add internal/ingestion/github.go internal/ingestion/github_test.go
git commit -m "feat: add GitHub release webhook handler with HMAC verification"
```

---

## Task 10: Polling Orchestrator

**Files:**
- Create: `internal/ingestion/orchestrator.go`
- Test: `internal/ingestion/orchestrator_test.go`

**Step 1: Write the failing test**

```go
// internal/ingestion/orchestrator_test.go
package ingestion

import (
	"context"
	"sync/atomic"
	"testing"
	"time"
)

type fakeSource struct {
	name    string
	calls   atomic.Int32
	results []IngestionResult
}

func (f *fakeSource) Name() string { return f.name }

func (f *fakeSource) FetchNewReleases(_ context.Context) ([]IngestionResult, error) {
	f.calls.Add(1)
	return f.results, nil
}

func TestOrchestratorPollsOnInterval(t *testing.T) {
	store := &mockStore{}
	svc := NewService(store)

	src := &fakeSource{
		name:    "test",
		results: []IngestionResult{{Repository: "r", RawVersion: "v1", Timestamp: time.Now()}},
	}

	orch := NewOrchestrator(svc, []IIngestionSource{src}, 50*time.Millisecond)

	ctx, cancel := context.WithTimeout(context.Background(), 180*time.Millisecond)
	defer cancel()

	orch.Run(ctx)

	// With 50ms interval and 180ms timeout, expect: immediate poll + ~3 ticks = ~4 calls.
	// At minimum 2 calls proves the interval loop works.
	calls := int(src.calls.Load())
	if calls < 2 {
		t.Errorf("expected at least 2 poll calls, got %d", calls)
	}
}

func TestOrchestratorMultipleSources(t *testing.T) {
	store := &mockStore{}
	svc := NewService(store)

	src1 := &fakeSource{name: "a", results: []IngestionResult{{Repository: "r1", RawVersion: "v1", Timestamp: time.Now()}}}
	src2 := &fakeSource{name: "b", results: []IngestionResult{{Repository: "r2", RawVersion: "v2", Timestamp: time.Now()}}}

	orch := NewOrchestrator(svc, []IIngestionSource{src1, src2}, 50*time.Millisecond)

	ctx, cancel := context.WithTimeout(context.Background(), 80*time.Millisecond)
	defer cancel()

	orch.Run(ctx)

	if src1.calls.Load() < 1 {
		t.Error("source 1 was not polled")
	}
	if src2.calls.Load() < 1 {
		t.Error("source 2 was not polled")
	}
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/ingestion/ -v -run TestOrchestrator`
Expected: FAIL — `NewOrchestrator` not defined

**Step 3: Implement orchestrator**

```go
// internal/ingestion/orchestrator.go
package ingestion

import (
	"context"
	"log/slog"
	"time"
)

// Orchestrator runs polling ingestion sources on a fixed interval.
type Orchestrator struct {
	service  *Service
	sources  []IIngestionSource
	interval time.Duration
}

func NewOrchestrator(service *Service, sources []IIngestionSource, interval time.Duration) *Orchestrator {
	return &Orchestrator{service: service, sources: sources, interval: interval}
}

// Run polls all sources immediately, then on every tick. Blocks until ctx is cancelled.
func (o *Orchestrator) Run(ctx context.Context) {
	ticker := time.NewTicker(o.interval)
	defer ticker.Stop()

	o.pollAll(ctx)

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			o.pollAll(ctx)
		}
	}
}

func (o *Orchestrator) pollAll(ctx context.Context) {
	for _, src := range o.sources {
		results, err := src.FetchNewReleases(ctx)
		if err != nil {
			slog.Error("poll failed", "source", src.Name(), "err", err)
			continue
		}
		if len(results) == 0 {
			continue
		}
		if err := o.service.ProcessResults(ctx, src.Name(), results); err != nil {
			slog.Error("process failed", "source", src.Name(), "err", err)
		}
	}
}
```

**Step 4: Run test to verify it passes**

Run: `go test ./internal/ingestion/ -v -run TestOrchestrator`
Expected: PASS

**Step 5: Commit**

```bash
git add internal/ingestion/orchestrator.go internal/ingestion/orchestrator_test.go
git commit -m "feat: add polling orchestrator for ingestion sources"
```

---

## Task 11: Wire Up main.go

**Files:**
- Modify: `cmd/server/main.go`

**Step 1: Connect all components**

```go
// cmd/server/main.go
package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"time"

	"github.com/sentioxyz/releaseguard/internal/db"
	"github.com/sentioxyz/releaseguard/internal/ingestion"
	"github.com/sentioxyz/releaseguard/internal/queue"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	dbURL := envOr("DATABASE_URL", "postgres://localhost:5432/releaseguard?sslmode=disable")
	ghSecret := envOr("GITHUB_WEBHOOK_SECRET", "")

	// Database
	pool, err := db.NewPool(ctx, dbURL)
	if err != nil {
		slog.Error("database connection failed", "err", err)
		os.Exit(1)
	}
	defer pool.Close()

	if err := db.RunMigrations(ctx, pool); err != nil {
		slog.Error("migrations failed", "err", err)
		os.Exit(1)
	}

	// River queue (insert-only — no workers registered yet)
	riverClient, err := queue.NewRiverClient(pool, nil)
	if err != nil {
		slog.Error("river client failed", "err", err)
		os.Exit(1)
	}

	// Ingestion layer
	store := ingestion.NewPgStore(pool, riverClient)
	svc := ingestion.NewService(store)

	sources := []ingestion.IIngestionSource{
		ingestion.NewDockerHubSource(http.DefaultClient, "library/golang"),
	}

	orch := ingestion.NewOrchestrator(svc, sources, 5*time.Minute)

	// GitHub webhook handler
	webhookHandler := ingestion.NewGitHubWebhookHandler(ghSecret, func(results []ingestion.IngestionResult) {
		if err := svc.ProcessResults(ctx, "github", results); err != nil {
			slog.Error("github webhook processing failed", "err", err)
		}
	})

	mux := http.NewServeMux()
	mux.Handle("POST /webhook/github", webhookHandler)

	srv := &http.Server{Addr: ":8080", Handler: mux}

	// Start polling in background
	go orch.Run(ctx)

	// Start HTTP server
	go func() {
		slog.Info("server starting", "addr", srv.Addr)
		if err := srv.ListenAndServe(); err != http.ErrServerClosed {
			slog.Error("server error", "err", err)
		}
	}()

	<-ctx.Done()
	slog.Info("shutting down")
	srv.Shutdown(context.Background())
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
```

**Step 2: Verify build**

Run: `go build ./cmd/server`
Expected: Success

**Step 3: Run all tests**

Run: `go test ./... -v`
Expected: All PASS

**Step 4: Run vet**

Run: `go vet ./...`
Expected: Clean

**Step 5: Commit**

```bash
git add cmd/server/main.go
git commit -m "feat: wire up ingestion layer in main server"
```
