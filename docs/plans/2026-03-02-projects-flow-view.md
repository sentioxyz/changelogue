# Projects Flow View Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Replace the default card view on the projects page with a compact, flowing layout that shows sources as clickable chips, semantic releases inline, and recent releases as flowing inline text.

**Architecture:** Replace the `ProjectCard` component in `web/app/projects/page.tsx` with a new `ProjectFlowCard` component. Add `SourceForm` dialog support (create + edit) reusing the existing `SourceForm` component and `Dialog` from shadcn. No API changes. The existing `ProjectCompactRow` view remains unchanged.

**Tech Stack:** Next.js (React), Tailwind CSS, SWR, existing shadcn UI components (`Dialog`, `SourceForm`)

---

### Task 1: Add ProjectFlowCard component

**Files:**
- Modify: `web/app/projects/page.tsx` (replace `ProjectCard` function, lines ~410-449)

**Step 1: Add imports needed for the dialog**

At the top of the file, the following imports already exist: `Dialog, DialogContent, DialogHeader, DialogTitle`, `Plus`, `Link`. Add `SourceForm` import:

```typescript
import { SourceForm } from "@/components/sources/source-form";
```

**Step 2: Write the `ProjectFlowCard` component**

Replace the `ProjectCard` function (lines 410-449) with this new component. Place it at the same location in the file:

