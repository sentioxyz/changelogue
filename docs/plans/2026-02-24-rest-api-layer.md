# REST API Layer Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Build the complete REST API layer — CRUD endpoints for projects/sources/subscriptions/channels, read-only release endpoints, middleware stack, API key auth, SSE real-time events — all backed by PostgreSQL.

**Architecture:** Pure Go stdlib `net/http` with Go 1.22+ enhanced `ServeMux`. Each resource gets a handler struct with an injected store interface. A single `PgStore` struct implements all store interfaces. Middleware uses the standard `func(http.Handler) http.Handler` chain pattern. SSE events are backed by PostgreSQL `LISTEN/NOTIFY`.

**Tech Stack:** Go 1.25, PostgreSQL (pgx/v5), `golang.org/x/time/rate` for rate limiting, net/http stdlib

**Prerequisites:** Ingestion layer (complete), database + River queue (complete)

**Key Reference:** `docs/designs/2026-02-24-api-design.md` — the full API specification

---

## Task 1: Database Schema Migration

The current schema has only `releases` and `subscriptions` in a simplified form. The API requires the full DESIGN.md schema: `projects`, `sources`, `releases` (with `source_id` FK), `pipeline_jobs`, `notification_channels`, `subscriptions` (redesigned), and `api_keys`.

Since this is early development with no production data, we replace the old schema entirely.

**Files:**
- Modify: `internal/db/migrations.go`

**Step 1: Write the new migration SQL**

Replace the `schema` constant in `internal/db/migrations.go` with the full DESIGN.md schema. Use `DROP TABLE IF EXISTS ... CASCADE` for old tables, then `CREATE TABLE IF NOT EXISTS` for all new tables.

```go
// internal/db/migrations.go
package db

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/riverqueue/river/riverdriver/riverpgxv5"
	"github.com/riverqueue/river/rivermigrate"
)

const schema = `
-- Drop old-format tables (dev only — no production data exists)
DROP TABLE IF EXISTS subscriptions CASCADE;
DROP TABLE IF EXISTS releases CASCADE;

-- Tracked software projects (the central entity)
CREATE TABLE IF NOT EXISTS projects (
    id SERIAL PRIMARY KEY,
    name VARCHAR(100) NOT NULL UNIQUE,
    description TEXT,
    url VARCHAR(500),
    pipeline_config JSONB NOT NULL DEFAULT '{"changelog_summarizer": {}, "urgency_scorer": {}}'::jsonb,
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW()
);

-- Configured ingestion sources
CREATE TABLE IF NOT EXISTS sources (
    id SERIAL PRIMARY KEY,
    project_id INT NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    source_type VARCHAR(50) NOT NULL,
    repository VARCHAR(255) NOT NULL,
    poll_interval_seconds INT DEFAULT 300,
    enabled BOOLEAN DEFAULT true,
    exclude_version_regexp TEXT,
    exclude_prereleases BOOLEAN DEFAULT false,
    last_polled_at TIMESTAMPTZ,
    last_error TEXT,
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW(),
    UNIQUE(source_type, repository)
);

-- Normalized release events (references source, not raw strings)
CREATE TABLE IF NOT EXISTS releases (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    source_id INT NOT NULL REFERENCES sources(id),
    version VARCHAR(100) NOT NULL,
    payload JSONB NOT NULL,
    created_at TIMESTAMPTZ DEFAULT NOW(),
    UNIQUE(source_id, version)
);

-- Pipeline job tracking (application-level, separate from River internals)
CREATE TABLE IF NOT EXISTS pipeline_jobs (
    id BIGSERIAL PRIMARY KEY,
    state VARCHAR(50) DEFAULT 'available',
    release_id UUID REFERENCES releases(id),
    current_node VARCHAR(50),
    node_results JSONB DEFAULT '{}',
    attempt INT DEFAULT 0,
    max_attempts INT DEFAULT 3,
    error_message TEXT,
    locked_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ DEFAULT NOW(),
    completed_at TIMESTAMPTZ
);

-- Registered notification channels
CREATE TABLE IF NOT EXISTS notification_channels (
    id SERIAL PRIMARY KEY,
    type VARCHAR(50) NOT NULL,
    name VARCHAR(100) NOT NULL,
    config JSONB NOT NULL,
    enabled BOOLEAN DEFAULT true,
    created_at TIMESTAMPTZ DEFAULT NOW()
);

-- Subscriptions: route project releases to notification channels
CREATE TABLE IF NOT EXISTS subscriptions (
    id SERIAL PRIMARY KEY,
    project_id INT NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    channel_type VARCHAR(50) NOT NULL,
    channel_id INT REFERENCES notification_channels(id),
    version_pattern TEXT,
    frequency VARCHAR(20) DEFAULT 'instant',
    enabled BOOLEAN DEFAULT true,
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW()
);

-- API authentication keys
CREATE TABLE IF NOT EXISTS api_keys (
    id SERIAL PRIMARY KEY,
    name VARCHAR(100) NOT NULL,
    key_hash VARCHAR(64) NOT NULL UNIQUE,
    key_prefix VARCHAR(12) NOT NULL,
    created_at TIMESTAMPTZ DEFAULT NOW(),
    last_used_at TIMESTAMPTZ
);

-- Trigger for SSE: notify on new releases
CREATE OR REPLACE FUNCTION notify_release_created() RETURNS trigger AS $$
BEGIN
    PERFORM pg_notify('release_events', NEW.id::text);
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

DROP TRIGGER IF EXISTS release_created_trigger ON releases;
CREATE TRIGGER release_created_trigger
    AFTER INSERT ON releases
    FOR EACH ROW EXECUTE FUNCTION notify_release_created();
`

// RunMigrations applies River's schema and the application schema. Idempotent — safe to call on every startup.
func RunMigrations(ctx context.Context, pool *pgxpool.Pool) error {
	migrator, err := rivermigrate.New(riverpgxv5.New(pool), nil)
	if err != nil {
		return fmt.Errorf("create river migrator: %w", err)
	}
	if _, err := migrator.Migrate(ctx, rivermigrate.DirectionUp, nil); err != nil {
		return fmt.Errorf("river migrations: %w", err)
	}

	if _, err := pool.Exec(ctx, schema); err != nil {
		return fmt.Errorf("app migrations: %w", err)
	}
	return nil
}
```

**Step 2: Verify build**

Run: `go build ./internal/db/`
Expected: Success

**Step 3: Commit**

```bash
git add internal/db/migrations.go
git commit -m "feat: migrate to full DESIGN.md schema with projects, sources, channels, api_keys"
```

---

## Task 2: Adapt Ingestion Layer for New Schema

The `releases` table now requires `source_id INT` instead of `source VARCHAR` + `repository VARCHAR`. Update the ingestion layer to accept a source ID.

**Files:**
- Modify: `internal/ingestion/store.go`
- Modify: `internal/ingestion/pgstore.go`
- Modify: `internal/ingestion/service.go`
- Modify: `internal/ingestion/service_test.go`
- Modify: `internal/ingestion/orchestrator.go`
- Modify: `internal/ingestion/source.go`
- Modify: `cmd/server/main.go`

**Step 1: Update the ReleaseStore interface**

```go
// internal/ingestion/store.go
package ingestion

import (
	"context"

	"github.com/sentioxyz/releaseguard/internal/models"
)

// ReleaseStore persists release events using the transactional outbox pattern.
type ReleaseStore interface {
	IngestRelease(ctx context.Context, sourceID int, event *models.ReleaseEvent) error
}
```

**Step 2: Update PgStore to use source_id**

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
func (s *PgStore) IngestRelease(ctx context.Context, sourceID int, event *models.ReleaseEvent) error {
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
		`INSERT INTO releases (id, source_id, version, payload) VALUES ($1, $2, $3, $4)`,
		event.ID, sourceID, event.RawVersion, payload,
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

**Step 3: Add SourceID to IIngestionSource**

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
type IIngestionSource interface {
	Name() string
	SourceID() int
	FetchNewReleases(ctx context.Context) ([]IngestionResult, error)
}
```

**Step 4: Update Service to pass sourceID**

```go
// internal/ingestion/service.go
package ingestion

import (
	"context"
	"log/slog"

	"github.com/google/uuid"
	"github.com/sentioxyz/releaseguard/internal/models"
)

type Service struct {
	store ReleaseStore
}

func NewService(store ReleaseStore) *Service {
	return &Service{store: store}
}

// ProcessResults normalizes raw ingestion results into ReleaseEvents and persists them.
func (s *Service) ProcessResults(ctx context.Context, sourceID int, sourceName string, results []IngestionResult) error {
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

		if err := s.store.IngestRelease(ctx, sourceID, event); err != nil {
			slog.Warn("ingest failed (may be duplicate)",
				"repo", r.Repository, "version", r.RawVersion, "err", err)
			continue
		}
	}
	return nil
}
```

**Step 5: Update Orchestrator**

```go
// internal/ingestion/orchestrator.go — update pollAll to use src.SourceID()
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
		if err := o.service.ProcessResults(ctx, src.SourceID(), src.Name(), results); err != nil {
			slog.Error("process failed", "source", src.Name(), "err", err)
		}
	}
}
```

**Step 6: Update DockerHubSource to implement SourceID()**

Add a `sourceID int` field to `DockerHubSource` and `NewDockerHubSource(client, repository, sourceID)`. Same for any other source implementations. Return `sourceID` from the `SourceID()` method.

**Step 7: Update tests**

Update `service_test.go` mock store signature and test calls to match new `ProcessResults(ctx, sourceID, sourceName, results)` signature. Update `fakeSource` in `orchestrator_test.go` to implement `SourceID() int`.

**Step 8: Update main.go**

The DockerHub source constructor now takes a sourceID. For now, hardcode `0` as a placeholder (sources will be managed via the API in production):

```go
sources := []ingestion.IIngestionSource{
    ingestion.NewDockerHubSource(http.DefaultClient, "library/golang", 0),
}
```

The GitHub webhook callback also needs a sourceID — use `0` as placeholder.

**Step 9: Run tests and verify**

Run: `go test ./... -v`
Expected: All PASS

**Step 10: Commit**

```bash
git add internal/ingestion/ cmd/server/main.go
git commit -m "refactor: adapt ingestion layer for source_id-based releases schema"
```

---

## Task 3: API Domain Models

Add Go structs for entities needed by the API: Project, Source, Subscription, NotificationChannel. Also add API-specific view models.

**Files:**
- Create: `internal/models/project.go`
- Create: `internal/models/source.go`
- Create: `internal/models/subscription.go`
- Create: `internal/models/channel.go`

**Step 1: Write Project model**

```go
// internal/models/project.go
package models

