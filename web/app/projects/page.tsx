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
import { Plus, ArrowRight, LayoutGrid, List, Search, Pencil, ArrowUpDown, Loader2, Info } from "lucide-react";
import { SourceForm } from "@/components/sources/source-form";
import { Dialog, DialogContent, DialogHeader, DialogTitle } from "@/components/ui/dialog";
import { ProjectForm } from "@/components/projects/project-form";
import type { Project, Source } from "@/lib/api/types";
import { URGENCY_STYLES, URGENCY_COLORS } from "@/components/ui/urgency-pill";
import { useTranslation } from "@/lib/i18n/context";

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
  moreLabel,
  children,
}: {
  label: string;
  moreHref: string;
  moreLabel: string;
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
    const raf = requestAnimationFrame(measure);
    const obs = new ResizeObserver(measure);
    if (measureRef.current) obs.observe(measureRef.current);
    return () => {
      cancelAnimationFrame(raf);
      obs.disconnect();
    };
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
          {moreLabel}
        </span>
      </div>
      {/* Visible section */}
      <div className="text-[13px] leading-7">
        <span
          className="text-[11px] font-medium uppercase tracking-[0.08em] mr-1.5 text-text-muted"
          style={{ fontFamily: "var(--font-dm-sans)" }}
        >
          {label}:
        </span>
        {displayed}
        {hasOverflow && (
          <Link
            href={moreHref}
            className="inline-flex items-baseline text-[12px] font-medium hover:underline whitespace-nowrap text-beacon-accent"
          >
            {moreLabel}
          </Link>
        )}
      </div>
    </div>
  );
}

/* ---------- Urgency Legend ---------- */

function UrgencyLegend() {
  const { t } = useTranslation();
  const entries = [
    { key: "critical", label: t("projects.urgency.critical"), desc: t("projects.urgency.criticalDesc") },
    { key: "high", label: t("projects.urgency.high"), desc: t("projects.urgency.highDesc") },
    { key: "medium", label: t("projects.urgency.medium"), desc: t("projects.urgency.mediumDesc") },
    { key: "low", label: t("projects.urgency.low"), desc: t("projects.urgency.lowDesc") },
  ] as const;

  return (
    <span className="relative inline-flex items-center group mr-1">
      <Info size={11} className="cursor-help text-text-muted" />
      <span
        className="absolute left-0 top-full mt-1 z-50 hidden group-hover:block rounded-lg shadow-lg py-2 px-3 bg-surface border-border"
        style={{ border: "1px solid var(--border)", width: 260 }}
      >
        <span className="block text-[10px] font-semibold uppercase tracking-wider mb-1.5 text-text-muted">
          {t("projects.urgencyLegend")}
        </span>
        {entries.map(({ key, label, desc }) => {
          const s = URGENCY_STYLES[key];
          const Icon = s.icon;
          return (
            <span key={key} className="flex items-start gap-2 mb-1 last:mb-0">
              <span
                className="inline-flex items-center justify-center rounded-full shrink-0 mt-0.5"
                style={{ backgroundColor: s.bg, border: `1px solid ${s.border}`, color: s.text, width: 16, height: 16 }}
              >
                <Icon size={9} />
              </span>
              <span className="flex flex-col">
                <span className="text-[11px] font-semibold leading-tight" style={{ color: s.text }}>{label}</span>
                <span className="text-[10px] leading-tight text-text-muted">{desc}</span>
              </span>
            </span>
          );
        })}
      </span>
    </span>
  );
}

