# ReleaseGuard Pivot Design

## Overview

Major pivot of ReleaseGuard from a pipeline-centric release processing system to an agent-driven release intelligence platform. The system monitors upstream sources for new releases and provides two notification paths: simple source-level release notifications, and AI-generated semantic releases that correlate information across multiple sources.

## Entity Model

### Four Core Entities

**Project** — Semantic grouping for a software project (e.g., "Geth"). Contains a user-written agent prompt for LLM research and configurable trigger rules. Projects own sources, context sources, and semantic releases.

**Source (polling)** — A release feed from a provider (GitHub, Docker Hub). Belongs to one project. The ingestion layer polls these for new releases. Each source has provider-specific config and exclusion filters.

**Channel** — A notification target (Slack, email, webhook, PagerDuty, Discord). Standalone entity, not tied to projects.

**Subscription** — Links a channel to either a source or a project. Two distinct types:
- *Source subscription*: auto-notifies on every new source release with release notes
- *Project subscription*: notifies when a new semantic release (AI report) is completed

### Supporting Entities

**Release** — A release detected from a polling source. Unique on `(source_id, version)`. Contains raw payload from the provider.

**Context Source** — Read-only reference the agent browses during research. Types: Discord channels, web pages, RSS feeds, forums. Belongs to a project.

**Semantic Release** — An AI-generated project-level release that correlates multiple source releases. Contains the structured report (availability, adoption, urgency). Links to constituent source releases via a join table.

**Agent Run** — Record of an agent execution. Scoped to a project, produces a semantic release. Tracks trigger reason, status, and errors.

## Database Schema

```sql
CREATE TABLE projects (
    id           UUID PRIMARY KEY,
    name         TEXT NOT NULL UNIQUE,
    description  TEXT,
    agent_prompt TEXT,
    agent_rules  JSONB,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at   TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE sources (
    id            UUID PRIMARY KEY,
    project_id    UUID NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    provider      TEXT NOT NULL,
    repository    TEXT NOT NULL,
    poll_interval INTERVAL DEFAULT '15m',
    config        JSONB,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (provider, repository)
);

CREATE TABLE context_sources (
    id         UUID PRIMARY KEY,
    project_id UUID NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    type       TEXT NOT NULL,
    name       TEXT NOT NULL,
    config     JSONB NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE releases (
    id          UUID PRIMARY KEY,
    source_id   UUID NOT NULL REFERENCES sources(id) ON DELETE CASCADE,
    version     TEXT NOT NULL,
    raw_data    JSONB,
    released_at TIMESTAMPTZ,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (source_id, version)
);

CREATE TABLE semantic_releases (
    id           UUID PRIMARY KEY,
    project_id   UUID NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    version      TEXT NOT NULL,
    report       JSONB,
    status       TEXT NOT NULL DEFAULT 'pending',
    error        TEXT,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    completed_at TIMESTAMPTZ,
    UNIQUE (project_id, version)
);

CREATE TABLE semantic_release_sources (
    semantic_release_id UUID NOT NULL REFERENCES semantic_releases(id) ON DELETE CASCADE,
    release_id          UUID NOT NULL REFERENCES releases(id) ON DELETE CASCADE,
    PRIMARY KEY (semantic_release_id, release_id)
);

CREATE TABLE notification_channels (
    id         UUID PRIMARY KEY,
    name       TEXT NOT NULL,
    type       TEXT NOT NULL,
    config     JSONB NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE subscriptions (
    id             UUID PRIMARY KEY,
    channel_id     UUID NOT NULL REFERENCES notification_channels(id) ON DELETE CASCADE,
    type           TEXT NOT NULL CHECK (type IN ('source', 'project')),
    source_id      UUID REFERENCES sources(id) ON DELETE CASCADE,
    project_id     UUID REFERENCES projects(id) ON DELETE CASCADE,
    version_filter TEXT,
    created_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
    CHECK (
        (type = 'source'  AND source_id  IS NOT NULL AND project_id IS NULL) OR
        (type = 'project' AND project_id IS NOT NULL AND source_id  IS NULL)
    )
);

CREATE TABLE agent_runs (
    id                  UUID PRIMARY KEY,
    project_id          UUID NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    semantic_release_id UUID REFERENCES semantic_releases(id) ON DELETE SET NULL,
    trigger             TEXT NOT NULL,
    status              TEXT NOT NULL DEFAULT 'pending',
    prompt_used         TEXT,
    error               TEXT,
    started_at          TIMESTAMPTZ,
    completed_at        TIMESTAMPTZ,
    created_at          TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE api_keys (
    id         UUID PRIMARY KEY,
    name       TEXT NOT NULL,
    key_hash   TEXT NOT NULL UNIQUE,
    prefix     TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
```

## Data Flow

### Source-Level Flow (Simple Notifications)

