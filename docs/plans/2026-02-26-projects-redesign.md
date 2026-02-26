# Projects Page & Onboarding Redesign — Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Replace the projects table with dashboard cards showing sources/releases inline, simplify project creation to include optional source fields instead of agent config, and add inline source creation on the projects page.

**Architecture:** Frontend-only changes. No backend modifications. Three files rewritten (`projects/page.tsx`, `project-form.tsx`), one minor edit (`project-edit.tsx`), one new page file unchanged but with different `onSubmit` logic.

**Tech Stack:** Next.js, React, SWR, Tailwind CSS, Lucide icons, react-icons (FaGithub, FaDocker)

---

### Task 1: Simplify ProjectForm — Remove Agent Fields, Add Source Fields

**Files:**
- Modify: `web/components/projects/project-form.tsx` (full rewrite)

**Step 1: Rewrite ProjectForm**

Replace the entire `web/components/projects/project-form.tsx` with a form that has:
- Name (required text input)
- Description (optional textarea)
- Optional "Add a Source" section with: provider dropdown, repository input, poll interval input

The form's `onSubmit` callback now returns `{ project: ProjectInput, source?: SourceInput }` instead of just `ProjectInput`.

```tsx
"use client";

import { useState } from "react";
import { useRouter } from "next/navigation";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Textarea } from "@/components/ui/textarea";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import type { Project, ProjectInput, SourceInput } from "@/lib/api/types";
import { Plus, X } from "lucide-react";

export interface ProjectFormResult {
  project: ProjectInput;
  source?: SourceInput;
}

interface ProjectFormProps {
  initial?: Project;
  onSubmit: (result: ProjectFormResult) => Promise<void>;
  title: string;
  /** Hide the source section (used in edit mode) */
  hideSource?: boolean;
}

export function ProjectForm({ initial, onSubmit, title, hideSource }: ProjectFormProps) {
  const router = useRouter();
  const [saving, setSaving] = useState(false);
  const [name, setName] = useState(initial?.name ?? "");
  const [description, setDescription] = useState(initial?.description ?? "");
  const [error, setError] = useState("");

  /* Source fields (only for create) */
  const [showSource, setShowSource] = useState(false);
  const [provider, setProvider] = useState("github");
  const [repository, setRepository] = useState("");
  const [pollInterval, setPollInterval] = useState("300");

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    setError("");
    setSaving(true);
    try {
      const result: ProjectFormResult = {
        project: {
          name,
          description: description || undefined,
        },
      };
      if (showSource && repository.trim()) {
        result.source = {
          provider,
          repository: repository.trim(),
          poll_interval_seconds: Number(pollInterval) || 300,
          enabled: true,
        };
      }
      await onSubmit(result);
      router.push("/projects");
    } catch (err: unknown) {
      setError(err instanceof Error ? err.message : "Failed to save");
    } finally {
      setSaving(false);
    }
  };

  return (
    <Card className="mx-auto max-w-2xl">
      <CardHeader>
        <CardTitle>{title}</CardTitle>
      </CardHeader>
      <CardContent>
        <form onSubmit={handleSubmit} className="space-y-4">
          {error && <div className="rounded-md bg-red-50 p-3 text-sm text-red-700">{error}</div>}
          <div className="space-y-2">
            <Label htmlFor="name">Name</Label>
            <Input id="name" value={name} onChange={(e) => setName(e.target.value)} required />
          </div>
          <div className="space-y-2">
            <Label htmlFor="description">Description</Label>
            <Textarea id="description" value={description} onChange={(e) => setDescription(e.target.value)} rows={2} />
          </div>

          {/* Optional source section — only in create mode */}
          {!hideSource && (
            <div className="space-y-3">
              {!showSource ? (
                <button
                  type="button"
                  onClick={() => setShowSource(true)}
                  className="inline-flex items-center gap-1.5 text-[13px] font-medium transition-colors hover:opacity-80"
                  style={{ color: "#e8601a" }}
                >
                  <Plus className="h-3.5 w-3.5" />
                  Add a Source
                </button>
              ) : (
                <div className="rounded-md border p-4 space-y-3" style={{ borderColor: "#e8e8e5" }}>
                  <div className="flex items-center justify-between">
                    <Label className="text-[13px] font-medium">Add a Source</Label>
                    <button
                      type="button"
                      onClick={() => { setShowSource(false); setRepository(""); }}
                      className="text-[#9ca3af] hover:text-[#6b7280]"
                    >
                      <X className="h-4 w-4" />
                    </button>
                  </div>
                  <div className="space-y-2">
                    <Label>Provider</Label>
                    <Select value={provider} onValueChange={setProvider}>
                      <SelectTrigger><SelectValue /></SelectTrigger>
                      <SelectContent>
                        <SelectItem value="github">GitHub</SelectItem>
                        <SelectItem value="dockerhub">Docker Hub</SelectItem>
                      </SelectContent>
                    </Select>
                  </div>
                  <div className="space-y-2">
                    <Label htmlFor="repository">Repository</Label>
                    <Input
                      id="repository"
                      value={repository}
                      onChange={(e) => setRepository(e.target.value)}
                      placeholder="e.g. ethereum/go-ethereum"
                    />
                  </div>
                  <div className="space-y-2">
                    <Label htmlFor="poll_interval">Poll Interval (seconds)</Label>
                    <Input
                      id="poll_interval"
                      type="number"
                      min={60}
                      value={pollInterval}
                      onChange={(e) => setPollInterval(e.target.value)}
                    />
                  </div>
                </div>
              )}
            </div>
          )}

          <div className="flex justify-end gap-2">
            <Button type="button" variant="outline" onClick={() => router.back()}>Cancel</Button>
            <Button type="submit" disabled={saving}>{saving ? "Saving..." : "Save"}</Button>
          </div>
        </form>
      </CardContent>
    </Card>
  );
}
```

