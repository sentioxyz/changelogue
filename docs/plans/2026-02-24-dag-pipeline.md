# Configurable Pipeline Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Build the configurable processing pipeline that consumes River jobs, runs release events through sequential processing nodes (version normalization, subscription routing, urgency scoring), tracks progress in a `pipeline_jobs` table, and is wired into `main.go` as a River worker.

**Architecture:** River workers pull `pipeline_process` jobs. A `Runner` orchestrates sequential execution: always-on nodes (Regex Normalizer, Subscription Router) run for every release; configurable nodes (Urgency Scorer, etc.) run only when their key is present in `pipeline_config`. Each node implements the `PipelineNode` interface, receives prior results and config as raw JSON. A `pipeline_jobs` table tracks `current_node`, `node_results`, and `state` for dashboard observability. Without the `projects` table (not yet built), configurable nodes are dormant — only always-on nodes execute. Execution is sequential — nodes that are independent (e.g., Availability Checker, Risk Assessor) could be parallelized in a future DAG-based runner without changing the node interface.

**Tech Stack:** Go 1.25, PostgreSQL (pgx/v5), River v0.31.0, regexp, encoding/json

---

## Task 1: Schema — Add Pipeline Jobs Table

**Files:**
- Modify: `internal/db/migrations.go`

**Step 1: Add pipeline_jobs table to the schema**

Add the `pipeline_jobs` CREATE TABLE to the existing `schema` constant in `internal/db/migrations.go`, after the existing `releases` and `subscriptions` tables. This table tracks pipeline execution progress for dashboard observability:

```sql
CREATE TABLE IF NOT EXISTS pipeline_jobs (
    id BIGSERIAL PRIMARY KEY,
    release_id UUID NOT NULL REFERENCES releases(id),
    state VARCHAR(50) NOT NULL DEFAULT 'running',
    current_node VARCHAR(50),
    node_results JSONB NOT NULL DEFAULT '{}',
    error_message TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    completed_at TIMESTAMPTZ,
    UNIQUE(release_id)
);
```

The full `schema` const should be:

```go
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

CREATE TABLE IF NOT EXISTS pipeline_jobs (
    id BIGSERIAL PRIMARY KEY,
    release_id UUID NOT NULL REFERENCES releases(id),
    state VARCHAR(50) NOT NULL DEFAULT 'running',
    current_node VARCHAR(50),
    node_results JSONB NOT NULL DEFAULT '{}',
    error_message TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    completed_at TIMESTAMPTZ,
    UNIQUE(release_id)
);
`
```

**Step 2: Verify build**

Run: `go build ./internal/db/`
Expected: Success

**Step 3: Commit**

```bash
git add internal/db/migrations.go
git commit -m "feat: add pipeline_jobs table for pipeline progress tracking"
```

---

## Task 2: Pipeline Node Interface

**Files:**
- Create: `internal/pipeline/node.go`
- Test: `internal/pipeline/node_test.go`

**Step 1: Write the failing test**

```go
// internal/pipeline/node_test.go
package pipeline

import (
	"errors"
	"testing"
)

func TestErrEventDropped(t *testing.T) {
	if !errors.Is(ErrEventDropped, ErrEventDropped) {
		t.Error("ErrEventDropped should be identifiable with errors.Is")
	}
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/pipeline/ -v`
Expected: FAIL — package does not exist

**Step 3: Write the interface and sentinel error**

```go
// internal/pipeline/node.go
package pipeline

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/sentioxyz/releaseguard/internal/models"
)

// ErrEventDropped signals that a node has decided to drop the event.
// The runner marks the pipeline job as "skipped" and stops processing.
var ErrEventDropped = errors.New("event dropped by pipeline node")

// PipelineNode is the interface all pipeline processing nodes implement.
// See DESIGN.md Section 2.2 for the canonical definition.
type PipelineNode interface {
	// Name returns the node identifier (e.g., "regex_normalizer").
	Name() string
	// Execute processes the event and returns a JSON result.
	// config is the node's config from pipeline_config (nil for always-on nodes).
	// prior contains results from previously executed nodes, keyed by node name.
	Execute(ctx context.Context, event *models.ReleaseEvent, config json.RawMessage, prior map[string]json.RawMessage) (json.RawMessage, error)
}
```

**Step 4: Run test to verify it passes**

Run: `go test ./internal/pipeline/ -v`
Expected: PASS

**Step 5: Commit**

```bash
git add internal/pipeline/node.go internal/pipeline/node_test.go
git commit -m "feat: add PipelineNode interface and ErrEventDropped sentinel"
```

---

## Task 3: Pipeline Store Interface + PostgreSQL Implementation

**Files:**
- Create: `internal/pipeline/store.go`
- Create: `internal/pipeline/pgstore.go`

No unit tests — this is a thin infrastructure adapter (same pattern as `internal/ingestion/pgstore.go`). Covered by integration tests.

**Step 1: Define the store interface**

```go
// internal/pipeline/store.go
package pipeline

import (
	"context"
	"encoding/json"

	"github.com/sentioxyz/releaseguard/internal/models"
)

