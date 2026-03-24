# Release Gate Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add a release gate system that delays agent analysis and notifications until all required sources report a version, with configurable timeouts and version mapping.

**Architecture:** A project-level `release_gate` config controls when the LLM agent runs for multi-source projects. A `GateCheckWorker` evaluates readiness on each release ingestion. A periodic `GateTimeoutWorker` sweeps expired gates. Existing `source_release` / `semantic_release` subscriptions remain unchanged.

**Tech Stack:** Go 1.25, PostgreSQL (JSONB), River v0.31.0 job queue, existing test patterns (mock stores, table-driven tests)

**Spec:** `docs/superpowers/specs/2026-03-23-release-gate-design.md`

---

## File Structure

| File | Action | Responsibility |
|------|--------|---------------|
| `internal/models/release_gate.go` | Create | `ReleaseGate`, `VersionReadiness`, `GateEvent` structs |
| `internal/gate/worker.go` | Create | `GateCheckWorker` — evaluates structured gate rules |
| `internal/gate/worker_test.go` | Create | Unit tests for `GateCheckWorker` |
| `internal/gate/nl_worker.go` | Create | `GateNLEvalWorker` — LLM evaluation of NL rules |
| `internal/gate/nl_worker_test.go` | Create | Unit tests for `GateNLEvalWorker` |
| `internal/gate/timeout_worker.go` | Create | `GateTimeoutWorker` — periodic sweep of expired gates |
| `internal/gate/timeout_worker_test.go` | Create | Unit tests for `GateTimeoutWorker` |
| `internal/gate/version.go` | Create | Version normalization logic (regex/template) |
| `internal/gate/version_test.go` | Create | Unit tests for version normalization |
| `internal/gate/store.go` | Create | `GateStore` interface definition |
| `internal/queue/jobs.go` | Modify | Add `GateCheckJobArgs`, `GateNLEvalJobArgs`, `GateTimeoutJobArgs` |
| `internal/db/migrations.go` | Modify | Add `release_gates`, `version_readiness`, `gate_events` tables + indexes |
| `internal/ingestion/pgstore.go` | Modify | Enqueue `GateCheckJobArgs` alongside `NotifyJobArgs` in `IngestRelease` |
| `internal/routing/worker.go` | Modify | Skip agent rule check when project has gate (gate handles timing) |
| `internal/api/pgstore.go` | Modify | Add gate store methods (CRUD, upsert readiness, list events) |
| `internal/api/gates.go` | Create | HTTP handlers for gate config + version readiness + events |
| `internal/api/server.go` | Modify | Register gate routes, add `GateStore` to Dependencies |
| `cmd/server/main.go` | Modify | Register `GateCheckWorker`, `GateNLEvalWorker`, `GateTimeoutWorker`, periodic job |
| `DESIGN.md` | Modify | Update subscription type names, add release gate documentation |

---

## Task 1: Models — ReleaseGate, VersionReadiness, GateEvent

**Files:**
- Create: `internal/models/release_gate.go`

- [ ] **Step 1: Create the models file with all three structs**

```go
package models

import (
	"encoding/json"
	"time"
)

// VersionMapping defines a per-source regex/template for normalizing versions.
type VersionMapping struct {
	Pattern  string `json:"pattern"`
	Template string `json:"template"`
}

// ReleaseGate is a per-project gate configuration that controls when the
// LLM agent runs for multi-source projects.
type ReleaseGate struct {
	ID              string                    `json:"id"`
	ProjectID       string                    `json:"project_id"`
	RequiredSources []string                  `json:"required_sources,omitempty"` // source UUIDs; empty = all
	TimeoutHours    int                       `json:"timeout_hours"`
	VersionMapping  map[string]VersionMapping `json:"version_mapping,omitempty"` // keyed by source ID
	NLRule          string                    `json:"nl_rule,omitempty"`
	Enabled         bool                      `json:"enabled"`
	CreatedAt       time.Time                 `json:"created_at"`
	UpdatedAt       time.Time                 `json:"updated_at"`
}

// VersionReadiness tracks gate state for a specific version.
type VersionReadiness struct {
	ID              string     `json:"id"`
	ProjectID       string     `json:"project_id"`
	Version         string     `json:"version"` // normalized
	Status          string     `json:"status"`  // pending, ready, timed_out
	SourcesMet      []string   `json:"sources_met"`
	SourcesMissing  []string   `json:"sources_missing"`
	NLRulePassed    *bool      `json:"nl_rule_passed,omitempty"`
	TimeoutAt       time.Time  `json:"timeout_at"`
	OpenedAt        *time.Time `json:"opened_at,omitempty"`
	AgentTriggered  bool       `json:"agent_triggered"`
	CreatedAt       time.Time  `json:"created_at"`
	UpdatedAt       time.Time  `json:"updated_at"`
}

// GateEvent records a state transition in the gate lifecycle.
type GateEvent struct {
	ID                 string          `json:"id"`
	VersionReadinessID string          `json:"version_readiness_id"`
	ProjectID          string          `json:"project_id"`
	Version            string          `json:"version"`
	EventType          string          `json:"event_type"`
	SourceID           *string         `json:"source_id,omitempty"`
	Details            json.RawMessage `json:"details,omitempty"`
	CreatedAt          time.Time       `json:"created_at"`
}
```

- [ ] **Step 2: Verify it compiles**

Run: `go vet ./internal/models/...`
Expected: no errors

- [ ] **Step 3: Commit**

```bash
git add internal/models/release_gate.go
git commit -m "feat(gate): add ReleaseGate, VersionReadiness, GateEvent models"
```

---

## Task 2: Database Migration — New Tables and Indexes

**Files:**
- Modify: `internal/db/migrations.go`

- [ ] **Step 1: Add the three new tables to the schema constant**

Append to the `schema` constant (before the closing backtick at line 210) the `release_gates`, `version_readiness`, and `gate_events` table definitions:

```sql
-- Release gates (per-project gate configuration)
CREATE TABLE IF NOT EXISTS release_gates (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    project_id UUID NOT NULL UNIQUE REFERENCES projects(id) ON DELETE CASCADE,
    required_sources JSONB,
    timeout_hours INT NOT NULL DEFAULT 168,
    version_mapping JSONB,
    nl_rule TEXT,
    enabled BOOLEAN NOT NULL DEFAULT true,
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW()
);

-- Version readiness (per-version gate state tracking)
CREATE TABLE IF NOT EXISTS version_readiness (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    project_id UUID NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    version VARCHAR(100) NOT NULL,
    status VARCHAR(20) NOT NULL DEFAULT 'pending'
        CHECK (status IN ('pending', 'ready', 'timed_out')),
    sources_met JSONB NOT NULL DEFAULT '[]',
    sources_missing JSONB NOT NULL DEFAULT '[]',
    nl_rule_passed BOOLEAN,
    timeout_at TIMESTAMPTZ NOT NULL,
    opened_at TIMESTAMPTZ,
    agent_triggered BOOLEAN NOT NULL DEFAULT false,
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW(),
    UNIQUE(project_id, version)
);
CREATE INDEX IF NOT EXISTS idx_version_readiness_timeout
    ON version_readiness(timeout_at) WHERE status = 'pending';

-- Gate events (audit log for gate lifecycle)
CREATE TABLE IF NOT EXISTS gate_events (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    version_readiness_id UUID NOT NULL REFERENCES version_readiness(id) ON DELETE CASCADE,
    project_id UUID NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    version VARCHAR(100) NOT NULL,
    event_type VARCHAR(30) NOT NULL,
    source_id UUID,
    details JSONB,
    created_at TIMESTAMPTZ DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_gate_events_readiness ON gate_events(version_readiness_id, created_at);
CREATE INDEX IF NOT EXISTS idx_gate_events_project ON gate_events(project_id, created_at);
```

