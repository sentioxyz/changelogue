# Frontend Redesign Design — ReleaseBeacon

**Date:** 2026-02-25
**Audience:** Internal SRE / platform team
**Aesthetic:** Two-tone editorial — dark charcoal sidebar + warm white content area
**Status:** Approved, ready for implementation

---

## Overview

Full visual redesign of the ReleaseBeacon frontend. The current implementation uses shadcn defaults (Geist font, neutral gray palette, no distinctive identity). The redesign establishes an **editorial / data-journalism** aesthetic — the dark sidebar recedes as chrome, the warm white content area reads like a well-structured intelligence report.

The design is post-pivot: Sources are nested under Projects, not top-level nav items. The semantic release detail page is the editorial centerpiece.

---

## 1. Color System

Replace all shadcn CSS variable defaults in `globals.css`:

| Variable | Value | Usage |
|----------|-------|-------|
| `--background` | `#fafaf9` | Main content background |
| `--surface` | `#ffffff` | Cards, panels, header |
| `--sidebar-bg` | `#16181c` | Sidebar background |
| `--sidebar-text` | `#9ca3af` | Inactive nav item text |
| `--sidebar-active-text` | `#ffffff` | Active nav item text |
| `--text-primary` | `#111113` | Headings, primary body text |
| `--text-secondary` | `#6b7280` | Metadata, labels, descriptions |
| `--accent` | `#e8601a` | Active indicator, links, primary buttons |
| `--border` | `#e8e8e5` | All dividers and borders |
| `--mono-bg` | `#f3f3f1` | Version/repo inline chips |

**Status colors:**

| Status | Color |
|--------|-------|
| completed | `#16a34a` |
| running | `#2563eb` |
| pending | `#d97706` |
| failed | `#dc2626` |

**Urgency callout backgrounds:**

| Urgency | Background | Border |
|---------|-----------|--------|
| HIGH | `#fff8f0` | `#d97706` |
| CRITICAL | `#fff1f2` | `#dc2626` |
| LOW | none | — |

---

## 2. Typography

Three-font stack. Load via `next/font/google` (Fraunces, DM Sans) and `@fontsource/jetbrains-mono` (npm).

| Role | Font | Weight | Usage |
|------|------|--------|-------|
| Display | Fraunces (variable serif) | 600–700 | Page titles, version headings, project names, pull-quotes |
| Body | DM Sans | 400–500 | Nav labels, table rows, form fields, descriptions, section labels |
| Mono | JetBrains Mono | 400 | Versions (`v1.14.2`), repository slugs, config values |

Replace `Geist` / `Geist_Mono` throughout `layout.tsx` and `globals.css`.

---

## 3. Layout & Sidebar

**Shell structure:** unchanged (`flex h-screen` with fixed sidebar + scrollable main).

**Sidebar (200px, fixed):**
- Background: `#16181c`. No border — color contrast is the boundary.
- Logo: "ReleaseBeacon" in Fraunces italic 16px `#ffffff`, with a small filled circle icon in `#e8601a`
- Nav items: DM Sans 13px, `#9ca3af` default → `#ffffff` on hover (150ms transition)
- **Active state:** 3px left border `#e8601a` + white text + `rgba(255,255,255,0.06)` background
- 8px vertical padding per item, 16px left padding
- Fixed width — no collapse toggle (internal tool, consistent layout prioritized)

**Post-pivot nav items (5 items only):**
```
Dashboard
Projects
Releases
Channels
Subscriptions
```
Sources, context sources, semantic releases, and agent pages are nested under Projects — no top-level nav items.

**Header (48px):**
- `bg: #ffffff`, `border-bottom: 1px solid #e8e8e5`
- Left: page title or breadcrumb in DM Sans 14px medium
  - Nested pages: `Projects / Geth / Agent` (breadcrumb)
- Right: primary action button slot (contextual per page)

---

## 4. Dashboard Page

**URL:** `/`

Two-row layout:

**Row 1 — Stat strip (4 cards, horizontal):**
- Number: Fraunces 32px bold
- Label: DM Sans 12px `#6b7280`, above the number
- Icon: top-right, muted color
- Style: `bg: #ffffff`, `border: 1px solid #e8e8e5`, no shadow
- Stats: Total Releases, Active Sources, Pending Jobs, Failed Jobs

**Row 2 — Two columns:**

**Left — Recent Source Releases:**
- Clean table, no zebra stripes
- Row dividers: `1px solid #e8e8e5`
- Columns: Repository (JetBrains Mono, muted), Version chip (mono `#f3f3f1` bg), Relative date
- "View all →" in `#e8601a` in card header
- Links to `/releases/[id]`

**Right — Semantic Releases:**
- Card list (not table)
- Each item: project name (Fraunces 15px semi-bold), version chip, one-line summary excerpt (italic DM Sans 13px), status dot + age
- Status dot: 8px filled circle using status colors
- Links to `/projects/[id]/semantic-releases/[id]`

No activity feed (removed for cleaner two-signal layout).

---

## 5. Project Detail Page

**URL:** `/projects/[id]`

