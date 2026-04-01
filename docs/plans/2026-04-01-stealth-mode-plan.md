# Stealth Mode Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add a headless, agent-native stealth mode binary that runs as a local system service with SQLite storage and shell command callbacks.

**Architecture:** Separate binary (`cmd/stealth/`) reusing existing ingestion sources, models, API router, and notification senders. New SQLite-backed store implements the same interfaces as `PgStore`. Synchronous in-process notification dispatch replaces River async queue. New `ShellSender` enables per-subscription shell command callbacks.

**Tech Stack:** Go 1.25, `modernc.org/sqlite` (pure Go SQLite), existing `internal/api`, `internal/ingestion`, `internal/routing`, `internal/models` packages.

---

### Task 1: Add `Config` Field to Subscription Model

**Files:**
- Modify: `internal/models/subscription.go`
- Modify: `internal/db/migrations.go`
- Modify: `internal/api/pgstore.go` (subscription CRUD — scan/insert the new column)
- Test: `go test ./internal/api/... ./internal/models/...`

- [ ] **Step 1: Add Config field to Subscription struct**

In `internal/models/subscription.go`, add the `Config` field:

```go
package models

import (
	"encoding/json"
	"time"
)

type Subscription struct {
	ID            string          `json:"id"`
	ChannelID     string          `json:"channel_id"`
	Type          string          `json:"type"`
	SourceID      *string         `json:"source_id,omitempty"`
	ProjectID     *string         `json:"project_id,omitempty"`
	VersionFilter string          `json:"version_filter,omitempty"`
	Config        json.RawMessage `json:"config,omitempty"`
	CreatedAt     time.Time       `json:"created_at"`
}
```

- [ ] **Step 2: Add migration for config column**

In `internal/db/migrations.go`, add the `ALTER TABLE` statement inside `RunMigrations` (after the existing `exclude_prereleases` migration):

```go
if _, err := pool.Exec(ctx, `
	ALTER TABLE subscriptions ADD COLUMN IF NOT EXISTS config JSONB;
`); err != nil {
	return fmt.Errorf("subscription config migration: %w", err)
}
```

- [ ] **Step 3: Update PgStore subscription CRUD**

In `internal/api/pgstore.go`, update all subscription methods to include the `config` column:

- `ListSubscriptions`: Add `config` to SELECT and Scan
- `CreateSubscription`: Add `config` to INSERT
- `GetSubscription`: Add `config` to SELECT and Scan
- `UpdateSubscription`: Add `config` to UPDATE
- `CreateSubscriptionBatch`: Add `config` to INSERT
- `ListSourceSubscriptions` (used by NotifyWorker): Add `config` to SELECT and Scan

Search for all SQL queries touching `subscriptions` and add the `config` column. Example for `ListSubscriptions`:

Before:
```sql
SELECT id, channel_id, type, source_id, project_id, version_filter, created_at FROM subscriptions
```
After:
```sql
SELECT id, channel_id, type, source_id, project_id, version_filter, config, created_at FROM subscriptions
```

And update the corresponding `Scan` call to include `&sub.Config`.

For INSERT, add `config` as a parameter:
```sql
INSERT INTO subscriptions (id, channel_id, type, source_id, project_id, version_filter, config) VALUES ($1, $2, $3, $4, $5, $6, $7)
```

- [ ] **Step 4: Run tests**

Run: `go test ./internal/api/... ./internal/models/...`
Expected: All existing tests pass. The new field is optional (`omitempty`), so existing behavior is unchanged.

- [ ] **Step 5: Verify with `go vet`**

Run: `go vet ./...`
Expected: No errors.

- [ ] **Step 6: Commit**

```bash
git add internal/models/subscription.go internal/db/migrations.go internal/api/pgstore.go
git commit -m "feat: add config field to subscription model for per-subscription settings"
```

---

### Task 2: Shell Sender

**Files:**
- Create: `internal/routing/shell_sender.go`
- Create: `internal/routing/shell_sender_test.go`
- Modify: `internal/routing/worker.go` (add "shell" to `NewSenders()`)

- [ ] **Step 1: Write the test**

Create `internal/routing/shell_sender_test.go`:

```go
package routing

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/sentioxyz/changelogue/internal/models"
)

func TestShellSender_Send(t *testing.T) {
	tmpDir := t.TempDir()
	outFile := filepath.Join(tmpDir, "out.txt")

	// Channel config: defaults
	chConfig, _ := json.Marshal(map[string]any{
		"timeout_seconds": 5,
	})

	// Subscription config: command that writes to a file
	subConfig, _ := json.Marshal(map[string]any{
		"command": "echo ${CHANGELOGUE_VERSION} > " + outFile,
	})

	ch := &models.NotificationChannel{
		ID:     "ch1",
		Name:   "test-shell",
		Type:   "shell",
		Config: chConfig,
	}

	msg := Notification{
		Version:    "v1.2.3",
		Repository: "owner/repo",
		Provider:   "github",
	}

	sender := &ShellSender{}
	err := sender.SendWithConfig(context.Background(), ch, msg, subConfig)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Wait for async execution
	time.Sleep(500 * time.Millisecond)

	data, err := os.ReadFile(outFile)
	if err != nil {
		t.Fatalf("output file not created: %v", err)
	}
	got := string(data)
	if got != "v1.2.3\n" {
		t.Errorf("got %q, want %q", got, "v1.2.3\n")
	}
}

func TestShellSender_SendMissingCommand(t *testing.T) {
	chConfig, _ := json.Marshal(map[string]any{})
	subConfig, _ := json.Marshal(map[string]any{}) // no command

	ch := &models.NotificationChannel{
		ID:     "ch1",
		Type:   "shell",
		Config: chConfig,
	}

	sender := &ShellSender{}
	err := sender.SendWithConfig(context.Background(), ch, Notification{}, subConfig)
	if err == nil {
		t.Fatal("expected error for missing command")
	}
}
```

- [ ] **Step 2: Run the test to verify it fails**

Run: `go test -v -run TestShellSender ./internal/routing/...`
Expected: FAIL — `ShellSender` not defined.

- [ ] **Step 3: Implement ShellSender**

Create `internal/routing/shell_sender.go`:

```go
package routing

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/sentioxyz/changelogue/internal/models"
)

type shellChannelConfig struct {
	TimeoutSeconds int    `json:"timeout_seconds"`
	WorkingDir     string `json:"working_dir"`
}

type shellSubscriptionConfig struct {
	Command        string `json:"command"`
	TimeoutSeconds int    `json:"timeout_seconds"`
	WorkingDir     string `json:"working_dir"`
}

// ShellSender executes a shell command when a notification fires.
// The command template comes from the subscription config; the channel
// config provides defaults for timeout and working directory.
type ShellSender struct{}

// Send implements the Sender interface. For shell channels, the subscription
// config is not available via the Sender interface, so this is a no-op that
// logs a warning. Use SendWithConfig for the full implementation.
func (s *ShellSender) Send(ctx context.Context, ch *models.NotificationChannel, msg Notification) error {
	slog.Warn("ShellSender.Send called without subscription config — use SendWithConfig")
	return nil
}

// SendWithConfig executes the shell command from the subscription config with
// environment variable substitution from the notification payload.
func (s *ShellSender) SendWithConfig(ctx context.Context, ch *models.NotificationChannel, msg Notification, subConfig json.RawMessage) error {
	// Parse channel config (defaults)
	var chCfg shellChannelConfig
	if len(ch.Config) > 0 {
		if err := json.Unmarshal(ch.Config, &chCfg); err != nil {
			return fmt.Errorf("parse shell channel config: %w", err)
		}
	}
	if chCfg.TimeoutSeconds == 0 {
		chCfg.TimeoutSeconds = 30
	}

	// Parse subscription config (command + overrides)
	var subCfg shellSubscriptionConfig
	if len(subConfig) > 0 {
		if err := json.Unmarshal(subConfig, &subCfg); err != nil {
			return fmt.Errorf("parse shell subscription config: %w", err)
		}
	}
	if subCfg.Command == "" {
		return fmt.Errorf("shell subscription config: command is required")
	}

	// Merge: subscription overrides channel defaults
	timeout := chCfg.TimeoutSeconds
	if subCfg.TimeoutSeconds > 0 {
		timeout = subCfg.TimeoutSeconds
	}
	workingDir := chCfg.WorkingDir
	if subCfg.WorkingDir != "" {
		workingDir = subCfg.WorkingDir
	}

	// Build environment variables
	env := append(os.Environ(),
		"CHANGELOGUE_VERSION="+msg.Version,
		"CHANGELOGUE_REPOSITORY="+msg.Repository,
		"CHANGELOGUE_PROVIDER="+msg.Provider,
		"CHANGELOGUE_PROJECT="+msg.ProjectName,
		"CHANGELOGUE_RELEASE_ID="+msg.ReleaseURL, // release URL contains the ID
		"CHANGELOGUE_SOURCE_ID="+msg.SourceURL,
		"CHANGELOGUE_RAW_DATA="+msg.Body,
	)

	// Expand ${VAR} placeholders in command
	command := os.Expand(subCfg.Command, func(key string) string {
		for _, e := range env {
			if strings.HasPrefix(e, key+"=") {
				return e[len(key)+1:]
			}
		}
		return ""
	})

	// Execute asynchronously
	go func() {
		execCtx, cancel := context.WithTimeout(context.Background(), time.Duration(timeout)*time.Second)
		defer cancel()

		cmd := exec.CommandContext(execCtx, "sh", "-c", command)
		cmd.Env = env
		if workingDir != "" {
			expanded := workingDir
			if strings.HasPrefix(expanded, "~") {
				home, _ := os.UserHomeDir()
				expanded = home + expanded[1:]
			}
			cmd.Dir = expanded
		}

		output, err := cmd.CombinedOutput()
		if err != nil {
			slog.Error("shell callback failed", "command", command, "err", err, "output", string(output))
		} else {
			slog.Debug("shell callback completed", "command", command, "output", string(output))
		}
	}()

	return nil
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test -v -run TestShellSender ./internal/routing/...`
Expected: PASS

- [ ] **Step 5: Register shell sender in NewSenders**

In `internal/routing/worker.go`, add `"shell"` to the `NewSenders()` map (line 44-51):

```go
func NewSenders() map[string]Sender {
	return map[string]Sender{
		"webhook": &WebhookSender{Client: &http.Client{Timeout: 10 * time.Second}},
		"slack":   &SlackSender{Client: &http.Client{Timeout: 10 * time.Second}},
		"discord": &DiscordSender{Client: &http.Client{Timeout: 10 * time.Second}},
		"email":   &EmailSender{},
		"shell":   &ShellSender{},
	}
}
```

- [ ] **Step 6: Run all routing tests**

Run: `go test ./internal/routing/...`
Expected: All tests pass.

- [ ] **Step 7: Commit**

```bash
git add internal/routing/shell_sender.go internal/routing/shell_sender_test.go internal/routing/worker.go
git commit -m "feat: add ShellSender for shell command callback notifications"
```

---

### Task 3: Refactor Ingestion Orchestrator for Database Abstraction

**Files:**
- Modify: `internal/ingestion/loader.go`
- Modify: `internal/ingestion/orchestrator.go`
- Modify: `internal/ingestion/service.go`
- Modify: `cmd/server/main.go` (update constructor calls)
- Test: `go test ./internal/ingestion/... ./cmd/server/...`

- [ ] **Step 1: Extract SourceLister interface in loader.go**

In `internal/ingestion/loader.go`, add a `SourceLister` interface and refactor `SourceLoader` to accept it:

```go
// SourceLister abstracts the database query for enabled sources so both
// PostgreSQL and SQLite stores can provide sources to the orchestrator.
type SourceLister interface {
	ListEnabledSources(ctx context.Context) ([]EnabledSource, error)
}

// EnabledSource carries the fields the loader needs to construct an IIngestionSource.
type EnabledSource struct {
	ID                  string
	Provider            string
	Repository          string
	PollIntervalSeconds int
	LastPolledAt         *time.Time
}
```

Then refactor `SourceLoader` to accept a `SourceLister` instead of `*pgxpool.Pool`:

```go
type SourceLoader struct {
	lister SourceLister
	client *http.Client
}

func NewSourceLoader(lister SourceLister, client *http.Client) *SourceLoader {
	return &SourceLoader{lister: lister, client: client}
}

func (l *SourceLoader) LoadEnabledSources(ctx context.Context) ([]ScheduledSource, error) {
	enabled, err := l.lister.ListEnabledSources(ctx)
	if err != nil {
		return nil, fmt.Errorf("list enabled sources: %w", err)
	}
	var sources []ScheduledSource
	for _, e := range enabled {
		src := BuildSource(l.client, e.ID, e.Provider, e.Repository)
		if src == nil {
			slog.Warn("unsupported source type, skipping",
				"id", e.ID, "type", e.Provider, "repo", e.Repository)
			continue
		}
		sources = append(sources, ScheduledSource{
			Source:              src,
			PollIntervalSeconds: e.PollIntervalSeconds,
			LastPolledAt:         e.LastPolledAt,
		})
	}
	return sources, nil
}
```

