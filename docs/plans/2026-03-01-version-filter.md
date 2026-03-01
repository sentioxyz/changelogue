# Version Filter for Source Releases — Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Add `version_filter_include` and `version_filter_exclude` regex fields to sources, applied at the DB level when listing releases and at the worker level when sending notifications.

**Architecture:** Two nullable TEXT columns on the `sources` table. PostgreSQL `~` / `!~` operators filter releases in SQL queries. The notification worker checks filters before dispatching. Frontend source form gets two new text inputs.

**Tech Stack:** Go, PostgreSQL (regex operators), Next.js/React, Tailwind CSS

---

### Task 1: Add version filter fields to Source model

**Files:**
- Modify: `internal/models/source.go:8-20`

**Step 1: Add fields to Source struct**

Add two new fields after `Config`:

```go
type Source struct {
	ID                   string          `json:"id"`
	ProjectID            string          `json:"project_id"`
	Provider             string          `json:"provider"`
	Repository           string          `json:"repository"`
	PollIntervalSeconds  int             `json:"poll_interval_seconds"`
	Enabled              bool            `json:"enabled"`
	Config               json.RawMessage `json:"config,omitempty"`
	VersionFilterInclude *string         `json:"version_filter_include,omitempty"`
	VersionFilterExclude *string         `json:"version_filter_exclude,omitempty"`
	LastPolledAt         *time.Time      `json:"last_polled_at,omitempty"`
	LastError            *string         `json:"last_error,omitempty"`
	CreatedAt            time.Time       `json:"created_at"`
	UpdatedAt            time.Time       `json:"updated_at"`
}
```

**Step 2: Run vet**

Run: `go vet ./internal/models/...`
Expected: PASS

**Step 3: Commit**

```bash
git add internal/models/source.go
git commit -m "feat: add version filter fields to Source model"
```

---

### Task 2: Add database migration

**Files:**
- Modify: `internal/db/migrations.go:160` (after schema const, before `RunMigrations`)

**Step 1: Add ALTER TABLE statements**

At the end of `RunMigrations`, after the subscription type migration block (line 191), add:

```go
if _, err := pool.Exec(ctx, `
    ALTER TABLE sources ADD COLUMN IF NOT EXISTS version_filter_include TEXT;
    ALTER TABLE sources ADD COLUMN IF NOT EXISTS version_filter_exclude TEXT;
`); err != nil {
    return fmt.Errorf("source version filter migration: %w", err)
}
```

**Step 2: Run vet**

Run: `go vet ./internal/db/...`
Expected: PASS

**Step 3: Commit**

```bash
git add internal/db/migrations.go
git commit -m "feat: add version_filter_include/exclude columns to sources table"
```

---

### Task 3: Update pgstore Source CRUD to read/write filter fields

**Files:**
- Modify: `internal/api/pgstore.go:122-194` (SourcesStore section)

**Step 1: Update `ListSourcesByProject` query (line 131-134)**

Change the SELECT to include the new columns and update the Scan:

```go
rows, err := s.pool.Query(ctx,
    `SELECT id, project_id, provider, repository, poll_interval_seconds, enabled,
            COALESCE(config,'{}'), version_filter_include, version_filter_exclude,
            last_polled_at, last_error, created_at, updated_at
     FROM sources WHERE project_id = $1 ORDER BY created_at DESC LIMIT $2 OFFSET $3`, projectID, perPage, offset)
```

Update the Scan (line 142-144):
```go
if err := rows.Scan(&src.ID, &src.ProjectID, &src.Provider, &src.Repository,
    &src.PollIntervalSeconds, &src.Enabled, &src.Config,
    &src.VersionFilterInclude, &src.VersionFilterExclude,
    &src.LastPolledAt, &src.LastError, &src.CreatedAt, &src.UpdatedAt); err != nil {
```

**Step 2: Update `CreateSource` (line 152-158)**

