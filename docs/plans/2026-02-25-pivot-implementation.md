# ReleaseGuard Pivot Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Pivot ReleaseGuard from pipeline-centric to agent-driven release intelligence platform with semantic releases, context sources, and ADK-Go integration.

**Architecture:** Major restructure of entity model and data flow. Remove pipeline layer (nodes, runner, pipeline_jobs). Keep ingestion layer. Add context sources, semantic releases, agent runs (ADK-Go), and notification routing. Two subscription paths: source-level (release notes) and project-level (semantic reports).

**Tech Stack:** Go 1.25, PostgreSQL + River v0.31.0, Google ADK-Go (`google.golang.org/adk`), Next.js 15 + Tailwind + shadcn/ui

**Design doc:** `docs/plans/2026-02-25-pivot-design.md`

---

## Phase 1: Entity Model & Schema

### Task 1: Update Database Schema

**Files:**
- Modify: `internal/db/migrations.go`
- Modify: `DESIGN.md` — update Section 4 (Database Schema) to reflect new tables, UUID IDs, removed pipeline_jobs

**Step 1: Write the new schema**

Replace the `schema` constant in `internal/db/migrations.go` with:

```go
const schema = `
-- Tracked software projects (the central entity)
CREATE TABLE IF NOT EXISTS projects (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name VARCHAR(100) NOT NULL UNIQUE,
    description TEXT,
    agent_prompt TEXT,
    agent_rules JSONB,
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW()
);

-- Configured ingestion sources (polling-based: GitHub, Docker Hub)
CREATE TABLE IF NOT EXISTS sources (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    project_id UUID NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    provider VARCHAR(50) NOT NULL,
    repository VARCHAR(255) NOT NULL,
    poll_interval_seconds INT DEFAULT 900,
    enabled BOOLEAN DEFAULT true,
    config JSONB,
    last_polled_at TIMESTAMPTZ,
    last_error TEXT,
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW(),
    UNIQUE(provider, repository)
);

-- Context sources (read-only references for agent research)
CREATE TABLE IF NOT EXISTS context_sources (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    project_id UUID NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    type VARCHAR(50) NOT NULL,
    name VARCHAR(100) NOT NULL,
    config JSONB NOT NULL,
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW()
);

-- Source-level releases (detected from polling sources)
CREATE TABLE IF NOT EXISTS releases (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    source_id UUID NOT NULL REFERENCES sources(id) ON DELETE CASCADE,
    version VARCHAR(100) NOT NULL,
    raw_data JSONB,
    released_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ DEFAULT NOW(),
    UNIQUE(source_id, version)
);

-- Project-level semantic releases (AI-generated, correlating source releases)
CREATE TABLE IF NOT EXISTS semantic_releases (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    project_id UUID NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    version VARCHAR(100) NOT NULL,
    report JSONB,
    status VARCHAR(50) NOT NULL DEFAULT 'pending',
    error TEXT,
    created_at TIMESTAMPTZ DEFAULT NOW(),
    completed_at TIMESTAMPTZ,
    UNIQUE(project_id, version)
);

-- Join table: which source releases compose a semantic release
CREATE TABLE IF NOT EXISTS semantic_release_sources (
    semantic_release_id UUID NOT NULL REFERENCES semantic_releases(id) ON DELETE CASCADE,
    release_id UUID NOT NULL REFERENCES releases(id) ON DELETE CASCADE,
    PRIMARY KEY (semantic_release_id, release_id)
);

-- Notification channels (standalone)
CREATE TABLE IF NOT EXISTS notification_channels (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name VARCHAR(100) NOT NULL,
    type VARCHAR(50) NOT NULL,
    config JSONB NOT NULL,
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW()
);

-- Subscriptions: two types (source-level and project-level)
CREATE TABLE IF NOT EXISTS subscriptions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    channel_id UUID NOT NULL REFERENCES notification_channels(id) ON DELETE CASCADE,
    type VARCHAR(50) NOT NULL CHECK (type IN ('source', 'project')),
    source_id UUID REFERENCES sources(id) ON DELETE CASCADE,
    project_id UUID REFERENCES projects(id) ON DELETE CASCADE,
    version_filter TEXT,
    created_at TIMESTAMPTZ DEFAULT NOW(),
    CHECK (
        (type = 'source'  AND source_id  IS NOT NULL AND project_id IS NULL) OR
        (type = 'project' AND project_id IS NOT NULL AND source_id  IS NULL)
    )
);

-- Agent runs (scoped to project)
CREATE TABLE IF NOT EXISTS agent_runs (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    project_id UUID NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    semantic_release_id UUID REFERENCES semantic_releases(id) ON DELETE SET NULL,
    trigger VARCHAR(100) NOT NULL,
    status VARCHAR(50) NOT NULL DEFAULT 'pending',
    prompt_used TEXT,
    error TEXT,
    started_at TIMESTAMPTZ,
    completed_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ DEFAULT NOW()
);

-- API authentication keys
CREATE TABLE IF NOT EXISTS api_keys (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name VARCHAR(100) NOT NULL,
    key_hash VARCHAR(64) NOT NULL UNIQUE,
    key_prefix VARCHAR(12) NOT NULL,
    created_at TIMESTAMPTZ DEFAULT NOW(),
    last_used_at TIMESTAMPTZ
);

-- Trigger for SSE: notify on new releases
CREATE OR REPLACE FUNCTION notify_release_created() RETURNS trigger AS $$
BEGIN
    PERFORM pg_notify('release_events', json_build_object('type', 'release', 'id', NEW.id)::text);
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

DROP TRIGGER IF EXISTS release_created_trigger ON releases;
CREATE TRIGGER release_created_trigger
    AFTER INSERT ON releases
    FOR EACH ROW EXECUTE FUNCTION notify_release_created();

-- Trigger for SSE: notify on semantic release completion
CREATE OR REPLACE FUNCTION notify_semantic_release() RETURNS trigger AS $$
BEGIN
    IF NEW.status = 'completed' AND (OLD IS NULL OR OLD.status != 'completed') THEN
        PERFORM pg_notify('release_events', json_build_object('type', 'semantic_release', 'id', NEW.id)::text);
    END IF;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

DROP TRIGGER IF EXISTS semantic_release_trigger ON semantic_releases;
CREATE TRIGGER semantic_release_trigger
    AFTER INSERT OR UPDATE ON semantic_releases
    FOR EACH ROW EXECUTE FUNCTION notify_semantic_release();
`
```

**Key changes from old schema:**
- All IDs are now UUID (were SERIAL int)
- Removed: `pipeline_jobs`, `pipeline_config` from projects, `url` from projects
- Added: `context_sources`, `semantic_releases`, `semantic_release_sources`, `agent_runs`
- `sources`: renamed `source_type` → `provider`, added `config JSONB`, removed `exclude_*` fields (moved to `config`)
- `subscriptions`: now has `type` (source/project), `source_id`, `project_id` with check constraint
- `notification_channels`: added `updated_at`
- Added SSE trigger for `semantic_releases`

**Step 2: Update DESIGN.md database schema section**

Update `DESIGN.md` Section 4 (Database Schema) to document the new tables: `projects` (with `agent_prompt`, `agent_rules`), `sources` (renamed `source_type` → `provider`, UUID IDs), `context_sources`, `releases`, `semantic_releases`, `semantic_release_sources`, `notification_channels`, `subscriptions` (with `type` check constraint), `agent_runs`, `api_keys`. Remove documentation of `pipeline_jobs` and `pipeline_config`. Document the new SSE trigger for `semantic_releases`.

**Step 3: Run `go vet ./internal/db/...`**

Run: `go vet ./internal/db/...`
Expected: PASS (schema is just a string constant)

**Step 4: Commit**

```bash
git add internal/db/migrations.go DESIGN.md
git commit -m "refactor: update database schema for pivot — UUID IDs, context sources, semantic releases, agent runs"
```