- [ ] **Step 2: Verify migrations compile**

Run: `go vet ./internal/db/...`
Expected: no errors

- [ ] **Step 3: Test migrations run (requires local Postgres)**

Run: `make db-reset && go test ./internal/db/... -v`
Expected: migrations apply without error

- [ ] **Step 4: Commit**

```bash
git add internal/db/migrations.go
git commit -m "feat(gate): add release_gates, version_readiness, gate_events tables"
```

---

## Task 3: Queue Jobs — GateCheck, GateNLEval, GateTimeout

**Files:**
- Modify: `internal/queue/jobs.go`

- [ ] **Step 1: Add the three new job types**

Append to `internal/queue/jobs.go` after line 36:

```go
// GateCheckJobArgs is enqueued when a release is ingested for any project.
// The worker checks if a release gate exists and evaluates readiness.
type GateCheckJobArgs struct {
	SourceID  string `json:"source_id"`
	ReleaseID string `json:"release_id"`
	Version   string `json:"version"` // raw version from source
}

func (GateCheckJobArgs) Kind() string { return "gate_check" }

var _ river.JobArgs = GateCheckJobArgs{}

// GateNLEvalJobArgs is enqueued when structured gate rules pass and an NL rule
// needs LLM evaluation.
type GateNLEvalJobArgs struct {
	VersionReadinessID string `json:"version_readiness_id"`
	ProjectID          string `json:"project_id"`
	Version            string `json:"version"`
}

func (GateNLEvalJobArgs) Kind() string { return "gate_nl_eval" }

var _ river.JobArgs = GateNLEvalJobArgs{}

// GateTimeoutJobArgs is a periodic job that sweeps expired pending gates.
type GateTimeoutJobArgs struct{}

func (GateTimeoutJobArgs) Kind() string { return "gate_timeout" }

var _ river.JobArgs = GateTimeoutJobArgs{}
```

- [ ] **Step 2: Verify compilation**

Run: `go vet ./internal/queue/...`
Expected: no errors

- [ ] **Step 3: Commit**

```bash
git add internal/queue/jobs.go
git commit -m "feat(gate): add GateCheck, GateNLEval, GateTimeout job types"
```

---

## Task 4: Version Normalization Logic

**Files:**
- Create: `internal/gate/version.go`
- Create: `internal/gate/version_test.go`

- [ ] **Step 1: Write failing tests for version normalization**

```go
package gate

import (
	"testing"

	"github.com/sentioxyz/changelogue/internal/models"
)

func TestNormalizeVersion(t *testing.T) {
	tests := []struct {
		name     string
		raw      string
		mapping  *models.VersionMapping
		expected string
	}{
		{
			name:     "no mapping strips v prefix",
			raw:      "v1.21.0",
			mapping:  nil,
			expected: "1.21.0",
		},
		{
			name:     "no mapping lowercases",
			raw:      "V1.21.0-RC1",
			mapping:  nil,
			expected: "1.21.0-rc1",
		},
		{
			name:     "no mapping no v prefix",
			raw:      "1.21.0",
			mapping:  nil,
			expected: "1.21.0",
		},
		{
			name:     "mapping with capture group",
			raw:      "v1.21.0",
			mapping:  &models.VersionMapping{Pattern: `^v?(.+)$`, Template: "$1"},
			expected: "1.21.0",
		},
		{
			name:     "mapping extracts semver from complex tag",
			raw:      "1.21.0-alpine",
			mapping:  &models.VersionMapping{Pattern: `^(\d+\.\d+\.\d+)`, Template: "$1"},
			expected: "1.21.0",
		},
		{
			name:     "mapping with invalid regex falls back to default",
			raw:      "v2.0.0",
			mapping:  &models.VersionMapping{Pattern: `[invalid`, Template: "$1"},
			expected: "2.0.0",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := NormalizeVersion(tt.raw, tt.mapping)
			if got != tt.expected {
				t.Errorf("NormalizeVersion(%q) = %q, want %q", tt.raw, got, tt.expected)
			}
		})
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/gate/... -v -run TestNormalizeVersion`
Expected: FAIL — function not defined

- [ ] **Step 3: Implement NormalizeVersion**

```go
package gate

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/sentioxyz/changelogue/internal/models"
)

// NormalizeVersion applies a version mapping (regex + template) to a raw version
// string. If no mapping is provided or the regex is invalid, it falls back to
// stripping the "v"/"V" prefix and lowercasing.
func NormalizeVersion(raw string, mapping *models.VersionMapping) string {
	if mapping != nil && mapping.Pattern != "" {
		re, err := regexp.Compile(mapping.Pattern)
		if err == nil {
			matches := re.FindStringSubmatch(raw)
			if len(matches) > 1 {
				// Apply template with capture group substitution.
				result := mapping.Template
				for i := 1; i < len(matches); i++ {
					placeholder := fmt.Sprintf("$%d", i)
					result = strings.ReplaceAll(result, placeholder, matches[i])
				}
				if result != "" {
					return result
				}
			}
		}
	}
	// Default: strip v/V prefix, lowercase.
	v := strings.TrimPrefix(raw, "v")
	v = strings.TrimPrefix(v, "V")
	return strings.ToLower(v)
}

// NormalizeVersionForSource looks up the mapping for a source and normalizes the version.
func NormalizeVersionForSource(raw string, sourceID string, mappings map[string]models.VersionMapping) string {
	if m, ok := mappings[sourceID]; ok {
		return NormalizeVersion(raw, &m)
	}
	return NormalizeVersion(raw, nil)
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/gate/... -v -run TestNormalizeVersion`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/gate/version.go internal/gate/version_test.go
git commit -m "feat(gate): add version normalization with regex/template mapping"
```

---

## Task 5: Gate Store Interface

**Files:**
- Create: `internal/gate/store.go`

- [ ] **Step 1: Define the GateStore interface**

The interface includes methods that already exist on `api.PgStore` (`GetSource`, `GetProject`, `ListSourcesByProject`, `EnqueueAgentRun`). The implementer should verify the existing method signatures match before adding new ones. New methods to implement: `GetReleaseGateBySource`, `GetReleaseGate`, `UpsertVersionReadiness`, `OpenGate`, `MarkAgentTriggered`, `RecordGateEvent`, `ListExpiredGates`, `GetVersionReadiness`, `UpdateNLRulePassed`.

```go
package gate

import (
	"context"
	"encoding/json"

	"github.com/sentioxyz/changelogue/internal/models"
)

// GateStore is the data access interface for gate workers.
type GateStore interface {
	// GetReleaseGateBySource loads the release gate config for the project that
	// owns the given source. Returns nil, nil if no gate exists.
	GetReleaseGateBySource(ctx context.Context, sourceID string) (*models.ReleaseGate, error)

	// GetReleaseGate loads a release gate by project ID. Returns nil, nil if none.
	GetReleaseGate(ctx context.Context, projectID string) (*models.ReleaseGate, error)

	// UpsertVersionReadiness atomically adds a source to sources_met for the
	// given project+version. Returns the updated row and whether the gate just
	// became ready (all sources met). Only updates rows with status='pending'.
	UpsertVersionReadiness(ctx context.Context, projectID, version, sourceID string, requiredSources []string, timeoutHours int) (*models.VersionReadiness, bool, error)

	// OpenGate sets a version_readiness row's status to the given value (ready
	// or timed_out). Only transitions from 'pending'. Returns false if already
	// transitioned.
	OpenGate(ctx context.Context, readinessID, status string) (bool, error)

	// MarkAgentTriggered sets agent_triggered=true on the readiness row.
	MarkAgentTriggered(ctx context.Context, readinessID string) error

	// RecordGateEvent inserts a gate_events row.
	RecordGateEvent(ctx context.Context, readinessID, projectID, version, eventType string, sourceID *string, details json.RawMessage) error

	// ListExpiredGates returns version_readiness rows where status='pending'
	// and timeout_at < now(), locked with FOR UPDATE SKIP LOCKED, up to limit.
	ListExpiredGates(ctx context.Context, limit int) ([]models.VersionReadiness, error)

	// GetSource loads a source by ID.
	GetSource(ctx context.Context, id string) (*models.Source, error)

	// GetProject loads a project by ID.
	GetProject(ctx context.Context, id string) (*models.Project, error)

	// ListSourcesByProject returns all sources for a project.
	ListSourcesByProject(ctx context.Context, projectID string, page, perPage int) ([]models.Source, int, error)

	// EnqueueAgentRun creates an agent_run row and enqueues the River job.
	EnqueueAgentRun(ctx context.Context, projectID, trigger, version string) error

	// GetVersionReadiness loads a version_readiness row by ID.
	GetVersionReadiness(ctx context.Context, id string) (*models.VersionReadiness, error)

	// UpdateNLRulePassed sets nl_rule_passed on a version_readiness row.
	UpdateNLRulePassed(ctx context.Context, readinessID string, passed bool) error
}
```

- [ ] **Step 2: Verify compilation**

Run: `go vet ./internal/gate/...`
Expected: no errors

- [ ] **Step 3: Commit**

```bash
git add internal/gate/store.go
git commit -m "feat(gate): define GateStore interface"
```

---

## Task 6: GateCheckWorker

**Files:**
- Create: `internal/gate/worker.go`
- Create: `internal/gate/worker_test.go`

- [ ] **Step 1: Write failing tests for GateCheckWorker**

The tests should cover:
1. No gate exists → worker returns nil (no-op)
2. Gate exists, first source reports → readiness created as pending
3. Gate exists, all required sources met → gate opens, agent triggered
4. Gate exists, NL rule present, structured rules pass → NL eval job enqueued (agent NOT triggered)
5. Gate disabled → worker returns nil (no-op)
6. Duplicate source report → idempotent, no double agent trigger

```go
package gate

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/riverqueue/river"
	"github.com/sentioxyz/changelogue/internal/models"
	"github.com/sentioxyz/changelogue/internal/queue"
)

