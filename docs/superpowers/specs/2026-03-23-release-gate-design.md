# Release Gate Design

**Date:** 2026-03-23
**Status:** Draft

## Problem

Users want release notifications optionally postponed until all relevant sources for a version are available. For example, a third-party Docker image may lag months behind a GitHub release. Today, notifications fire immediately per-source and the semantic report runs independently — resulting in two separate notifications and potentially incomplete analysis.

## Goals

1. Allow multi-source projects to delay agent analysis until all (or critical) sources report a version
2. Unify release + semantic report into a single notification for users who prefer it
3. Support configurable version mapping across sources (GitHub `v1.21.0` ↔ Docker Hub `1.21.0`)
4. Provide hybrid readiness rules: structured (deterministic) + natural language (LLM-evaluated)
5. Timeout gracefully with partial reports when sources never arrive
6. No behavior change for single-source projects or users who want immediate notifications

## Non-Goals

- Changing the existing `source_release` / `semantic_release` subscription model
- Replacing the agent rules system (gate controls timing; agent rules control whether to run)
- Real-time push notifications for gate status changes (polling/dashboard only)

## Design

### Subscription Model (Unchanged)

The existing subscription types handle both use cases:

| Type | Behavior |
|------|----------|
| `source_release` | Fires immediately when a source detects a release (unchanged) |
| `semantic_release` | Fires when the semantic report is completed for the version |

Users who want the unified gated notification simply subscribe to `semantic_release`. Users who want raw speed keep `source_release`. Both can coexist on the same project.

> **Note:** The migration code uses `source_release` / `semantic_release` as subscription type values. `DESIGN.md` references older names (`source` / `project`). `DESIGN.md` should be updated to match the migration as part of this work.

### Data Model

#### `release_gates` table

Per-project gate configuration. One gate per project (1:1).

```sql
CREATE TABLE release_gates (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    project_id UUID NOT NULL UNIQUE REFERENCES projects(id) ON DELETE CASCADE,
    required_sources JSONB,               -- JSON array of source UUIDs; null/empty = all
    timeout_hours INT NOT NULL DEFAULT 168, -- 7 days default
    version_mapping JSONB,                -- per-source version transform rules
    nl_rule TEXT,                          -- optional NL condition for LLM evaluation
    enabled BOOLEAN NOT NULL DEFAULT true,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
```

**Fields:**

- `required_sources`: JSONB array of source UUID strings (e.g., `["uuid-1", "uuid-2"]`). Uses JSONB rather than `UUID[]` for consistency with the rest of the schema — all flexible fields use JSONB. Go-side: unmarshal to `[]string`. If null/empty, all project sources are required.
- `timeout_hours`: how long to wait before force-opening the gate. Default 7 days (168 hours).
- `version_mapping`: per-source regex/template transforms to normalize versions across providers.
- `nl_rule`: optional free-text condition evaluated by LLM when structured rules pass.

#### `version_readiness` table

Per-version gate state tracking.

```sql
CREATE TABLE version_readiness (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    project_id UUID NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    version VARCHAR(100) NOT NULL,         -- normalized version string
    status VARCHAR(20) NOT NULL DEFAULT 'pending'
        CHECK (status IN ('pending', 'ready', 'timed_out')),
    sources_met JSONB NOT NULL DEFAULT '[]',     -- JSON array of source UUIDs
    sources_missing JSONB NOT NULL DEFAULT '[]', -- JSON array of source UUIDs
    nl_rule_passed BOOLEAN,               -- null = not evaluated yet
    timeout_at TIMESTAMPTZ NOT NULL,
    opened_at TIMESTAMPTZ,
    agent_triggered BOOLEAN NOT NULL DEFAULT false,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE(project_id, version)
);

CREATE INDEX idx_version_readiness_timeout
    ON version_readiness(timeout_at) WHERE status = 'pending';
```

> **JSONB arrays instead of `UUID[]`:** The existing schema consistently uses JSONB for flexible/array-like fields. We follow the same convention. Go-side scanning uses `json.RawMessage` or `[]string`, consistent with how `raw_data`, `config`, and `agent_rules` fields are handled elsewhere.

### Version Mapping

The `version_mapping` JSONB defines how to normalize versions across sources:

```json
{
  "<github_source_id>": {
    "pattern": "^v?(.+)$",
    "template": "$1"
  },
  "<docker_source_id>": {
    "pattern": "^(\\d+\\.\\d+\\.\\d+).*$",
    "template": "$1"
  }
}
```

- When a release arrives, apply the source's `pattern` regex and `template` substitution to produce a normalized version.
- If no mapping exists for a source, default: strip `v` prefix, lowercase.
- The normalized version is the key in `version_readiness(project_id, version)`.

