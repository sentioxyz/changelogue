# Flat UI Redesign: Dialog-Based CRUD

**Date:** 2026-02-26
**Status:** Approved

## Problem

All create/edit operations use standalone nested route pages (e.g., `/projects/new`, `/sources/[id]/edit`). This creates deep URL nesting and unnecessary page navigation for simple form interactions.

## Solution

Replace standalone CRUD pages with centered `<Dialog>` modals on their parent list pages. Use `useState`-driven dialog state (no URL routing for modals).

## Design Decisions

- **Dialog style:** Centered modal (shadcn `<Dialog>`) — already exists in UI library but unused
- **State management:** `useState` per list page (`createOpen`, `editingId`, `deletingId`)
- **Delete confirmations:** Reusable `<ConfirmDialog>` replaces native `confirm()`
- **Form reuse:** Existing form components (`ProjectForm`, `SourceForm`, `ChannelForm`, `SubscriptionForm`) refactored to remove `<Card>` wrapper, accept `onSuccess`/`onCancel` callbacks
- **No URL-backed modals:** Simplicity over deep-linking (admin tool, not consumer-facing)

## Scope

### New Components

1. **`ConfirmDialog`** — Reusable delete confirmation dialog
   - Props: `open`, `onOpenChange`, `title`, `description`, `onConfirm`, `loading`
   - Destructive variant button styling

### Form Component Refactors

Each form component:
- Remove outer `<Card>` wrapper (Dialog provides container)
- Accept `onSuccess` callback (close dialog + SWR revalidation)
- Accept `onCancel` callback (close dialog)

### List Page Enhancements

Each list page gains:
- "New" button → opens create dialog
- Row "Edit" action → opens edit dialog (pre-filled)
- Row "Delete" → opens confirm dialog
- State: `createOpen`, `editingId`, `deletingId`

### Pages to Delete (10 routes)

```
projects/new/page.tsx
projects/[id]/edit/page.tsx
projects/[id]/sources/new/page.tsx
projects/[id]/context-sources/new/page.tsx
sources/new/page.tsx
sources/[id]/edit/page.tsx
channels/new/page.tsx
channels/[id]/edit/page.tsx
subscriptions/new/page.tsx
subscriptions/[id]/edit/page.tsx
```

### Pages Unchanged

```
/ (dashboard)
/projects (list — enhanced with dialogs)
/projects/[id] (detail page — stays as full page)
/projects/[id]/semantic-releases
/projects/[id]/semantic-releases/[srId]
/projects/[id]/agent
/sources (list — enhanced with dialogs)
/channels (list — enhanced with dialogs)
/subscriptions (list — enhanced with dialogs)
/releases
/releases/[id]
```

### Edit Components to Delete

```
components/projects/project-edit.tsx
components/sources/source-edit.tsx
components/channels/channel-edit.tsx
components/subscriptions/subscription-edit.tsx
```

These wrappers just fetch data + render the form + add delete button. The list pages will handle this directly.

## Entity-Specific Notes

### Projects
- Create/edit dialog on `/projects` list page
- Project detail (`/projects/[id]`) stays as full page with tabs
- "Add Source" on project detail Sources tab opens source create dialog

### Sources
- Create/edit dialog on `/sources` list page
- Also accessible from project detail Sources tab
- `NewProjectSource` wrapper replaced by dialog on project detail

### Channels
- Create/edit dialog on `/channels` list page
- Dynamic fields by channel type (webhook, Slack, Discord) stay as-is

### Subscriptions
- Create/edit dialog on `/subscriptions` list page
- Conditional source/project scope toggle stays as-is

### Context Sources
- Create dialog accessible from project detail Context Sources tab
- No edit — only create and delete (current behavior)
