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

1. **Node: Regex Normalizer:** Parses `RawVersion` into `SemanticData` and sets `IsPreRelease`.
2. **Node: Subscription Router:** Checks the Postgres DB to see if any team is subscribed to this repository's pre-release channel. If not, execution halts (the event is dropped).
3. **Node: AI Urgency Scorer:** Calls the LLM to analyze the `Changelog`.
4. **Node: Validation Trigger:** Evaluates the AI's urgency score and triggers the SRE agent if a threshold is met.

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

The system relies on three core tables to manage state and message passing.

```sql
-- Stores the normalized Intermediate Representation
CREATE TABLE releases (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    repository VARCHAR(255) NOT NULL,
    version VARCHAR(100) NOT NULL,
    payload JSONB NOT NULL,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

-- River-compatible job queue for the DAG pipeline
CREATE TABLE pipeline_jobs (
    id BIGSERIAL PRIMARY KEY,
    state VARCHAR(50) DEFAULT 'available', -- available, running, completed, retry
    release_id UUID REFERENCES releases(id),
    attempt INT DEFAULT 0,
    max_attempts INT DEFAULT 3,
    locked_at TIMESTAMP WITH TIME ZONE,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

-- Tracks routing rules for specific repos and channels
CREATE TABLE subscriptions (
    id SERIAL PRIMARY KEY,
    repository VARCHAR(255) NOT NULL,
    channel_type VARCHAR(50) NOT NULL, -- 'stable', 'pre-release', 'security'
    notification_target VARCHAR(255) NOT NULL -- e.g., 'slack_channel_id'
);

```

## 4. Error Handling & Idempotency

* **Idempotent Ingestion:** The `releases` table enforces a unique constraint on `(repository, version)`. If a polling worker and a webhook both report the same release simultaneously, the database rejects the duplicate, preventing duplicate pipeline jobs.
* **Dead-Letter Queue (DLQ):** If a DAG pipeline job fails `max_attempts` (e.g., the LLM API is down), the `pipeline_jobs` state is updated to `discarded`. A separate Postgres trigger alerts the system admin that an event requires manual intervention.
* **Agent Fallback:** If the SRE Agent sandbox deployment fails due to infrastructure timeout (unrelated to the software release itself), the agent safely aborts the validation phase, flags the release event as "Unverified," and routes it to a human reviewer rather than dropping the notification.