**Multi-variant tags:** Source-level version filters (`version_filter_include` / `version_filter_exclude` on the `sources` table) are applied during ingestion *before* the gate check. For Docker Hub sources tracking multiple tag variants (e.g., `1.21.0-alpine`, `1.21.0-bookworm`), configure the source's `version_filter_include` to match only the canonical tag pattern (e.g., `^\d+\.\d+\.\d+$`), ensuring only `1.21.0` enters the gate pipeline. The version mapping then normalizes across sources. This avoids the gate needing to understand tag variants — existing source filters handle that.

### Flows

#### Single-Source Project (No Gate)

No behavior change:

```
Release ingested
  → source_release subscribers notified immediately
  → Agent rules checked → agent runs immediately
  → Semantic report created
  → semantic_release subscribers notified
```

#### Multi-Source Project (With Gate)

```
Release ingested
  → source_release subscribers notified immediately
  → GateCheckWorker:
      1. Normalize version via mapping
      2. Upsert version_readiness (add source to sources_met)
         (atomic — see Concurrency Control section)
      3. Evaluate structured rules (all required sources met?)
         → No: remain pending, wait for more sources
         → Yes: check NL rule?
            → NL rule exists: enqueue GateNLEvalJob
            → No NL rule: open gate
  → Gate opens (ready or timed_out):
      → Check agent rules → enqueue agent if rules match
      → Agent runs with all available source data
      → Semantic report created
      → semantic_release subscribers notified
```

#### Timeout

```
GateTimeoutWorker (periodic, every 15 min):
  → Query: version_readiness WHERE status='pending' AND timeout_at < now()
    (SELECT ... FOR UPDATE SKIP LOCKED, LIMIT 100)
  → Set status='timed_out', record opened_at
  → Trigger agent with partial flag
  → Agent report notes missing sources
  → Notification includes "⚠ partial" indicator
```

### Gate Evaluation Logic

**Enqueue strategy:** `GateCheckJobArgs` is **always enqueued unconditionally** in `IngestRelease()` alongside `NotifyJobArgs`. The current `IngestRelease()` has no project awareness (only knows `sourceID`), and adding a project lookup inside the transaction would complicate the ingestion layer. Instead, the `GateCheckWorker` loads the project via source → project join and short-circuits immediately (returns success) when no gate exists for the project. This keeps `IngestRelease()` unchanged and the gate logic fully contained in the worker.

When `GateCheckWorker` processes a job:

1. Load project via source → project join. If no gate exists or gate is disabled → return success (no-op).
2. Load `release_gates` for the project.
3. Normalize the version using `version_mapping` for this source.
4. Upsert `version_readiness` atomically (see Concurrency Control).
5. If `sources_missing` is empty:
   - If `nl_rule` exists → enqueue `GateNLEvalJob`
   - If no `nl_rule` → set status `ready`, record `opened_at`, trigger agent
6. If `sources_missing` is not empty → remain `pending`

#### Concurrency Control

Two `GateCheckWorker` instances may process releases from different sources for the same version simultaneously. The upsert and gate-open transition must be atomic to prevent duplicate agent triggers.

**Approach:** Use `INSERT ... ON CONFLICT` with a conditional status transition:

```sql
-- Step 1: Atomic upsert — add source to sources_met
INSERT INTO version_readiness (project_id, version, sources_met, sources_missing, timeout_at)
VALUES ($1, $2, jsonb_build_array($3), $4, $5)
ON CONFLICT (project_id, version) DO UPDATE
SET sources_met = CASE
        WHEN NOT version_readiness.sources_met @> jsonb_build_array($3)
        THEN version_readiness.sources_met || jsonb_build_array($3)
        ELSE version_readiness.sources_met
    END,
    sources_missing = $4,  -- recalculated by worker
    updated_at = now()
WHERE version_readiness.status = 'pending'
RETURNING id, sources_met, sources_missing, status;

-- Step 2: If sources_missing is empty, atomically open the gate
UPDATE version_readiness
SET status = 'ready', opened_at = now()
WHERE id = $1 AND status = 'pending'
  AND sources_missing = '[]'::jsonb
RETURNING id;
```

The `WHERE status = 'pending'` guard on the status transition ensures only one worker can open the gate — the second worker's UPDATE returns zero rows, preventing duplicate agent triggers.

### River Jobs

#### `GateCheckJobArgs`

**Always enqueued** in `IngestRelease()` alongside `NotifyJobArgs`. Worker short-circuits for non-gated projects.

```go
type GateCheckJobArgs struct {
    SourceID  string
    ReleaseID string
    Version   string // raw version from source
}
// Kind() = "gate_check"
```

