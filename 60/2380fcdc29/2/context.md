# Session Context

## User Prompts

### Prompt 1

Implement the following plan:

# Fix: Join project_name in semantic releases backend queries

## Context
The semantic releases page shows "—" for project names when a project falls outside the first 25 results fetched by the frontend's `projectsApi.list()` call. The proper fix is to JOIN `projects.name` in the backend SQL, following the existing `Release` model pattern, eliminating the need for a separate project name lookup.

## Changes

### 1. Add `ProjectName` to Go model
**File:** `interna...

### Prompt 2

cool, commit and push

### Prompt 3

the project filter in the semantic releases page seems not right still

### Prompt 4

yes

