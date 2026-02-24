# Project-Centric Redesign Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Restructure ReleaseBeacon from pipeline-centric to project-centric architecture with two-layer processing (deterministic + optional agent enrichment).

**Architecture:** Sources become children of projects. A new `source_releases` table caches raw per-source data. The `releases` table becomes project-scoped with cross-source correlation. The fixed pipeline is removed in favor of deterministic processing + optional agent enrichment. The frontend consolidates sources, releases, and subscriptions into the project detail page.

**Tech Stack:** Go 1.25, PostgreSQL, Next.js 15, React 19, Tailwind CSS v4, SWR, MSW, shadcn/ui

**Design Doc:** `docs/plans/2026-02-24-project-centric-redesign-design.md`

---

## Phase 1: Database Schema & Models

### Task 1: Write migration for schema changes

**Files:**
- Modify: `internal/db/migrations.go`

**Step 1: Read current migration file**

Read `internal/db/migrations.go` to understand the full schema.

**Step 2: Write the new migration SQL**

Add a new migration block after the existing schema. The migration must be idempotent (use `IF NOT EXISTS`, `IF EXISTS`).

Changes needed:

```sql
-- 1. Add new columns to projects
ALTER TABLE projects ADD COLUMN IF NOT EXISTS poll_interval INTERVAL DEFAULT '15 minutes';
ALTER TABLE projects ADD COLUMN IF NOT EXISTS idle_window INTERVAL DEFAULT '6 hours';
ALTER TABLE projects ADD COLUMN IF NOT EXISTS agent_enabled BOOLEAN DEFAULT false;

-- 2. Create source_releases table (raw cache layer)
CREATE TABLE IF NOT EXISTS source_releases (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    source_id INT NOT NULL REFERENCES sources(id) ON DELETE CASCADE,
    project_id INT NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    release_id UUID REFERENCES releases(id) ON DELETE SET NULL,
    raw_version TEXT NOT NULL,
    release_notes TEXT,
    metadata JSONB DEFAULT '{}',
    detected_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(source_id, raw_version)
);
CREATE INDEX IF NOT EXISTS idx_source_releases_project ON source_releases(project_id);
CREATE INDEX IF NOT EXISTS idx_source_releases_release ON source_releases(release_id);

-- 3. Restructure releases table
-- We need to migrate releases from source-scoped to project-scoped.
-- Strategy: add new columns, migrate data, then drop old constraints.

-- Add new columns
ALTER TABLE releases ADD COLUMN IF NOT EXISTS project_id INT REFERENCES projects(id) ON DELETE CASCADE;
ALTER TABLE releases ADD COLUMN IF NOT EXISTS status TEXT DEFAULT 'new';
ALTER TABLE releases ADD COLUMN IF NOT EXISTS urgency_score FLOAT;
ALTER TABLE releases ADD COLUMN IF NOT EXISTS version_info JSONB;
ALTER TABLE releases ADD COLUMN IF NOT EXISTS agent_report JSONB;
ALTER TABLE releases ADD COLUMN IF NOT EXISTS version TEXT;
ALTER TABLE releases ADD COLUMN IF NOT EXISTS updated_at TIMESTAMPTZ DEFAULT NOW();

-- Backfill project_id from sources
UPDATE releases SET project_id = s.project_id FROM sources s WHERE releases.source_id = s.id AND releases.project_id IS NULL;

-- Backfill version from payload (the raw_version field in the JSONB payload)
UPDATE releases SET version = payload->>'raw_version' WHERE version IS NULL;

-- Make project_id NOT NULL after backfill
ALTER TABLE releases ALTER COLUMN project_id SET NOT NULL;

-- Add new unique constraint (project_id, version)
-- Drop old unique constraint first
ALTER TABLE releases DROP CONSTRAINT IF EXISTS releases_source_id_version_key;
-- Create new one (use DO block for idempotency)
DO $$ BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'releases_project_id_version_key') THEN
        ALTER TABLE releases ADD CONSTRAINT releases_project_id_version_key UNIQUE(project_id, version);
    END IF;
END $$;

-- 4. Create agent_sessions table
CREATE TABLE IF NOT EXISTS agent_sessions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    release_id UUID NOT NULL REFERENCES releases(id) ON DELETE CASCADE,
    project_id INT NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    status TEXT DEFAULT 'active',
    tool_calls JSONB DEFAULT '[]',
    findings JSONB DEFAULT '{}',
    idle_deadline TIMESTAMPTZ,
    started_at TIMESTAMPTZ DEFAULT NOW(),
    completed_at TIMESTAMPTZ
);
CREATE INDEX IF NOT EXISTS idx_agent_sessions_release ON agent_sessions(release_id);
CREATE INDEX IF NOT EXISTS idx_agent_sessions_project ON agent_sessions(project_id);

-- 5. Drop pipeline_jobs table
DROP TABLE IF EXISTS pipeline_jobs;

-- 6. Update releases trigger to include project_id in notification
CREATE OR REPLACE FUNCTION notify_release_created() RETURNS trigger AS $$
BEGIN
    PERFORM pg_notify('release_events', NEW.id::text);
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

-- 7. Remove poll_interval_seconds from sources (now on project)
-- Keep column for now for backwards compat during migration, mark as deprecated
-- We'll stop reading it in code
```

