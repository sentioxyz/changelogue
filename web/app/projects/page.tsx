"use client";

import React, { useState, useMemo, useRef, useEffect, useCallback } from "react";
import useSWR, { mutate } from "swr";
import Link from "next/link";
import { useRouter } from "next/navigation";
import {
  projects as projectsApi,
  releases as releasesApi,
  sources as sourcesApi,
  semanticReleases as srApi,
} from "@/lib/api/client";
import { getProviderIcon } from "@/components/ui/provider-badge";
import { ProjectLogo } from "@/components/ui/project-logo";
import { timeAgo } from "@/lib/format";
import { Plus, ArrowRight, LayoutGrid, List, Search, Pencil, ArrowUpDown } from "lucide-react";
import { SourceForm } from "@/components/sources/source-form";
import { Dialog, DialogContent, DialogHeader, DialogTitle } from "@/components/ui/dialog";
import { ProjectForm } from "@/components/projects/project-form";
import type { Project, Source } from "@/lib/api/types";

const URGENCY_COLORS: Record<string, { bg: string; text: string }> = {
  critical: { bg: "#dc2626", text: "#ffffff" },
  high: { bg: "#f97316", text: "#ffffff" },
  medium: { bg: "#f59e0b", text: "#ffffff" },
  low: { bg: "#6b7280", text: "#ffffff" },
};

/* ---------- Project Card Logo ---------- */

function ProjectCardLogo({ projectId, name }: { projectId: string; name: string }) {
  const { data } = useSWR(`project-${projectId}-card-sources`, () =>
    sourcesApi.listByProject(projectId)
  );
  return <ProjectLogo name={name} sources={data?.data} size={40} />;
}

/* ---------- Overflow Flow ---------- */

const LINE_HEIGHT = 28; // matches leading-7 (1.75rem = 28px)
const MAX_LINES = 2;
const MAX_HEIGHT = LINE_HEIGHT * MAX_LINES;