// Store provides data access for the pipeline runner.
type Store interface {
	GetReleasePayload(ctx context.Context, releaseID string) (*models.ReleaseEvent, error)
	UpsertPipelineJob(ctx context.Context, releaseID string) (int64, error)
	UpdateNodeProgress(ctx context.Context, jobID int64, currentNode string, nodeResults map[string]json.RawMessage) error
	CompletePipelineJob(ctx context.Context, jobID int64, nodeResults map[string]json.RawMessage) error
	SkipPipelineJob(ctx context.Context, jobID int64, reason string) error
	FailPipelineJob(ctx context.Context, jobID int64, errMsg string) error
}
```

**Step 2: Implement PgStore**

```go
// internal/pipeline/pgstore.go
package pipeline

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/sentioxyz/releaseguard/internal/models"
)

// PgStore implements Store and SubscriptionChecker using PostgreSQL.
type PgStore struct {
	pool *pgxpool.Pool
}

func NewPgStore(pool *pgxpool.Pool) *PgStore {
	return &PgStore{pool: pool}
}

func (s *PgStore) GetReleasePayload(ctx context.Context, releaseID string) (*models.ReleaseEvent, error) {
	var payload []byte
	err := s.pool.QueryRow(ctx, `SELECT payload FROM releases WHERE id = $1`, releaseID).Scan(&payload)
	if err != nil {
		return nil, fmt.Errorf("get release: %w", err)
	}
	var event models.ReleaseEvent
	if err := json.Unmarshal(payload, &event); err != nil {
		return nil, fmt.Errorf("unmarshal payload: %w", err)
	}
	return &event, nil
}

func (s *PgStore) UpsertPipelineJob(ctx context.Context, releaseID string) (int64, error) {
	var id int64
	err := s.pool.QueryRow(ctx, `
		INSERT INTO pipeline_jobs (release_id, state)
		VALUES ($1, 'running')
		ON CONFLICT (release_id) DO UPDATE SET
			state = 'running',
			current_node = NULL,
			node_results = '{}',
			error_message = NULL,
			completed_at = NULL
		RETURNING id
	`, releaseID).Scan(&id)
	if err != nil {
		return 0, fmt.Errorf("upsert pipeline job: %w", err)
	}
	return id, nil
}

func (s *PgStore) UpdateNodeProgress(ctx context.Context, jobID int64, currentNode string, nodeResults map[string]json.RawMessage) error {
	resultsJSON, err := json.Marshal(nodeResults)
	if err != nil {
		return fmt.Errorf("marshal results: %w", err)
	}
	_, err = s.pool.Exec(ctx,
		`UPDATE pipeline_jobs SET current_node = $2, node_results = $3 WHERE id = $1`,
		jobID, currentNode, resultsJSON,
	)
	return err
}

func (s *PgStore) CompletePipelineJob(ctx context.Context, jobID int64, nodeResults map[string]json.RawMessage) error {
	resultsJSON, err := json.Marshal(nodeResults)
	if err != nil {
		return fmt.Errorf("marshal results: %w", err)
	}
	_, err = s.pool.Exec(ctx,
		`UPDATE pipeline_jobs SET state = 'completed', current_node = NULL, node_results = $2, completed_at = NOW() WHERE id = $1`,
		jobID, resultsJSON,
	)
	return err
}

func (s *PgStore) SkipPipelineJob(ctx context.Context, jobID int64, reason string) error {
	_, err := s.pool.Exec(ctx,
		`UPDATE pipeline_jobs SET state = 'skipped', error_message = $2, completed_at = NOW() WHERE id = $1`,
		jobID, reason,
	)
	return err
}

func (s *PgStore) FailPipelineJob(ctx context.Context, jobID int64, errMsg string) error {
	_, err := s.pool.Exec(ctx,
		`UPDATE pipeline_jobs SET state = 'failed', error_message = $2, completed_at = NOW() WHERE id = $1`,
		jobID, errMsg,
	)
	return err
}

// HasSubscribers returns true if the repository has active subscriptions,
// or if no subscriptions exist at all (default open — system unconfigured).
func (s *PgStore) HasSubscribers(ctx context.Context, repository string) (bool, error) {
	var result bool
	err := s.pool.QueryRow(ctx, `
		SELECT EXISTS(SELECT 1 FROM subscriptions WHERE repository = $1)
			OR NOT EXISTS(SELECT 1 FROM subscriptions)
	`, repository).Scan(&result)
	if err != nil {
		return false, fmt.Errorf("check subscribers: %w", err)
	}
	return result, nil
}
```

**Step 3: Verify build**

Run: `go build ./internal/pipeline/`
Expected: Success

**Step 4: Commit**

```bash
git add internal/pipeline/store.go internal/pipeline/pgstore.go
git commit -m "feat: add pipeline store interface and PostgreSQL implementation"
```

---

## Task 4: Regex Normalizer Node

**Files:**
- Create: `internal/pipeline/regex_normalizer.go`
- Test: `internal/pipeline/regex_normalizer_test.go`

The Regex Normalizer is the first always-on node. It parses `RawVersion` into `SemanticData`, sets `IsPreRelease`, and mutates the event for downstream nodes.

**Step 1: Write the failing test**

```go
// internal/pipeline/regex_normalizer_test.go
package pipeline

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/sentioxyz/releaseguard/internal/models"
)

func TestRegexNormalizerName(t *testing.T) {
	n := NewRegexNormalizer()
	if got := n.Name(); got != "regex_normalizer" {
		t.Errorf("Name() = %q, want %q", got, "regex_normalizer")
	}
}

