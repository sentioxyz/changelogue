# GitHub Atom Feed Polling Source

**Date:** 2026-02-26
**Status:** Approved

## Summary

Replace the push-based `GitHubWebhookHandler` with a poll-based `GitHubAtomSource` that implements `IIngestionSource`. The Atom feed at `https://github.com/{owner}/{repo}/releases.atom` is fetched each polling cycle. No new dependencies — uses Go's `encoding/xml`.

## Design

### GitHubAtomSource

New file `internal/ingestion/github_atom.go`. Struct mirrors DockerHub: `client`, `repository`, `baseURL`, `sourceID`. `FetchNewReleases()` fetches the Atom feed, parses XML, and maps entries to `IngestionResult`.

Atom XML structs (inline):

```go
type atomFeed struct {
    Entries []atomEntry `xml:"entry"`
}
type atomEntry struct {
    ID      string `xml:"id"`
    Title   string `xml:"title"`
    Updated string `xml:"updated"`
    Content string `xml:"content"`
    Link    struct {
        Href string `xml:"href,attr"`
    } `xml:"link"`
}
```

Version extraction: `<id>` field format is `tag:github.com,2008:Repository/15452919/v1.17.0` — split on `/` and take the last segment to get the tag name.

### Data Mapping

| Atom field | IngestionResult field | Notes |
|------------|----------------------|-------|
| `<id>` last segment | `RawVersion` | e.g. `v1.17.0` |
| `<content>` | `Changelog` | HTML release notes |
| `<updated>` | `Timestamp` | RFC3339 |
| constructor arg | `Repository` | e.g. `ethereum/go-ethereum` |

### Changes

| File | Action | Detail |
|------|--------|--------|
| `internal/ingestion/github_atom.go` | Create | `GitHubAtomSource` implementing `IIngestionSource` |
| `internal/ingestion/github_atom_test.go` | Create | Unit tests with mock HTTP server |
| `internal/ingestion/loader.go` | Edit | Add `case "github":` in `buildSource()` |
| `internal/ingestion/github.go` | Delete | Remove webhook handler |
| `internal/ingestion/github_test.go` | Delete | Remove webhook tests |
| `cmd/server/main.go` | Edit | Remove webhook wiring and `GITHUB_WEBHOOK_SECRET` |

### Unchanged

- `sources` table still uses `provider = "github"` — no schema change
- Orchestrator, service, pgstore, transactional outbox untouched
- Deduplication via `UNIQUE(source_id, version)` works as-is
