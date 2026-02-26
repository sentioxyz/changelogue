# GitHub Atom Feed Polling Source — Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Replace the GitHub webhook-based ingestion with a polling-based source that fetches Atom feeds from `https://github.com/{owner}/{repo}/releases.atom`.

**Architecture:** New `GitHubAtomSource` implements `IIngestionSource` (same interface as `DockerHubSource`). It fetches the Atom feed via HTTP GET, parses XML with `encoding/xml`, and maps entries to `IngestionResult`. The existing orchestrator polls it on the standard 5-minute interval. No new dependencies.

**Tech Stack:** Go `encoding/xml`, `net/http`, `net/http/httptest` for tests.

**Design doc:** `docs/plans/2026-02-26-github-atom-feed-design.md`

---

### Task 1: Create GitHubAtomSource with TDD

**Files:**
- Create: `internal/ingestion/github_atom_test.go`
- Create: `internal/ingestion/github_atom.go`

**Step 1: Write the failing tests**

Create `internal/ingestion/github_atom_test.go`:

```go
package ingestion

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

const sampleAtomFeed = `<?xml version="1.0" encoding="UTF-8"?>
<feed xmlns="http://www.w3.org/2005/Atom">
  <title>Release notes from go-ethereum</title>
  <entry>
    <id>tag:github.com,2008:Repository/15452919/v1.17.0</id>
    <updated>2026-02-17T16:31:00Z</updated>
    <title>Eezo-Inlaid Circuitry (v1.17.0)</title>
    <content type="html">&lt;h2&gt;Changes&lt;/h2&gt;&lt;p&gt;Bug fixes&lt;/p&gt;</content>
    <link rel="alternate" type="text/html" href="https://github.com/ethereum/go-ethereum/releases/tag/v1.17.0"/>
  </entry>
  <entry>
    <id>tag:github.com,2008:Repository/15452919/v1.16.9</id>
    <updated>2026-02-17T16:06:58Z</updated>
    <title>Shield Focusing Module (v1.16.9)</title>
    <content type="html">&lt;p&gt;Patch release&lt;/p&gt;</content>
    <link rel="alternate" type="text/html" href="https://github.com/ethereum/go-ethereum/releases/tag/v1.16.9"/>
  </entry>
</feed>`

func TestGitHubAtomSourceName(t *testing.T) {
	src := NewGitHubAtomSource(http.DefaultClient, "ethereum/go-ethereum", "src-id")
	if got := src.Name(); got != "github" {
		t.Errorf("Name() = %q, want %q", got, "github")
	}
}

func TestGitHubAtomFetchNewReleases(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/atom+xml")
		w.Write([]byte(sampleAtomFeed))
	}))
	defer srv.Close()

	src := NewGitHubAtomSource(srv.Client(), "ethereum/go-ethereum", "src-id")
	src.baseURL = srv.URL

	results, err := src.FetchNewReleases(context.Background())
	if err != nil {
		t.Fatalf("FetchNewReleases: %v", err)
	}

	if len(results) != 2 {
		t.Fatalf("got %d results, want 2", len(results))
	}

	// First entry
	if results[0].RawVersion != "v1.17.0" {
		t.Errorf("results[0].RawVersion = %q, want %q", results[0].RawVersion, "v1.17.0")
	}
	if results[0].Repository != "ethereum/go-ethereum" {
		t.Errorf("results[0].Repository = %q, want %q", results[0].Repository, "ethereum/go-ethereum")
	}
	if results[0].Changelog == "" {
		t.Error("results[0].Changelog should not be empty")
	}
	if results[0].Timestamp.IsZero() {
		t.Error("results[0].Timestamp should not be zero")
	}

	// Second entry
	if results[1].RawVersion != "v1.16.9" {
		t.Errorf("results[1].RawVersion = %q, want %q", results[1].RawVersion, "v1.16.9")
	}
}

func TestGitHubAtomFetchEmpty(t *testing.T) {
	emptyFeed := `<?xml version="1.0" encoding="UTF-8"?>
<feed xmlns="http://www.w3.org/2005/Atom">
  <title>Release notes from new-repo</title>
</feed>`

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/atom+xml")
		w.Write([]byte(emptyFeed))
	}))
	defer srv.Close()

	src := NewGitHubAtomSource(srv.Client(), "org/new-repo", "src-id")
	src.baseURL = srv.URL

	results, err := src.FetchNewReleases(context.Background())
	if err != nil {
		t.Fatalf("FetchNewReleases: %v", err)
	}
	if len(results) != 0 {
		t.Fatalf("got %d results, want 0", len(results))
	}
}

func TestGitHubAtomFetchHTTPError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	src := NewGitHubAtomSource(srv.Client(), "org/missing", "src-id")
	src.baseURL = srv.URL

	_, err := src.FetchNewReleases(context.Background())
	if err == nil {
		t.Fatal("expected error for 404 response")
	}
}
```

**Step 2: Run tests to verify they fail**

Run: `go test -v -run TestGitHubAtom ./internal/ingestion/...`
Expected: Compilation error — `NewGitHubAtomSource` undefined.

**Step 3: Write the implementation**

Create `internal/ingestion/github_atom.go`:

```go
package ingestion

import (
	"context"
	"encoding/xml"
	"fmt"
	"net/http"
	"strings"
	"time"
)

const defaultGitHubURL = "https://github.com"

// atomFeed represents a GitHub releases Atom feed.
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

// GitHubAtomSource polls a GitHub repository's releases Atom feed.
type GitHubAtomSource struct {
	client     *http.Client
	repository string
	baseURL    string
	sourceID   string
}

func NewGitHubAtomSource(client *http.Client, repository string, sourceID string) *GitHubAtomSource {
	return &GitHubAtomSource{
		client:     client,
		repository: repository,
		baseURL:    defaultGitHubURL,
		sourceID:   sourceID,
	}
}

func (s *GitHubAtomSource) Name() string    { return "github" }
func (s *GitHubAtomSource) SourceID() string { return s.sourceID }

func (s *GitHubAtomSource) FetchNewReleases(ctx context.Context) ([]IngestionResult, error) {
	url := fmt.Sprintf("%s/%s/releases.atom", s.baseURL, s.repository)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetch atom feed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status: %d", resp.StatusCode)
	}

	var feed atomFeed
	if err := xml.NewDecoder(resp.Body).Decode(&feed); err != nil {
		return nil, fmt.Errorf("decode atom feed: %w", err)
	}

	results := make([]IngestionResult, 0, len(feed.Entries))
	for _, entry := range feed.Entries {
		version := extractVersion(entry.ID)
		if version == "" {
			continue
		}
		ts, _ := time.Parse(time.RFC3339, entry.Updated)
		results = append(results, IngestionResult{
			Repository: s.repository,
			RawVersion: version,
			Changelog:  entry.Content,
			Timestamp:  ts,
		})
	}
	return results, nil
}

// extractVersion extracts the tag name from an Atom entry ID.
// Format: "tag:github.com,2008:Repository/15452919/v1.17.0" → "v1.17.0"
func extractVersion(id string) string {
	idx := strings.LastIndex(id, "/")
	if idx < 0 || idx == len(id)-1 {
		return ""
	}
	return id[idx+1:]
}
```

**Step 4: Run tests to verify they pass**

Run: `go test -v -run TestGitHubAtom ./internal/ingestion/...`
Expected: All 4 tests PASS.

**Step 5: Commit**

```bash
git add internal/ingestion/github_atom.go internal/ingestion/github_atom_test.go
git commit -m "feat(ingestion): add GitHubAtomSource polling via Atom feed"
```

---

### Task 2: Register GitHubAtomSource in the source loader

**Files:**
- Modify: `internal/ingestion/loader.go:67-74`

**Step 1: Edit the `buildSource` switch**

In `internal/ingestion/loader.go`, add `case "github":` to the switch in `buildSource()`:

```go
func (l *SourceLoader) buildSource(id string, sourceType, repository string) IIngestionSource {
	switch sourceType {
	case "dockerhub":
		return NewDockerHubSource(l.client, repository, id)
	case "github":
		return NewGitHubAtomSource(l.client, repository, id)
	default:
		return nil
	}
}
```

**Step 2: Run all ingestion tests**

Run: `go test ./internal/ingestion/...`
Expected: All tests PASS (including existing DockerHub and webhook tests — webhook tests still pass since the code exists at this point).

**Step 3: Commit**

```bash
git add internal/ingestion/loader.go
git commit -m "feat(ingestion): register github source type in loader"
```

---

### Task 3: Remove GitHub webhook handler and wiring

**Files:**
- Delete: `internal/ingestion/github.go`
- Delete: `internal/ingestion/github_test.go`
- Modify: `cmd/server/main.go:25,86-98,103-104`

**Step 1: Delete the webhook handler files**

```bash
rm internal/ingestion/github.go internal/ingestion/github_test.go
```

**Step 2: Clean up `cmd/server/main.go`**

Remove these pieces from `cmd/server/main.go`:

1. Remove `ghSecret` variable (line 25):
   ```go
   // DELETE this line:
   ghSecret := envOr("GITHUB_WEBHOOK_SECRET", "")
   ```

2. Remove webhook handler creation (lines 86-98):
   ```go
   // DELETE this entire block:
   // GitHub webhook handler
   webhookHandler := ingestion.NewGitHubWebhookHandler(ghSecret, func(results []ingestion.IngestionResult) {
       ...
   })
   ```

3. Remove webhook route registration (lines 103-104):
   ```go
   // DELETE these lines:
   // Register webhook route
   mux.Handle("POST /webhook/github", webhookHandler)
   ```

The result should look like (after the ingestion layer setup):

```go
	// Ingestion layer
	ingestionStore := ingestion.NewPgStore(pool, riverClient)
	svc := ingestion.NewService(ingestionStore)

	loader := ingestion.NewSourceLoader(pool, http.DefaultClient)
	orch := ingestion.NewOrchestrator(svc, loader, 5*time.Minute)

	broadcaster := api.NewBroadcaster()

	mux := http.NewServeMux()

	// Register all API v1 routes
	...
```

**Step 3: Verify build and tests**

Run: `go build ./cmd/server && go test ./...`
Expected: Build succeeds. All tests pass. No references to `GitHubWebhookHandler` remain.

**Step 4: Verify no dangling references**

Run: `grep -r "GitHubWebhook\|GITHUB_WEBHOOK_SECRET\|webhook/github" --include="*.go" .`
Expected: No results (zero matches).

**Step 5: Commit**

```bash
git add -A
git commit -m "refactor(ingestion): remove GitHub webhook handler in favor of Atom feed polling"
```

---

### Task 4: Final verification

**Step 1: Run full test suite**

Run: `go test ./...`
Expected: All tests pass.

**Step 2: Run vet**

Run: `go vet ./...`
Expected: No issues.

**Step 3: Build binary**

Run: `go build -o releaseguard ./cmd/server`
Expected: Build succeeds.

**Step 4: Verify CLAUDE.md env table is still accurate**

Check if `CLAUDE.md` references `GITHUB_WEBHOOK_SECRET`. If so, remove it from the environment variables table.

---
