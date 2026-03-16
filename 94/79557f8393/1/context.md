# Session Context

## User Prompts

### Prompt 1

Add the provider for npm registry

### Prompt 2

Base directory for this skill: /Users/pc/web3/ReleaseBeacon/.claude/skills/adding-a-provider

# Adding a Provider

## Overview

Every provider must be registered in **10 locations** across backend, frontend, and tests. Missing any one causes silent failures or missing UI elements.

## Checklist

### Backend (Go)

1. **`internal/ingestion/<provider>.go`** — Create. Implement `IIngestionSource` interface (`Name()`, `SourceID()`, `FetchNewReleases()`). Follow `github.go` as template.

2. **`inter...

### Prompt 3

Base directory for this skill: /Users/pc/.claude/plugins/cache/superpowers-marketplace/superpowers/5.0.2/skills/brainstorming

# Brainstorming Ideas Into Designs

Help turn ideas into fully formed designs and specs through natural collaborative dialogue.

Start by understanding the current project context, then ask questions one at a time to refine the idea. Once you understand what you're building, present the design and get user approval.

<HARD-GATE>
Do NOT invoke any implementation skill, wr...

