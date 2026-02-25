# Design Document: ReleaseGuard

## 1. System Context & Principles

**ReleaseGuard** is designed to solve the "release noise" problem by acting as an intelligent middleware between upstream software registries (Docker Hub, GitHub) and downstream operational systems (Base A Box, Ops Opsack, Slack).

### Core Design Principles

* **Single Binary Deployment:** The entire system (Go backend, Next.js frontend frontend, and background workers) compiles down to a single executable.
* **Zero-Loss Eventing:** Utilizing the Transactional Outbox pattern with PostgreSQL ensures no webhook or polled release is ever lost, even during system restarts.
* **Abstracted Intelligence:** The LLM reasoning and agentic validation are isolated from the core ingestion loops, allowing the system to fall back to basic regex routing if the LLM API is unavailable.

## 2. Component Design & Interactions

### 2.1 Core Models

The system uses a set of Go structs that map directly to the database schema. These models replace the previous `ReleaseEvent` IR approach -- instead of normalizing into an intermediate representation, raw provider data is stored as JSONB and analyzed by AI agents.

#### Release

The primary release representation. Stores the raw provider payload as JSONB in `RawData`, preserving full fidelity from the upstream source. Replaces the old `ReleaseEvent` IR.

```go
type Release struct {
    ID         string          `json:"id"`                    // UUID
    SourceID   string          `json:"source_id"`             // FK to sources table
    Version    string          `json:"version"`               // e.g., "v1.21.0-rc.1"
    RawData    json.RawMessage `json:"raw_data,omitempty"`    // Full upstream payload as JSONB
    ReleasedAt *time.Time      `json:"released_at,omitempty"` // Upstream release timestamp
    CreatedAt  time.Time       `json:"created_at"`
}
```

#### ContextSource

Read-only references that agents consult during analysis (runbooks, deployment docs, monitoring dashboards). These are not polled -- they provide background context for generating richer semantic release reports.

```go
type ContextSource struct {
    ID        string          `json:"id"`         // UUID
    ProjectID string          `json:"project_id"` // FK to projects table
    Type      string          `json:"type"`       // 'url', 'github_repo', 'confluence', etc.
    Name      string          `json:"name"`
    Config    json.RawMessage `json:"config"`     // Type-specific config (URL, credentials ref, etc.)
    CreatedAt time.Time       `json:"created_at"`
    UpdatedAt time.Time       `json:"updated_at"`
}
```

#### SemanticRelease & SemanticReport

AI-generated, project-level analysis. An agent correlates one or more source-level releases into a single semantic release with a structured report. Replaces the old pipeline node-by-node result accumulation.

```go
type SemanticReport struct {
    Summary        string `json:"summary"`        // Human-readable summary of the release
    Availability   string `json:"availability"`   // Artifact availability status
    Adoption       string `json:"adoption"`       // Network/ecosystem adoption metrics
    Urgency        string `json:"urgency"`        // LOW / MEDIUM / HIGH / CRITICAL
    Recommendation string `json:"recommendation"` // Agent's recommended action
}

type SemanticRelease struct {
    ID          string          `json:"id"`                      // UUID
    ProjectID   string          `json:"project_id"`              // FK to projects table
    Version     string          `json:"version"`
    Report      json.RawMessage `json:"report,omitempty"`        // SemanticReport as JSONB
    Status      string          `json:"status"`                  // pending, processing, completed, failed
    Error       string          `json:"error,omitempty"`
    CreatedAt   time.Time       `json:"created_at"`
    CompletedAt *time.Time      `json:"completed_at,omitempty"`
}
```

#### AgentRun

Provides observability into agent executions. Each run is scoped to a project and optionally linked to a semantic release. Captures the exact prompt for debugging and auditability.

```go
type AgentRun struct {
    ID                string     `json:"id"`                              // UUID
    ProjectID         string     `json:"project_id"`                      // FK to projects table
    SemanticReleaseID *string    `json:"semantic_release_id,omitempty"`   // FK to semantic_releases
    Trigger           string     `json:"trigger"`                         // 'release', 'manual', etc.
    Status            string     `json:"status"`                          // pending, running, completed, failed
    PromptUsed        string     `json:"prompt_used,omitempty"`           // Exact prompt sent to LLM
    Error             string     `json:"error,omitempty"`
    StartedAt         *time.Time `json:"started_at,omitempty"`
    CompletedAt       *time.Time `json:"completed_at,omitempty"`
    CreatedAt         time.Time  `json:"created_at"`
}
```

