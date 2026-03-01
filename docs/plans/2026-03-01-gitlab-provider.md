# GitLab Provider Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Add GitLab as a source provider that polls gitlab.com REST API v4 for repository releases.

**Architecture:** Mirrors the existing GitHub provider pattern — new `GitLabSource` struct implementing `IIngestionSource`, registered in `BuildSource()` factory, exposed in providers API and frontend form.

**Tech Stack:** Go (backend source + tests), Next.js/React (frontend form), GitLab REST API v4

---

### Task 1: Write GitLab source tests

**Files:**
- Create: `internal/ingestion/gitlab_test.go`

**Step 1: Write the test file**

```go
package ingestion

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

const sampleGitLabReleases = `[
  {
    "tag_name": "v1.5.0",
    "description": "## What's New\n\nStability improvements",
    "released_at": "2026-02-20T14:00:00Z",
    "upcoming_release": false,
    "_links": {
      "self": "https://gitlab.com/api/v4/projects/inkscape%2Finkscape/releases/v1.5.0"
    }
  },
  {
    "tag_name": "v1.5.0-rc.1",
    "description": "Release candidate",
    "released_at": "2026-02-18T10:00:00Z",
    "upcoming_release": true,
    "_links": {
      "self": "https://gitlab.com/api/v4/projects/inkscape%2Finkscape/releases/v1.5.0-rc.1"
    }
  },
  {
    "tag_name": "",
    "description": "Empty tag",
    "released_at": "2026-02-17T10:00:00Z",
    "upcoming_release": false,
    "_links": {
      "self": "https://gitlab.com/api/v4/projects/inkscape%2Finkscape/releases/"
    }
  },
  {
    "tag_name": "v1.4.0",
    "description": "Patch release",
    "released_at": "2026-02-15T16:06:58Z",
    "upcoming_release": false,
    "_links": {
      "self": "https://gitlab.com/api/v4/projects/inkscape%2Finkscape/releases/v1.4.0"
    }
  }
]`

func TestGitLabSourceName(t *testing.T) {
	src := NewGitLabSource(http.DefaultClient, "inkscape/inkscape", "src-id")
	if got := src.Name(); got != "gitlab" {
		t.Errorf("Name() = %q, want %q", got, "gitlab")
	}
}

func TestGitLabFetchNewReleases(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(sampleGitLabReleases))
	}))
	defer srv.Close()

	src := NewGitLabSource(srv.Client(), "inkscape/inkscape", "src-id")
	src.baseURL = srv.URL

	results, err := src.FetchNewReleases(context.Background())
	if err != nil {
		t.Fatalf("FetchNewReleases: %v", err)
	}

	// Empty tag_name should be excluded, so 3 results
	if len(results) != 3 {
		t.Fatalf("got %d results, want 3", len(results))
	}

	// First entry — stable release
	if results[0].RawVersion != "v1.5.0" {
		t.Errorf("results[0].RawVersion = %q, want %q", results[0].RawVersion, "v1.5.0")
	}
	if results[0].Repository != "inkscape/inkscape" {
		t.Errorf("results[0].Repository = %q, want %q", results[0].Repository, "inkscape/inkscape")
	}
	if results[0].Changelog == "" {
		t.Error("results[0].Changelog should not be empty")
	}
	if results[0].Timestamp.IsZero() {
		t.Error("results[0].Timestamp should not be zero")
	}
	if results[0].Metadata["prerelease"] != "false" {
		t.Errorf("results[0].Metadata[prerelease] = %q, want %q", results[0].Metadata["prerelease"], "false")
	}
	if results[0].Metadata["release_url"] == "" {
		t.Error("results[0].Metadata[release_url] should not be empty")
	}

	// Second entry — upcoming/prerelease
	if results[1].RawVersion != "v1.5.0-rc.1" {
		t.Errorf("results[1].RawVersion = %q, want %q", results[1].RawVersion, "v1.5.0-rc.1")
	}
	if results[1].Metadata["prerelease"] != "true" {
		t.Errorf("results[1].Metadata[prerelease] = %q, want %q", results[1].Metadata["prerelease"], "true")
	}

	// Third entry
	if results[2].RawVersion != "v1.4.0" {
		t.Errorf("results[2].RawVersion = %q, want %q", results[2].RawVersion, "v1.4.0")
	}
}

func TestGitLabFetchEmpty(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte("[]"))
	}))
	defer srv.Close()

	src := NewGitLabSource(srv.Client(), "org/new-repo", "src-id")
	src.baseURL = srv.URL

	results, err := src.FetchNewReleases(context.Background())
	if err != nil {
		t.Fatalf("FetchNewReleases: %v", err)
	}
	if len(results) != 0 {
		t.Fatalf("got %d results, want 0", len(results))
	}
}

func TestGitLabFetchHTTPError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	src := NewGitLabSource(srv.Client(), "org/missing", "src-id")
	src.baseURL = srv.URL

	_, err := src.FetchNewReleases(context.Background())
	if err == nil {
		t.Fatal("expected error for 404 response")
	}
}
```

**Step 2: Run test to verify it fails**

Run: `go test -v -run TestGitLab ./internal/ingestion/...`
Expected: FAIL — `NewGitLabSource` undefined

**Step 3: Commit**

```bash
git add internal/ingestion/gitlab_test.go
git commit -m "test: add GitLab source unit tests"
```

---

### Task 2: Implement GitLab source