import (
	"encoding/json"
	"time"
)

type Project struct {
	ID             int             `json:"id"`
	Name           string          `json:"name"`
	Description    string          `json:"description,omitempty"`
	URL            string          `json:"url,omitempty"`
	PipelineConfig json.RawMessage `json:"pipeline_config"`
	CreatedAt      time.Time       `json:"created_at"`
	UpdatedAt      time.Time       `json:"updated_at"`
}
```

**Step 2: Write Source model**

```go
// internal/models/source.go
package models

import "time"

type Source struct {
	ID                   int        `json:"id"`
	ProjectID            int        `json:"project_id"`
	SourceType           string     `json:"type"`
	Repository           string     `json:"repository"`
	PollIntervalSeconds  int        `json:"poll_interval_seconds"`
	Enabled              bool       `json:"enabled"`
	ExcludeVersionRegexp string     `json:"exclude_version_regexp,omitempty"`
	ExcludePrereleases   bool       `json:"exclude_prereleases"`
	LastPolledAt         *time.Time `json:"last_polled_at,omitempty"`
	LastError            *string    `json:"last_error,omitempty"`
	CreatedAt            time.Time  `json:"created_at"`
	UpdatedAt            time.Time  `json:"updated_at"`
}
```

**Step 3: Write Subscription model**

```go
// internal/models/subscription.go
package models

import "time"