> Note: `ProjectID` is not in the args — the worker resolves it from `source.project_id`. This keeps `IngestRelease()` unchanged (it only knows source ID).

#### `GateNLEvalJobArgs`

Enqueued by `GateCheckWorker` when structured rules pass and an NL rule exists.

```go
type GateNLEvalJobArgs struct {
    VersionReadinessID string
    ProjectID          string
    Version            string
}
// Kind() = "gate_nl_eval"
```

Worker makes a single LLM call with the NL rule text and version context. Updates `nl_rule_passed`. If passed, opens gate and triggers agent.

**NL rule failure behavior:**
- If the NL rule evaluates to `false`, the gate remains `pending` and `nl_rule_passed` is set to `false`.
- The NL rule is re-evaluated when the next source reports in for this version (the `GateCheckWorker` re-enqueues `GateNLEvalJob` if structured rules still pass and `nl_rule_passed` is `false` or `null`).
- If the gate times out while the NL rule has failed, the timeout path still triggers the agent — timeout always opens the gate regardless of NL rule state.
- No manual override for NL rules in v1. Users can disable the gate or remove the NL rule if it's stuck.

#### `GateTimeoutJobArgs`

Periodic River job running every 15 minutes.

```go
type GateTimeoutJobArgs struct{}
// Kind() = "gate_timeout"
```

**Registration:** Uses River's `NewPeriodicJob` API:

```go
river.NewPeriodicJob(
    river.PeriodicInterval(15 * time.Minute),
    func() (river.JobArgs, *river.InsertOpts) {
        return GateTimeoutJobArgs{}, nil
    },
    &river.PeriodicJobOpts{RunOnStart: true},
)
```

**Sweep mechanics:**
- Uses `SELECT ... FOR UPDATE SKIP LOCKED` on `version_readiness` rows where `status = 'pending' AND timeout_at < now()`.
- Processes up to 100 rows per sweep to bound lock duration.
- If more than 100 remain, the next periodic execution picks them up.
- Overlapping sweeps are safe due to `SKIP LOCKED` — two concurrent sweeps won't process the same row.

### Enriched Semantic Notifications

`semantic_release` notification messages are enriched to be self-contained:

**Normal (all sources ready):**
```
📦 Go v1.21.0 — High Urgency

Security patch addressing CVE-2024-XXXX in net/http.

Sources: GitHub ✓, Docker Hub ✓
Recommendation: Upgrade immediately

Changelog summary: ...
Download: docker pull golang:1.21.0
```

**Partial (timed out, missing sources):**
```
📦 Go v1.21.0 — High Urgency (⚠ partial)

Security patch addressing CVE-2024-XXXX in net/http.

Sources: GitHub ✓, Docker Hub ✗ (not yet available)
Recommendation: Upgrade when Docker image is available

Changelog summary: ...
```

Data sources: `SemanticReport` fields (Urgency, ChangelogSummary, StatusChecks, DownloadCommands) merged with `version_readiness` (sources_met, sources_missing).

No changes to the `Sender` interface — the enriched content is assembled into the `Notification.Body` field.

### API Endpoints

| Method | Path | Description |
|--------|------|-------------|
| `GET` | `/api/v1/projects/{id}/release-gate` | Get gate config |
| `PUT` | `/api/v1/projects/{id}/release-gate` | Create/update gate config |
| `DELETE` | `/api/v1/projects/{id}/release-gate` | Remove gate |
| `GET` | `/api/v1/projects/{id}/version-readiness` | List version readiness (paginated) |
| `GET` | `/api/v1/projects/{id}/version-readiness/{version}` | Get specific version status |

**PUT body example:**
```json
{
  "required_sources": ["uuid-github", "uuid-dockerhub"],
  "timeout_hours": 720,
  "version_mapping": {
    "uuid-github": {"pattern": "^v?(.+)$", "template": "$1"},
    "uuid-dockerhub": {"pattern": "^(.+)$", "template": "$1"}
  },
  "nl_rule": "Wait until the Docker image has at least 100 pulls",
  "enabled": true
}
```

### Dashboard Additions

- **Project settings page:** gate configuration form (required sources picker, timeout slider, version mapping editor, NL rule textarea)
- **Project overview:** version readiness status table showing pending/ready/timed_out versions with source checklist

### Interaction with Existing Systems

- **Agent rules:** Gate controls *when* the agent can run. Agent rules control *whether* to run. Both must agree for the agent to execute.
- **Source filters:** Applied during ingestion, before gate check. Filtered releases never enter the gate pipeline.
- **Transactional outbox:** `GateCheckJobArgs` always enqueued in the same transaction as `NotifyJobArgs` and the release insert. No conditional logic in `IngestRelease()`.

### `WaitForAllSources` Deprecation

