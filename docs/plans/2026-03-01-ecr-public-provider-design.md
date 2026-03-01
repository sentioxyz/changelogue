# AWS ECR Public Provider Design

**Date:** 2026-03-01
**Status:** Approved

## Goal

Add support for AWS ECR Public (`gallery.ecr.aws`) as a new source provider, enabling polling for new container image tags from public ECR repositories.

## Approach

Use the Docker Registry v2 HTTP API at `public.ecr.aws` — same pattern as the existing Docker Hub provider. No AWS SDK dependency.

### Repository Format

Users provide `{registry_alias}/{repo_name}`, e.g., `i6b2w2n6/op-node` (from `gallery.ecr.aws/i6b2w2n6/op-node`).

### Authentication Flow

ECR Public requires a bearer token even for public repos:

1. `GET https://public.ecr.aws/token/` → `{"token": "..."}`
2. Use token as `Authorization: Bearer <token>` on subsequent requests

### Fetching Tags

`GET https://public.ecr.aws/v2/{alias}/{repo}/tags/list`

Response: `{"name": "{alias}/{repo}", "tags": ["v1.0", "v1.1", ...]}`

The v2 tags/list endpoint returns tag names only (no timestamps). `Timestamp` set to `time.Now()` for discovered tags.

## Files Changed

| File | Change |
|------|--------|
| `internal/ingestion/ecr_public.go` | New — `ECRPublicSource` implementing `IIngestionSource` |
| `internal/ingestion/ecr_public_test.go` | New — tests with `httptest` mock |
| `internal/ingestion/loader.go` | Add `"ecr-public"` case to `BuildSource()` |
| `internal/api/providers.go` | Add `ecr-public` to providers list |

## What Doesn't Change

- No database schema changes
- No new Go dependencies
- No changes to ingestion service, orchestrator, or notification pipeline