type Subscription struct {
	ID             int       `json:"id"`
	ProjectID      int       `json:"project_id"`
	ChannelType    string    `json:"channel_type"`
	ChannelID      int       `json:"channel_id"`
	VersionPattern string    `json:"version_pattern,omitempty"`
	Frequency      string    `json:"frequency"`
	Enabled        bool      `json:"enabled"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}
```

**Step 4: Write NotificationChannel model**

```go
// internal/models/channel.go
package models

import (
	"encoding/json"
	"time"
)

type NotificationChannel struct {
	ID        int             `json:"id"`
	Type      string          `json:"type"`
	Name      string          `json:"name"`
	Config    json.RawMessage `json:"config"`
	Enabled   bool            `json:"enabled"`
	CreatedAt time.Time       `json:"created_at"`
}
```

**Step 5: Verify build**

Run: `go build ./internal/models/`
Expected: Success

**Step 6: Commit**

```bash
git add internal/models/project.go internal/models/source.go internal/models/subscription.go internal/models/channel.go
git commit -m "feat: add domain models for projects, sources, subscriptions, channels"
```

---

## Task 4: Add Rate Limiter Dependency

**Files:**
- Modify: `go.mod` (via `go get`)

**Step 1: Install dependency**

```bash
go get golang.org/x/time
```

**Step 2: Tidy**

```bash
go mod tidy
```

**Step 3: Commit**

```bash
git add go.mod go.sum
git commit -m "chore: add golang.org/x/time for API rate limiting"
```

---

## Task 5: Response Envelope Helpers

Every API response uses a consistent JSON envelope. Build helpers for success, list, and error responses.

**Files:**
- Create: `internal/api/response.go`
- Create: `internal/api/response_test.go`

**Step 1: Write the failing test**

```go
// internal/api/response_test.go
package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestRespondJSON(t *testing.T) {
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r = r.WithContext(withRequestID(r.Context(), "test-req-id"))

	RespondJSON(w, r, http.StatusOK, map[string]string{"name": "test"})

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", w.Code, http.StatusOK)
	}
	if ct := w.Header().Get("Content-Type"); ct != "application/json" {
		t.Errorf("Content-Type = %q, want application/json", ct)
	}

	var env envelope
	if err := json.Unmarshal(w.Body.Bytes(), &env); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if env.Meta.RequestID != "test-req-id" {
		t.Errorf("request_id = %q, want test-req-id", env.Meta.RequestID)
	}
}

func TestRespondError(t *testing.T) {
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r = r.WithContext(withRequestID(r.Context(), "err-req"))

	RespondError(w, r, http.StatusNotFound, "not_found", "Project not found")

	if w.Code != http.StatusNotFound {
		t.Errorf("status = %d, want %d", w.Code, http.StatusNotFound)
	}

	var env errorEnvelope
	if err := json.Unmarshal(w.Body.Bytes(), &env); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if env.Error.Code != "not_found" {
		t.Errorf("code = %q, want not_found", env.Error.Code)
	}
}

func TestRespondList(t *testing.T) {
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r = r.WithContext(withRequestID(r.Context(), "list-req"))

	items := []string{"a", "b"}
	RespondList(w, r, http.StatusOK, items, 1, 25, 2)

	var env listEnvelope
	if err := json.Unmarshal(w.Body.Bytes(), &env); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if env.Meta.Total != 2 {
		t.Errorf("total = %d, want 2", env.Meta.Total)
	}
	if env.Meta.Page != 1 {
		t.Errorf("page = %d, want 1", env.Meta.Page)
	}
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/api/ -v -run TestRespond`
Expected: FAIL — types not defined

**Step 3: Implement response helpers**

```go
// internal/api/response.go
package api

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"
)

type contextKey string

const requestIDKey contextKey = "request_id"

func withRequestID(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, requestIDKey, id)
}

func getRequestID(ctx context.Context) string {
	if id, ok := ctx.Value(requestIDKey).(string); ok {
		return id
	}
	return ""
}

type meta struct {
	RequestID string `json:"request_id"`
}

type listMeta struct {
	RequestID string `json:"request_id"`
	Page      int    `json:"page"`
	PerPage   int    `json:"per_page"`
	Total     int    `json:"total"`
}

type envelope struct {
	Data any   `json:"data"`
	Meta meta  `json:"meta"`
}

type listEnvelope struct {
	Data any      `json:"data"`
	Meta listMeta `json:"meta"`
}

type apiError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

type errorEnvelope struct {
	Error apiError `json:"error"`
	Meta  meta     `json:"meta"`
}

// RespondJSON writes a single-resource success response.
func RespondJSON(w http.ResponseWriter, r *http.Request, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(envelope{
		Data: data,
		Meta: meta{RequestID: getRequestID(r.Context())},
	})
}

// RespondList writes a paginated list response.
func RespondList(w http.ResponseWriter, r *http.Request, status int, data any, page, perPage, total int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(listEnvelope{
		Data: data,
		Meta: listMeta{
			RequestID: getRequestID(r.Context()),
			Page:      page,
			PerPage:   perPage,
			Total:     total,
		},
	})
}

// RespondError writes an error response.
func RespondError(w http.ResponseWriter, r *http.Request, status int, code, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(errorEnvelope{
		Error: apiError{Code: code, Message: message},
		Meta:  meta{RequestID: getRequestID(r.Context())},
	})
}

// ParsePagination extracts page and per_page from query params with defaults.
func ParsePagination(r *http.Request) (page, perPage int) {
	page = 1
	perPage = 25
	if v := r.URL.Query().Get("page"); v != "" {
		if p, err := strconv.Atoi(v); err == nil && p > 0 {
			page = p
		}
	}
	if v := r.URL.Query().Get("per_page"); v != "" {
		if pp, err := strconv.Atoi(v); err == nil && pp > 0 && pp <= 100 {
			perPage = pp
		}
	}
	return
}

// DecodeJSON reads and decodes the request body into dst.
func DecodeJSON(r *http.Request, dst any) error {
	return json.NewDecoder(r.Body).Decode(dst)
}
```

**Step 4: Run test to verify it passes**

Run: `go test ./internal/api/ -v -run TestRespond`
Expected: PASS

**Step 5: Commit**

```bash
git add internal/api/response.go internal/api/response_test.go
git commit -m "feat: add API response envelope helpers"
```

---

## Task 6: Core Middleware (Request ID, Logger, Recovery, CORS)

**Files:**
- Create: `internal/api/middleware.go`
- Create: `internal/api/middleware_test.go`

**Step 1: Write the failing test**

```go
// internal/api/middleware_test.go
package api

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestRequestIDMiddleware(t *testing.T) {
	handler := RequestID(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := getRequestID(r.Context())
		if id == "" {
			t.Error("request ID should be set in context")
		}
		w.WriteHeader(http.StatusOK)
	}))

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	handler.ServeHTTP(w, r)

	if rid := w.Header().Get("X-Request-ID"); rid == "" {
		t.Error("X-Request-ID header should be set")
	}
}

func TestRecoveryMiddleware(t *testing.T) {
	handler := Recovery(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		panic("test panic")
	}))

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r = r.WithContext(withRequestID(r.Context(), "panic-req"))
	handler.ServeHTTP(w, r)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want %d", w.Code, http.StatusInternalServerError)
	}
}

func TestCORSMiddleware(t *testing.T) {
	handler := CORS(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	// Preflight
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodOptions, "/", nil)
	r.Header.Set("Origin", "http://localhost:3000")
	handler.ServeHTTP(w, r)

	if w.Code != http.StatusNoContent {
		t.Errorf("preflight status = %d, want %d", w.Code, http.StatusNoContent)
	}
	if w.Header().Get("Access-Control-Allow-Origin") != "*" {
		t.Error("CORS Allow-Origin should be *")
	}
}

func TestChain(t *testing.T) {
	var order []string
	m1 := func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			order = append(order, "m1")
			next.ServeHTTP(w, r)
		})
	}
	m2 := func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			order = append(order, "m2")
			next.ServeHTTP(w, r)
		})
	}
	final := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		order = append(order, "handler")
	})

	Chain(m1, m2)(final).ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/", nil))

	if len(order) != 3 || order[0] != "m1" || order[1] != "m2" || order[2] != "handler" {
		t.Errorf("execution order = %v, want [m1 m2 handler]", order)
	}
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/api/ -v -run "TestRequestID|TestRecovery|TestCORS|TestChain"`
Expected: FAIL

**Step 3: Implement middleware**

```go
// internal/api/middleware.go
package api

import (
	"log/slog"
	"net/http"
	"time"

	"github.com/google/uuid"
)

// Middleware is a function that wraps an http.Handler.
type Middleware func(http.Handler) http.Handler

// Chain composes middleware in order: Chain(A, B)(handler) → A(B(handler)).
func Chain(mws ...Middleware) Middleware {
	return func(final http.Handler) http.Handler {
		for i := len(mws) - 1; i >= 0; i-- {
			final = mws[i](final)
		}
		return final
	}
}

// RequestID generates a UUID request ID and injects it into the context and response header.
func RequestID(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := uuid.New().String()
		w.Header().Set("X-Request-ID", id)
		next.ServeHTTP(w, r.WithContext(withRequestID(r.Context(), id)))
	})
}

// Logger logs structured request information: method, path, duration, status.
func Logger(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		sw := &statusWriter{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(sw, r)
		slog.Info("request",
			"request_id", getRequestID(r.Context()),
			"method", r.Method,
			"path", r.URL.Path,
			"status", sw.status,
			"duration_ms", time.Since(start).Milliseconds(),
		)
	})
}

type statusWriter struct {
	http.ResponseWriter
	status int
}

func (w *statusWriter) WriteHeader(code int) {
	w.status = code
	w.ResponseWriter.WriteHeader(code)
}

// Recovery catches panics and returns a 500 error with the request ID.
func Recovery(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if err := recover(); err != nil {
				slog.Error("panic recovered",
					"request_id", getRequestID(r.Context()),
					"error", err,
				)
				RespondError(w, r, http.StatusInternalServerError, "internal_error", "Internal server error")
			}
		}()
		next.ServeHTTP(w, r)
	})
}

// CORS adds CORS headers for the frontend. Handles preflight OPTIONS requests.
func CORS(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
		w.Header().Set("Access-Control-Max-Age", "86400")

		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}
```

**Step 4: Run test to verify it passes**

Run: `go test ./internal/api/ -v -run "TestRequestID|TestRecovery|TestCORS|TestChain"`
Expected: PASS

**Step 5: Commit**

```bash
git add internal/api/middleware.go internal/api/middleware_test.go
git commit -m "feat: add core middleware — request ID, logger, recovery, CORS"
```

---

## Task 7: Auth + Rate Limit Middleware

**Files:**
- Create: `internal/api/auth.go`
- Create: `internal/api/auth_test.go`

**Step 1: Write the failing test**

```go
// internal/api/auth_test.go
package api

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

type mockKeyStore struct {
	valid bool
}

func (m *mockKeyStore) ValidateKey(ctx context.Context, rawKey string) (bool, error) {
	return m.valid, nil
}

func (m *mockKeyStore) TouchKeyUsage(ctx context.Context, rawKey string) {}

func TestAuthMiddlewareValidKey(t *testing.T) {
	handler := Auth(&mockKeyStore{valid: true})(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/api/v1/projects", nil)
	r = r.WithContext(withRequestID(r.Context(), "auth-test"))
	r.Header.Set("Authorization", "Bearer rg_live_testkey123")
	handler.ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", w.Code, http.StatusOK)
	}
}

func TestAuthMiddlewareMissingKey(t *testing.T) {
	handler := Auth(&mockKeyStore{valid: true})(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("handler should not be called")
	}))

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/api/v1/projects", nil)
	r = r.WithContext(withRequestID(r.Context(), "auth-test"))
	handler.ServeHTTP(w, r)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want %d", w.Code, http.StatusUnauthorized)
	}
}

func TestAuthMiddlewareInvalidKey(t *testing.T) {
	handler := Auth(&mockKeyStore{valid: false})(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("handler should not be called")
	}))

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/api/v1/projects", nil)
	r = r.WithContext(withRequestID(r.Context(), "auth-test"))
	r.Header.Set("Authorization", "Bearer rg_live_badkey")
	handler.ServeHTTP(w, r)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want %d", w.Code, http.StatusUnauthorized)
	}
}

func TestRateLimitMiddleware(t *testing.T) {
	handler := RateLimit(2, 2)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	// First two requests should succeed (burst=2)
	for i := 0; i < 2; i++ {
		w := httptest.NewRecorder()
		r := httptest.NewRequest(http.MethodGet, "/", nil)
		r = r.WithContext(withRequestID(r.Context(), "rl-test"))
		r.RemoteAddr = "1.2.3.4:1234"
		handler.ServeHTTP(w, r)
		if w.Code != http.StatusOK {
			t.Errorf("request %d: status = %d, want %d", i, w.Code, http.StatusOK)
		}
	}

	// Third request should be rate limited
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r = r.WithContext(withRequestID(r.Context(), "rl-test"))
	r.RemoteAddr = "1.2.3.4:1234"
	handler.ServeHTTP(w, r)
	if w.Code != http.StatusTooManyRequests {
		t.Errorf("rate limited: status = %d, want %d", w.Code, http.StatusTooManyRequests)
	}
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/api/ -v -run "TestAuth|TestRateLimit"`
Expected: FAIL

**Step 3: Implement auth + rate limit**

```go
// internal/api/auth.go
package api

import (
	"context"
	"net/http"
	"strings"
	"sync"

	"golang.org/x/time/rate"
)

// KeyStore validates API keys.
type KeyStore interface {
	ValidateKey(ctx context.Context, rawKey string) (bool, error)
	TouchKeyUsage(ctx context.Context, rawKey string)
}

// Auth validates the Bearer token in the Authorization header.
func Auth(store KeyStore) Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			header := r.Header.Get("Authorization")
			if !strings.HasPrefix(header, "Bearer ") {
				RespondError(w, r, http.StatusUnauthorized, "unauthorized", "Missing API key")
				return
			}
			rawKey := strings.TrimPrefix(header, "Bearer ")

			valid, err := store.ValidateKey(r.Context(), rawKey)
			if err != nil || !valid {
				RespondError(w, r, http.StatusUnauthorized, "unauthorized", "Invalid API key")
				return
			}

			go store.TouchKeyUsage(r.Context(), rawKey)
			next.ServeHTTP(w, r)
		})
	}
}

// RateLimit applies a per-IP token bucket rate limiter.
// rps is requests per second, burst is the max burst size.
func RateLimit(rps float64, burst int) Middleware {
	var mu sync.Mutex
	limiters := make(map[string]*rate.Limiter)

	getLimiter := func(key string) *rate.Limiter {
		mu.Lock()
		defer mu.Unlock()
		if lim, ok := limiters[key]; ok {
			return lim
		}
		lim := rate.NewLimiter(rate.Limit(rps), burst)
		limiters[key] = lim
		return lim
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Key by API key if present, else by IP
			key := r.RemoteAddr
			if auth := r.Header.Get("Authorization"); strings.HasPrefix(auth, "Bearer ") {
				key = strings.TrimPrefix(auth, "Bearer ")
			}

			lim := getLimiter(key)
			if !lim.Allow() {
				w.Header().Set("Retry-After", "1")
				RespondError(w, r, http.StatusTooManyRequests, "rate_limited", "Too many requests")
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}
```

**Step 4: Run test to verify it passes**

Run: `go test ./internal/api/ -v -run "TestAuth|TestRateLimit"`
Expected: PASS

**Step 5: Commit**

```bash
git add internal/api/auth.go internal/api/auth_test.go
git commit -m "feat: add auth and rate limit middleware with API key validation"
```

---

## Task 8: PgStore Foundation + Projects Store & Handler

This is the reference pattern for all subsequent resource handlers. Full detail here; later tasks follow the same structure.

**Files:**
- Create: `internal/api/pgstore.go`
- Create: `internal/api/projects.go`
- Create: `internal/api/projects_test.go`

**Step 1: Write PgStore struct**

```go
// internal/api/pgstore.go
package api

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/sentioxyz/releaseguard/internal/models"
)

// PgStore implements all API store interfaces using PostgreSQL.
type PgStore struct {
	pool *pgxpool.Pool
}

func NewPgStore(pool *pgxpool.Pool) *PgStore {
	return &PgStore{pool: pool}
}

// --- ProjectsStore ---

func (s *PgStore) ListProjects(ctx context.Context, page, perPage int) ([]models.Project, int, error) {
	var total int
	err := s.pool.QueryRow(ctx, `SELECT COUNT(*) FROM projects`).Scan(&total)
	if err != nil {
		return nil, 0, fmt.Errorf("count projects: %w", err)
	}

	offset := (page - 1) * perPage
	rows, err := s.pool.Query(ctx,
		`SELECT id, name, COALESCE(description,''), COALESCE(url,''), pipeline_config, created_at, updated_at
		 FROM projects ORDER BY created_at DESC LIMIT $1 OFFSET $2`, perPage, offset)
	if err != nil {
		return nil, 0, fmt.Errorf("list projects: %w", err)
	}
	defer rows.Close()

	var projects []models.Project
	for rows.Next() {
		var p models.Project
		if err := rows.Scan(&p.ID, &p.Name, &p.Description, &p.URL, &p.PipelineConfig, &p.CreatedAt, &p.UpdatedAt); err != nil {
			return nil, 0, fmt.Errorf("scan project: %w", err)
		}
		projects = append(projects, p)
	}
	return projects, total, nil
}

func (s *PgStore) CreateProject(ctx context.Context, p *models.Project) error {
	return s.pool.QueryRow(ctx,
		`INSERT INTO projects (name, description, url, pipeline_config)
		 VALUES ($1, $2, $3, $4)
		 RETURNING id, created_at, updated_at`,
		p.Name, p.Description, p.URL, p.PipelineConfig,
	).Scan(&p.ID, &p.CreatedAt, &p.UpdatedAt)
}

func (s *PgStore) GetProject(ctx context.Context, id int) (*models.Project, error) {
	var p models.Project
	err := s.pool.QueryRow(ctx,
		`SELECT id, name, COALESCE(description,''), COALESCE(url,''), pipeline_config, created_at, updated_at
		 FROM projects WHERE id = $1`, id,
	).Scan(&p.ID, &p.Name, &p.Description, &p.URL, &p.PipelineConfig, &p.CreatedAt, &p.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return &p, nil
}

func (s *PgStore) UpdateProject(ctx context.Context, id int, p *models.Project) error {
	return s.pool.QueryRow(ctx,
		`UPDATE projects SET name=$1, description=$2, url=$3, pipeline_config=$4, updated_at=NOW()
		 WHERE id=$5 RETURNING updated_at`,
		p.Name, p.Description, p.URL, p.PipelineConfig, id,
	).Scan(&p.UpdatedAt)
}

func (s *PgStore) DeleteProject(ctx context.Context, id int) error {
	tag, err := s.pool.Exec(ctx, `DELETE FROM projects WHERE id = $1`, id)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("not found")
	}
	return nil
}

// --- KeyStore ---

func (s *PgStore) ValidateKey(ctx context.Context, rawKey string) (bool, error) {
	hash := sha256.Sum256([]byte(rawKey))
	keyHash := hex.EncodeToString(hash[:])
	var exists bool
	err := s.pool.QueryRow(ctx,
		`SELECT EXISTS(SELECT 1 FROM api_keys WHERE key_hash = $1)`, keyHash,
	).Scan(&exists)
	return exists, err
}

func (s *PgStore) TouchKeyUsage(ctx context.Context, rawKey string) {
	hash := sha256.Sum256([]byte(rawKey))
	keyHash := hex.EncodeToString(hash[:])
	s.pool.Exec(ctx, `UPDATE api_keys SET last_used_at = NOW() WHERE key_hash = $1`, keyHash)
}
```

**Step 2: Write the failing handler test**

```go
// internal/api/projects_test.go
package api

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/sentioxyz/releaseguard/internal/models"
)

type mockProjectsStore struct {
	projects []models.Project
	created  *models.Project
	err      error
}

func (m *mockProjectsStore) ListProjects(_ context.Context, page, perPage int) ([]models.Project, int, error) {
	return m.projects, len(m.projects), m.err
}

func (m *mockProjectsStore) CreateProject(_ context.Context, p *models.Project) error {
	if m.err != nil {
		return m.err
	}
	p.ID = 1
	p.CreatedAt = time.Now()
	p.UpdatedAt = time.Now()
	m.created = p
	return nil
}

func (m *mockProjectsStore) GetProject(_ context.Context, id int) (*models.Project, error) {
	if m.err != nil {
		return nil, m.err
	}
	if len(m.projects) > 0 {
		return &m.projects[0], nil
	}
	return nil, fmt.Errorf("not found")
}

func (m *mockProjectsStore) UpdateProject(_ context.Context, id int, p *models.Project) error {
	return m.err
}

func (m *mockProjectsStore) DeleteProject(_ context.Context, id int) error {
	return m.err
}

func TestProjectsHandlerList(t *testing.T) {
	store := &mockProjectsStore{
		projects: []models.Project{
			{ID: 1, Name: "Geth", PipelineConfig: json.RawMessage(`{}`)},
		},
	}
	h := &ProjectsHandler{store: store}

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/api/v1/projects?page=1&per_page=25", nil)
	r = r.WithContext(withRequestID(r.Context(), "test"))
	h.List(w, r)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", w.Code, http.StatusOK)
	}
}

func TestProjectsHandlerCreate(t *testing.T) {
	store := &mockProjectsStore{}
	h := &ProjectsHandler{store: store}

	body := `{"name":"Geth","description":"Go Ethereum","pipeline_config":{}}`
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/api/v1/projects", bytes.NewBufferString(body))
	r = r.WithContext(withRequestID(r.Context(), "test"))
	h.Create(w, r)

	if w.Code != http.StatusCreated {
		t.Errorf("status = %d, want %d; body = %s", w.Code, http.StatusCreated, w.Body.String())
	}
	if store.created == nil {
		t.Fatal("project should have been created")
	}
	if store.created.Name != "Geth" {
		t.Errorf("name = %q, want Geth", store.created.Name)
	}
}
```

Note: add `"fmt"` to imports in test file for the `fmt.Errorf("not found")` call.

**Step 3: Run test to verify it fails**

Run: `go test ./internal/api/ -v -run TestProjects`
Expected: FAIL — `ProjectsHandler` not defined

**Step 4: Implement Projects handler**

```go
// internal/api/projects.go
package api

import (
	"context"
	"net/http"
	"strconv"

	"github.com/sentioxyz/releaseguard/internal/models"
)

// ProjectsStore defines the data operations needed by the projects handler.
type ProjectsStore interface {
	ListProjects(ctx context.Context, page, perPage int) ([]models.Project, int, error)
	CreateProject(ctx context.Context, p *models.Project) error
	GetProject(ctx context.Context, id int) (*models.Project, error)
	UpdateProject(ctx context.Context, id int, p *models.Project) error
	DeleteProject(ctx context.Context, id int) error
}

// ProjectsHandler handles project CRUD endpoints.
type ProjectsHandler struct {
	store ProjectsStore
}

func (h *ProjectsHandler) List(w http.ResponseWriter, r *http.Request) {
	page, perPage := ParsePagination(r)
	projects, total, err := h.store.ListProjects(r.Context(), page, perPage)
	if err != nil {
		RespondError(w, r, http.StatusInternalServerError, "internal_error", "Failed to list projects")
		return
	}
	if projects == nil {
		projects = []models.Project{}
	}
	RespondList(w, r, http.StatusOK, projects, page, perPage, total)
}

func (h *ProjectsHandler) Create(w http.ResponseWriter, r *http.Request) {
	var p models.Project
	if err := DecodeJSON(r, &p); err != nil {
		RespondError(w, r, http.StatusBadRequest, "bad_request", "Invalid JSON")
		return
	}
	if p.Name == "" {
		RespondError(w, r, http.StatusUnprocessableEntity, "validation_error", "Name is required")
		return
	}
	if p.PipelineConfig == nil {
		p.PipelineConfig = []byte(`{"changelog_summarizer": {}, "urgency_scorer": {}}`)
	}
	if err := h.store.CreateProject(r.Context(), &p); err != nil {
		RespondError(w, r, http.StatusConflict, "conflict", "Project name already exists")
		return
	}
	RespondJSON(w, r, http.StatusCreated, p)
}

func (h *ProjectsHandler) Get(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(r.PathValue("id"))
	if err != nil {
		RespondError(w, r, http.StatusBadRequest, "bad_request", "Invalid project ID")
		return
	}
	p, err := h.store.GetProject(r.Context(), id)
	if err != nil {
		RespondError(w, r, http.StatusNotFound, "not_found", "Project not found")
		return
	}
	RespondJSON(w, r, http.StatusOK, p)
}

func (h *ProjectsHandler) Update(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(r.PathValue("id"))
	if err != nil {
		RespondError(w, r, http.StatusBadRequest, "bad_request", "Invalid project ID")
		return
	}
	var p models.Project
	if err := DecodeJSON(r, &p); err != nil {
		RespondError(w, r, http.StatusBadRequest, "bad_request", "Invalid JSON")
		return
	}
	p.ID = id
	if err := h.store.UpdateProject(r.Context(), id, &p); err != nil {
		RespondError(w, r, http.StatusNotFound, "not_found", "Project not found")
		return
	}
	RespondJSON(w, r, http.StatusOK, p)
}

func (h *ProjectsHandler) Delete(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(r.PathValue("id"))
	if err != nil {
		RespondError(w, r, http.StatusBadRequest, "bad_request", "Invalid project ID")
		return
	}
	if err := h.store.DeleteProject(r.Context(), id); err != nil {
		RespondError(w, r, http.StatusNotFound, "not_found", "Project not found")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
```

**Step 5: Run test to verify it passes**

Run: `go test ./internal/api/ -v -run TestProjects`
Expected: PASS

**Step 6: Commit**

```bash
git add internal/api/pgstore.go internal/api/projects.go internal/api/projects_test.go
git commit -m "feat: add projects CRUD handler, PgStore, and store interfaces"
```

---

## Task 9: Sources Store & Handler

Follow the same pattern as Projects. Sources belong to a project and have two extra endpoints: latest-release and release-by-version.

**Files:**
- Modify: `internal/api/pgstore.go` (add SourcesStore methods)
- Create: `internal/api/sources.go`
- Create: `internal/api/sources_test.go`

**Step 1: Write the failing test**

Create `internal/api/sources_test.go` with a `mockSourcesStore` and tests for `List`, `Create`, `Get`. Same pattern as projects tests: mock store, create handler, use httptest.

**Step 2: Run test to verify it fails**

Run: `go test ./internal/api/ -v -run TestSources`
Expected: FAIL

**Step 3: Define SourcesStore interface and handler**

```go
// internal/api/sources.go
package api

import (
	"context"
	"net/http"
	"strconv"

	"github.com/sentioxyz/releaseguard/internal/models"
)

type SourcesStore interface {
	ListSources(ctx context.Context, page, perPage int) ([]models.Source, int, error)
	CreateSource(ctx context.Context, src *models.Source) error
	GetSource(ctx context.Context, id int) (*models.Source, error)
	UpdateSource(ctx context.Context, id int, src *models.Source) error
	DeleteSource(ctx context.Context, id int) error
	GetLatestRelease(ctx context.Context, sourceID int) (*ReleaseView, error)
	GetReleaseByVersion(ctx context.Context, sourceID int, version string) (*ReleaseView, error)
}

// ReleaseView is a denormalized read model joining releases, sources, and projects.
type ReleaseView struct {
	ID             string          `json:"id"`
	SourceID       int             `json:"source_id"`
	SourceType     string          `json:"source_type"`
	Repository     string          `json:"repository"`
	ProjectID      int             `json:"project_id"`
	ProjectName    string          `json:"project_name"`
	RawVersion     string          `json:"raw_version"`
	IsPreRelease   bool            `json:"is_pre_release"`
	PipelineStatus string          `json:"pipeline_status"`
	CreatedAt      string          `json:"created_at"`
}

type SourcesHandler struct {
	store SourcesStore
}

func (h *SourcesHandler) List(w http.ResponseWriter, r *http.Request) {
	page, perPage := ParsePagination(r)
	sources, total, err := h.store.ListSources(r.Context(), page, perPage)
	if err != nil {
		RespondError(w, r, http.StatusInternalServerError, "internal_error", "Failed to list sources")
		return
	}
	if sources == nil {
		sources = []models.Source{}
	}
	RespondList(w, r, http.StatusOK, sources, page, perPage, total)
}

func (h *SourcesHandler) Create(w http.ResponseWriter, r *http.Request) {
	var src models.Source
	if err := DecodeJSON(r, &src); err != nil {
		RespondError(w, r, http.StatusBadRequest, "bad_request", "Invalid JSON")
		return
	}
	if src.ProjectID == 0 || src.SourceType == "" || src.Repository == "" {
		RespondError(w, r, http.StatusUnprocessableEntity, "validation_error", "project_id, type, and repository are required")
		return
	}
	if err := h.store.CreateSource(r.Context(), &src); err != nil {
		RespondError(w, r, http.StatusConflict, "conflict", "Source already registered")
		return
	}
	RespondJSON(w, r, http.StatusCreated, src)
}

func (h *SourcesHandler) Get(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(r.PathValue("id"))
	if err != nil {
		RespondError(w, r, http.StatusBadRequest, "bad_request", "Invalid source ID")
		return
	}
	src, err := h.store.GetSource(r.Context(), id)
	if err != nil {
		RespondError(w, r, http.StatusNotFound, "not_found", "Source not found")
		return
	}
	RespondJSON(w, r, http.StatusOK, src)
}

func (h *SourcesHandler) Update(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(r.PathValue("id"))
	if err != nil {
		RespondError(w, r, http.StatusBadRequest, "bad_request", "Invalid source ID")
		return
	}
	var src models.Source
	if err := DecodeJSON(r, &src); err != nil {
		RespondError(w, r, http.StatusBadRequest, "bad_request", "Invalid JSON")
		return
	}
	src.ID = id
	if err := h.store.UpdateSource(r.Context(), id, &src); err != nil {
		RespondError(w, r, http.StatusNotFound, "not_found", "Source not found")
		return
	}
	RespondJSON(w, r, http.StatusOK, src)
}

func (h *SourcesHandler) Delete(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(r.PathValue("id"))
	if err != nil {
		RespondError(w, r, http.StatusBadRequest, "bad_request", "Invalid source ID")
		return
	}
	if err := h.store.DeleteSource(r.Context(), id); err != nil {
		RespondError(w, r, http.StatusNotFound, "not_found", "Source not found")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *SourcesHandler) LatestRelease(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(r.PathValue("id"))
	if err != nil {
		RespondError(w, r, http.StatusBadRequest, "bad_request", "Invalid source ID")
		return
	}
	rel, err := h.store.GetLatestRelease(r.Context(), id)
	if err != nil {
		RespondError(w, r, http.StatusNotFound, "not_found", "No releases found")
		return
	}
	RespondJSON(w, r, http.StatusOK, rel)
}

func (h *SourcesHandler) ReleaseByVersion(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(r.PathValue("id"))
	if err != nil {
		RespondError(w, r, http.StatusBadRequest, "bad_request", "Invalid source ID")
		return
	}
	version := r.PathValue("version")
	rel, err := h.store.GetReleaseByVersion(r.Context(), id, version)
	if err != nil {
		RespondError(w, r, http.StatusNotFound, "not_found", "Release not found")
		return
	}
	RespondJSON(w, r, http.StatusOK, rel)
}
```

**Step 4: Add PgStore methods for sources**

Add to `internal/api/pgstore.go`:

```go
// --- SourcesStore ---

func (s *PgStore) ListSources(ctx context.Context, page, perPage int) ([]models.Source, int, error) {
	var total int
	err := s.pool.QueryRow(ctx, `SELECT COUNT(*) FROM sources`).Scan(&total)
	if err != nil {
		return nil, 0, err
	}
	offset := (page - 1) * perPage
	rows, err := s.pool.Query(ctx,
		`SELECT id, project_id, source_type, repository, poll_interval_seconds,
		        enabled, COALESCE(exclude_version_regexp,''), exclude_prereleases,
		        last_polled_at, last_error, created_at, updated_at
		 FROM sources ORDER BY created_at DESC LIMIT $1 OFFSET $2`, perPage, offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var sources []models.Source
	for rows.Next() {
		var src models.Source
		if err := rows.Scan(&src.ID, &src.ProjectID, &src.SourceType, &src.Repository,
			&src.PollIntervalSeconds, &src.Enabled, &src.ExcludeVersionRegexp,
			&src.ExcludePrereleases, &src.LastPolledAt, &src.LastError,
			&src.CreatedAt, &src.UpdatedAt); err != nil {
			return nil, 0, err
		}
		sources = append(sources, src)
	}
	return sources, total, nil
}

func (s *PgStore) CreateSource(ctx context.Context, src *models.Source) error {
	return s.pool.QueryRow(ctx,
		`INSERT INTO sources (project_id, source_type, repository, poll_interval_seconds, enabled, exclude_version_regexp, exclude_prereleases)
		 VALUES ($1, $2, $3, $4, $5, $6, $7)
		 RETURNING id, created_at, updated_at`,
		src.ProjectID, src.SourceType, src.Repository, src.PollIntervalSeconds,
		src.Enabled, src.ExcludeVersionRegexp, src.ExcludePrereleases,
	).Scan(&src.ID, &src.CreatedAt, &src.UpdatedAt)
}

func (s *PgStore) GetSource(ctx context.Context, id int) (*models.Source, error) {
	var src models.Source
	err := s.pool.QueryRow(ctx,
		`SELECT id, project_id, source_type, repository, poll_interval_seconds,
		        enabled, COALESCE(exclude_version_regexp,''), exclude_prereleases,
		        last_polled_at, last_error, created_at, updated_at
		 FROM sources WHERE id = $1`, id,
	).Scan(&src.ID, &src.ProjectID, &src.SourceType, &src.Repository,
		&src.PollIntervalSeconds, &src.Enabled, &src.ExcludeVersionRegexp,
		&src.ExcludePrereleases, &src.LastPolledAt, &src.LastError,
		&src.CreatedAt, &src.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return &src, nil
}

func (s *PgStore) UpdateSource(ctx context.Context, id int, src *models.Source) error {
	return s.pool.QueryRow(ctx,
		`UPDATE sources SET project_id=$1, source_type=$2, repository=$3, poll_interval_seconds=$4,
		        enabled=$5, exclude_version_regexp=$6, exclude_prereleases=$7, updated_at=NOW()
		 WHERE id=$8 RETURNING updated_at`,
		src.ProjectID, src.SourceType, src.Repository, src.PollIntervalSeconds,
		src.Enabled, src.ExcludeVersionRegexp, src.ExcludePrereleases, id,
	).Scan(&src.UpdatedAt)
}

func (s *PgStore) DeleteSource(ctx context.Context, id int) error {
	tag, err := s.pool.Exec(ctx, `DELETE FROM sources WHERE id = $1`, id)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("not found")
	}
	return nil
}

const releaseViewQuery = `
SELECT r.id, s.id, s.source_type, s.repository, p.id, p.name,
       r.version, r.payload->>'is_pre_release',
       COALESCE(pj.state, 'pending'), r.created_at
FROM releases r
JOIN sources s ON r.source_id = s.id
JOIN projects p ON s.project_id = p.id
LEFT JOIN pipeline_jobs pj ON pj.release_id = r.id
`

func (s *PgStore) GetLatestRelease(ctx context.Context, sourceID int) (*ReleaseView, error) {
	var rv ReleaseView
	var isPreStr *string
	err := s.pool.QueryRow(ctx,
		releaseViewQuery+` WHERE r.source_id = $1 ORDER BY r.created_at DESC LIMIT 1`, sourceID,
	).Scan(&rv.ID, &rv.SourceID, &rv.SourceType, &rv.Repository,
		&rv.ProjectID, &rv.ProjectName, &rv.RawVersion, &isPreStr,
		&rv.PipelineStatus, &rv.CreatedAt)
	if err != nil {
		return nil, err
	}
	rv.IsPreRelease = isPreStr != nil && *isPreStr == "true"
	return &rv, nil
}

func (s *PgStore) GetReleaseByVersion(ctx context.Context, sourceID int, version string) (*ReleaseView, error) {
	var rv ReleaseView
	var isPreStr *string
	err := s.pool.QueryRow(ctx,
		releaseViewQuery+` WHERE r.source_id = $1 AND r.version = $2`, sourceID, version,
	).Scan(&rv.ID, &rv.SourceID, &rv.SourceType, &rv.Repository,
		&rv.ProjectID, &rv.ProjectName, &rv.RawVersion, &isPreStr,
		&rv.PipelineStatus, &rv.CreatedAt)
	if err != nil {
		return nil, err
	}
	rv.IsPreRelease = isPreStr != nil && *isPreStr == "true"
	return &rv, nil
}
```

**Step 5: Run tests**

Run: `go test ./internal/api/ -v -run TestSources`
Expected: PASS

**Step 6: Commit**

```bash
git add internal/api/sources.go internal/api/sources_test.go internal/api/pgstore.go
git commit -m "feat: add sources CRUD handler with latest-release and version lookup"
```

---

## Task 10: Releases Store & Handler (Read-Only)

Releases are read-only via the API. Four endpoints: list (paginated/filterable), get, pipeline status, and release notes.

**Files:**
- Create: `internal/api/releases.go`
- Create: `internal/api/releases_test.go`
- Modify: `internal/api/pgstore.go`

**Step 1: Define ReleasesStore interface and handler**

```go
// internal/api/releases.go
package api

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"
	"time"
)

type ListReleasesOpts struct {
	Page       int
	PerPage    int
	ProjectID  *int
	SourceID   *int
	PreRelease *bool
	Sort       string
	Order      string
}

type PipelineStatus struct {
	ReleaseID   string          `json:"release_id"`
	State       string          `json:"state"`
	CurrentNode *string         `json:"current_node"`
	NodeResults json.RawMessage `json:"node_results"`
	Attempt     int             `json:"attempt"`
	CompletedAt *time.Time      `json:"completed_at,omitempty"`
}

type ReleasesStore interface {
	ListReleases(ctx context.Context, opts ListReleasesOpts) ([]ReleaseView, int, error)
	GetRelease(ctx context.Context, id string) (*ReleaseView, error)
	GetReleaseNotes(ctx context.Context, id string) (string, error)
	GetPipelineStatus(ctx context.Context, releaseID string) (*PipelineStatus, error)
}

type ReleasesHandler struct {
	store ReleasesStore
}

func (h *ReleasesHandler) List(w http.ResponseWriter, r *http.Request) {
	page, perPage := ParsePagination(r)
	opts := ListReleasesOpts{Page: page, PerPage: perPage, Sort: "created_at", Order: "desc"}

	if v := r.URL.Query().Get("project_id"); v != "" {
		if id, err := strconv.Atoi(v); err == nil {
			opts.ProjectID = &id
		}
	}
	if v := r.URL.Query().Get("source_id"); v != "" {
		if id, err := strconv.Atoi(v); err == nil {
			opts.SourceID = &id
		}
	}
	if v := r.URL.Query().Get("pre_release"); v != "" {
		b := v == "true"
		opts.PreRelease = &b
	}
	if v := r.URL.Query().Get("sort"); v == "version" || v == "created_at" {
		opts.Sort = v
	}
	if v := r.URL.Query().Get("order"); v == "asc" || v == "desc" {
		opts.Order = v
	}

	releases, total, err := h.store.ListReleases(r.Context(), opts)
	if err != nil {
		RespondError(w, r, http.StatusInternalServerError, "internal_error", "Failed to list releases")
		return
	}
	if releases == nil {
		releases = []ReleaseView{}
	}
	RespondList(w, r, http.StatusOK, releases, page, perPage, total)
}

func (h *ReleasesHandler) Get(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	rel, err := h.store.GetRelease(r.Context(), id)
	if err != nil {
		RespondError(w, r, http.StatusNotFound, "not_found", "Release not found")
		return
	}
	RespondJSON(w, r, http.StatusOK, rel)
}

func (h *ReleasesHandler) Pipeline(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	ps, err := h.store.GetPipelineStatus(r.Context(), id)
	if err != nil {
		RespondError(w, r, http.StatusNotFound, "not_found", "Pipeline status not found")
		return
	}
	RespondJSON(w, r, http.StatusOK, ps)
}

func (h *ReleasesHandler) Notes(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	notes, err := h.store.GetReleaseNotes(r.Context(), id)
	if err != nil {
		RespondError(w, r, http.StatusNotFound, "not_found", "Release not found")
		return
	}
	RespondJSON(w, r, http.StatusOK, map[string]string{"changelog": notes})
}
```

**Step 2: Add PgStore methods for releases**

Key SQL for `ListReleases` — build query dynamically based on filters:

```go
func (s *PgStore) ListReleases(ctx context.Context, opts ListReleasesOpts) ([]ReleaseView, int, error) {
	where := "WHERE 1=1"
	args := []any{}
	argN := 1

	if opts.ProjectID != nil {
		where += fmt.Sprintf(" AND p.id = $%d", argN)
		args = append(args, *opts.ProjectID)
		argN++
	}
	if opts.SourceID != nil {
		where += fmt.Sprintf(" AND s.id = $%d", argN)
		args = append(args, *opts.SourceID)
		argN++
	}

	// Count
	var total int
	countQ := fmt.Sprintf(`SELECT COUNT(*) FROM releases r
		JOIN sources s ON r.source_id = s.id
		JOIN projects p ON s.project_id = p.id %s`, where)
	if err := s.pool.QueryRow(ctx, countQ, args...).Scan(&total); err != nil {
		return nil, 0, err
	}

	// Fetch
	orderCol := "r.created_at"
	if opts.Sort == "version" {
		orderCol = "r.version"
	}
	orderDir := "DESC"
	if opts.Order == "asc" {
		orderDir = "ASC"
	}

	offset := (opts.Page - 1) * opts.PerPage
	dataQ := fmt.Sprintf(`%s %s ORDER BY %s %s LIMIT $%d OFFSET $%d`,
		releaseViewQuery, where, orderCol, orderDir, argN, argN+1)
	args = append(args, opts.PerPage, offset)

	rows, err := s.pool.Query(ctx, dataQ, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var releases []ReleaseView
	for rows.Next() {
		var rv ReleaseView
		var isPreStr *string
		if err := rows.Scan(&rv.ID, &rv.SourceID, &rv.SourceType, &rv.Repository,
			&rv.ProjectID, &rv.ProjectName, &rv.RawVersion, &isPreStr,
			&rv.PipelineStatus, &rv.CreatedAt); err != nil {
			return nil, 0, err
		}
		rv.IsPreRelease = isPreStr != nil && *isPreStr == "true"
		releases = append(releases, rv)
	}
	return releases, total, nil
}

func (s *PgStore) GetRelease(ctx context.Context, id string) (*ReleaseView, error) {
	var rv ReleaseView
	var isPreStr *string
	err := s.pool.QueryRow(ctx,
		releaseViewQuery+` WHERE r.id = $1`, id,
	).Scan(&rv.ID, &rv.SourceID, &rv.SourceType, &rv.Repository,
		&rv.ProjectID, &rv.ProjectName, &rv.RawVersion, &isPreStr,
		&rv.PipelineStatus, &rv.CreatedAt)
	if err != nil {
		return nil, err
	}
	rv.IsPreRelease = isPreStr != nil && *isPreStr == "true"
	return &rv, nil
}

func (s *PgStore) GetReleaseNotes(ctx context.Context, id string) (string, error) {
	var notes string
	err := s.pool.QueryRow(ctx,
		`SELECT COALESCE(payload->>'changelog', '') FROM releases WHERE id = $1`, id,
	).Scan(&notes)
	return notes, err
}

func (s *PgStore) GetPipelineStatus(ctx context.Context, releaseID string) (*PipelineStatus, error) {
	var ps PipelineStatus
	err := s.pool.QueryRow(ctx,
		`SELECT release_id, state, current_node, node_results, attempt, completed_at
		 FROM pipeline_jobs WHERE release_id = $1
		 ORDER BY created_at DESC LIMIT 1`, releaseID,
	).Scan(&ps.ReleaseID, &ps.State, &ps.CurrentNode, &ps.NodeResults, &ps.Attempt, &ps.CompletedAt)
	if err != nil {
		return nil, err
	}
	return &ps, nil
}
```

**Step 3: Write handler tests, run, verify**

Same pattern as projects tests with mock store.

**Step 4: Commit**

```bash
git add internal/api/releases.go internal/api/releases_test.go internal/api/pgstore.go
git commit -m "feat: add releases read-only handler with pagination, pipeline status, notes"
```

---

## Task 11: Subscriptions Store & Handler

Full CRUD. Subscriptions link a project to a notification channel.

**Files:**
- Create: `internal/api/subscriptions.go`
- Create: `internal/api/subscriptions_test.go`
- Modify: `internal/api/pgstore.go`

**Step 1: Define interface and handler**

```go
// internal/api/subscriptions.go
package api

import (
	"context"
	"net/http"
	"strconv"

	"github.com/sentioxyz/releaseguard/internal/models"
)

type SubscriptionsStore interface {
	ListSubscriptions(ctx context.Context, page, perPage int) ([]models.Subscription, int, error)
	CreateSubscription(ctx context.Context, sub *models.Subscription) error
	GetSubscription(ctx context.Context, id int) (*models.Subscription, error)
	UpdateSubscription(ctx context.Context, id int, sub *models.Subscription) error
	DeleteSubscription(ctx context.Context, id int) error
}

type SubscriptionsHandler struct {
	store SubscriptionsStore
}
```

Handler methods follow the exact same pattern as Projects (List, Create, Get, Update, Delete). Validation: `ProjectID` and `ChannelType` are required on Create. Default `Frequency` to `"instant"` if empty.

**Step 2: Add PgStore methods**

Standard CRUD SQL against `subscriptions` table. `CreateSubscription` uses `RETURNING id, created_at, updated_at`. Same scan pattern as projects/sources.

**Step 3: Write tests, run, verify, commit**

```bash
git add internal/api/subscriptions.go internal/api/subscriptions_test.go internal/api/pgstore.go
git commit -m "feat: add subscriptions CRUD handler"
```

---

## Task 12: Channels Store & Handler

Full CRUD for notification channels (Slack, PagerDuty, webhooks).

**Files:**
- Create: `internal/api/channels.go`
- Create: `internal/api/channels_test.go`
- Modify: `internal/api/pgstore.go`

**Step 1: Define interface and handler**

```go
// internal/api/channels.go
package api

import (
	"context"
	"net/http"
	"strconv"

	"github.com/sentioxyz/releaseguard/internal/models"
)

type ChannelsStore interface {
	ListChannels(ctx context.Context, page, perPage int) ([]models.NotificationChannel, int, error)
	CreateChannel(ctx context.Context, ch *models.NotificationChannel) error
	GetChannel(ctx context.Context, id int) (*models.NotificationChannel, error)
	UpdateChannel(ctx context.Context, id int, ch *models.NotificationChannel) error
	DeleteChannel(ctx context.Context, id int) error
}

type ChannelsHandler struct {
	store ChannelsStore
}
```

Same CRUD pattern. Validation: `Type`, `Name`, and `Config` are required on Create. The `Config` JSONB is provider-specific (Slack needs `webhook_url`, PagerDuty needs `routing_key`).

**Step 2: Add PgStore methods, tests, commit**

```bash
git add internal/api/channels.go internal/api/channels_test.go internal/api/pgstore.go
git commit -m "feat: add notification channels CRUD handler"
```

---

## Task 13: Health, Stats, and Providers Handlers

System endpoints. Health is public (no auth). Stats and Providers require auth.

**Files:**
- Create: `internal/api/health.go`
- Create: `internal/api/health_test.go`
- Create: `internal/api/providers.go`

**Step 1: Write the failing test**

```go
// internal/api/health_test.go
package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

type mockHealthChecker struct {
	dbOK    bool
	queueOK bool
}

func (m *mockHealthChecker) PingDB(ctx context.Context) error {
	if !m.dbOK {
		return fmt.Errorf("db down")
	}
	return nil
}

func (m *mockHealthChecker) GetStats(ctx context.Context) (*DashboardStats, error) {
	return &DashboardStats{TotalReleases: 42, ActiveSources: 3}, nil
}

func TestHealthCheck(t *testing.T) {
	h := &HealthHandler{checker: &mockHealthChecker{dbOK: true, queueOK: true}}

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/api/v1/health", nil)
	r = r.WithContext(withRequestID(r.Context(), "health-test"))
	h.Check(w, r)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", w.Code, http.StatusOK)
	}

	var body map[string]any
	json.Unmarshal(w.Body.Bytes(), &body)
	data := body["data"].(map[string]any)
	if data["status"] != "healthy" {
		t.Errorf("status = %v, want healthy", data["status"])
	}
}
```

**Step 2: Implement health handler**

```go
// internal/api/health.go
package api

import (
	"context"
	"net/http"
)

type DashboardStats struct {
	TotalReleases int `json:"total_releases"`
	ActiveSources int `json:"active_sources"`
	PendingJobs   int `json:"pending_jobs"`
	FailedJobs    int `json:"failed_jobs"`
}

type HealthChecker interface {
	PingDB(ctx context.Context) error
	GetStats(ctx context.Context) (*DashboardStats, error)
}

type HealthHandler struct {
	checker HealthChecker
}

func (h *HealthHandler) Check(w http.ResponseWriter, r *http.Request) {
	checks := map[string]string{"database": "ok", "queue": "ok"}

	if err := h.checker.PingDB(r.Context()); err != nil {
		checks["database"] = "error"
		RespondJSON(w, r, http.StatusServiceUnavailable, map[string]any{
			"status": "unhealthy", "checks": checks,
		})
		return
	}

	RespondJSON(w, r, http.StatusOK, map[string]any{
		"status": "healthy", "checks": checks,
	})
}

func (h *HealthHandler) Stats(w http.ResponseWriter, r *http.Request) {
	stats, err := h.checker.GetStats(r.Context())
	if err != nil {
		RespondError(w, r, http.StatusInternalServerError, "internal_error", "Failed to get stats")
		return
	}
	RespondJSON(w, r, http.StatusOK, stats)
}
```

**Step 3: Implement providers handler**

```go
// internal/api/providers.go
package api

import "net/http"

type ProvidersHandler struct{}

func (h *ProvidersHandler) List(w http.ResponseWriter, r *http.Request) {
	providers := []map[string]string{
		{"id": "dockerhub", "name": "Docker Hub", "type": "polling"},
		{"id": "github", "name": "GitHub", "type": "webhook"},
	}
	RespondJSON(w, r, http.StatusOK, providers)
}
```

**Step 4: Add PgStore health methods**

```go
func (s *PgStore) PingDB(ctx context.Context) error {
	return s.pool.Ping(ctx)
}

func (s *PgStore) GetStats(ctx context.Context) (*DashboardStats, error) {
	var stats DashboardStats
	s.pool.QueryRow(ctx, `SELECT COUNT(*) FROM releases`).Scan(&stats.TotalReleases)
	s.pool.QueryRow(ctx, `SELECT COUNT(*) FROM sources WHERE enabled = true`).Scan(&stats.ActiveSources)
	s.pool.QueryRow(ctx, `SELECT COUNT(*) FROM pipeline_jobs WHERE state = 'available'`).Scan(&stats.PendingJobs)
	s.pool.QueryRow(ctx, `SELECT COUNT(*) FROM pipeline_jobs WHERE state = 'discarded'`).Scan(&stats.FailedJobs)
	return &stats, nil
}
```

**Step 5: Run tests, commit**

```bash
git add internal/api/health.go internal/api/health_test.go internal/api/providers.go internal/api/pgstore.go
git commit -m "feat: add health check, dashboard stats, and providers metadata endpoints"
```

---

## Task 14: SSE Broadcaster

Real-time events via Server-Sent Events backed by PostgreSQL `LISTEN/NOTIFY`.

**Files:**
- Create: `internal/api/events.go`
- Create: `internal/api/events_test.go`

**Step 1: Write the failing test**

```go
// internal/api/events_test.go
package api

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestBroadcasterSubscribeAndSend(t *testing.T) {
	b := NewBroadcaster()

	// Subscribe a client
	ch := b.Subscribe()
	defer b.Unsubscribe(ch)

	// Send an event
	b.Send(SSEEvent{Event: "release.created", Data: `{"id":"abc"}`})

	select {
	case evt := <-ch:
		if evt.Event != "release.created" {
			t.Errorf("event = %q, want release.created", evt.Event)
		}
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for event")
	}
}

func TestEventsHandlerSSE(t *testing.T) {
	b := NewBroadcaster()
	h := &EventsHandler{broadcaster: b}

	// Start request in goroutine (SSE blocks)
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/api/v1/events", nil)
	r = r.WithContext(withRequestID(r.Context(), "sse-test"))

	done := make(chan struct{})
	go func() {
		h.Stream(w, r)
		close(done)
	}()

	// Give handler time to set up
	time.Sleep(50 * time.Millisecond)

	// Send event
	b.Send(SSEEvent{Event: "release.created", Data: `{"id":"test"}`})

	// Wait briefly for write
	time.Sleep(50 * time.Millisecond)

	body := w.Body.String()
	if !strings.Contains(body, "event: release.created") {
		t.Errorf("body should contain SSE event, got: %s", body)
	}
}
```

**Step 2: Implement broadcaster**

```go
// internal/api/events.go
package api

import (
	"fmt"
	"net/http"
	"sync"
)

// SSEEvent is a server-sent event.
type SSEEvent struct {
	Event string
	Data  string
}

// Broadcaster manages SSE client subscriptions and event distribution.
type Broadcaster struct {
	mu      sync.RWMutex
	clients map[chan SSEEvent]struct{}
}

func NewBroadcaster() *Broadcaster {
	return &Broadcaster{clients: make(map[chan SSEEvent]struct{})}
}

func (b *Broadcaster) Subscribe() chan SSEEvent {
	ch := make(chan SSEEvent, 64)
	b.mu.Lock()
	b.clients[ch] = struct{}{}
	b.mu.Unlock()
	return ch
}

func (b *Broadcaster) Unsubscribe(ch chan SSEEvent) {
	b.mu.Lock()
	delete(b.clients, ch)
	b.mu.Unlock()
	close(ch)
}

func (b *Broadcaster) Send(evt SSEEvent) {
	b.mu.RLock()
	defer b.mu.RUnlock()
	for ch := range b.clients {
		select {
		case ch <- evt:
		default:
			// Drop event for slow clients
		}
	}
}

// EventsHandler serves the SSE stream.
type EventsHandler struct {
	broadcaster *Broadcaster
}

func (h *EventsHandler) Stream(w http.ResponseWriter, r *http.Request) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		RespondError(w, r, http.StatusInternalServerError, "internal_error", "Streaming not supported")
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.WriteHeader(http.StatusOK)
	flusher.Flush()

	ch := h.broadcaster.Subscribe()
	defer h.broadcaster.Unsubscribe(ch)

	for {
		select {
		case <-r.Context().Done():
			return
		case evt := <-ch:
			fmt.Fprintf(w, "event: %s\ndata: %s\n\n", evt.Event, evt.Data)
			flusher.Flush()
		}
	}
}
```

**Step 3: Add LISTEN/NOTIFY integration**

Add a function to start listening on the PostgreSQL `release_events` channel and forward notifications to the broadcaster. Called from `main.go`:

```go
// Add to internal/api/events.go

// ListenForNotifications connects to PostgreSQL LISTEN and forwards to the broadcaster.
// Blocks until ctx is cancelled. Run in a goroutine.
func ListenForNotifications(ctx context.Context, pool *pgxpool.Pool, broadcaster *Broadcaster) {
	conn, err := pool.Acquire(ctx)
	if err != nil {
		slog.Error("listen: acquire connection", "err", err)
		return
	}
	defer conn.Release()

	_, err = conn.Exec(ctx, "LISTEN release_events")
	if err != nil {
		slog.Error("listen: LISTEN command", "err", err)
		return
	}

	for {
		notification, err := conn.Conn().WaitForNotification(ctx)
		if err != nil {
			if ctx.Err() != nil {
				return // Clean shutdown
			}
			slog.Error("listen: wait for notification", "err", err)
			return
		}
		broadcaster.Send(SSEEvent{
			Event: "release.created",
			Data:  fmt.Sprintf(`{"id":"%s"}`, notification.Payload),
		})
	}
}
```

Note: add necessary imports (`context`, `fmt`, `log/slog`, `github.com/jackc/pgx/v5/pgxpool`).

**Step 4: Run tests, commit**

```bash
git add internal/api/events.go internal/api/events_test.go
git commit -m "feat: add SSE broadcaster with PostgreSQL LISTEN/NOTIFY integration"
```

---

## Task 15: Route Registration + main.go Wiring

Wire all handlers, middleware, and the SSE broadcaster into the HTTP server.

**Files:**
- Create: `internal/api/server.go`
- Modify: `cmd/server/main.go`

**Step 1: Create route registration**

```go
// internal/api/server.go
package api

import (
	"net/http"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Dependencies holds all injected dependencies for the API layer.
type Dependencies struct {
	DB                 *pgxpool.Pool
	ProjectsStore      ProjectsStore
	ReleasesStore      ReleasesStore
	SubscriptionsStore SubscriptionsStore
	SourcesStore       SourcesStore
	ChannelsStore      ChannelsStore
	KeyStore           KeyStore
	HealthChecker      HealthChecker
	Broadcaster        *Broadcaster
}

// RegisterRoutes wires all API endpoints with middleware chains.
func RegisterRoutes(mux *http.ServeMux, deps Dependencies) {
	// Middleware chains
	chain := Chain(RequestID, Logger, Recovery, RateLimit(10, 20), Auth(deps.KeyStore), CORS)
	publicChain := Chain(RequestID, Logger, Recovery, CORS)

	// Projects
	projects := &ProjectsHandler{store: deps.ProjectsStore}
	mux.Handle("GET /api/v1/projects", chain(http.HandlerFunc(projects.List)))
	mux.Handle("POST /api/v1/projects", chain(http.HandlerFunc(projects.Create)))
	mux.Handle("GET /api/v1/projects/{id}", chain(http.HandlerFunc(projects.Get)))
	mux.Handle("PUT /api/v1/projects/{id}", chain(http.HandlerFunc(projects.Update)))
	mux.Handle("DELETE /api/v1/projects/{id}", chain(http.HandlerFunc(projects.Delete)))

	// Releases (read-only)
	releases := &ReleasesHandler{store: deps.ReleasesStore}
	mux.Handle("GET /api/v1/releases", chain(http.HandlerFunc(releases.List)))
	mux.Handle("GET /api/v1/releases/{id}", chain(http.HandlerFunc(releases.Get)))
	mux.Handle("GET /api/v1/releases/{id}/pipeline", chain(http.HandlerFunc(releases.Pipeline)))
	mux.Handle("GET /api/v1/releases/{id}/notes", chain(http.HandlerFunc(releases.Notes)))

	// Subscriptions
	subs := &SubscriptionsHandler{store: deps.SubscriptionsStore}
	mux.Handle("GET /api/v1/subscriptions", chain(http.HandlerFunc(subs.List)))
	mux.Handle("POST /api/v1/subscriptions", chain(http.HandlerFunc(subs.Create)))
	mux.Handle("GET /api/v1/subscriptions/{id}", chain(http.HandlerFunc(subs.Get)))
	mux.Handle("PUT /api/v1/subscriptions/{id}", chain(http.HandlerFunc(subs.Update)))
	mux.Handle("DELETE /api/v1/subscriptions/{id}", chain(http.HandlerFunc(subs.Delete)))

	// Sources
	sources := &SourcesHandler{store: deps.SourcesStore}
	mux.Handle("GET /api/v1/sources", chain(http.HandlerFunc(sources.List)))
	mux.Handle("POST /api/v1/sources", chain(http.HandlerFunc(sources.Create)))
	mux.Handle("GET /api/v1/sources/{id}", chain(http.HandlerFunc(sources.Get)))
	mux.Handle("PUT /api/v1/sources/{id}", chain(http.HandlerFunc(sources.Update)))
	mux.Handle("DELETE /api/v1/sources/{id}", chain(http.HandlerFunc(sources.Delete)))
	mux.Handle("GET /api/v1/sources/{id}/latest-release", chain(http.HandlerFunc(sources.LatestRelease)))
	mux.Handle("GET /api/v1/sources/{id}/releases/{version}", chain(http.HandlerFunc(sources.ReleaseByVersion)))

	// Notification channels
	channels := &ChannelsHandler{store: deps.ChannelsStore}
	mux.Handle("GET /api/v1/channels", chain(http.HandlerFunc(channels.List)))
	mux.Handle("POST /api/v1/channels", chain(http.HandlerFunc(channels.Create)))
	mux.Handle("GET /api/v1/channels/{id}", chain(http.HandlerFunc(channels.Get)))
	mux.Handle("PUT /api/v1/channels/{id}", chain(http.HandlerFunc(channels.Update)))
	mux.Handle("DELETE /api/v1/channels/{id}", chain(http.HandlerFunc(channels.Delete)))

	// Providers (metadata)
	providers := &ProvidersHandler{}
	mux.Handle("GET /api/v1/providers", chain(http.HandlerFunc(providers.List)))

	// SSE events
	events := &EventsHandler{broadcaster: deps.Broadcaster}
	mux.Handle("GET /api/v1/events", chain(http.HandlerFunc(events.Stream)))

	// Health (public — no auth)
	health := &HealthHandler{checker: deps.HealthChecker}
	mux.Handle("GET /api/v1/health", publicChain(http.HandlerFunc(health.Check)))
	mux.Handle("GET /api/v1/stats", chain(http.HandlerFunc(health.Stats)))
}
```

**Step 2: Update main.go**

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

	"github.com/sentioxyz/releaseguard/internal/api"
	"github.com/sentioxyz/releaseguard/internal/db"
	"github.com/sentioxyz/releaseguard/internal/ingestion"
	"github.com/sentioxyz/releaseguard/internal/queue"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	dbURL := envOr("DATABASE_URL", "postgres://localhost:5432/releaseguard?sslmode=disable")
	ghSecret := envOr("GITHUB_WEBHOOK_SECRET", "")
	addr := envOr("LISTEN_ADDR", ":8080")

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
	ingestionStore := ingestion.NewPgStore(pool, riverClient)
	svc := ingestion.NewService(ingestionStore)

	sources := []ingestion.IIngestionSource{
		ingestion.NewDockerHubSource(http.DefaultClient, "library/golang", 0),
	}

	orch := ingestion.NewOrchestrator(svc, sources, 5*time.Minute)

	// GitHub webhook handler
	webhookHandler := ingestion.NewGitHubWebhookHandler(ghSecret, func(results []ingestion.IngestionResult) {
		if err := svc.ProcessResults(ctx, 0, "github", results); err != nil {
			slog.Error("github webhook processing failed", "err", err)
		}
	})

	// API layer
	pgStore := api.NewPgStore(pool)
	broadcaster := api.NewBroadcaster()

	mux := http.NewServeMux()

	api.RegisterRoutes(mux, api.Dependencies{
		DB:                 pool,
		ProjectsStore:      pgStore,
		ReleasesStore:      pgStore,
		SubscriptionsStore: pgStore,
		SourcesStore:       pgStore,
		ChannelsStore:      pgStore,
		KeyStore:           pgStore,
		HealthChecker:      pgStore,
		Broadcaster:        broadcaster,
	})

	// Webhook routes (outside /api/v1, separate auth)
	mux.Handle("POST /webhook/github", webhookHandler)

	srv := &http.Server{Addr: addr, Handler: mux}

	// Start background processes
	go orch.Run(ctx)
	go api.ListenForNotifications(ctx, pool, broadcaster)

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

**Step 3: Verify build**

Run: `go build ./cmd/server`
Expected: Success

**Step 4: Run all tests**

Run: `go test ./... -v`
Expected: All PASS

**Step 5: Run vet**

Run: `go vet ./...`
Expected: Clean

**Step 6: Commit**

```bash
git add internal/api/server.go cmd/server/main.go
git commit -m "feat: wire API routes, middleware, and SSE into main server"
```

---

## Summary

| Task | Component | Files Created | Endpoints |
|------|-----------|--------------|-----------|
| 1 | Schema Migration | migrations.go | — |
| 2 | Ingestion Adaptation | store.go, pgstore.go, service.go, source.go | — |
| 3 | Domain Models | project.go, source.go, subscription.go, channel.go | — |
| 4 | Rate Limit Dep | go.mod | — |
| 5 | Response Helpers | response.go | — |
| 6 | Core Middleware | middleware.go | — |
| 7 | Auth + Rate Limit | auth.go | — |
| 8 | Projects | pgstore.go, projects.go | 5 endpoints |
| 9 | Sources | sources.go | 7 endpoints |
| 10 | Releases | releases.go | 4 endpoints |
| 11 | Subscriptions | subscriptions.go | 5 endpoints |
| 12 | Channels | channels.go | 5 endpoints |
| 13 | Health/Stats/Providers | health.go, providers.go | 3 endpoints |
| 14 | SSE Broadcaster | events.go | 1 endpoint |
| 15 | Route Wiring | server.go, main.go | — |

**Total: 30 API endpoints, 15 tasks, ~15 new files in `internal/api/`**