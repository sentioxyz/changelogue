# Architecture: Changelogue

## Overview

Changelogue is an event-driven, hybrid-architecture system designed to centralize release discovery, automate validation, manage configurations, and distribute targeted notifications. It combines the high-concurrency performance of a Go-based polling engine with the reasoning capabilities of LLM-based SRE agents. By leveraging PostgreSQL for both persistent storage and message brokering, the system maintains high reliability and transactional consistency within a streamlined, single-binary deployment.

## 1. Tech Stack & Infrastructure

* **Backend / Engine:** Go (Golang). Chosen for its lightweight concurrency model (Goroutines) to handle simultaneous polling across multiple registries.
* **Frontend / Dashboard:** Next.js (React) with Tailwind CSS. Provides a fast, modern control center for viewing release streams and managing configurations.
* **Database, Queue, & Event Bus:** PostgreSQL.
* *Persistent Storage:* Standard relational tables for release metadata and system configurations.
* *Task Queue:* Utilizes `FOR UPDATE SKIP LOCKED` (via the **River** Go library) for robust, concurrent background job processing without race conditions.
* *Pub/Sub:* Utilizes native `LISTEN` and `NOTIFY` for real-time broadcasting to connected clients.


* **Intelligence Layer:** LLMs (Gemini / GPT-4o-mini) orchestrated via agent frameworks (e.g., LangGraph or Claude Agent SDK) for semantic changelog analysis and autonomous validation.
* **Packaging:** Single binary deployment utilizing Go's `//go:embed` to serve the Next.js static export directly from the Go server.

## 2. System Design & Abstractions

The system is decoupled into four primary layers communicating entirely through PostgreSQL using the Transactional Outbox pattern:

1. **Ingestion Layer** — Polling workers and webhook handlers discover new releases from upstream registries.
2. **Notification Routing** — River workers send immediate notifications to source-level subscribers on new releases.
3. **Agent Layer** — ADK-Go agents research releases, consult context sources, and produce semantic release reports.
4. **Routing & Output** — Notification channels (Slack, PagerDuty, webhooks) deliver alerts via `INotificationChannel`.

A **Project** is the central domain entity — it groups multiple ingestion sources, context sources, and notification subscriptions under a single tracked piece of software.

```mermaid
graph LR
    %% Data Flow
    subgraph Dashboard [API & Dashboard]
        API[REST API] --> |CRUD| PG
        API --> |SSE| Browser[Next.js Dashboard]
    end

    subgraph Ingestion [Ingestion Layer]
        I_Hub(Docker Hub Poller) --> |IIngestionSource| Engine
        I_Git(GitHub Webhooks) --> |HTTP Handler| Engine
    end

    subgraph Core [Event Bus & Queue - PostgreSQL]
        Engine((Go Engine)) --> |INSERT release + jobs\nTransactional Outbox| PG[(PostgreSQL)]
    end

    subgraph Notification [Notification Routing]
        PG --> |SKIP LOCKED\nRiver notify_release| NotifyWorker[Notify Worker]
        NotifyWorker --> |INotificationChannel| O_Slack(Slack/Teams)
        NotifyWorker --> |INotificationChannel| O_PD(PagerDuty)
        NotifyWorker --> |INotificationChannel| O_Hook(Custom Webhooks)
    end

    subgraph AgentLayer [Agent Layer]
        PG --> |SKIP LOCKED\nRiver agent_run| AgentWorker[Agent Worker]
        AgentWorker --> |ADK-Go| LLM[LLM Provider]
        AgentWorker --> |Research| ContextSources[Context Sources]
        AgentWorker --> |Produce| SemanticRelease[Semantic Release + Report]
    end

```

### Entity Relationship Model