```
1. Ingestion poller/webhook detects new release on a Source
2. Transactional outbox: INSERT release + enqueue River job (same TX)
3. River worker picks up job:
   a. Look up source-level subscriptions for this source
   b. For each subscription → format release notes → send to channel
   c. Check project's agent_rules → if triggered, enqueue agent job
4. NOTIFY 'new_release' → SSE → dashboard updates
```

### Project-Level Flow (Agent + Semantic Releases)

```
1. Agent triggered (manual API call or rule match on new release)
2. INSERT agent_run (status=pending), enqueue River agent job
3. River worker picks up agent job:
   a. Load project + all sources + latest releases + context sources
   b. Group source releases by version
   c. Call LLM (via ADK-Go) with project's agent_prompt + release context
   d. LLM uses tools to research: availability, adoption, urgency, context sources
   e. LLM returns structured SemanticReport
   f. INSERT semantic_release with report, link to source releases
   g. UPDATE agent_run (status=completed)
4. Look up project-level subscriptions
5. For each subscription → format semantic report → send to channel
6. NOTIFY 'semantic_release_completed' → SSE → dashboard
```

## Agent Architecture (ADK-Go)

### Framework

Uses Google ADK-Go (`google.golang.org/adk` v0.5.0) for agent orchestration. The `model.LLM` interface enables pluggable LLM backends (Gemini built-in, custom adapters for Claude and OpenAI).

### Agent Definition

```go
agent, _ := llmagent.New(llmagent.Config{
    Name:         "release_researcher",
    Description:  "Researches releases across a project's sources",
    Model:        model,             // pluggable via model.LLM interface
    Instruction:  projectPrompt,     // user's custom prompt from project.agent_prompt
    Tools:        researchTools,     // see below
    OutputSchema: semanticReportSchema,
    OutputKey:    "report",
})
```

### Agent Tools

| Tool | Purpose |
|------|---------|
| `get_releases` | Fetch latest releases for all sources in the project |
| `get_release_notes` | Get release notes/changelog for a specific release |
| `browse_context_sources` | Fetch content from configured context sources (Discord, blogs, RSS, forums) |
| `check_availability` | Verify a Docker image tag exists and is pullable |
| `check_adoption` | Look up download stats, npm installs, Docker Hub pulls |
| `web_search` | General web research for community sentiment, breaking changes |

### Custom LLM Providers

Thin adapters in `internal/agent/providers/` for Claude and OpenAI. Each implements `model.LLM` by translating between `genai.Content` format and the provider's native format.

### Agent Rules Engine

```go
type AgentRules struct {
    OnMajorRelease  bool   `json:"on_major_release"`
    OnMinorRelease  bool   `json:"on_minor_release"`
    OnSecurityPatch bool   `json:"on_security_patch"`
    VersionPattern  string `json:"version_pattern"` // regex
}
```

When a new release is ingested, the project's rules are checked against the release version. If matched, an agent run is auto-enqueued.

### Semantic Report Structure

```go
type SemanticReport struct {
    Summary        string             `json:"summary"`
    Sources        []SourceAnalysis   `json:"sources"`
    Availability   AvailabilityStatus `json:"availability"`
    Adoption       AdoptionMetrics    `json:"adoption"`
    Urgency        UrgencyAssessment  `json:"urgency"`
    Recommendation string             `json:"recommendation"`
}
```

## Context Sources

Read-only references the agent browses during research. Configured per-project.

| Type | Config | Agent Action |
|------|--------|-------------|
| `discord` | `webhook_url`, `channel_id` | Read recent messages via Discord API |
| `webpage` | `url` | HTTP GET + HTML-to-text extraction |
| `rss` | `feed_url` | Parse RSS feed for recent entries |
| `forum` | `url`, `api_key` | Fetch recent posts via forum API |

## REST API

All endpoints under `/api/v1`. List endpoints support cursor-based pagination (`?cursor=<id>&limit=<n>`).

### Projects
| Method | Path | Description |
|--------|------|-------------|
| GET | `/projects` | List projects |
| POST | `/projects` | Create project |
| GET | `/projects/:id` | Get project detail |
| PUT | `/projects/:id` | Update project |
| DELETE | `/projects/:id` | Delete project (cascades) |

### Sources (polling)
| Method | Path | Description |
|--------|------|-------------|
| GET | `/projects/:id/sources` | List sources for a project |
| POST | `/projects/:id/sources` | Add source to project |
| GET | `/sources/:id` | Get source detail |
| PUT | `/sources/:id` | Update source |
| DELETE | `/sources/:id` | Delete source |

### Context Sources
| Method | Path | Description |
|--------|------|-------------|
| GET | `/projects/:id/context-sources` | List context sources |
| POST | `/projects/:id/context-sources` | Add context source |
| GET | `/context-sources/:id` | Get context source detail |
| PUT | `/context-sources/:id` | Update context source |
| DELETE | `/context-sources/:id` | Delete context source |