```go
func (s *PgStore) CreateSource(ctx context.Context, src *models.Source) error {
	return s.pool.QueryRow(ctx,
		`INSERT INTO sources (project_id, provider, repository, poll_interval_seconds, enabled, config, version_filter_include, version_filter_exclude)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		 RETURNING id, enabled, created_at, updated_at`,
		src.ProjectID, src.Provider, src.Repository, src.PollIntervalSeconds, src.Enabled, src.Config,
		src.VersionFilterInclude, src.VersionFilterExclude,
	).Scan(&src.ID, &src.Enabled, &src.CreatedAt, &src.UpdatedAt)
}
```

**Step 3: Update `GetSource` (line 161-174)**

```go
func (s *PgStore) GetSource(ctx context.Context, id string) (*models.Source, error) {
	var src models.Source
	err := s.pool.QueryRow(ctx,
		`SELECT id, project_id, provider, repository, poll_interval_seconds, enabled,
		        COALESCE(config,'{}'), version_filter_include, version_filter_exclude,
		        last_polled_at, last_error, created_at, updated_at
		 FROM sources WHERE id = $1`, id,
	).Scan(&src.ID, &src.ProjectID, &src.Provider, &src.Repository,
		&src.PollIntervalSeconds, &src.Enabled, &src.Config,
		&src.VersionFilterInclude, &src.VersionFilterExclude,
		&src.LastPolledAt, &src.LastError, &src.CreatedAt, &src.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return &src, nil
}
```

**Step 4: Update `UpdateSource` (line 176-183)**

```go
func (s *PgStore) UpdateSource(ctx context.Context, id string, src *models.Source) error {
	return s.pool.QueryRow(ctx,
		`UPDATE sources SET provider=$1, repository=$2, poll_interval_seconds=$3, enabled=$4,
		        config=$5, version_filter_include=$6, version_filter_exclude=$7, updated_at=NOW()
		 WHERE id=$8 RETURNING updated_at`,
		src.Provider, src.Repository, src.PollIntervalSeconds, src.Enabled, src.Config,
		src.VersionFilterInclude, src.VersionFilterExclude, id,
	).Scan(&src.UpdatedAt)
}
```

**Step 5: Run tests**

