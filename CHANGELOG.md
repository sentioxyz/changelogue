# Changelog

All notable changes to Changelogue will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [0.1.0] — 2026-03-23

Initial release of Changelogue — an agent-driven release intelligence platform
that polls upstream registries for new releases and uses LLM agents to produce
semantic release reports.

### Added

#### Ingestion Engine
- Polling orchestrator with configurable intervals per source
- Docker Hub, GitHub Releases, ECR Public, GitLab, and PyPI providers
- GitHub release webhook handler with HMAC signature verification
- Transactional outbox pattern for zero-loss event delivery

#### Agent Intelligence
- Google ADK-Go (Gemini) agent orchestrator for semantic release analysis
- Agent tools for fetching releases and context sources
- Agent rules engine with version-based auto-trigger for agent runs
- River-backed async worker for agent job processing

#### Notification System
- Subscription-based notification routing
- Webhook, Slack, and Discord senders
- River worker for reliable notification delivery

#### REST API
- Full CRUD for projects, sources, subscriptions, and notification channels
- Releases endpoint with pagination and pipeline status
- Health check, dashboard stats, and providers metadata endpoints
- SSE broadcaster with PostgreSQL `LISTEN`/`NOTIFY` for real-time events
- API key authentication and rate limiting middleware
- GitHub OAuth authentication with `NO_AUTH` dev mode
- Personalized suggestions (starred repos, dependency scanning)

#### Web Dashboard
- Next.js 15 static export embedded in Go binary via `//go:embed`
- Dashboard with stats cards, recent releases, and live activity feed
- Projects, sources, releases, subscriptions, and channels CRUD pages
- Quick onboard flow for new users
- Personalized GitHub suggestions (stars and dependency tabs)
- Dark mode support
- Internationalization (English and Chinese)
- Settings dialog with theme and language controls

#### CLI (`clog`)
- Cobra-based CLI with projects, sources, releases, subscriptions, and channels subcommands
- Table and JSON output formats
- AI-friendly error hints for typos and unknown flags

#### Infrastructure
- PostgreSQL database with schema migrations
- River v0.31.0 job queue integration
- Single-binary deployment (Go server + embedded frontend)
- Integration test script with isolated Postgres on port 5433
- Docker Compose for local development
- Makefile with `dev`, `up`, `run`, `db-reset`, and `clean` targets
