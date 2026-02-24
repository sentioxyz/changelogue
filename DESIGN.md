# Design Document: ReleaseGuard

## 1. System Context & Principles

**ReleaseGuard** is designed to solve the "release noise" problem by acting as an intelligent middleware between upstream software registries (Docker Hub, GitHub) and downstream operational systems (Base A Box, Ops Opsack, Slack).

### Core Design Principles

* **Single Binary Deployment:** The entire system (Go backend, Next.js frontend frontend, and background workers) compiles down to a single executable.
* **Zero-Loss Eventing:** Utilizing the Transactional Outbox pattern with PostgreSQL ensures no webhook or polled release is ever lost, even during system restarts.
* **Abstracted Intelligence:** The LLM reasoning and agentic validation are isolated from the core ingestion loops, allowing the system to fall back to basic regex routing if the LLM API is unavailable.

## 2. Component Design & Interactions

### 2.1 The Ingestion Engine & Standardized Payload

The ingestion layer normalizes fragmented data formats (a GitHub webhook looks very different from a Docker Hub poll) into a single, standardized format.

This payload acts as the **Intermediate Representation (IR)** of the event, which is then passed downstream.

```go
// The Intermediate Representation (IR) of a Release Event
type ReleaseEvent struct {
    ID              string            `json:"id"`               // UUID
    Source          string            `json:"source"`           // e.g., "dockerhub", "github"
    Repository      string            `json:"repository"`       // e.g., "golang/go"
    RawVersion      string            `json:"raw_version"`      // e.g., "v1.21.0-rc.1"
    SemanticVersion SemanticData      `json:"semantic_version"` // Parsed major/minor/patch
    Changelog       string            `json:"changelog"`        // Markdown or raw text
    IsPreRelease    bool              `json:"is_pre_release"`   // Flagged by regex
    Metadata        map[string]string `json:"metadata"`         // Upstream-specific data
    Timestamp       time.Time         `json:"timestamp"`
}

```

### 2.2 The DAG Processing Pipeline

Instead of a rigid, procedural filtering function, the event processing is designed as a **Directed Acyclic Graph (DAG)**. The IR payload is passed through these nodes sequentially. This compiler-like approach makes it trivial to add new evaluation steps or branch the logic for multiple outputs without breaking existing flows.

**Which nodes run is configurable per project** via `pipeline_config` — an opaque JSONB map where each key is a node name and its value is the node's configuration blob. A key being present means the node is enabled; absent or `null` means disabled. Each node owns and validates its own config schema independently.

Two nodes are **always-on** (structural — cannot be disabled). The rest are **configurable** and receive their config blob at runtime.

#### Always-On Nodes

1. **Node: Regex Normalizer** — Parses `RawVersion` into `SemanticData` and sets `IsPreRelease`. Applies source-level exclusion rules (`exclude_version_regexp`, `exclude_prereleases`).
2. **Node: Subscription Router** — Checks the Postgres DB to see if any team is subscribed to this project's channel type. If not, execution halts (the event is dropped).

#### Configurable Nodes