```typescript
/* ---------- Flow Card ---------- */

function ProjectFlowCard({ project }: { project: Project }) {
  const [sourceCreateOpen, setSourceCreateOpen] = useState(false);
  const [editingSource, setEditingSource] = useState<Source | null>(null);

  const { data: srcData, mutate: mutateSources } = useSWR(
    `project-${project.id}-card-sources`,
    () => sourcesApi.listByProject(project.id),
  );
  const { data: relData } = useSWR(
    `project-${project.id}-card-releases`,
    () => releasesApi.listByProject(project.id, 1, 8),
  );
  const { data: srData } = useSWR(
    `project-${project.id}-card-sr`,
    () => srApi.list(project.id, 1, 3),
  );

  const sources = srcData?.data ?? [];
  const releases = relData?.data ?? [];
  const srItems = srData?.data ?? [];

  const sourceMap = new Map<string, Source>();
  for (const s of sources) sourceMap.set(s.id, s);

  return (
    <div
      className="rounded-md px-5 py-4"
      style={{ border: "1px solid #e8e8e5", backgroundColor: "#ffffff" }}
    >
      {/* Header */}
      <div className="flex items-center justify-between mb-2">
        <div className="flex items-center gap-3 min-w-0">
          <ProjectCardLogo projectId={project.id} name={project.name} />
          <div className="min-w-0">
            <Link
              href={`/projects/${project.id}`}
              className="group inline-flex items-center gap-1.5 text-[16px] font-bold transition-colors"
              style={{ fontFamily: "var(--font-fraunces)", color: "#e8601a" }}
            >
              {project.name}
              <ArrowRight className="h-3.5 w-3.5 opacity-0 -translate-x-1 transition-all group-hover:opacity-100 group-hover:translate-x-0" />
            </Link>
            {project.description && (
              <p
                className="text-[13px] truncate"
                style={{ color: "#6b7280", fontFamily: "var(--font-dm-sans)" }}
              >
                {project.description}
              </p>
            )}
          </div>
        </div>
        <button
          onClick={() => setSourceCreateOpen(true)}
          className="shrink-0 inline-flex items-center gap-1 px-2.5 py-1 text-[12px] font-medium rounded-md border transition-colors hover:bg-[#f3f3f1]"
          style={{
            borderColor: "#e8e8e5",
            color: "#6b7280",
            fontFamily: "var(--font-dm-sans)",
          }}
        >
          <Plus className="h-3 w-3" />
          Add Source
        </button>
      </div>

      {/* Sources chips */}
      {sources.length > 0 && (
        <div className="flex flex-wrap items-center gap-1.5 mb-3">
          <span
            className="text-[11px] font-medium uppercase tracking-[0.08em] mr-0.5"
            style={{ color: "#9ca3af", fontFamily: "var(--font-dm-sans)" }}
          >
            Sources:
          </span>
          {sources.map((source) => (
            <button
              key={source.id}
              onClick={() => setEditingSource(source)}
              className="inline-flex items-center gap-1.5 rounded px-2 py-0.5 text-[12px] transition-colors hover:bg-[#f0f0ee]"
              style={{
                backgroundColor: "#fafaf9",
                border: "1px solid #e8e8e5",
              }}
            >
              <span
                className="inline-block h-1.5 w-1.5 rounded-full"
                style={{ backgroundColor: source.enabled ? "#16a34a" : "#d1d5db" }}
              />
              <span style={{ fontFamily: "'JetBrains Mono', monospace", color: "#374151" }}>
                {source.repository}
              </span>
              {source.last_polled_at && (
                <span style={{ color: "#9ca3af" }}>
                  {timeAgo(source.last_polled_at).replace(" ago", "")}
                </span>
              )}
            </button>
          ))}
        </div>
      )}

      {/* Semantic releases */}
      {srItems.length > 0 && (
        <div className="flex flex-wrap items-center gap-x-2 gap-y-1 mb-2 text-[13px] leading-7">
          <span
            className="text-[11px] font-medium uppercase tracking-[0.08em]"
            style={{ color: "#e8601a", fontFamily: "var(--font-dm-sans)" }}
          >
            Semantic:
          </span>
          {srItems.map((sr) => {
            const urgencyStyle = sr.report?.urgency
              ? URGENCY_COLORS[sr.report.urgency.toLowerCase()]
              : undefined;
            return (
              <span key={sr.id} className="inline-flex items-center gap-1.5">
                <Link
                  href={`/projects/${project.id}/semantic-releases/${sr.id}`}
                  className="inline-flex items-center rounded px-1.5 py-0.5 text-[12px] leading-none hover:ring-1 hover:ring-blue-300 transition-shadow"
                  style={{
                    backgroundColor: "#eff6ff",
                    fontFamily: "'JetBrains Mono', monospace",
                    color: "#1d4ed8",
                  }}
                >
                  {sr.version}
                </Link>
                <span
                  className="rounded px-1.5 py-0.5 text-[10px] uppercase font-bold tracking-wide leading-none"
                  style={{ backgroundColor: "#f3f3f1", color: "#6b7280" }}
                >
                  {sr.status}
                </span>
                {urgencyStyle && (
                  <span
                    className="rounded px-1.5 py-0.5 text-[10px] uppercase font-bold tracking-wide leading-none"
                    style={{ backgroundColor: urgencyStyle.bg, color: urgencyStyle.text }}
                  >
                    {sr.report!.urgency}
                  </span>
                )}
              </span>
            );
          })}
        </div>
      )}

      {/* Recent releases flow */}
      {releases.length > 0 && (
        <div className="text-[13px] leading-7">
          <span
            className="text-[11px] font-medium uppercase tracking-[0.08em] mr-1.5"
            style={{ color: "#9ca3af", fontFamily: "var(--font-dm-sans)" }}
          >
            Recent:
          </span>
          {releases.map((r) => {
            const src = sourceMap.get(r.source_id);
            return (
              <span key={r.id} className="inline-flex items-baseline mr-2.5">
                <Link
                  href={`/releases/${r.id}`}
                  className="text-[#2563eb] hover:underline"
                  style={{ fontFamily: "'JetBrains Mono', monospace", fontSize: "12px" }}
                >
                  {r.version}
                </Link>
                {src && (
                  <span
                    className="text-[11px] ml-1 hidden sm:inline"
                    style={{ color: "#9ca3af" }}
                  >
                    ({src.repository.split("/").pop()})
                  </span>
                )}
                <span
                  className="text-[11px] ml-1"
                  style={{ color: "#c4c4c0" }}
                >
                  {timeAgo(r.released_at || r.created_at).replace(" ago", "")}
                </span>
              </span>
            );
          })}
          <Link
            href={`/releases?project=${project.id}`}
            className="text-[12px] font-medium hover:underline"
            style={{ color: "#e8601a" }}
          >
            more…
          </Link>
        </div>
      )}

      {/* Add Source dialog */}
      <Dialog open={sourceCreateOpen} onOpenChange={setSourceCreateOpen}>
        <DialogContent className="sm:max-w-lg">
          <DialogHeader>
            <DialogTitle>Add Source</DialogTitle>
          </DialogHeader>
          <SourceForm
            title="Add Source"
            projectId={project.id}
            onSubmit={async (input) => {
              const res = await sourcesApi.create(project.id, input);
              if (res.data?.id) sourcesApi.poll(res.data.id).catch(() => {});
            }}
            onSuccess={() => {
              setSourceCreateOpen(false);
              mutateSources();
            }}
            onCancel={() => setSourceCreateOpen(false)}
          />
        </DialogContent>
      </Dialog>

      {/* Edit Source dialog */}
      <Dialog
        open={!!editingSource}
        onOpenChange={(open) => { if (!open) setEditingSource(null); }}
      >
        <DialogContent className="sm:max-w-lg">
          <DialogHeader>
            <DialogTitle>Edit Source</DialogTitle>
          </DialogHeader>
          {editingSource && (
            <SourceForm
              key={editingSource.id}
              title="Edit Source"
              initial={editingSource}
              onSubmit={async (input) => {
                await sourcesApi.update(editingSource.id, input);
              }}
              onSuccess={() => {
                setEditingSource(null);
                mutateSources();
              }}
              onCancel={() => setEditingSource(null)}
            />
          )}
        </DialogContent>
      </Dialog>
    </div>
  );
}
```