**Step 3: Run migration test**

Run: `go build ./cmd/server && echo "migration compiles"`

**Step 4: Commit**

```bash
git add internal/db/migrations.go
git commit -m "feat: add schema migration for project-centric redesign"
```

---

### Task 2: Update Go models

**Files:**
- Modify: `internal/models/project.go`
- Modify: `internal/models/source.go`
- Modify: `internal/models/release.go` (the existing ReleaseEvent IR stays, we add a new Release model)
- Create: `internal/models/source_release.go`
- Modify: `internal/models/subscription.go`
- Remove references to pipeline config

**Step 1: Update Project model**

In `internal/models/project.go`, update the struct:

```go
type Project struct {
	ID           int       `json:"id"`
	Name         string    `json:"name"`
	Description  string    `json:"description,omitempty"`
	URL          string    `json:"url,omitempty"`
	PollInterval string    `json:"poll_interval"`     // e.g., "15m"
	IdleWindow   string    `json:"idle_window"`       // e.g., "6h"
	AgentEnabled bool      `json:"agent_enabled"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}
```

Remove `PipelineConfig json.RawMessage` field.

**Step 2: Create SourceRelease model**

Create `internal/models/source_release.go`:

```go
package models

import (
	"encoding/json"
	"time"
)

type SourceRelease struct {
	ID           string          `json:"id"`
	SourceID     int             `json:"source_id"`
	ProjectID    int             `json:"project_id"`
	ReleaseID    *string         `json:"release_id,omitempty"` // nullable until correlated
	RawVersion   string          `json:"raw_version"`
	ReleaseNotes string          `json:"release_notes,omitempty"`
	Metadata     json.RawMessage `json:"metadata,omitempty"`
	DetectedAt   time.Time       `json:"detected_at"`
}
```

**Step 3: Create Release model (project-scoped, distinct from ReleaseEvent IR)**

Add to `internal/models/release.go` (keep existing ReleaseEvent):

```go
// Release is the project-scoped correlated release view.
type Release struct {
	ID           string          `json:"id"`
	ProjectID    int             `json:"project_id"`
	Version      string          `json:"version"`       // normalized semver
	Status       string          `json:"status"`        // "new", "investigating", "complete"
	UrgencyScore *float64        `json:"urgency_score,omitempty"`
	VersionInfo  json.RawMessage `json:"version_info,omitempty"`
	AgentReport  json.RawMessage `json:"agent_report,omitempty"`
	CreatedAt    time.Time       `json:"created_at"`
	UpdatedAt    time.Time       `json:"updated_at"`
}
```

**Step 4: Verify compilation**

Run: `go build ./...`

**Step 5: Commit**

```bash
git add internal/models/
git commit -m "feat: update models for project-centric schema"
```

---

### Task 3: Remove pipeline package (runner, worker, subscription_router)

The regex normalizer and urgency scorer logic will be extracted as utilities. The pipeline runner, worker, and subscription router are being removed.

**Files:**
- Keep: `internal/pipeline/regex_normalizer.go` and its test
- Keep: `internal/pipeline/urgency_scorer.go` and its test
- Remove: `internal/pipeline/runner.go`, `internal/pipeline/runner_test.go`
- Remove: `internal/pipeline/worker.go`, `internal/pipeline/worker_test.go`
- Remove: `internal/pipeline/subscription_router.go`, `internal/pipeline/subscription_router_test.go`
- Remove: `internal/pipeline/node.go`, `internal/pipeline/node_test.go`
- Remove: `internal/pipeline/store.go`
- Remove: `internal/pipeline/pgstore.go`
- Remove: `internal/queue/` (pipeline job args no longer needed)
- Modify: `cmd/server/main.go` — remove pipeline and River setup

**Step 1: Refactor regex_normalizer to be callable as a utility**

Currently `RegexNormalizer.Execute()` takes a `ReleaseEvent` and pipeline context. Extract the version parsing logic into a standalone function:

```go
// ParseVersion extracts semantic version from a raw version string.
// Returns the parsed SemanticData, whether it's a pre-release, and whether parsing succeeded.
func ParseVersion(rawVersion string) (SemanticData, bool, bool) {
    // ... existing regex logic from Execute()
}
```

Keep `RegexNormalizer` struct for backwards compat but have its `Execute` call `ParseVersion` internally. Actually — since we're removing the pipeline `PipelineNode` interface, we can just keep `ParseVersion` as the public API and remove the struct.

Rename file to `internal/pipeline/version.go` (or move to `internal/correlation/version.go` — decide in Step 2).

**Step 2: Refactor urgency_scorer similarly**

Extract a standalone function:

```go
// ScoreUrgency computes urgency based on semantic version and changelog.
func ScoreUrgency(sem SemanticData, isPreRelease bool, changelog string) (score string, factors []string) {
    // ... existing scoring logic from Execute()
}
```

Rename file to `internal/pipeline/urgency.go` or move.

**Step 3: Remove pipeline runner, worker, node interface, store, pgstore, subscription_router, and queue package**

Delete the files listed above.

**Step 4: Update `cmd/server/main.go`**

Remove:
- Pipeline runner creation (lines ~41-51)
- River workers setup (lines ~54-66)
- River client start
- Pipeline worker registration
- References to `internal/pipeline` runner/worker
- References to `internal/queue`

Keep:
- Database setup
- Ingestion setup (will be updated in later task)
- API setup
- HTTP server

The main.go should become simpler — just DB, ingestion, API, server.

**Step 5: Verify compilation**

Run: `go build ./... && go test ./internal/pipeline/...`

**Step 6: Commit**

```bash
git add -A
git commit -m "refactor: remove fixed pipeline, extract version parser and urgency scorer as utilities"
```

---

## Phase 2: Correlation Engine & Ingestion Update

### Task 4: Create correlation package

**Files:**
- Create: `internal/correlation/correlator.go`
- Create: `internal/correlation/correlator_test.go`
- Move: `internal/pipeline/regex_normalizer.go` → `internal/correlation/version.go`
- Move: `internal/pipeline/urgency_scorer.go` → `internal/correlation/urgency.go`
- Move their tests too

**Step 1: Write failing test for version normalization utility**

```go
// internal/correlation/version_test.go
func TestParseVersion(t *testing.T) {
    tests := []struct {
        raw     string
        wantSem models.SemanticData
        wantPre bool
        wantOK  bool
    }{
        {"v1.2.3", models.SemanticData{Major: 1, Minor: 2, Patch: 3}, false, true},
        {"1.2.3-rc1", models.SemanticData{Major: 1, Minor: 2, Patch: 3, PreRelease: "rc1"}, true, true},
        {"REL_16_3", models.SemanticData{}, false, false}, // won't match standard regex
    }
    for _, tt := range tests {
        sem, isPre, ok := ParseVersion(tt.raw)
        // assertions
    }
}
```

Run: `go test ./internal/correlation/... -v` — expect FAIL

**Step 2: Implement ParseVersion**

Move and refactor regex normalizer logic into `internal/correlation/version.go`.

Run: `go test ./internal/correlation/... -v` — expect PASS

**Step 3: Write failing test for correlator**

The correlator's job: given a new SourceRelease, find or create the matching project-scoped Release.

```go
// internal/correlation/correlator_test.go
func TestCorrelator_Correlate(t *testing.T) {
    // Test 1: new version creates new Release
    // Test 2: same normalized version links to existing Release
    // Test 3: different raw versions that normalize to same semver get linked
}
```

**Step 4: Implement Correlator**

```go
// internal/correlation/correlator.go
type CorrelationStore interface {
    FindReleaseByVersion(ctx context.Context, projectID int, version string) (*models.Release, error)
    CreateRelease(ctx context.Context, r *models.Release) error
    LinkSourceRelease(ctx context.Context, sourceReleaseID string, releaseID string) error
}

