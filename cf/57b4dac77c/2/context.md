# Session Context

## User Prompts

### Prompt 1

Implement the following plan:

# Add Provider Icons to Release Chips on Projects Page

## Context

The projects list page (`web/app/projects/page.tsx`) shows recent releases as version chips (e.g. `v1.25.0`), but there's no visual indicator of which source provider (GitHub vs Docker Hub) each release came from. Users want to see at a glance which provider a release originated from.

## Approach: Frontend-only lookup

The `ReleaseChips` component already has `projectId`. We'll fetch sources for t...

### Prompt 2

I would like to see the similar icon on the dashboard "recent source releases" as well

### Prompt 3

looks good, commit and push