### Releases
| Method | Path | Description |
|--------|------|-------------|
| GET | `/sources/:id/releases` | List releases for a source |
| GET | `/projects/:id/releases` | List all releases across project's sources |
| GET | `/releases/:id` | Get release detail |

### Semantic Releases
| Method | Path | Description |
|--------|------|-------------|
| GET | `/projects/:id/semantic-releases` | List semantic releases (paginated) |
| GET | `/semantic-releases/:id` | Get detail (report + linked source releases) |

### Agent
| Method | Path | Description |
|--------|------|-------------|
| POST | `/projects/:id/agent/run` | Trigger agent run |
| GET | `/projects/:id/agent/runs` | List agent runs (paginated) |
| GET | `/agent-runs/:id` | Get agent run detail |

### Channels
| Method | Path | Description |
|--------|------|-------------|
| GET | `/channels` | List channels |
| POST | `/channels` | Create channel |
| GET | `/channels/:id` | Get channel |
| PUT | `/channels/:id` | Update channel |
| DELETE | `/channels/:id` | Delete channel |

### Subscriptions
| Method | Path | Description |
|--------|------|-------------|
| GET | `/subscriptions` | List (filterable by source/project/channel) |
| POST | `/subscriptions` | Create subscription |
| GET | `/subscriptions/:id` | Get subscription |
| PUT | `/subscriptions/:id` | Update subscription |
| DELETE | `/subscriptions/:id` | Delete subscription |

### System
| Method | Path | Description |
|--------|------|-------------|
| POST | `/webhook/github` | GitHub release webhook |
| GET | `/events` | SSE stream |
| GET | `/health` | Health check |
| GET | `/stats` | System statistics |

## Notification Routing

### Channel Types

| Type | Config Fields | Delivery |
|------|--------------|----------|
| `slack` | `webhook_url`, `channel` | POST to Slack incoming webhook |
| `email` | `to`, `smtp_config` | Send formatted email |
| `webhook` | `url`, `headers`, `method` | HTTP POST/PUT with JSON payload |
| `pagerduty` | `routing_key`, `severity` | PagerDuty Events API v2 |
| `discord` | `webhook_url` | POST to Discord webhook with embeds |

### Delivery

Source subscriptions send release notes on every new source release. Project subscriptions send semantic reports when agent completes. Failed sends retry via River jobs with exponential backoff.

## Frontend

### Pages

| Page | Purpose |
|------|---------|
| Dashboard | Recent source releases, semantic releases, agent status |
| Projects list | All projects with source count, latest activity |
| Project detail | Sources, context sources, semantic releases, agent config |
| Project > Sources | CRUD for polling sources |
| Project > Context Sources | CRUD for context sources |
| Project > Semantic Releases | List with AI reports |
| Project > Agent | Prompt editor, rules config, run history, manual trigger |
| Releases | Global view of source releases |
| Semantic Release detail | Full report, linked source releases |
| Channels | CRUD for notification channels |
| Subscriptions | CRUD with source/project type selector |

### User Flows

**Simple (source notifications only):**
Create project → Add source → Create channel → Create source subscription → Done

**Full (with AI analysis):**
Create project → Add sources → Add context sources → Write agent prompt → Set rules → Create channel → Create project subscription → Done

### Technical

Remove MSW mocks. Wire to real REST API with SWR. Next.js 15 with static export embedded via Go `//go:embed`.

## What Changes from Current System

### Removed
- Pipeline with sequential configurable nodes (Regex Normalizer, Subscription Router, Urgency Scorer)
- Pipeline runner and pipeline jobs table
- Node interface and node configuration

### Kept
- Ingestion layer (Docker Hub poller, GitHub webhook, `IIngestionSource` interface)
- Transactional outbox pattern (release + job in one TX)
- River job queue for async processing
- PostgreSQL LISTEN/NOTIFY → SSE for real-time updates
- API middleware (auth, rate limiting, CORS, request IDs)
- Next.js frontend scaffold with shadcn/ui

### Added
- Context sources entity and CRUD
- Semantic releases with join to source releases
- Agent runs with ADK-Go integration
- Agent tools (availability, adoption, context browsing, web search)
- Agent rules engine for auto-triggering
- Notification routing with five channel types
- Source-level and project-level subscriptions as distinct types

## Implementation Approach

Phased pivot:

1. **Phase 1: Entity Model** — Rework schema, migrations, models, and CRUD APIs
2. **Phase 2: Source Subscriptions** — Wire ingestion to source-level notifications
3. **Phase 3: Agent** — ADK-Go integration, tools, orchestrator, semantic releases
4. **Phase 4: Project Subscriptions** — Semantic report notifications
5. **Phase 5: Frontend** — Restructure dashboard, wire to real API
