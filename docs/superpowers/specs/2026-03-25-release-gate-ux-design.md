# Release Gate UX Design

## Goal

Add a "Release Gate" tab to the project detail page that lets users configure release gates and monitor version readiness status and gate events.

## Context

The release gate backend is fully implemented (17 tasks completed). The backend provides:
- `GET/PUT/DELETE /api/v1/projects/{id}/release-gate` — gate CRUD
- `GET /api/v1/projects/{id}/version-readiness` — paginated list
- `GET /api/v1/projects/{id}/version-readiness/{version}` — single version
- `GET /api/v1/projects/{id}/version-readiness/{version}/events` — events for a version
- `GET /api/v1/projects/{id}/gate-events` — all gate events, paginated

This spec covers the frontend only — TypeScript types, API client, i18n, and the new tab component.

## Architecture

A new extracted component `<ReleaseGateTab>` renders inside the project detail page when the "Release Gate" tab is active. The parent (`project-detail.tsx`) adds the tab key and passes `projectId` and `sources` as props. All gate-specific state and data fetching lives inside the child component.

### File Structure

| File | Action | Purpose |
|------|--------|---------|
| `web/lib/api/types.ts` | Modify | Add `ReleaseGate`, `VersionMapping`, `VersionReadiness`, `GateEvent`, `ReleaseGateInput` types |
| `web/lib/api/client.ts` | Modify | Add `gates` namespace with 7 methods |
| `web/lib/i18n/messages/en.json` | Modify | Add `projects.detail.tabGates` and ~30 gate-related i18n keys |
| `web/lib/i18n/messages/zh.json` | Modify | Add corresponding Chinese translations |
| `web/components/projects/release-gate-tab.tsx` | Create | New component — gate config form + readiness table + events timeline |
| `web/components/projects/project-detail.tsx` | Modify | Add `"gates"` to `TabKey`, add tab entry, render `<ReleaseGateTab>` |

## TypeScript Types

Add to `web/lib/api/types.ts`:

```typescript
export interface VersionMapping {
  pattern: string;
  template: string;
}

export interface ReleaseGate {
  id: string;
  project_id: string;
  required_sources?: string[];
  timeout_hours: number;
  version_mapping?: Record<string, VersionMapping>;
  nl_rule?: string;
  enabled: boolean;
  created_at: string;
  updated_at: string;
}

export interface ReleaseGateInput {
  required_sources?: string[];
  timeout_hours: number;
  version_mapping?: Record<string, VersionMapping>;
  nl_rule?: string;
  enabled: boolean;
}

export interface VersionReadiness {
  id: string;
  project_id: string;
  version: string;
  status: "pending" | "ready" | "timed_out";
  sources_met: string[];
  sources_missing: string[];
  nl_rule_passed?: boolean;
  timeout_at: string;
  opened_at?: string;
  agent_triggered: boolean;
  created_at: string;
  updated_at: string;
}

export interface GateEvent {
  id: string;
  version_readiness_id: string;
  project_id: string;
  version: string;
  event_type: string;
  source_id?: string;
  details?: Record<string, unknown>;
  created_at: string;
}
```

## API Client

Add to `web/lib/api/client.ts`:

```typescript
export const gates = {
  get: async (projectId: string) => {
    try {
      return await request<ApiResponse<ReleaseGate>>(`/projects/${projectId}/release-gate`);
    } catch (e) {
      if (e instanceof Error && e.message.includes("404")) {
        return { data: null as unknown as ReleaseGate };
      }
      throw e;
    }
  },
  upsert: (projectId: string, input: ReleaseGateInput) =>
    request<ApiResponse<ReleaseGate>>(`/projects/${projectId}/release-gate`, {
      method: "PUT",
      body: JSON.stringify(input),
    }),
  delete: (projectId: string) =>
    request<ApiResponse<null>>(`/projects/${projectId}/release-gate`, {
      method: "DELETE",
    }),
  listReadiness: (projectId: string, page = 1, perPage = 25) =>
    request<ApiResponse<VersionReadiness[]>>(
      `/projects/${projectId}/version-readiness?page=${page}&per_page=${perPage}`
    ),
  getReadiness: (projectId: string, version: string) =>
    request<ApiResponse<VersionReadiness>>(
      `/projects/${projectId}/version-readiness/${encodeURIComponent(version)}`
    ),
  listEvents: (projectId: string, page = 1, perPage = 25) =>
    request<ApiResponse<GateEvent[]>>(
      `/projects/${projectId}/gate-events?page=${page}&per_page=${perPage}`
    ),
  listEventsByVersion: (projectId: string, version: string, page = 1, perPage = 25) =>
    request<ApiResponse<GateEvent[]>>(
      `/projects/${projectId}/version-readiness/${encodeURIComponent(version)}/events?page=${page}&per_page=${perPage}`
    ),
};
```