```mermaid
erDiagram
    projects ||--o{ sources : "has many"
    projects ||--o{ context_sources : "has many"
    projects ||--o{ semantic_releases : "has many"
    projects ||--o{ agent_runs : "has many"
    sources ||--o{ releases : "produces"
    subscriptions }o--|| notification_channels : "routes to"
    subscriptions }o--o| sources : "source-level"
    subscriptions }o--o| projects : "project-level"
    semantic_releases ||--o{ semantic_release_sources : "composed of"
    releases ||--o{ semantic_release_sources : "contributes to"

    projects {
        uuid id PK
        string name UK
        text agent_prompt
        jsonb agent_rules
    }
    sources {
        uuid id PK
        uuid project_id FK
        string provider
        string repository UK
        boolean enabled
    }
    releases {
        uuid id PK
        uuid source_id FK
        string version
        jsonb raw_data
    }
    semantic_releases {
        uuid id PK
        uuid project_id FK
        string version
        jsonb report
        string status
    }
    agent_runs {
        uuid id PK
        uuid project_id FK
        uuid semantic_release_id FK
        string trigger
        string status
    }
    subscriptions {
        uuid id PK
        uuid channel_id FK
        string type
        uuid source_id FK
        uuid project_id FK
    }
    notification_channels {
        uuid id PK
        string type
        string name
        jsonb config
    }
```

### 2.1 The Event-Driven Backbone (PostgreSQL Only)

Components do not call each other synchronously. Instead, they rely on PostgreSQL to guarantee delivery:

* **The Transactional Outbox:** When a new release is detected, the ingestion worker writes the metadata to the `releases` table (linked to its `source_id`) and simultaneously enqueues River jobs (notification and/or agent) *within the exact same SQL transaction*. This guarantees no events are ever lost.
* **Real-time Pub/Sub:** Database triggers use `pg_notify` to broadcast lightweight events (e.g., telling the Next.js UI via SSE to refresh) over standard Postgres connections using `LISTEN`.
* **Reliable Queues:** River workers process jobs using `FOR UPDATE SKIP LOCKED`, ensuring exactly-once processing for both notification delivery and agent runs.
* **Project-Centric Model:** All data flows through the `projects` → `sources` → `releases` hierarchy. Subscriptions can attach at the source level (for raw release notifications) or the project level (for semantic release notifications).

### 2.2 Provider Interfaces (I/O)

All external integrations are abstracted behind strict Go interfaces.

* `IIngestionSource`: Standardizes how polling workers fetch data. Adding a new registry (like npm or NuGet) only requires implementing this interface. Each implementation maps to a `source_type` in the database.
* `INotificationChannel`: Standardizes output routing. Each implementation maps to a `type` in the `notification_channels` table (Slack, PagerDuty, custom webhooks).

### 2.3 REST API & Dashboard

The Go server exposes a RESTful API (`/api/v1`) serving the Next.js dashboard and external consumers:

* **Resource CRUD:** Projects, sources, subscriptions, notification channels — all manageable through the API.
* **Read-Only Releases:** Releases and semantic releases are queryable but not writable via the API — they're created exclusively through the ingestion layer and agent runs.
* **SSE Real-Time Events:** `GET /api/v1/events` streams server-sent events backed by PostgreSQL `LISTEN/NOTIFY`, pushing release and semantic release updates to connected dashboard clients.
* **API Key Auth:** Bearer token authentication with hashed key storage. Webhooks use their own HMAC-based auth.
* **Rate Limiting:** Per-key token bucket with standard `X-Ratelimit-*` response headers.

### 2.4 Notification Routing

When a new source release is detected, a `notify_release` River job is enqueued in the same transaction as the release insert. The notification worker:

1. Resolves all source-level subscriptions for the release's source.
2. For each subscription, sends a notification to the linked channel via `INotificationChannel`.

This provides immediate, low-latency alerts for raw releases without waiting for AI analysis. Source-level subscriptions are simple: "tell me when this source has a new release."

### 2.5 Agent Layer (Implemented)

For project-level intelligence, an `agent_run` River job triggers an LLM agent (via ADK-Go v0.5.0) that:

