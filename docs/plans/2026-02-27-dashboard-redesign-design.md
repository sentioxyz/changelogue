# Dashboard Redesign: Unified Timeline

**Date:** 2026-02-27
**Status:** Approved

## Problem

The current dashboard shows system-level operational metrics (Total Releases, Active Sources, Pending Jobs, Total Projects) and splits recent releases into two columns (raw releases vs semantic releases). The "Pending Jobs" metric and general ops focus doesn't serve app users who need actionable release intelligence.

## Design

### Approach: Unified Timeline

Replace the current dashboard with a stats bar + single chronological activity feed.

### Stats Bar (3 cards)

| Card | Source | Purpose |
|------|--------|---------|
| Projects Tracked | `stats.total_projects` | Scope awareness |
| Releases This Week | Count releases from past 7 days | Velocity signal |
| Needs Attention | Count semantic releases with critical/high urgency | Actionable urgency |

- Compact horizontal row, no borders, icon + large number + small label
- "Needs Attention" uses accent color (`#e8601a`) when count > 0

### Unified Activity Feed

Single chronological feed mixing raw releases and semantic releases, sorted by `created_at` desc. Full width.

**Each entry:**
- Provider icon (GitHub/Docker) or AI indicator for semantic releases
- Project name (linked) + source repository
- Version chip
- Semantic releases: 1-line summary + urgency badge (critical/high)
- Relative timestamp on the right

**Visual differentiation:**
- Raw releases: minimal card with provider badge
- Semantic releases: elevated card with accent border-left or subtle background tint, summary visible

**Pagination:** 15 items initial load, "Load more" button.

**Data:** Client-side merge of `/releases` + `/semantic-releases` via SWR, sorted by timestamp.

### Empty State

Centered message "Start by adding a project" with CTA button to `/projects` when no projects exist.

## What Changes

### Frontend (web/)
- `app/page.tsx` — rewrite dashboard page
- `components/dashboard/stats-cards.tsx` — reduce to 3 cards, add new metrics
- `components/dashboard/recent-releases.tsx` — remove (replaced by unified feed)
- New: `components/dashboard/activity-feed.tsx` — unified chronological feed component
- New: `components/dashboard/feed-item.tsx` — individual feed entry (handles both release types)
- New: `components/dashboard/empty-state.tsx` — empty state component

### Backend (optional, can defer)
- `/stats` endpoint: add `releases_this_week` and `attention_needed` fields (or compute client-side)

## What Stays the Same
- Sidebar navigation
- All other pages (projects, releases, semantic-releases, channels, subscriptions)
- Design system (colors, fonts, components)
- API client structure
