# Session Context

## User Prompts

### Prompt 1

Implement the following plan:

# Rename Subscription Types: source→source_release, project→semantic_release

## Context

The current subscription type values `"source"` and `"project"` are confusing — they describe what entity you're subscribing to, not what you receive. Users expect to choose between "Source Releases" (raw release notifications) and "Semantic Releases" (AI-analyzed reports). Renaming to `source_release` / `semantic_release` makes the intent clear.

## Scope

Rename the su...

### Prompt 2

➜  ReleaseBeacon git:(main) ✗ LOG_LEVEL=debug make run
go build -o changelogue ./cmd/server
DATABASE_URL="postgres://postgres:postgres@localhost:5432/releaseguard?sslmode=disable" LISTEN_ADDR=":8080" NO_AUTH=true ./changelogue
time=2026-03-01T14:19:57.579+08:00 level=ERROR msg="migrations failed" err="subscription type migration: ERROR: new row for relation \"subscriptions\" violates check constraint \"subscriptions_check\" (SQLSTATE 23514)"

### Prompt 3

all good, please support batch delete for subscriptions as well

### Prompt 4

[Request interrupted by user for tool use]

