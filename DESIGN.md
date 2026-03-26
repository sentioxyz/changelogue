# Design Document: Changelogue

## 1. System Context & Principles

**Changelogue** is designed to solve the "release noise" problem by acting as an intelligent middleware between upstream software registries (Docker Hub, GitHub, ECR Public, GitLab, PyPI, npm) and downstream operational systems (Base A Box, Ops Opsack, Slack).

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

1. **Notification Routing** -- When a new source release is detected, a `NotifyJobArgs` River job is enqueued. The notification worker resolves source-level subscriptions and sends alerts to configured channels (Slack, Discord, email, webhooks). This provides immediate, low-latency notifications without waiting for AI analysis.

2. **Agent Layer** -- For project-level intelligence, an `AgentJobArgs` River job triggers an LLM agent (via ADK-Go) that researches releases, consults context sources, and produces a `SemanticRelease` with a structured `SemanticReport`. Agent behavior is configured per-project via `agent_prompt` and `agent_rules`. Detailed design for both paths will be specified in later tasks (Phase 2: Notification, Phase 3: Agent).

### 2.3 SRE Agent Orchestration (The Validation Sandbox)

For high-urgency releases or critical internal projects (like the token compiler), Changelogue employs an SRE agent to validate the build.

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

The system relies on seventeen core tables to manage state, configuration, agent intelligence, authentication, and notification routing. All primary keys are UUIDs generated by PostgreSQL's `gen_random_uuid()`.

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
    provider VARCHAR(50) NOT NULL,              -- 'dockerhub', 'github', 'ecr-public', 'gitlab', 'pypi', 'npm'
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
    type VARCHAR(50) NOT NULL,                  -- 'slack', 'pagerduty', 'webhook', 'email'
    config JSONB NOT NULL,                      -- webhook_url, channel_id, routing_key, etc.
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW()
);

-- Subscriptions: two types (source-level and project-level)
-- Source subscriptions notify on raw releases; project subscriptions notify on semantic releases
CREATE TABLE subscriptions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    channel_id UUID NOT NULL REFERENCES notification_channels(id) ON DELETE CASCADE,
    type VARCHAR(50) NOT NULL CHECK (type IN ('source_release', 'semantic_release')),
    source_id UUID REFERENCES sources(id) ON DELETE CASCADE,
    project_id UUID REFERENCES projects(id) ON DELETE CASCADE,
    version_filter TEXT,                        -- regex: only notify for matching versions
    created_at TIMESTAMPTZ DEFAULT NOW(),
    CHECK (
        (type = 'source_release'  AND source_id  IS NOT NULL AND project_id IS NULL) OR
        (type = 'semantic_release' AND project_id IS NOT NULL AND source_id  IS NULL)
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

-- Release action items (acknowledge/resolve workflow)
CREATE TABLE release_todos (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    release_id UUID REFERENCES releases(id) ON DELETE CASCADE,
    semantic_release_id UUID REFERENCES semantic_releases(id) ON DELETE CASCADE,
    status VARCHAR(50) NOT NULL DEFAULT 'pending', -- pending, acknowledged, resolved
    acknowledged_at TIMESTAMPTZ,
    resolved_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ DEFAULT NOW()
);

-- GitHub repo dependency scans (onboarding)
CREATE TABLE onboard_scans (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    repo_url TEXT NOT NULL,
    status VARCHAR(50) NOT NULL DEFAULT 'pending', -- pending, scanning, completed, failed
    results JSONB,                              -- extracted dependencies as JSON array
    error TEXT,
    started_at TIMESTAMPTZ,
    completed_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ DEFAULT NOW()
);

-- Authenticated users (GitHub OAuth)
CREATE TABLE users (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    github_id BIGINT NOT NULL UNIQUE,
    github_login VARCHAR(100) NOT NULL,
    name VARCHAR(255),
    avatar_url TEXT,
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW()
);

-- Server-side session records
CREATE TABLE sessions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    expires_at TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ DEFAULT NOW()
);

