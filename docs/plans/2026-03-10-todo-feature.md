# TODO Feature Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Add release TODO tracking with acknowledge/resolve actions in notification cards and a dedicated web portal tab.

**Architecture:** New `release_todos` table with 1:1 relationship to releases/semantic_releases. TODOs created during notification routing. API endpoints for list/get/acknowledge/resolve. One-click GET endpoints for notification button links with redirect. Senders add action buttons/links. New `/todo` page in frontend.

**Tech Stack:** Go (backend), PostgreSQL (schema), Next.js + React + SWR + Tailwind (frontend)

**Design Doc:** `docs/plans/2026-03-10-todo-feature-design.md`

---

### Task 1: Database Schema — Add `release_todos` Table

**Files:**
- Modify: `internal/db/migrations.go:160` (add table to schema string)

**Step 1: Add release_todos table to schema**

In `internal/db/migrations.go`, add the following to the `schema` constant, after the `semantic_release_trigger` CREATE TRIGGER block (line 159) and before the closing backtick (line 160):

```sql
-- Release TODOs (acknowledge/resolve tracking)
CREATE TABLE IF NOT EXISTS release_todos (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    release_id UUID REFERENCES releases(id) ON DELETE CASCADE,
    semantic_release_id UUID REFERENCES semantic_releases(id) ON DELETE CASCADE,
    status VARCHAR(20) NOT NULL DEFAULT 'pending',
    created_at TIMESTAMPTZ DEFAULT NOW(),
    acknowledged_at TIMESTAMPTZ,
    resolved_at TIMESTAMPTZ,
    CHECK (
        (release_id IS NOT NULL AND semantic_release_id IS NULL) OR
        (release_id IS NULL AND semantic_release_id IS NOT NULL)
    )
);
CREATE UNIQUE INDEX IF NOT EXISTS idx_release_todos_release_id ON release_todos(release_id) WHERE release_id IS NOT NULL;
CREATE UNIQUE INDEX IF NOT EXISTS idx_release_todos_semantic_release_id ON release_todos(semantic_release_id) WHERE semantic_release_id IS NOT NULL;
```

Note: Use partial unique indexes instead of UNIQUE constraints since the columns are nullable.

**Step 2: Verify migration compiles**

Run: `go build ./internal/db/...`
Expected: Success, no errors.

**Step 3: Commit**

```bash
git add internal/db/migrations.go
git commit -m "feat(db): add release_todos table for acknowledge/resolve tracking"
```

---

### Task 2: Model — Add Todo Struct

**Files:**
- Create: `internal/models/todo.go`

**Step 1: Create the model**

```go
package models

import "time"

// Todo represents a release TODO item for acknowledge/resolve tracking.
type Todo struct {
	ID                string     `json:"id"`
	ReleaseID         *string    `json:"release_id,omitempty"`
	SemanticReleaseID *string    `json:"semantic_release_id,omitempty"`
	Status            string     `json:"status"` // pending, acknowledged, resolved
	CreatedAt         time.Time  `json:"created_at"`
	AcknowledgedAt    *time.Time `json:"acknowledged_at,omitempty"`
	ResolvedAt        *time.Time `json:"resolved_at,omitempty"`
	// Enriched fields from JOINs (populated by list queries)
	ProjectName string `json:"project_name,omitempty"`
	Version     string `json:"version,omitempty"`
	Provider    string `json:"provider,omitempty"`
	Repository  string `json:"repository,omitempty"`
	SourceURL   string `json:"source_url,omitempty"`
	ReleaseURL  string `json:"release_url,omitempty"`
	Urgency     string `json:"urgency,omitempty"`
	TodoType    string `json:"todo_type,omitempty"` // "release" or "semantic"
}
```

**Step 2: Verify it compiles**

Run: `go build ./internal/models/...`
Expected: Success.

**Step 3: Commit**

```bash
git add internal/models/todo.go
git commit -m "feat(models): add Todo struct for release TODO tracking"
```

---

### Task 3: Store — Add TodosStore to PgStore

**Files:**
- Create: `internal/api/todos.go` (handler — but write store interface here first)
- Modify: `internal/api/pgstore.go` (add store methods at the end)

**Step 1: Add store methods to pgstore.go**

Add the following at the end of `internal/api/pgstore.go`:

```go
// --- TodosStore ---

func (s *PgStore) ListTodos(ctx context.Context, status string, page, perPage int) ([]models.Todo, int, error) {
	// Count query
	countQuery := `SELECT COUNT(*) FROM release_todos`
	args := []any{}
	if status != "" {
		countQuery += ` WHERE status = $1`
		args = append(args, status)
	}
	var total int
	if err := s.pool.QueryRow(ctx, countQuery, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count todos: %w", err)
	}

	offset := (page - 1) * perPage

	// Build the enriched query with LEFT JOINs to releases and semantic_releases.
	query := `
		SELECT
			t.id, t.release_id, t.semantic_release_id, t.status,
			t.created_at, t.acknowledged_at, t.resolved_at,
			COALESCE(p1.name, p2.name, ''),
			COALESCE(r.version, sr.version, ''),
			COALESCE(src.provider, ''),
			COALESCE(src.repository, ''),
			CASE WHEN t.release_id IS NOT NULL THEN 'release' ELSE 'semantic' END
		FROM release_todos t
		LEFT JOIN releases r ON r.id = t.release_id
		LEFT JOIN sources src ON src.id = r.source_id
		LEFT JOIN projects p1 ON p1.id = src.project_id
		LEFT JOIN semantic_releases sr ON sr.id = t.semantic_release_id
		LEFT JOIN projects p2 ON p2.id = sr.project_id
	`
	queryArgs := []any{}
	argIdx := 1
	if status != "" {
		query += fmt.Sprintf(` WHERE t.status = $%d`, argIdx)
		queryArgs = append(queryArgs, status)
		argIdx++
	}
	query += fmt.Sprintf(` ORDER BY t.created_at DESC LIMIT $%d OFFSET $%d`, argIdx, argIdx+1)
	queryArgs = append(queryArgs, perPage, offset)

	rows, err := s.pool.Query(ctx, query, queryArgs...)
	if err != nil {
		return nil, 0, fmt.Errorf("list todos: %w", err)
	}
	defer rows.Close()

	var todos []models.Todo
	for rows.Next() {
		var t models.Todo
		if err := rows.Scan(
			&t.ID, &t.ReleaseID, &t.SemanticReleaseID, &t.Status,
			&t.CreatedAt, &t.AcknowledgedAt, &t.ResolvedAt,
			&t.ProjectName, &t.Version, &t.Provider, &t.Repository, &t.TodoType,
		); err != nil {
			return nil, 0, fmt.Errorf("scan todo: %w", err)
		}
		todos = append(todos, t)
	}
	return todos, total, nil
}

func (s *PgStore) GetTodo(ctx context.Context, id string) (*models.Todo, error) {
	var t models.Todo
	err := s.pool.QueryRow(ctx, `
		SELECT
			t.id, t.release_id, t.semantic_release_id, t.status,
			t.created_at, t.acknowledged_at, t.resolved_at,
			COALESCE(p1.name, p2.name, ''),
			COALESCE(r.version, sr.version, ''),
			COALESCE(src.provider, ''),
			COALESCE(src.repository, ''),
			CASE WHEN t.release_id IS NOT NULL THEN 'release' ELSE 'semantic' END
		FROM release_todos t
		LEFT JOIN releases r ON r.id = t.release_id
		LEFT JOIN sources src ON src.id = r.source_id
		LEFT JOIN projects p1 ON p1.id = src.project_id
		LEFT JOIN semantic_releases sr ON sr.id = t.semantic_release_id
		LEFT JOIN projects p2 ON p2.id = sr.project_id
		WHERE t.id = $1
	`, id).Scan(
		&t.ID, &t.ReleaseID, &t.SemanticReleaseID, &t.Status,
		&t.CreatedAt, &t.AcknowledgedAt, &t.ResolvedAt,
		&t.ProjectName, &t.Version, &t.Provider, &t.Repository, &t.TodoType,
	)
	if err != nil {
		return nil, fmt.Errorf("get todo: %w", err)
	}
	return &t, nil
}

func (s *PgStore) AcknowledgeTodo(ctx context.Context, id string) error {
	tag, err := s.pool.Exec(ctx,
		`UPDATE release_todos SET status = 'acknowledged', acknowledged_at = NOW() WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("acknowledge todo: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("todo not found")
	}
	return nil
}