Run: `go test ./internal/api/... -run TestSourcesHandler -v`
Expected: PASS (existing tests should still pass — mock store doesn't care about new fields)

**Step 6: Commit**

```bash
git add internal/api/pgstore.go
git commit -m "feat: read/write version filter fields in source CRUD queries"
```

---

### Task 4: Add version filter to release listing queries

**Files:**
- Modify: `internal/api/pgstore.go:198-288` (ReleasesStore section)

**Step 1: Update `ListAllReleases` (line 198-226)**

The query already JOINs sources. Add conditional WHERE clauses:

```go
func (s *PgStore) ListAllReleases(ctx context.Context, page, perPage int) ([]models.Release, int, error) {
	var total int
	err := s.pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM releases r
		 LEFT JOIN sources s ON r.source_id = s.id
		 WHERE (s.version_filter_include IS NULL OR r.version ~ s.version_filter_include)
		   AND (s.version_filter_exclude IS NULL OR r.version !~ s.version_filter_exclude)`).Scan(&total)
	if err != nil {
		return nil, 0, fmt.Errorf("count releases: %w", err)
	}
	offset := (page - 1) * perPage
	rows, err := s.pool.Query(ctx,
		`SELECT r.id, r.source_id, r.version, COALESCE(r.raw_data,'{}'), r.released_at, r.created_at,
		        COALESCE(p.id,''), COALESCE(p.name,''), COALESCE(s.provider,''), COALESCE(s.repository,'')
		 FROM releases r
		 LEFT JOIN sources s ON r.source_id = s.id
		 LEFT JOIN projects p ON s.project_id = p.id
		 WHERE (s.version_filter_include IS NULL OR r.version ~ s.version_filter_include)
		   AND (s.version_filter_exclude IS NULL OR r.version !~ s.version_filter_exclude)
		 ORDER BY COALESCE(r.released_at, r.created_at) DESC LIMIT $1 OFFSET $2`, perPage, offset)
	if err != nil {
		return nil, 0, fmt.Errorf("list all releases: %w", err)
	}
	defer rows.Close()
	var releases []models.Release
	for rows.Next() {
		var rel models.Release
		if err := rows.Scan(&rel.ID, &rel.SourceID, &rel.Version, &rel.RawData, &rel.ReleasedAt, &rel.CreatedAt,
			&rel.ProjectID, &rel.ProjectName, &rel.Provider, &rel.Repository); err != nil {
			return nil, 0, fmt.Errorf("scan release: %w", err)
		}
		releases = append(releases, rel)
	}
	return releases, total, nil
}
```

**Step 2: Update `ListReleasesBySource` (line 228-256)**

```go
func (s *PgStore) ListReleasesBySource(ctx context.Context, sourceID string, page, perPage int) ([]models.Release, int, error) {
	var total int
	err := s.pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM releases r
		 JOIN sources s ON r.source_id = s.id
		 WHERE r.source_id = $1
		   AND (s.version_filter_include IS NULL OR r.version ~ s.version_filter_include)
		   AND (s.version_filter_exclude IS NULL OR r.version !~ s.version_filter_exclude)`, sourceID).Scan(&total)
	if err != nil {
		return nil, 0, fmt.Errorf("count releases: %w", err)
	}
	offset := (page - 1) * perPage
	rows, err := s.pool.Query(ctx,
		`SELECT r.id, r.source_id, r.version, COALESCE(r.raw_data,'{}'), r.released_at, r.created_at,
		        COALESCE(p.id,''), COALESCE(p.name,''), COALESCE(s.provider,''), COALESCE(s.repository,'')
		 FROM releases r
		 LEFT JOIN sources s ON r.source_id = s.id
		 LEFT JOIN projects p ON s.project_id = p.id
		 WHERE r.source_id = $1
		   AND (s.version_filter_include IS NULL OR r.version ~ s.version_filter_include)
		   AND (s.version_filter_exclude IS NULL OR r.version !~ s.version_filter_exclude)
		 ORDER BY COALESCE(r.released_at, r.created_at) DESC LIMIT $2 OFFSET $3`, sourceID, perPage, offset)
	if err != nil {
		return nil, 0, fmt.Errorf("list releases by source: %w", err)
	}
	defer rows.Close()
	var releases []models.Release
	for rows.Next() {
		var rel models.Release
		if err := rows.Scan(&rel.ID, &rel.SourceID, &rel.Version, &rel.RawData, &rel.ReleasedAt, &rel.CreatedAt,
			&rel.ProjectID, &rel.ProjectName, &rel.Provider, &rel.Repository); err != nil {
			return nil, 0, fmt.Errorf("scan release: %w", err)
		}
		releases = append(releases, rel)
	}
	return releases, total, nil
}
```

**Step 3: Update `ListReleasesByProject` (line 258-288)**

```go
func (s *PgStore) ListReleasesByProject(ctx context.Context, projectID string, page, perPage int) ([]models.Release, int, error) {
	var total int
	err := s.pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM releases r
		 JOIN sources s ON r.source_id = s.id
		 WHERE s.project_id = $1
		   AND (s.version_filter_include IS NULL OR r.version ~ s.version_filter_include)
		   AND (s.version_filter_exclude IS NULL OR r.version !~ s.version_filter_exclude)`,
		projectID).Scan(&total)
	if err != nil {
		return nil, 0, fmt.Errorf("count releases: %w", err)
	}
	offset := (page - 1) * perPage
	rows, err := s.pool.Query(ctx,
		`SELECT r.id, r.source_id, r.version, COALESCE(r.raw_data,'{}'), r.released_at, r.created_at,
		        p.id, p.name, s.provider, s.repository
		 FROM releases r
		 JOIN sources s ON r.source_id = s.id
		 JOIN projects p ON s.project_id = p.id
		 WHERE s.project_id = $1
		   AND (s.version_filter_include IS NULL OR r.version ~ s.version_filter_include)
		   AND (s.version_filter_exclude IS NULL OR r.version !~ s.version_filter_exclude)
		 ORDER BY COALESCE(r.released_at, r.created_at) DESC LIMIT $2 OFFSET $3`, projectID, perPage, offset)
	if err != nil {
		return nil, 0, fmt.Errorf("list releases by project: %w", err)
	}
	defer rows.Close()
	var releases []models.Release
	for rows.Next() {
		var rel models.Release
		if err := rows.Scan(&rel.ID, &rel.SourceID, &rel.Version, &rel.RawData, &rel.ReleasedAt, &rel.CreatedAt,
			&rel.ProjectID, &rel.ProjectName, &rel.Provider, &rel.Repository); err != nil {
			return nil, 0, fmt.Errorf("scan release: %w", err)
		}
		releases = append(releases, rel)
	}
	return releases, total, nil
}
```

**Step 4: Run vet**

Run: `go vet ./internal/api/...`
Expected: PASS

**Step 5: Commit**

```bash
git add internal/api/pgstore.go
git commit -m "feat: apply source version filters when listing releases"
```

---

### Task 5: Add version filter check to notification worker

**Files:**
- Modify: `internal/routing/worker.go:58-97`
- Test: `internal/routing/worker_test.go`

**Step 1: Write failing tests for version filter in worker**

Add to `internal/routing/worker_test.go`:

```go
func TestNotifyWorker_VersionFilterExclude(t *testing.T) {
	sourceID := "src-1"
	releaseID := "rel-1"
	channelID := "ch-1"
	exclude := "-(alpha|beta|rc)"

	store := &mockNotifyStore{
		releases: map[string]*models.Release{
			releaseID: {ID: releaseID, SourceID: sourceID, Version: "v1.0.0-beta", RawData: json.RawMessage(`{}`)},
		},
		sources: map[string]*models.Source{
			sourceID: {ID: sourceID, ProjectID: "proj-1", VersionFilterExclude: &exclude},
		},
		subscriptions: map[string][]models.Subscription{
			sourceID: {{ID: "sub-1", ChannelID: channelID, Type: "source_release", SourceID: &sourceID}},
		},
		channels: map[string]*models.NotificationChannel{
			channelID: {ID: channelID, Name: "test", Type: "webhook", Config: json.RawMessage(`{"url":"http://example.com"}`)},
		},
		projects: map[string]*models.Project{
			"proj-1": {ID: "proj-1", Name: "test", AgentRules: json.RawMessage(`{}`)},
		},
	}

	webhookSender := &mockSender{}
	worker := &NotifyWorker{store: store, senders: map[string]Sender{"webhook": webhookSender}}
	job := &river.Job[queue.NotifyJobArgs]{Args: queue.NotifyJobArgs{ReleaseID: releaseID, SourceID: sourceID}}

	err := worker.Work(context.Background(), job)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if webhookSender.sentCount() != 0 {
		t.Fatalf("expected 0 notifications (excluded by filter), got %d", webhookSender.sentCount())
	}
	if store.agentRunCallCount() != 0 {
		t.Fatalf("expected 0 agent runs (excluded by filter), got %d", store.agentRunCallCount())
	}
}

