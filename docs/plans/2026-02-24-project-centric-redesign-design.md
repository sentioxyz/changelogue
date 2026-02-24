# Project-Centric Redesign — Design Document

**Date:** 2026-02-24
**Status:** Approved

## Summary

Redesign ReleaseBeacon from a pipeline-centric model to a project-centric, two-layer architecture. A project represents one upstream dependency (e.g., "PostgreSQL") with multiple sources. Deterministic processing runs instantly on every release. Optional LLM agent enrichment provides deeper analysis when enabled per project.

## Core Principles

1. **Project = one dependency** — sources are children of projects, not standalone entities
2. **Two-layer processing** — deterministic layer always works; agent enrichment is opt-in
3. **Cross-source intelligence** — detect identical releases across sources and correlate them
4. **Idle-based batching** — agent accumulates signals until configurable quiet period, then reports
5. **Stored artifacts** — agent reports are persisted, not generated on-the-fly

---

## Data Model

### Projects (revised)

```
Project {
  id              UUID PRIMARY KEY
  name            TEXT NOT NULL
  description     TEXT
  poll_interval   INTERVAL DEFAULT '15m'    -- applies to all sources
  idle_window     INTERVAL DEFAULT '6h'     -- quiet period before agent reports
  agent_enabled   BOOLEAN DEFAULT false     -- opt-in agent enrichment
  created_at      TIMESTAMPTZ
  updated_at      TIMESTAMPTZ
}
```

### Sources (scoped to project)

```
Source {
  id              UUID PRIMARY KEY
  project_id      UUID REFERENCES projects(id) ON DELETE CASCADE
  type            TEXT NOT NULL              -- "dockerhub", "github", etc.
  config          JSONB NOT NULL            -- {repo, owner, image, ...}
  created_at      TIMESTAMPTZ
  updated_at      TIMESTAMPTZ
}
```

### Source Releases (raw cache layer)

Per-source raw release data. One row per source observation.

```
SourceRelease {
  id              UUID PRIMARY KEY
  source_id       UUID REFERENCES sources(id) ON DELETE CASCADE
  project_id      UUID REFERENCES projects(id)
  release_id      UUID REFERENCES releases(id)    -- nullable until correlated
  raw_version     TEXT NOT NULL              -- original tag e.g., "REL_16_3"
  release_notes   TEXT                       -- raw from upstream
  metadata        JSONB                      -- commit SHA, image digest, etc.
  detected_at     TIMESTAMPTZ NOT NULL
  UNIQUE(source_id, raw_version)
}
```

### Releases (correlated project-level view)

One row per unique release version within a project. Links to one or more SourceReleases.

```
Release {
  id              UUID PRIMARY KEY
  project_id      UUID REFERENCES projects(id) ON DELETE CASCADE
  version         TEXT NOT NULL              -- normalized semver
  status          TEXT DEFAULT 'new'         -- "new" | "investigating" | "complete"
  urgency_score   FLOAT
  version_info    JSONB                      -- {major, minor, patch, pre_release, ...}
  agent_report    JSONB                      -- null if agent not enabled or not yet complete
  created_at      TIMESTAMPTZ
  updated_at      TIMESTAMPTZ
  UNIQUE(project_id, version)
}
```

### Subscriptions (kept)

```
Subscription {
  id              UUID PRIMARY KEY
  project_id      UUID REFERENCES projects(id) ON DELETE CASCADE
  channel_id      UUID REFERENCES channels(id) ON DELETE CASCADE
  filters         JSONB                      -- {min_urgency, version_patterns, ...}
  enabled         BOOLEAN DEFAULT true
  created_at      TIMESTAMPTZ
  updated_at      TIMESTAMPTZ
}
```

### Channels (unchanged)

Notification endpoints (Slack webhook, PagerDuty, etc.). Shared across projects.

### Agent Sessions (new, replaces pipeline_jobs)

```
AgentSession {
  id              UUID PRIMARY KEY
  release_id      UUID REFERENCES releases(id) ON DELETE CASCADE
  project_id      UUID REFERENCES projects(id)
  status          TEXT DEFAULT 'active'      -- "active" | "idle_waiting" | "complete" | "failed"
  tool_calls      JSONB                      -- ordered log of tool invocations and results
  findings        JSONB                      -- accumulated context from tools
  idle_deadline   TIMESTAMPTZ               -- when to produce final report
  started_at      TIMESTAMPTZ
  completed_at    TIMESTAMPTZ
}
```

### Removed Tables

- `pipeline_jobs` — replaced by `agent_sessions`

### Server Configuration

```
AGENT_MODEL=gpt-4o-mini    # LLM model for agent enrichment (env var, global)
```

---

## Processing Flow

### Layer 1: Deterministic (always runs)

```
Source poll fires (per project.poll_interval)
  -> fetch new tags/releases from upstream registry
  -> store as SourceRelease (raw cache)
  -> normalize version via regex normalizer (existing logic)
  -> cross-source linking:
      1. Match by normalized semver within same project
      2. Consider temporal proximity (within project.idle_window)
      3. Match found -> link SourceRelease to existing Release
      4. No match -> create new Release
  -> compute urgency score (existing scorer logic)
  -> Release.status = "new"
  -> visible in UI immediately
  -> if NOT agent_enabled: trigger notifications via subscriptions
```