type Correlator struct {
    store CorrelationStore
}

func NewCorrelator(store CorrelationStore) *Correlator {
    return &Correlator{store: store}
}

// Correlate takes a new SourceRelease and returns the correlated Release (found or created).
func (c *Correlator) Correlate(ctx context.Context, sr *models.SourceRelease) (*models.Release, error) {
    // 1. Parse version
    sem, isPre, ok := ParseVersion(sr.RawVersion)

    // 2. Build normalized version string
    var version string
    if ok {
        version = sem.String()
    } else {
        version = sr.RawVersion // fallback to raw
    }

    // 3. Look for existing release with same project + version
    existing, err := c.store.FindReleaseByVersion(ctx, sr.ProjectID, version)
    if err != nil && !errors.Is(err, ErrNotFound) {
        return nil, err
    }

    if existing != nil {
        // Link this source release to existing
        c.store.LinkSourceRelease(ctx, sr.ID, existing.ID)
        return existing, nil
    }

    // 4. Create new release
    score, factors := ScoreUrgency(sem, isPre, sr.ReleaseNotes)
    release := &models.Release{
        ProjectID:    sr.ProjectID,
        Version:      version,
        Status:       "new",
        UrgencyScore: &urgencyFloat, // convert score string to float
        VersionInfo:  marshalVersionInfo(sem, isPre, factors),
    }
    err = c.store.CreateRelease(ctx, release)
    if err != nil {
        return nil, err
    }

    c.store.LinkSourceRelease(ctx, sr.ID, release.ID)
    return release, nil
}
```

Run: `go test ./internal/correlation/... -v` — expect PASS

**Step 5: Commit**

```bash
git add internal/correlation/
git commit -m "feat: add correlation engine for cross-source release linking"
```

---

### Task 5: Update ingestion layer for new schema

**Files:**
- Modify: `internal/ingestion/service.go`
- Modify: `internal/ingestion/pgstore.go`
- Modify: `internal/ingestion/store.go`
- Modify: `internal/ingestion/orchestrator.go`
- Modify: `internal/ingestion/loader.go`
- Update tests

The ingestion layer now:
1. Stores raw data as `SourceRelease` (instead of directly into `releases`)
2. Calls the `Correlator` to link source releases to project-scoped releases
3. No longer enqueues River pipeline jobs

**Step 1: Update ReleaseStore interface**

```go
// internal/ingestion/store.go
type ReleaseStore interface {
    IngestSourceRelease(ctx context.Context, sourceID int, projectID int, result *IngestionResult) (*models.SourceRelease, error)
}
```

**Step 2: Update PgStore**

```go
// internal/ingestion/pgstore.go
func (s *PgStore) IngestSourceRelease(ctx context.Context, sourceID int, projectID int, result *IngestionResult) (*models.SourceRelease, error) {
    sr := &models.SourceRelease{}
    err := s.pool.QueryRow(ctx,
        `INSERT INTO source_releases (source_id, project_id, raw_version, release_notes, metadata, detected_at)
         VALUES ($1, $2, $3, $4, $5, $6)
         RETURNING id, source_id, project_id, raw_version, release_notes, metadata, detected_at`,
        sourceID, projectID, result.RawVersion, result.Changelog,
        metadataJSON(result.Metadata), result.Timestamp,
    ).Scan(&sr.ID, &sr.SourceID, &sr.ProjectID, &sr.RawVersion, &sr.ReleaseNotes, &sr.Metadata, &sr.DetectedAt)
    if err != nil {
        return nil, err
    }
    return sr, nil
}
```

The PgStore no longer needs a River client. Remove that dependency.

**Step 3: Update Service to use correlator**

```go
// internal/ingestion/service.go
type Service struct {
    store      ReleaseStore
    correlator *correlation.Correlator
}