Note: `encodeURIComponent(version)` is needed because versions like `v2.1.0` are used as URL path segments.

## Component: ReleaseGateTab

### Props

```typescript
interface ReleaseGateTabProps {
  projectId: string;
  sources: Source[];  // project's sources, for the required sources checklist and version mapping
}
```

### Layout (Stacked Sections)

Three cards stacked vertically in a `div.space-y-6`, matching the Agent tab pattern:

#### Section 1: Gate Configuration Card

A `rounded-lg border p-5 bg-surface` card containing:

1. **Header row**: "Gate Configuration" title + description on the left, `<Switch>` toggle (enabled/disabled) on the right
2. **Required Sources**: Checkbox list showing each of the project's sources. Each checkbox shows `{provider}/{repository}`. Empty selection means "all sources required". Helper text explains this.
3. **Timeout (hours)**: `<Input type="number">` with label. Default: 168 (7 days), matching the database default.
4. **NL Rule**: `<Textarea>` with label and helper text. Optional.
5. **Version Mapping**: A table (header row + data rows) with columns:
   - Source (display name from sources list)
   - Pattern (regex input)
   - Template (template input)
   - Remove button (x icon)
   Only shows rows for sources that have custom mappings. An "Add mapping" button below lets the user pick a source (from a dropdown of unmapped sources) and add a row.
6. **Action buttons**: "Delete Gate" (destructive outline) and "Save Configuration" (primary). Delete shows a `<ConfirmDialog>`.

**Data fetching**: `useSWR` with key `project-${projectId}-gate` calling `gates.get(projectId)`. The standard `request()` function throws on 404. To handle the "no gate" case, the `gates.get` method must catch 404 errors and return `null` instead of throwing. Implement this by wrapping the `request` call in a try/catch that checks for "404" in the error message and returns `{ data: null }` on 404. The SWR data will then be `null` when no gate exists.

**Save behavior**: Calls `gates.upsert(projectId, input)` then mutates the SWR cache. If no gate exists, this creates one. If one exists, this updates it.

**Delete behavior**: Calls `gates.delete(projectId)` then mutates the SWR cache to null.

**Form state**: Local `useState` for each field, initialized from the SWR data. Reset when SWR data changes.

#### Section 2: Version Readiness Table

A card with a `<Table>` showing paginated version readiness entries. Columns:

| Column | Content |
|--------|---------|
| Version | Version string, bold |
| Status | Badge: pending (amber), ready (green), timed_out (red) |
| Sources Met | Comma-separated source display names |
| Sources Missing | Comma-separated source display names, muted color |
| Timeout | Relative time remaining for pending, "expired" for timed_out, dash for ready |
| Actions | "Events" link that expands/navigates to version-specific events |

**Data fetching**: `useSWR` with key `project-${projectId}-readiness` calling `gates.listReadiness(projectId)`. Only fetches when a gate exists and is enabled.

**Pagination**: Simple "Load more" button or page navigation matching existing patterns in the codebase.

**Empty state**: "No versions tracked yet" message when the list is empty.

#### Section 3: Gate Events Timeline

A card showing a chronological list of gate events. Each event rendered as:

- **Colored dot** on the left — color mapped by backend `event_type` values:
  - `gate_opened` → green dot (gate ready, all sources met)
  - `source_met` → blue dot (a source released its version)
  - `gate_timed_out` → amber dot (gate timed out waiting)
  - `nl_eval_passed` → green dot
  - `nl_eval_failed` → red dot
  - `nl_eval_started` → gray dot
  - `agent_triggered` → blue dot
  - Any other `event_type` → gray dot
- **Event description**: Bold version + human-readable description derived from `event_type` and `source_id`
- **Metadata line**: Event type string + relative timestamp

**Data fetching**: `useSWR` with key `project-${projectId}-gate-events` calling `gates.listEvents(projectId)`. Only fetches when a gate exists.

**Pagination**: "Load more" button.

**Empty state**: "No gate events yet" message.

### Source Display Names

The component receives `sources: Source[]` as a prop. To display human-readable source names, build a lookup map: `Record<string, string>` mapping `source.id` to `${source.provider}/${source.repository}`. Use this for:
- Required sources checkbox labels
- Version mapping source column
- Readiness table sources met/missing columns
- Event descriptions (when `source_id` is present)

## Integration in project-detail.tsx

### Changes Required

1. **TabKey type**: Add `"gates"` to the union: `type TabKey = "sources" | "context" | "agent" | "gates"`
2. **Tabs array**: Add `{ key: "gates", label: t("projects.detail.tabGates") }`
3. **Tab content**: Add conditional render block:
   ```tsx
   {activeTab === "gates" && (
     <ReleaseGateTab
       projectId={id}
       sources={sourcesData?.data ?? []}
     />
   )}
   ```
4. **Import**: Add `import { ReleaseGateTab } from "./release-gate-tab"`