**Step 3: Update the render section to use `ProjectFlowCard`**

In the page's render section (around line 690), change the card view reference from `ProjectCard` to `ProjectFlowCard`:

```typescript
// Change this line:
<ProjectCard key={project.id} project={project} />
// To:
<ProjectFlowCard key={project.id} project={project} />
```

Also update the spacing for the card view (around line 688):

```typescript
// Change this:
<div className={viewMode === "cards" ? "space-y-4" : "space-y-1.5"}>
// To:
<div className={viewMode === "cards" ? "space-y-5" : "space-y-1.5"}>
```

**Step 4: Remove the old `ProjectCard`, `SourcesSection`, `RecentReleasesSection`, `SemanticReleasesSection`, and `InlineSourceForm` components**

Delete these functions since they are no longer used:
- `InlineSourceForm` (lines 23-148)
- `SourcesSection` (lines 152-230)
- `RecentReleasesSection` (lines 234-304)
- `SemanticReleasesSection` (lines 315-397)
- `ProjectCard` (lines 410-449)

Keep these (still used by compact view or flow card):
- `ProjectCardLogo` (lines 401-406) — used by both views
- `ProjectCompactRow` (lines 453-564) — compact view
- `URGENCY_COLORS` (lines 308-313) — used by both views

Also remove the now-unused imports: `X` from lucide-react, `validateRepository` and `formatInterval` from format.

**Step 5: Verify the build compiles**

Run: `cd web && npx next build`
Expected: Build succeeds with no TypeScript or lint errors.

**Step 6: Manual verification**

Run: `cd web && npm run dev`
- Navigate to `/projects`
- Verify the default view shows flowing layout with sources chips, inline releases, semantic badges
- Click a source chip → edit dialog opens pre-filled
- Click "+ Add Source" → create dialog opens
- Toggle to compact view → old compact rows still work
- Click project name → navigates to project detail

**Step 7: Commit**

```bash
git add web/app/projects/page.tsx
git commit -m "feat(web): redesign projects default view to compact flowing layout

Replace card grid with inline flowing layout showing sources as clickable
chips, semantic releases with status/urgency badges, and recent releases
as flowing text with individual dates."
```