func (s *Service) ProcessResults(ctx context.Context, sourceID, projectID int, sourceName string, results []IngestionResult) error {
    for _, r := range results {
        // 1. Store as SourceRelease
        sr, err := s.store.IngestSourceRelease(ctx, sourceID, projectID, &r)
        if err != nil {
            // Skip duplicates (unique constraint violation)
            continue
        }
        // 2. Correlate to project-level Release
        _, err = s.correlator.Correlate(ctx, sr)
        if err != nil {
            log.Printf("correlation error: %v", err)
        }
    }
    return nil
}
```

**Step 4: Update Orchestrator to pass projectID**

The orchestrator needs to know the project_id for each source. Update `IIngestionSource` to include `ProjectID()`:

```go
type IIngestionSource interface {
    Name() string
    SourceID() int
    ProjectID() int  // NEW
    FetchNewReleases(ctx context.Context) ([]IngestionResult, error)
}
```

Update `DockerHubSource` and `GitHubWebhookHandler` accordingly.

Update `SourceLoader.LoadEnabledSources()` to read project_id from the sources query and pass it to source constructors.

**Step 5: Update orchestrator's pollAll**

```go
func (o *Orchestrator) pollAll(ctx context.Context) {
    sources, err := o.loader.LoadEnabledSources(ctx)
    // ...
    for _, src := range sources {
        results, err := src.FetchNewReleases(ctx)
        // ...
        o.service.ProcessResults(ctx, src.SourceID(), src.ProjectID(), src.Name(), results)
    }
}
```

**Step 6: Update tests**

Update `internal/ingestion/service_test.go`, `orchestrator_test.go` to use new interface.

Run: `go test ./internal/ingestion/... -v`

**Step 7: Commit**

```bash
git add internal/ingestion/
git commit -m "feat: update ingestion layer to store source_releases and use correlator"
```

---

### Task 6: Update main.go

**Files:**
- Modify: `cmd/server/main.go`

**Step 1: Remove pipeline and River setup**

Remove:
- Pipeline runner creation
- River workers and client
- Pipeline worker registration

**Step 2: Add correlation setup**

```go
// Create correlation store + correlator
corrStore := correlation.NewPgStore(pool)
correlator := correlation.NewCorrelator(corrStore)

