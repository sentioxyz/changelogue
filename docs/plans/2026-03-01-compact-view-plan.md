# Compact View Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Add a compact view toggle to the projects page where each project is one row showing the latest release and latest semantic release.

**Architecture:** Add a `viewMode` state to `ProjectsPage`, a toggle button group in the header, and a `ProjectCompactRow` component that fetches the same SWR data as the cards but renders a single row. All changes in one file: `web/app/projects/page.tsx`.

**Tech Stack:** React, SWR, Tailwind CSS, Lucide icons, existing design system

---

### Task 1: Add view mode state and toggle icons to page header

**Files:**
- Modify: `web/app/projects/page.tsx`

**Step 1: Add LayoutGrid and List to imports**

Change line 14 from:
```tsx
import { Plus, X, ArrowRight } from "lucide-react";
```
to:
```tsx
import { Plus, X, ArrowRight, LayoutGrid, List } from "lucide-react";
```

**Step 2: Add viewMode state**

In `ProjectsPage`, after line 404 (`const [createOpen, setCreateOpen] = useState(false);`), add:
```tsx
const [viewMode, setViewMode] = useState<"cards" | "compact">("cards");
```

**Step 3: Add toggle buttons in the header**

Insert toggle buttons between the subtitle `<p>` and the "New Project" button. Replace the header `<div className="flex items-center justify-between">` block (lines 410-437) with:

```tsx
<div className="flex items-center justify-between">
  <div>
    <h1
      style={{
        fontFamily: "var(--font-fraunces)",
        fontSize: "24px",
        fontWeight: 700,
        color: "#111113",
      }}
    >
      Projects
    </h1>
    <p
      className="mt-1 text-[13px] text-[#6b7280]"
      style={{ fontFamily: "var(--font-dm-sans)" }}
    >
      Tracked software projects and their recent releases.
    </p>
  </div>
  <div className="flex items-center gap-2">
    <div className="flex items-center rounded-md border" style={{ borderColor: "#e8e8e5" }}>
      <button
        onClick={() => setViewMode("cards")}
        className="p-1.5 rounded-l-md transition-colors"
        style={{
          backgroundColor: viewMode === "cards" ? "#f3f3f1" : "transparent",
          color: viewMode === "cards" ? "#111113" : "#9ca3af",
        }}
        title="Card view"
      >
        <LayoutGrid className="h-4 w-4" />
      </button>
      <button
        onClick={() => setViewMode("compact")}
        className="p-1.5 rounded-r-md transition-colors"
        style={{
          backgroundColor: viewMode === "compact" ? "#f3f3f1" : "transparent",
          color: viewMode === "compact" ? "#111113" : "#9ca3af",
        }}
        title="Compact view"
      >
        <List className="h-4 w-4" />
      </button>
    </div>
    <button
      onClick={() => setCreateOpen(true)}
      className="flex items-center gap-1.5 rounded px-3 py-1.5 text-[13px] text-white transition-opacity hover:opacity-90"
      style={{ backgroundColor: "#e8601a", fontFamily: "var(--font-dm-sans)" }}
    >
      <Plus className="h-3.5 w-3.5" />
      New Project
    </button>
  </div>
</div>
```

**Step 4: Verify toggle renders**

Run: `cd web && npm run dev`
Open http://localhost:3000/projects — toggle icons should appear and switch active state.

**Step 5: Commit**

```bash
git add web/app/projects/page.tsx
git commit -m "feat(frontend): add view mode toggle to projects page header"
```

---

### Task 2: Add ProjectCompactRow component and wire up conditional rendering

**Files:**
- Modify: `web/app/projects/page.tsx`

**Step 1: Add `ProjectCompactRow` component**

Insert this component before the `/* ---------- Page ---------- */` comment (before line 401):