**Step 2: Run build to verify compilation**

Run: `cd web && npx next build 2>&1 | head -30`
Expected: May show errors from callers not updated yet — that's fine, proceed.

**Step 3: Commit**

```bash
git add web/components/projects/project-form.tsx
git commit -m "refactor: simplify ProjectForm — remove agent fields, add optional source"
```

---

### Task 2: Update NewProjectPage to Handle Source Creation

**Files:**
- Modify: `web/app/projects/new/page.tsx`

**Step 1: Update the onSubmit handler**

The new page must:
1. Call `projectsApi.create(result.project)` to create the project
2. If `result.source` is provided, call `sourcesApi.create(newProjectId, result.source)`

```tsx
"use client";

import { projects as projectsApi, sources as sourcesApi } from "@/lib/api/client";
import { ProjectForm } from "@/components/projects/project-form";

export default function NewProjectPage() {
  return (
    <ProjectForm
      title="Create Project"
      onSubmit={async (result) => {
        const resp = await projectsApi.create(result.project);
        if (result.source && resp.data?.id) {
          await sourcesApi.create(resp.data.id, result.source);
        }
      }}
    />
  );
}
```

**Step 2: Run build to verify**

Run: `cd web && npx next build 2>&1 | head -30`
Expected: May still fail from project-edit.tsx — proceed to next task.

**Step 3: Commit**

```bash
git add web/app/projects/new/page.tsx
git commit -m "feat: create source alongside project in onboarding flow"
```

---

### Task 3: Update ProjectEdit to Match New Form Interface

**Files:**
- Modify: `web/components/projects/project-edit.tsx`

**Step 1: Update the component**

The edit form passes `hideSource` and adapts to the new `ProjectFormResult` interface. Only sends `result.project` to the update API.

```tsx
"use client";

import useSWR from "swr";
import { projects as projectsApi } from "@/lib/api/client";
import { ProjectForm } from "@/components/projects/project-form";

export function ProjectEdit({ id }: { id: string }) {
  const { data, isLoading } = useSWR(`project-${id}`, () => projectsApi.get(id));

  if (isLoading) return <div className="py-12 text-center text-muted-foreground">Loading...</div>;
  if (!data?.data) return <div className="py-12 text-center">Project not found</div>;

  return (
    <ProjectForm
      title="Edit Project"
      initial={data.data}
      hideSource
      onSubmit={async (result) => { await projectsApi.update(id, result.project); }}
    />
  );
}
```

**Step 2: Run build to verify all form changes compile**

Run: `cd web && npx next build 2>&1 | head -30`
Expected: Should compile successfully (all callers of ProjectForm now pass correct types).

**Step 3: Commit**

```bash
git add web/components/projects/project-edit.tsx
git commit -m "refactor: update ProjectEdit for simplified form interface"
```

---

### Task 4: Rewrite Projects Page — Dashboard Cards with Inline Source Add

**Files:**
- Modify: `web/app/projects/page.tsx` (full rewrite)

This is the largest task. Replace the table layout with dashboard-style project cards.

**Step 1: Rewrite the projects page**