No new SWR hooks in the parent — all gate data fetching happens inside `<ReleaseGateTab>`.

## i18n Keys

### English (`en.json`)

```json
"projects.detail.tabGates": "Release Gate",
"projects.detail.gateConfig": "Gate Configuration",
"projects.detail.gateConfigDesc": "Delay agent analysis until all required sources release the same version.",
"projects.detail.gateEnabled": "Enabled",
"projects.detail.gateRequiredSources": "Required Sources",
"projects.detail.gateRequiredSourcesHint": "Select which sources must all release a version before the agent runs. Leave empty to require all sources.",
"projects.detail.gateTimeoutHours": "Timeout (hours)",
"projects.detail.gateTimeoutHoursHint": "How long to wait for all sources before timing out.",
"projects.detail.gateNLRule": "Natural Language Rule",
"projects.detail.gateNLRuleOptional": "(optional)",
"projects.detail.gateNLRuleHint": "Extra constraint evaluated by AI. E.g., \"Only proceed if the Docker image tag is a stable release.\"",
"projects.detail.gateVersionMapping": "Version Mapping",
"projects.detail.gateVersionMappingHint": "Define how to normalize version strings per source for comparison.",
"projects.detail.gateVMSource": "Source",
"projects.detail.gateVMPattern": "Pattern (regex)",
"projects.detail.gateVMTemplate": "Template",
"projects.detail.gateAddMapping": "Add Mapping",
"projects.detail.gateSave": "Save Configuration",
"projects.detail.gateSaving": "Saving...",
"projects.detail.gateDelete": "Delete Gate",
"projects.detail.gateDeleteConfirm": "Delete Release Gate",
"projects.detail.gateDeleteConfirmDesc": "This will remove the release gate configuration. Version readiness data will be preserved.",
"projects.detail.gateNoConfig": "No release gate configured. Click Save to create one.",
"projects.detail.versionReadiness": "Version Readiness",
"projects.detail.versionReadinessDesc": "Track which versions are waiting for sources and which are ready.",
"projects.detail.vrVersion": "Version",
"projects.detail.vrStatus": "Status",
"projects.detail.vrSourcesMet": "Sources Met",
"projects.detail.vrSourcesMissing": "Sources Missing",
"projects.detail.vrTimeout": "Timeout",
"projects.detail.vrPending": "pending",
"projects.detail.vrReady": "ready",
"projects.detail.vrTimedOut": "timed out",
"projects.detail.vrExpired": "expired",
"projects.detail.vrEvents": "Events",
"projects.detail.vrEmpty": "No versions tracked yet.",
"projects.detail.gateEvents": "Gate Events",
"projects.detail.gateEventsDesc": "Audit log of gate activity across all versions.",
"projects.detail.gateEventsEmpty": "No gate events yet.",
"projects.detail.gateEventGateOpened": "Gate opened: all sources ready",
"projects.detail.gateEventSourceMet": "Source released: {source}",
"projects.detail.gateEventTimedOut": "Gate timed out",
"projects.detail.gateEventNLStarted": "NL rule evaluation started",
"projects.detail.gateEventNLPassed": "NL rule passed",
"projects.detail.gateEventNLFailed": "NL rule failed",
"projects.detail.gateEventAgentTriggered": "Agent analysis triggered"
```

### Chinese (`zh.json`)

Corresponding translations for all keys above. Follow the existing translation style in the file.

## Error Handling

- **404 on GET gate**: Treat as "no gate configured" — show empty config form with defaults (enabled=false, timeout=168, empty sources). The `gates.get` method handles this by catching 404 and returning null data.
- **Save failure**: Show error toast/inline error message.
- **Delete failure**: Show error in ConfirmDialog.
- **Readiness/Events fetch failure**: Show error state in the respective card.

## Edge Cases

- **No sources on project**: The required sources checklist and version mapping are empty. Show a message like "Add sources to configure gate requirements."
- **Gate disabled**: Config form is still editable. Readiness and events sections show a muted "Gate is disabled" message instead of fetching data.
- **Version mapping for deleted source**: If a source ID in version_mapping no longer exists in the sources list, show the raw ID with a "(deleted)" suffix.

## UI Components Used

From `web/components/ui/`:
- `Switch` — enabled toggle
- `Checkbox` — required sources selection
- `Input` — timeout hours, version mapping pattern/template
- `Textarea` — NL rule
- `Button` — save, delete, add mapping
- `Badge` — readiness status
- `Table` — readiness table
- `SectionLabel` — section headings
- `ConfirmDialog` — delete confirmation
- `Select` — source picker for adding version mapping rows
- `StatusDot` — event timeline dots

## Non-Goals

- Real-time updates via SSE for readiness changes (can be added later)
- Version-specific event detail page (events shown inline or in expandable rows)
- Editing version readiness entries (read-only monitoring)