Each configurable node has a **source-linked mode** (auto-resolves targets from the project's sources — no config needed) and may support **external targets** via explicit config.

3. **Node: Availability Checker** (`availability_checker`) — Verifies the release artifact is obtainable. Source-linked checks are automatic (Docker Hub → verify image digest, GitHub → verify binary download URLs). Extra artifacts beyond the project's sources can be configured explicitly.

4. **Node: Risk Assessor** (`risk_assessor`) — Scans for high-risk signals. Changelog keyword scanning is automatic (uses the release payload). External signal sources (Discord channels, Telegram groups, GitHub security advisories) are configured explicitly per project.

5. **Node: Adoption Tracker** (`adoption_tracker`) — Queries domain-specific metrics for release adoption. Always requires explicit config since adoption data comes from external APIs (ethernodes for blockchain, npm registry for packages, Docker Hub pulls for containers).

6. **Node: Changelog Summarizer** (`changelog_summarizer`) — Calls the LLM to generate a concise summary. Uses the release changelog from the payload. Config can override LLM provider or prompt strategy.

7. **Node: Urgency Scorer** (`urgency_scorer`) — Computes a composite urgency (LOW / MEDIUM / HIGH / CRITICAL) from all preceding node results. Config can adjust thresholds and weighting.

8. **Node: Validation Trigger** (`validation_trigger`) — Triggers the SRE agent if the urgency score meets a threshold. Only relevant for projects with sandbox validation configured.

#### Node Interface (Go)

```go
// Every pipeline node implements this interface.
// The runner passes the node's config blob from pipeline_config as raw JSON.
type PipelineNode interface {
    Name() string
    Execute(ctx context.Context, event *ReleaseEvent, config json.RawMessage, prior map[string]json.RawMessage) (json.RawMessage, error)
}
```

Each node unmarshals `config` into its own typed struct. If the node's needs grow complex enough to warrant dedicated storage (e.g., a `discord_monitors` table), its config can reference an ID (`{"monitor_id": 5}`) — the pipeline_config contract stays the same.

#### Example: pipeline_config for a Blockchain Project (Geth)

```json
{
  "availability_checker": {
    "extra_artifacts": [
      {"type": "npm_package", "name": "geth"}
    ]
  },
  "risk_assessor": {
    "keywords": ["hard fork", "breaking change", "CVE", "security"],
    "external_signals": [
      {"type": "discord", "guild_id": "714888", "channel_id": "announcements"},
      {"type": "github_advisories"}
    ]
  },
  "adoption_tracker": {
    "provider": "ethernodes",
    "config": {"network": "mainnet"}
  },
  "changelog_summarizer": {},
  "urgency_scorer": {}
}
```

#### Example: pipeline_config for a Simple Library (lodash)

```json
{
  "changelog_summarizer": {},
  "urgency_scorer": {}
}
```

Source-linked availability checks (GitHub binary verification) happen automatically. No risk assessor, no adoption tracking — just summary and urgency.

#### Notification Assembly

The final notification is assembled from `pipeline_jobs.node_results` using a fixed template with dynamic sections. Each enabled node maps to a notification section — disabled nodes produce no section:

```
🚀 Ready to Deploy: {project} {version} ({urgency} Update)

Status: ✅ Docker Image Verified | ✅ Binaries Available    ← availability_checker
Risk Level: 🔴 CRITICAL (Keyword "Hard Fork" detected)      ← risk_assessor
Adoption: 📊 12% of network updated                         ← adoption_tracker
Summary: "Fixes sync bug in block 14,000,000."              ← changelog_summarizer
Urgency: HIGH                                                ← urgency_scorer
```

For the lodash project (only `changelog_summarizer` and `urgency_scorer` enabled):

```
🚀 New Release: lodash v4.18.0

Summary: "Adds array chunking utility, fixes deep clone edge case."
Urgency: LOW
```

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
4. **Resolve:** If healthy, the agent approves the pipeline job. If degraded, it formats a critical error summary for the Notification Matrix.

## 3. Database Schema (PostgreSQL)

The system relies on seven core tables to manage state, configuration, and message passing.

```sql
-- Tracked software projects (the central entity)
CREATE TABLE projects (
    id SERIAL PRIMARY KEY,
    name VARCHAR(100) NOT NULL UNIQUE,          -- display name: 'Go Runtime', 'PostgreSQL'
    description TEXT,
    url VARCHAR(500),                           -- upstream project URL
    pipeline_config JSONB NOT NULL DEFAULT '{   -- per-node config: present key = enabled, absent = disabled
        "changelog_summarizer": {},
        "urgency_scorer": {}
    }'::jsonb,                                  -- each node owns its config schema; see DESIGN.md §2.2
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW()
);

-- Configured ingestion sources (what to poll and how)
-- A project can have multiple sources (e.g., GitHub + Docker Hub)
CREATE TABLE sources (
    id SERIAL PRIMARY KEY,
    project_id INT NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    source_type VARCHAR(50) NOT NULL,           -- 'dockerhub', 'github'
    repository VARCHAR(255) NOT NULL,
    poll_interval_seconds INT DEFAULT 300,
    enabled BOOLEAN DEFAULT true,
    exclude_version_regexp TEXT,                 -- regex to skip matching versions at ingestion
    exclude_prereleases BOOLEAN DEFAULT false,   -- drop pre-releases before pipeline
    last_polled_at TIMESTAMPTZ,
    last_error TEXT,
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW(),
    UNIQUE(source_type, repository)             -- no duplicate source+repo pairs
);

-- Stores the normalized Intermediate Representation
-- References the source it came from; project is reachable via JOIN
CREATE TABLE releases (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    source_id INT NOT NULL REFERENCES sources(id),
    version VARCHAR(100) NOT NULL,
    payload JSONB NOT NULL,                     -- full ReleaseEvent IR as JSON
    created_at TIMESTAMPTZ DEFAULT NOW(),
    UNIQUE(source_id, version)                  -- idempotent: same version from same source
);

-- River-compatible job queue for the DAG pipeline
CREATE TABLE pipeline_jobs (
    id BIGSERIAL PRIMARY KEY,
    state VARCHAR(50) DEFAULT 'available',      -- available, running, completed, retry, discarded
    release_id UUID REFERENCES releases(id),
    current_node VARCHAR(50),                   -- 'regex_normalizer', 'subscription_router', etc.
    node_results JSONB DEFAULT '{}',            -- per-node output keyed by node name
    attempt INT DEFAULT 0,
    max_attempts INT DEFAULT 3,
    error_message TEXT,
    locked_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ DEFAULT NOW(),
    completed_at TIMESTAMPTZ
);

-- Registered notification channels (output targets)
CREATE TABLE notification_channels (
    id SERIAL PRIMARY KEY,
    type VARCHAR(50) NOT NULL,                  -- 'slack', 'pagerduty', 'webhook'
    name VARCHAR(100) NOT NULL,
    config JSONB NOT NULL,                      -- webhook_url, channel_id, routing_key, etc.
    enabled BOOLEAN DEFAULT true,
    created_at TIMESTAMPTZ DEFAULT NOW()
);

-- Routing rules: subscribe a project to a notification channel
-- Subscriptions attach to projects, not individual sources
CREATE TABLE subscriptions (
    id SERIAL PRIMARY KEY,
    project_id INT NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    channel_type VARCHAR(50) NOT NULL,          -- 'stable', 'pre-release', 'security'
    channel_id INT REFERENCES notification_channels(id),
    version_pattern TEXT,                       -- regex: only notify for matching versions
    frequency VARCHAR(20) DEFAULT 'instant',    -- 'instant', 'hourly', 'daily', 'weekly'
    enabled BOOLEAN DEFAULT true,
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW()
);

-- API authentication keys
CREATE TABLE api_keys (
    id SERIAL PRIMARY KEY,
    name VARCHAR(100) NOT NULL,
    key_hash VARCHAR(64) NOT NULL UNIQUE,       -- SHA-256 hash, raw key shown once
    key_prefix VARCHAR(12) NOT NULL,            -- 'rg_live_' or 'rg_test_' prefix for identification
    created_at TIMESTAMPTZ DEFAULT NOW(),
    last_used_at TIMESTAMPTZ
);

```

### 3.1 Schema Design Notes

* **Projects as the central entity:** A project represents a tracked piece of software (e.g., "Go Runtime"). It can have multiple sources — a GitHub source for webhook-based repos you own and a Docker Hub source for polling the container image. Subscriptions attach to projects, so a single notification rule covers all sources for that project. The `pipeline_config` JSONB column is an opaque map of node name → node config. Each node owns its config schema — presence means enabled, absence means disabled. Source-linked behavior (e.g., Docker image verification) is automatic; external targets (Discord monitors, adoption APIs) are configured explicitly in the node's config blob.
* **Multi-source ingestion:** The `sources` table uses `UNIQUE(source_type, repository)` instead of `UNIQUE(repository)` — the same repository name on different providers won't collide. Each source belongs to exactly one project via `project_id`.
* **Release provenance:** Releases reference `source_id` (not raw strings), so you always know which source detected a release. The unique constraint `UNIQUE(source_id, version)` allows the same version to exist from different sources (e.g., Go 1.21.0 from GitHub and Docker Hub are separate release records).
* **Pipeline node tracking:** The `current_node` and `node_results` columns on `pipeline_jobs` provide per-node observability. The dashboard can show exactly where a release is in the pipeline and what each node produced (e.g., the AI urgency score, regex normalizer output).
* **Notification channels:** Separating channel registration from subscription routing allows multiple subscriptions to reference the same Slack channel or PagerDuty service. The `config` JSONB column stores provider-specific credentials (webhook URLs, routing keys).
* **Notification frequency:** The `frequency` column on subscriptions supports digest-style notifications. A background worker aggregates `hourly`/`daily`/`weekly` subscriptions and sends batched summaries instead of individual alerts.

## 4. Error Handling & Idempotency

* **Idempotent Ingestion:** The `releases` table enforces a `UNIQUE(source_id, version)` constraint. If a polling worker and a webhook both report the same release from the same source simultaneously, the database rejects the duplicate, preventing duplicate pipeline jobs.
* **Source-Level Filtering:** Before events enter the pipeline, the ingestion layer applies source-level exclusion rules (`exclude_version_regexp`, `exclude_prereleases`). Filtered releases are never inserted, reducing pipeline load.
* **Dead-Letter Queue (DLQ):** If a DAG pipeline job fails `max_attempts` (e.g., the LLM API is down), the `pipeline_jobs` state is updated to `discarded` and `error_message` captures the failure reason. A separate Postgres trigger alerts the system admin that an event requires manual intervention.
* **Pipeline Observability:** Each pipeline job tracks its `current_node` and accumulates `node_results` as it progresses through the DAG. If a job fails mid-pipeline, the exact failure point and partial results are preserved for debugging.
* **Agent Fallback:** If the SRE Agent sandbox deployment fails due to infrastructure timeout (unrelated to the software release itself), the agent safely aborts the validation phase, flags the release event as "Unverified," and routes it to a human reviewer rather than dropping the notification.
* **Notification Digest Fallback:** If a digest batch fails to send (e.g., Slack API outage), the batch is retried on the next interval. Individual items within the batch are not lost — they remain queued until the channel recovers.