#### AgentRules

Structured rules governing when and how an agent should analyze releases for a project. Stored as JSONB in the `projects.agent_rules` column.

```go
type AgentRules struct {
    OnMajorRelease  bool   `json:"on_major_release,omitempty"`  // Trigger on major version bumps
    OnMinorRelease  bool   `json:"on_minor_release,omitempty"`  // Trigger on minor version bumps
    OnSecurityPatch bool   `json:"on_security_patch,omitempty"` // Trigger on security patches
    VersionPattern  string `json:"version_pattern,omitempty"`   // Regex for custom version matching
}
```

### 2.2 Processing Architecture (Agent + Notification Routing)

The configurable processing pipeline has been replaced by an agent-driven architecture with two distinct paths:

1. **Notification Routing** -- When a new source release is detected, a `NotifyJobArgs` River job is enqueued. The notification worker resolves source-level subscriptions and sends alerts to configured channels (Slack, PagerDuty, webhooks). This provides immediate, low-latency notifications without waiting for AI analysis.

2. **Agent Layer** -- For project-level intelligence, an `AgentJobArgs` River job triggers an LLM agent (via ADK-Go) that researches releases, consults context sources, and produces a `SemanticRelease` with a structured `SemanticReport`. Agent behavior is configured per-project via `agent_prompt` and `agent_rules`. Detailed design for both paths will be specified in later tasks (Phase 2: Notification, Phase 3: Agent).

### 2.3 SRE Agent Orchestration (The Validation Sandbox)

For high-urgency releases or critical internal projects (like the token compiler), ReleaseGuard employs an SRE agent to validate the build.

To maintain predictable execution, the agent is modeled as a state machine (suitable for implementation via frameworks like LangGraph or the Claude Agent SDK).

**Agent Toolset:**

* `GetBaseABoxConfig(repo)`: Fetches the current master config.
* `DraftConfigUpgrade(repo, newVersion)`: Proposes a new configuration.
* `DeploySandbox(config)`: Triggers a test deployment.
* `QueryAgentStatus(envId)`: Checks the health of the newly deployed agent.

**Agent Workflow Loop:**

1. **Observe:** Agent receives the `ReleaseEvent` IR and the "Production Record" summary.
2. **Act:** Agent uses `DraftConfigUpgrade` and `DeploySandbox`.
3. **Evaluate:** Agent uses `QueryAgentStatus` to ensure no regressions occurred (e.g., login failures).
4. **Resolve:** If healthy, the agent approves the release. If degraded, it formats a critical error summary for the Notification Matrix.

## 3. Database Schema (PostgreSQL)

The system relies on ten core tables to manage state, configuration, agent intelligence, and notification routing. All primary keys are UUIDs generated by PostgreSQL's `gen_random_uuid()`.