func TestRegexNormalizerParsesVersions(t *testing.T) {
	tests := []struct {
		name       string
		rawVersion string
		wantMajor  int
		wantMinor  int
		wantPatch  int
		wantPre    string
		wantPreRel bool
		wantParsed bool
	}{
		{"basic", "1.21.0", 1, 21, 0, "", false, true},
		{"v-prefix", "v1.21.0", 1, 21, 0, "", false, true},
		{"prerelease", "v2.0.0-rc.1", 2, 0, 0, "rc.1", true, true},
		{"beta", "1.0.0-beta.3", 1, 0, 0, "beta.3", true, true},
		{"two-part", "1.21", 1, 21, 0, "", false, true},
		{"major-only", "v3", 3, 0, 0, "", false, true},
		{"unparseable", "latest", 0, 0, 0, "", false, false},
		{"date-version", "20240115", 20240115, 0, 0, "", false, true},
	}

	n := NewRegexNormalizer()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			event := &models.ReleaseEvent{
				ID:         "test-id",
				RawVersion: tt.rawVersion,
				Timestamp:  time.Now(),
			}

			result, err := n.Execute(context.Background(), event, nil, nil)
			if err != nil {
				t.Fatalf("Execute: %v", err)
			}

			var res RegexNormalizerResult
			if err := json.Unmarshal(result, &res); err != nil {
				t.Fatalf("unmarshal: %v", err)
			}

			if res.Parsed != tt.wantParsed {
				t.Errorf("Parsed = %v, want %v", res.Parsed, tt.wantParsed)
			}
			if res.SemanticVersion.Major != tt.wantMajor {
				t.Errorf("Major = %d, want %d", res.SemanticVersion.Major, tt.wantMajor)
			}
			if res.SemanticVersion.Minor != tt.wantMinor {
				t.Errorf("Minor = %d, want %d", res.SemanticVersion.Minor, tt.wantMinor)
			}
			if res.SemanticVersion.Patch != tt.wantPatch {
				t.Errorf("Patch = %d, want %d", res.SemanticVersion.Patch, tt.wantPatch)
			}
			if res.SemanticVersion.PreRelease != tt.wantPre {
				t.Errorf("PreRelease = %q, want %q", res.SemanticVersion.PreRelease, tt.wantPre)
			}
			if res.IsPreRelease != tt.wantPreRel {
				t.Errorf("IsPreRelease = %v, want %v", res.IsPreRelease, tt.wantPreRel)
			}

			// Verify event was mutated for downstream nodes
			if event.SemanticVersion.Major != tt.wantMajor {
				t.Errorf("event.SemanticVersion.Major = %d, want %d", event.SemanticVersion.Major, tt.wantMajor)
			}
			if event.IsPreRelease != tt.wantPreRel {
				t.Errorf("event.IsPreRelease = %v, want %v", event.IsPreRelease, tt.wantPreRel)
			}
		})
	}
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/pipeline/ -v -run TestRegexNormalizer`
Expected: FAIL — `NewRegexNormalizer` not defined

**Step 3: Implement the Regex Normalizer**

```go
// internal/pipeline/regex_normalizer.go
package pipeline

import (
	"context"
	"encoding/json"
	"regexp"
	"strconv"

	"github.com/sentioxyz/releaseguard/internal/models"
)

var semverRegex = regexp.MustCompile(`^v?(\d+)(?:\.(\d+))?(?:\.(\d+))?(?:[+-]([\w.]+))?$`)

// RegexNormalizerResult is the output of the Regex Normalizer node.
type RegexNormalizerResult struct {
	SemanticVersion models.SemanticData `json:"semantic_version"`
	IsPreRelease    bool                `json:"is_pre_release"`
	Parsed          bool                `json:"parsed"`
}

// RegexNormalizer parses RawVersion into SemanticData and detects pre-releases.
// Always-on node — runs for every release regardless of pipeline_config.
type RegexNormalizer struct{}

func NewRegexNormalizer() *RegexNormalizer { return &RegexNormalizer{} }

func (n *RegexNormalizer) Name() string { return "regex_normalizer" }

func (n *RegexNormalizer) Execute(_ context.Context, event *models.ReleaseEvent, _ json.RawMessage, _ map[string]json.RawMessage) (json.RawMessage, error) {
	result := RegexNormalizerResult{}

	matches := semverRegex.FindStringSubmatch(event.RawVersion)
	if matches != nil {
		result.Parsed = true
		result.SemanticVersion.Major, _ = strconv.Atoi(matches[1])
		if matches[2] != "" {
			result.SemanticVersion.Minor, _ = strconv.Atoi(matches[2])
		}
		if matches[3] != "" {
			result.SemanticVersion.Patch, _ = strconv.Atoi(matches[3])
		}
		if matches[4] != "" {
			result.SemanticVersion.PreRelease = matches[4]
			result.IsPreRelease = true
		}
	}

	// Mutate event for downstream nodes
	event.SemanticVersion = result.SemanticVersion
	event.IsPreRelease = result.IsPreRelease

	return json.Marshal(result)
}

var _ PipelineNode = (*RegexNormalizer)(nil)
```

**Step 4: Run test to verify it passes**

Run: `go test ./internal/pipeline/ -v -run TestRegexNormalizer`
Expected: PASS

**Step 5: Commit**

```bash
git add internal/pipeline/regex_normalizer.go internal/pipeline/regex_normalizer_test.go
git commit -m "feat: add Regex Normalizer pipeline node"
```

---

## Task 5: Subscription Router Node

**Files:**
- Create: `internal/pipeline/subscription_router.go`
- Test: `internal/pipeline/subscription_router_test.go`

The Subscription Router is the second always-on node. It checks whether any subscriptions exist for the release's repository. If none match (and the system has subscriptions configured), it drops the event via `ErrEventDropped`.

**Step 1: Write the failing test**

```go
// internal/pipeline/subscription_router_test.go
package pipeline

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/sentioxyz/releaseguard/internal/models"
)