---

### Task 2: Update Models

**Files:**
- Modify: `internal/models/project.go`
- Modify: `internal/models/source.go`
- Modify: `internal/models/release.go`
- Modify: `internal/models/channel.go`
- Modify: `internal/models/subscription.go`
- Create: `internal/models/context_source.go`
- Create: `internal/models/semantic_release.go`
- Create: `internal/models/agent_run.go`
- Modify: `DESIGN.md` — update Section 2.1 (ReleaseEvent IR) to document the new `Release` model replacing `ReleaseEvent`

**Step 1: Update Project model**

Replace `internal/models/project.go` with:

```go
package models

import (
	"encoding/json"
	"time"
)

type AgentRules struct {
	OnMajorRelease  bool   `json:"on_major_release,omitempty"`
	OnMinorRelease  bool   `json:"on_minor_release,omitempty"`
	OnSecurityPatch bool   `json:"on_security_patch,omitempty"`
	VersionPattern  string `json:"version_pattern,omitempty"`
}

type Project struct {
	ID          string          `json:"id"`
	Name        string          `json:"name"`
	Description string          `json:"description,omitempty"`
	AgentPrompt string          `json:"agent_prompt,omitempty"`
	AgentRules  json.RawMessage `json:"agent_rules,omitempty"`
	CreatedAt   time.Time       `json:"created_at"`
	UpdatedAt   time.Time       `json:"updated_at"`
}
```

**Step 2: Update Source model**

Replace `internal/models/source.go` with:

```go
package models

import (
	"encoding/json"
	"time"
)

type Source struct {
	ID                  string          `json:"id"`
	ProjectID           string          `json:"project_id"`
	Provider            string          `json:"provider"`
	Repository          string          `json:"repository"`
	PollIntervalSeconds int             `json:"poll_interval_seconds"`
	Enabled             bool            `json:"enabled"`
	Config              json.RawMessage `json:"config,omitempty"`
	LastPolledAt        *time.Time      `json:"last_polled_at,omitempty"`
	LastError           *string         `json:"last_error,omitempty"`
	CreatedAt           time.Time       `json:"created_at"`
	UpdatedAt           time.Time       `json:"updated_at"`
}
```

**Step 3: Update Release model**

Replace `internal/models/release.go` with:

```go
package models

import (
	"encoding/json"
	"time"
)

type Release struct {
	ID         string          `json:"id"`
	SourceID   string          `json:"source_id"`
	Version    string          `json:"version"`
	RawData    json.RawMessage `json:"raw_data,omitempty"`
	ReleasedAt *time.Time      `json:"released_at,omitempty"`
	CreatedAt  time.Time       `json:"created_at"`
}
```

Note: `ReleaseEvent` IR struct is removed. The `Release` model is now the primary release representation. `RawData` stores the full provider payload as JSONB.

**Step 4: Update Channel model**

Replace `internal/models/channel.go` with:

```go
package models

import (
	"encoding/json"
	"time"
)

type NotificationChannel struct {
	ID        string          `json:"id"`
	Name      string          `json:"name"`
	Type      string          `json:"type"`
	Config    json.RawMessage `json:"config"`
	CreatedAt time.Time       `json:"created_at"`
	UpdatedAt time.Time       `json:"updated_at"`
}
```

**Step 5: Update Subscription model**

Replace `internal/models/subscription.go` with:

```go
package models

import "time"

type Subscription struct {
	ID            string    `json:"id"`
	ChannelID     string    `json:"channel_id"`
	Type          string    `json:"type"`
	SourceID      *string   `json:"source_id,omitempty"`
	ProjectID     *string   `json:"project_id,omitempty"`
	VersionFilter string    `json:"version_filter,omitempty"`
	CreatedAt     time.Time `json:"created_at"`
}
```

**Step 6: Create ContextSource model**

Create `internal/models/context_source.go`:

```go
package models

import (
	"encoding/json"
	"time"
)

type ContextSource struct {
	ID        string          `json:"id"`
	ProjectID string          `json:"project_id"`
	Type      string          `json:"type"`
	Name      string          `json:"name"`
	Config    json.RawMessage `json:"config"`
	CreatedAt time.Time       `json:"created_at"`
	UpdatedAt time.Time       `json:"updated_at"`
}
```

**Step 7: Create SemanticRelease model**

Create `internal/models/semantic_release.go`:

```go
package models

import (
	"encoding/json"
	"time"
)

type SemanticReport struct {
	Summary        string `json:"summary"`
	Availability   string `json:"availability"`
	Adoption       string `json:"adoption"`
	Urgency        string `json:"urgency"`
	Recommendation string `json:"recommendation"`
}

type SemanticRelease struct {
	ID          string          `json:"id"`
	ProjectID   string          `json:"project_id"`
	Version     string          `json:"version"`
	Report      json.RawMessage `json:"report,omitempty"`
	Status      string          `json:"status"`
	Error       string          `json:"error,omitempty"`
	CreatedAt   time.Time       `json:"created_at"`
	CompletedAt *time.Time      `json:"completed_at,omitempty"`
}
```

**Step 8: Create AgentRun model**

Create `internal/models/agent_run.go`:

```go
package models

import "time"

type AgentRun struct {
	ID                string     `json:"id"`
	ProjectID         string     `json:"project_id"`
	SemanticReleaseID *string    `json:"semantic_release_id,omitempty"`
	Trigger           string     `json:"trigger"`
	Status            string     `json:"status"`
	PromptUsed        string     `json:"prompt_used,omitempty"`
	Error             string     `json:"error,omitempty"`
	StartedAt         *time.Time `json:"started_at,omitempty"`
	CompletedAt       *time.Time `json:"completed_at,omitempty"`
	CreatedAt         time.Time  `json:"created_at"`
}
```

**Step 9: Update DESIGN.md Section 2.1**

Replace the `ReleaseEvent` IR struct documentation in `DESIGN.md` Section 2.1 with documentation of the new `Release` model. Remove `SemanticData` struct docs. Add documentation for new model types: `ContextSource`, `SemanticRelease`, `SemanticReport`, `AgentRun`, `AgentRules`.

**Step 10: Build to check compilation**

Run: `go build ./internal/models/...`
Expected: PASS (models are standalone, no external deps)

**Step 11: Commit**

```bash
git add internal/models/ DESIGN.md
git commit -m "refactor: update models for pivot — UUID IDs, context sources, semantic releases, agent runs"
```

---

### Task 3: Remove Pipeline Layer

**Files:**
- Delete: `internal/pipeline/` (entire directory)
- Modify: `DESIGN.md` — remove Section 2.2 (Configurable Processing Pipeline), all node documentation
- Modify: `ARCH.md` — update system diagram to remove Pipeline, replace with Agent + Notification Routing

**Step 1: Delete the pipeline directory**

```bash
rm -rf internal/pipeline/
```

**Step 2: Update queue/jobs.go — replace PipelineJobArgs with new job types**

Replace `internal/queue/jobs.go` with:

```go
package queue

import "github.com/riverqueue/river"

// NotifyJobArgs is enqueued when a new source release is detected.
// The worker sends notifications to source-level subscribers.
type NotifyJobArgs struct {
	ReleaseID string `json:"release_id"`
	SourceID  string `json:"source_id"`
}

func (NotifyJobArgs) Kind() string { return "notify_release" }

var _ river.JobArgs = NotifyJobArgs{}

// AgentJobArgs is enqueued when an agent run is triggered.
// The worker runs the LLM agent to produce a semantic release.
type AgentJobArgs struct {
	AgentRunID string `json:"agent_run_id"`
	ProjectID  string `json:"project_id"`
}

func (AgentJobArgs) Kind() string { return "agent_run" }

var _ river.JobArgs = AgentJobArgs{}
```

