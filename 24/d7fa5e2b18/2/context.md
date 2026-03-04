# Session Context

## User Prompts

### Prompt 1

I want to make some redesign on the default page -- dashboard so that users can have some shortcuts to onboard new projects quickly from the github trends/stars from the users or the dockerhub stars etc.

### Prompt 2

Base directory for this skill: /Users/pc/.claude/plugins/cache/superpowers-marketplace/superpowers/4.3.1/skills/brainstorming

# Brainstorming Ideas Into Designs

## Overview

Help turn ideas into fully formed designs and specs through natural collaborative dialogue.

Start by understanding the current project context, then ask questions one at a time to refine the idea. Once you understand what you're building, present the design and get user approval.

<HARD-GATE>
Do NOT invoke any implementat...

### Prompt 3

A

### Prompt 4

Yes, please remember to update the skill to add new provider for this, if not support we can skip

### Prompt 5

Base directory for this skill: /Users/pc/web3/ReleaseBeacon/.claude/skills/adding-a-provider

# Adding a Provider

## Overview

Every provider must be registered in **10 locations** across backend, frontend, and tests. Missing any one causes silent failures or missing UI elements.

## Checklist

### Backend (Go)

1. **`internal/ingestion/<provider>.go`** — Create. Implement `IIngestionSource` interface (`Name()`, `SourceID()`, `FetchNewReleases()`). Follow `github.go` as template.

2. **`inter...

### Prompt 6

Yes

### Prompt 7

Yes

### Prompt 8

yes

### Prompt 9

Base directory for this skill: /Users/pc/.claude/plugins/cache/superpowers-marketplace/superpowers/4.3.1/skills/writing-plans

# Writing Plans

## Overview

Write comprehensive implementation plans assuming the engineer has zero context for our codebase and questionable taste. Document everything they need to know: which files to touch for each task, code, testing, docs they might need to check, how to test it. Give them the whole plan as bite-sized tasks. DRY. YAGNI. TDD. Frequent commits.

Ass...

