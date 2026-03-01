# Compact View for Projects Page

## Overview

Add a compact view toggle to the projects page. Users can switch between the existing card view and a new compact view where each project is a single row showing the latest release and latest semantic release.

## Toggle UI

- Two icon buttons (LayoutGrid + List) in the page header, next to "New Project"
- Active view gets highlighted background
- State: `useState<'cards' | 'compact'>('cards')` — no persistence, defaults to cards

## Compact Row Layout

```
┌──────────────────────────────────────────────────────────────────┐
│ Project Name    │  v1.2.3 [GitHub]  │  v1.2.3 🔴 High — summary │ →
└──────────────────────────────────────────────────────────────────┘
```

- **Left:** Project name (clickable, links to project detail)
- **Middle:** Latest release version + provider icon
- **Right:** Latest semantic release version + urgency badge + truncated summary
- **Far right:** Arrow icon for navigation
- Muted placeholders when no data exists

## Data Fetching

No changes — reuses existing SWR data fetched per project card. Compact row renders `releases[0]` and `semanticReleases[0]`.

## Implementation Approach

Inline in `projects/page.tsx` — conditional render based on view mode state. No new files needed beyond the page modification itself.