type mockSubscriptionChecker struct {
	hasSubscribers bool
	err            error
}

func (m *mockSubscriptionChecker) HasSubscribers(_ context.Context, _ string) (bool, error) {
	return m.hasSubscribers, m.err
}

func TestSubscriptionRouterName(t *testing.T) {
	n := NewSubscriptionRouter(&mockSubscriptionChecker{})
	if got := n.Name(); got != "subscription_router" {
		t.Errorf("Name() = %q, want %q", got, "subscription_router")
	}
}

func TestSubscriptionRouterWithSubscribers(t *testing.T) {
	checker := &mockSubscriptionChecker{hasSubscribers: true}
	n := NewSubscriptionRouter(checker)

	event := &models.ReleaseEvent{Repository: "library/golang", Timestamp: time.Now()}

	result, err := n.Execute(context.Background(), event, nil, nil)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}

	var res SubscriptionRouterResult
	if err := json.Unmarshal(result, &res); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if !res.Routed {
		t.Error("expected Routed=true")
	}
}

func TestSubscriptionRouterNoSubscribers(t *testing.T) {
	checker := &mockSubscriptionChecker{hasSubscribers: false}
	n := NewSubscriptionRouter(checker)

	event := &models.ReleaseEvent{Repository: "library/golang", Timestamp: time.Now()}

	_, err := n.Execute(context.Background(), event, nil, nil)
	if !errors.Is(err, ErrEventDropped) {
		t.Errorf("expected ErrEventDropped, got: %v", err)
	}
}

func TestSubscriptionRouterCheckerError(t *testing.T) {
	checker := &mockSubscriptionChecker{err: errors.New("db error")}
	n := NewSubscriptionRouter(checker)

	event := &models.ReleaseEvent{Repository: "library/golang", Timestamp: time.Now()}

	_, err := n.Execute(context.Background(), event, nil, nil)
	if err == nil {
		t.Fatal("expected error")
	}
	if errors.Is(err, ErrEventDropped) {
		t.Error("db error should not be ErrEventDropped")
	}
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/pipeline/ -v -run TestSubscriptionRouter`
Expected: FAIL — `NewSubscriptionRouter` not defined

**Step 3: Implement the Subscription Router**

```go
// internal/pipeline/subscription_router.go
package pipeline

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/sentioxyz/releaseguard/internal/models"
)

// SubscriptionChecker checks whether a repository has active subscribers.
type SubscriptionChecker interface {
	HasSubscribers(ctx context.Context, repository string) (bool, error)
}

// SubscriptionRouterResult is the output of the Subscription Router node.
type SubscriptionRouterResult struct {
	Routed bool `json:"routed"`
}

// SubscriptionRouter checks if any subscriptions exist for the release's repository.
// Always-on node — drops events with no subscribers via ErrEventDropped.
type SubscriptionRouter struct {
	checker SubscriptionChecker
}

func NewSubscriptionRouter(checker SubscriptionChecker) *SubscriptionRouter {
	return &SubscriptionRouter{checker: checker}
}

func (n *SubscriptionRouter) Name() string { return "subscription_router" }

func (n *SubscriptionRouter) Execute(ctx context.Context, event *models.ReleaseEvent, _ json.RawMessage, _ map[string]json.RawMessage) (json.RawMessage, error) {
	hasSubscribers, err := n.checker.HasSubscribers(ctx, event.Repository)
	if err != nil {
		return nil, fmt.Errorf("check subscribers: %w", err)
	}

	if !hasSubscribers {
		return nil, ErrEventDropped
	}

	return json.Marshal(SubscriptionRouterResult{Routed: true})
}

var _ PipelineNode = (*SubscriptionRouter)(nil)
```

**Step 4: Run test to verify it passes**

Run: `go test ./internal/pipeline/ -v -run TestSubscriptionRouter`
Expected: PASS

**Step 5: Commit**

```bash
git add internal/pipeline/subscription_router.go internal/pipeline/subscription_router_test.go
git commit -m "feat: add Subscription Router pipeline node"
```

---

## Task 6: Urgency Scorer Node

**Files:**
- Create: `internal/pipeline/urgency_scorer.go`
- Test: `internal/pipeline/urgency_scorer_test.go`

The Urgency Scorer is the first configurable node — it only runs when `"urgency_scorer"` is present in `pipeline_config`. It reads the Regex Normalizer's result from `prior` and the release changelog to compute an urgency level.

**Step 1: Write the failing test**

```go
// internal/pipeline/urgency_scorer_test.go
package pipeline

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/sentioxyz/releaseguard/internal/models"
)

func mustMarshal(v any) json.RawMessage {
	data, _ := json.Marshal(v)
	return data
}

func TestUrgencyScorerName(t *testing.T) {
	n := NewUrgencyScorer()
	if got := n.Name(); got != "urgency_scorer" {
		t.Errorf("Name() = %q, want %q", got, "urgency_scorer")
	}
}