The new page renders each project as a card with three sections:

1. **Header** — project name (link to detail), description, edit link
2. **Sources** — compact list of sources with provider icon, repo, status dot, poll interval, and inline "Add Source" form
3. **Recent Releases** — latest 5 releases across all sources with version chip, provider icon, repo name, relative time

```tsx
"use client";

import { useState } from "react";
import useSWR, { mutate } from "swr";
import Link from "next/link";
import {
  projects as projectsApi,
  releases as releasesApi,
  sources as sourcesApi,
} from "@/lib/api/client";
import { getProviderIcon } from "@/components/ui/provider-badge";
import { timeAgo } from "@/lib/format";
import { Plus, X, Pencil } from "lucide-react";
import type { Project, Source, Release } from "@/lib/api/types";

/* ---------- Inline Add Source Form ---------- */

function InlineSourceForm({
  projectId,
  onDone,
}: {
  projectId: string;
  onDone: () => void;
}) {
  const [provider, setProvider] = useState("github");
  const [repository, setRepository] = useState("");
  const [pollInterval, setPollInterval] = useState("300");
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState("");

  const handleAdd = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!repository.trim()) return;
    setSaving(true);
    setError("");
    try {
      await sourcesApi.create(projectId, {
        provider,
        repository: repository.trim(),
        poll_interval_seconds: Number(pollInterval) || 300,
        enabled: true,
      });
      // Revalidate sources for this project
      mutate(`project-${projectId}-card-sources`);
      onDone();
    } catch (err: unknown) {
      setError(err instanceof Error ? err.message : "Failed to add source");
    } finally {
      setSaving(false);
    }
  };

  return (
    <form onSubmit={handleAdd} className="mt-2 rounded-md border p-3 space-y-2" style={{ borderColor: "#e8e8e5", backgroundColor: "#fafaf9" }}>
      {error && <div className="text-[12px] text-red-600">{error}</div>}
      <div className="flex items-center gap-2">
        <select
          value={provider}
          onChange={(e) => setProvider(e.target.value)}
          className="rounded-md border px-2 py-1 text-[12px]"
          style={{ borderColor: "#e8e8e5", fontFamily: "var(--font-dm-sans)" }}
        >
          <option value="github">GitHub</option>
          <option value="dockerhub">Docker Hub</option>
        </select>
        <input
          type="text"
          value={repository}
          onChange={(e) => setRepository(e.target.value)}
          placeholder="e.g. ethereum/go-ethereum"
          className="flex-1 rounded-md border px-2 py-1 text-[12px]"
          style={{ borderColor: "#e8e8e5", fontFamily: "'JetBrains Mono', monospace" }}
        />
        <input
          type="number"
          min={60}
          value={pollInterval}
          onChange={(e) => setPollInterval(e.target.value)}
          className="w-20 rounded-md border px-2 py-1 text-[12px]"
          style={{ borderColor: "#e8e8e5" }}
          title="Poll interval (seconds)"
        />
      </div>
      <div className="flex items-center justify-end gap-2">
        <button
          type="button"
          onClick={onDone}
          className="text-[12px] text-[#9ca3af] hover:text-[#6b7280]"
        >
          Cancel
        </button>
        <button
          type="submit"
          disabled={saving || !repository.trim()}
          className="rounded-md px-3 py-1 text-[12px] font-medium text-white disabled:opacity-40"
          style={{ backgroundColor: "#e8601a" }}
        >
          {saving ? "Adding..." : "Add"}
        </button>
      </div>
    </form>
  );
}

/* ---------- Sources Section ---------- */

function SourcesSection({ projectId }: { projectId: string }) {
  const [adding, setAdding] = useState(false);
  const { data } = useSWR(`project-${projectId}-card-sources`, () =>
    sourcesApi.listByProject(projectId)
  );
  const sources = data?.data ?? [];

  return (
    <div
      className="border-t px-4 py-3"
      style={{ borderColor: "#e8e8e5" }}
    >
      <div className="flex items-center justify-between mb-1">
        <span
          className="text-[11px] font-medium uppercase tracking-[0.08em]"
          style={{ color: "#9ca3af", fontFamily: "var(--font-dm-sans)" }}
        >
          Sources
        </span>
        {!adding && (
          <button
            onClick={() => setAdding(true)}
            className="inline-flex items-center gap-1 text-[11px] font-medium transition-colors hover:opacity-80"
            style={{ color: "#e8601a", fontFamily: "var(--font-dm-sans)" }}
          >
            <Plus className="h-3 w-3" />
            Add Source
          </button>
        )}
        {adding && (
          <button
            onClick={() => setAdding(false)}
            className="text-[#9ca3af] hover:text-[#6b7280]"
          >
            <X className="h-3.5 w-3.5" />
          </button>
        )}
      </div>

      {sources.length > 0 ? (
        <div className="space-y-1">
          {sources.map((source) => {
            const Icon = getProviderIcon(source.provider);
            return (
              <div
                key={source.id}
                className="flex items-center gap-2 text-[12px]"
                style={{ fontFamily: "var(--font-dm-sans)" }}
              >
                {Icon && <Icon size={13} className="shrink-0" style={{ color: "#6b7280" }} />}
                <span style={{ fontFamily: "'JetBrains Mono', monospace", color: "#374151" }}>
                  {source.repository}
                </span>
                <span
                  className="ml-auto flex items-center gap-1.5 text-[11px]"
                  style={{ color: "#9ca3af" }}
                >
                  <span
                    className="inline-block h-1.5 w-1.5 rounded-full"
                    style={{ backgroundColor: source.enabled ? "#16a34a" : "#d1d5db" }}
                  />
                  {source.enabled ? "Active" : "Disabled"}
                  <span className="ml-1" style={{ color: "#c4c4c0" }}>
                    {source.poll_interval_seconds < 60
                      ? `${source.poll_interval_seconds}s`
                      : source.poll_interval_seconds < 3600
                        ? `${Math.round(source.poll_interval_seconds / 60)}m`
                        : `${(source.poll_interval_seconds / 3600).toFixed(1)}h`}
                  </span>
                </span>
              </div>
            );
          })}
        </div>
      ) : (
        <p className="text-[12px] italic" style={{ color: "#c4c4c0" }}>
          No sources configured
        </p>
      )}

      {adding && (
        <InlineSourceForm projectId={projectId} onDone={() => setAdding(false)} />
      )}
    </div>
  );
}

/* ---------- Recent Releases Section ---------- */

function RecentReleasesSection({ projectId }: { projectId: string }) {
  const { data: relData } = useSWR(`project-${projectId}-card-releases`, () =>
    releasesApi.listByProject(projectId, 1, 5)
  );
  const { data: srcData } = useSWR(`project-${projectId}-card-sources`, () =>
    sourcesApi.listByProject(projectId)
  );
  const releases = relData?.data ?? [];
  const sources = srcData?.data ?? [];

  const sourceMap = new Map<string, Source>();
  for (const s of sources) sourceMap.set(s.id, s);

  return (
    <div className="border-t px-4 py-3" style={{ borderColor: "#e8e8e5" }}>
      <span
        className="text-[11px] font-medium uppercase tracking-[0.08em] mb-1 block"
        style={{ color: "#9ca3af", fontFamily: "var(--font-dm-sans)" }}
      >
        Recent Releases
      </span>

      {releases.length > 0 ? (
        <div className="space-y-1">
          {releases.map((r) => {
            const src = sourceMap.get(r.source_id);
            const Icon = src ? getProviderIcon(src.provider) : undefined;
            return (
              <Link
                key={r.id}
                href={`/releases/${r.id}`}
                className="flex items-center gap-2 text-[12px] transition-colors hover:bg-[#fafaf9] rounded px-1 -mx-1 py-0.5"
              >
                <span
                  className="inline-flex items-center rounded px-1.5 py-0.5 text-[12px] leading-none"
                  style={{
                    backgroundColor: "#f3f3f1",
                    fontFamily: "'JetBrains Mono', monospace",
                    color: "#374151",
                  }}
                >
                  {r.version}
                </span>
                {Icon && <Icon size={11} className="shrink-0" style={{ color: "#9ca3af" }} />}
                {src && (
                  <span className="text-[11px] truncate" style={{ color: "#9ca3af" }}>
                    {src.repository}
                  </span>
                )}
                <span className="ml-auto text-[11px] shrink-0" style={{ color: "#c4c4c0" }}>
                  {timeAgo(r.released_at || r.created_at)}
                </span>
              </Link>
            );
          })}
          <Link
            href={`/projects/${projectId}`}
            className="block text-[11px] mt-1 hover:underline"
            style={{ color: "#e8601a", fontFamily: "var(--font-dm-sans)" }}
          >
            View all &rarr;
          </Link>
        </div>
      ) : (
        <p className="text-[12px] italic" style={{ color: "#c4c4c0" }}>
          No releases yet
        </p>
      )}
    </div>
  );
}

/* ---------- Project Card ---------- */

function ProjectCard({ project }: { project: Project }) {
  return (
    <div
      className="overflow-hidden rounded-md"
      style={{ border: "1px solid #e8e8e5", backgroundColor: "#ffffff" }}
    >
      {/* Header */}
      <div className="flex items-start justify-between px-4 py-3">
        <div className="min-w-0 flex-1">
          <Link
            href={`/projects/${project.id}`}
            className="text-[16px] font-bold hover:underline"
            style={{ fontFamily: "var(--font-fraunces)", color: "#111113" }}
          >
            {project.name}
          </Link>
          {project.description && (
            <p
              className="mt-0.5 text-[13px] truncate"
              style={{ color: "#6b7280", fontFamily: "var(--font-dm-sans)" }}
            >
              {project.description}
            </p>
          )}
        </div>
        <Link
          href={`/projects/${project.id}/edit`}
          className="ml-2 shrink-0 inline-flex items-center gap-1 rounded-md border px-2 py-1 text-[12px] font-medium transition-colors hover:bg-[#f3f3f1]"
          style={{
            fontFamily: "var(--font-dm-sans)",
            borderColor: "#e8e8e5",
            color: "#6b7280",
          }}
        >
          <Pencil className="h-3 w-3" />
          Edit
        </Link>
      </div>

      {/* Sources */}
      <SourcesSection projectId={project.id} />

      {/* Recent Releases */}
      <RecentReleasesSection projectId={project.id} />
    </div>
  );
}

/* ---------- Page ---------- */

export default function ProjectsPage() {
  const { data, isLoading } = useSWR("projects", () => projectsApi.list());
  const items = data?.data ?? [];

  return (
    <div className="flex flex-col gap-4 fade-in">
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
        <Link
          href="/projects/new"
          className="flex items-center gap-1.5 rounded px-3 py-1.5 text-[13px] text-white transition-opacity hover:opacity-90"
          style={{ backgroundColor: "#e8601a", fontFamily: "var(--font-dm-sans)" }}
        >
          <Plus className="h-3.5 w-3.5" />
          New Project
        </Link>
      </div>

      {isLoading ? (
        <p
          className="px-4 py-8 text-center text-[14px] italic text-[#9ca3af]"
          style={{ fontFamily: "var(--font-fraunces)" }}
        >
          Loading...
        </p>
      ) : items.length === 0 ? (
        <div
          className="flex flex-col items-center justify-center rounded-md border py-12"
          style={{ borderColor: "#e8e8e5", backgroundColor: "#ffffff" }}
        >
          <p
            className="text-[14px] italic text-[#9ca3af]"
            style={{ fontFamily: "var(--font-fraunces)" }}
          >
            No projects yet — create one to start tracking releases
          </p>
          <Link
            href="/projects/new"
            className="mt-4 flex items-center gap-1.5 rounded px-3 py-1.5 text-[13px] text-white transition-opacity hover:opacity-90"
            style={{ backgroundColor: "#e8601a", fontFamily: "var(--font-dm-sans)" }}
          >
            <Plus className="h-3.5 w-3.5" />
            New Project
          </Link>
        </div>
      ) : (
        <div className="space-y-4">
          {items.map((project) => (
            <ProjectCard key={project.id} project={project} />
          ))}
        </div>
      )}
    </div>
  );
}
```

**Step 2: Run build to verify**

Run: `cd web && npx next build 2>&1 | head -40`
Expected: Successful compilation.

**Step 3: Commit**

```bash
git add web/app/projects/page.tsx
git commit -m "feat: redesign projects page with dashboard cards, inline source add"
```

---

### Task 5: Verify Everything Works End-to-End

**Step 1: Run full build**

Run: `cd web && npx next build`
Expected: Successful build with no errors.

**Step 2: Visual verification**

Run: `cd /Users/pc/web3/ReleaseBeacon && make dev`

Then manually verify:
1. `/projects` — shows dashboard cards per project with sources section, releases section, inline add source form
2. `/projects/new` — shows simplified form with name, description, optional source fields (no agent config)
3. `/projects/{id}/edit` — shows simplified form with just name and description (no source section, no agent fields)
4. `/projects/{id}` — detail page unchanged, Agent tab still has agent prompt and trigger rules
5. Adding a source inline on the projects page works and refreshes the card

**Step 3: Commit (if any fixes needed)**

```bash
git add -A
git commit -m "fix: address issues found during verification"
```