```sql
-- Tracked software projects (the central entity)
CREATE TABLE projects (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name VARCHAR(100) NOT NULL UNIQUE,          -- display name: 'Go Runtime', 'PostgreSQL'
    description TEXT,
    agent_prompt TEXT,                           -- custom prompt for agent analysis of this project
    agent_rules JSONB,                          -- structured rules governing agent behavior
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW()
);

-- Configured ingestion sources (polling-based: GitHub, Docker Hub)
-- A project can have multiple sources (e.g., GitHub + Docker Hub)
CREATE TABLE sources (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    project_id UUID NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    provider VARCHAR(50) NOT NULL,              -- 'dockerhub', 'github'
    repository VARCHAR(255) NOT NULL,
    poll_interval_seconds INT DEFAULT 900,
    enabled BOOLEAN DEFAULT true,
    config JSONB,                               -- provider-specific config (exclude patterns, etc.)
    last_polled_at TIMESTAMPTZ,
    last_error TEXT,
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW(),
    UNIQUE(provider, repository)                -- no duplicate provider+repo pairs
);

-- Context sources (read-only references for agent research)
-- Examples: runbooks, deployment docs, monitoring dashboards
CREATE TABLE context_sources (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    project_id UUID NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    type VARCHAR(50) NOT NULL,                  -- 'url', 'github_repo', 'confluence', etc.
    name VARCHAR(100) NOT NULL,
    config JSONB NOT NULL,                      -- type-specific config (URL, credentials ref, etc.)
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW()
);

-- Source-level releases (detected from polling sources)
-- References the source it came from; project is reachable via JOIN
CREATE TABLE releases (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    source_id UUID NOT NULL REFERENCES sources(id) ON DELETE CASCADE,
    version VARCHAR(100) NOT NULL,
    raw_data JSONB,                             -- upstream payload as-is
    released_at TIMESTAMPTZ,                    -- when the upstream released it
    created_at TIMESTAMPTZ DEFAULT NOW(),
    UNIQUE(source_id, version)                  -- idempotent: same version from same source
);

-- Project-level semantic releases (AI-generated, correlating source releases)
-- An agent analyzes source releases and produces a semantic release with a report
CREATE TABLE semantic_releases (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    project_id UUID NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    version VARCHAR(100) NOT NULL,
    report JSONB,                               -- AI-generated analysis report
    status VARCHAR(50) NOT NULL DEFAULT 'pending', -- pending, processing, completed, failed
    error TEXT,
    created_at TIMESTAMPTZ DEFAULT NOW(),
    completed_at TIMESTAMPTZ,
    UNIQUE(project_id, version)
);

-- Join table: which source releases compose a semantic release
CREATE TABLE semantic_release_sources (
    semantic_release_id UUID NOT NULL REFERENCES semantic_releases(id) ON DELETE CASCADE,
    release_id UUID NOT NULL REFERENCES releases(id) ON DELETE CASCADE,
    PRIMARY KEY (semantic_release_id, release_id)
);

-- Notification channels (standalone, reusable across subscriptions)
CREATE TABLE notification_channels (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name VARCHAR(100) NOT NULL,
    type VARCHAR(50) NOT NULL,                  -- 'slack', 'pagerduty', 'webhook'
    config JSONB NOT NULL,                      -- webhook_url, channel_id, routing_key, etc.
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW()
);

-- Subscriptions: two types (source-level and project-level)
-- Source subscriptions notify on raw releases; project subscriptions notify on semantic releases
CREATE TABLE subscriptions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    channel_id UUID NOT NULL REFERENCES notification_channels(id) ON DELETE CASCADE,
    type VARCHAR(50) NOT NULL CHECK (type IN ('source', 'project')),
    source_id UUID REFERENCES sources(id) ON DELETE CASCADE,
    project_id UUID REFERENCES projects(id) ON DELETE CASCADE,
    version_filter TEXT,                        -- regex: only notify for matching versions
    created_at TIMESTAMPTZ DEFAULT NOW(),
    CHECK (
        (type = 'source'  AND source_id  IS NOT NULL AND project_id IS NULL) OR
        (type = 'project' AND project_id IS NOT NULL AND source_id  IS NULL)
    )
);

-- Agent runs (scoped to project, tracks each agent execution)
CREATE TABLE agent_runs (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    project_id UUID NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    semantic_release_id UUID REFERENCES semantic_releases(id) ON DELETE SET NULL,
    trigger VARCHAR(100) NOT NULL,              -- what initiated the run: 'release', 'manual', etc.
    status VARCHAR(50) NOT NULL DEFAULT 'pending', -- pending, running, completed, failed
    prompt_used TEXT,                            -- the actual prompt sent to the LLM
    error TEXT,
    started_at TIMESTAMPTZ,
    completed_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ DEFAULT NOW()
);

-- API authentication keys
CREATE TABLE api_keys (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name VARCHAR(100) NOT NULL,
    key_hash VARCHAR(64) NOT NULL UNIQUE,       -- SHA-256 hash, raw key shown once
    key_prefix VARCHAR(12) NOT NULL,            -- 'rg_live_' or 'rg_test_' prefix for identification
    created_at TIMESTAMPTZ DEFAULT NOW(),
    last_used_at TIMESTAMPTZ
);
```