// Create ingestion service with correlator
ingestionStore := ingestion.NewPgStore(pool) // no River client
ingestionService := ingestion.NewService(ingestionStore, correlator)
```

**Step 3: Simplify startup**

The server now starts: DB → migrations → correlator → ingestion → API → HTTP.

Run: `go build ./cmd/server`

**Step 4: Commit**

```bash
git add cmd/server/main.go
git commit -m "refactor: simplify main.go, remove pipeline/River, add correlator"
```

---

## Phase 3: API Restructure

### Task 7: Update API store interfaces and pgstore

**Files:**
- Modify: `internal/api/projects.go` — update ProjectsStore interface
- Modify: `internal/api/releases.go` — rewrite for project-scoped releases
- Modify: `internal/api/sources.go` — update for project-scoped sources
- Modify: `internal/api/pgstore.go` — rewrite queries
- Modify: `internal/api/server.go` — update routes
- Modify: `internal/api/health.go` — update stats

**Step 1: Update ProjectsStore to include sources inline**

The project GET endpoint should return sources inline. Update the store interface:

```go
type ProjectsStore interface {
    ListProjects(ctx context.Context, page, perPage int) ([]models.Project, int, error)
    CreateProject(ctx context.Context, p *models.Project) error
    GetProject(ctx context.Context, id int) (*models.Project, error)
    UpdateProject(ctx context.Context, id int, p *models.Project) error
    DeleteProject(ctx context.Context, id int) error
    // NEW: get sources for a project
    ListProjectSources(ctx context.Context, projectID int) ([]models.Source, error)
    // NEW: get releases for a project
    ListProjectReleases(ctx context.Context, projectID int, page, perPage int) ([]ProjectRelease, int, error)
    // NEW: get a release with its source_releases
    GetProjectRelease(ctx context.Context, projectID int, releaseID string) (*ProjectReleaseDetail, error)
}
```

Where `ProjectRelease` is a view model:

```go
type ProjectRelease struct {
    ID           string   `json:"id"`
    Version      string   `json:"version"`
    Status       string   `json:"status"`
    UrgencyScore *float64 `json:"urgency_score,omitempty"`
    SourceCount  int      `json:"source_count"`     // how many sources detected this
    SourceTypes  []string `json:"source_types"`     // ["dockerhub", "github"]
    CreatedAt    string   `json:"created_at"`
}

type ProjectReleaseDetail struct {
    ProjectRelease
    VersionInfo    json.RawMessage       `json:"version_info,omitempty"`
    AgentReport    json.RawMessage       `json:"agent_report,omitempty"`
    SourceReleases []models.SourceRelease `json:"source_releases"` // per-source raw data
}
```

**Step 2: Update route structure**

Remove standalone `/api/v1/releases` and `/api/v1/sources` top-level routes.
Add project-nested routes:

```
GET    /api/v1/projects                           — list projects
POST   /api/v1/projects                           — create project
GET    /api/v1/projects/{id}                       — get project (includes sources)
PUT    /api/v1/projects/{id}                       — update project
DELETE /api/v1/projects/{id}                       — delete project
GET    /api/v1/projects/{id}/sources               — list sources for project
POST   /api/v1/projects/{id}/sources               — add source to project
PUT    /api/v1/projects/{id}/sources/{sourceId}    — update source
DELETE /api/v1/projects/{id}/sources/{sourceId}    — remove source
GET    /api/v1/projects/{id}/releases              — list releases for project
GET    /api/v1/projects/{id}/releases/{releaseId}  — get release detail (with source_releases)

GET    /api/v1/channels                            — unchanged
POST   /api/v1/channels                            — unchanged
GET    /api/v1/channels/{id}                       — unchanged
PUT    /api/v1/channels/{id}                       — unchanged
DELETE /api/v1/channels/{id}                       — unchanged

GET    /api/v1/subscriptions                       — keep (may filter by project_id)
POST   /api/v1/subscriptions                       — keep
GET    /api/v1/subscriptions/{id}                  — keep
PUT    /api/v1/subscriptions/{id}                  — keep
DELETE /api/v1/subscriptions/{id}                  — keep

GET    /api/v1/health                              — unchanged
GET    /api/v1/stats                               — update stats
GET    /api/v1/events                              — unchanged
GET    /api/v1/providers                           — unchanged
POST   /webhook/github                             — unchanged
```

**Step 3: Implement pgstore methods for new queries**

Key new queries:

```sql
-- ListProjectReleases
SELECT r.id, r.version, r.status, r.urgency_score, r.created_at,
       COUNT(sr.id) as source_count,
       ARRAY_AGG(DISTINCT s.source_type) as source_types