// mockGateStore implements GateStore for testing.
type mockGateStore struct {
	gate              *models.ReleaseGate
	readiness         *models.VersionReadiness
	gateOpened        bool
	agentTriggered    bool
	events            []mockGateEvent
	agentRunEnqueued  bool
	nlRuleUpdated     *bool
	expiredGates      []models.VersionReadiness

	// Control returns
	upsertReady bool
	openResult  bool
}

type mockGateEvent struct {
	eventType string
	sourceID  *string
}

func (m *mockGateStore) GetReleaseGateBySource(_ context.Context, _ string) (*models.ReleaseGate, error) {
	return m.gate, nil
}

func (m *mockGateStore) GetReleaseGate(_ context.Context, _ string) (*models.ReleaseGate, error) {
	return m.gate, nil
}

func (m *mockGateStore) UpsertVersionReadiness(_ context.Context, _, _, _ string, _ []string, _ int) (*models.VersionReadiness, bool, error) {
	if m.readiness == nil {
		m.readiness = &models.VersionReadiness{ID: "vr-1", Status: "pending"}
	}
	return m.readiness, m.upsertReady, nil
}

func (m *mockGateStore) OpenGate(_ context.Context, _, _ string) (bool, error) {
	m.gateOpened = true
	return m.openResult, nil
}

func (m *mockGateStore) MarkAgentTriggered(_ context.Context, _ string) error {
	m.agentTriggered = true
	return nil
}

func (m *mockGateStore) RecordGateEvent(_ context.Context, _, _, _, eventType string, sourceID *string, _ json.RawMessage) error {
	m.events = append(m.events, mockGateEvent{eventType: eventType, sourceID: sourceID})
	return nil
}

func (m *mockGateStore) ListExpiredGates(_ context.Context, _ int) ([]models.VersionReadiness, error) {
	return m.expiredGates, nil
}

func (m *mockGateStore) GetSource(_ context.Context, _ string) (*models.Source, error) {
	return &models.Source{ID: "src-1", ProjectID: "proj-1"}, nil
}

func (m *mockGateStore) GetProject(_ context.Context, _ string) (*models.Project, error) {
	return &models.Project{ID: "proj-1", AgentRules: json.RawMessage(`{"on_major_release":true}`)}, nil
}

func (m *mockGateStore) ListSourcesByProject(_ context.Context, _ string, _, _ int) ([]models.Source, int, error) {
	return []models.Source{{ID: "src-1"}, {ID: "src-2"}}, 2, nil
}

func (m *mockGateStore) EnqueueAgentRun(_ context.Context, _, _, _ string) error {
	m.agentRunEnqueued = true
	return nil
}

func (m *mockGateStore) GetVersionReadiness(_ context.Context, _ string) (*models.VersionReadiness, error) {
	return m.readiness, nil
}

func (m *mockGateStore) UpdateNLRulePassed(_ context.Context, _ string, passed bool) error {
	m.nlRuleUpdated = &passed
	return nil
}

func TestGateCheckWorker_NoGate(t *testing.T) {
	store := &mockGateStore{gate: nil}
	w := NewGateCheckWorker(store, nil)
	job := &river.Job[queue.GateCheckJobArgs]{
		Args: queue.GateCheckJobArgs{SourceID: "src-1", ReleaseID: "rel-1", Version: "v1.0.0"},
	}
	if err := w.Work(context.Background(), job); err != nil {
		t.Fatalf("expected nil, got %v", err)
	}
	if store.agentRunEnqueued {
		t.Fatal("agent should not be enqueued when no gate exists")
	}
}

func TestGateCheckWorker_GateOpens(t *testing.T) {
	store := &mockGateStore{
		gate: &models.ReleaseGate{
			ID:              "gate-1",
			ProjectID:       "proj-1",
			RequiredSources: []string{"src-1", "src-2"},
			TimeoutHours:    168,
			Enabled:         true,
		},
		upsertReady: true,
		openResult:  true,
	}
	w := NewGateCheckWorker(store, nil)
	job := &river.Job[queue.GateCheckJobArgs]{
		Args: queue.GateCheckJobArgs{SourceID: "src-1", ReleaseID: "rel-1", Version: "v1.0.0"},
	}
	if err := w.Work(context.Background(), job); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !store.gateOpened {
		t.Fatal("gate should have been opened")
	}
}

func TestGateCheckWorker_GateDisabled(t *testing.T) {
	store := &mockGateStore{
		gate: &models.ReleaseGate{
			ID:        "gate-1",
			ProjectID: "proj-1",
			Enabled:   false,
		},
	}
	w := NewGateCheckWorker(store, nil)
	job := &river.Job[queue.GateCheckJobArgs]{
		Args: queue.GateCheckJobArgs{SourceID: "src-1", ReleaseID: "rel-1", Version: "v1.0.0"},
	}
	if err := w.Work(context.Background(), job); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if store.agentRunEnqueued {
		t.Fatal("agent should not be enqueued when gate is disabled")
	}
}