function FlowSection({
  label,
  moreHref,
  children,
}: {
  label: string;
  moreHref: string;
  children: React.ReactNode;
}) {
  const measureRef = useRef<HTMLDivElement>(null);
  const moreRef = useRef<HTMLSpanElement>(null);
  const [visibleCount, setVisibleCount] = useState<number | null>(null);

  const items = React.Children.toArray(children);

  const measure = useCallback(() => {
    const el = measureRef.current;
    if (!el) return;
    const nodes = el.children;
    const itemNodeCount = nodes.length - 2; // exclude label (first) and hidden "more…" span (last)
    // Check if everything fits without overflow
    if (el.scrollHeight <= MAX_HEIGHT + 4) {
      setVisibleCount(items.length);
      return;
    }
    // Measure actual "more…" link width
    const moreWidth = moreRef.current ? moreRef.current.offsetWidth + 8 : 52;
    const containerWidth = el.offsetWidth;
    // First pass: find all items that fit within MAX_HEIGHT (2 lines)
    let lastFitting = 0;
    for (let i = 1; i <= itemNodeCount; i++) {
      const node = nodes[i] as HTMLElement;
      if (node.offsetTop + node.offsetHeight > MAX_HEIGHT) break;
      lastFitting = i;
    }
    // Second pass: walk backward from lastFitting until "more…" fits after the item
    let count = lastFitting;
    while (count > 1) {
      const node = nodes[count] as HTMLElement;
      const nodeBottom = node.offsetTop + node.offsetHeight;
      // If this item is on line 1, "more…" can always wrap to line 2 — we're fine
      if (nodeBottom <= LINE_HEIGHT) break;
      // Item is on line 2 — check if "more…" fits after it on the same line
      const nodeRight = node.offsetLeft + node.offsetWidth;
      if (nodeRight + moreWidth <= containerWidth) break;
      count--;
    }
    setVisibleCount(Math.max(1, count));
  }, [items.length]);

  useEffect(() => {
    measure();
    const obs = new ResizeObserver(measure);
    if (measureRef.current) obs.observe(measureRef.current);
    return () => obs.disconnect();
  }, [measure, items.length]);

  const hasOverflow = visibleCount !== null && visibleCount < items.length;
  const displayed = visibleCount !== null ? items.slice(0, visibleCount) : items;

  return (
    <div className="relative">
      {/* Hidden measurer — renders all items to find cutoff */}
      <div
        ref={measureRef}
        className="text-[13px] leading-7"
        style={{ position: "absolute", visibility: "hidden", top: 0, left: 0, right: 0, pointerEvents: "none" }}
        aria-hidden="true"
      >
        <span className="text-[11px] font-medium uppercase tracking-[0.08em] mr-1.5">
          {label}:
        </span>
        {items}
        {/* Hidden "more…" to measure its actual width */}
        <span ref={moreRef} className="inline-flex items-baseline text-[12px] font-medium whitespace-nowrap">
          more…
        </span>
      </div>
      {/* Visible section */}
      <div className="text-[13px] leading-7">
        <span
          className="text-[11px] font-medium uppercase tracking-[0.08em] mr-1.5"
          style={{ color: "#9ca3af", fontFamily: "var(--font-dm-sans)" }}
        >
          {label}:
        </span>
        {displayed}
        {hasOverflow && (
          <Link
            href={moreHref}
            className="inline-flex items-baseline text-[12px] font-medium hover:underline whitespace-nowrap"
            style={{ color: "#e8601a" }}
          >
            more…
          </Link>
        )}
      </div>
    </div>
  );
}

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
    () => releasesApi.listByProject(project.id, 1, 25, true),
  );
  const { data: srData } = useSWR(
    `project-${project.id}-card-sr`,
    () => srApi.list(project.id, 1, 10),
  );

  const sources = srcData?.data ?? [];
  const releases = relData?.data ?? [];
  const srItems = srData?.data ?? [];

  const sourceMap = useMemo(() => {
    const m = new Map<string, Source>();
    for (const s of sources) m.set(s.id, s);
    return m;
  }, [sources]);

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
      </div>

      {/* Sources chips */}
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
            className="group/chip inline-flex items-center gap-1.5 rounded px-2 py-0.5 text-[12px] transition-colors hover:bg-[#f0f0ee]"
            style={{
              backgroundColor: "#fafaf9",
              border: "1px solid #e8e8e5",
            }}
          >
            <span
              className="inline-block h-1.5 w-1.5 rounded-full shrink-0"
              style={{ backgroundColor: source.enabled ? "#16a34a" : "#d1d5db" }}
            />
            {(() => { const Icon = getProviderIcon(source.provider); return Icon ? <Icon size={12} className="shrink-0" style={{ color: "#9ca3af" }} /> : null; })()}
            <span style={{ fontFamily: "'JetBrains Mono', monospace", color: "#374151" }}>
              {source.repository}
            </span>
            {source.last_polled_at && (
              <span style={{ color: "#9ca3af" }}>
                {timeAgo(source.last_polled_at).replace(" ago", "")}
              </span>
            )}
            <Pencil className="h-2.5 w-2.5 hidden group-hover/chip:inline shrink-0" style={{ color: "#9ca3af", opacity: 0.5 }} />
          </button>
        ))}
        <button
          onClick={() => setSourceCreateOpen(true)}
          className="inline-flex items-center gap-1 px-2 py-0.5 text-[12px] font-medium rounded border transition-colors hover:bg-[#f3f3f1]"
          style={{
            borderColor: "#e8e8e5",
            color: "#9ca3af",
            fontFamily: "var(--font-dm-sans)",
          }}
        >
          <Plus className="h-3 w-3" />
          Add Source
        </button>
      </div>

      {/* Releases flow */}
      {releases.length > 0 && (
        <FlowSection label="Releases" moreHref={`/releases?project=${project.id}`}>
          {releases.map((r) => {
            const src = sourceMap.get(r.source_id);
            return (
              <span key={r.id} className="inline-flex items-baseline mr-2.5">
                <Link
                  href={`/releases/${r.id}`}
                  className={r.excluded ? "" : "text-[#2563eb] hover:underline"}
                  style={{
                    fontFamily: "'JetBrains Mono', monospace",
                    fontSize: "12px",
                    ...(r.excluded ? { color: "#c4c4c0" } : {}),
                  }}
                >
                  {r.version}
                </Link>
                {src && (
                  <span
                    className="text-[11px] ml-1 hidden sm:inline"
                    style={{ color: r.excluded ? "#dcdcda" : "#9ca3af" }}
                  >
                    ({src.repository.split("/").pop()})
                  </span>
                )}
                <span
                  className="text-[11px] ml-1"
                  style={{ color: r.excluded ? "#dcdcda" : "#c4c4c0" }}
                >
                  {timeAgo(r.released_at || r.created_at).replace(" ago", "")}
                </span>
              </span>
            );
          })}
        </FlowSection>
      )}

      {/* Semantic releases */}
      {srItems.length > 0 && (
        <FlowSection label="Semantic Releases" moreHref={`/semantic-releases?project=${project.id}`}>
          {srItems.map((sr) => {
            const urgencyStyle = sr.report?.urgency
              ? URGENCY_COLORS[sr.report.urgency.toLowerCase()]
              : undefined;
            return (
              <span key={sr.id} className="inline-flex items-center gap-1.5 mr-2.5">
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
                {urgencyStyle && (
                  <span
                    className="rounded px-1.5 py-0.5 text-[10px] uppercase font-bold tracking-wide leading-none"
                    style={{ backgroundColor: urgencyStyle.bg, color: urgencyStyle.text }}
                  >
                    {sr.report?.urgency}
                  </span>
                )}
              </span>
            );
          })}
        </FlowSection>
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

/* ---------- Compact Row ---------- */

function ProjectCompactRow({ project }: { project: Project }) {
  const router = useRouter();
  const { data: relData } = useSWR(`project-${project.id}-card-releases`, () =>
    releasesApi.listByProject(project.id, 1, 5, true)
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

  const sourceMap = useMemo(() => {
    const m = new Map<string, Source>();
    for (const s of sources) m.set(s.id, s);
    return m;
  }, [sources]);

  const latestSrc = latest ? sourceMap.get(latest.source_id) : undefined;
  const latestIcon = latestSrc ? getProviderIcon(latestSrc.provider) : undefined;

  const urgencyStyle = latestSr?.report?.urgency
    ? URGENCY_COLORS[latestSr.report.urgency.toLowerCase()]
    : undefined;

  return (
    <div
      onClick={() => router.push(`/projects/${project.id}`)}
      className="flex items-center gap-4 rounded-md px-4 py-2.5 transition-colors hover:bg-[#f9f9f7] cursor-pointer"
      style={{ border: "1px solid #e8e8e5", backgroundColor: "#ffffff" }}
    >
      {/* Project name */}
      <span
        className="flex items-center gap-2 text-[14px] font-bold shrink-0"
        style={{ fontFamily: "var(--font-fraunces)", color: "#e8601a", minWidth: "160px" }}
      >
        <ProjectLogo name={project.name} sources={sources} size={24} />
        {project.name}
      </span>

      {/* Latest release */}
      <span className="flex items-center gap-1.5 shrink-0" style={{ minWidth: "140px" }}>
        {latest ? (
          <>
            <Link
              href={`/releases/${latest.id}`}
              onClick={(e) => e.stopPropagation()}
              className="inline-flex items-center rounded px-1.5 py-0.5 text-[12px] leading-none hover:ring-1 hover:ring-gray-300 transition-shadow"
              style={{
                backgroundColor: "#f3f3f1",
                fontFamily: "'JetBrains Mono', monospace",
                color: "#374151",
              }}
            >
              {latest.version}
            </Link>
            {latestIcon && latestIcon({ size: 12, className: "shrink-0", style: { color: "#9ca3af" } })}
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
            <Link
              href={`/projects/${project.id}/semantic-releases/${latestSr.id}`}
              onClick={(e) => e.stopPropagation()}
              className="inline-flex items-center rounded px-1.5 py-0.5 text-[12px] leading-none shrink-0 hover:ring-1 hover:ring-blue-300 transition-shadow"
              style={{
                backgroundColor: "#eff6ff",
                fontFamily: "'JetBrains Mono', monospace",
                color: "#1d4ed8",
              }}
            >
              {latestSr.version}
            </Link>
            {urgencyStyle && (
              <span
                className="inline-flex items-center rounded-full px-1.5 py-0.5 text-[10px] font-medium leading-none shrink-0"
                style={{ backgroundColor: urgencyStyle.bg, color: urgencyStyle.text }}
              >
                {latestSr.report?.urgency}
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
    </div>
  );
}

/* ---------- Page ---------- */

export default function ProjectsPage() {
  const [createOpen, setCreateOpen] = useState(false);
  const [viewMode, setViewMode] = useState<"cards" | "compact">("cards");
  const [search, setSearch] = useState("");
  const [sortBy, setSortBy] = useState<"updated" | "added" | "name">("updated");
  const [page, setPage] = useState(1);
  const { data, isLoading } = useSWR("projects", () => projectsApi.list(1, 100));
  const items = data?.data ?? [];

  const PAGE_SIZE = 12;
  const filtered = search
    ? items.filter((p) => p.name.toLowerCase().includes(search.toLowerCase()))
    : items;
  const sorted = useMemo(() => {
    const arr = [...filtered];
    switch (sortBy) {
      case "updated":
        return arr.sort((a, b) => new Date(b.updated_at).getTime() - new Date(a.updated_at).getTime());
      case "added":
        return arr.sort((a, b) => new Date(b.created_at).getTime() - new Date(a.created_at).getTime());
      case "name":
        return arr.sort((a, b) => a.name.localeCompare(b.name));
    }
  }, [filtered, sortBy]);
  const totalPages = Math.max(1, Math.ceil(sorted.length / PAGE_SIZE));
  const currentPage = Math.min(page, totalPages);
  const paged = sorted.slice((currentPage - 1) * PAGE_SIZE, currentPage * PAGE_SIZE);

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
        <div className="flex items-center gap-2">
          <div className="relative">
            <Search className="absolute left-2.5 top-1/2 -translate-y-1/2 h-3.5 w-3.5" style={{ color: "#9ca3af" }} />
            <input
              type="text"
              value={search}
              onChange={(e) => { setSearch(e.target.value); setPage(1); }}
              placeholder="Search projects..."
              className="rounded-md border py-1.5 pl-8 pr-3 text-[13px] w-48"
              style={{ borderColor: "#e8e8e5", fontFamily: "var(--font-dm-sans)", color: "#374151" }}
            />
          </div>
          <div className="flex items-center gap-1.5 rounded-md border px-2.5 py-1.5 text-[13px]" style={{ borderColor: "#e8e8e5", fontFamily: "var(--font-dm-sans)", color: "#374151" }}>
            <ArrowUpDown className="h-3.5 w-3.5 shrink-0" style={{ color: "#9ca3af" }} />
            <select
              value={sortBy}
              onChange={(e) => { setSortBy(e.target.value as "updated" | "added" | "name"); setPage(1); }}
              className="bg-transparent outline-none cursor-pointer text-[13px]"
              style={{ fontFamily: "var(--font-dm-sans)", color: "#374151" }}
            >
              <option value="updated">Recently updated</option>
              <option value="added">Recently added</option>
              <option value="name">Name</option>
            </select>
          </div>
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
          <button
            onClick={() => setCreateOpen(true)}
            className="mt-4 flex items-center gap-1.5 rounded px-3 py-1.5 text-[13px] text-white transition-opacity hover:opacity-90"
            style={{ backgroundColor: "#e8601a", fontFamily: "var(--font-dm-sans)" }}
          >
            <Plus className="h-3.5 w-3.5" />
            New Project
          </button>
        </div>
      ) : filtered.length === 0 ? (
        <p
          className="px-4 py-8 text-center text-[14px] italic text-[#9ca3af]"
          style={{ fontFamily: "var(--font-fraunces)" }}
        >
          No projects matching &ldquo;{search}&rdquo;
        </p>
      ) : (
        <>
          <div className={viewMode === "cards" ? "space-y-5" : "space-y-1.5"}>
            {paged.map((project) =>
              viewMode === "cards" ? (
                <ProjectFlowCard key={project.id} project={project} />
              ) : (
                <ProjectCompactRow key={project.id} project={project} />
              )
            )}
          </div>
          {totalPages > 1 && (
            <div className="flex items-center justify-center gap-3 pt-2">
              <button
                onClick={() => setPage((p) => Math.max(1, p - 1))}
                disabled={currentPage <= 1}
                className="rounded-md border px-3 py-1 text-[13px] transition-colors disabled:opacity-40"
                style={{ borderColor: "#e8e8e5", color: "#374151", fontFamily: "var(--font-dm-sans)" }}
              >
                Previous
              </button>
              <span
                className="text-[13px]"
                style={{ color: "#6b7280", fontFamily: "var(--font-dm-sans)" }}
              >
                Page {currentPage} of {totalPages}
              </span>
              <button
                onClick={() => setPage((p) => Math.min(totalPages, p + 1))}
                disabled={currentPage >= totalPages}
                className="rounded-md border px-3 py-1 text-[13px] transition-colors disabled:opacity-40"
                style={{ borderColor: "#e8e8e5", color: "#374151", fontFamily: "var(--font-dm-sans)" }}
              >
                Next
              </button>
            </div>
          )}
        </>
      )}

      <Dialog open={createOpen} onOpenChange={setCreateOpen}>
        <DialogContent className="sm:max-w-lg">
          <DialogHeader>
            <DialogTitle>Create Project</DialogTitle>
          </DialogHeader>
          <ProjectForm
            title="Create Project"
            onSubmit={async (result) => {
              const created = await projectsApi.create(result.project);
              if (result.source && created.data?.id) {
                const res = await sourcesApi.create(created.data.id, result.source);
                if (res.data?.id) sourcesApi.poll(res.data.id).catch(() => {});
              }
            }}
            onSuccess={() => { setCreateOpen(false); mutate("projects"); }}
            onCancel={() => setCreateOpen(false)}
          />
        </DialogContent>
      </Dialog>
    </div>
  );
}