func TestUrgencyScorerPreRelease(t *testing.T) {
	n := NewUrgencyScorer()
	event := &models.ReleaseEvent{
		RawVersion:      "v1.0.0-rc.1",
		IsPreRelease:    true,
		SemanticVersion: models.SemanticData{Major: 1, PreRelease: "rc.1"},
		Timestamp:       time.Now(),
	}
	prior := map[string]json.RawMessage{
		"regex_normalizer": mustMarshal(RegexNormalizerResult{
			SemanticVersion: event.SemanticVersion,
			IsPreRelease:    true,
			Parsed:          true,
		}),
	}

	result, err := n.Execute(context.Background(), event, json.RawMessage(`{}`), prior)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}

	var res UrgencyScorerResult
	json.Unmarshal(result, &res)
	if res.Score != "LOW" {
		t.Errorf("Score = %q, want %q", res.Score, "LOW")
	}
}

func TestUrgencyScorerMajorVersion(t *testing.T) {
	n := NewUrgencyScorer()
	event := &models.ReleaseEvent{
		RawVersion:      "v2.0.0",
		SemanticVersion: models.SemanticData{Major: 2},
		Timestamp:       time.Now(),
	}
	prior := map[string]json.RawMessage{
		"regex_normalizer": mustMarshal(RegexNormalizerResult{
			SemanticVersion: event.SemanticVersion,
			Parsed:          true,
		}),
	}

	result, err := n.Execute(context.Background(), event, json.RawMessage(`{}`), prior)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}

	var res UrgencyScorerResult
	json.Unmarshal(result, &res)
	if res.Score != "HIGH" {
		t.Errorf("Score = %q, want %q", res.Score, "HIGH")
	}
}

func TestUrgencyScorerSecurityKeywords(t *testing.T) {
	n := NewUrgencyScorer()
	event := &models.ReleaseEvent{
		RawVersion:      "v1.0.1",
		Changelog:       "Fixes critical CVE-2024-1234 vulnerability",
		SemanticVersion: models.SemanticData{Major: 1, Patch: 1},
		Timestamp:       time.Now(),
	}
	prior := map[string]json.RawMessage{
		"regex_normalizer": mustMarshal(RegexNormalizerResult{
			SemanticVersion: event.SemanticVersion,
			Parsed:          true,
		}),
	}

	result, err := n.Execute(context.Background(), event, json.RawMessage(`{}`), prior)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}

	var res UrgencyScorerResult
	json.Unmarshal(result, &res)
	if res.Score != "CRITICAL" {
		t.Errorf("Score = %q, want %q", res.Score, "CRITICAL")
	}
}

func TestUrgencyScorerPatchVersion(t *testing.T) {
	n := NewUrgencyScorer()
	event := &models.ReleaseEvent{
		RawVersion:      "v1.21.3",
		SemanticVersion: models.SemanticData{Major: 1, Minor: 21, Patch: 3},
		Timestamp:       time.Now(),
	}
	prior := map[string]json.RawMessage{
		"regex_normalizer": mustMarshal(RegexNormalizerResult{
			SemanticVersion: event.SemanticVersion,
			Parsed:          true,
		}),
	}

	result, err := n.Execute(context.Background(), event, json.RawMessage(`{}`), prior)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}

	var res UrgencyScorerResult
	json.Unmarshal(result, &res)
	if res.Score != "LOW" {
		t.Errorf("Score = %q, want %q", res.Score, "LOW")
	}
}

func TestUrgencyScorerMinorVersion(t *testing.T) {
	n := NewUrgencyScorer()
	event := &models.ReleaseEvent{
		RawVersion:      "v1.22.0",
		SemanticVersion: models.SemanticData{Major: 1, Minor: 22},
		Timestamp:       time.Now(),
	}
	prior := map[string]json.RawMessage{
		"regex_normalizer": mustMarshal(RegexNormalizerResult{
			SemanticVersion: event.SemanticVersion,
			Parsed:          true,
		}),
	}

	result, err := n.Execute(context.Background(), event, json.RawMessage(`{}`), prior)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}

	var res UrgencyScorerResult
	json.Unmarshal(result, &res)
	if res.Score != "MEDIUM" {
		t.Errorf("Score = %q, want %q", res.Score, "MEDIUM")
	}
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/pipeline/ -v -run TestUrgencyScorer`
Expected: FAIL — `NewUrgencyScorer` not defined

**Step 3: Implement the Urgency Scorer**

```go
// internal/pipeline/urgency_scorer.go
package pipeline

import (
	"context"
	"encoding/json"
	"strings"

	"github.com/sentioxyz/releaseguard/internal/models"
)

var securityKeywords = []string{"security", "cve", "vulnerability", "critical", "exploit", "rce", "injection"}

// UrgencyScorerResult is the output of the Urgency Scorer node.
type UrgencyScorerResult struct {
	Score   string   `json:"score"`
	Factors []string `json:"factors"`
}

// UrgencyScorer computes a composite urgency level from prior node results and
// the release changelog. Configurable node — only runs when enabled in pipeline_config.
type UrgencyScorer struct{}

func NewUrgencyScorer() *UrgencyScorer { return &UrgencyScorer{} }

func (n *UrgencyScorer) Name() string { return "urgency_scorer" }

