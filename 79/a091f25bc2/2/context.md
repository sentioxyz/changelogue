# Session Context

## User Prompts

### Prompt 1

Implement the following plan:

# Fix: NotifyWorker's auto-triggered agent runs stuck as "pending"

## Context

When the ingestion layer detects a new release that matches a project's agent rules (e.g., `on_major_release`), the `NotifyWorker` calls `EnqueueAgentRun()` to create an agent run and enqueue a River job. However, the `agent_runs` row is created with status `pending` but **no River job is ever enqueued**, leaving the run orphaned forever.

**Root cause:** In `cmd/server/main.go`, the `N...

### Prompt 2

commit and push

