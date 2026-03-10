# Design: Release TODO Tracking

## Overview

Add TODO tracking to releases so users can acknowledge and resolve release notifications. Notification cards include action buttons, and a new web portal tab provides a centralized view of pending/acknowledged/resolved items.

## Scope

- Applies to **both** source releases and semantic releases
- **Anonymous** actions (no user tracking)
- Notification buttons: URL-based buttons in Slack/Discord, styled links in email, URLs in webhook payload
- New "Todo" sidebar tab in the web portal

## Database Schema

New `release_todos` table:

```sql
CREATE TABLE release_todos (
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
    ),
    UNIQUE(release_id),
    UNIQUE(semantic_release_id)
);
```

- 1:1 with release or semantic release (UNIQUE constraints)
- CHECK constraint ensures exactly one FK is set (same pattern as `subscriptions`)
- CASCADE deletes clean up when underlying release is removed
- Status: `pending` -> `acknowledged` -> `resolved`

## API Endpoints

### CRUD

```
GET    /api/v1/todos                    -- List (filterable: ?status=pending|acknowledged|resolved, paginated)
GET    /api/v1/todos/{id}              -- Get single todo
PATCH  /api/v1/todos/{id}/acknowledge  -- Mark acknowledged
PATCH  /api/v1/todos/{id}/resolve      -- Mark resolved
```

### One-Click (for notification buttons)

```
GET /api/v1/todos/{id}/acknowledge?redirect=true  -- Acknowledge + 302 redirect to web portal
GET /api/v1/todos/{id}/resolve?redirect=true       -- Resolve + 302 redirect to web portal
```

GET endpoints so they work as clickable links from Slack/Discord/email. The `redirect=true` param triggers a 302 redirect to the Todo page after updating.

### List Response Shape

```json
{
  "data": [{
    "id": "uuid",
    "status": "pending",
    "release_id": "uuid",
    "semantic_release_id": null,
    "project_name": "Go Runtime",
    "version": "v1.22.0",
    "provider": "github",
    "repository": "golang/go",
    "source_url": "https://github.com/golang/go/releases/tag/v1.22.0",
    "release_url": "/releases/uuid",
    "urgency": null,
    "created_at": "...",
    "acknowledged_at": null,
    "resolved_at": null
  }],
  "meta": { "page": 1, "per_page": 25, "total": 42 }
}
```

List endpoint returns enriched data via JOINs (project name, version, provider, etc.) to avoid extra round-trips.

## Notification Card Changes

### Slack

Add `"type": "actions"` block with URL buttons:

```json
{
  "type": "actions",
  "elements": [
    {"type": "button", "text": {"type": "plain_text", "text": "Acknowledge"}, "url": "{PUBLIC_URL}/api/v1/todos/{id}/acknowledge?redirect=true"},
    {"type": "button", "text": {"type": "plain_text", "text": "Resolve"}, "url": "{PUBLIC_URL}/api/v1/todos/{id}/resolve?redirect=true"}
  ]
}
```

URL buttons look native in Slack but open a browser. No Slack App required.

### Discord

Add markdown links in embed description (Discord webhooks don't support buttons without a Bot):

```
[Acknowledge](url) | [Resolve](url)
```

### Email

Add styled HTML anchor tags (button-styled) in the email template.

### Webhook

Add `acknowledge_url` and `resolve_url` fields to JSON payload.

### Passing TODO ID

`Notification` struct gets a new `TodoID string` field. `NotifyWorker` creates the TODO row first, then passes the ID to senders for URL construction.

## TODO Creation Flow

TODOs are created during notification routing:

**Source releases:** In `NotifyWorker.Work()`, before sending notifications:
1. `INSERT INTO release_todos (release_id) ... ON CONFLICT DO NOTHING`
2. Get the todo ID
3. Pass to senders via `Notification.TodoID`

**Semantic releases:** In the agent orchestrator's notification path, after completing a semantic release:
1. `INSERT INTO release_todos (semantic_release_id) ... ON CONFLICT DO NOTHING`
2. Get the todo ID
3. Pass to senders via `Notification.TodoID`

`ON CONFLICT DO NOTHING` ensures idempotency (safe for job retries).

## Frontend: Todo Tab

### Sidebar

New "Todo" entry between Dashboard and Projects, using `ListTodo` icon from lucide-react.

### Page: `/todo`

Three sub-tabs with count badges:

| Tab | Filter | Default |
|-----|--------|---------|
| Pending (N) | `status=pending` | Yes |
| Acknowledged (N) | `status=acknowledged` | |
| Resolved (N) | `status=resolved` | |

### Table Columns

| Column | Content |
|--------|---------|
| Project | Project name |
| Version | Version string (links to release detail) |
| Type | Badge: "Release" or "Semantic" |
| Provider | Provider badge |
| Urgency | Urgency badge (semantic only) |
| Created | Relative timestamp |
| Actions | Acknowledge/Resolve buttons |

### Behavior

- Pending tab: "Acknowledge" button per row
- Acknowledged tab: "Resolve" button per row
- Resolved tab: no action buttons
- Optimistic UI updates via SWR mutation
- SSE revalidation for live updates
- Standard pagination (Previous/Next)
- Follows existing styling patterns (inline styles, CSS variables, DM Sans)