### Layer 2: Agent Enrichment (optional)

```
New Release created or SourceRelease linked to existing Release
  -> if NOT project.agent_enabled: skip
  -> check for active AgentSession for this Release
  -> if none: spawn AgentSession, set idle_deadline = now + project.idle_window
  -> if exists: feed new signal to session, reset idle_deadline

Agent tools (wrapping existing logic):
  - check_source_releases(release_id)   -> all linked SourceReleases with notes
  - normalize_version(raw_version)      -> existing regex normalizer
  - score_urgency(version_info)         -> existing urgency scorer
  - check_availability(source, version) -> poll for binary/image existence
  - check_adoption(source, version)     -> download stats, star counts
  - generate_report(findings)           -> produce structured JSON + markdown

Idle timer fires:
  -> agent produces final report
  -> stored in Release.agent_report
  -> Release.status = "complete"
  -> trigger notifications via subscriptions
```

### Cross-Source Release Linking

Detection strategy (in order):

1. **Normalized semver match** — if two SourceReleases in the same project normalize to the same semver, they're the same release
2. **Temporal proximity** — releases detected within the project's idle_window are candidates for matching
3. **Agent resolution** — if agent_enabled and matching is ambiguous, the agent decides using context (changelogs, timing, metadata)

---

## UX Design

### Sidebar Navigation (simplified)

```
Dashboard           -- overview across all projects
Projects            -- list of tracked dependencies
Channels            -- notification channel management
Settings            -- global configuration
```

Sources, Releases, and Subscriptions are no longer top-level navigation items. They live within project views.

### Dashboard

- **Stats cards:** total projects, active agents, releases this week, pending notifications
- **Activity feed (SSE):** real-time stream of new releases and agent reports
- Click any item -> navigate to project release view

### Project List Page

Table: name, source count, latest release, urgency, agent enabled/disabled.

### Project Detail Page (core view)

The main workhorse. Three sections:

**Header:** Project name, description, edit, agent toggle, poll interval, idle window.

**Sources panel:** Collapsible list of configured sources with type badges. Add/remove inline.

**Releases section:**
- **Dropdown filter:** "All sources" | per-source filters
- **Release table/cards:**
  - Normalized version
  - Source badges (which sources detected it)
  - Urgency score
  - Status badge: New / Investigating / Complete
  - Detected timestamp
- **Click release -> popup/drawer:**
  - Per-source release notes (tabs or accordion by source)
  - Deterministic report: urgency, version breakdown
  - Agent report section (if enabled):
    - Summary (markdown)
    - Availability matrix (docker, binary, etc.)
    - Adoption metrics
    - Risk assessment
    - Recommendations
  - If agent still investigating: progress indicator

**Notifications tab:** Subscriptions for this project — which channels, filter rules.

### Project Create/Edit

- Name, description, poll interval, idle window, agent toggle
- Sources: inline list with add/remove, type selector, config fields per type

### Channels Page (standalone)

Unchanged — manage Slack webhooks, PagerDuty keys, etc. Shared across projects.

---

## Impact on Existing Code

### Backend — Keep & Reuse

| Package | What to keep | Changes needed |
|---------|-------------|----------------|
| `internal/ingestion/` | Polling engine, DockerHub source, GitHub webhook | Add `project_id` scoping to sources |
| `internal/pipeline/regex_normalizer.go` | Version normalization logic | Extract as utility/tool callable by agent |
| `internal/pipeline/urgency_scorer.go` | Urgency scoring logic | Extract as utility/tool callable by agent |
| `internal/api/` | Handler structure, middleware, auth, SSE | Restructure routes, update handlers |
| `internal/db/` | Migrations, connection pooling | New migration for schema changes |
| `internal/models/` | Model patterns | Update to new schema |

### Backend — Remove

| What | Why |
|------|-----|
| `internal/pipeline/runner.go` | Fixed pipeline runner replaced by agent |
| `internal/pipeline/worker.go` | River pipeline worker no longer needed |
| `internal/pipeline/subscription_router.go` | Routing moves to notification trigger |
| Pipeline job DB table | Replaced by `agent_sessions` |

### Backend — Add New

| Package | Purpose |
|---------|---------|
| `internal/correlation/` | Cross-source release linking (semver + timing) |
| `internal/agent/` | Agent session management, tool definitions, idle timer, LLM client |
| DB tables | `source_releases`, `agent_sessions` |
| Schema changes | `sources.project_id` FK, `releases.agent_report` column |

### Frontend — Keep & Reuse

- App shell, sidebar layout (simplify nav items)
- shadcn/ui components, SWR, MSW mock layer
- Channel CRUD pages (mostly unchanged)
- Dashboard layout (update stats cards and activity feed)

### Frontend — Restructure

- Project detail page becomes main workhorse (absorbs sources, releases, subscriptions)
- Source management moves inline into project create/edit
- Release list becomes project sub-view with source dropdown filter
- Release detail becomes popup/drawer (not standalone page)
- Remove standalone Sources, Subscriptions, and Releases pages

### Frontend — Add New

- Release popup/drawer with tabbed per-source notes + agent report
- Agent status indicators (investigating / complete)
- Source dropdown filter on project releases
- Subscription management tab within project detail