FROM releases r
LEFT JOIN source_releases sr ON sr.release_id = r.id
LEFT JOIN sources s ON s.id = sr.source_id
WHERE r.project_id = $1
GROUP BY r.id
ORDER BY r.created_at DESC
LIMIT $2 OFFSET $3

-- GetProjectReleaseDetail
-- Release + all linked source_releases
SELECT r.*, sr.id, sr.source_id, sr.raw_version, sr.release_notes, sr.metadata, sr.detected_at
FROM releases r
LEFT JOIN source_releases sr ON sr.release_id = r.id
WHERE r.id = $1 AND r.project_id = $2
```

**Step 4: Update pgstore project queries**

Update `ListProjects` to include source count:
```sql
SELECT p.*, COUNT(s.id) as source_count
FROM projects p
LEFT JOIN sources s ON s.project_id = p.id
GROUP BY p.id
ORDER BY p.created_at DESC
```

Update `CreateProject` / `UpdateProject` to handle new columns (poll_interval, idle_window, agent_enabled). Remove `pipeline_config`.

**Step 5: Remove old releases/sources handler files or gut them**

- `internal/api/releases.go` — Rewrite handlers for project-nested routes
- `internal/api/sources.go` — Rewrite handlers for project-nested routes
- Remove pipeline-related views from releases (PipelineStatus, pipeline endpoint)

**Step 6: Update server.go RegisterRoutes**

Remove old top-level release/source routes. Add new nested routes.

**Step 7: Update stats query**

Remove `pending_jobs` and `failed_jobs` (no pipeline). Add `active_agents` count:

```go
type DashboardStats struct {
    TotalProjects  int `json:"total_projects"`
    TotalReleases  int `json:"total_releases"`
    ActiveSources  int `json:"active_sources"`
    ActiveAgents   int `json:"active_agents"`
}
```

**Step 8: Run tests, fix what breaks**

Run: `go test ./internal/api/... -v`

Many tests will break because interfaces changed. Update the mock stores in test files to match new interfaces.

**Step 9: Verify build**

Run: `go build ./cmd/server`

**Step 10: Commit**

```bash
git add internal/api/ cmd/server/
git commit -m "feat: restructure API for project-centric routes"
```

---

## Phase 4: Frontend Restructure

### Task 8: Update TypeScript types and API client

**Files:**
- Modify: `web/lib/api/types.ts`
- Modify: `web/lib/api/client.ts`

**Step 1: Update types**

```typescript
// Updated Project
interface Project {
  id: number;
  name: string;
  description: string;
  url: string;
  poll_interval: string;    // "15m"
  idle_window: string;      // "6h"
  agent_enabled: boolean;
  source_count?: number;
  created_at: string;
  updated_at: string;
}

interface ProjectInput {
  name: string;
  description?: string;
  url?: string;
  poll_interval?: string;
  idle_window?: string;
  agent_enabled?: boolean;
}

// New: SourceRelease (raw per-source data)
interface SourceRelease {
  id: string;
  source_id: number;
  project_id: number;
  release_id?: string;
  raw_version: string;
  release_notes?: string;
  metadata?: Record<string, unknown>;
  detected_at: string;
}

// Updated Release (project-scoped)
interface Release {
  id: string;
  project_id: number;
  version: string;
  status: "new" | "investigating" | "complete";
  urgency_score?: number;
  source_count: number;
  source_types: string[];
  created_at: string;
}

// New: Release detail with source releases
interface ReleaseDetail extends Release {
  version_info?: Record<string, unknown>;
  agent_report?: AgentReport;
  source_releases: SourceRelease[];
}