func (n *UrgencyScorer) Execute(_ context.Context, event *models.ReleaseEvent, _ json.RawMessage, prior map[string]json.RawMessage) (json.RawMessage, error) {
	var normResult RegexNormalizerResult
	if raw, ok := prior["regex_normalizer"]; ok {
		json.Unmarshal(raw, &normResult)
	}

	score := "MEDIUM"
	var factors []string

	// Security keywords override everything
	changelogLower := strings.ToLower(event.Changelog)
	for _, kw := range securityKeywords {
		if strings.Contains(changelogLower, kw) {
			score = "CRITICAL"
			factors = append(factors, "security_keyword_"+kw)
			break
		}
	}

	if score != "CRITICAL" {
		switch {
		case normResult.IsPreRelease:
			score = "LOW"
			factors = append(factors, "pre_release")
		case normResult.Parsed && normResult.SemanticVersion.Minor == 0 && normResult.SemanticVersion.Patch == 0:
			score = "HIGH"
			factors = append(factors, "major_version")
		case normResult.Parsed && normResult.SemanticVersion.Patch > 0:
			score = "LOW"
			factors = append(factors, "patch_version")
		default:
			factors = append(factors, "minor_version")
		}
	}

	return json.Marshal(UrgencyScorerResult{Score: score, Factors: factors})
}

var _ PipelineNode = (*UrgencyScorer)(nil)
```

**Step 4: Run test to verify it passes**

Run: `go test ./internal/pipeline/ -v -run TestUrgencyScorer`
Expected: PASS

**Step 5: Commit**

```bash
git add internal/pipeline/urgency_scorer.go internal/pipeline/urgency_scorer_test.go
git commit -m "feat: add Urgency Scorer pipeline node"
```

---

## Task 7: Pipeline Runner

**Files:**
- Create: `internal/pipeline/runner.go`
- Test: `internal/pipeline/runner_test.go`

The Runner is the core engine that orchestrates node execution. It fetches the release, creates a `pipeline_jobs` row, runs always-on nodes, then runs enabled configurable nodes.

**Step 1: Write the failing test**

```go
// internal/pipeline/runner_test.go
package pipeline

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/sentioxyz/releaseguard/internal/models"
)

// --- Mock Store ---

type mockPipelineStore struct {
	release      *models.ReleaseEvent
	getReleaseErr error
	jobID        int64
	upsertErr    error
	finalState   string
	finalResults map[string]json.RawMessage
	skipReason   string
	failMsg      string
}

func (m *mockPipelineStore) GetReleasePayload(_ context.Context, _ string) (*models.ReleaseEvent, error) {
	return m.release, m.getReleaseErr
}
func (m *mockPipelineStore) UpsertPipelineJob(_ context.Context, _ string) (int64, error) {
	return m.jobID, m.upsertErr
}
func (m *mockPipelineStore) UpdateNodeProgress(_ context.Context, _ int64, _ string, _ map[string]json.RawMessage) error {
	return nil
}
func (m *mockPipelineStore) CompletePipelineJob(_ context.Context, _ int64, nodeResults map[string]json.RawMessage) error {
	m.finalState = "completed"
	m.finalResults = nodeResults
	return nil
}
func (m *mockPipelineStore) SkipPipelineJob(_ context.Context, _ int64, reason string) error {
	m.finalState = "skipped"
	m.skipReason = reason
	return nil
}
func (m *mockPipelineStore) FailPipelineJob(_ context.Context, _ int64, errMsg string) error {
	m.finalState = "failed"
	m.failMsg = errMsg
	return nil
}

// --- Mock Node ---

type mockNode struct {
	name   string
	result json.RawMessage
	err    error
}

func (n *mockNode) Name() string { return n.name }
func (n *mockNode) Execute(_ context.Context, _ *models.ReleaseEvent, _ json.RawMessage, _ map[string]json.RawMessage) (json.RawMessage, error) {
	return n.result, n.err
}

// --- Tests ---

func TestRunnerHappyPath(t *testing.T) {
	store := &mockPipelineStore{
		release: &models.ReleaseEvent{ID: "r-1", RawVersion: "v1.0.0", Timestamp: time.Now()},
		jobID:   42,
	}

	alwaysOn := &mockNode{name: "normalizer", result: json.RawMessage(`{"parsed":true}`)}
	configurable := &mockNode{name: "scorer", result: json.RawMessage(`{"score":"HIGH"}`)}

	runner := NewRunner(store, []PipelineNode{alwaysOn}, []PipelineNode{configurable})

	config := map[string]json.RawMessage{"scorer": json.RawMessage(`{}`)}

	err := runner.Process(context.Background(), "r-1", config)
	if err != nil {
		t.Fatalf("Process: %v", err)
	}

	if store.finalState != "completed" {
		t.Errorf("state = %q, want %q", store.finalState, "completed")
	}
	if _, ok := store.finalResults["normalizer"]; !ok {
		t.Error("missing normalizer result")
	}
	if _, ok := store.finalResults["scorer"]; !ok {
		t.Error("missing scorer result")
	}
}

func TestRunnerNodeDropsEvent(t *testing.T) {
	store := &mockPipelineStore{
		release: &models.ReleaseEvent{ID: "r-1", Timestamp: time.Now()},
		jobID:   42,
	}

	dropper := &mockNode{name: "router", err: ErrEventDropped}

	runner := NewRunner(store, []PipelineNode{dropper}, nil)

	err := runner.Process(context.Background(), "r-1", nil)
	if err != nil {
		t.Fatalf("Process should not error on drop: %v", err)
	}

	if store.finalState != "skipped" {
		t.Errorf("state = %q, want %q", store.finalState, "skipped")
	}
}

