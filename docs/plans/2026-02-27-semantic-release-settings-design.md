# Semantic Release Settings Tab Redesign

## Overview

Redesign the "Agent" tab in the project detail page to make its purpose clearer (semantic release configuration) and allow users to trigger test agent runs by selecting a specific source + version.

## Changes

### 1. Tab Rename & Header Cleanup

- Rename tab: "Agent" → "Semantic Release Settings"
- Remove the "Run Agent" button from the project header banner

### 2. Auto-Trigger Rules Card

Card titled "Trigger Rules" with subtitle explaining automatic agent triggers:

- Checkbox: "Major release" — trigger on major version bump
- Checkbox: "Minor release" — trigger on minor version bump
- Checkbox: "Security patch" — trigger on security-related releases
- Text input: "Version pattern" — optional regex filter
- Combined "Save Settings" button (shared with prompt section)

### 3. Agent Prompt Card

Card titled "Agent Prompt" with custom instructions textarea:

- ~5 row textarea
- Ghost text when empty: "Using default agent prompt"
- Shares save action with trigger rules

### 4. Test Run Card

Card titled "Test Run" for one-off agent runs:

- Source dropdown: lists project sources as "provider: repository"
- Version dropdown: loads recent releases from selected source
- "Run Test" button: POST with `{ trigger: "test", version: "<version>" }`
- Inline status indicator after triggering

### 5. Run History Table

Unchanged — shows Trigger, Status, Started, Duration, Semantic Release columns.

## Backend Change

Add optional `version` field to `triggerRequest` struct in `internal/api/agent.go`. When present, set it directly on `agent_runs.version` column. No schema change needed (column already exists).

## Files to Modify

### Frontend
- `web/components/projects/project-detail.tsx` — Main changes: rename tab, remove Run Agent button, redesign agent tab content with 4 cards
- `web/lib/api/client.ts` — Update `agent.triggerRun()` to accept optional version parameter
- `web/lib/api/types.ts` — No changes needed (Release/Source types already sufficient)

### Backend
- `internal/api/agent.go` — Add `Version` field to `triggerRequest`, pass to store
- `internal/api/pgstore.go` — Update `TriggerAgentRun` to accept and set version