func TestNotifyWorker_VersionFilterInclude(t *testing.T) {
	sourceID := "src-1"
	releaseID := "rel-1"
	channelID := "ch-1"
	include := `^v\d+\.\d+\.\d+$`

	store := &mockNotifyStore{
		releases: map[string]*models.Release{
			releaseID: {ID: releaseID, SourceID: sourceID, Version: "nightly-20260301", RawData: json.RawMessage(`{}`)},
		},
		sources: map[string]*models.Source{
			sourceID: {ID: sourceID, ProjectID: "proj-1", VersionFilterInclude: &include},
		},
		subscriptions: map[string][]models.Subscription{
			sourceID: {{ID: "sub-1", ChannelID: channelID, Type: "source_release", SourceID: &sourceID}},
		},
		channels: map[string]*models.NotificationChannel{
			channelID: {ID: channelID, Name: "test", Type: "webhook", Config: json.RawMessage(`{"url":"http://example.com"}`)},
		},
		projects: map[string]*models.Project{
			"proj-1": {ID: "proj-1", Name: "test", AgentRules: json.RawMessage(`{}`)},
		},
	}

	webhookSender := &mockSender{}
	worker := &NotifyWorker{store: store, senders: map[string]Sender{"webhook": webhookSender}}
	job := &river.Job[queue.NotifyJobArgs]{Args: queue.NotifyJobArgs{ReleaseID: releaseID, SourceID: sourceID}}

	err := worker.Work(context.Background(), job)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if webhookSender.sentCount() != 0 {
		t.Fatalf("expected 0 notifications (not matching include), got %d", webhookSender.sentCount())
	}
}