Remove the `LookupSourceID` method (it was using `pool` directly — check if it's used anywhere; if so, move it to the store interface).

- [ ] **Step 2: Extract PollStatusUpdater interface in orchestrator.go**

In `internal/ingestion/orchestrator.go`, replace direct `pool` usage for `updateSourcePollStatus` with an interface:

```go
// PollStatusUpdater persists poll status for a source.
type PollStatusUpdater interface {
	UpdateSourcePollStatus(ctx context.Context, id string, pollErr error) error
}

type Orchestrator struct {
	service       *Service
	loader        *SourceLoader
	pollUpdater   PollStatusUpdater
	staticSources []IIngestionSource
	interval      time.Duration
}

func NewOrchestrator(service *Service, loader *SourceLoader, pollUpdater PollStatusUpdater, interval time.Duration) *Orchestrator {
	return &Orchestrator{service: service, loader: loader, pollUpdater: pollUpdater, interval: interval}
}

func NewOrchestratorWithSources(service *Service, sources []IIngestionSource, interval time.Duration) *Orchestrator {
	return &Orchestrator{service: service, staticSources: sources, interval: interval}
}
```

Update `updateSourcePollStatus` to use the interface:

```go
func (o *Orchestrator) updateSourcePollStatus(ctx context.Context, sourceID string, pollErr error) {
	if o.pollUpdater == nil {
		return
	}
	if err := o.pollUpdater.UpdateSourcePollStatus(ctx, sourceID, pollErr); err != nil {
		slog.Error("update source poll status", "source", sourceID, "err", err)
	}
}
```

- [ ] **Step 3: Add SourceLister implementation to ingestion PgStore**

In `internal/ingestion/pgstore.go`, add the `ListEnabledSources` method:

```go
func (s *PgStore) ListEnabledSources(ctx context.Context) ([]EnabledSource, error) {
	rows, err := s.pool.Query(ctx,
		`SELECT id, provider, repository, poll_interval_seconds, last_polled_at
		 FROM sources WHERE enabled = true`)
	if err != nil {
		return nil, fmt.Errorf("query enabled sources: %w", err)
	}
	defer rows.Close()

	var sources []EnabledSource
	for rows.Next() {
		var e EnabledSource
		if err := rows.Scan(&e.ID, &e.Provider, &e.Repository, &e.PollIntervalSeconds, &e.LastPolledAt); err != nil {
			return nil, fmt.Errorf("scan source row: %w", err)
		}
		sources = append(sources, e)
	}
	return sources, rows.Err()
}
```

- [ ] **Step 4: Make isUniqueViolation also match SQLite errors**

In `internal/ingestion/service.go`, update `isUniqueViolation`:

```go
func isUniqueViolation(err error) bool {
	s := err.Error()
	return strings.Contains(s, "23505") ||
		strings.Contains(s, "unique_violation") ||
		strings.Contains(s, "UNIQUE constraint failed")
}
```

- [ ] **Step 5: Update cmd/server/main.go**

In `cmd/server/main.go`, update the orchestrator construction (around line 184-189):

```go
ingestionStore := ingestion.NewPgStore(pool, riverClient)
svc := ingestion.NewService(ingestionStore)
loader := ingestion.NewSourceLoader(ingestionStore, http.DefaultClient)
orch := ingestion.NewOrchestrator(svc, loader, pgStore, 5*time.Minute)
```

Note: `pgStore` already implements `UpdateSourcePollStatus` via the `api.SourcesStore` interface. Pass `ingestionStore` as the `SourceLister` (it has `ListEnabledSources`). Pass `pgStore` as the `PollStatusUpdater` (check if `api.PgStore` has `UpdateSourcePollStatus` — if so, use it; otherwise add it to `ingestionStore`).

Actually, check `api.SourcesStore` — it has `UpdateSourcePollStatus`. So `pgStore` (which is `*api.PgStore`) implements `PollStatusUpdater`. Use it:

```go
loader := ingestion.NewSourceLoader(ingestionStore, http.DefaultClient)
orch := ingestion.NewOrchestrator(svc, loader, pgStore, 5*time.Minute)
```

- [ ] **Step 6: Run tests**

Run: `go test ./internal/ingestion/... && go vet ./...`
Expected: All tests pass.

- [ ] **Step 7: Commit**

```bash
git add internal/ingestion/loader.go internal/ingestion/orchestrator.go internal/ingestion/service.go internal/ingestion/pgstore.go cmd/server/main.go
git commit -m "refactor: extract SourceLister and PollStatusUpdater interfaces from ingestion orchestrator"
```

---

### Task 4: SQLite Store — Schema and Connection

**Files:**
- Create: `internal/stealth/store.go`
- Create: `internal/stealth/migrations.go`
- Create: `internal/stealth/store_test.go`

- [ ] **Step 1: Write the test**

Create `internal/stealth/store_test.go`:

```go
package stealth

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func testStore(t *testing.T) *Store {
	t.Helper()
	dbPath := filepath.Join(t.TempDir(), "test.db")
	store, err := NewStore(dbPath)
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}
	t.Cleanup(func() { store.Close() })
	return store
}

func TestNewStore(t *testing.T) {
	store := testStore(t)
	if err := store.PingDB(context.Background()); err != nil {
		t.Fatalf("PingDB: %v", err)
	}
}

func TestNewStore_CreatesDirectory(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "sub", "dir")
	dbPath := filepath.Join(dir, "test.db")
	store, err := NewStore(dbPath)
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}
	defer store.Close()
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		t.Fatal("expected directory to be created")
	}
}
```

- [ ] **Step 2: Run the test to verify it fails**

Run: `go test -v -run TestNewStore ./internal/stealth/...`
Expected: FAIL — package does not exist.

- [ ] **Step 3: Create the migrations**

Create `internal/stealth/migrations.go`:

```go
package stealth

const schema = `
CREATE TABLE IF NOT EXISTS projects (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL UNIQUE,
    description TEXT DEFAULT '',
    agent_prompt TEXT DEFAULT '',
    agent_rules TEXT DEFAULT '{}',
    created_at TEXT DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now')),
    updated_at TEXT DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now'))
);

CREATE TABLE IF NOT EXISTS sources (
    id TEXT PRIMARY KEY,
    project_id TEXT NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    provider TEXT NOT NULL,
    repository TEXT NOT NULL,
    poll_interval_seconds INTEGER DEFAULT 86400,
    enabled INTEGER DEFAULT 1,
    config TEXT,
    version_filter_include TEXT,
    version_filter_exclude TEXT,
    exclude_prereleases INTEGER DEFAULT 0,
    last_polled_at TEXT,
    last_error TEXT,
    created_at TEXT DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now')),
    updated_at TEXT DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now')),
    UNIQUE(provider, repository)
);

CREATE TABLE IF NOT EXISTS releases (
    id TEXT PRIMARY KEY,
    source_id TEXT NOT NULL REFERENCES sources(id) ON DELETE CASCADE,
    version TEXT NOT NULL,
    raw_data TEXT,
    released_at TEXT,
    created_at TEXT DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now')),
    UNIQUE(source_id, version)
);

CREATE TABLE IF NOT EXISTS notification_channels (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL,
    type TEXT NOT NULL,
    config TEXT NOT NULL,
    created_at TEXT DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now')),
    updated_at TEXT DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now'))
);

CREATE TABLE IF NOT EXISTS subscriptions (
    id TEXT PRIMARY KEY,
    channel_id TEXT NOT NULL REFERENCES notification_channels(id) ON DELETE CASCADE,
    type TEXT NOT NULL CHECK (type IN ('source_release', 'semantic_release')),
    source_id TEXT REFERENCES sources(id) ON DELETE CASCADE,
    project_id TEXT REFERENCES projects(id) ON DELETE CASCADE,
    version_filter TEXT,
    config TEXT,
    created_at TEXT DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now')),
    CHECK (
        (type = 'source_release'   AND source_id  IS NOT NULL AND project_id IS NULL) OR
        (type = 'semantic_release' AND project_id IS NOT NULL AND source_id  IS NULL)
    )
);

CREATE TABLE IF NOT EXISTS api_keys (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL,
    key_hash TEXT NOT NULL UNIQUE,
    key_prefix TEXT NOT NULL,
    created_at TEXT DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now')),
    last_used_at TEXT
);
`
```

- [ ] **Step 4: Create the store**

Create `internal/stealth/store.go`:

```go
package stealth

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"

	_ "modernc.org/sqlite"
)

// Store is a SQLite-backed store for stealth mode.
type Store struct {
	db *sql.DB
}

// NewStore opens (or creates) the SQLite database at dbPath and runs migrations.
func NewStore(dbPath string) (*Store, error) {
	dir := filepath.Dir(dbPath)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, fmt.Errorf("create db directory: %w", err)
	}

	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}

	// Set pragmas
	for _, pragma := range []string{
		"PRAGMA journal_mode=WAL",
		"PRAGMA busy_timeout=5000",
		"PRAGMA foreign_keys=ON",
	} {
		if _, err := db.Exec(pragma); err != nil {
			db.Close()
			return nil, fmt.Errorf("set %s: %w", pragma, err)
		}
	}

	// Run migrations
	if _, err := db.Exec(schema); err != nil {
		db.Close()
		return nil, fmt.Errorf("run migrations: %w", err)
	}

	return &Store{db: db}, nil
}

// Close closes the database connection.
func (s *Store) Close() error {
	return s.db.Close()
}

// PingDB checks that the database is reachable.
func (s *Store) PingDB(ctx context.Context) error {
	return s.db.PingContext(ctx)
}
```

- [ ] **Step 5: Add `modernc.org/sqlite` dependency**

Run: `cd /Users/pc/web3/ReleaseBeacon && go get modernc.org/sqlite`

- [ ] **Step 6: Run tests**

Run: `go test -v -run TestNewStore ./internal/stealth/...`
Expected: PASS

- [ ] **Step 7: Commit**

```bash
git add internal/stealth/store.go internal/stealth/migrations.go internal/stealth/store_test.go go.mod go.sum
git commit -m "feat(stealth): add SQLite store with schema migrations"
```

---

### Task 5: SQLite Store — Projects CRUD

**Files:**
- Modify: `internal/stealth/store.go`
- Modify: `internal/stealth/store_test.go`

- [ ] **Step 1: Write the test**

Add to `internal/stealth/store_test.go`:

```go
func TestProjects_CRUD(t *testing.T) {
	ctx := context.Background()
	store := testStore(t)

	// Create
	p := &models.Project{Name: "test-project", Description: "desc"}
	if err := store.CreateProject(ctx, p); err != nil {
		t.Fatalf("CreateProject: %v", err)
	}
	if p.ID == "" {
		t.Fatal("expected ID to be set")
	}

	// Get
	got, err := store.GetProject(ctx, p.ID)
	if err != nil {
		t.Fatalf("GetProject: %v", err)
	}
	if got.Name != "test-project" {
		t.Errorf("got name %q, want %q", got.Name, "test-project")
	}

	// List
	projects, total, err := store.ListProjects(ctx, 1, 50)
	if err != nil {
		t.Fatalf("ListProjects: %v", err)
	}
	if total != 1 {
		t.Errorf("got total %d, want 1", total)
	}
	if len(projects) != 1 {
		t.Errorf("got %d projects, want 1", len(projects))
	}

	// Update
	p.Description = "updated"
	if err := store.UpdateProject(ctx, p.ID, p); err != nil {
		t.Fatalf("UpdateProject: %v", err)
	}
	got, _ = store.GetProject(ctx, p.ID)
	if got.Description != "updated" {
		t.Errorf("got description %q, want %q", got.Description, "updated")
	}

	// Delete
	if err := store.DeleteProject(ctx, p.ID); err != nil {
		t.Fatalf("DeleteProject: %v", err)
	}
	_, err = store.GetProject(ctx, p.ID)
	if err == nil {
		t.Fatal("expected error after delete")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test -v -run TestProjects_CRUD ./internal/stealth/...`
Expected: FAIL — methods not defined.

- [ ] **Step 3: Implement Projects CRUD**

Add to `internal/stealth/store.go`:

```go
import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
	"github.com/sentioxyz/changelogue/internal/models"
)

func (s *Store) ListProjects(ctx context.Context, page, perPage int) ([]models.Project, int, error) {
	var total int
	err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM projects`).Scan(&total)
	if err != nil {
		return nil, 0, fmt.Errorf("count projects: %w", err)
	}
	offset := (page - 1) * perPage
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, name, COALESCE(description,''), COALESCE(agent_prompt,''),
		        COALESCE(agent_rules,'{}'), created_at, updated_at
		 FROM projects ORDER BY created_at DESC LIMIT ? OFFSET ?`, perPage, offset)
	if err != nil {
		return nil, 0, fmt.Errorf("list projects: %w", err)
	}
	defer rows.Close()
	var projects []models.Project
	for rows.Next() {
		var p models.Project
		var createdAt, updatedAt string
		if err := rows.Scan(&p.ID, &p.Name, &p.Description, &p.AgentPrompt, &p.AgentRules, &createdAt, &updatedAt); err != nil {
			return nil, 0, fmt.Errorf("scan project: %w", err)
		}
		p.CreatedAt, _ = time.Parse(time.RFC3339Nano, createdAt)
		p.UpdatedAt, _ = time.Parse(time.RFC3339Nano, updatedAt)
		projects = append(projects, p)
	}
	return projects, total, nil
}

func (s *Store) CreateProject(ctx context.Context, p *models.Project) error {
	if p.ID == "" {
		p.ID = uuid.New().String()
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	if p.AgentRules == nil {
		p.AgentRules = json.RawMessage("{}")
	}
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO projects (id, name, description, agent_prompt, agent_rules, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?)`,
		p.ID, p.Name, p.Description, p.AgentPrompt, string(p.AgentRules), now, now)
	if err != nil {
		return fmt.Errorf("insert project: %w", err)
	}
	p.CreatedAt, _ = time.Parse(time.RFC3339Nano, now)
	p.UpdatedAt = p.CreatedAt
	return nil
}