-- Release gate configuration (per-project)
CREATE TABLE release_gates (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    project_id UUID NOT NULL UNIQUE REFERENCES projects(id) ON DELETE CASCADE,
    required_sources JSONB NOT NULL,            -- array of source IDs that must report
    timeout_hours INT NOT NULL DEFAULT 168,     -- max wait time (default 7 days)
    version_mapping JSONB,                      -- per-source regex/template for version normalization
    nl_rule TEXT,                               -- natural language readiness rule (LLM-evaluated)
    enabled BOOLEAN NOT NULL DEFAULT true,
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW()
);

-- Per-version gate state tracking
CREATE TABLE version_readiness (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    project_id UUID NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    version VARCHAR(100) NOT NULL,
    status VARCHAR(50) NOT NULL DEFAULT 'pending', -- pending, ready, timed_out
    sources_met JSONB,                          -- source IDs that have reported this version
    sources_missing JSONB,                      -- source IDs still missing
    nl_rule_passed BOOLEAN,                     -- result of NL rule evaluation
    timeout_at TIMESTAMPTZ,
    opened_at TIMESTAMPTZ,
    agent_triggered BOOLEAN DEFAULT false,
    created_at TIMESTAMPTZ DEFAULT NOW(),
    UNIQUE(project_id, version)
);