### SSE Triggers

Two PostgreSQL triggers publish real-time events via `pg_notify` on the `release_events` channel:

1. **`release_created_trigger`** — Fires on `INSERT` into `releases`. Publishes `{"type": "release", "id": "<uuid>"}`.
2. **`semantic_release_trigger`** — Fires on `INSERT OR UPDATE` on `semantic_releases` when `status` transitions to `'completed'`. Publishes `{"type": "semantic_release", "id": "<uuid>"}`.

### 3.1 Schema Design Notes

* **Projects as the central entity:** A project represents a tracked piece of software (e.g., "Go Runtime"). It can have multiple sources — a GitHub source for repos you own and a Docker Hub source for polling the container image. The `agent_prompt` and `agent_rules` fields configure how the AI agent analyzes releases for this project, replacing the old `pipeline_config` approach with flexible, agent-driven intelligence.
* **Multi-source ingestion:** The `sources` table uses `UNIQUE(provider, repository)` instead of `UNIQUE(repository)` — the same repository name on different providers won't collide. Each source belongs to exactly one project via `project_id`. Provider-specific settings (exclude patterns, pre-release filtering) live in the `config` JSONB column.
* **Context sources:** The `context_sources` table stores read-only references (URLs, repo links, documentation) that the agent can consult during analysis. These are not polled — they provide background context for generating richer semantic release reports.
* **Release provenance:** Releases reference `source_id` (not raw strings), so you always know which source detected a release. The unique constraint `UNIQUE(source_id, version)` allows the same version to exist from different sources (e.g., Go 1.21.0 from GitHub and Docker Hub are separate release records).
* **Semantic releases:** The `semantic_releases` table captures AI-generated, project-level analysis. An agent correlates one or more source-level releases (via the `semantic_release_sources` join table) into a single semantic release with a structured report. This replaces the old `pipeline_jobs` table — instead of tracking node-by-node pipeline progress, the system tracks agent-driven analysis runs.
* **Agent runs:** The `agent_runs` table provides observability into agent executions. Each run is scoped to a project and optionally linked to a semantic release. The `trigger` field records what initiated the run, and `prompt_used` captures the exact prompt for debugging and auditability.
* **Subscription routing:** Subscriptions now support two granularity levels via a `type` check constraint: `'source'` subscriptions (linked to a specific source via `source_id`) notify on raw releases, while `'project'` subscriptions (linked via `project_id`) notify on semantic releases. This replaces the old project-only subscription model.
* **Notification channels:** Separating channel registration from subscription routing allows multiple subscriptions to reference the same Slack channel or PagerDuty service. The `config` JSONB column stores provider-specific credentials (webhook URLs, routing keys).

## 4. Error Handling & Idempotency

* **Idempotent Ingestion:** The `releases` table enforces a `UNIQUE(source_id, version)` constraint. If a polling worker and a webhook both report the same release from the same source simultaneously, the database rejects the duplicate, preventing duplicate notification or agent jobs.
* **Source-Level Filtering:** Before events enter the system, the ingestion layer applies source-level exclusion rules (`exclude_version_regexp`, `exclude_prereleases`). Filtered releases are never inserted, reducing downstream load.
* **Dead-Letter Queue (DLQ):** If a River job fails `max_attempts` (e.g., the LLM API is down), River marks it as `discarded`. The failure reason is captured for debugging. A monitoring hook alerts the system admin that the job requires manual intervention.
* **Agent Observability:** Each agent run tracks its status (`pending`, `running`, `completed`, `failed`), the prompt used, and any error encountered. If an agent run fails, the exact failure context is preserved in the `agent_runs` table for debugging.
* **Agent Fallback:** If the SRE Agent sandbox deployment fails due to infrastructure timeout (unrelated to the software release itself), the agent safely aborts the validation phase, flags the release event as "Unverified," and routes it to a human reviewer rather than dropping the notification.
* **Notification Digest Fallback:** If a digest batch fails to send (e.g., Slack API outage), the batch is retried on the next interval. Individual items within the batch are not lost — they remain queued until the channel recovers.