// New: Agent report
interface AgentReport {
  summary: string;
  availability?: Record<string, boolean>;
  adoption?: Record<string, unknown>;
  risk_assessment?: string;
  recommendations?: string[];
  completed_at: string;
}
```

Remove: `PipelineStatus`, pipeline-related types.

**Step 2: Update API client**

```typescript
const api = {
  projects: {
    list: (page?, perPage?) => request<Project[]>(`/projects?...`),
    get: (id) => request<Project>(`/projects/${id}`),
    create: (input) => request<Project>(`/projects`, { method: "POST", body: input }),
    update: (id, input) => request<Project>(`/projects/${id}`, { method: "PUT", body: input }),
    delete: (id) => request<void>(`/projects/${id}`, { method: "DELETE" }),

    // Nested sources
    listSources: (projectId) => request<Source[]>(`/projects/${projectId}/sources`),
    createSource: (projectId, input) => request<Source>(`/projects/${projectId}/sources`, { method: "POST", body: input }),
    updateSource: (projectId, sourceId, input) => request<Source>(`/projects/${projectId}/sources/${sourceId}`, { method: "PUT", body: input }),
    deleteSource: (projectId, sourceId) => request<void>(`/projects/${projectId}/sources/${sourceId}`, { method: "DELETE" }),

    // Nested releases
    listReleases: (projectId, params?) => request<Release[]>(`/projects/${projectId}/releases?...`),
    getRelease: (projectId, releaseId) => request<ReleaseDetail>(`/projects/${projectId}/releases/${releaseId}`),
  },

  // Remove top-level sources and releases
  // Keep subscriptions, channels, system
};
```

**Step 3: Commit**

```bash
git add web/lib/api/
git commit -m "feat: update frontend types and API client for project-centric model"
```

---

### Task 9: Update mock data and handlers

**Files:**
- Modify: `web/lib/mock/data.ts`
- Modify: `web/lib/mock/handlers.ts`

**Step 1: Update mock data**

Update `mockProjects` to include new fields (`poll_interval`, `idle_window`, `agent_enabled`). Remove `pipeline_config`.

Add `mockSourceReleases` — per-source raw releases linked to mock releases.

Update `mockReleases` to be project-scoped with `version`, `status`, `urgency_score`, `source_count`, `source_types`.

Update `mockStats` to use new stats shape.

**Step 2: Update mock handlers**

Remove handlers for:
- `GET /api/v1/releases` (top-level)
- `GET /api/v1/releases/:id`
- `GET /api/v1/releases/:id/pipeline`
- `GET /api/v1/releases/:id/notes`
- `GET /api/v1/sources` (top-level)
- `POST /api/v1/sources`
- `GET /api/v1/sources/:id`
- `PUT /api/v1/sources/:id`
- `DELETE /api/v1/sources/:id`
- `GET /api/v1/sources/:id/latest-release`

Add handlers for:
- `GET /api/v1/projects/:id/sources`
- `POST /api/v1/projects/:id/sources`
- `PUT /api/v1/projects/:id/sources/:sourceId`
- `DELETE /api/v1/projects/:id/sources/:sourceId`
- `GET /api/v1/projects/:id/releases`
- `GET /api/v1/projects/:id/releases/:releaseId` — returns release + source_releases

**Step 3: Verify MSW works**

Run: `cd web && npm run build`

**Step 4: Commit**

```bash
git add web/lib/mock/
git commit -m "feat: update mock data and handlers for project-centric API"
```

---

### Task 10: Restructure sidebar and navigation

**Files:**
- Modify: `web/components/layout/sidebar.tsx`
- Modify: `web/components/layout/header.tsx`
- Delete: `web/app/sources/` (entire directory)
- Delete: `web/app/releases/` (entire directory)

**Step 1: Update sidebar nav items**

```typescript
const navItems = [
  { href: "/", label: "Dashboard", icon: LayoutDashboard },
  { href: "/projects", label: "Projects", icon: FolderKanban },
  { href: "/channels", label: "Channels", icon: Megaphone },
];
```

Remove: Releases, Sources, Subscriptions from sidebar.

**Step 2: Update header title mapping**

Remove entries for /releases, /sources, /subscriptions routes.

**Step 3: Delete standalone pages**

Delete entire directories:
- `web/app/sources/`
- `web/app/releases/`

Subscriptions pages can be deleted too — subscriptions will be managed within project detail:
- `web/app/subscriptions/`

**Step 4: Delete standalone components**

These will be rebuilt inline in the project detail:
- `web/components/sources/source-form.tsx`
- `web/components/sources/source-edit.tsx`
- `web/components/releases/release-detail.tsx`
- `web/components/releases/pipeline-status.tsx`
- `web/components/subscriptions/subscription-form.tsx`
- `web/components/subscriptions/subscription-edit.tsx`

**Step 5: Verify build**

Run: `cd web && npm run build`

**Step 6: Commit**

```bash
git add -A
git commit -m "refactor: simplify navigation, remove standalone source/release/subscription pages"
```

---

### Task 11: Rebuild project detail page

This is the core UX change — the project detail page becomes the main workhorse.

**Files:**
- Rewrite: `web/app/projects/[id]/page.tsx`
- Rewrite: `web/components/projects/project-detail.tsx`
- Create: `web/components/projects/source-panel.tsx`
- Create: `web/components/projects/release-list.tsx`
- Create: `web/components/projects/release-popup.tsx`
- Create: `web/components/projects/subscription-tab.tsx`

**Step 1: Design project detail page layout**

The page has three tabs: Overview (releases), Sources, Notifications.

```
┌─────────────────────────────────────────────────┐
│ Project Name          [Edit] [Agent: ON/OFF]    │
│ Description                                      │
│ Poll: 15m | Idle: 6h | Sources: 3               │
├─────────────────────────────────────────────────┤
│ [Releases] [Sources] [Notifications]             │
├─────────────────────────────────────────────────┤
│ Filter: [All sources ▼]                          │
│                                                   │
│ ┌─ v16.3.0  ── [dockerhub] [github] ── HIGH ──┐ │
│ │  Status: Complete  |  2 sources  |  2h ago   │ │
│ └──────────────────────────────────────────────┘ │
│ ┌─ v16.2.1  ── [dockerhub] ── LOW ────────────┐ │
│ │  Status: New  |  1 source  |  1d ago         │ │
│ └──────────────────────────────────────────────┘ │
└─────────────────────────────────────────────────┘
```

**Step 2: Implement source-panel component**

List of sources for this project with add/remove buttons. Uses `api.projects.listSources(projectId)`.

**Step 3: Implement release-list component**

Table of project releases with:
- Source type dropdown filter
- Version, source badges, urgency score, status, timestamp
- Click handler to open release popup

Uses `api.projects.listReleases(projectId)`.

**Step 4: Implement release-popup component**

Dialog/sheet that shows:
- Per-source release notes (tabs by source)
- Deterministic report: urgency, version breakdown
- Agent report section (if available)

Uses `api.projects.getRelease(projectId, releaseId)`.

**Step 5: Implement subscription-tab component**

List of subscriptions for this project with inline add/edit/remove. Uses existing subscription API filtered by project_id.

**Step 6: Wire it all together in project detail page**

The page uses `Tabs` component from shadcn/ui with three tab panels.

**Step 7: Verify build**

Run: `cd web && npm run build`

**Step 8: Commit**

```bash
git add web/
git commit -m "feat: rebuild project detail page with releases, sources, and subscriptions tabs"
```

---

### Task 12: Update project create/edit forms

**Files:**
- Modify: `web/components/projects/project-form.tsx`
- Modify: `web/app/projects/new/page.tsx`
- Modify: `web/app/projects/[id]/edit/page.tsx`

**Step 1: Update ProjectForm fields**

Replace `pipeline_config` JSON textarea with:
- `poll_interval` — select or input (5m, 15m, 30m, 1h, 6h, 24h)
- `idle_window` — select or input (1h, 6h, 12h, 24h)
- `agent_enabled` — switch toggle

Add inline source management:
- List of sources with type + repository fields
- Add/remove source rows
- On create: POST project, then POST each source
- On edit: PUT project, then reconcile sources (create new, delete removed)

**Step 2: Update create/edit pages**

Wire up the new form fields and source management.

**Step 3: Verify build**

Run: `cd web && npm run build`

**Step 4: Commit**

```bash
git add web/
git commit -m "feat: update project form with poll interval, idle window, agent toggle, and inline sources"
```

---

### Task 13: Update dashboard

**Files:**
- Modify: `web/components/dashboard/stats-cards.tsx`
- Modify: `web/components/dashboard/recent-releases.tsx`
- Modify: `web/components/dashboard/activity-feed.tsx`
- Modify: `web/app/page.tsx`

**Step 1: Update stats cards**

New stats: Total Projects, Total Releases, Active Sources, Active Agents.

**Step 2: Update recent releases**

Show recent releases across all projects. Each row: project name, version, source badges, urgency, status. Click navigates to project detail page.

Note: We need a global releases endpoint for the dashboard. Add `GET /api/v1/releases` back as a read-only global view (no CRUD) that returns releases across all projects. This is for the dashboard only.

**Step 3: Update activity feed**

Update SSE event types to include project context. The feed items should show project name.

**Step 4: Verify build**

Run: `cd web && npm run build`

**Step 5: Commit**

```bash
git add web/
git commit -m "feat: update dashboard for project-centric model"
```

---

## Phase 5: Cleanup & Verification

### Task 14: Clean up dead code and verify

**Files:**
- Remove: `internal/pipeline/` (anything remaining that's dead code)
- Remove: `internal/queue/` (if not already removed)
- Verify: all Go tests pass
- Verify: frontend builds cleanly

**Step 1: Run all Go tests**

Run: `go test ./... -v`

Fix any remaining test failures from interface changes.

**Step 2: Run Go vet**

Run: `go vet ./...`

**Step 3: Build frontend**

Run: `cd web && npm run build`

**Step 4: Build Go binary**

Run: `go build -o releaseguard ./cmd/server`

**Step 5: Final commit**

```bash
git add -A
git commit -m "chore: clean up dead code and verify full build"
```

---

## Future Work (Separate Plan)

These items are out of scope for this plan and will get their own design + implementation plan:

1. **Agent Layer** — `internal/agent/` package with session management, LLM tool definitions, idle timer, API integration
2. **Advanced agent tools** — check_availability, check_adoption, generate_report
3. **Agent UI** — investigating status, progress indicators, agent report rendering
4. **Notification routing** — actually sending notifications via channels when releases complete
5. **Real SSE events** — update PostgreSQL NOTIFY to include richer event data for the new schema