func TestRunnerNodeError(t *testing.T) {
	store := &mockPipelineStore{
		release: &models.ReleaseEvent{ID: "r-1", Timestamp: time.Now()},
		jobID:   42,
	}

	failNode := &mockNode{name: "broken", err: errors.New("api timeout")}

	runner := NewRunner(store, []PipelineNode{failNode}, nil)

	err := runner.Process(context.Background(), "r-1", nil)
	if err == nil {
		t.Fatal("expected error")
	}

	if store.finalState != "failed" {
		t.Errorf("state = %q, want %q", store.finalState, "failed")
	}
}

func TestRunnerSkipsDisabledConfigurableNodes(t *testing.T) {
	store := &mockPipelineStore{
		release: &models.ReleaseEvent{ID: "r-1", Timestamp: time.Now()},
		jobID:   42,
	}

	alwaysOn := &mockNode{name: "always", result: json.RawMessage(`{}`)}
	optional := &mockNode{name: "optional", result: json.RawMessage(`{"should":"not appear"}`)}

	runner := NewRunner(store, []PipelineNode{alwaysOn}, []PipelineNode{optional})

	// Empty config — "optional" node is not enabled
	err := runner.Process(context.Background(), "r-1", nil)
	if err != nil {
		t.Fatalf("Process: %v", err)
	}

	if store.finalState != "completed" {
		t.Errorf("state = %q, want %q", store.finalState, "completed")
	}
	if _, ok := store.finalResults["optional"]; ok {
		t.Error("disabled node should not have results")
	}
}

func TestRunnerReleaseNotFound(t *testing.T) {
	store := &mockPipelineStore{getReleaseErr: errors.New("not found")}

	runner := NewRunner(store, nil, nil)

	err := runner.Process(context.Background(), "bad-id", nil)
	if err == nil {
		t.Fatal("expected error")
	}
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/pipeline/ -v -run TestRunner`
Expected: FAIL — `NewRunner` not defined

**Step 3: Implement the Runner**

```go
// internal/pipeline/runner.go
package pipeline

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
)

// Runner orchestrates sequential pipeline execution for a single release event.
type Runner struct {
	store        Store
	alwaysOn     []PipelineNode
	configurable []PipelineNode
}

func NewRunner(store Store, alwaysOn []PipelineNode, configurable []PipelineNode) *Runner {
	return &Runner{store: store, alwaysOn: alwaysOn, configurable: configurable}
}

// Process runs the full pipeline for a release. pipelineConfig controls which
// configurable nodes are enabled (nil = only always-on nodes run).
func (r *Runner) Process(ctx context.Context, releaseID string, pipelineConfig map[string]json.RawMessage) error {
	event, err := r.store.GetReleasePayload(ctx, releaseID)
	if err != nil {
		return fmt.Errorf("get release: %w", err)
	}

	jobID, err := r.store.UpsertPipelineJob(ctx, releaseID)
	if err != nil {
		return fmt.Errorf("create pipeline job: %w", err)
	}

	nodeResults := make(map[string]json.RawMessage)

	// Always-on nodes
	for _, node := range r.alwaysOn {
		if err := r.runNode(ctx, jobID, node, event, nil, nodeResults); err != nil {
			return err
		}
	}

	// Configurable nodes — only enabled ones
	for _, node := range r.configurable {
		config, enabled := pipelineConfig[node.Name()]
		if !enabled {
			continue
		}
		if err := r.runNode(ctx, jobID, node, event, config, nodeResults); err != nil {
			return err
		}
	}

	return r.store.CompletePipelineJob(ctx, jobID, nodeResults)
}

func (r *Runner) runNode(ctx context.Context, jobID int64, node PipelineNode, event *models.ReleaseEvent, config json.RawMessage, nodeResults map[string]json.RawMessage) error {
	r.store.UpdateNodeProgress(ctx, jobID, node.Name(), nodeResults)

	result, err := node.Execute(ctx, event, config, nodeResults)
	if errors.Is(err, ErrEventDropped) {
		return r.store.SkipPipelineJob(ctx, jobID, fmt.Sprintf("dropped by %s", node.Name()))
	}
	if err != nil {
		r.store.FailPipelineJob(ctx, jobID, err.Error())
		return fmt.Errorf("node %s: %w", node.Name(), err)
	}

	nodeResults[node.Name()] = result
	return nil
}
```

Note: `runNode` needs access to `models` for the event type — add the import:

```go
import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/sentioxyz/releaseguard/internal/models"
)
```

**Step 4: Run test to verify it passes**

Run: `go test ./internal/pipeline/ -v -run TestRunner`
Expected: PASS

**Step 5: Commit**

```bash
git add internal/pipeline/runner.go internal/pipeline/runner_test.go
git commit -m "feat: add pipeline runner engine"
```

---

## Task 8: River Worker

**Files:**
- Create: `internal/pipeline/worker.go`
- Test: `internal/pipeline/worker_test.go`

Thin wrapper that connects River's job processing to the pipeline Runner.

**Step 1: Write the failing test**

```go
// internal/pipeline/worker_test.go
package pipeline

import (
	"testing"

	"github.com/riverqueue/river"
	"github.com/sentioxyz/releaseguard/internal/queue"
)