func (s *PgStore) ResolveTodo(ctx context.Context, id string) error {
	tag, err := s.pool.Exec(ctx,
		`UPDATE release_todos SET status = 'resolved', resolved_at = NOW() WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("resolve todo: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("todo not found")
	}
	return nil
}

// CreateReleaseTodo inserts a TODO for a source release. Returns the todo ID.
// Uses ON CONFLICT DO NOTHING for idempotency.
func (s *PgStore) CreateReleaseTodo(ctx context.Context, releaseID string) (string, error) {
	var id string
	err := s.pool.QueryRow(ctx,
		`INSERT INTO release_todos (release_id) VALUES ($1)
		 ON CONFLICT (release_id) WHERE release_id IS NOT NULL DO UPDATE SET release_id = EXCLUDED.release_id
		 RETURNING id`, releaseID).Scan(&id)
	if err != nil {
		return "", fmt.Errorf("create release todo: %w", err)
	}
	return id, nil
}

// CreateSemanticReleaseTodo inserts a TODO for a semantic release. Returns the todo ID.
// Uses ON CONFLICT DO NOTHING for idempotency.
func (s *PgStore) CreateSemanticReleaseTodo(ctx context.Context, semanticReleaseID string) (string, error) {
	var id string
	err := s.pool.QueryRow(ctx,
		`INSERT INTO release_todos (semantic_release_id) VALUES ($1)
		 ON CONFLICT (semantic_release_id) WHERE semantic_release_id IS NOT NULL DO UPDATE SET semantic_release_id = EXCLUDED.semantic_release_id
		 RETURNING id`, semanticReleaseID).Scan(&id)
	if err != nil {
		return "", fmt.Errorf("create semantic release todo: %w", err)
	}
	return id, nil
}
```

**Step 2: Verify it compiles**

Run: `go build ./internal/api/...`
Expected: Success.

**Step 3: Commit**

```bash
git add internal/api/pgstore.go
git commit -m "feat(store): add TodosStore methods to PgStore"
```

---

### Task 4: API Handler — Todos Endpoints

**Files:**
- Create: `internal/api/todos.go`

**Step 1: Create the handler file**

```go
package api

import (
	"context"
	"net/http"

	"github.com/sentioxyz/changelogue/internal/models"
)

// TodosStore defines the data access interface for TODO operations.
type TodosStore interface {
	ListTodos(ctx context.Context, status string, page, perPage int) ([]models.Todo, int, error)
	GetTodo(ctx context.Context, id string) (*models.Todo, error)
	AcknowledgeTodo(ctx context.Context, id string) error
	ResolveTodo(ctx context.Context, id string) error
}

// TodosHandler handles HTTP requests for release TODOs.
type TodosHandler struct {
	store     TodosStore
	publicURL string
}

// NewTodosHandler creates a new TodosHandler.
func NewTodosHandler(store TodosStore, publicURL string) *TodosHandler {
	return &TodosHandler{store: store, publicURL: publicURL}
}

// List returns a paginated list of TODOs, optionally filtered by status.
func (h *TodosHandler) List(w http.ResponseWriter, r *http.Request) {
	page, perPage := ParsePagination(r)
	status := r.URL.Query().Get("status")

	todos, total, err := h.store.ListTodos(r.Context(), status, page, perPage)
	if err != nil {
		WriteError(w, r, http.StatusInternalServerError, "internal_error", err.Error())
		return
	}
	if todos == nil {
		todos = []models.Todo{}
	}
	WriteList(w, r, todos, page, perPage, total)
}

// Get returns a single TODO by ID.
func (h *TodosHandler) Get(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	todo, err := h.store.GetTodo(r.Context(), id)
	if err != nil {
		WriteError(w, r, http.StatusNotFound, "not_found", "Todo not found")
		return
	}
	WriteJSON(w, r, http.StatusOK, todo)
}

// Acknowledge marks a TODO as acknowledged. Supports both PATCH (API) and GET with redirect (notification links).
func (h *TodosHandler) Acknowledge(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if err := h.store.AcknowledgeTodo(r.Context(), id); err != nil {
		WriteError(w, r, http.StatusNotFound, "not_found", "Todo not found")
		return
	}

	// If redirect=true, send 302 to the frontend todo page.
	if r.URL.Query().Get("redirect") == "true" && h.publicURL != "" {
		http.Redirect(w, r, h.publicURL+"/todo", http.StatusFound)
		return
	}

	WriteJSON(w, r, http.StatusOK, map[string]string{"status": "acknowledged"})
}

// Resolve marks a TODO as resolved. Supports both PATCH (API) and GET with redirect (notification links).
func (h *TodosHandler) Resolve(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if err := h.store.ResolveTodo(r.Context(), id); err != nil {
		WriteError(w, r, http.StatusNotFound, "not_found", "Todo not found")
		return
	}

	// If redirect=true, send 302 to the frontend todo page.
	if r.URL.Query().Get("redirect") == "true" && h.publicURL != "" {
		http.Redirect(w, r, h.publicURL+"/todo", http.StatusFound)
		return
	}

	WriteJSON(w, r, http.StatusOK, map[string]string{"status": "resolved"})
}
```

**Step 2: Verify it compiles**

Run: `go build ./internal/api/...`
Expected: Success.

**Step 3: Commit**

```bash
git add internal/api/todos.go
git commit -m "feat(api): add TodosHandler with list/get/acknowledge/resolve endpoints"
```

---

### Task 5: Route Registration — Wire Todos into Server

**Files:**
- Modify: `internal/api/server.go:12-28` (add TodosStore to Dependencies)
- Modify: `internal/api/server.go:100-104` (add routes after Agent section)
- Modify: `cmd/server/main.go:120-136` (add TodosStore to Dependencies)

**Step 1: Add TodosStore to Dependencies**

In `internal/api/server.go`, add to the `Dependencies` struct (line 22, after `AgentStore`):

```go
TodosStore            TodosStore
PublicURL             string
```

**Step 2: Register todo routes**

In `internal/api/server.go`, after the Agent routes block (after line 104), add:

```go
	// Todos
	todos := NewTodosHandler(deps.TodosStore, deps.PublicURL)
	mux.Handle("GET /api/v1/todos", chain(http.HandlerFunc(todos.List)))
	mux.Handle("GET /api/v1/todos/{id}", chain(http.HandlerFunc(todos.Get)))
	mux.Handle("PATCH /api/v1/todos/{id}/acknowledge", chain(http.HandlerFunc(todos.Acknowledge)))
	mux.Handle("PATCH /api/v1/todos/{id}/resolve", chain(http.HandlerFunc(todos.Resolve)))
	// One-click endpoints for notification links (GET so they work as <a href>)
	mux.Handle("GET /api/v1/todos/{id}/acknowledge", chain(http.HandlerFunc(todos.Acknowledge)))
	mux.Handle("GET /api/v1/todos/{id}/resolve", chain(http.HandlerFunc(todos.Resolve)))
```

**Step 3: Wire in main.go**

In `cmd/server/main.go`, add to the `api.Dependencies` struct (around line 130):

```go
TodosStore:            pgStore,
PublicURL:             os.Getenv("PUBLIC_URL"),
```

**Step 4: Verify it compiles**

Run: `go build ./cmd/server/...`
Expected: Success.

**Step 5: Commit**

```bash
git add internal/api/server.go cmd/server/main.go
git commit -m "feat(api): register TODO routes and wire Dependencies"
```

---

### Task 6: Notification Integration — Add TodoID to Notification + Create TODOs

**Files:**
- Modify: `internal/routing/sender.go:15-24` (add TodoID field)
- Modify: `internal/routing/worker.go:18-27` (add CreateReleaseTodo to NotifyStore)
- Modify: `internal/routing/worker.go:81-154` (create TODO before sending)
- Modify: `internal/agent/orchestrator.go:32-42` (add CreateSemanticReleaseTodo to OrchestratorStore)
- Modify: `internal/agent/orchestrator.go:454-511` (create TODO in sendProjectNotifications)

**Step 1: Add TodoID to Notification struct**

In `internal/routing/sender.go`, add after line 23 (`SourceURL`):

```go
	TodoID    string `json:"todo_id,omitempty"`    // for constructing acknowledge/resolve URLs
```

**Step 2: Add CreateReleaseTodo to NotifyStore**

In `internal/routing/worker.go`, add to the `NotifyStore` interface (after line 26):

```go
	CreateReleaseTodo(ctx context.Context, releaseID string) (string, error)
```

**Step 3: Create TODO in NotifyWorker.Work()**

In `internal/routing/worker.go`, after the version filter checks (after line 106, before the `subs` fetch on line 108), add:

```go
	// Create a TODO for this release (idempotent — safe for retries).
	todoID, todoErr := w.store.CreateReleaseTodo(ctx, release.ID)
	if todoErr != nil {
		slog.Error("create release todo failed", "release_id", release.ID, "err", todoErr)
		// Continue — notification delivery is primary responsibility.
	}
```

Then in the Notification construction (around line 132), add `TodoID`:

After `msg.ReleaseURL = ...` (line 143), add:
```go
		if todoID != "" {
			msg.TodoID = todoID
		}
```

**Step 4: Add CreateSemanticReleaseTodo to OrchestratorStore**

In `internal/agent/orchestrator.go`, add to the `OrchestratorStore` interface (after line 41):

```go
	CreateSemanticReleaseTodo(ctx context.Context, semanticReleaseID string) (string, error)
```

**Step 5: Create TODO in sendProjectNotifications**

In `internal/agent/orchestrator.go`, in `sendProjectNotifications()` (around line 467, after `senders := defaultSenders()`), add:

```go
	// Create a TODO for this semantic release.
	todoID, todoErr := o.store.CreateSemanticReleaseTodo(ctx, result.semanticReleaseID)
	if todoErr != nil {
		slog.Error("create semantic release todo failed",
			"semantic_release_id", result.semanticReleaseID, "err", todoErr)
	}
```

Then add `TodoID` to the `msg` construction (around line 474):
```go
	msg.TodoID = todoID
```

**Step 6: Verify it compiles**

Run: `go build ./cmd/server/...`
Expected: Success.

**Step 7: Commit**

```bash
git add internal/routing/sender.go internal/routing/worker.go internal/agent/orchestrator.go
git commit -m "feat(routing): create TODOs during notification routing and pass TodoID to senders"
```

---

### Task 7: Sender Updates — Add Action Buttons/Links

**Files:**
- Modify: `internal/routing/slack.go` (add actions block)
- Modify: `internal/routing/discord.go` (add action links)
- Modify: `internal/routing/email.go` (add TodoAckURL/TodoResolveURL to emailData)
- Modify: `internal/routing/email.html.tmpl` (add action buttons)
- Modify: `internal/routing/webhook.go` (add URLs to payload)
- Modify: `internal/routing/sender.go` (add helper to build todo URLs)

**Step 1: Add TodoURLs helper to sender.go**

In `internal/routing/sender.go`, add after the `ProviderLabel` function (after line 90):

```go
// TodoAcknowledgeURL returns the one-click acknowledge URL for a TODO.
func TodoAcknowledgeURL(publicURL, todoID string) string {
	if publicURL == "" || todoID == "" {
		return ""
	}
	return fmt.Sprintf("%s/api/v1/todos/%s/acknowledge?redirect=true", publicURL, todoID)
}

// TodoResolveURL returns the one-click resolve URL for a TODO.
func TodoResolveURL(publicURL, todoID string) string {
	if publicURL == "" || todoID == "" {
		return ""
	}
	return fmt.Sprintf("%s/api/v1/todos/%s/resolve?redirect=true", publicURL, todoID)
}
```

**Step 2: Update SlackSender**

In `internal/routing/slack.go`, modify `buildSemanticBlocks()` — after the footer context block (line 141), before the `return blocks` (line 143), add:

```go
	// Action buttons for TODO acknowledge/resolve
	if msg.TodoID != "" {
		ackURL := TodoAcknowledgeURL(msg.ReleaseURL[:strings.Index(msg.ReleaseURL, "/releases")], msg.TodoID)
		resURL := TodoResolveURL(msg.ReleaseURL[:strings.Index(msg.ReleaseURL, "/releases")], msg.TodoID)
		if ackURL != "" {
			blocks = append(blocks, slackBlock{
				Type: "actions",
				Elements: []slackText{
					{Type: "button", Text: "✅ Acknowledge"},
					{Type: "button", Text: "☑️ Resolve"},
				},
			})
		}
	}
```

Wait — the Slack Block Kit `actions` block needs a different struct shape for buttons with URLs. Let me revise.

Instead, add a new `slackActionsBlock` type and use the `slackBlock` approach differently. Actually, looking at the existing code, the `slackBlock.Elements` field uses `slackText` type, but Slack buttons need `url` fields. We need to add proper button support.

Add to `internal/routing/slack.go`, after the `slackText` struct (after line 39):

```go
// slackButton represents a Slack button element in an actions block.
type slackButton struct {
	Type string    `json:"type"`
	Text slackText `json:"text"`
	URL  string    `json:"url,omitempty"`
}

// slackActionsBlock is a Slack actions block containing button elements.
type slackActionsBlock struct {
	Type     string        `json:"type"`
	Elements []slackButton `json:"elements"`
}
```

Now, the challenge is that `slackPayload.Blocks` is `[]slackBlock`. We need to support both block types. Change `slackPayload.Blocks` to `[]any`:

In `internal/routing/slack.go`, change the `slackPayload` struct (line 24-27):
```go
type slackPayload struct {
	Blocks      []any             `json:"blocks,omitempty"`
	Attachments []slackAttachment `json:"attachments,omitempty"`
}
```

And update `buildSemanticBlocks` return type to `[]any` and cast accordingly. After the footer context block, before `return blocks`, add:

```go
	// Action buttons for TODO acknowledge/resolve
	if msg.TodoID != "" {
		ackURL := TodoAcknowledgeURL("", msg.TodoID)
		resURL := TodoResolveURL("", msg.TodoID)
		// The URLs are constructed in Send() using publicURL, so we store them in Notification-level.
		// Actually, we need the public URL here. It's embedded in ReleaseURL already.
		// Extract base from ReleaseURL.
		if msg.ReleaseURL != "" {
			if idx := strings.Index(msg.ReleaseURL, "/releases"); idx > 0 {
				base := msg.ReleaseURL[:idx]
				ackURL = TodoAcknowledgeURL(base, msg.TodoID)
				resURL = TodoResolveURL(base, msg.TodoID)
			} else if idx := strings.Index(msg.ReleaseURL, "/projects"); idx > 0 {
				base := msg.ReleaseURL[:idx]
				ackURL = TodoAcknowledgeURL(base, msg.TodoID)
				resURL = TodoResolveURL(base, msg.TodoID)
			}
		}
		if ackURL != "" {
			blocks = append(blocks, slackActionsBlock{
				Type: "actions",
				Elements: []slackButton{
					{Type: "button", Text: slackText{Type: "plain_text", Text: "✅ Acknowledge"}, URL: ackURL},
					{Type: "button", Text: slackText{Type: "plain_text", Text: "☑️ Resolve"}, URL: resURL},
				},
			})
		}
	}
```

Actually this is getting complicated extracting the base URL. Let's take a simpler approach — add a helper function in `sender.go` that extracts the base URL from the `ReleaseURL`, or better yet, just store the public URL in the Notification struct.

**Revised Step 1: Add PublicURL to Notification struct**

In `internal/routing/sender.go`, add after `TodoID` (the field added in Task 6):

```go
	PublicURL string `json:"-"` // base URL for constructing action links (not serialized)
```

Then in `internal/routing/worker.go`, in the Work() method, set it:
```go
	msg.PublicURL = w.publicURL
```

And in `internal/agent/orchestrator.go`, in `sendProjectNotifications()`:
```go
	msg.PublicURL = o.publicURL
```

**Revised Step 2: Update Slack buildSemanticBlocks**

Change `buildSemanticBlocks` to return `[]any` and add action buttons. After footer context block:

```go
	if msg.TodoID != "" && msg.PublicURL != "" {
		blocks = append(blocks, slackActionsBlock{
			Type: "actions",
			Elements: []slackButton{
				{Type: "button", Text: slackText{Type: "plain_text", Text: "✅ Acknowledge"}, URL: TodoAcknowledgeURL(msg.PublicURL, msg.TodoID)},
				{Type: "button", Text: slackText{Type: "plain_text", Text: "☑️ Resolve"}, URL: TodoResolveURL(msg.PublicURL, msg.TodoID)},
			},
		})
	}
```

Also add action buttons to the source-level Slack message (the attachment path and the no-changelog path). For the attachment path, add action links as text after the links. For the no-changelog path, add an actions block.

In the no-changelog path (around line 234, after `payload.Blocks = blocks`), add the actions block similarly.

In the attachment path (around line 189, after the linkParts), add acknowledge/resolve links:
```go
			if msg.TodoID != "" && msg.PublicURL != "" {
				linkParts = append(linkParts,
					fmt.Sprintf("<%s|✅ Acknowledge>", TodoAcknowledgeURL(msg.PublicURL, msg.TodoID)),
					fmt.Sprintf("<%s|☑️ Resolve>", TodoResolveURL(msg.PublicURL, msg.TodoID)),
				)
			}
```

**Step 3: Update Discord buildSemanticEmbed**

In `internal/routing/discord.go`, in `buildSemanticEmbed()`, after the links section (around line 115), add:

```go
	// Action links for TODO acknowledge/resolve
	if msg.TodoID != "" && msg.PublicURL != "" {
		linkParts = append(linkParts,
			fmt.Sprintf("[✅ Acknowledge](%s)", TodoAcknowledgeURL(msg.PublicURL, msg.TodoID)),
			fmt.Sprintf("[☑️ Resolve](%s)", TodoResolveURL(msg.PublicURL, msg.TodoID)),
		)
	}
```

Also add to the source-level fallback in `Send()` (around line 167):
```go
		if msg.TodoID != "" && msg.PublicURL != "" {
			linkParts = append(linkParts,
				fmt.Sprintf("[✅ Acknowledge](%s)", TodoAcknowledgeURL(msg.PublicURL, msg.TodoID)),
				fmt.Sprintf("[☑️ Resolve](%s)", TodoResolveURL(msg.PublicURL, msg.TodoID)),
			)
		}
```

**Step 4: Update Email**

Add `TodoAckURL` and `TodoResolveURL` fields to `emailData` struct in `internal/routing/email.go`:

```go
	TodoAckURL      string
	TodoResolveURL  string
```

In `buildEmailData()`, set them:
```go
	if msg.TodoID != "" && msg.PublicURL != "" {
		data.TodoAckURL = TodoAcknowledgeURL(msg.PublicURL, msg.TodoID)
		data.TodoResolveURL = TodoResolveURL(msg.PublicURL, msg.TodoID)
	}
```

In `email.html.tmpl`, after the existing Links section (line 53-54), add:

```html
{{if .TodoAckURL}}
<!-- Action Buttons -->
<tr><td style="padding:0 24px 16px;">
<a href="{{.TodoAckURL}}" style="display:inline-block;padding:8px 16px;background-color:#16A34A;color:#ffffff;font-size:13px;font-weight:600;text-decoration:none;border-radius:4px;margin-right:8px;">✅ Acknowledge</a>
<a href="{{.TodoResolveURL}}" style="display:inline-block;padding:8px 16px;background-color:#2563EB;color:#ffffff;font-size:13px;font-weight:600;text-decoration:none;border-radius:4px;">☑️ Resolve</a>
</td></tr>
{{end}}
```

Also add to `buildPlainText()`:
```go
	if data.TodoAckURL != "" {
		b.WriteString("\nAcknowledge: " + data.TodoAckURL + "\n")
		b.WriteString("Resolve: " + data.TodoResolveURL + "\n")
	}
```

**Step 5: Update Webhook**

In `internal/routing/webhook.go`, add to `webhookSemanticPayload` (after line 31):

```go
	AcknowledgeURL string `json:"acknowledge_url,omitempty"`
	ResolveURL     string `json:"resolve_url,omitempty"`
```

In `Send()`, after building the payload (around line 53), set the URLs:
```go
		if msg.TodoID != "" && msg.PublicURL != "" {
			p := payload.(webhookSemanticPayload) // doesn't work with interface — rearrange
		}
```

Actually, simpler — add the fields inline when constructing the payload. Add to the `webhookSemanticPayload` construction (around line 44-53):
```go
			AcknowledgeURL: TodoAcknowledgeURL(msg.PublicURL, msg.TodoID),
			ResolveURL:     TodoResolveURL(msg.PublicURL, msg.TodoID),
```

For the raw `msg` fallback (line 55), the Notification struct now includes TodoID which will be serialized.

**Step 6: Verify it compiles**

Run: `go build ./cmd/server/...`
Expected: Success.

**Step 7: Commit**

```bash
git add internal/routing/sender.go internal/routing/slack.go internal/routing/discord.go internal/routing/email.go internal/routing/email.html.tmpl internal/routing/webhook.go internal/routing/worker.go internal/agent/orchestrator.go
git commit -m "feat(senders): add acknowledge/resolve action buttons to all notification channels"
```

---

### Task 8: Tests — Backend

**Files:**
- Create: `internal/api/todos_test.go`
- Create: `internal/routing/todo_urls_test.go`

**Step 1: Test the URL helpers**

Create `internal/routing/todo_urls_test.go`:

```go
package routing

import "testing"

func TestTodoAcknowledgeURL(t *testing.T) {
	got := TodoAcknowledgeURL("https://example.com", "abc-123")
	want := "https://example.com/api/v1/todos/abc-123/acknowledge?redirect=true"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestTodoResolveURL(t *testing.T) {
	got := TodoResolveURL("https://example.com", "abc-123")
	want := "https://example.com/api/v1/todos/abc-123/resolve?redirect=true"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestTodoURLsEmptyInputs(t *testing.T) {
	if got := TodoAcknowledgeURL("", "abc"); got != "" {
		t.Errorf("expected empty, got %q", got)
	}
	if got := TodoResolveURL("https://example.com", ""); got != "" {
		t.Errorf("expected empty, got %q", got)
	}
}
```

**Step 2: Run tests**

Run: `go test ./internal/routing/... -run TestTodo -v`
Expected: PASS.

**Step 3: Commit**

```bash
git add internal/routing/todo_urls_test.go
git commit -m "test(routing): add unit tests for TODO URL helpers"
```

---

### Task 9: Frontend — Types and API Client

**Files:**
- Modify: `web/lib/api/types.ts` (add Todo interface)
- Modify: `web/lib/api/client.ts` (add todos API module)

**Step 1: Add Todo type**

In `web/lib/api/types.ts`, after the `BatchDeleteSubscriptionInput` interface (after line 191), add:

```typescript
// --- Todo Types ---

export interface Todo {
  id: string;
  release_id?: string;
  semantic_release_id?: string;
  status: "pending" | "acknowledged" | "resolved";
  created_at: string;
  acknowledged_at?: string;
  resolved_at?: string;
  project_name?: string;
  version?: string;
  provider?: string;
  repository?: string;
  source_url?: string;
  release_url?: string;
  urgency?: string;
  todo_type?: "release" | "semantic";
}
```

**Step 2: Add todos API client**

In `web/lib/api/client.ts`, add the import for `Todo`:

```typescript
import type { ..., Todo } from "./types";
```

Then add the todos module (before the `// --- System ---` section):

```typescript
// --- Todos ---

export const todos = {
  list: (status?: string, page = 1, perPage = 25) => {
    const params = new URLSearchParams({ page: String(page), per_page: String(perPage) });
    if (status) params.set("status", status);
    return request<ApiResponse<Todo[]>>(`/todos?${params}`);
  },
  get: (id: string) =>
    request<ApiResponse<Todo>>(`/todos/${id}`),
  acknowledge: (id: string) =>
    request<ApiResponse<{ status: string }>>(`/todos/${id}/acknowledge`, { method: "PATCH" }),
  resolve: (id: string) =>
    request<ApiResponse<{ status: string }>>(`/todos/${id}/resolve`, { method: "PATCH" }),
};
```

**Step 3: Commit**

```bash
git add web/lib/api/types.ts web/lib/api/client.ts
git commit -m "feat(frontend): add Todo type and API client module"
```

---

### Task 10: Frontend — Sidebar Navigation

**Files:**
- Modify: `web/components/layout/sidebar.tsx:18-25` (add Todo nav item)

**Step 1: Add Todo to navItems**

In `web/components/layout/sidebar.tsx`, add `ListTodo` to the lucide-react import (line 6-15):

```typescript
import {
  LayoutDashboard,
  FolderKanban,
  Package,
  Brain,
  Bell,
  Megaphone,
  PanelLeftOpen,
  PanelLeftClose,
  ListTodo,
} from "lucide-react";
```

Then add the Todo nav item to `navItems` array (after the Dashboard entry, line 19):

```typescript
const navItems = [
  { href: "/", label: "Dashboard", icon: LayoutDashboard },
  { href: "/todo", label: "Todo", icon: ListTodo },
  { href: "/projects", label: "Projects", icon: FolderKanban },
  // ... rest unchanged
];
```

**Step 2: Commit**

```bash
git add web/components/layout/sidebar.tsx
git commit -m "feat(frontend): add Todo item to sidebar navigation"
```

---

### Task 11: Frontend — Todo Page

**Files:**
- Create: `web/app/todo/page.tsx`

**Step 1: Build the todo page**

Use `superpowers:frontend-design` (via the Skill tool) to create a polished Todo page following the existing patterns from `web/app/releases/page.tsx`. The page should:

- Use `"use client"` and Suspense wrapper pattern
- Three tab buttons: Pending / Acknowledged / Resolved (with counts)
- Table with columns: Project, Version, Type, Provider, Urgency, Created, Actions
- SWR data fetching with `todos.list(status, page, perPage)`
- Pagination (Previous/Next) matching existing pattern
- Action buttons: "Acknowledge" on pending tab, "Resolve" on acknowledged tab
- Optimistic UI updates via SWR `mutate()`
- SSE revalidation (listen to `/api/v1/events` and revalidate SWR key)
- Follow existing styling patterns (inline styles, CSS variables, DM Sans font)
- Provider badges, urgency badges matching existing components
- Version links to release detail pages

**Step 2: Commit**

```bash
git add web/app/todo/page.tsx
git commit -m "feat(frontend): add Todo page with status tabs and action buttons"
```

---

### Task 12: Verification — End-to-end

**Step 1: Run all Go tests**

Run: `go test ./...`
Expected: All pass.

**Step 2: Run go vet**

Run: `go vet ./...`
Expected: No issues.

**Step 3: Build the binary**

Run: `go build -o changelogue ./cmd/server`
Expected: Success.

**Step 4: Build frontend (if npm available)**

Run: `cd web && npm run build` (or verify no TypeScript errors)
Expected: Success.

**Step 5: Manual smoke test**

Run: `make dev` and verify:
- `/api/v1/todos` returns `{"data":[],"meta":{"page":1,"per_page":25,"total":0}}`
- The frontend `/todo` page loads with empty state
- Sidebar shows "Todo" between Dashboard and Projects

**Step 6: Final commit if any fixes needed**

```bash
git add -A
git commit -m "fix: address any issues found during verification"
```
