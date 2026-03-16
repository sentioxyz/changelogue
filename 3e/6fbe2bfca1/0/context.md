# Session Context

## User Prompts

### Prompt 1

Implement the following plan:

# Plan: Auto-updating Code Coverage Badge via CI

## Context

The project has no code coverage reporting. The user wants coverage surfaced through the CI pipeline with an auto-updated badge in the README. The repo already uses `Schneegans/dynamic-badges-action@v1.7.0` + a GitHub Gist for the "AI Co-Authored" badge (`.github/workflows/ai-badge.yml`), so we replicate the same pattern.

## Changes

### 1. Update `.github/workflows/ci.yml` — add coverage step to back...

### Prompt 2

commit and push