**Step 3: Update `cmd/server/main.go` — remove pipeline references**

Remove the pipeline import and all pipeline-related code from `main.go`. The file should still compile but with reduced functionality (we'll wire new workers in later tasks).

Remove these lines/blocks:
- Import: `"github.com/sentioxyz/releaseguard/internal/pipeline"`
- Lines 42-52: pipeline store, runner, nodes
- Line 56: `river.AddWorker(workers, pipeline.NewWorker(pipelineRunner))`

Replace with a placeholder comment:

```go
// Workers will be added in Phase 2 (notification) and Phase 3 (agent)
workers := river.NewWorkers()
```

**Step 4: Update DESIGN.md**

Remove Section 2.2 (Configurable Processing Pipeline) and all pipeline node documentation (Regex Normalizer, Subscription Router, Urgency Scorer, etc.). Replace with a brief note that the pipeline has been replaced by the agent and notification routing system (detailed in later tasks).

**Step 5: Update ARCH.md**

Update the mermaid system diagram to remove the `Processing Pipeline` subgraph. Replace with two new subgraphs:
- `Notification Routing` — River worker sends to notification channels on new source releases
- `Agent Layer` — ADK-Go agent researches releases, produces semantic reports

Update the four-layer description to reflect the new architecture:
1. Ingestion Layer (unchanged)
2. Notification Routing (replaces pipeline for source-level notifications)
3. Agent Layer (replaces pipeline for analysis, uses ADK-Go)
4. Routing & Output (notification channels)

**Step 6: Build to verify**

Run: `go build ./...`
Expected: PASS (or compilation errors from API layer referencing pipeline types — fix those in Task 4)

**Step 7: Commit**

```bash
git add -A
git commit -m "refactor: remove pipeline layer — replaced by agent + notification workers"
```

---

### Task 4: Update Ingestion Layer for UUID IDs

**Files:**
- Modify: `internal/ingestion/source.go`
- Modify: `internal/ingestion/store.go`
- Modify: `internal/ingestion/pgstore.go`
- Modify: `internal/ingestion/service.go`
- Modify: `internal/ingestion/orchestrator.go`
- Modify: `internal/ingestion/loader.go`
- Modify: `internal/ingestion/dockerhub.go`
- Modify: `internal/ingestion/github_webhook.go` (if exists)

**Step 1: Update IngestionResult and IIngestionSource to use string IDs**

In `internal/ingestion/source.go`, change `SourceID()` return type from `int` to `string`:

```go
type IIngestionSource interface {
	Name() string
	SourceID() string
	FetchNewReleases(ctx context.Context) ([]IngestionResult, error)
}
```

**Step 2: Update ReleaseStore to use string sourceID**

In `internal/ingestion/store.go`:

```go
type ReleaseStore interface {
	IngestRelease(ctx context.Context, sourceID string, result *IngestionResult) error
}
```

Note: The store now takes `IngestionResult` directly instead of `models.ReleaseEvent` (which was removed).

**Step 3: Update PgStore.IngestRelease**

In `internal/ingestion/pgstore.go`, update to:
- Accept `string` sourceID
- Insert `IngestionResult` fields directly into `releases` table (id, source_id, version, raw_data)
- Enqueue `queue.NotifyJobArgs` instead of `queue.PipelineJobArgs`
- Generate UUID for release ID

```go
func (s *PgStore) IngestRelease(ctx context.Context, sourceID string, result *IngestionResult) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback(ctx)

	rawData, err := json.Marshal(result.Metadata)
	if err != nil {
		return fmt.Errorf("marshal raw_data: %w", err)
	}

	releaseID := uuid.New().String()
	_, err = tx.Exec(ctx,
		`INSERT INTO releases (id, source_id, version, raw_data, released_at) VALUES ($1, $2, $3, $4, $5)`,
		releaseID, sourceID, result.RawVersion, rawData, result.Timestamp,
	)
	if err != nil {
		return fmt.Errorf("insert release: %w", err)
	}

	_, err = s.river.InsertTx(ctx, tx, queue.NotifyJobArgs{
		ReleaseID: releaseID,
		SourceID:  sourceID,
	}, nil)
	if err != nil {
		return fmt.Errorf("enqueue job: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit: %w", err)
	}
	return nil
}
```

Add `"github.com/google/uuid"` to imports.

**Step 4: Update Service.ProcessResults**

In `internal/ingestion/service.go`, update to pass `IngestionResult` directly to the store instead of converting to `ReleaseEvent`. Change `sourceID` param from `int` to `string`.

```go
func (s *Service) ProcessResults(ctx context.Context, sourceID string, sourceName string, results []IngestionResult) error {
	for _, r := range results {
		if err := s.store.IngestRelease(ctx, sourceID, &r); err != nil {
			if isUniqueViolation(err) {
				slog.Debug("duplicate release, skipping", "version", r.RawVersion)
				continue
			}
			return fmt.Errorf("ingest %s: %w", r.RawVersion, err)
		}
		slog.Info("ingested release", "source", sourceName, "version", r.RawVersion)
	}
	return nil
}
```

**Step 5: Update DockerHubSource to return string SourceID**

In `internal/ingestion/dockerhub.go`, change the `id` field from `int` to `string` and update `SourceID()`.

**Step 6: Update SourceLoader to use string IDs**

In `internal/ingestion/loader.go`, change `LoadEnabledSources` and `LookupSourceID` to work with UUID strings. The SQL queries scan UUID columns as strings.

**Step 7: Update Orchestrator**

In `internal/ingestion/orchestrator.go`, no interface changes needed — it calls `Service.ProcessResults` which now uses string IDs.

**Step 8: Update GitHub webhook handler**

If `internal/ingestion/github_webhook.go` exists, update the `LookupSourceID` call to expect `string` return.

**Step 9: Build and verify**

Run: `go build ./internal/ingestion/...`
Expected: PASS

**Step 10: Commit**

```bash
git add internal/ingestion/ internal/queue/
git commit -m "refactor: update ingestion layer for UUID IDs and NotifyJobArgs"
```

---

### Task 5: Update API Layer — Store Interfaces and Handlers

This is a large task. The API layer needs significant changes to support UUID IDs, new entities, and new subscription model.

**Files:**
- Modify: `internal/api/server.go` — update Dependencies, add new routes
- Modify: `internal/api/projects.go` — update store interface and handler for new Project fields
- Modify: `internal/api/releases.go` — update store interface for UUID
- Modify: `internal/api/sources.go` — update store interface, nest under projects
- Modify: `internal/api/subscriptions.go` — update for source/project types
- Modify: `internal/api/channels.go` — update for UUID
- Modify: `internal/api/pgstore.go` — rewrite all queries for new schema
- Modify: `internal/api/health.go` — update stats queries
- Modify: `internal/api/events.go` — update SSE event types
- Create: `internal/api/context_sources.go` — new handler
- Create: `internal/api/semantic_releases.go` — new handler
- Create: `internal/api/agent.go` — new handler
- Modify: `docs/designs/2026-02-24-api-design.md` — rewrite to document all new endpoints, request/response shapes, and entity relationships

**Step 1: Update ProjectsStore interface and handler**

In `internal/api/projects.go`, update the store interface:

```go
type ProjectsStore interface {
	ListProjects(ctx context.Context, page, perPage int) ([]models.Project, int, error)
	CreateProject(ctx context.Context, p *models.Project) error
	GetProject(ctx context.Context, id string) (*models.Project, error)
	UpdateProject(ctx context.Context, id string, p *models.Project) error
	DeleteProject(ctx context.Context, id string) error
}
```

Update handler methods to use `string` id from path. Remove `strconv.Atoi` — just use `r.PathValue("id")` directly.

Update Create handler to accept `agent_prompt` and `agent_rules` fields. Remove `url` and `pipeline_config` field handling.

**Step 2: Update SourcesStore interface**

In `internal/api/sources.go`, update to:

```go
type SourcesStore interface {
	ListSourcesByProject(ctx context.Context, projectID string, page, perPage int) ([]models.Source, int, error)
	CreateSource(ctx context.Context, src *models.Source) error
	GetSource(ctx context.Context, id string) (*models.Source, error)
	UpdateSource(ctx context.Context, id string, src *models.Source) error
	DeleteSource(ctx context.Context, id string) error
}
```

Remove `GetLatestRelease`, `GetReleaseByVersion` (releases are accessed via the releases handler).
Remove `ReleaseView` type (will be replaced in releases handler).

**Step 3: Update ReleasesStore interface**

In `internal/api/releases.go`:

```go
type ReleasesStore interface {
	ListReleasesBySource(ctx context.Context, sourceID string, page, perPage int) ([]models.Release, int, error)
	ListReleasesByProject(ctx context.Context, projectID string, page, perPage int) ([]models.Release, int, error)
	GetRelease(ctx context.Context, id string) (*models.Release, error)
}
```

Remove `ListReleasesOpts`, `PipelineStatus`, pipeline-related methods.

**Step 4: Update SubscriptionsStore interface**

In `internal/api/subscriptions.go`:

```go
type SubscriptionsStore interface {
	ListSubscriptions(ctx context.Context, page, perPage int) ([]models.Subscription, int, error)
	CreateSubscription(ctx context.Context, sub *models.Subscription) error
	GetSubscription(ctx context.Context, id string) (*models.Subscription, error)
	UpdateSubscription(ctx context.Context, id string, sub *models.Subscription) error
	DeleteSubscription(ctx context.Context, id string) error
}
```

Update Create handler to validate `type` is `source` or `project`, and that the corresponding `source_id` or `project_id` is set.

**Step 5: Update ChannelsStore interface**

In `internal/api/channels.go`, change all `int` IDs to `string`.

**Step 6: Create ContextSourcesHandler**

Create `internal/api/context_sources.go`:

```go
package api

import (
	"context"
	"net/http"

	"github.com/sentioxyz/releaseguard/internal/models"
)

type ContextSourcesStore interface {
	ListContextSources(ctx context.Context, projectID string, page, perPage int) ([]models.ContextSource, int, error)
	CreateContextSource(ctx context.Context, cs *models.ContextSource) error
	GetContextSource(ctx context.Context, id string) (*models.ContextSource, error)
	UpdateContextSource(ctx context.Context, id string, cs *models.ContextSource) error
	DeleteContextSource(ctx context.Context, id string) error
}

type ContextSourcesHandler struct {
	store ContextSourcesStore
}

func NewContextSourcesHandler(store ContextSourcesStore) *ContextSourcesHandler {
	return &ContextSourcesHandler{store: store}
}

func (h *ContextSourcesHandler) List(w http.ResponseWriter, r *http.Request) {
	projectID := r.PathValue("projectId")
	page, perPage := ParsePagination(r)
	items, total, err := h.store.ListContextSources(r.Context(), projectID, page, perPage)
	if err != nil {
		RespondError(w, r, http.StatusInternalServerError, "INTERNAL", err.Error())
		return
	}
	RespondList(w, r, http.StatusOK, items, page, perPage, total)
}

func (h *ContextSourcesHandler) Create(w http.ResponseWriter, r *http.Request) {
	projectID := r.PathValue("projectId")
	var cs models.ContextSource
	if err := DecodeJSON(r, &cs); err != nil {
		RespondError(w, r, http.StatusBadRequest, "INVALID_JSON", err.Error())
		return
	}
	cs.ProjectID = projectID
	if err := h.store.CreateContextSource(r.Context(), &cs); err != nil {
		RespondError(w, r, http.StatusInternalServerError, "INTERNAL", err.Error())
		return
	}
	RespondJSON(w, r, http.StatusCreated, cs)
}

func (h *ContextSourcesHandler) Get(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	cs, err := h.store.GetContextSource(r.Context(), id)
	if err != nil {
		RespondError(w, r, http.StatusNotFound, "NOT_FOUND", "context source not found")
		return
	}
	RespondJSON(w, r, http.StatusOK, cs)
}

func (h *ContextSourcesHandler) Update(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	var cs models.ContextSource
	if err := DecodeJSON(r, &cs); err != nil {
		RespondError(w, r, http.StatusBadRequest, "INVALID_JSON", err.Error())
		return
	}
	if err := h.store.UpdateContextSource(r.Context(), id, &cs); err != nil {
		RespondError(w, r, http.StatusInternalServerError, "INTERNAL", err.Error())
		return
	}
	RespondJSON(w, r, http.StatusOK, cs)
}

func (h *ContextSourcesHandler) Delete(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if err := h.store.DeleteContextSource(r.Context(), id); err != nil {
		RespondError(w, r, http.StatusInternalServerError, "INTERNAL", err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
```

**Step 7: Create SemanticReleasesHandler**

Create `internal/api/semantic_releases.go`:

```go
package api

import (
	"context"
	"net/http"

	"github.com/sentioxyz/releaseguard/internal/models"
)

type SemanticReleasesStore interface {
	ListSemanticReleases(ctx context.Context, projectID string, page, perPage int) ([]models.SemanticRelease, int, error)
	GetSemanticRelease(ctx context.Context, id string) (*models.SemanticRelease, error)
	GetSemanticReleaseSources(ctx context.Context, id string) ([]models.Release, error)
}

type SemanticReleasesHandler struct {
	store SemanticReleasesStore
}

func NewSemanticReleasesHandler(store SemanticReleasesStore) *SemanticReleasesHandler {
	return &SemanticReleasesHandler{store: store}
}

func (h *SemanticReleasesHandler) List(w http.ResponseWriter, r *http.Request) {
	projectID := r.PathValue("projectId")
	page, perPage := ParsePagination(r)
	items, total, err := h.store.ListSemanticReleases(r.Context(), projectID, page, perPage)
	if err != nil {
		RespondError(w, r, http.StatusInternalServerError, "INTERNAL", err.Error())
		return
	}
	RespondList(w, r, http.StatusOK, items, page, perPage, total)
}

func (h *SemanticReleasesHandler) Get(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	sr, err := h.store.GetSemanticRelease(r.Context(), id)
	if err != nil {
		RespondError(w, r, http.StatusNotFound, "NOT_FOUND", "semantic release not found")
		return
	}
	RespondJSON(w, r, http.StatusOK, sr)
}
```

**Step 8: Create AgentHandler**

Create `internal/api/agent.go`:

```go
package api

import (
	"context"
	"net/http"

	"github.com/sentioxyz/releaseguard/internal/models"
)

type AgentStore interface {
	TriggerAgentRun(ctx context.Context, projectID, trigger string) (*models.AgentRun, error)
	ListAgentRuns(ctx context.Context, projectID string, page, perPage int) ([]models.AgentRun, int, error)
	GetAgentRun(ctx context.Context, id string) (*models.AgentRun, error)
}

type AgentHandler struct {
	store AgentStore
}

func NewAgentHandler(store AgentStore) *AgentHandler {
	return &AgentHandler{store: store}
}

func (h *AgentHandler) TriggerRun(w http.ResponseWriter, r *http.Request) {
	projectID := r.PathValue("projectId")
	run, err := h.store.TriggerAgentRun(r.Context(), projectID, "manual")
	if err != nil {
		RespondError(w, r, http.StatusInternalServerError, "INTERNAL", err.Error())
		return
	}
	RespondJSON(w, r, http.StatusAccepted, run)
}

func (h *AgentHandler) ListRuns(w http.ResponseWriter, r *http.Request) {
	projectID := r.PathValue("projectId")
	page, perPage := ParsePagination(r)
	runs, total, err := h.store.ListAgentRuns(r.Context(), projectID, page, perPage)
	if err != nil {
		RespondError(w, r, http.StatusInternalServerError, "INTERNAL", err.Error())
		return
	}
	RespondList(w, r, http.StatusOK, runs, page, perPage, total)
}

func (h *AgentHandler) GetRun(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	run, err := h.store.GetAgentRun(r.Context(), id)
	if err != nil {
		RespondError(w, r, http.StatusNotFound, "NOT_FOUND", "agent run not found")
		return
	}
	RespondJSON(w, r, http.StatusOK, run)
}
```

**Step 9: Update RegisterRoutes**

Update `internal/api/server.go` with new Dependencies and routes:

```go
type Dependencies struct {
	DB                    *pgxpool.Pool
	ProjectsStore         ProjectsStore
	ReleasesStore         ReleasesStore
	SubscriptionsStore    SubscriptionsStore
	SourcesStore          SourcesStore
	ChannelsStore         ChannelsStore
	ContextSourcesStore   ContextSourcesStore
	SemanticReleasesStore SemanticReleasesStore
	AgentStore            AgentStore
	KeyStore              KeyStore
	HealthChecker         HealthChecker
	Broadcaster           *Broadcaster
	NoAuth                bool
}
```

Add new route registrations:

```go
// Sources (nested under projects)
sources := NewSourcesHandler(deps.SourcesStore)
mux.Handle("GET /api/v1/projects/{projectId}/sources", chain(http.HandlerFunc(sources.List)))
mux.Handle("POST /api/v1/projects/{projectId}/sources", chain(http.HandlerFunc(sources.Create)))
mux.Handle("GET /api/v1/sources/{id}", chain(http.HandlerFunc(sources.Get)))
mux.Handle("PUT /api/v1/sources/{id}", chain(http.HandlerFunc(sources.Update)))
mux.Handle("DELETE /api/v1/sources/{id}", chain(http.HandlerFunc(sources.Delete)))

// Context Sources (nested under projects)
contextSources := NewContextSourcesHandler(deps.ContextSourcesStore)
mux.Handle("GET /api/v1/projects/{projectId}/context-sources", chain(http.HandlerFunc(contextSources.List)))
mux.Handle("POST /api/v1/projects/{projectId}/context-sources", chain(http.HandlerFunc(contextSources.Create)))
mux.Handle("GET /api/v1/context-sources/{id}", chain(http.HandlerFunc(contextSources.Get)))
mux.Handle("PUT /api/v1/context-sources/{id}", chain(http.HandlerFunc(contextSources.Update)))
mux.Handle("DELETE /api/v1/context-sources/{id}", chain(http.HandlerFunc(contextSources.Delete)))

// Releases
releases := NewReleasesHandler(deps.ReleasesStore)
mux.Handle("GET /api/v1/sources/{id}/releases", chain(http.HandlerFunc(releases.ListBySource)))
mux.Handle("GET /api/v1/projects/{projectId}/releases", chain(http.HandlerFunc(releases.ListByProject)))
mux.Handle("GET /api/v1/releases/{id}", chain(http.HandlerFunc(releases.Get)))

// Semantic Releases (nested under projects)
semanticReleases := NewSemanticReleasesHandler(deps.SemanticReleasesStore)
mux.Handle("GET /api/v1/projects/{projectId}/semantic-releases", chain(http.HandlerFunc(semanticReleases.List)))
mux.Handle("GET /api/v1/semantic-releases/{id}", chain(http.HandlerFunc(semanticReleases.Get)))

// Agent (nested under projects)
agent := NewAgentHandler(deps.AgentStore)
mux.Handle("POST /api/v1/projects/{projectId}/agent/run", chain(http.HandlerFunc(agent.TriggerRun)))
mux.Handle("GET /api/v1/projects/{projectId}/agent/runs", chain(http.HandlerFunc(agent.ListRuns)))
mux.Handle("GET /api/v1/agent-runs/{id}", chain(http.HandlerFunc(agent.GetRun)))
```

Remove old flat source routes and pipeline-related release routes.

**Step 10: Rewrite PgStore**

Rewrite `internal/api/pgstore.go` to implement all new store interfaces with the updated schema. This is the largest change — all SQL queries need updating for UUID columns and new tables.

Key patterns:
- Scan UUID columns as `string` (PostgreSQL UUID type auto-converts)
- New CRUD methods for context_sources, semantic_releases, agent_runs
- `TriggerAgentRun` creates an agent_run row and enqueues a River `AgentJobArgs`
- `ListReleasesByProject` joins `releases` → `sources` → `projects`

The `AgentStore.TriggerAgentRun` method needs access to a River client to enqueue the agent job. Add `river *river.Client[pgx.Tx]` to `PgStore`:

```go
type PgStore struct {
	pool  *pgxpool.Pool
	river *river.Client[pgx.Tx]
}
```

**Step 11: Rewrite API Design Doc**

Rewrite `docs/designs/2026-02-24-api-design.md` to document the new API:
- Update all endpoint tables to reflect new routes (nested sources under projects, context sources, semantic releases, agent endpoints)
- Update all request/response shapes for UUID IDs and new model fields
- Document the two subscription types (source and project) with check constraint behavior
- Add documentation for context sources CRUD, semantic releases read endpoints, and agent trigger/status endpoints
- Update SSE event types to include `semantic_release` events
- Update query parameters (remove pipeline-related, add `has_report` filter)
- Update authentication section if needed (unchanged but verify)

**Step 12: Build and verify**

Run: `go build ./...`
Expected: PASS

**Step 13: Commit**

```bash
git add internal/api/ docs/designs/2026-02-24-api-design.md
git commit -m "refactor: update API layer for pivot — new handlers, stores, routes for context sources, semantic releases, agent"
```

---

### Task 6: Update main.go Wiring

**Files:**
- Modify: `cmd/server/main.go`

**Step 1: Update main.go**

Wire the new dependencies:

```go
// API layer
pgStore := api.NewPgStore(pool, riverClient)
broadcaster := api.NewBroadcaster()

mux := http.NewServeMux()

// Register webhook route
mux.Handle("POST /webhook/github", webhookHandler)

// Register all API v1 routes
api.RegisterRoutes(mux, api.Dependencies{
	DB:                    pool,
	ProjectsStore:         pgStore,
	ReleasesStore:         pgStore,
	SubscriptionsStore:    pgStore,
	SourcesStore:          pgStore,
	ChannelsStore:         pgStore,
	ContextSourcesStore:   pgStore,
	SemanticReleasesStore: pgStore,
	AgentStore:            pgStore,
	KeyStore:              pgStore,
	HealthChecker:         pgStore,
	Broadcaster:           broadcaster,
	NoAuth:                noAuth,
})
```

**Step 2: Build and verify**

Run: `go build ./...`
Expected: PASS

**Step 3: Commit**

```bash
git add cmd/server/main.go
git commit -m "refactor: wire new API dependencies in main.go"
```

---

### Task 7: Update Tests

**Files:**
- Modify: all `*_test.go` files in `internal/ingestion/`, `internal/api/`

**Step 1: Update ingestion tests**

Update test fixtures to use string UUIDs instead of integer IDs. Update mock stores to match new `ReleaseStore` interface.

**Step 2: Update API tests**

Update test fixtures for new model shapes. Remove pipeline-related test cases.

**Step 3: Run all tests**

Run: `go test ./...`
Expected: PASS

**Step 4: Commit**

```bash
git add -A
git commit -m "test: update tests for pivot — UUID IDs, new models, removed pipeline"
```

---

## Phase 2: Source-Level Notifications

### Task 8: Implement Notification Worker

**Files:**
- Create: `internal/routing/sender.go` — notification sender interface
- Create: `internal/routing/webhook.go` — webhook sender
- Create: `internal/routing/slack.go` — Slack sender
- Create: `internal/routing/discord.go` — Discord sender
- Create: `internal/routing/worker.go` — River worker for NotifyJobArgs
- Modify: `DESIGN.md` — add Section on Notification Routing (sender interface, channel types, delivery flow)

**Step 1: Write failing test for webhook sender**

Create `internal/routing/webhook_test.go`:

```go
package routing

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/sentioxyz/releaseguard/internal/models"
)

func TestWebhookSender_Send(t *testing.T) {
	var received []byte
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		received, _ = io.ReadAll(r.Body)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	sender := &WebhookSender{Client: srv.Client()}
	ch := &models.NotificationChannel{
		Type:   "webhook",
		Config: []byte(`{"url": "` + srv.URL + `"}`),
	}
	msg := Notification{
		Title: "New release: geth v1.14.0",
		Body:  "Released on GitHub",
	}

	err := sender.Send(context.Background(), ch, msg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(received) == 0 {
		t.Fatal("expected webhook to receive payload")
	}
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/routing/... -v -run TestWebhookSender`
Expected: FAIL (package doesn't exist yet)

**Step 3: Implement notification sender interface and webhook sender**

Create `internal/routing/sender.go`:

```go
package routing

import (
	"context"

	"github.com/sentioxyz/releaseguard/internal/models"
)

type Notification struct {
	Title   string `json:"title"`
	Body    string `json:"body"`
	Version string `json:"version"`
}

type Sender interface {
	Send(ctx context.Context, ch *models.NotificationChannel, msg Notification) error
}
```

Create `internal/routing/webhook.go`:

```go
package routing

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/sentioxyz/releaseguard/internal/models"
)

type webhookConfig struct {
	URL string `json:"url"`
}

type WebhookSender struct {
	Client *http.Client
}

func (s *WebhookSender) Send(ctx context.Context, ch *models.NotificationChannel, msg Notification) error {
	var cfg webhookConfig
	if err := json.Unmarshal(ch.Config, &cfg); err != nil {
		return fmt.Errorf("parse webhook config: %w", err)
	}

	body, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("marshal notification: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, cfg.URL, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := s.Client.Do(req)
	if err != nil {
		return fmt.Errorf("send webhook: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return fmt.Errorf("webhook returned status %d", resp.StatusCode)
	}
	return nil
}
```

**Step 4: Run test to verify it passes**

Run: `go test ./internal/routing/... -v -run TestWebhookSender`
Expected: PASS

**Step 5: Implement Slack and Discord senders**

Create `internal/routing/slack.go` and `internal/routing/discord.go` following the same pattern as webhook. Slack uses incoming webhook URL. Discord uses webhook URL with embeds.

**Step 6: Implement notification dispatch worker**

Create `internal/routing/worker.go`:

```go
package routing

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/riverqueue/river"
	"github.com/sentioxyz/releaseguard/internal/models"
	"github.com/sentioxyz/releaseguard/internal/queue"
)

type NotifyStore interface {
	GetRelease(ctx context.Context, id string) (*models.Release, error)
	GetSource(ctx context.Context, id string) (*models.Source, error)
	ListSourceSubscriptions(ctx context.Context, sourceID string) ([]models.Subscription, error)
	GetChannel(ctx context.Context, id string) (*models.NotificationChannel, error)
}

type NotifyWorker struct {
	river.WorkerDefaults[queue.NotifyJobArgs]
	store   NotifyStore
	senders map[string]Sender
}

func NewNotifyWorker(store NotifyStore) *NotifyWorker {
	return &NotifyWorker{
		store: store,
		senders: map[string]Sender{
			"webhook": &WebhookSender{Client: &http.Client{Timeout: 10 * time.Second}},
			"slack":   &SlackSender{Client: &http.Client{Timeout: 10 * time.Second}},
			"discord": &DiscordSender{Client: &http.Client{Timeout: 10 * time.Second}},
		},
	}
}

func (w *NotifyWorker) Work(ctx context.Context, job *river.Job[queue.NotifyJobArgs]) error {
	release, err := w.store.GetRelease(ctx, job.Args.ReleaseID)
	if err != nil {
		return fmt.Errorf("get release: %w", err)
	}

	subs, err := w.store.ListSourceSubscriptions(ctx, job.Args.SourceID)
	if err != nil {
		return fmt.Errorf("list subscriptions: %w", err)
	}

	for _, sub := range subs {
		ch, err := w.store.GetChannel(ctx, sub.ChannelID)
		if err != nil {
			slog.Error("get channel failed", "channel_id", sub.ChannelID, "err", err)
			continue
		}

		sender, ok := w.senders[ch.Type]
		if !ok {
			slog.Warn("unknown channel type", "type", ch.Type)
			continue
		}

		msg := Notification{
			Title:   fmt.Sprintf("New release: %s", release.Version),
			Body:    string(release.RawData),
			Version: release.Version,
		}

		if err := sender.Send(ctx, ch, msg); err != nil {
			slog.Error("send notification failed", "channel", ch.Name, "err", err)
		}
	}

	return nil
}
```

**Step 7: Register NotifyWorker in main.go**

Add the notification worker to River:

```go
import "github.com/sentioxyz/releaseguard/internal/routing"

// In main():
notifyWorker := routing.NewNotifyWorker(pgStore)
river.AddWorker(workers, notifyWorker)
```

This requires `pgStore` to implement `NotifyStore` interface. Add the methods `GetRelease`, `GetSource`, `ListSourceSubscriptions`, `GetChannel` to `api.PgStore`.

**Step 8: Build and run tests**

Run: `go test ./internal/routing/... -v`
Expected: PASS

Run: `go build ./...`
Expected: PASS

**Step 9: Commit**

```bash
git add internal/routing/ cmd/server/main.go internal/api/pgstore.go DESIGN.md
git commit -m "feat: implement notification routing — webhook, Slack, Discord senders + River worker"
```

---

### Task 9: Agent Rule Engine

**Files:**
- Create: `internal/routing/rules.go` — rule matching
- Create: `internal/routing/rules_test.go` — tests

**Step 1: Write failing test**

```go
func TestCheckAgentRules_MajorRelease(t *testing.T) {
	rules := &models.AgentRules{OnMajorRelease: true}
	triggered := CheckAgentRules(rules, "v2.0.0", "v1.5.3")
	if !triggered {
		t.Fatal("expected agent to be triggered for major version bump")
	}
}

func TestCheckAgentRules_NoMatch(t *testing.T) {
	rules := &models.AgentRules{OnMajorRelease: true}
	triggered := CheckAgentRules(rules, "v1.5.4", "v1.5.3")
	if triggered {
		t.Fatal("expected agent not to be triggered for patch bump")
	}
}
```

**Step 2: Implement rule checking**

Create `internal/routing/rules.go`:

```go
package routing

import (
	"regexp"
	"strconv"
	"strings"

	"github.com/sentioxyz/releaseguard/internal/models"
)

func CheckAgentRules(rules *models.AgentRules, newVersion, previousVersion string) bool {
	if rules == nil {
		return false
	}

	newMajor, newMinor, _ := parseVersion(newVersion)
	oldMajor, oldMinor, _ := parseVersion(previousVersion)

	if rules.OnMajorRelease && newMajor > oldMajor {
		return true
	}
	if rules.OnMinorRelease && (newMajor > oldMajor || newMinor > oldMinor) {
		return true
	}
	if rules.OnSecurityPatch && isSecurityPatch(newVersion) {
		return true
	}
	if rules.VersionPattern != "" {
		if matched, _ := regexp.MatchString(rules.VersionPattern, newVersion); matched {
			return true
		}
	}
	return false
}

func parseVersion(v string) (major, minor, patch int) {
	v = strings.TrimPrefix(v, "v")
	parts := strings.SplitN(v, ".", 3)
	if len(parts) >= 1 {
		major, _ = strconv.Atoi(parts[0])
	}
	if len(parts) >= 2 {
		minor, _ = strconv.Atoi(parts[1])
	}
	if len(parts) >= 3 {
		patchStr := strings.SplitN(parts[2], "-", 2)[0]
		patch, _ = strconv.Atoi(patchStr)
	}
	return
}

func isSecurityPatch(version string) bool {
	lower := strings.ToLower(version)
	keywords := []string{"security", "cve", "vuln"}
	for _, kw := range keywords {
		if strings.Contains(lower, kw) {
			return true
		}
	}
	return false
}
```

**Step 3: Run tests**

Run: `go test ./internal/routing/... -v -run TestCheckAgentRules`
Expected: PASS

**Step 4: Wire rule checking into NotifyWorker**

After sending source notifications, check the project's agent rules. If triggered, enqueue an `AgentJobArgs`. This requires the worker to also look up the project and its previous release.

**Step 5: Commit**

```bash
git add internal/routing/
git commit -m "feat: implement agent rules engine — version-based auto-trigger for agent runs"
```

---

## Phase 3: Agent Integration (ADK-Go)

### Task 10: Add ADK-Go Dependency

**Step 1: Add dependency**

```bash
go get google.golang.org/adk@latest
```

**Step 2: Verify**

Run: `go build ./...`
Expected: PASS

**Step 3: Commit**

```bash
git add go.mod go.sum
git commit -m "deps: add Google ADK-Go for agent orchestration"
```

---

### Task 11: Implement Agent Tools

**Files:**
- Create: `internal/agent/tools.go` — ADK function tools
- Create: `internal/agent/tools_test.go` — tests
- Modify: `DESIGN.md` — add Section on Agent Architecture (tools, LLM interface, ADK-Go integration)

**Step 1: Write failing test for get_releases tool**

```go
func TestGetReleasesTool(t *testing.T) {
	store := &mockAgentStore{
		releases: []models.Release{
			{ID: "r1", Version: "v1.14.0"},
			{ID: "r2", Version: "v1.13.0"},
		},
	}
	tool := NewGetReleasesTool(store)
	// Verify tool name and that it returns releases
	if tool.Name() != "get_releases" {
		t.Fatalf("expected name 'get_releases', got '%s'", tool.Name())
	}
}
```

**Step 2: Implement agent tools**

Create `internal/agent/tools.go` with ADK `functiontool` wrappers:

```go
package agent

import (
	"context"

	"github.com/sentioxyz/releaseguard/internal/models"
	"google.golang.org/adk/tool"
	"google.golang.org/adk/tool/functiontool"
)

type AgentDataStore interface {
	ListReleasesByProject(ctx context.Context, projectID string) ([]models.Release, error)
	GetRelease(ctx context.Context, id string) (*models.Release, error)
	ListContextSources(ctx context.Context, projectID string) ([]models.ContextSource, error)
}

type GetReleasesInput struct {
	ProjectID string `json:"project_id"`
}

type GetReleasesOutput struct {
	Releases []models.Release `json:"releases"`
}

func NewGetReleasesTool(store AgentDataStore) tool.Tool {
	t, _ := functiontool.New(functiontool.Config{
		Name:        "get_releases",
		Description: "Get all recent releases across the project's sources",
	}, func(ctx tool.Context, input GetReleasesInput) (GetReleasesOutput, error) {
		releases, err := store.ListReleasesByProject(context.Background(), input.ProjectID)
		if err != nil {
			return GetReleasesOutput{}, err
		}
		return GetReleasesOutput{Releases: releases}, nil
	})
	return t
}
```

Implement similar tools: `get_release_notes`, `browse_context_sources`, `check_availability`, `check_adoption`.

**Step 3: Run tests**

Run: `go test ./internal/agent/... -v`
Expected: PASS

**Step 4: Commit**

```bash
git add internal/agent/
git commit -m "feat: implement ADK-Go agent tools — releases, context sources, availability, adoption"
```

---

### Task 12: Implement Agent Orchestrator

**Files:**
- Create: `internal/agent/orchestrator.go` — agent setup and execution
- Create: `internal/agent/worker.go` — River worker for AgentJobArgs
- Modify: `ARCH.md` — update Agent/Intelligence layer description with ADK-Go details
- Modify: `DESIGN.md` — add SemanticReport structure, agent run lifecycle, context source browsing

**Step 1: Implement orchestrator**

Create `internal/agent/orchestrator.go`:

```go
package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"time"

	"github.com/sentioxyz/releaseguard/internal/models"
	"google.golang.org/adk/agent/llmagent"
	"google.golang.org/adk/model"
	"google.golang.org/adk/model/gemini"
	"google.golang.org/adk/runner"
	"google.golang.org/adk/session"
	"google.golang.org/adk/tool"
	"google.golang.org/genai"
)

type OrchestratorStore interface {
	AgentDataStore
	GetProject(ctx context.Context, id string) (*models.Project, error)
	GetAgentRun(ctx context.Context, id string) (*models.AgentRun, error)
	UpdateAgentRunStatus(ctx context.Context, id, status string) error
	CreateSemanticRelease(ctx context.Context, sr *models.SemanticRelease, releaseIDs []string) error
	UpdateAgentRunResult(ctx context.Context, id string, semanticReleaseID string) error
}

type Orchestrator struct {
	store OrchestratorStore
	model model.LLM
	tools []tool.Tool
}

func NewOrchestrator(store OrchestratorStore) (*Orchestrator, error) {
	// Default to Gemini — other providers can be added via config
	apiKey := os.Getenv("GOOGLE_API_KEY")
	ctx := context.Background()
	m, err := gemini.NewModel(ctx, "gemini-2.5-flash", &genai.ClientConfig{
		APIKey: apiKey,
	})
	if err != nil {
		return nil, fmt.Errorf("create gemini model: %w", err)
	}

	o := &Orchestrator{
		store: store,
		model: m,
	}

	o.tools = []tool.Tool{
		NewGetReleasesTool(store),
		// Add other tools here
	}

	return o, nil
}

func (o *Orchestrator) RunAgent(ctx context.Context, run *models.AgentRun) error {
	// Update status to running
	if err := o.store.UpdateAgentRunStatus(ctx, run.ID, "running"); err != nil {
		return err
	}

	project, err := o.store.GetProject(ctx, run.ProjectID)
	if err != nil {
		return fmt.Errorf("get project: %w", err)
	}

	// Create ADK agent with project's prompt
	agentDef, err := llmagent.New(llmagent.Config{
		Name:        "release_researcher",
		Description: "Researches releases across a project's sources",
		Model:       o.model,
		Instruction: project.AgentPrompt,
		Tools:       o.tools,
	})
	if err != nil {
		return fmt.Errorf("create agent: %w", err)
	}

	// Run agent
	r, err := runner.New(runner.Config{
		AppName:        "releaseguard",
		Agent:          agentDef,
		SessionService: session.InMemoryService(),
	})
	if err != nil {
		return fmt.Errorf("create runner: %w", err)
	}

	sess, err := r.SessionService().Create(ctx, &session.Session{
		AppName: "releaseguard",
		UserID:  "system",
	})
	if err != nil {
		return fmt.Errorf("create session: %w", err)
	}

	prompt := fmt.Sprintf("Analyze the latest releases for project '%s'. "+
		"Use the available tools to gather release data, context sources, "+
		"availability, and adoption metrics. Produce a comprehensive analysis.", project.Name)

	msg := genai.NewContentFromText(prompt, "user")

	var finalResponse string
	for event, err := range r.Run(ctx, sess.UserID, sess.ID, msg, nil) {
		if err != nil {
			slog.Error("agent event error", "err", err)
			o.store.UpdateAgentRunStatus(ctx, run.ID, "failed")
			return err
		}
		if event.IsFinalResponse() && event.Content != nil {
			for _, part := range event.Content.Parts {
				if part.Text != "" {
					finalResponse = part.Text
				}
			}
		}
	}

	// Parse and store semantic release
	report := json.RawMessage(finalResponse)
	sr := &models.SemanticRelease{
		ProjectID: run.ProjectID,
		Version:   "auto", // Will be determined from release data
		Report:    report,
		Status:    "completed",
		now:       time.Now(),
	}

	// Get release IDs to link
	releases, _ := o.store.ListReleasesByProject(ctx, run.ProjectID)
	var releaseIDs []string
	for _, r := range releases {
		releaseIDs = append(releaseIDs, r.ID)
	}

	if err := o.store.CreateSemanticRelease(ctx, sr, releaseIDs); err != nil {
		return fmt.Errorf("create semantic release: %w", err)
	}

	o.store.UpdateAgentRunResult(ctx, run.ID, sr.ID)
	o.store.UpdateAgentRunStatus(ctx, run.ID, "completed")

	return nil
}
```

Note: This is a starting point. The exact ADK-Go API usage may need adjustment based on the actual library version at implementation time. The key pattern is: create agent with tools → run → extract final response → store as semantic release.

**Step 2: Implement River worker**

Create `internal/agent/worker.go`:

```go
package agent

import (
	"context"
	"fmt"

	"github.com/riverqueue/river"
	"github.com/sentioxyz/releaseguard/internal/queue"
)

type AgentWorker struct {
	river.WorkerDefaults[queue.AgentJobArgs]
	orchestrator *Orchestrator
	store        OrchestratorStore
}

func NewAgentWorker(orchestrator *Orchestrator, store OrchestratorStore) *AgentWorker {
	return &AgentWorker{orchestrator: orchestrator, store: store}
}

func (w *AgentWorker) Work(ctx context.Context, job *river.Job[queue.AgentJobArgs]) error {
	run, err := w.store.GetAgentRun(ctx, job.Args.AgentRunID)
	if err != nil {
		return fmt.Errorf("get agent run: %w", err)
	}
	return w.orchestrator.RunAgent(ctx, run)
}
```

**Step 3: Register agent worker in main.go**

```go
import "github.com/sentioxyz/releaseguard/internal/agent"

agentOrchestrator, err := agent.NewOrchestrator(pgStore)
if err != nil {
	slog.Warn("agent orchestrator not available", "err", err)
} else {
	agentWorker := agent.NewAgentWorker(agentOrchestrator, pgStore)
	river.AddWorker(workers, agentWorker)
}
```

**Step 4: Build and verify**

Run: `go build ./...`
Expected: PASS

**Step 5: Commit**

```bash
git add internal/agent/ cmd/server/main.go
git commit -m "feat: implement agent orchestrator with ADK-Go and River worker"
```

---

## Phase 4: Project-Level Subscriptions

### Task 13: Implement Project Notification on Semantic Release

**Files:**
- Modify: `internal/agent/orchestrator.go` — after creating semantic release, send project notifications
- Modify: `internal/routing/worker.go` — add project notification logic

**Step 1: Add project notification to agent orchestrator**

After the agent completes and creates a semantic release, look up project-level subscriptions and send notifications:

```go
// After CreateSemanticRelease in orchestrator.RunAgent:
subs, _ := o.store.ListProjectSubscriptions(ctx, run.ProjectID)
for _, sub := range subs {
	ch, _ := o.store.GetChannel(ctx, sub.ChannelID)
	sender := o.senders[ch.Type]
	msg := Notification{
		Title:   fmt.Sprintf("Semantic Release Report: %s %s", project.Name, sr.Version),
		Body:    finalResponse,
		Version: sr.Version,
	}
	sender.Send(ctx, ch, msg)
}
```

**Step 2: Add ListProjectSubscriptions to store interface**

```go
ListProjectSubscriptions(ctx context.Context, projectID string) ([]models.Subscription, error)
```

Implement in `api/pgstore.go`:

```sql
SELECT id, channel_id, type, source_id, project_id, version_filter, created_at
FROM subscriptions
WHERE type = 'project' AND project_id = $1
```

**Step 3: Commit**

```bash
git add internal/agent/ internal/api/pgstore.go
git commit -m "feat: send project-level notifications when semantic release is completed"
```

---

## Phase 5: Frontend Restructure

### Task 14: Update TypeScript API Client

**Files:**
- Modify: `web/lib/api/` — update API client for new endpoints

**Step 1: Update API client types and functions**

Update TypeScript types to match new Go models (UUID IDs, new entities). Add functions for context sources, semantic releases, and agent endpoints.

**Step 2: Remove MSW mocks**

Delete mock handlers. Wire all pages to real API.

**Step 3: Commit**

```bash
git add web/
git commit -m "refactor: update frontend API client for pivot — new types, endpoints, remove MSW"
```

---

### Task 15: Restructure Pages

**Files:**
- Modify: `web/app/projects/[id]/page.tsx` — show sources, context sources, agent config
- Create: `web/app/projects/[id]/context-sources/` — CRUD pages
- Create: `web/app/projects/[id]/semantic-releases/` — list + detail pages
- Create: `web/app/projects/[id]/agent/` — prompt editor, runs list, trigger button
- Modify: `web/app/releases/page.tsx` — update for new Release model
- Modify: `web/app/subscriptions/new/page.tsx` — add source/project type selector

**Step 1: Update project detail page**

Add tabs/sections for: Sources, Context Sources, Semantic Releases, Agent Config.

**Step 2: Create context sources pages**

Follow existing CRUD page patterns from sources.

**Step 3: Create semantic releases pages**

List page with report preview. Detail page with full report and linked source releases.

**Step 4: Create agent pages**

Agent prompt editor (textarea), rules config (checkbox form), run history table, "Run Agent" button.

**Step 5: Update subscriptions form**

Add type selector (source/project). Show source or project picker based on type.

**Step 6: Build frontend**

```bash
cd web && npm run build
```
Expected: PASS

**Step 7: Commit**

```bash
git add web/
git commit -m "feat: restructure frontend for pivot — context sources, semantic releases, agent UI"
```

---

### Task 16: Update Dashboard

**Files:**
- Modify: `web/app/page.tsx` — show source releases, semantic releases, agent status

**Step 1: Update dashboard**

Replace old stats cards with:
- Recent source releases (across all projects)
- Recent semantic releases (with report summaries)
- Active agent runs
- Notification delivery stats

**Step 2: Build and verify**

```bash
cd web && npm run build
```
Expected: PASS

**Step 3: Commit**

```bash
git add web/
git commit -m "feat: update dashboard for pivot — semantic releases, agent status"
```

---

## Phase 6: Integration & Verification

### Task 17: End-to-End Build Verification

**Step 1: Build Go backend**

Run: `go build -o releaseguard ./cmd/server`
Expected: PASS — binary compiles

**Step 2: Run all Go tests**

Run: `go test ./...`
Expected: PASS

**Step 3: Run go vet**

Run: `go vet ./...`
Expected: PASS

**Step 4: Build frontend**

```bash
cd web && npm run build
```
Expected: PASS

**Step 5: Commit any fixes**

```bash
git add -A
git commit -m "chore: fix build issues from pivot integration"
```

---

### Task 18: Update CLAUDE.md

**Files:**
- Modify: `CLAUDE.md` — update project overview, key interfaces, build commands

**Step 1: Update CLAUDE.md**

Update the Key Interfaces section to remove pipeline references (`PipelineNode`, `Store`). Add new interfaces: `Sender` (notification), `AgentDataStore`, `LLM` (ADK-Go). Update entity descriptions, data flow diagram, and directory structure.

**Step 2: Commit**

```bash
git add CLAUDE.md
git commit -m "docs: update CLAUDE.md for pivot"
```
