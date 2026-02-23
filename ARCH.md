# Architecture: ReleaseGuard

## Overview

ReleaseGuard is an event-driven, hybrid-architecture system designed to centralize release discovery, automate validation, manage configurations, and distribute targeted notifications. It combines the high-concurrency performance of a Go-based polling engine with the reasoning capabilities of LLM-based SRE agents. By leveraging PostgreSQL for both persistent storage and message brokering, the system maintains high reliability and transactional consistency within a streamlined, single-binary deployment.

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

The system is decoupled into four primary layers communicating entirely through PostgreSQL using the Transactional Outbox pattern.

```mermaid
graph LR
    %% Data Flow
    subgraph Ingestion [Ingestion Layer]
        I_Hub(Docker Hub) --> |Provider| Engine
        I_Git(GitHub Webhooks) --> |Provider| Engine
    end

    subgraph Core [Event Bus & Queue - PostgreSQL]
        Engine((Go Engine)) --> |INSERT / Transaction| PG[(PostgreSQL)]
        PG --> |SKIP LOCKED (River)| DAG[DAG Processing Pipeline]
        DAG --> |UPDATE status| PG
    end

    subgraph Intelligence [Agentic Validation]
        DAG -.-> |Trigger Tool| Agent[SRE Agent]
        Agent -.-> |Verify| Sandbox[(Sandbox / Base A Box)]
    end

    subgraph Output [Routing & Notification]
        PG --> |LISTEN / Job Route| Router[Notification Matrix]
        Router --> O_Slack(Slack/Teams)
        Router --> O_Opsack(Ops Opsack)
    end

```

### 2.1 The Event-Driven Backbone (PostgreSQL Only)

Components do not call each other synchronously. Instead, they rely on PostgreSQL to guarantee delivery:

* **The Transactional Outbox:** When a new release is detected, the ingestion worker writes the metadata to the `releases` table and simultaneously inserts a processing job into the `pipeline_jobs` table *within the exact same SQL transaction*. This guarantees no events are ever lost.
* **Real-time Pub/Sub:** Database triggers use `pg_notify` to broadcast lightweight events (e.g., telling the Next.js UI to refresh) over standard Postgres connections using `LISTEN`.
* **Reliable Queues:** Go background workers poll the `pipeline_jobs` table using `FOR UPDATE SKIP LOCKED`, ensuring only one worker processes a specific release's DAG pipeline at a time.

### 2.2 Provider Interfaces (I/O)

All external integrations are abstracted behind strict Go interfaces.

* `IIngestionSource`: Standardizes how polling workers fetch data. Adding a new registry (like npm or NuGet) only requires implementing this interface.
* `INotificationChannel`: Standardizes output routing.

### 2.3 The DAG Processing Pipeline

The core filtering and scoring logic is structured as a Directed Acyclic Graph (DAG). Instead of monolithic conditional blocks, the system compiles a pipeline of independent execution nodes.

1. **Regex Node:** Filters `-alpha`, `-beta`, `-rc`.
2. **Semantic Node:** LLM parses the changelog for keywords (e.g., "token compiler", "login regression").
3. **Urgency Node:** Calculates a composite urgency score.
An Intermediate Representation (IR) of the `ReleaseEvent` is passed sequentially through these nodes, allowing for seamless injection of new validation steps without refactoring the core engine.

### 2.4 Agentic Tooling

For deep validation, ReleaseGuard utilizes SRE agents. The agent is provided a suite of abstracted tools:

* `UpgradeBaseABoxConfig(version)`
* `CheckAgentStatus(environment)`
* `SyncOpsOpsack(payload)`
This allows the agent to autonomously deploy a sandbox, verify that the deployment is healthy, and rollback or alert on failure.

## 3. Data Flow: Lifecycle of a Release Event

1. **Discovery:** A Go worker polling Docker Hub detects a new base image tag.
2. **Ingestion & Transaction:** The worker standardizes the payload and executes a single database transaction to insert the record into `releases` and queue a job in `pipeline_jobs`.
3. **Processing (Queue Pull):** A Go worker pulls the job using `SKIP LOCKED` and routes the event through the DAG pipeline. Unsubscribed pre-releases are marked as `skipped`.
4. **Analysis & Validation:** For stable releases, the LLM analyzes the release notes to generate a "Production Record" summary. If the urgency score meets a threshold, an SRE agent is triggered to draft a master config update for Base A Box and run automated health checks.
5. **Finalization & Broadcast:** The worker updates the job status to `completed` in PostgreSQL. A Postgres trigger fires a `NOTIFY` payload to alert the routing matrix.
6. **Notification:** The Notification Matrix reads the finalized data and routes it to the appropriate channels (e.g., a critical PagerDuty alert for hotfixes, or a quiet Slack message to the "Hyper research" channel).

## 4. Directory Structure

```text
/releaseguard
├── cmd/
│   └── server/          # Main Go application entry point
├── internal/
│   ├── ingestion/       # Polling workers and webhook handlers (IIngestionSource)
│   ├── pipeline/        # DAG node implementations for filtering & scoring
│   ├── agents/          # LLM orchestration and validation tools
│   ├── routing/         # Notification matrix and I/O providers (INotificationChannel)
│   ├── queue/           # PostgreSQL River queue setup and job definitions
│   └── models/          # Shared domain structs (ReleaseEvent, etc.)
├── web/                 # Next.js frontend application
│   ├── app/
│   └── components/
├── deployments/         # Dockerfiles and Base A Box integration scripts
└── go.mod

```