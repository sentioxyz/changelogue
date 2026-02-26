# Projects Page & Onboarding Redesign

## Problem

1. The projects page (table layout) doesn't surface sources or per-source releases — users must navigate to the project detail page to discover them.
2. Adding a source requires navigating away from the projects page to a separate form.
3. Project creation asks for agent prompt and trigger rules upfront — these are advanced features only relevant when semantic releases are needed.

## Decisions

- **Projects page layout:** Dashboard-style cards with sections (not table rows)
- **Project creation flow:** Simple form swap — replace agent fields with optional source fields
- **Add source from projects page:** Inline form within the project card (no navigation)

## Design

### 1. Projects Page — Dashboard Cards

Each project is rendered as a card with three sections stacked vertically.

**Card Header:**
- Project name (Fraunces serif, clickable link to project detail)
- Description (muted, one line)
- Right-aligned action: "Edit" link

**Sources Section:**
- Section label "Sources" with `[+ Add Source]` button on the right
- Each source as a compact row: provider icon, repository name (monospace), status dot (green=active, gray=disabled), poll interval (muted)
- Empty state: "No sources configured" with prominent `[+ Add Source]`
- **Inline add form:** `[+ Add Source]` expands a form within the card — provider dropdown, repository input, poll interval input, Add/Cancel buttons. On submit, calls `sourcesApi.create()` and refreshes via SWR mutate.

**Recent Releases Section:**
- Section label "Recent Releases"
- Latest 3-5 releases across all sources, sorted by date
- Each row: version chip (monospace), provider icon, source repo name, relative time
- Empty state: "No releases yet"
- "View all" link to project detail page

**Data Fetching:**
- `projectsApi.list()` for all projects
- Per project: `sourcesApi.listByProject(projectId)` for sources
- Per project: `releasesApi.listByProject(projectId, 1, 5)` for recent releases
- SWR for caching and revalidation

### 2. Project Creation Form

**Route:** `/projects/new` (same URL, redesigned form)

**Fields:**
1. Name (required, text input)
2. Description (optional, textarea)
3. Source section (optional, labeled "Add a Source"):
   - Provider dropdown: "GitHub" or "Docker Hub"
   - Repository text input, placeholder: "e.g. ethereum/go-ethereum"
   - Poll Interval number input, default 300, minimum 60, suffix "seconds"

**Removed from creation form:**
- Agent prompt textarea
- Agent trigger rules (on_major_release, on_minor_release, on_security_patch, version_pattern)
- These remain accessible in the project detail page's Agent tab

**Submit flow:**
1. `projectsApi.create({ name, description })` — no agent fields
2. If source fields filled: `sourcesApi.create(newProjectId, { provider, repository, poll_interval_seconds, enabled: true })`
3. If source creation fails: show error, project already created — redirect to project detail with warning
4. On success: redirect to `/projects`

### 3. Project Detail Page Adjustments

**Unchanged:**
- All four tabs (Sources, Context Sources, Semantic Releases, Agent)
- Agent tab keeps agent prompt and trigger rules editing
- Source/context source management within tabs

**Changed:**
- Edit project form (`/projects/[id]/edit`): Remove agent prompt and trigger rules — only name and description. Agent config managed via the Agent tab.

## Scope

- Frontend only — no backend API changes needed
- All existing API endpoints reused as-is
- Project detail page structure preserved (tabs)
- Source edit/delete flows unchanged