func TestPipelineWorkerRegistration(t *testing.T) {
	// Verify Worker can be added to River workers without panic
	workers := river.NewWorkers()
	river.AddWorker(workers, NewWorker(nil))
}

// Compile-time check that Worker implements river.Worker.
var _ river.Worker[queue.PipelineJobArgs] = (*Worker)(nil)
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/pipeline/ -v -run TestPipelineWorker`
Expected: FAIL — `NewWorker` not defined

**Step 3: Implement the Worker**

```go
// internal/pipeline/worker.go
package pipeline

import (
	"context"
	"encoding/json"

	"github.com/riverqueue/river"
	"github.com/sentioxyz/releaseguard/internal/queue"
)

// Worker is a River worker that processes pipeline jobs.
type Worker struct {
	river.WorkerDefaults[queue.PipelineJobArgs]
	runner *Runner
}

func NewWorker(runner *Runner) *Worker {
	return &Worker{runner: runner}
}

func (w *Worker) Work(ctx context.Context, job *river.Job[queue.PipelineJobArgs]) error {
	// TODO: Load pipeline_config from project when projects table is implemented.
	// For now, only always-on nodes run (nil config = no configurable nodes enabled).
	var pipelineConfig map[string]json.RawMessage
	return w.runner.Process(ctx, job.Args.ReleaseID, pipelineConfig)
}
```

**Step 4: Run test to verify it passes**

Run: `go test ./internal/pipeline/ -v -run TestPipelineWorker`
Expected: PASS

**Step 5: Commit**

```bash
git add internal/pipeline/worker.go internal/pipeline/worker_test.go
git commit -m "feat: add River worker for pipeline job processing"
```

---

## Task 9: Wire Pipeline into main.go

**Files:**
- Modify: `cmd/server/main.go`

**Step 1: Update main.go to create pipeline components and start River with workers**

Replace the current `main()` function with:

```go
package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"time"

	"github.com/riverqueue/river"
	"github.com/sentioxyz/releaseguard/internal/db"
	"github.com/sentioxyz/releaseguard/internal/ingestion"
	"github.com/sentioxyz/releaseguard/internal/pipeline"
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

	// Pipeline
	pipelineStore := pipeline.NewPgStore(pool)
	pipelineRunner := pipeline.NewRunner(
		pipelineStore,
		[]pipeline.PipelineNode{
			pipeline.NewRegexNormalizer(),
			pipeline.NewSubscriptionRouter(pipelineStore),
		},
		[]pipeline.PipelineNode{
			pipeline.NewUrgencyScorer(),
		},
	)

	// River queue with pipeline worker
	workers := river.NewWorkers()
	river.AddWorker(workers, pipeline.NewWorker(pipelineRunner))

	riverClient, err := queue.NewRiverClient(pool, workers)
	if err != nil {
		slog.Error("river client failed", "err", err)
		os.Exit(1)
	}

	if err := riverClient.Start(ctx); err != nil {
		slog.Error("river start failed", "err", err)
		os.Exit(1)
	}

	// Ingestion layer
	ingestStore := ingestion.NewPgStore(pool, riverClient)
	svc := ingestion.NewService(ingestStore)

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

	srv := &http.Server{Addr: addr, Handler: mux}

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

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	riverClient.Stop(shutdownCtx)
	srv.Shutdown(shutdownCtx)
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
```

Key changes from current `main.go`:
- Added `pipeline` and `river` imports
- Created pipeline store, runner, and nodes
- Registered `pipeline.Worker` with River
- Changed River client from insert-only (`nil` workers) to processing mode
- Called `riverClient.Start(ctx)` to begin processing jobs
- Added graceful `riverClient.Stop()` on shutdown

**Step 2: Verify build**

Run: `go build ./cmd/server/`
Expected: Success

**Step 3: Commit**

```bash
git add cmd/server/main.go
git commit -m "feat: wire pipeline runner and River workers into main server"
```

---

## Task 10: Full Verification

**Step 1: Run all tests**

Run: `go test ./... -v`
Expected: All PASS

**Step 2: Run vet**

Run: `go vet ./...`
Expected: Clean (no warnings)

**Step 3: Verify build**

Run: `go build -o /dev/null ./cmd/server/`
Expected: Success

**Step 4: Verify test count**

Ensure the following tests exist and pass:

- `internal/pipeline`: `TestErrEventDropped`, `TestRegexNormalizerName`, `TestRegexNormalizerParsesVersions` (8 subtests), `TestSubscriptionRouterName`, `TestSubscriptionRouterWithSubscribers`, `TestSubscriptionRouterNoSubscribers`, `TestSubscriptionRouterCheckerError`, `TestUrgencyScorerName`, `TestUrgencyScorerPreRelease`, `TestUrgencyScorerMajorVersion`, `TestUrgencyScorerSecurityKeywords`, `TestUrgencyScorerPatchVersion`, `TestUrgencyScorerMinorVersion`, `TestRunnerHappyPath`, `TestRunnerNodeDropsEvent`, `TestRunnerNodeError`, `TestRunnerSkipsDisabledConfigurableNodes`, `TestRunnerReleaseNotFound`, `TestPipelineWorkerRegistration`
- `internal/models`: existing tests still pass
- `internal/queue`: existing tests still pass
- `internal/ingestion`: existing tests still pass

**Step 5: Final commit (if any fixes were needed)**

```bash
git add -A
git commit -m "fix: address verification issues from pipeline implementation"
```