**Header zone** (white, `border-bottom: 1px solid #e8e8e5`, 80px tall):
- Project name: Fraunces 28px bold
- Description: DM Sans 14px `#6b7280`
- Meta row: `X sources · Y context sources · last run Z ago` in DM Sans 12px `#9ca3af`
- "Run Agent" primary button top-right: `#e8601a` bg, white text

**Four tabs** (DM Sans 13px, thin `2px underline` active indicator in `#e8601a`):

1. **Sources** — Table with columns: Provider badge, Repository (mono), Poll interval, Status dot, Last polled, Last error. "+ Add Source" button. Kebab menu per row: Edit / Delete.

2. **Context Sources** — Same table pattern. Columns: Type badge, Name, Config URL (truncated). "+ Add Context Source" button.

3. **Semantic Releases** — Card list. Each card shows: version (mono chip), status dot, age, one-line summary excerpt (italic), Urgency chip, Availability chip. "[View →]" link.

4. **Agent** — Three stacked sub-sections:
   - **Prompt:** labeled textarea with `agent_prompt` value, "Save" button
   - **Rules:** four checkboxes (On Major Release, On Minor Release, On Security Patch) + optional regex field "Version Pattern"
   - **Run History:** compact table with columns: Trigger, Status dot, Started, Duration, Semantic Release link

---

## 6. Semantic Release Detail Page

**URL:** `/projects/[id]/semantic-releases/[id]`

The editorial centerpiece. Single column, max-width 760px, centered.

**Structure (top to bottom):**

1. **Breadcrumb:** `Projects / Geth / Semantic Releases` in DM Sans 12px `#9ca3af`

2. **Byline:** Project name in Fraunces 13px muted, above version

3. **Version heading:** Fraunces 42px bold, tight letter-spacing

4. **Meta line:** status dot + "completed · generated 2h ago" in DM Sans 13px `#6b7280`

5. **Divider:** `1px solid #e8e8e5`

6. **Summary section:**
   - Label: DM Sans 11px uppercase `#9ca3af`, 0.12em letter-spacing
   - Body: DM Sans 16px, line-height 1.7, `#111113`

7. **Urgency callout** (HIGH/CRITICAL only):
   - `#fff8f0` background (amber) or `#fff1f2` (red)
   - 3px left border matching color
   - Icon + bold label + one-line description
   - DM Sans 14px

8. **Availability + Adoption cards** (side-by-side, 50/50):
   - White cards, `1px solid #e8e8e5`
   - Label + value in DM Sans

9. **Recommendation pull-quote:**
   - Fraunces italic 18px, `#16181c`
   - 3px left border `#e8601a`
   - `#fafaf9` background, padded

10. **Source Releases section:**
    - Label: uppercase DM Sans 11px
    - Clean table: Provider badge, Repository (mono), Version chip, Date

---

## 7. Remaining Pages

**Releases (`/releases`):**
- Full-width table: Project name, Provider badge, Repository (mono), Version chip, Released date, Age
- Project filter dropdown
- Read-only — no edit actions
- Row links to `/releases/[id]`

**Release Detail (`/releases/[id]`):**
- Source info (provider, repo, version)
- Raw data rendered as formatted JSON
- Linked semantic releases (if any)

**Channels (`/channels`):**
- CRUD table: Name, Type badge, Config summary, Created date, Edit/Delete
- "+ New Channel" button
- Form as slide-over sheet with type selector revealing type-specific config fields

**Subscriptions (`/subscriptions`):**
- CRUD table: Type badge (source/project), Target name, Channel name, Version filter (mono)
- "+ New Subscription" button
- Form with type selector conditionally showing source picker or project picker

---

## 8. Shared Component Patterns

| Component | Treatment |
|-----------|-----------|
| Provider badges | GitHub: `#1a1a1a` bg / white text. DockerHub: `#2496ed` / white. DM Sans 11px, pill shape |
| Type badges | Distinct color per type (Slack, Discord, Webhook, etc.) |
| Status dots | 8px filled circle. green/blue/amber/red per status |
| Version chips | JetBrains Mono 12px, `#f3f3f1` bg, `#374151` text, 4px border-radius |
| Section labels | DM Sans 11px uppercase, `#9ca3af`, letter-spacing 0.12em |
| Primary button | `#e8601a` bg, white text, DM Sans 13px medium, 6px border-radius |
| Danger button | Outline variant, `#dc2626` text/border, fills red on hover |
| Empty states | Fraunces italic 16px muted, centered, with contextual action button |
| Loading states | Skeleton shimmer bars matching content shape — no spinners |
| Tables | No zebra stripes, `1px solid #e8e8e5` row dividers, hover: `bg: #fafaf9` |

---

## 9. Motion

- Page content: fade in on mount (`opacity: 0 → 1`, 150ms ease)
- Table rows: `transition: background 100ms` on hover
- Buttons: `transition: background 120ms` on hover
- No other animation — internal ops tool, restraint over delight

---

## 10. Dependencies to Add

| Package | Purpose |
|---------|---------|
| `@fontsource/jetbrains-mono` | JetBrains Mono for mono text |
| Google Fonts: `Fraunces` | Display serif via `next/font/google` |
| Google Fonts: `DM Sans` | Body font via `next/font/google` |

Remove: `Geist`, `Geist_Mono` from `next/font/google` imports.