func TestNotifyWorker_VersionFilterPassesThrough(t *testing.T) {
	sourceID := "src-1"
	releaseID := "rel-1"
	channelID := "ch-1"
	include := `^v\d+\.\d+\.\d+$`
	exclude := "-beta"

	store := &mockNotifyStore{
		releases: map[string]*models.Release{
			releaseID: {ID: releaseID, SourceID: sourceID, Version: "v2.0.0", RawData: json.RawMessage(`{}`)},
		},
		sources: map[string]*models.Source{
			sourceID: {ID: sourceID, ProjectID: "proj-1", VersionFilterInclude: &include, VersionFilterExclude: &exclude},
		},
		subscriptions: map[string][]models.Subscription{
			sourceID: {{ID: "sub-1", ChannelID: channelID, Type: "source_release", SourceID: &sourceID}},
		},
		channels: map[string]*models.NotificationChannel{
			channelID: {ID: channelID, Name: "test", Type: "webhook", Config: json.RawMessage(`{"url":"http://example.com"}`)},
		},
		projects: map[string]*models.Project{
			"proj-1": {ID: "proj-1", Name: "test", AgentRules: json.RawMessage(`{}`)},
		},
	}

	webhookSender := &mockSender{}
	worker := &NotifyWorker{store: store, senders: map[string]Sender{"webhook": webhookSender}}
	job := &river.Job[queue.NotifyJobArgs]{Args: queue.NotifyJobArgs{ReleaseID: releaseID, SourceID: sourceID}}

	err := worker.Work(context.Background(), job)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if webhookSender.sentCount() != 1 {
		t.Fatalf("expected 1 notification (passes all filters), got %d", webhookSender.sentCount())
	}
}
```

**Step 2: Run tests to verify they fail**

Run: `go test ./internal/routing/... -run TestNotifyWorker_VersionFilter -v`
Expected: FAIL (filter logic not implemented yet)

**Step 3: Add `VersionPassesFilter` helper and update `Work` method**

In `internal/routing/worker.go`, add a helper function and modify the `Work` method to fetch the source early and check filters:

```go
// VersionPassesFilter returns true if the version string passes the source's
// include/exclude regex filters. When both are nil the version always passes.
func VersionPassesFilter(version string, include, exclude *string) bool {
	if include != nil && *include != "" {
		matched, err := regexp.MatchString(*include, version)
		if err != nil || !matched {
			return false
		}
	}
	if exclude != nil && *exclude != "" {
		matched, err := regexp.MatchString(*exclude, version)
		if err != nil || matched {
			return false
		}
	}
	return true
}
```

Update the `Work` method — add the source fetch and filter check right after fetching the release (before listing subscriptions):

```go
func (w *NotifyWorker) Work(ctx context.Context, job *river.Job[queue.NotifyJobArgs]) error {
	release, err := w.store.GetRelease(ctx, job.Args.ReleaseID)
	if err != nil {
		return fmt.Errorf("get release: %w", err)
	}

	// Check source version filters — skip entirely if filtered out.
	source, err := w.store.GetSource(ctx, job.Args.SourceID)
	if err != nil {
		return fmt.Errorf("get source: %w", err)
	}
	if !VersionPassesFilter(release.Version, source.VersionFilterInclude, source.VersionFilterExclude) {
		slog.Debug("release filtered by version filter", "version", release.Version, "source_id", job.Args.SourceID)
		return nil
	}

	subs, err := w.store.ListSourceSubscriptions(ctx, job.Args.SourceID)
	// ... rest of existing code ...
```

Also update `checkAgentRules` to reuse the already-fetched source (pass it as a parameter instead of refetching):

```go
func (w *NotifyWorker) checkAgentRules(ctx context.Context, release *models.Release, source *models.Source) {
	project, err := w.store.GetProject(ctx, source.ProjectID)
	// ... rest unchanged, just remove the GetSource call at the top ...
```

Update the call in `Work`:
```go
w.checkAgentRules(ctx, release, source)
```

Don't forget to add `"regexp"` to the import list.

**Step 4: Run tests to verify they pass**

Run: `go test ./internal/routing/... -v`
Expected: ALL PASS (both new and existing tests)

**Step 5: Commit**

```bash
git add internal/routing/worker.go internal/routing/worker_test.go
git commit -m "feat: apply source version filters in notification worker"
```

---

### Task 6: Update frontend TypeScript types

**Files:**
- Modify: `web/lib/api/types.ts:48-68`

**Step 1: Add fields to Source and SourceInput interfaces**

In `Source` interface (after `config`):
```typescript
export interface Source {
  id: string;
  project_id: string;
  provider: string;
  repository: string;
  poll_interval_seconds: number;
  enabled: boolean;
  config?: Record<string, unknown>;
  version_filter_include?: string;
  version_filter_exclude?: string;
  last_polled_at?: string;
  last_error?: string;
  created_at: string;
  updated_at: string;
}

export interface SourceInput {
  provider: string;
  repository: string;
  poll_interval_seconds: number;
  enabled: boolean;
  config?: Record<string, unknown>;
  version_filter_include?: string;
  version_filter_exclude?: string;
}
```

**Step 2: Commit**

```bash
git add web/lib/api/types.ts
git commit -m "feat: add version filter fields to Source TypeScript types"
```

---

### Task 7: Update source form with version filter inputs

**Files:**
- Modify: `web/components/sources/source-form.tsx`

**Step 1: Add state variables**

After existing state variables (line 36), add:

```tsx
const [versionFilterInclude, setVersionFilterInclude] = useState(initial?.version_filter_include ?? "");
const [versionFilterExclude, setVersionFilterExclude] = useState(initial?.version_filter_exclude ?? "");
```

**Step 2: Include in submit payload**

In `handleSubmit`, update the `onSubmit` call (line 60-66) to include the new fields:

```tsx
await onSubmit({
  provider,
  repository: repository.trim(),
  poll_interval_seconds: Number(pollInterval),
  enabled,
  config: parsedConfig,
  version_filter_include: versionFilterInclude.trim() || undefined,
  version_filter_exclude: versionFilterExclude.trim() || undefined,
});
```

**Step 3: Add form fields**

After the Config textarea section (after line 117 `</div>`), add:

```tsx
<div className="space-y-2">
  <Label htmlFor="version_filter_include">Version Filter — Include (regex, optional)</Label>
  <Input
    id="version_filter_include"
    value={versionFilterInclude}
    onChange={(e) => setVersionFilterInclude(e.target.value)}
    placeholder='e.g. ^v\d+\.\d+\.\d+$'
    className="font-mono text-sm"
  />
  <p className="text-xs text-muted-foreground">Only show/notify versions matching this pattern</p>
</div>
<div className="space-y-2">
  <Label htmlFor="version_filter_exclude">Version Filter — Exclude (regex, optional)</Label>
  <Input
    id="version_filter_exclude"
    value={versionFilterExclude}
    onChange={(e) => setVersionFilterExclude(e.target.value)}
    placeholder='e.g. -(alpha|beta|rc|nightly)'
    className="font-mono text-sm"
  />
  <p className="text-xs text-muted-foreground">Hide/suppress versions matching this pattern</p>
</div>
```

**Step 4: Build frontend to verify**

Run: `cd web && npx next build` (or `npx next lint`)
Expected: PASS

**Step 5: Commit**

```bash
git add web/components/sources/source-form.tsx
git commit -m "feat: add version filter inputs to source form"
```

---

### Task 8: Update API documentation

**Files:**
- Modify: `API.md`

**Step 1: Update source endpoints docs**

Add `version_filter_include` and `version_filter_exclude` to the source create/update request body docs and the response schema.

**Step 2: Commit**

```bash
git add API.md
git commit -m "docs: add version filter fields to API documentation"
```

---

### Task 9: Final verification

**Step 1: Run all Go tests**

Run: `go test ./...`
Expected: ALL PASS

**Step 2: Run Go vet**

Run: `go vet ./...`
Expected: PASS

**Step 3: Build backend**

Run: `go build -o changelogue ./cmd/server`
Expected: PASS

**Step 4: Build frontend**

Run: `cd web && npx next build`
Expected: PASS
