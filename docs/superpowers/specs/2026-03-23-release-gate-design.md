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

### Data Model

#### `release_gates` table

Per-project gate configuration. One gate per project (1:1).

```sql
CREATE TABLE release_gates (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    project_id UUID NOT NULL UNIQUE REFERENCES projects(id) ON DELETE CASCADE,
    required_sources UUID[],              -- source IDs that must have the version; null/empty = all
    timeout_hours INT NOT NULL DEFAULT 168, -- 7 days default
    version_mapping JSONB,                -- per-source version transform rules
    nl_rule TEXT,                          -- optional NL condition for LLM evaluation
    enabled BOOLEAN NOT NULL DEFAULT true,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
```

**Fields:**

- `required_sources`: array of source UUIDs that must report the version. If null/empty, all project sources are required.
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
    sources_met UUID[] NOT NULL DEFAULT '{}',
    sources_missing UUID[] NOT NULL DEFAULT '{}',
    nl_rule_passed BOOLEAN,               -- null = not evaluated yet
    timeout_at TIMESTAMPTZ NOT NULL,
    opened_at TIMESTAMPTZ,
    agent_triggered BOOLEAN NOT NULL DEFAULT false,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE(project_id, version)
);
```

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
  → Set status='timed_out', record opened_at
  → Trigger agent with partial flag
  → Agent report notes missing sources
  → Notification includes "⚠ partial" indicator
```

### Gate Evaluation Logic

When `NotifyWorker` processes a release for a gated project:

1. Load `release_gates` for the project
2. Normalize the version using `version_mapping` for this source
3. Upsert `version_readiness`:
   - Add source ID to `sources_met`
   - Recalculate `sources_missing` = `required_sources - sources_met`
   - Set `timeout_at = now() + timeout_hours` on first creation only
4. If `sources_missing` is empty:
   - If `nl_rule` exists → enqueue `GateNLEvalJob`
   - If no `nl_rule` → set status `ready`, record `opened_at`, trigger agent
5. If `sources_missing` is not empty → remain `pending`

### River Jobs

#### `GateCheckJobArgs`

Enqueued transactionally in `IngestRelease()` alongside `NotifyJobArgs` when the project has a gate.

```go
type GateCheckJobArgs struct {
    ProjectID string
    SourceID  string
    ReleaseID string
    Version   string // raw version from source
}
// Kind() = "gate_check"
```

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

#### `GateTimeoutJobArgs`

Periodic River job running every 15 minutes.

```go
type GateTimeoutJobArgs struct{}
// Kind() = "gate_timeout"
```

Sweeps expired pending gates, marks `timed_out`, triggers agents with partial flag.

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
- **Source filters:** Applied before gate check. Filtered releases never enter the gate pipeline.
- **Transactional outbox:** `GateCheckJobArgs` enqueued in the same transaction as `NotifyJobArgs` and the release insert.
- **Existing `WaitForAllSources` field in AgentRules:** Superseded by release gates. Can be deprecated.

### Observability: Gate Event History

A `gate_events` table records every state transition for auditing and debugging:

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
- **Source deleted while gate pending:** `GateCheckWorker` recalculates required sources, removes deleted source from missing list.
- **Gate config changed while versions pending:** Existing `version_readiness` rows are not retroactively recalculated. New config applies to future releases only.
- **Duplicate release ingestion:** Gate check is idempotent — adding an already-met source to `sources_met` is a no-op.