-- Gate lifecycle audit log
CREATE TABLE gate_events (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    version_readiness_id UUID NOT NULL REFERENCES version_readiness(id) ON DELETE CASCADE,
    project_id UUID NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    version VARCHAR(100) NOT NULL,
    event_type VARCHAR(50) NOT NULL,            -- source_arrived, all_sources_met, nl_eval_passed, nl_eval_failed, timeout, gate_opened
    source_id UUID REFERENCES sources(id) ON DELETE SET NULL,
    details JSONB,
    created_at TIMESTAMPTZ DEFAULT NOW()
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
* **Subscription routing:** Subscriptions now support two granularity levels via a `type` check constraint: `'source_release'` subscriptions (linked to a specific source via `source_id`) notify on raw releases, while `'semantic_release'` subscriptions (linked via `project_id`) notify on semantic releases. This replaces the old project-only subscription model.
* **Notification channels:** Separating channel registration from subscription routing allows multiple subscriptions to reference the same Slack channel or PagerDuty service. The `config` JSONB column stores provider-specific credentials (webhook URLs, routing keys).
* **Release TODOs:** Each release or semantic release can have an associated TODO that tracks operator acknowledgment and resolution. The `release_todos` table links to either `release_id` (source-level) or `semantic_release_id` (project-level), never both. Notification channels include action links for one-click acknowledge/resolve.
* **Onboarding scans:** The `onboard_scans` table tracks GitHub repo dependency scanning jobs. After scanning, extracted dependencies are stored as JSONB and can be applied to auto-create sources.
* **Users and sessions:** GitHub OAuth users are stored in `users` with their GitHub ID as a unique key. Server-side sessions in the `sessions` table reference users and have an expiry timestamp. Expired sessions are cleaned up periodically.
* **Release gates:** Each project can have at most one gate (`UNIQUE(project_id)`). The `version_readiness` table tracks per-version state, and `gate_events` provides an append-only audit log of every state transition.

## 4. Notification Routing

### 4.1 Overview

When the ingestion layer detects a new release, it enqueues a `NotifyJobArgs` River job in the same transaction that inserts the release (transactional outbox pattern). The notification routing system picks up these jobs and delivers alerts to all source-level subscribers.

### 4.2 Sender Interface

All notification channel types implement the `Sender` interface:

```go
type Sender interface {
    Send(ctx context.Context, ch *models.NotificationChannel, msg Notification) error
}
```

The `Notification` struct is the channel-agnostic payload:

```go
type Notification struct {
    Title   string `json:"title"`
    Body    string `json:"body"`
    Version string `json:"version"`
}
```

### 4.3 Channel Types

| Type | Implementation | Config Fields | Payload Format |
|------|---------------|---------------|----------------|
| `webhook` | `WebhookSender` | `{"url": "..."}` | Raw JSON `Notification` |
| `slack` | `SlackSender` | `{"webhook_url": "..."}` | Slack Block Kit (header + section) |
| `discord` | `DiscordSender` | `{"webhook_url": "..."}` | Discord webhook with embeds |
| `email` | `EmailSender` | `{"smtp_host", "smtp_port", "username", "password", "from_address", "to_addresses"}` | Multipart MIME (HTML + plain text) via SMTP |

Each sender parses provider-specific config from the channel's `Config` JSONB field, formats the notification into the provider's expected payload structure, and delivers via the appropriate transport. HTTP-based senders (webhook, Slack, Discord) use a 10-second HTTP timeout. The email sender connects via SMTP with STARTTLS (port 587) or direct TLS (port 465) and a 10-second dial timeout.

### 4.4 Delivery Flow

```
River picks up NotifyJobArgs {release_id, source_id}
  → NotifyWorker.Work()
    → store.GetRelease(release_id)
    → store.ListSourceSubscriptions(source_id)
    → for each subscription:
        → store.GetChannel(sub.channel_id)
        → senders[ch.type].Send(ch, notification)
```

### 4.5 Error Handling

* **Unknown channel type:** Logged as a warning and skipped. The job completes successfully (partial delivery).
* **Sender failure:** Individual send errors are logged but do not fail the job. Other subscriptions still receive their notifications.
* **Store errors:** If the release or subscription list cannot be fetched, the job returns an error and River retries per its configured retry policy.
* **Channel lookup failure:** If a specific channel cannot be found (e.g., deleted between subscription creation and notification), it is logged and skipped.

## 5. Agent Architecture (ADK-Go)

### 5.1 Overview

The agent layer uses Google's ADK-Go (`google.golang.org/adk v0.5.0`) to create LLM-powered agents that analyze upstream releases and produce structured semantic release reports. Each agent run is scoped to a project and orchestrated through the River job queue.

### 5.2 Agent Tools

The agent is provided with three function tools (via ADK `functiontool`), all scoped to the project being analyzed:

| Tool | Input | Output | Purpose |
|------|-------|--------|---------|
| `get_releases` | `{page, per_page}` | Paginated release list | Fetch recent releases for the project |
| `get_release_detail` | `{release_id}` | Full release with raw data | Inspect changelogs, commit lists, metadata |
| `list_context_sources` | `{page, per_page}` | Context source list | Discover runbooks, docs, dashboards |

Tools are created by `NewTools(store, projectID)` which returns `[]tool.Tool` compatible with the ADK `llmagent.Config.Tools` field. The `toolFactory` struct holds the data store and project ID, ensuring all tool invocations are properly scoped.

### 5.3 Store Interfaces

Two layered interfaces separate concerns:

```go
// AgentDataStore — read-only access for tool implementations
type AgentDataStore interface {
    ListReleasesByProject(ctx, projectID, page, perPage) ([]Release, int, error)
    GetRelease(ctx, id) (*Release, error)
    ListContextSources(ctx, projectID, page, perPage) ([]ContextSource, int, error)
}

// OrchestratorStore — full access for the orchestrator lifecycle
type OrchestratorStore interface {
    AgentDataStore
    GetProject(ctx, id) (*Project, error)
    GetAgentRun(ctx, id) (*AgentRun, error)
    UpdateAgentRunStatus(ctx, id, status) error
    CreateSemanticRelease(ctx, sr, releaseIDs) error
    UpdateAgentRunResult(ctx, id, semanticReleaseID) error
    ListProjectSubscriptions(ctx, projectID) ([]Subscription, error)
    GetChannel(ctx, id) (*NotificationChannel, error)
}
```

Both are implemented by `api.PgStore`.

### 5.4 Orchestrator Lifecycle

The `Orchestrator` manages the end-to-end agent run:

1. **Mark running** — `UpdateAgentRunStatus(id, "running")` sets `started_at`.
2. **Load project** — Fetches project config including `agent_prompt`.
3. **Build instruction** — Merges the project's custom `agent_prompt` with a default system instruction that tells the agent to produce a JSON `SemanticReport`.
4. **Create LLM model** — Initializes a Gemini model via `model/gemini.NewModel()` using the `GOOGLE_API_KEY` environment variable.
5. **Create tools** — `NewTools(store, projectID)` creates project-scoped function tools.
6. **Create ADK agent** — `llmagent.New()` wires the model, instruction, and tools into an ADK LLM agent.
7. **Run agent** — Uses `runner.Runner.Run()` with an in-memory session service. Iterates over events and captures the final text response.
8. **Parse report** — Attempts to parse the agent's output as a `SemanticReport` JSON. Falls back to storing raw text as the summary if parsing fails.
9. **Persist** — Creates a `SemanticRelease` (with `semantic_release_sources` links) in a single transaction.
10. **Mark completed** — Links the semantic release to the agent run and updates status.

### 5.5 River Worker

The `AgentWorker` implements `river.Worker[queue.AgentJobArgs]`:

```go
func (w *AgentWorker) Work(ctx, job) error {
    run := w.store.GetAgentRun(job.Args.AgentRunID)
    return w.orchestrator.RunAgent(ctx, run)
}
```

The worker is registered conditionally in `main.go` — only when `GOOGLE_API_KEY` is set. If the key is missing, agent jobs remain in the River queue unprocessed until the server is restarted with the key configured.

### 5.6 Graceful Degradation

* **No API key** — `NewOrchestrator()` returns an error, the worker is not registered, and a warning is logged. The rest of the system (ingestion, notifications) continues to function.
* **LLM failure** — The agent run is marked as "failed" with the error preserved in the `agent_runs` table.
* **Invalid agent output** — If the LLM produces non-JSON output, the raw text is stored as the report summary rather than failing the entire run.

## 6. Error Handling & Idempotency

* **Idempotent Ingestion:** The `releases` table enforces a `UNIQUE(source_id, version)` constraint. If a polling worker and a webhook both report the same release from the same source simultaneously, the database rejects the duplicate, preventing duplicate notification or agent jobs.
* **Source-Level Filtering:** Before events enter the system, the ingestion layer applies source-level exclusion rules (`exclude_version_regexp`, `exclude_prereleases`). Filtered releases are never inserted, reducing downstream load.
* **Dead-Letter Queue (DLQ):** If a River job fails `max_attempts` (e.g., the LLM API is down), River marks it as `discarded`. The failure reason is captured for debugging. A monitoring hook alerts the system admin that the job requires manual intervention.
* **Agent Observability:** Each agent run tracks its status (`pending`, `running`, `completed`, `failed`), the prompt used, and any error encountered. If an agent run fails, the exact failure context is preserved in the `agent_runs` table for debugging.
* **Agent Fallback:** If the SRE Agent sandbox deployment fails due to infrastructure timeout (unrelated to the software release itself), the agent safely aborts the validation phase, flags the release event as "Unverified," and routes it to a human reviewer rather than dropping the notification.
* **Notification Digest Fallback:** If a digest batch fails to send (e.g., Slack API outage), the batch is retried on the next interval. Individual items within the batch are not lost — they remain queued until the channel recovers.

## 7. Release Gates

### 7.1 Overview

Release gates are a per-project configuration that delays agent analysis until all required sources have reported a version (and optional NL-based readiness rules pass). This prevents premature semantic analysis when a project is assembled from multiple registries (e.g., GitHub tag + Docker Hub image) that publish at slightly different times.

### 7.2 Tables

| Table | Purpose |
|---|---|
| `release_gates` | Per-project gate config: required sources, NL rules, timeout duration |
| `version_readiness` | Per-version state: which sources have reported, gate status, partial flag |
| `gate_events` | Append-only audit log of gate state transitions |

### 7.3 Flow

```
Release ingested (source_id, version)
  → GateCheckWorker evaluates version_readiness for the project
      → record source arrival in version_readiness
      → if all required sources have reported:
          → GateNLEvalWorker evaluates NL rules (if configured)
              → if rules pass (or none configured):
                  → gate opens → AgentJobArgs enqueued
      → if timeout exceeded (GateTimeoutWorker periodic sweep):
          → gate opens with partial=true → AgentJobArgs enqueued (agent receives partial flag)
```

### 7.4 Workers

| Worker | Trigger | Responsibility |
|---|---|---|
| `GateCheckWorker` | Per-release River job | Records source arrival; opens gate when all sources met and NL rules pass |
| `GateTimeoutWorker` | Periodic sweep (15 min) | Finds gates past timeout threshold; forces open with `partial=true` |
| `GateNLEvalWorker` | Enqueued by GateCheckWorker | Evaluates natural-language readiness rules via LLM; responds with pass/fail |

### 7.5 Version Mapping

Each source entry in a `release_gates` config can define a per-source regex/template for normalizing versions across registries. For example, a GitHub source may tag `v1.21.0` while a Docker Hub source publishes `1.21.0`. The version mapping normalizes both to a canonical form (`1.21.0`) before comparing across sources.

### 7.6 Integration with NotifyWorker

When a project has an active release gate, `NotifyWorker` skips the agent rule checking step (i.e., it does not enqueue an `AgentJobArgs` directly). The gate is responsible for determining when agent analysis should be triggered. Source-level `source_release` subscriptions still fire immediately; only the agent trigger is gated.

## 8. TODO Tracking

### 8.1 Overview

Release TODOs provide an operator acknowledgment workflow for releases. When a release is ingested and notifications are sent, a TODO is created so operators can track whether they've reviewed and acted on each release.

### 8.2 Model

```go
type ReleaseTodo struct {
    ID                string     `json:"id"`
    ReleaseID         *string    `json:"release_id,omitempty"`
    SemanticReleaseID *string    `json:"semantic_release_id,omitempty"`
    Status            string     `json:"status"`          // pending, acknowledged, resolved
    AcknowledgedAt    *time.Time `json:"acknowledged_at,omitempty"`
    ResolvedAt        *time.Time `json:"resolved_at,omitempty"`
    CreatedAt         time.Time  `json:"created_at"`
}
```

### 8.3 Status Flow

```
pending → acknowledged → resolved
                ↑              |
                └──────────────┘  (reopen)
```

### 8.4 Notification Integration

Notification channels (Slack, Discord) include action buttons/links for one-click acknowledge and resolve. These use GET endpoints (`/api/v1/todos/{id}/acknowledge`, `/api/v1/todos/{id}/resolve`) that support `redirect=true` to bounce operators to the frontend TODO page after clicking.

## 9. Onboarding (Dependency Scanning)

### 9.1 Overview

The onboarding system scans GitHub repositories to auto-detect dependencies and suggest ingestion sources. This accelerates project setup by extracting dependency information from manifest files using LLM-based analysis.

### 9.2 Flow

```
POST /api/v1/onboard/scan {repo_url}
  → ScanDependenciesJobArgs enqueued in River
    → Scanner.FetchDependencyFiles() via GitHub API
      → Recognized files: go.mod, package.json, requirements.txt,
        Pipfile, pyproject.toml, Cargo.toml, Gemfile, pom.xml,
        build.gradle, Dockerfile, docker-compose.yml
    → DependencyExtractor (LLM) parses manifests
      → Extracts package names, versions, and maps to providers
    → Results stored in onboard_scans table
  → User reviews results via GET /api/v1/onboard/scans/{id}
  → User applies via POST /api/v1/onboard/scans/{id}/apply
    → Sources auto-created from extracted dependencies
```

### 9.3 Model

```go
type OnboardScan struct {
    ID          string          `json:"id"`
    RepoURL     string          `json:"repo_url"`
    Status      string          `json:"status"`       // pending, scanning, completed, failed
    Results     json.RawMessage `json:"results,omitempty"`
    Error       string          `json:"error,omitempty"`
    StartedAt   *time.Time      `json:"started_at,omitempty"`
    CompletedAt *time.Time      `json:"completed_at,omitempty"`
    CreatedAt   time.Time       `json:"created_at"`
}
```