func (s *Store) GetProject(ctx context.Context, id string) (*models.Project, error) {
	var p models.Project
	var createdAt, updatedAt string
	err := s.db.QueryRowContext(ctx,
		`SELECT id, name, COALESCE(description,''), COALESCE(agent_prompt,''),
		        COALESCE(agent_rules,'{}'), created_at, updated_at
		 FROM projects WHERE id = ?`, id).Scan(
		&p.ID, &p.Name, &p.Description, &p.AgentPrompt, &p.AgentRules, &createdAt, &updatedAt)
	if err != nil {
		return nil, fmt.Errorf("get project: %w", err)
	}
	p.CreatedAt, _ = time.Parse(time.RFC3339Nano, createdAt)
	p.UpdatedAt, _ = time.Parse(time.RFC3339Nano, updatedAt)
	return &p, nil
}

func (s *Store) UpdateProject(ctx context.Context, id string, p *models.Project) error {
	now := time.Now().UTC().Format(time.RFC3339Nano)
	if p.AgentRules == nil {
		p.AgentRules = json.RawMessage("{}")
	}
	res, err := s.db.ExecContext(ctx,
		`UPDATE projects SET name = ?, description = ?, agent_prompt = ?, agent_rules = ?, updated_at = ?
		 WHERE id = ?`,
		p.Name, p.Description, p.AgentPrompt, string(p.AgentRules), now, id)
	if err != nil {
		return fmt.Errorf("update project: %w", err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return fmt.Errorf("project not found")
	}
	return nil
}

func (s *Store) DeleteProject(ctx context.Context, id string) error {
	res, err := s.db.ExecContext(ctx, `DELETE FROM projects WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("delete project: %w", err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return fmt.Errorf("project not found")
	}
	return nil
}
```

- [ ] **Step 4: Run tests**

Run: `go test -v -run TestProjects_CRUD ./internal/stealth/...`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/stealth/store.go internal/stealth/store_test.go
git commit -m "feat(stealth): implement Projects CRUD on SQLite store"
```

---

### Task 6: SQLite Store — Sources CRUD + SourceLister + PollStatusUpdater

**Files:**
- Modify: `internal/stealth/store.go`
- Modify: `internal/stealth/store_test.go`

- [ ] **Step 1: Write the test**

Add to `internal/stealth/store_test.go`:

```go
func TestSources_CRUD(t *testing.T) {
	ctx := context.Background()
	store := testStore(t)

	// Create project first
	p := &models.Project{Name: "proj"}
	store.CreateProject(ctx, p)

	// Create source
	src := &models.Source{
		ProjectID:           p.ID,
		Provider:            "github",
		Repository:          "owner/repo",
		PollIntervalSeconds: 300,
		Enabled:             true,
	}
	if err := store.CreateSource(ctx, src); err != nil {
		t.Fatalf("CreateSource: %v", err)
	}
	if src.ID == "" {
		t.Fatal("expected ID to be set")
	}

	// List
	sources, total, err := store.ListSourcesByProject(ctx, p.ID, 1, 50)
	if err != nil {
		t.Fatalf("ListSourcesByProject: %v", err)
	}
	if total != 1 || len(sources) != 1 {
		t.Errorf("got %d/%d, want 1/1", len(sources), total)
	}

	// ListEnabledSources (SourceLister interface)
	enabled, err := store.ListEnabledSources(ctx)
	if err != nil {
		t.Fatalf("ListEnabledSources: %v", err)
	}
	if len(enabled) != 1 {
		t.Errorf("got %d enabled, want 1", len(enabled))
	}

	// UpdateSourcePollStatus (PollStatusUpdater interface)
	if err := store.UpdateSourcePollStatus(ctx, src.ID, nil); err != nil {
		t.Fatalf("UpdateSourcePollStatus: %v", err)
	}

	// Delete
	if err := store.DeleteSource(ctx, src.ID); err != nil {
		t.Fatalf("DeleteSource: %v", err)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test -v -run TestSources_CRUD ./internal/stealth/...`
Expected: FAIL

- [ ] **Step 3: Implement Sources CRUD + ListEnabledSources + UpdateSourcePollStatus**

Add to `internal/stealth/store.go` — implement `ListSourcesByProject`, `CreateSource`, `GetSource`, `UpdateSource`, `DeleteSource`, `UpdateSourcePollStatus`, `ListAllSourceRepos`, and `ListEnabledSources`. Follow the same pattern as Projects CRUD but with the `sources` table columns. Use `?` placeholders, `time.Parse` for timestamps, and `uuid.New().String()` for IDs.

Key signatures:
```go
func (s *Store) ListSourcesByProject(ctx context.Context, projectID string, page, perPage int) ([]models.Source, int, error)
func (s *Store) CreateSource(ctx context.Context, src *models.Source) error
func (s *Store) GetSource(ctx context.Context, id string) (*models.Source, error)
func (s *Store) UpdateSource(ctx context.Context, id string, src *models.Source) error
func (s *Store) DeleteSource(ctx context.Context, id string) error
func (s *Store) UpdateSourcePollStatus(ctx context.Context, id string, pollErr error) error
func (s *Store) ListAllSourceRepos(ctx context.Context) ([]models.SourceRepo, error)
func (s *Store) ListEnabledSources(ctx context.Context) ([]ingestion.EnabledSource, error)
```

- [ ] **Step 4: Run tests**

Run: `go test -v -run TestSources_CRUD ./internal/stealth/...`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/stealth/store.go internal/stealth/store_test.go
git commit -m "feat(stealth): implement Sources CRUD + SourceLister + PollStatusUpdater"
```

---

### Task 7: SQLite Store — Releases, Channels, Subscriptions, KeyStore, HealthChecker

**Files:**
- Modify: `internal/stealth/store.go`
- Modify: `internal/stealth/store_test.go`

- [ ] **Step 1: Write tests for Releases read, Channels CRUD, Subscriptions CRUD, KeyStore, HealthChecker**

Add tests for each resource following the same CRUD pattern as Projects and Sources. Test:
- `ListAllReleases`, `ListReleasesBySource`, `ListReleasesByProject`, `GetRelease`
- `ListChannels`, `CreateChannel`, `GetChannel`, `UpdateChannel`, `DeleteChannel`
- `ListSubscriptions`, `CreateSubscription`, `GetSubscription`, `UpdateSubscription`, `DeleteSubscription`, `CreateSubscriptionBatch`, `DeleteSubscriptionBatch`
- `ValidateKey`, `TouchKeyUsage`
- `GetStats`

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test -v ./internal/stealth/...`
Expected: FAIL

- [ ] **Step 3: Implement all remaining store methods**

Follow the patterns established in Tasks 5 and 6. Key methods:

**Releases (read-only — writes come from IngestRelease):**
```go
func (s *Store) ListAllReleases(ctx context.Context, page, perPage int, includeExcluded bool, filter api.ReleaseFilter) ([]models.Release, int, error)
func (s *Store) ListReleasesBySource(ctx context.Context, sourceID string, page, perPage int, includeExcluded bool, filter api.ReleaseFilter) ([]models.Release, int, error)
func (s *Store) ListReleasesByProject(ctx context.Context, projectID string, page, perPage int, includeExcluded bool, filter api.ReleaseFilter) ([]models.Release, int, error)
func (s *Store) GetRelease(ctx context.Context, id string) (*models.Release, error)
```

**Channels CRUD:**
```go
func (s *Store) ListChannels(ctx context.Context, page, perPage int) ([]models.NotificationChannel, int, error)
func (s *Store) CreateChannel(ctx context.Context, ch *models.NotificationChannel) error
func (s *Store) GetChannel(ctx context.Context, id string) (*models.NotificationChannel, error)
func (s *Store) UpdateChannel(ctx context.Context, id string, ch *models.NotificationChannel) error
func (s *Store) DeleteChannel(ctx context.Context, id string) error
```

**Subscriptions CRUD:**
```go
func (s *Store) ListSubscriptions(ctx context.Context, page, perPage int) ([]models.Subscription, int, error)
func (s *Store) CreateSubscription(ctx context.Context, sub *models.Subscription) error
func (s *Store) CreateSubscriptionBatch(ctx context.Context, subs []models.Subscription) ([]models.Subscription, error)
func (s *Store) GetSubscription(ctx context.Context, id string) (*models.Subscription, error)
func (s *Store) UpdateSubscription(ctx context.Context, id string, sub *models.Subscription) error
func (s *Store) DeleteSubscription(ctx context.Context, id string) error
func (s *Store) DeleteSubscriptionBatch(ctx context.Context, ids []string) error
```

**KeyStore:**
```go
func (s *Store) ValidateKey(ctx context.Context, rawKey string) (bool, error)
func (s *Store) TouchKeyUsage(ctx context.Context, rawKey string)
```

**HealthChecker:**
```go
func (s *Store) GetStats(ctx context.Context) (*api.DashboardStats, error)
func (s *Store) GetTrend(ctx context.Context, granularity string, start, end time.Time) ([]api.TrendBucket, error)
```

- [ ] **Step 4: Run tests**

Run: `go test -v ./internal/stealth/...`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/stealth/store.go internal/stealth/store_test.go
git commit -m "feat(stealth): implement Releases, Channels, Subscriptions, KeyStore, HealthChecker"
```

---

### Task 8: SQLite Store — IngestRelease + NotifyStore

**Files:**
- Modify: `internal/stealth/store.go`
- Create: `internal/stealth/notify.go`
- Modify: `internal/stealth/store_test.go`

- [ ] **Step 1: Write the test**

Add to `internal/stealth/store_test.go`:

```go
func TestIngestRelease(t *testing.T) {
	ctx := context.Background()
	store := testStore(t)

	// Setup: project + source
	p := &models.Project{Name: "proj"}
	store.CreateProject(ctx, p)
	src := &models.Source{
		ProjectID: p.ID, Provider: "github", Repository: "owner/repo",
		PollIntervalSeconds: 300, Enabled: true,
	}
	store.CreateSource(ctx, src)

	// Ingest
	result := &ingestion.IngestionResult{
		Repository: "owner/repo",
		RawVersion: "v1.0.0",
		Timestamp:  time.Now(),
		Metadata:   map[string]string{"tag": "v1.0.0"},
	}
	if err := store.IngestRelease(ctx, src.ID, result); err != nil {
		t.Fatalf("IngestRelease: %v", err)
	}

	// Verify release exists
	releases, total, err := store.ListReleasesBySource(ctx, src.ID, 1, 50, false, api.ReleaseFilter{})
	if err != nil {
		t.Fatalf("ListReleasesBySource: %v", err)
	}
	if total != 1 || len(releases) != 1 {
		t.Fatalf("got %d/%d, want 1/1", len(releases), total)
	}
	if releases[0].Version != "v1.0.0" {
		t.Errorf("got version %q, want %q", releases[0].Version, "v1.0.0")
	}

	// Duplicate should succeed (unique constraint handled)
	if err := store.IngestRelease(ctx, src.ID, result); err != nil {
		// Should be caught by isUniqueViolation in service layer, not here
		// But the store should return the error for the service to handle
		if !strings.Contains(err.Error(), "UNIQUE constraint failed") {
			t.Fatalf("expected unique constraint error, got: %v", err)
		}
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test -v -run TestIngestRelease ./internal/stealth/...`
Expected: FAIL

- [ ] **Step 3: Implement IngestRelease**

Add to `internal/stealth/store.go`:

```go
func (s *Store) IngestRelease(ctx context.Context, sourceID string, result *ingestion.IngestionResult) error {
	raw := make(map[string]string)
	for k, v := range result.Metadata {
		raw[k] = v
	}
	if result.Changelog != "" {
		raw["changelog"] = result.Changelog
	}
	rawData, err := json.Marshal(raw)
	if err != nil {
		return fmt.Errorf("marshal raw_data: %w", err)
	}

	releaseID := uuid.New().String()
	releasedAt := result.Timestamp.UTC().Format(time.RFC3339Nano)
	now := time.Now().UTC().Format(time.RFC3339Nano)

	_, err = s.db.ExecContext(ctx,
		`INSERT INTO releases (id, source_id, version, raw_data, released_at, created_at)
		 VALUES (?, ?, ?, ?, ?, ?)`,
		releaseID, sourceID, result.RawVersion, string(rawData), releasedAt, now)
	if err != nil {
		return fmt.Errorf("insert release: %w", err)
	}

	return nil
}
```

- [ ] **Step 4: Implement NotifyStore methods**

Add to `internal/stealth/store.go` the methods required by `routing.NotifyStore`:

```go
func (s *Store) ListSourceSubscriptions(ctx context.Context, sourceID string) ([]models.Subscription, error)
func (s *Store) GetPreviousRelease(ctx context.Context, sourceID string, beforeVersion string) (*models.Release, error)
func (s *Store) EnqueueAgentRun(ctx context.Context, projectID, trigger, version string) error  // no-op in stealth
func (s *Store) CreateReleaseTodo(ctx context.Context, releaseID string) (string, error)         // no-op in stealth
func (s *Store) HasReleaseGate(ctx context.Context, projectID string) (bool, error)              // always false
```

- [ ] **Step 5: Implement notify dispatcher**

Create `internal/stealth/notify.go`:

```go
package stealth

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

	"github.com/sentioxyz/changelogue/internal/routing"
)

// NotifyRelease dispatches notifications for a newly ingested release.
// It reuses the routing.Sender implementations but runs synchronously
// instead of through a River job queue.
func (s *Store) NotifyRelease(ctx context.Context, releaseID, sourceID string, senders map[string]routing.Sender) {
	release, err := s.GetRelease(ctx, releaseID)
	if err != nil {
		slog.Error("notify: get release", "release_id", releaseID, "err", err)
		return
	}

	source, err := s.GetSource(ctx, sourceID)
	if err != nil {
		slog.Error("notify: get source", "source_id", sourceID, "err", err)
		return
	}

	// Check version filters
	if !routing.VersionPassesFilter(release.Version, source.VersionFilterInclude, source.VersionFilterExclude) {
		slog.Debug("release filtered by version filter", "version", release.Version)
		return
	}

	subs, err := s.ListSourceSubscriptions(ctx, sourceID)
	if err != nil {
		slog.Error("notify: list subscriptions", "source_id", sourceID, "err", err)
		return
	}

	for _, sub := range subs {
		ch, err := s.GetChannel(ctx, sub.ChannelID)
		if err != nil {
			slog.Error("notify: get channel", "channel_id", sub.ChannelID, "err", err)
			continue
		}

		sender, ok := senders[ch.Type]
		if !ok {
			slog.Warn("notify: unknown channel type", "type", ch.Type)
			continue
		}

		msg := routing.Notification{
			Title:      release.Version,
			Body:       string(release.RawData),
			Version:    release.Version,
			Provider:   release.Provider,
			Repository: release.Repository,
			SourceURL:  routing.ProviderURL(release.Provider, release.Repository, release.Version),
		}

		// For shell senders, pass subscription config
		if ch.Type == "shell" {
			if ss, ok := sender.(*routing.ShellSender); ok {
				if err := ss.SendWithConfig(ctx, ch, msg, sub.Config); err != nil {
					slog.Error("notify: shell send failed", "channel", ch.Name, "err", err)
				}
				continue
			}
		}

		if err := sender.Send(ctx, ch, msg); err != nil {
			slog.Error("notify: send failed", "channel", ch.Name, "err", err)
		}
	}
}
```

- [ ] **Step 6: Run tests**

Run: `go test -v ./internal/stealth/...`
Expected: PASS

- [ ] **Step 7: Commit**

```bash
git add internal/stealth/store.go internal/stealth/notify.go internal/stealth/store_test.go
git commit -m "feat(stealth): implement IngestRelease, NotifyStore, and synchronous notification dispatch"
```

---

### Task 9: Stub Implementations for Unsupported Interfaces

**Files:**
- Create: `internal/stealth/stubs.go`

- [ ] **Step 1: Create stubs returning 501**

Create `internal/stealth/stubs.go`:

```go
package stealth

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/sentioxyz/changelogue/internal/api"
	"github.com/sentioxyz/changelogue/internal/auth"
	"github.com/sentioxyz/changelogue/internal/models"
)

var errNotImplemented = fmt.Errorf("not implemented in stealth mode")

// --- SemanticReleasesStore stub ---

type SemanticReleasesStub struct{}

func (SemanticReleasesStub) ListAllSemanticReleases(ctx context.Context, page, perPage int) ([]models.SemanticRelease, int, error) {
	return nil, 0, errNotImplemented
}
func (SemanticReleasesStub) ListSemanticReleases(ctx context.Context, projectID string, page, perPage int) ([]models.SemanticRelease, int, error) {
	return nil, 0, errNotImplemented
}
func (SemanticReleasesStub) GetSemanticRelease(ctx context.Context, id string) (*models.SemanticRelease, error) {
	return nil, errNotImplemented
}
func (SemanticReleasesStub) GetSemanticReleaseSources(ctx context.Context, id string) ([]models.Release, error) {
	return nil, errNotImplemented
}
func (SemanticReleasesStub) DeleteSemanticRelease(ctx context.Context, id string) error {
	return errNotImplemented
}

// --- AgentStore stub ---

type AgentStub struct{}

func (AgentStub) TriggerAgentRun(ctx context.Context, projectID, trigger, version string) (*models.AgentRun, error) {
	return nil, errNotImplemented
}
func (AgentStub) ListAgentRuns(ctx context.Context, projectID string, page, perPage int) ([]models.AgentRun, int, error) {
	return nil, 0, errNotImplemented
}
func (AgentStub) GetAgentRun(ctx context.Context, id string) (*models.AgentRun, error) {
	return nil, errNotImplemented
}

// --- TodosStore stub ---

type TodosStub struct{}

func (TodosStub) ListTodos(ctx context.Context, status string, page, perPage int, aggregated bool, filter api.TodoFilter) ([]models.Todo, int, error) {
	return nil, 0, errNotImplemented
}
func (TodosStub) GetTodo(ctx context.Context, id string) (*models.Todo, error) {
	return nil, errNotImplemented
}
func (TodosStub) AcknowledgeTodo(ctx context.Context, id string, cascade bool) error {
	return errNotImplemented
}
func (TodosStub) ResolveTodo(ctx context.Context, id string, cascade bool) error {
	return errNotImplemented
}
func (TodosStub) ReopenTodo(ctx context.Context, id string) error {
	return errNotImplemented
}

// --- OnboardStore stub ---

type OnboardStub struct{}

func (OnboardStub) CreateOnboardScan(ctx context.Context, repoURL string) (*models.OnboardScan, error) {
	return nil, errNotImplemented
}
func (OnboardStub) GetOnboardScan(ctx context.Context, id string) (*models.OnboardScan, error) {
	return nil, errNotImplemented
}
func (OnboardStub) UpdateOnboardScanStatus(ctx context.Context, id, status string, results json.RawMessage, scanErr string) error {
	return errNotImplemented
}
func (OnboardStub) ActiveScanForRepo(ctx context.Context, repoURL string) (*models.OnboardScan, error) {
	return nil, errNotImplemented
}
func (OnboardStub) ApplyOnboardScan(ctx context.Context, scanID string, selections []api.OnboardSelection) (*api.OnboardApplyResult, error) {
	return nil, errNotImplemented
}

// --- GatesStore stub ---

type GatesStub struct{}

func (GatesStub) GetReleaseGate(ctx context.Context, projectID string) (*models.ReleaseGate, error) {
	return nil, errNotImplemented
}
func (GatesStub) CreateReleaseGate(ctx context.Context, g *models.ReleaseGate) error {
	return errNotImplemented
}
func (GatesStub) UpdateReleaseGate(ctx context.Context, g *models.ReleaseGate) error {
	return errNotImplemented
}
func (GatesStub) DeleteReleaseGate(ctx context.Context, projectID string) error {
	return errNotImplemented
}
func (GatesStub) ListVersionReadiness(ctx context.Context, projectID string, page, perPage int) ([]models.VersionReadiness, int, error) {
	return nil, 0, errNotImplemented
}
func (GatesStub) GetVersionReadinessByVersion(ctx context.Context, projectID, version string) (*models.VersionReadiness, error) {
	return nil, errNotImplemented
}
func (GatesStub) ListGateEvents(ctx context.Context, projectID string, page, perPage int) ([]models.GateEvent, int, error) {
	return nil, 0, errNotImplemented
}
func (GatesStub) ListGateEventsByVersion(ctx context.Context, projectID, version string, page, perPage int) ([]models.GateEvent, int, error) {
	return nil, 0, errNotImplemented
}

// --- ContextSourcesStore stub ---

type ContextSourcesStub struct{}

func (ContextSourcesStub) ListContextSources(ctx context.Context, projectID string, page, perPage int) ([]models.ContextSource, int, error) {
	return nil, 0, errNotImplemented
}
func (ContextSourcesStub) CreateContextSource(ctx context.Context, cs *models.ContextSource) error {
	return errNotImplemented
}
func (ContextSourcesStub) GetContextSource(ctx context.Context, id string) (*models.ContextSource, error) {
	return nil, errNotImplemented
}
func (ContextSourcesStub) UpdateContextSource(ctx context.Context, id string, cs *models.ContextSource) error {
	return errNotImplemented
}
func (ContextSourcesStub) DeleteContextSource(ctx context.Context, id string) error {
	return errNotImplemented
}

// --- SessionValidator stub ---

type SessionValidatorStub struct{}

func (SessionValidatorStub) ValidateSession(ctx context.Context, cookie string) (*auth.User, error) {
	return nil, errNotImplemented
}
```

- [ ] **Step 2: Verify it compiles**

Run: `go build ./internal/stealth/...`
Expected: Compiles with no errors.

- [ ] **Step 3: Commit**

```bash
git add internal/stealth/stubs.go
git commit -m "feat(stealth): add stub implementations for unsupported store interfaces"
```

---

### Task 10: Stealth Binary — Main Entrypoint

**Files:**
- Create: `cmd/stealth/main.go`
- Modify: `Makefile`

- [ ] **Step 1: Create the stealth binary**

Create `cmd/stealth/main.go`:

```go
package main

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"time"

	"github.com/sentioxyz/changelogue/internal/api"
	"github.com/sentioxyz/changelogue/internal/ingestion"
	"github.com/sentioxyz/changelogue/internal/routing"
	"github.com/sentioxyz/changelogue/internal/stealth"
)

func main() {
	if len(os.Args) > 1 {
		switch os.Args[1] {
		case "install":
			install()
			return
		case "uninstall":
			uninstall()
			return
		case "status":
			status()
			return
		case "serve":
			// fall through to server startup
		default:
			fmt.Fprintf(os.Stderr, "Usage: clog-stealth [serve|install|uninstall|status]\n")
			os.Exit(1)
		}
	}

	// Configure logging
	logLevel := new(slog.LevelVar)
	switch os.Getenv("LOG_LEVEL") {
	case "debug":
		logLevel.Set(slog.LevelDebug)
	case "warn":
		logLevel.Set(slog.LevelWarn)
	case "error":
		logLevel.Set(slog.LevelError)
	}
	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: logLevel})))

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	// Database path
	home, err := os.UserHomeDir()
	if err != nil {
		slog.Error("cannot determine home directory", "err", err)
		os.Exit(1)
	}
	dbPath := envOr("CHANGELOGUE_STEALTH_DB", filepath.Join(home, ".changelogue", "stealth.db"))
	port := envOr("CHANGELOGUE_STEALTH_PORT", "9876")
	addr := "localhost:" + port
	noAuth := os.Getenv("NO_AUTH") != "false" // default to no auth in stealth mode

	// Open SQLite store
	store, err := stealth.NewStore(dbPath)
	if err != nil {
		slog.Error("failed to open database", "path", dbPath, "err", err)
		os.Exit(1)
	}
	defer store.Close()

	// Ingestion layer
	svc := ingestion.NewService(store)
	loader := ingestion.NewSourceLoader(store, http.DefaultClient)
	pollInterval := 5 * time.Minute
	if v := os.Getenv("POLL_INTERVAL"); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			pollInterval = d
		}
	}
	orch := ingestion.NewOrchestrator(svc, loader, store, pollInterval)

	// Senders (reuse all existing + shell)
	senders := routing.NewSenders()

	// Wire up post-ingest notification hook
	store.SetNotifyHook(func(ctx context.Context, releaseID, sourceID string) {
		store.NotifyRelease(ctx, releaseID, sourceID, senders)
	})

	// API dependencies
	broadcaster := api.NewBroadcaster()
	mux := http.NewServeMux()
	api.RegisterRoutes(mux, api.Dependencies{
		ProjectsStore:         store,
		ReleasesStore:         store,
		SubscriptionsStore:    store,
		SourcesStore:          store,
		ChannelsStore:         store,
		ContextSourcesStore:   stealth.ContextSourcesStub{},
		SemanticReleasesStore: stealth.SemanticReleasesStub{},
		AgentStore:            stealth.AgentStub{},
		TodosStore:            stealth.TodosStub{},
		OnboardStore:          stealth.OnboardStub{},
		GatesStore:            stealth.GatesStub{},
		KeyStore:              store,
		SessionValidator:      stealth.SessionValidatorStub{},
		HealthChecker:         store,
		Broadcaster:           broadcaster,
		NoAuth:                noAuth,
		IngestionService:      svc,
		HTTPClient:            http.DefaultClient,
	})

	srv := &http.Server{Addr: addr, Handler: api.CORS(mux)}

	// Write PID file
	pidPath := filepath.Join(home, ".changelogue", "stealth.pid")
	os.WriteFile(pidPath, []byte(fmt.Sprintf("%d", os.Getpid())), 0o644)
	defer os.Remove(pidPath)

	// Start polling in background
	go orch.Run(ctx)

	// Start HTTP server
	go func() {
		slog.Info("stealth server starting", "addr", addr, "db", dbPath)
		if err := srv.ListenAndServe(); err != http.ErrServerClosed {
			slog.Error("server error", "err", err)
		}
	}()

	<-ctx.Done()
	slog.Info("shutting down")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	srv.Shutdown(shutdownCtx)
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func install() {
	fmt.Println("TODO: install system service (launchd/systemd)")
	// Platform-specific implementation deferred to a follow-up task.
	// The stealth binary works via `clog-stealth serve` in the meantime.
}

func uninstall() {
	fmt.Println("TODO: uninstall system service")
	// Platform-specific implementation deferred to a follow-up task.
}

func status() {
	home, _ := os.UserHomeDir()
	pidPath := filepath.Join(home, ".changelogue", "stealth.pid")
	data, err := os.ReadFile(pidPath)
	if err != nil {
		fmt.Println("stealth: not running (no PID file)")
		return
	}
	fmt.Printf("stealth: running (PID %s)\n", string(data))
}
```

Note: This requires `Store.SetNotifyHook` — a callback that `IngestRelease` calls after a successful insert. Add this to `internal/stealth/store.go`:

```go
type NotifyHook func(ctx context.Context, releaseID, sourceID string)

func (s *Store) SetNotifyHook(hook NotifyHook) {
	s.notifyHook = hook
}
```

And in `IngestRelease`, after the successful insert:

```go
if s.notifyHook != nil {
	s.notifyHook(ctx, releaseID, sourceID)
}
```

Add `notifyHook NotifyHook` to the `Store` struct.

- [ ] **Step 2: Add Makefile targets**

Append to `Makefile`:

```makefile
# --- Stealth Mode ---
stealth:
	go build -o clog-stealth ./cmd/stealth

stealth-run: stealth
	./clog-stealth serve
```

- [ ] **Step 3: Build and verify**

Run: `cd /Users/pc/web3/ReleaseBeacon && go build ./cmd/stealth/...`
Expected: Compiles successfully.

- [ ] **Step 4: Commit**

```bash
git add cmd/stealth/main.go Makefile
git commit -m "feat(stealth): add stealth mode binary with SQLite, polling, and shell callbacks"
```

---

### Task 11: Integration Test — End-to-End Stealth Flow

**Files:**
- Create: `internal/stealth/integration_test.go`

- [ ] **Step 1: Write integration test**

Create `internal/stealth/integration_test.go`:

```go
package stealth

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/sentioxyz/changelogue/internal/api"
	"github.com/sentioxyz/changelogue/internal/models"
	"github.com/sentioxyz/changelogue/internal/routing"
)

func TestStealthIntegration(t *testing.T) {
	ctx := context.Background()
	store := testStore(t)

	// Set up API server
	broadcaster := api.NewBroadcaster()
	mux := http.NewServeMux()
	api.RegisterRoutes(mux, api.Dependencies{
		ProjectsStore:         store,
		ReleasesStore:         store,
		SubscriptionsStore:    store,
		SourcesStore:          store,
		ChannelsStore:         store,
		ContextSourcesStore:   ContextSourcesStub{},
		SemanticReleasesStore: SemanticReleasesStub{},
		AgentStore:            AgentStub{},
		TodosStore:            TodosStub{},
		OnboardStore:          OnboardStub{},
		GatesStore:            GatesStub{},
		KeyStore:              store,
		SessionValidator:      SessionValidatorStub{},
		HealthChecker:         store,
		Broadcaster:           broadcaster,
		NoAuth:                true,
		HTTPClient:            http.DefaultClient,
	})
	ts := httptest.NewServer(mux)
	defer ts.Close()

	// 1. Create project via API
	resp := post(t, ts, "/api/v1/projects", `{"name":"test-proj","description":"test"}`)
	if resp.StatusCode != http.StatusCreated {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("create project: %d %s", resp.StatusCode, string(body))
	}
	var projResp struct{ Data models.Project }
	json.NewDecoder(resp.Body).Decode(&projResp)
	projectID := projResp.Data.ID

	// 2. Create source via API
	resp = post(t, ts, fmt.Sprintf("/api/v1/projects/%s/sources", projectID),
		`{"provider":"github","repository":"cli/cli","poll_interval_seconds":300}`)
	if resp.StatusCode != http.StatusCreated {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("create source: %d %s", resp.StatusCode, string(body))
	}
	var srcResp struct{ Data models.Source }
	json.NewDecoder(resp.Body).Decode(&srcResp)
	sourceID := srcResp.Data.ID

	// 3. Create shell channel
	resp = post(t, ts, "/api/v1/notification-channels",
		`{"name":"test-shell","type":"shell","config":{"timeout_seconds":5}}`)
	if resp.StatusCode != http.StatusCreated {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("create channel: %d %s", resp.StatusCode, string(body))
	}
	var chResp struct{ Data models.NotificationChannel }
	json.NewDecoder(resp.Body).Decode(&chResp)
	channelID := chResp.Data.ID

	// 4. Create subscription with command config
	subBody := fmt.Sprintf(`{
		"channel_id":"%s","type":"source_release","source_id":"%s",
		"config":{"command":"echo test-callback"}
	}`, channelID, sourceID)
	resp = post(t, ts, "/api/v1/subscriptions", subBody)
	if resp.StatusCode != http.StatusCreated {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("create subscription: %d %s", resp.StatusCode, string(body))
	}

	// 5. Verify store has the data
	projects, total, _ := store.ListProjects(ctx, 1, 50)
	if total != 1 || len(projects) != 1 {
		t.Errorf("expected 1 project, got %d", total)
	}

	sources, total, _ := store.ListSourcesByProject(ctx, projectID, 1, 50)
	if total != 1 || len(sources) != 1 {
		t.Errorf("expected 1 source, got %d", total)
	}

	// 6. Test notification dispatch (manual ingest)
	senders := routing.NewSenders()
	store.SetNotifyHook(func(ctx context.Context, releaseID, sourceID string) {
		store.NotifyRelease(ctx, releaseID, sourceID, senders)
	})

	result := &ingestion.IngestionResult{
		Repository: "cli/cli",
		RawVersion: "v2.50.0",
		Timestamp:  time.Now(),
		Metadata:   map[string]string{"tag": "v2.50.0"},
	}
	if err := store.IngestRelease(ctx, sourceID, result); err != nil {
		t.Fatalf("IngestRelease: %v", err)
	}

	// Wait briefly for async shell execution
	time.Sleep(200 * time.Millisecond)

	// 7. Verify release via API
	resp = get(t, ts, fmt.Sprintf("/api/v1/sources/%s/releases", sourceID))
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("list releases: %d", resp.StatusCode)
	}
}

func get(t *testing.T, ts *httptest.Server, path string) *http.Response {
	t.Helper()
	resp, err := http.Get(ts.URL + path)
	if err != nil {
		t.Fatalf("GET %s: %v", path, err)
	}
	return resp
}

func post(t *testing.T, ts *httptest.Server, path, body string) *http.Response {
	t.Helper()
	resp, err := http.Post(ts.URL+path, "application/json", strings.NewReader(body))
	if err != nil {
		t.Fatalf("POST %s: %v", path, err)
	}
	return resp
}
```

- [ ] **Step 2: Run the integration test**

Run: `go test -v -run TestStealthIntegration ./internal/stealth/...`
Expected: PASS

- [ ] **Step 3: Fix any issues, then commit**

```bash
git add internal/stealth/integration_test.go
git commit -m "test(stealth): add end-to-end integration test for stealth mode"
```

---

### Task 12: CLI `--config` Flag for Subscriptions

**Files:**
- Modify: `internal/cli/subscriptions.go`

- [ ] **Step 1: Add `--config` flag to subscription create command**

In `internal/cli/subscriptions.go`, find the `create` subcommand and add a `--config` string flag. When set, include it in the request body as parsed JSON:

```go
createCmd.Flags().String("config", "", "JSON config for the subscription (e.g., shell command)")
```

In the run function, check if `--config` is set, parse it as `json.RawMessage`, and include it in the body map:

```go
if cfgStr, _ := cmd.Flags().GetString("config"); cfgStr != "" {
	var cfgJSON json.RawMessage
	if err := json.Unmarshal([]byte(cfgStr), &cfgJSON); err != nil {
		return fmt.Errorf("invalid --config JSON: %w", err)
	}
	body["config"] = cfgJSON
}
```

- [ ] **Step 2: Verify build**

Run: `go build ./cmd/cli/...`
Expected: Compiles.

- [ ] **Step 3: Commit**

```bash
git add internal/cli/subscriptions.go
git commit -m "feat(cli): add --config flag to subscription create command"
```

---

### Task 13: Final verification and cleanup

**Files:**
- All modified files

- [ ] **Step 1: Run full test suite**

Run: `go test ./...`
Expected: All tests pass.

- [ ] **Step 2: Run go vet**

Run: `go vet ./...`
Expected: No errors.

- [ ] **Step 3: Build all binaries**

Run: `go build ./cmd/server && go build -o clog ./cmd/cli && go build -o clog-stealth ./cmd/stealth`
Expected: All three binaries build successfully.

- [ ] **Step 4: Manual smoke test**

Run: `./clog-stealth serve &`
Then:
```bash
export CHANGELOGUE_SERVER=http://localhost:9876
./clog projects create --name test-project
./clog sources create --project-id <ID> --provider github --repository cli/cli
./clog channels create --name my-shell --type shell --config '{"timeout_seconds":10}'
./clog subscriptions create --source-id <ID> --channel-id <ID> --type source_release --config '{"command":"echo hello"}'
./clog sources list
./clog releases list
kill %1
```

- [ ] **Step 5: Final commit**

```bash
git add -A
git commit -m "feat(stealth): complete stealth mode implementation with SQLite, shell callbacks, and system service"
```