The existing `AgentRules.WaitForAllSources` field is superseded by release gates.

**Migration strategy:**
1. Keep the field in the `AgentRules` struct but mark it as deprecated in code comments.
2. During this implementation, add a one-time migration that auto-creates a `release_gate` for any project where `agent_rules->>'wait_for_all_sources' = 'true'`. The gate's `required_sources` is set to all current sources for that project, with default timeout.
3. The `AgentWorker`'s existing `WaitForAllSources` snooze logic remains functional during the transition — it only triggers if no `release_gate` exists for the project. Once a gate exists, the gate takes precedence.
4. In a future release, remove `WaitForAllSources` from `AgentRules` and the snooze logic from `AgentWorker`.

### Observability: Gate Event History

A `gate_events` table records every state transition for auditing and debugging. The `project_id` and `version` columns are intentionally denormalized from `version_readiness` for query performance — gate event queries are almost always filtered by project and often by version, and requiring a join through `version_readiness` for every event listing query would be unnecessarily expensive for a high-volume audit table.

```sql
CREATE TABLE gate_events (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    version_readiness_id UUID NOT NULL REFERENCES version_readiness(id) ON DELETE CASCADE,
    project_id UUID NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    version VARCHAR(100) NOT NULL,
    event_type VARCHAR(30) NOT NULL
        CHECK (event_type IN (
            'source_met',       -- a required source reported this version
            'gate_opened',      -- all structured rules passed, gate opened
            'gate_timed_out',   -- timeout expired, gate force-opened
            'nl_eval_started',  -- NL rule LLM evaluation began
            'nl_eval_passed',   -- NL rule evaluated to true
            'nl_eval_failed',   -- NL rule evaluated to false
            'agent_triggered',  -- agent job enqueued after gate opened
            'agent_completed',  -- agent finished, semantic report created
            'notified'          -- semantic_release notifications sent
        )),
    source_id UUID,                       -- which source (for source_met events)
    details JSONB,                        -- event-specific context
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_gate_events_readiness ON gate_events(version_readiness_id, created_at);
CREATE INDEX idx_gate_events_project ON gate_events(project_id, created_at);
```

**`details` JSONB examples by event type:**

| Event | Details |
|-------|---------|
| `source_met` | `{"source_name": "Docker Hub", "raw_version": "1.21.0", "normalized_version": "1.21.0", "sources_met": 2, "sources_required": 3}` |
| `gate_timed_out` | `{"sources_missing": ["uuid-dockerhub"], "waited_hours": 168}` |
| `nl_eval_passed` | `{"rule": "Wait until Docker image has 100 pulls", "llm_response": "true — image has 1,247 pulls"}` |
| `nl_eval_failed` | `{"rule": "Wait until Docker image has 100 pulls", "llm_response": "false — image has 12 pulls"}` |
| `agent_triggered` | `{"agent_run_id": "uuid", "partial": false}` |
| `agent_completed` | `{"semantic_release_id": "uuid", "urgency": "High"}` |

**API endpoints for history:**

| Method | Path | Description |
|--------|------|-------------|
| `GET` | `/api/v1/projects/{id}/version-readiness/{version}/events` | Event timeline for a version |
| `GET` | `/api/v1/projects/{id}/gate-events` | All gate events for project (paginated, filterable by event_type) |

**Dashboard: Gate History View**

The version readiness detail page shows a timeline of gate events:

```
v1.21.0 — Ready (opened 2h ago)
──────────────────────────────────
  12:00  source_met      GitHub        ✓ (1/2 sources)
  12:01  notified        source_release subscribers (immediate)
  14:30  source_met      Docker Hub    ✓ (2/2 sources)
  14:30  gate_opened     All required sources met
  14:31  agent_triggered Agent run started
  14:45  agent_completed Urgency: High — security patch
  14:45  notified        semantic_release subscribers
```

This gives full visibility into why a gate is pending, what's blocking it, and the complete history of how it resolved.

### Error Handling

- **LLM unavailable for NL eval:** Job retries per River policy. Gate remains pending until eval succeeds or timeout.
- **NL rule evaluates to false:** Gate remains pending. Re-evaluated when the next source reports in. Timeout still opens the gate regardless.
- **Source deleted while gate pending:** `GateCheckWorker` recalculates required sources, removes deleted source from missing list.
- **Gate config changed while versions pending:** Existing `version_readiness` rows are not retroactively recalculated. New config applies to future releases only.
- **Duplicate release ingestion:** Gate check is idempotent — adding an already-met source to `sources_met` is a no-op (JSONB containment check).
- **Concurrent gate checks:** `WHERE status = 'pending'` guard on status transition ensures only one worker opens the gate. See Concurrency Control section.