function ProjectFlowCard({ project }: { project: Project }) {
  const { t } = useTranslation();
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
      className="rounded-md px-5 py-4 border-border bg-surface"
      style={{ border: "1px solid var(--border)" }}
    >
      {/* Header */}
      <div className="flex items-center justify-between mb-2">
        <div className="flex items-center gap-3 min-w-0">
          <ProjectCardLogo projectId={project.id} name={project.name} />
          <div className="min-w-0">
            <Link
              href={`/projects/${project.id}`}
              className="group inline-flex items-center gap-1.5 text-[16px] font-bold transition-colors text-beacon-accent"
              style={{ fontFamily: "var(--font-fraunces)" }}
            >
              {project.name}
              <ArrowRight className="h-3.5 w-3.5 opacity-0 -translate-x-1 transition-all group-hover:opacity-100 group-hover:translate-x-0" />
            </Link>
            {project.description && (
              <p
                className="text-[13px] truncate text-text-secondary"
                style={{ fontFamily: "var(--font-dm-sans)" }}
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
          className="text-[11px] font-medium uppercase tracking-[0.08em] mr-0.5 text-text-muted"
          style={{ fontFamily: "var(--font-dm-sans)" }}
        >
          {t("projects.sources")}:
        </span>
        {sources.map((source) => (
          <button
            key={source.id}
            onClick={() => setEditingSource(source)}
            className="group/chip inline-flex items-center gap-1.5 rounded px-2 py-0.5 text-[12px] transition-colors hover:bg-mono-bg"
            style={{
              backgroundColor: source.last_error ? "var(--error-bg)" : "var(--background)",
              border: `1px solid ${source.last_error ? "var(--error-border)" : "var(--border)"}`,
            }}
            title={source.last_error ? `Error: ${source.last_error}` : undefined}
          >
            <span
              className="inline-block h-1.5 w-1.5 rounded-full shrink-0"
              style={{ backgroundColor: source.last_error ? "var(--error-text)" : source.enabled ? "var(--status-completed)" : "var(--text-muted)" }}
            />
            {(() => { const Icon = getProviderIcon(source.provider); return Icon ? <Icon size={12} className="shrink-0 text-text-muted" /> : null; })()}
            <span className="text-secondary-foreground" style={{ fontFamily: "'JetBrains Mono', monospace" }}>
              {source.repository}
            </span>
            {source.last_polled_at && (
              <span className="text-text-muted">
                {timeAgo(source.last_polled_at).replace(" ago", "")}
              </span>
            )}
            <Pencil className="h-2.5 w-2.5 hidden group-hover/chip:inline shrink-0 text-text-muted opacity-50" />
          </button>
        ))}
        <button
          onClick={() => setSourceCreateOpen(true)}
          className="inline-flex items-center gap-1 px-2 py-0.5 text-[12px] font-medium rounded border transition-colors hover:bg-mono-bg border-border text-text-muted"
          style={{
            fontFamily: "var(--font-dm-sans)",
          }}
        >
          <Plus className="h-3 w-3" />
          {t("projects.addSource")}
        </button>
      </div>

      {/* Releases flow */}
      {releases.length > 0 && (
        <FlowSection label={t("projects.releases")} moreHref={`/releases?project=${project.id}`} moreLabel={t("projects.more")}>
          {releases.map((r) => {
            const src = sourceMap.get(r.source_id);
            const matchingSr = srItems.find((sr) => sr.version === r.version);
            return (
              <span key={r.id} className="inline-flex items-baseline mr-2.5">
                <Link
                  href={`/releases/${r.id}`}
                  className={r.excluded ? "" : "text-[#2563eb] hover:underline"}
                  style={{
                    fontFamily: "'JetBrains Mono', monospace",
                    fontSize: "12px",
                    ...(r.excluded ? { color: "var(--text-muted)" } : {}),
                  }}
                >
                  {r.version}
                </Link>
                {!r.excluded && matchingSr?.status === "completed" && (() => {
                  const pill = matchingSr.report?.urgency
                    ? URGENCY_STYLES[matchingSr.report.urgency.toLowerCase()]
                    : undefined;
                  return pill ? (
                    <Link
                      href={`/projects/${project.id}/semantic-releases/${matchingSr.id}`}
                      className="inline-flex items-center justify-center rounded-full ml-1 transition-colors"
                      style={{ backgroundColor: pill.bg, border: `1px solid ${pill.border}`, color: pill.text, width: 18, height: 18 }}
                      title={`${matchingSr.report!.urgency} — ${t("projects.viewReport")}`}
                    >
                      <pill.icon size={10} />
                    </Link>
                  ) : (
                    <Link
                      href={`/projects/${project.id}/semantic-releases/${matchingSr.id}`}
                      className="inline-flex items-center gap-0.5 rounded-full px-2 py-0.5 ml-1 text-[10px] font-semibold transition-colors bg-muted text-text-secondary"
                      style={{ border: "1px solid rgba(107,114,128,0.18)", fontFamily: "var(--font-dm-sans)" }}
                      title={t("projects.viewReport")}
                    >
                      {t("projects.report")}
                    </Link>
                  );
                })()}
                {!r.excluded && (matchingSr?.status === "pending" || matchingSr?.status === "processing") && (
                  <span
                    className="inline-flex items-center gap-0.5 rounded px-1 py-0.5 ml-1 text-[10px]"
                    style={{ backgroundColor: "#fef9c3", color: "#92400e", fontFamily: "var(--font-dm-sans)" }}
                  >
                    <Loader2 size={10} className="animate-spin" />
                  </span>
                )}
                {src && (
                  <span
                    className="text-[11px] ml-1 hidden sm:inline text-text-muted"
                    style={r.excluded ? { opacity: 0.5 } : {}}
                  >
                    ({src.repository.split("/").pop()})
                  </span>
                )}
                <span
                  className="text-[11px] ml-1 text-text-muted"
                  style={r.excluded ? { opacity: 0.5 } : {}}
                >
                  {timeAgo(r.released_at || r.created_at).replace(" ago", "")}
                </span>
              </span>
            );
          })}
        </FlowSection>
      )}

      {/* Add Source dialog */}
      <Dialog open={sourceCreateOpen} onOpenChange={setSourceCreateOpen}>
        <DialogContent className="sm:max-w-lg">
          <DialogHeader>
            <DialogTitle>{t("projects.addSource")}</DialogTitle>
          </DialogHeader>
          <SourceForm
            title={t("projects.addSource")}
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
            <DialogTitle>{t("projects.editSource")}</DialogTitle>
          </DialogHeader>
          {editingSource && (
            <SourceForm
              key={editingSource.id}
              title={t("projects.editSource")}
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
  const { t } = useTranslation();
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
      className="flex items-center gap-4 rounded-md px-4 py-2.5 transition-colors hover:bg-background cursor-pointer bg-surface"
      style={{ border: "1px solid var(--border)" }}
    >
      {/* Project name */}
      <span
        className="flex items-center gap-2 text-[14px] font-bold shrink-0 text-beacon-accent"
        style={{ fontFamily: "var(--font-fraunces)", minWidth: "160px" }}
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
              className="inline-flex items-center rounded px-1.5 py-0.5 text-[12px] leading-none hover:ring-1 hover:ring-border transition-shadow bg-mono-bg text-secondary-foreground"
              style={{
                fontFamily: "'JetBrains Mono', monospace",
              }}
            >
              {latest.version}
            </Link>
            {latestIcon && latestIcon({ size: 12, className: "shrink-0 text-text-muted" })}
            {latestSr?.status === "completed" && (() => {
              const pill = latestSr.report?.urgency
                ? URGENCY_STYLES[latestSr.report.urgency.toLowerCase()]
                : undefined;
              return pill ? (
                <Link
                  href={`/projects/${project.id}/semantic-releases/${latestSr.id}`}
                  onClick={(e) => e.stopPropagation()}
                  className="inline-flex items-center justify-center rounded-full transition-colors"
                  style={{ backgroundColor: pill.bg, border: `1px solid ${pill.border}`, color: pill.text, width: 18, height: 18 }}
                  title={`${latestSr.report!.urgency} — ${t("projects.viewReport")}`}
                >
                  <pill.icon size={10} />
                </Link>
              ) : (
                <Link
                  href={`/projects/${project.id}/semantic-releases/${latestSr.id}`}
                  onClick={(e) => e.stopPropagation()}
                  className="inline-flex items-center gap-0.5 rounded-full px-2 py-0.5 text-[10px] font-semibold transition-colors bg-muted text-text-secondary"
                  style={{ border: "1px solid rgba(107,114,128,0.18)", fontFamily: "var(--font-dm-sans)" }}
                  title={t("projects.viewReport")}
                >
                  {t("projects.report")}
                </Link>
              );
            })()}
            {(latestSr?.status === "pending" || latestSr?.status === "processing") && (
              <span
                className="inline-flex items-center gap-0.5 rounded px-1.5 py-0.5 text-[10px]"
                style={{ backgroundColor: "#fef9c3", color: "#92400e", fontFamily: "var(--font-dm-sans)" }}
              >
                <Loader2 size={10} className="animate-spin" />
              </span>
            )}
          </>
        ) : (
          <span className="text-[12px] italic text-text-muted">
            {t("projects.noReleases")}
          </span>
        )}
      </span>

      {/* Summary */}
      <span className="flex items-center gap-1.5 flex-1 min-w-0">
        {latestSr?.status === "completed" ? (
          <>
            {urgencyStyle && (
              <span
                className="inline-flex items-center rounded-full px-1.5 py-0.5 text-[10px] font-medium leading-none shrink-0"
                style={{ backgroundColor: urgencyStyle.bg, color: urgencyStyle.text }}
              >
                {latestSr.report?.urgency}
              </span>
            )}
            {latestSr.report?.summary && (
              <span className="text-[11px] truncate text-text-muted">
                {latestSr.report.summary.length > 60
                  ? latestSr.report.summary.slice(0, 60) + "\u2026"
                  : latestSr.report.summary}
              </span>
            )}
          </>
        ) : (
          <span className="text-[12px] italic text-text-muted">
            {latestSr ? latestSr.status : t("projects.noAnalysis")}
          </span>
        )}
      </span>

      {/* Arrow */}
      <ArrowRight className="h-3.5 w-3.5 shrink-0 text-text-muted" />
    </div>
  );
}

/* ---------- Page ---------- */

export default function ProjectsPage() {
  const { t } = useTranslation();
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
            className="text-foreground"
            style={{
              fontFamily: "var(--font-fraunces)",
              fontSize: "24px",
              fontWeight: 700,
            }}
          >
            {t("projects.title")}
          </h1>
          <p
            className="mt-1 text-[13px] text-text-secondary inline-flex items-center gap-1"
            style={{ fontFamily: "var(--font-dm-sans)" }}
          >
            {t("projects.description")}
            <UrgencyLegend />
          </p>
        </div>
        <div className="flex items-center gap-2">
          <div className="relative">
            <Search className="absolute left-2.5 top-1/2 -translate-y-1/2 h-3.5 w-3.5 text-text-muted" />
            <input
              type="text"
              value={search}
              onChange={(e) => { setSearch(e.target.value); setPage(1); }}
              placeholder={t("projects.searchPlaceholder")}
              className="rounded-md border py-1.5 pl-8 pr-3 text-[13px] w-48 border-border text-secondary-foreground bg-surface"
              style={{ fontFamily: "var(--font-dm-sans)" }}
            />
          </div>
          <div className="flex items-center gap-1.5 rounded-md border px-2.5 py-1.5 text-[13px] border-border text-secondary-foreground" style={{ fontFamily: "var(--font-dm-sans)" }}>
            <ArrowUpDown className="h-3.5 w-3.5 shrink-0 text-text-muted" />
            <select
              value={sortBy}
              onChange={(e) => { setSortBy(e.target.value as "updated" | "added" | "name"); setPage(1); }}
              className="bg-transparent outline-none cursor-pointer text-[13px] text-secondary-foreground"
              style={{ fontFamily: "var(--font-dm-sans)" }}
            >
              <option value="updated">{t("projects.sortUpdated")}</option>
              <option value="added">{t("projects.sortAdded")}</option>
              <option value="name">{t("projects.sortName")}</option>
            </select>
          </div>
          <div className="flex items-center rounded-md border border-border">
            <button
              onClick={() => setViewMode("cards")}
              className="p-1.5 rounded-l-md transition-colors"
              style={{
                backgroundColor: viewMode === "cards" ? "var(--mono-bg)" : "transparent",
                color: viewMode === "cards" ? "var(--foreground)" : "var(--text-muted)",
              }}
              title={t("projects.cardView")}
            >
              <LayoutGrid className="h-4 w-4" />
            </button>
            <button
              onClick={() => setViewMode("compact")}
              className="p-1.5 rounded-r-md transition-colors"
              style={{
                backgroundColor: viewMode === "compact" ? "var(--mono-bg)" : "transparent",
                color: viewMode === "compact" ? "var(--foreground)" : "var(--text-muted)",
              }}
              title={t("projects.compactView")}
            >
              <List className="h-4 w-4" />
            </button>
          </div>
          <button
            onClick={() => setCreateOpen(true)}
            className="flex items-center gap-1.5 rounded px-3 py-1.5 text-[13px] text-white transition-opacity hover:opacity-90 bg-beacon-accent"
            style={{ fontFamily: "var(--font-dm-sans)" }}
          >
            <Plus className="h-3.5 w-3.5" />
            {t("projects.newProject")}
          </button>
        </div>
      </div>

      {isLoading ? (
        <p
          className="px-4 py-8 text-center text-[14px] italic text-text-muted"
          style={{ fontFamily: "var(--font-fraunces)" }}
        >
          {t("projects.loading")}
        </p>
      ) : items.length === 0 ? (
        <div
          className="flex flex-col items-center justify-center rounded-md border py-12 border-border bg-surface"
        >
          <p
            className="text-[14px] italic text-text-muted"
            style={{ fontFamily: "var(--font-fraunces)" }}
          >
            {t("projects.empty")}
          </p>
          <button
            onClick={() => setCreateOpen(true)}
            className="mt-4 flex items-center gap-1.5 rounded px-3 py-1.5 text-[13px] text-white transition-opacity hover:opacity-90 bg-beacon-accent"
            style={{ fontFamily: "var(--font-dm-sans)" }}
          >
            <Plus className="h-3.5 w-3.5" />
            {t("projects.newProject")}
          </button>
        </div>
      ) : filtered.length === 0 ? (
        <p
          className="px-4 py-8 text-center text-[14px] italic text-text-muted"
          style={{ fontFamily: "var(--font-fraunces)" }}
        >
          {t("projects.noMatch").replace("{search}", search)}
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
                className="rounded-md border px-3 py-1 text-[13px] transition-colors disabled:opacity-40 border-border text-secondary-foreground"
                style={{ fontFamily: "var(--font-dm-sans)" }}
              >
                {t("projects.previous")}
              </button>
              <span
                className="text-[13px] text-text-secondary"
                style={{ fontFamily: "var(--font-dm-sans)" }}
              >
                {t("projects.page").replace("{current}", String(currentPage)).replace("{total}", String(totalPages))}
              </span>
              <button
                onClick={() => setPage((p) => Math.min(totalPages, p + 1))}
                disabled={currentPage >= totalPages}
                className="rounded-md border px-3 py-1 text-[13px] transition-colors disabled:opacity-40 border-border text-secondary-foreground"
                style={{ fontFamily: "var(--font-dm-sans)" }}
              >
                {t("projects.next")}
              </button>
            </div>
          )}
        </>
      )}

      <Dialog open={createOpen} onOpenChange={setCreateOpen}>
        <DialogContent className="sm:max-w-lg">
          <DialogHeader>
            <DialogTitle>{t("projects.createProject")}</DialogTitle>
          </DialogHeader>
          <ProjectForm
            title={t("projects.createProject")}
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