func TestGateCheckWorker_PendingWaitsForMore(t *testing.T) {
	store := &mockGateStore{
		gate: &models.ReleaseGate{
			ID:              "gate-1",
			ProjectID:       "proj-1",
			RequiredSources: []string{"src-1", "src-2"},
			TimeoutHours:    168,
			Enabled:         true,
		},
		upsertReady: false, // not all sources met yet
	}
	w := NewGateCheckWorker(store, nil)
	job := &river.Job[queue.GateCheckJobArgs]{
		Args: queue.GateCheckJobArgs{SourceID: "src-1", ReleaseID: "rel-1", Version: "v1.0.0"},
	}
	if err := w.Work(context.Background(), job); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if store.gateOpened {
		t.Fatal("gate should NOT be opened when sources are still missing")
	}
	if store.agentRunEnqueued {
		t.Fatal("agent should NOT be enqueued when gate is still pending")
	}
}

func TestGateCheckWorker_NLRuleEnqueuesEval(t *testing.T) {
	store := &mockGateStore{
		gate: &models.ReleaseGate{
			ID:              "gate-1",
			ProjectID:       "proj-1",
			RequiredSources: []string{"src-1"},
			TimeoutHours:    168,
			NLRule:          "Docker image must have 100 pulls",
			Enabled:         true,
		},
		upsertReady: true, // structured rules pass
	}
	// Pass nil river client — the worker should NOT open the gate (NL rule pending).
	w := NewGateCheckWorker(store, nil)
	job := &river.Job[queue.GateCheckJobArgs]{
		Args: queue.GateCheckJobArgs{SourceID: "src-1", ReleaseID: "rel-1", Version: "v1.0.0"},
	}
	if err := w.Work(context.Background(), job); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if store.gateOpened {
		t.Fatal("gate should NOT be opened when NL rule is pending evaluation")
	}
	if store.agentRunEnqueued {
		t.Fatal("agent should NOT be enqueued when NL rule is pending")
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/gate/... -v -run TestGateCheckWorker`
Expected: FAIL — `NewGateCheckWorker` not defined

- [ ] **Step 3: Implement GateCheckWorker**

```go
package gate

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

	"github.com/jackc/pgx/v5"
	"github.com/riverqueue/river"
	"github.com/sentioxyz/changelogue/internal/models"
	"github.com/sentioxyz/changelogue/internal/queue"
)

// GateCheckWorker evaluates release gate readiness when a new release is ingested.
type GateCheckWorker struct {
	river.WorkerDefaults[queue.GateCheckJobArgs]
	store       GateStore
	riverClient *river.Client[pgx.Tx] // for enqueuing NL eval jobs
}

// NewGateCheckWorker creates a new GateCheckWorker.
func NewGateCheckWorker(store GateStore, riverClient *river.Client[pgx.Tx]) *GateCheckWorker {
	return &GateCheckWorker{store: store, riverClient: riverClient}
}

func (w *GateCheckWorker) Work(ctx context.Context, job *river.Job[queue.GateCheckJobArgs]) error {
	// Load gate config via source → project lookup.
	gate, err := w.store.GetReleaseGateBySource(ctx, job.Args.SourceID)
	if err != nil {
		return fmt.Errorf("get release gate: %w", err)
	}
	if gate == nil || !gate.Enabled {
		return nil // no gate — no-op
	}

	// Normalize the version.
	normalized := NormalizeVersionForSource(job.Args.Version, job.Args.SourceID, gate.VersionMapping)

	slog.Info("gate check: evaluating",
		"source_id", job.Args.SourceID,
		"raw_version", job.Args.Version,
		"normalized", normalized,
		"project_id", gate.ProjectID,
	)

	// Determine required sources: if gate.RequiredSources is empty, use all project sources.
	required := gate.RequiredSources
	if len(required) == 0 {
		sources, _, err := w.store.ListSourcesByProject(ctx, gate.ProjectID, 1, 1000)
		if err != nil {
			return fmt.Errorf("list sources: %w", err)
		}
		for _, s := range sources {
			required = append(required, s.ID)
		}
	}

	// Upsert version readiness (atomic).
	vr, allMet, err := w.store.UpsertVersionReadiness(ctx, gate.ProjectID, normalized, job.Args.SourceID, required, gate.TimeoutHours)
	if err != nil {
		return fmt.Errorf("upsert version readiness: %w", err)
	}

	// Record source_met event.
	sourceID := job.Args.SourceID
	details, _ := json.Marshal(map[string]interface{}{
		"raw_version":        job.Args.Version,
		"normalized_version": normalized,
		"sources_met":        len(vr.SourcesMet),
		"sources_required":   len(required),
	})
	_ = w.store.RecordGateEvent(ctx, vr.ID, gate.ProjectID, normalized, "source_met", &sourceID, details)

	if !allMet {
		slog.Info("gate check: waiting for more sources",
			"project_id", gate.ProjectID,
			"version", normalized,
			"met", len(vr.SourcesMet),
			"required", len(required),
		)
		return nil
	}

	// All structured rules pass. Check for NL rule.
	if gate.NLRule != "" {
		// Check if NL rule already passed from a previous evaluation.
		if vr.NLRulePassed != nil && *vr.NLRulePassed {
			// Already passed — open gate.
		} else {
			// Enqueue NL evaluation.
			if w.riverClient != nil {
				_, err := w.riverClient.Insert(ctx, queue.GateNLEvalJobArgs{
					VersionReadinessID: vr.ID,
					ProjectID:          gate.ProjectID,
					Version:            normalized,
				}, nil)
				if err != nil {
					slog.Error("gate check: failed to enqueue NL eval", "err", err)
				}
			}
			return nil // wait for NL eval
		}
	}

	// Open the gate.
	opened, err := w.store.OpenGate(ctx, vr.ID, "ready")
	if err != nil {
		return fmt.Errorf("open gate: %w", err)
	}
	if !opened {
		return nil // already opened by another worker
	}

	_ = w.store.RecordGateEvent(ctx, vr.ID, gate.ProjectID, normalized, "gate_opened", nil, nil)

	// Trigger agent.
	return w.triggerAgent(ctx, gate.ProjectID, normalized, vr.ID)
}

// triggerAgent checks agent rules and enqueues an agent run.
func (w *GateCheckWorker) triggerAgent(ctx context.Context, projectID, version, readinessID string) error {
	trigger := fmt.Sprintf("gate:version:%s", version)
	if err := w.store.EnqueueAgentRun(ctx, projectID, trigger, version); err != nil {
		slog.Error("gate: failed to enqueue agent run", "project_id", projectID, "version", version, "err", err)
		return nil // don't fail the gate job
	}

	_ = w.store.MarkAgentTriggered(ctx, readinessID)

	details, _ := json.Marshal(map[string]interface{}{"partial": false})
	_ = w.store.RecordGateEvent(ctx, readinessID, projectID, version, "agent_triggered", nil, details)

	slog.Info("gate: agent triggered", "project_id", projectID, "version", version)
	return nil
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/gate/... -v -run TestGateCheckWorker`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/gate/worker.go internal/gate/worker_test.go
git commit -m "feat(gate): implement GateCheckWorker with structured rule evaluation"
```

---

## Task 7: GateTimeoutWorker

**Files:**
- Create: `internal/gate/timeout_worker.go`
- Create: `internal/gate/timeout_worker_test.go`

- [ ] **Step 1: Write failing test**

```go
package gate

import (
	"context"
	"testing"
	"time"

	"github.com/riverqueue/river"
	"github.com/sentioxyz/changelogue/internal/models"
	"github.com/sentioxyz/changelogue/internal/queue"
)

func TestGateTimeoutWorker_SweepsExpired(t *testing.T) {
	store := &mockGateStore{
		expiredGates: []models.VersionReadiness{
			{ID: "vr-1", ProjectID: "proj-1", Version: "1.0.0", Status: "pending"},
		},
		openResult: true,
	}
	w := NewGateTimeoutWorker(store)
	job := &river.Job[queue.GateTimeoutJobArgs]{Args: queue.GateTimeoutJobArgs{}}
	if err := w.Work(context.Background(), job); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !store.gateOpened {
		t.Fatal("expired gate should have been opened")
	}
	// Check that gate_timed_out event was recorded
	found := false
	for _, e := range store.events {
		if e.eventType == "gate_timed_out" {
			found = true
		}
	}
	if !found {
		t.Fatal("gate_timed_out event should have been recorded")
	}
	if !store.agentRunEnqueued {
		t.Fatal("agent should have been enqueued after timeout")
	}
}

func TestGateTimeoutWorker_NoExpired(t *testing.T) {
	store := &mockGateStore{expiredGates: nil}
	w := NewGateTimeoutWorker(store)
	job := &river.Job[queue.GateTimeoutJobArgs]{Args: queue.GateTimeoutJobArgs{}}
	if err := w.Work(context.Background(), job); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if store.gateOpened {
		t.Fatal("no gates should have been opened")
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/gate/... -v -run TestGateTimeoutWorker`
Expected: FAIL — `NewGateTimeoutWorker` not defined

- [ ] **Step 3: Implement GateTimeoutWorker**

```go
package gate

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

	"github.com/riverqueue/river"
	"github.com/sentioxyz/changelogue/internal/queue"
)

// GateTimeoutWorker is a periodic River worker that sweeps expired pending gates.
type GateTimeoutWorker struct {
	river.WorkerDefaults[queue.GateTimeoutJobArgs]
	store GateStore
}

// NewGateTimeoutWorker creates a new GateTimeoutWorker.
func NewGateTimeoutWorker(store GateStore) *GateTimeoutWorker {
	return &GateTimeoutWorker{store: store}
}

func (w *GateTimeoutWorker) Work(ctx context.Context, _ *river.Job[queue.GateTimeoutJobArgs]) error {
	expired, err := w.store.ListExpiredGates(ctx, 100)
	if err != nil {
		return fmt.Errorf("list expired gates: %w", err)
	}

	for _, vr := range expired {
		opened, err := w.store.OpenGate(ctx, vr.ID, "timed_out")
		if err != nil {
			slog.Error("gate timeout: failed to open gate", "readiness_id", vr.ID, "err", err)
			continue
		}
		if !opened {
			continue // already opened
		}

		details, _ := json.Marshal(map[string]interface{}{
			"sources_missing": vr.SourcesMissing,
		})
		_ = w.store.RecordGateEvent(ctx, vr.ID, vr.ProjectID, vr.Version, "gate_timed_out", nil, details)

		slog.Info("gate timeout: gate force-opened",
			"project_id", vr.ProjectID,
			"version", vr.Version,
		)

		// Trigger agent with partial flag.
		trigger := fmt.Sprintf("gate:timeout:%s", vr.Version)
		if err := w.store.EnqueueAgentRun(ctx, vr.ProjectID, trigger, vr.Version); err != nil {
			slog.Error("gate timeout: failed to enqueue agent", "err", err)
			continue
		}
		_ = w.store.MarkAgentTriggered(ctx, vr.ID)

		agentDetails, _ := json.Marshal(map[string]interface{}{"partial": true})
		_ = w.store.RecordGateEvent(ctx, vr.ID, vr.ProjectID, vr.Version, "agent_triggered", nil, agentDetails)
	}

	return nil
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/gate/... -v -run TestGateTimeoutWorker`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/gate/timeout_worker.go internal/gate/timeout_worker_test.go
git commit -m "feat(gate): implement GateTimeoutWorker for expired gate sweep"
```

---

## Task 8: GateNLEvalWorker (Stub)

**Files:**
- Create: `internal/gate/nl_worker.go`
- Create: `internal/gate/nl_worker_test.go`

- [ ] **Step 1: Write failing test**

```go
package gate

import (
	"context"
	"testing"

	"github.com/riverqueue/river"
	"github.com/sentioxyz/changelogue/internal/models"
	"github.com/sentioxyz/changelogue/internal/queue"
)

func TestGateNLEvalWorker_Passes(t *testing.T) {
	store := &mockGateStore{
		gate: &models.ReleaseGate{
			ID:        "gate-1",
			ProjectID: "proj-1",
			NLRule:    "Docker image must have 100 pulls",
			Enabled:   true,
		},
		readiness:  &models.VersionReadiness{ID: "vr-1", ProjectID: "proj-1", Version: "1.0.0", Status: "pending"},
		openResult: true,
	}
	// Use a stub evaluator that always returns true.
	eval := &stubNLEvaluator{result: true}
	w := NewGateNLEvalWorker(store, eval)
	job := &river.Job[queue.GateNLEvalJobArgs]{
		Args: queue.GateNLEvalJobArgs{VersionReadinessID: "vr-1", ProjectID: "proj-1", Version: "1.0.0"},
	}
	if err := w.Work(context.Background(), job); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if store.nlRuleUpdated == nil || !*store.nlRuleUpdated {
		t.Fatal("nl_rule_passed should be true")
	}
	if !store.gateOpened {
		t.Fatal("gate should have been opened after NL rule passed")
	}
}

func TestGateNLEvalWorker_Fails(t *testing.T) {
	store := &mockGateStore{
		gate: &models.ReleaseGate{
			ID:        "gate-1",
			ProjectID: "proj-1",
			NLRule:    "Docker image must have 100 pulls",
			Enabled:   true,
		},
		readiness: &models.VersionReadiness{ID: "vr-1", ProjectID: "proj-1", Version: "1.0.0", Status: "pending"},
	}
	eval := &stubNLEvaluator{result: false}
	w := NewGateNLEvalWorker(store, eval)
	job := &river.Job[queue.GateNLEvalJobArgs]{
		Args: queue.GateNLEvalJobArgs{VersionReadinessID: "vr-1", ProjectID: "proj-1", Version: "1.0.0"},
	}
	if err := w.Work(context.Background(), job); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if store.nlRuleUpdated == nil || *store.nlRuleUpdated {
		t.Fatal("nl_rule_passed should be false")
	}
	if store.gateOpened {
		t.Fatal("gate should NOT be opened when NL rule fails")
	}
}

type stubNLEvaluator struct {
	result bool
}

func (s *stubNLEvaluator) Evaluate(_ context.Context, _ string, _ *models.VersionReadiness) (bool, string, error) {
	return s.result, "stub response", nil
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/gate/... -v -run TestGateNLEvalWorker`
Expected: FAIL — types not defined

- [ ] **Step 3: Implement GateNLEvalWorker with pluggable evaluator interface**

```go
package gate

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

	"github.com/riverqueue/river"
	"github.com/sentioxyz/changelogue/internal/models"
	"github.com/sentioxyz/changelogue/internal/queue"
)

// NLEvaluator evaluates a natural language gate rule. Returns (passed, reason, error).
type NLEvaluator interface {
	Evaluate(ctx context.Context, rule string, readiness *models.VersionReadiness) (bool, string, error)
}

// GateNLEvalWorker evaluates natural language gate rules via an LLM.
type GateNLEvalWorker struct {
	river.WorkerDefaults[queue.GateNLEvalJobArgs]
	store     GateStore
	evaluator NLEvaluator
}

// NewGateNLEvalWorker creates a new GateNLEvalWorker.
func NewGateNLEvalWorker(store GateStore, evaluator NLEvaluator) *GateNLEvalWorker {
	return &GateNLEvalWorker{store: store, evaluator: evaluator}
}

func (w *GateNLEvalWorker) Work(ctx context.Context, job *river.Job[queue.GateNLEvalJobArgs]) error {
	vr, err := w.store.GetVersionReadiness(ctx, job.Args.VersionReadinessID)
	if err != nil {
		return fmt.Errorf("get version readiness: %w", err)
	}
	if vr.Status != "pending" {
		return nil // already resolved
	}

	gate, err := w.store.GetReleaseGate(ctx, job.Args.ProjectID)
	if err != nil || gate == nil {
		return fmt.Errorf("get release gate: %w", err)
	}

	_ = w.store.RecordGateEvent(ctx, vr.ID, vr.ProjectID, vr.Version, "nl_eval_started", nil, nil)

	passed, reason, err := w.evaluator.Evaluate(ctx, gate.NLRule, vr)
	if err != nil {
		return fmt.Errorf("nl evaluation: %w", err) // River will retry
	}

	if err := w.store.UpdateNLRulePassed(ctx, vr.ID, passed); err != nil {
		return fmt.Errorf("update nl_rule_passed: %w", err)
	}

	eventType := "nl_eval_passed"
	if !passed {
		eventType = "nl_eval_failed"
	}
	details, _ := json.Marshal(map[string]interface{}{
		"rule":         gate.NLRule,
		"llm_response": reason,
	})
	_ = w.store.RecordGateEvent(ctx, vr.ID, vr.ProjectID, vr.Version, eventType, nil, details)

	if !passed {
		slog.Info("gate NL eval: rule not satisfied",
			"project_id", vr.ProjectID,
			"version", vr.Version,
			"reason", reason,
		)
		return nil // remain pending, will be re-evaluated on next source or timeout
	}

	// NL rule passed — open the gate.
	opened, err := w.store.OpenGate(ctx, vr.ID, "ready")
	if err != nil {
		return fmt.Errorf("open gate: %w", err)
	}
	if !opened {
		return nil
	}

	_ = w.store.RecordGateEvent(ctx, vr.ID, vr.ProjectID, vr.Version, "gate_opened", nil, nil)

	// Trigger agent.
	trigger := fmt.Sprintf("gate:version:%s", vr.Version)
	if err := w.store.EnqueueAgentRun(ctx, vr.ProjectID, trigger, vr.Version); err != nil {
		slog.Error("gate NL: failed to enqueue agent", "err", err)
		return nil
	}
	_ = w.store.MarkAgentTriggered(ctx, vr.ID)

	agentDetails, _ := json.Marshal(map[string]interface{}{"partial": false})
	_ = w.store.RecordGateEvent(ctx, vr.ID, vr.ProjectID, vr.Version, "agent_triggered", nil, agentDetails)

	return nil
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/gate/... -v -run TestGateNLEvalWorker`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/gate/nl_worker.go internal/gate/nl_worker_test.go
git commit -m "feat(gate): implement GateNLEvalWorker with pluggable evaluator"
```

---

## Task 9: PgStore — Gate Database Operations

**Files:**
- Modify: `internal/api/pgstore.go`

This task implements the database operations needed by the gate workers and API handlers. The methods should be added to the existing `PgStore` struct.

- [ ] **Step 1: Add GetReleaseGateBySource method**

Query: JOIN `sources` → `release_gates` via `project_id`. Return `nil, nil` if no row. Use `errors.Is(err, pgx.ErrNoRows)` for the no-row check (consistent with newer codebase patterns at `pgstore.go:1577`).

```go
func (s *PgStore) GetReleaseGateBySource(ctx context.Context, sourceID string) (*models.ReleaseGate, error) {
	var g models.ReleaseGate
	var requiredSources, versionMapping json.RawMessage
	err := s.pool.QueryRow(ctx, `
		SELECT rg.id, rg.project_id, rg.required_sources, rg.timeout_hours,
		       rg.version_mapping, rg.nl_rule, rg.enabled, rg.created_at, rg.updated_at
		FROM release_gates rg
		JOIN sources s ON s.project_id = rg.project_id
		WHERE s.id = $1
	`, sourceID).Scan(
		&g.ID, &g.ProjectID, &requiredSources, &g.TimeoutHours,
		&versionMapping, &g.NLRule, &g.Enabled, &g.CreatedAt, &g.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	if len(requiredSources) > 0 {
		json.Unmarshal(requiredSources, &g.RequiredSources)
	}
	if len(versionMapping) > 0 {
		json.Unmarshal(versionMapping, &g.VersionMapping)
	}
	return &g, nil
}
```

- [ ] **Step 2: Add GetReleaseGate by project ID**

```go
func (s *PgStore) GetReleaseGate(ctx context.Context, projectID string) (*models.ReleaseGate, error) {
	// Same as above but WHERE rg.project_id = $1
}
```

- [ ] **Step 3: Add UpsertVersionReadiness**

Uses `INSERT ... ON CONFLICT` with JSONB array append. The store method receives the full `requiredSources` list and computes `sources_missing` internally (as `required - met`). Returns `(row, allMet, error)` where `allMet` is true when `sources_missing` is empty after the upsert.

```go
func (s *PgStore) UpsertVersionReadiness(ctx context.Context, projectID, version, sourceID string, requiredSources []string, timeoutHours int) (*models.VersionReadiness, bool, error) {
	// 1. Build JSONB arrays for required and the new source.
	// 2. INSERT ... ON CONFLICT (project_id, version) DO UPDATE:
	//    - Append sourceID to sources_met if not already present (JSONB containment check)
	//    - Recompute sources_missing = requiredSources ∖ sources_met (done in SQL or Go)
	//    - Only update rows WHERE status = 'pending'
	// 3. RETURNING id, sources_met, sources_missing, status
	// 4. allMet = len(sources_missing) == 0
}
```

- [ ] **Step 4: Add OpenGate, MarkAgentTriggered, RecordGateEvent, ListExpiredGates**

Each is a straightforward SQL operation. `OpenGate` uses `UPDATE ... WHERE status = 'pending' RETURNING id` for atomicity. `ListExpiredGates` uses `SELECT ... FOR UPDATE SKIP LOCKED LIMIT $1`.

- [ ] **Step 5: Add CRUD operations for API (CreateReleaseGate, UpdateReleaseGate, DeleteReleaseGate, ListVersionReadiness, GetVersionReadiness, ListGateEvents)**

- [ ] **Step 6: Verify compilation**

Run: `go vet ./internal/api/...`
Expected: no errors

- [ ] **Step 7: Commit**

```bash
git add internal/api/pgstore.go
git commit -m "feat(gate): add PgStore gate database operations"
```

---

## Task 10: Modify IngestRelease — Enqueue GateCheckJob

**Files:**
- Modify: `internal/ingestion/pgstore.go` (lines 25-68)

- [ ] **Step 1: Add GateCheckJobArgs enqueue alongside NotifyJobArgs**

After line 62 (the `NotifyJobArgs` enqueue), add:

```go
_, err = s.river.InsertTx(ctx, tx, queue.GateCheckJobArgs{
    SourceID:  sourceID,
    ReleaseID: releaseID,
    Version:   result.RawVersion,
}, nil)
if err != nil {
    return fmt.Errorf("enqueue gate check: %w", err)
}
```

This is unconditional — the worker short-circuits for non-gated projects.

- [ ] **Step 2: Verify compilation**

Run: `go vet ./internal/ingestion/...`
Expected: no errors

- [ ] **Step 3: Commit**

```bash
git add internal/ingestion/pgstore.go
git commit -m "feat(gate): enqueue GateCheckJob unconditionally on release ingestion"
```

---

## Task 11: Modify NotifyWorker — Skip Agent Rules When Gate Exists

**Files:**
- Modify: `internal/routing/worker.go` (line 163)
- Modify: `internal/routing/worker.go` (NotifyStore interface, line 19)

- [ ] **Step 1: Add HasReleaseGate to NotifyStore interface**

Add to the `NotifyStore` interface at `internal/routing/worker.go:19`:

```go
HasReleaseGate(ctx context.Context, projectID string) (bool, error)
```

- [ ] **Step 2: Implement HasReleaseGate in PgStore**

Add to `internal/api/pgstore.go`:

```go
func (s *PgStore) HasReleaseGate(ctx context.Context, projectID string) (bool, error) {
	var exists bool
	err := s.pool.QueryRow(ctx,
		`SELECT EXISTS(SELECT 1 FROM release_gates WHERE project_id = $1 AND enabled = true)`,
		projectID,
	).Scan(&exists)
	return exists, err
}
```

- [ ] **Step 3: Modify checkAgentRules to skip when gate exists**

Insert after the `GetProject` error check in `checkAgentRules` at `internal/routing/worker.go:177` (after the `return` in the error block for `GetProject`):

```go
// If the project has an active release gate, skip agent rule checking here.
// The gate worker handles agent triggering.
hasGate, err := w.store.HasReleaseGate(ctx, source.ProjectID)
if err != nil {
    slog.Error("check release gate", "project_id", source.ProjectID, "err", err)
    return
}
if hasGate {
    slog.Debug("agent rules skipped — project has release gate", "project_id", source.ProjectID)
    return
}
```

- [ ] **Step 4: Update mock store in worker_test.go**

Add `HasReleaseGate` to the mock store in `internal/routing/worker_test.go`.

- [ ] **Step 5: Run existing tests**

Run: `go test ./internal/routing/... -v`
Expected: PASS (existing tests unaffected — mock returns `false` for HasReleaseGate)

- [ ] **Step 6: Commit**

```bash
git add internal/routing/worker.go internal/routing/worker_test.go internal/api/pgstore.go
git commit -m "feat(gate): skip agent rule check when project has release gate"
```

---

## Task 12: API Handlers — Gate Config + Version Readiness + Events

**Files:**
- Create: `internal/api/gates.go`
- Modify: `internal/api/server.go`

- [ ] **Step 1: Define GateStore interface in gates.go**

```go
type GatesStore interface {
	GetReleaseGate(ctx context.Context, projectID string) (*models.ReleaseGate, error)
	CreateReleaseGate(ctx context.Context, g *models.ReleaseGate) error
	UpdateReleaseGate(ctx context.Context, g *models.ReleaseGate) error
	DeleteReleaseGate(ctx context.Context, projectID string) error
	ListVersionReadiness(ctx context.Context, projectID string, page, perPage int) ([]models.VersionReadiness, int, error)
	GetVersionReadinessByVersion(ctx context.Context, projectID, version string) (*models.VersionReadiness, error)
	ListGateEvents(ctx context.Context, projectID string, page, perPage int) ([]models.GateEvent, int, error)
	ListGateEventsByVersion(ctx context.Context, projectID, version string, page, perPage int) ([]models.GateEvent, int, error)
}
```

- [ ] **Step 2: Implement handler methods**

Follow the existing handler pattern from `internal/api/projects.go`:
- `GatesHandler` struct with `store GatesStore`
- `GetGate` — GET `/api/v1/projects/{id}/release-gate`
- `UpsertGate` — PUT `/api/v1/projects/{id}/release-gate`
- `DeleteGate` — DELETE `/api/v1/projects/{id}/release-gate`
- `ListReadiness` — GET `/api/v1/projects/{id}/version-readiness`
- `GetReadiness` — GET `/api/v1/projects/{id}/version-readiness/{version}`
- `ListEvents` — GET `/api/v1/projects/{id}/gate-events`
- `ListEventsByVersion` — GET `/api/v1/projects/{id}/version-readiness/{version}/events`

- [ ] **Step 3: Register routes in server.go**

Add `GatesStore` to `Dependencies` struct and register routes in `RegisterRoutes`:

```go
// Release Gates
gates := NewGatesHandler(deps.GatesStore)
mux.Handle("GET /api/v1/projects/{id}/release-gate", chain(http.HandlerFunc(gates.GetGate)))
mux.Handle("PUT /api/v1/projects/{id}/release-gate", chain(http.HandlerFunc(gates.UpsertGate)))
mux.Handle("DELETE /api/v1/projects/{id}/release-gate", chain(http.HandlerFunc(gates.DeleteGate)))
mux.Handle("GET /api/v1/projects/{id}/version-readiness", chain(http.HandlerFunc(gates.ListReadiness)))
mux.Handle("GET /api/v1/projects/{id}/version-readiness/{version}", chain(http.HandlerFunc(gates.GetReadiness)))
mux.Handle("GET /api/v1/projects/{id}/version-readiness/{version}/events", chain(http.HandlerFunc(gates.ListEventsByVersion)))
mux.Handle("GET /api/v1/projects/{id}/gate-events", chain(http.HandlerFunc(gates.ListEvents)))
```

- [ ] **Step 4: Wire GatesStore in main.go**

Add `GatesStore: pgStore` to the `Dependencies` struct in `cmd/server/main.go:179`.

- [ ] **Step 5: Verify compilation**

Run: `go vet ./internal/api/... && go vet ./cmd/server/...`
Expected: no errors

- [ ] **Step 6: Commit**

```bash
git add internal/api/gates.go internal/api/server.go cmd/server/main.go
git commit -m "feat(gate): add API handlers for gate config, version readiness, and events"
```

---

## Task 13: Register Workers + Periodic Job in main.go

**Files:**
- Modify: `cmd/server/main.go`
- Modify: `internal/queue/client.go`

- [ ] **Step 1: Update NewRiverClient to accept periodic jobs**

Modify `internal/queue/client.go` to accept `PeriodicJobs`:

```go
func NewRiverClient(pool *pgxpool.Pool, workers *river.Workers, periodicJobs ...*river.PeriodicJob) (*river.Client[pgx.Tx], error) {
	config := &river.Config{}
	if workers != nil {
		config.Workers = workers
		config.Queues = map[string]river.QueueConfig{
			river.QueueDefault: {MaxWorkers: 100},
		}
		if len(periodicJobs) > 0 {
			config.PeriodicJobs = periodicJobs
		}
	}
	return river.NewClient(riverpgxv5.New(pool), config)
}
```

- [ ] **Step 2: Register gate workers in main.go**

After the scan worker registration (line 144), add:

```go
// Gate workers
gatepkg "github.com/sentioxyz/changelogue/internal/gate"

gateCheckWorker := gatepkg.NewGateCheckWorker(pgStore, nil) // river client set later
river.AddWorker(workers, gateCheckWorker)

gateTimeoutWorker := gatepkg.NewGateTimeoutWorker(pgStore)
river.AddWorker(workers, gateTimeoutWorker)

// NL eval worker (stub evaluator for now — replace with LLM evaluator when ready)
// Only register if LLM is available
if agentOrchestrator != nil {
    gateNLWorker := gatepkg.NewGateNLEvalWorker(pgStore, nil) // TODO: LLM evaluator
    river.AddWorker(workers, gateNLWorker)
}
slog.Info("gate workers registered")
```

- [ ] **Step 3: Add periodic timeout job**

Pass the periodic job to `NewRiverClient`:

```go
timeoutPeriodic := river.NewPeriodicJob(
    river.PeriodicInterval(15 * time.Minute),
    func() (river.JobArgs, *river.InsertOpts) {
        return queue.GateTimeoutJobArgs{}, nil
    },
    &river.PeriodicJobOpts{RunOnStart: true},
)
riverClient, err := queue.NewRiverClient(pool, workers, timeoutPeriodic)
```

- [ ] **Step 4: Set river client on gate check worker after creation**

After `pgStore.SetRiverClient(riverClient)` (line 154), set it on the gate worker too:

```go
gateCheckWorker.SetRiverClient(riverClient)
```

Add the corresponding method to `GateCheckWorker`:

```go
func (w *GateCheckWorker) SetRiverClient(c *river.Client[pgx.Tx]) {
    w.riverClient = c
}
```

- [ ] **Step 5: Verify compilation and startup**

Run: `go build ./cmd/server`
Expected: compiles without error

- [ ] **Step 6: Commit**

```bash
git add cmd/server/main.go internal/queue/client.go internal/gate/worker.go
git commit -m "feat(gate): register gate workers and periodic timeout job"
```

---

## Task 14: Enriched Semantic Notifications

**Files:**
- Modify: `internal/agent/orchestrator.go:236-266` — `RunAgent` method
- Modify: `internal/agent/orchestrator.go:32-43` — `OrchestratorStore` interface
- Modify: `internal/api/pgstore.go` — add `GetVersionReadinessByVersion`

- [ ] **Step 1: Add GetVersionReadinessByVersion to OrchestratorStore interface**

At `internal/agent/orchestrator.go:32-43`, add to the `OrchestratorStore` interface:

```go
GetVersionReadinessByVersion(ctx context.Context, projectID, version string) (*models.VersionReadiness, error)
```

- [ ] **Step 2: Implement GetVersionReadinessByVersion in PgStore**

Add to `internal/api/pgstore.go`:

```go
func (s *PgStore) GetVersionReadinessByVersion(ctx context.Context, projectID, version string) (*models.VersionReadiness, error) {
	var vr models.VersionReadiness
	var sourcesMet, sourcesMissing json.RawMessage
	err := s.pool.QueryRow(ctx, `
		SELECT id, project_id, version, status, sources_met, sources_missing,
		       nl_rule_passed, timeout_at, opened_at, agent_triggered, created_at, updated_at
		FROM version_readiness
		WHERE project_id = $1 AND version = $2
	`, projectID, version).Scan(
		&vr.ID, &vr.ProjectID, &vr.Version, &vr.Status, &sourcesMet, &sourcesMissing,
		&vr.NLRulePassed, &vr.TimeoutAt, &vr.OpenedAt, &vr.AgentTriggered, &vr.CreatedAt, &vr.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	json.Unmarshal(sourcesMet, &vr.SourcesMet)
	json.Unmarshal(sourcesMissing, &vr.SourcesMissing)
	return &vr, nil
}
```

- [ ] **Step 3: Build source status lines helper**

Add a helper function in `internal/agent/orchestrator.go`:

```go
// buildSourceStatusLines returns lines like "GitHub ✓, Docker Hub ✗ (not yet available)"
// for inclusion in semantic release notifications.
func (o *Orchestrator) buildSourceStatusLines(ctx context.Context, projectID, version string) string {
	vr, err := o.store.GetVersionReadinessByVersion(ctx, projectID, version)
	if err != nil || vr == nil {
		return "" // no gate data — skip enrichment
	}

	sources, _, _ := o.store.ListSourcesByProject(ctx, projectID, 1, 1000)
	sourceNames := make(map[string]string) // id → "Provider/Repo"
	for _, s := range sources {
		sourceNames[s.ID] = fmt.Sprintf("%s/%s", s.Provider, s.Repository)
	}

	metSet := make(map[string]bool)
	for _, id := range vr.SourcesMet {
		metSet[id] = true
	}

	var parts []string
	for _, s := range sources {
		name := sourceNames[s.ID]
		if metSet[s.ID] {
			parts = append(parts, name+" ✓")
		} else {
			parts = append(parts, name+" ✗ (not yet available)")
		}
	}
	return "Sources: " + strings.Join(parts, ", ")
}
```

- [ ] **Step 4: Call buildSourceStatusLines when sending project notifications**

In `RunAgent` at `internal/agent/orchestrator.go:263` (where `sendProjectNotifications` is called), pass the source status string. Append it to the `Notification.Body` before sending:

```go
sourceStatus := o.buildSourceStatusLines(ctx, run.ProjectID, run.Version)
// When building Notification, append sourceStatus to Body if non-empty
```

- [ ] **Step 5: Update mock in agent tests**

Add `GetVersionReadinessByVersion` to the agent test mock store (return nil, nil by default so existing tests pass unchanged).

- [ ] **Step 6: Verify existing tests still pass**

Run: `go test ./internal/agent/... -v`
Expected: PASS

- [ ] **Step 7: Commit**

```bash
git add internal/agent/orchestrator.go internal/api/pgstore.go
git commit -m "feat(gate): enrich semantic notifications with source availability status"
```

---

## Task 15: WaitForAllSources Deprecation Migration

**Files:**
- Modify: `internal/db/migrations.go`

- [ ] **Step 1: Add one-time migration for existing WaitForAllSources projects**

Append to `RunMigrations` in `internal/db/migrations.go`, after the existing migration blocks:

```go
// Auto-create release_gates for projects with WaitForAllSources enabled.
if _, err := pool.Exec(ctx, `
    INSERT INTO release_gates (project_id, timeout_hours, enabled)
    SELECT p.id, 168, true
    FROM projects p
    WHERE p.agent_rules->>'wait_for_all_sources' = 'true'
    ON CONFLICT (project_id) DO NOTHING
`); err != nil {
    return fmt.Errorf("wait_for_all_sources migration: %w", err)
}
```

The gate's `required_sources` is left null (= all sources), and `timeout_hours` defaults to 168 (7 days). The existing `WaitForAllSources` snooze logic in `internal/agent/worker.go:83-118` continues to function as a fallback — it only triggers when no `release_gate` exists.

- [ ] **Step 2: Verify migration compiles**

Run: `go vet ./internal/db/...`
Expected: no errors

- [ ] **Step 3: Commit**

```bash
git add internal/db/migrations.go
git commit -m "feat(gate): auto-migrate WaitForAllSources projects to release gates"
```

---

## Task 16: Update DESIGN.md

**Files:**
- Modify: `DESIGN.md`

- [ ] **Step 1: Update subscription type names**

Find references to `type = 'source'` / `type = 'project'` and update to `'source_release'` / `'semantic_release'` to match the actual migration.

- [ ] **Step 2: Add Release Gate section**

Add a section describing the release gate feature: tables, flow, and how it interacts with the existing notification and agent systems.

- [ ] **Step 3: Commit**

```bash
git add DESIGN.md
git commit -m "docs: update DESIGN.md with release gate and correct subscription types"
```

---

## Task 17: Integration Smoke Test

**Files:**
- No new files — uses existing test infrastructure

- [ ] **Step 1: Verify full build**

Run: `go build ./cmd/server && go vet ./...`
Expected: clean compile and vet

- [ ] **Step 2: Run all tests**

Run: `go test ./...`
Expected: all pass

- [ ] **Step 3: Manual smoke test with local Postgres**

Run: `make dev`

1. Create a project via API
2. Add two sources (e.g., GitHub + Docker Hub)
3. Create a release gate via `PUT /api/v1/projects/{id}/release-gate`
4. Simulate a release ingestion for one source
5. Verify `version_readiness` shows `pending` status
6. Verify `gate_events` shows `source_met`
7. Simulate release from second source
8. Verify gate opens and agent is triggered

- [ ] **Step 4: Commit any fixes**

```bash
git add -A
git commit -m "fix(gate): integration smoke test fixes"
```