**Files:**
- Create: `internal/ingestion/gitlab.go`

**Step 1: Write the implementation**

```go
package ingestion

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"time"
)

const defaultGitLabAPIURL = "https://gitlab.com"

// glRelease represents a single release from the GitLab REST API v4.
type glRelease struct {
	TagName         string  `json:"tag_name"`
	Description     string  `json:"description"`
	ReleasedAt      string  `json:"released_at"`
	UpcomingRelease bool    `json:"upcoming_release"`
	Links           glLinks `json:"_links"`
}

type glLinks struct {
	Self string `json:"self"`
}

// GitLabSource polls the GitLab REST API v4 for project releases.
type GitLabSource struct {
	client     *http.Client
	repository string
	baseURL    string
	sourceID   string
}

func NewGitLabSource(client *http.Client, repository string, sourceID string) *GitLabSource {
	return &GitLabSource{
		client:     client,
		repository: repository,
		baseURL:    defaultGitLabAPIURL,
		sourceID:   sourceID,
	}
}

func (s *GitLabSource) Name() string     { return "gitlab" }
func (s *GitLabSource) SourceID() string { return s.sourceID }

func (s *GitLabSource) FetchNewReleases(ctx context.Context) ([]IngestionResult, error) {
	encoded := url.PathEscape(s.repository)
	apiURL := fmt.Sprintf("%s/api/v4/projects/%s/releases?per_page=20", s.baseURL, encoded)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, apiURL, nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	if token := os.Getenv("GITLAB_TOKEN"); token != "" {
		req.Header.Set("PRIVATE-TOKEN", token)
	}

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetch releases: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status: %d", resp.StatusCode)
	}

	var releases []glRelease
	if err := json.NewDecoder(resp.Body).Decode(&releases); err != nil {
		return nil, fmt.Errorf("decode releases: %w", err)
	}

	results := make([]IngestionResult, 0, len(releases))
	for _, rel := range releases {
		if rel.TagName == "" {
			continue
		}

		ts, _ := time.Parse(time.RFC3339, rel.ReleasedAt)

		prerelease := "false"
		if rel.UpcomingRelease {
			prerelease = "true"
		}

		results = append(results, IngestionResult{
			Repository: s.repository,
			RawVersion: rel.TagName,
			Changelog:  rel.Description,
			Metadata: map[string]string{
				"release_url": rel.Links.Self,
				"prerelease":  prerelease,
			},
			Timestamp: ts,
		})
	}
	return results, nil
}
```

**Step 2: Run tests to verify they pass**

Run: `go test -v -run TestGitLab ./internal/ingestion/...`
Expected: All 4 tests PASS

**Step 3: Commit**

```bash
git add internal/ingestion/gitlab.go
git commit -m "feat: add GitLab source provider"
```

---

### Task 3: Register GitLab in BuildSource and providers API

**Files:**
- Modify: `internal/ingestion/loader.go:73-83` — add `"gitlab"` case
- Modify: `internal/api/providers.go:16-20` — add GitLab entry

**Step 1: Add gitlab case to BuildSource switch**

In `internal/ingestion/loader.go`, add after the `"ecr-public"` case (line 79):

```go
	case "gitlab":
		return NewGitLabSource(client, repository, id)
```

**Step 2: Add gitlab to providers list**

In `internal/api/providers.go`, add after the ecr-public entry (line 19):

```go
		{"id": "gitlab", "name": "GitLab", "type": "polling"},
```

**Step 3: Run all tests**

Run: `go test ./internal/ingestion/... ./internal/api/...`
Expected: PASS

**Step 4: Commit**

```bash
git add internal/ingestion/loader.go internal/api/providers.go
git commit -m "feat: register GitLab provider in loader and API"
```

---

### Task 4: Update frontend source form and validation

**Files:**
- Modify: `web/components/sources/source-form.tsx:100-105` — add GitLab option
- Modify: `web/components/sources/source-form.tsx:148` — show exclude_prereleases for gitlab too
- Modify: `web/lib/format.ts:14-22` — add GitLab URL validation

**Step 1: Add GitLab to provider dropdown**

In `web/components/sources/source-form.tsx`, add after the ECR Public SelectItem (line 103):

```tsx
            <SelectItem value="gitlab">GitLab</SelectItem>
```

**Step 2: Show exclude_prereleases for gitlab**

In `web/components/sources/source-form.tsx`, change the condition on line 148 from:

```tsx
      {provider === "github" && (
```

to:

```tsx
      {(provider === "github" || provider === "gitlab") && (
```

**Step 3: Add GitLab URL validation**

In `web/lib/format.ts`, add after the ecr-public validation block (after line 22):

```typescript
  if (provider === "gitlab" && /^(https?:\/\/)?gitlab\.com\//i.test(trimmed)) {
    return "Use owner/repo format (e.g. inkscape/inkscape), not a full URL";
  }
```

**Step 4: Verify frontend builds**

Run: `cd web && npx next build` (or `npm run build`)
Expected: Build succeeds

**Step 5: Commit**

```bash
git add web/components/sources/source-form.tsx web/lib/format.ts
git commit -m "feat(web): add GitLab to source form and validation"
```

---

### Task 5: Run full test suite and verify

**Step 1: Run all Go tests**

Run: `go test ./...`
Expected: All PASS

**Step 2: Run go vet**

Run: `go vet ./...`
Expected: No issues