```tsx
/* ---------- Compact Row ---------- */

function ProjectCompactRow({ project }: { project: Project }) {
  const { data: relData } = useSWR(`project-${project.id}-card-releases`, () =>
    releasesApi.listByProject(project.id, 1, 5)
  );
  const { data: srcData } = useSWR(`project-${project.id}-card-sources`, () =>
    sourcesApi.listByProject(project.id)
  );
  const { data: srData } = useSWR(`project-${project.id}-card-sr`, () =>
    srApi.list(project.id, 1, 3)
  );

  const releases = relData?.data ?? [];
  const sources = srcData?.data ?? [];
  const srItems = srData?.data ?? [];
  const latest = releases[0];
  const latestSr = srItems[0];

  const sourceMap = new Map<string, Source>();
  for (const s of sources) sourceMap.set(s.id, s);

  const latestSrc = latest ? sourceMap.get(latest.source_id) : undefined;
  const LatestIcon = latestSrc ? getProviderIcon(latestSrc.provider) : undefined;

  const urgencyStyle = latestSr?.report?.urgency
    ? URGENCY_COLORS[latestSr.report.urgency.toLowerCase()]
    : undefined;

  return (
    <Link
      href={`/projects/${project.id}`}
      className="flex items-center gap-4 rounded-md px-4 py-2.5 transition-colors hover:bg-[#f9f9f7]"
      style={{ border: "1px solid #e8e8e5", backgroundColor: "#ffffff" }}
    >
      {/* Project name */}
      <span
        className="text-[14px] font-bold shrink-0"
        style={{ fontFamily: "var(--font-fraunces)", color: "#e8601a", minWidth: "160px" }}
      >
        {project.name}
      </span>

      {/* Latest release */}
      <span className="flex items-center gap-1.5 shrink-0" style={{ minWidth: "140px" }}>
        {latest ? (
          <>
            <span
              className="inline-flex items-center rounded px-1.5 py-0.5 text-[12px] leading-none"
              style={{
                backgroundColor: "#f3f3f1",
                fontFamily: "'JetBrains Mono', monospace",
                color: "#374151",
              }}
            >
              {latest.version}
            </span>
            {LatestIcon && <LatestIcon size={12} className="shrink-0" style={{ color: "#9ca3af" }} />}
          </>
        ) : (
          <span className="text-[12px] italic" style={{ color: "#c4c4c0" }}>
            No releases
          </span>
        )}
      </span>

      {/* Latest semantic release */}
      <span className="flex items-center gap-1.5 flex-1 min-w-0">
        {latestSr ? (
          <>
            <span
              className="inline-flex items-center rounded px-1.5 py-0.5 text-[12px] leading-none shrink-0"
              style={{
                backgroundColor: "#eff6ff",
                fontFamily: "'JetBrains Mono', monospace",
                color: "#1d4ed8",
              }}
            >
              {latestSr.version}
            </span>
            {urgencyStyle && (
              <span
                className="inline-flex items-center rounded-full px-1.5 py-0.5 text-[10px] font-medium leading-none shrink-0"
                style={{ backgroundColor: urgencyStyle.bg, color: urgencyStyle.text }}
              >
                {latestSr.report!.urgency}
              </span>
            )}
            {latestSr.report?.summary && (
              <span className="text-[11px] truncate" style={{ color: "#9ca3af" }}>
                {latestSr.report.summary.length > 60
                  ? latestSr.report.summary.slice(0, 60) + "\u2026"
                  : latestSr.report.summary}
              </span>
            )}
          </>
        ) : (
          <span className="text-[12px] italic" style={{ color: "#c4c4c0" }}>
            No analysis
          </span>
        )}
      </span>

      {/* Arrow */}
      <ArrowRight className="h-3.5 w-3.5 shrink-0" style={{ color: "#c4c4c0" }} />
    </Link>
  );
}
```

**Step 2: Conditional rendering based on viewMode**

Replace the project list rendering block (the `<div className="space-y-4">` around line 467) to switch between views:

Change:
```tsx
<div className="space-y-4">
  {items.map((project) => (
    <ProjectCard key={project.id} project={project} />
  ))}
</div>
```

To:
```tsx
<div className={viewMode === "cards" ? "space-y-4" : "space-y-1.5"}>
  {items.map((project) =>
    viewMode === "cards" ? (
      <ProjectCard key={project.id} project={project} />
    ) : (
      <ProjectCompactRow key={project.id} project={project} />
    )
  )}
</div>
```

**Step 3: Verify compact view renders**

Run: `cd web && npm run dev`
Open http://localhost:3000/projects — click List icon. Each project should show as a single clickable row.

**Step 4: Commit**

```bash
git add web/app/projects/page.tsx
git commit -m "feat(frontend): add compact view for projects page"
```