1. Gathers recent source releases for the project using the `get_releases` tool.
2. Inspects individual release details (changelogs, raw data) using `get_release_detail`.
3. Consults context sources (runbooks, docs, monitoring dashboards) using `list_context_sources`.
4. Produces a `SemanticRelease` with a structured `SemanticReport` (summary, availability, adoption, urgency, recommendation).

Agent behavior is configured per-project via `agent_prompt` (custom instructions) and `agent_rules` (structured triggers like `on_major_release`, `on_security_patch`, `version_pattern`). Agent runs are tracked in the `agent_runs` table for observability and auditability.

**Implementation details (`internal/agent/`):**

* **`tools.go`** — Three ADK function tools (`get_releases`, `get_release_detail`, `list_context_sources`) created via `functiontool.New()`. Tools are scoped to a project via `toolFactory` which holds the `AgentDataStore` and project ID.
* **`orchestrator.go`** — `Orchestrator` manages the full agent lifecycle: loading project config, creating an ADK `llmagent` with Gemini as the LLM backend, running the agent via `runner.Runner`, parsing the JSON report, and persisting the `SemanticRelease` in a transaction.
* **`worker.go`** — `AgentWorker` implements `river.Worker[queue.AgentJobArgs]`, loading the agent run from the store and delegating to `Orchestrator.RunAgent()`.
* **Graceful degradation** — If `GOOGLE_API_KEY` is not set, the orchestrator is not created, the worker is not registered, and a warning is logged. The rest of the system continues to function. Agent jobs remain in the River queue until the key is configured.

### 2.6 Agentic Tooling (Planned: SRE Validation)

For deep validation, Changelogue will utilize SRE agents with a suite of abstracted tools:

* `UpgradeBaseABoxConfig(version)`
* `CheckAgentStatus(environment)`
* `SyncOpsOpsack(payload)`
This will allow the agent to autonomously deploy a sandbox, verify that the deployment is healthy, and rollback or alert on failure.

## 3. Data Flow: Lifecycle of a Release Event

1. **Discovery:** A Go worker polling Docker Hub detects a new base image tag for a configured source (e.g., the "Docker Hub" source under the "Go Runtime" project). Source-level exclusion filters (`exclude_version_regexp`, `exclude_prereleases`) are applied -- filtered versions are discarded immediately.
2. **Ingestion & Transaction:** The worker stores the raw upstream payload and executes a single database transaction to insert the record into `releases` (linked to its `source_id`) and enqueue River jobs (`notify_release` for immediate notifications, optionally `agent_run` if the project's agent rules match).
3. **Notification (Source-Level):** A River worker picks up the `notify_release` job, resolves source-level subscriptions, and sends alerts to configured notification channels (Slack, PagerDuty, webhooks).
4. **Agent Analysis (Project-Level):** If triggered, a River worker picks up the `agent_run` job. The ADK-Go agent gathers recent releases, consults context sources, and produces a `SemanticRelease` with a structured report (summary, availability, adoption, urgency, recommendation).
5. **Broadcast:** PostgreSQL triggers fire `NOTIFY` payloads on release insert and semantic release completion, pushing SSE events to connected dashboard clients.
6. **Project-Level Notification:** When a semantic release is completed, project-level subscribers are notified with the AI-generated report (e.g., a critical PagerDuty alert for high-urgency releases, or a batched daily digest to Slack for low-priority updates).

## 4. Directory Structure

```text
/changelogue
├── cmd/
│   └── server/          # Main Go application entry point
├── internal/
│   ├── api/             # REST API handlers, middleware, SSE broadcaster
│   ├── ingestion/       # Polling workers and webhook handlers (IIngestionSource)
│   ├── agent/           # ADK-Go agent for semantic release analysis
│   ├── notification/    # Notification routing worker and channel implementations
│   ├── queue/           # PostgreSQL River queue setup and job definitions
│   ├── db/              # Connection pool and schema migrations
│   └── models/          # Shared domain structs (Release, Project, SemanticRelease, etc.)
├── web/                 # Next.js frontend application
│   ├── app/
│   └── components/
├── deployments/         # Dockerfiles and Base A Box integration scripts
└── go.mod

```