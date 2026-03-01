"use client";

import { useState } from "react";
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
import { timeAgo, validateRepository, formatInterval } from "@/lib/format";
import { Plus, X, ArrowRight, LayoutGrid, List, Search } from "lucide-react";
import { Dialog, DialogContent, DialogHeader, DialogTitle } from "@/components/ui/dialog";
import { ProjectForm } from "@/components/projects/project-form";
import type { Project, Source } from "@/lib/api/types";

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
  const [pollInterval, setPollInterval] = useState("86400");
  const [versionFilterInclude, setVersionFilterInclude] = useState("");
  const [versionFilterExclude, setVersionFilterExclude] = useState("");
  const [excludePrereleases, setExcludePrereleases] = useState(false);
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState("");

  const handleAdd = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!repository.trim()) return;
    setSaving(true);
    setError("");
    try {
      const repoError = validateRepository(provider, repository.trim());
      if (repoError) {
        setError(repoError);
        setSaving(false);
        return;
      }
      const res = await sourcesApi.create(projectId, {
        provider,
        repository: repository.trim(),
        poll_interval_seconds: Number(pollInterval) || 86400,
        enabled: true,
        version_filter_include: versionFilterInclude.trim() || undefined,
        version_filter_exclude: versionFilterExclude.trim() || undefined,
        exclude_prereleases: excludePrereleases || undefined,
      });
      if (res.data?.id) sourcesApi.poll(res.data.id).catch(() => {});
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
          <option value="ecr-public">ECR Public</option>
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
      <input
        type="text"
        value={versionFilterInclude}
        onChange={(e) => setVersionFilterInclude(e.target.value)}
        placeholder="Include filter (regex, optional)"
        className="w-full rounded-md border px-2 py-1 text-[12px]"
        style={{ borderColor: "#e8e8e5", fontFamily: "'JetBrains Mono', monospace" }}
      />
      <input
        type="text"
        value={versionFilterExclude}
        onChange={(e) => setVersionFilterExclude(e.target.value)}
        placeholder="Exclude filter (regex, optional)"
        className="w-full rounded-md border px-2 py-1 text-[12px]"
        style={{ borderColor: "#e8e8e5", fontFamily: "'JetBrains Mono', monospace" }}
      />
      {provider === "github" && (
        <label className="flex items-center gap-2 text-[12px] text-[#6b7280]">
          <input
            type="checkbox"
            checked={excludePrereleases}
            onChange={(e) => setExcludePrereleases(e.target.checked)}
            className="rounded"
          />
          Exclude pre-releases
        </label>
      )}
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
    <div className="px-4 py-3 md:border-r" style={{ borderColor: "#e8e8e5" }}>
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
                    {formatInterval(source.poll_interval_seconds)}
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
    <div className="border-t md:border-t-0 md:border-r px-4 py-3" style={{ borderColor: "#e8e8e5" }}>
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
            href={`/releases?project=${projectId}`}
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

/* ---------- Semantic Releases Section ---------- */

const URGENCY_COLORS: Record<string, { bg: string; text: string }> = {
  critical: { bg: "#dc2626", text: "#ffffff" },
  high: { bg: "#f97316", text: "#ffffff" },
  medium: { bg: "#f59e0b", text: "#ffffff" },
  low: { bg: "#6b7280", text: "#ffffff" },
};

function SemanticReleasesSection({ projectId }: { projectId: string }) {
  const { data } = useSWR(`project-${projectId}-card-sr`, () =>
    srApi.list(projectId, 1, 3)
  );
  const items = data?.data ?? [];

  return (
    <div className="border-t md:border-t-0 px-4 py-3" style={{ borderColor: "#e8e8e5" }}>
      <span
        className="text-[11px] font-medium uppercase tracking-[0.08em] mb-1 block"
        style={{ color: "#9ca3af", fontFamily: "var(--font-dm-sans)" }}
      >
        Semantic Releases
      </span>

      {items.length > 0 ? (
        <div className="space-y-1">
          {items.map((sr) => {
            const urgencyStyle = sr.report?.urgency
              ? URGENCY_COLORS[sr.report.urgency.toLowerCase()]
              : undefined;
            return (
              <Link
                key={sr.id}
                href={`/projects/${projectId}/semantic-releases/${sr.id}`}
                className="flex items-center gap-2 text-[12px] transition-colors hover:bg-[#fafaf9] rounded px-1 -mx-1 py-0.5"
              >
                <span
                  className="inline-flex items-center rounded px-1.5 py-0.5 text-[12px] leading-none"
                  style={{
                    backgroundColor: "#eff6ff",
                    fontFamily: "'JetBrains Mono', monospace",
                    color: "#1d4ed8",
                  }}
                >
                  {sr.version}
                </span>
                <span
                  className="text-[11px]"
                  style={{ color: "#9ca3af" }}
                >
                  {sr.status}
                </span>
                {urgencyStyle && (
                  <span
                    className="inline-flex items-center rounded-full px-1.5 py-0.5 text-[10px] font-medium leading-none"
                    style={{ backgroundColor: urgencyStyle.bg, color: urgencyStyle.text }}
                  >
                    {sr.report!.urgency}
                  </span>
                )}
                {sr.report?.summary && (
                  <span
                    className="text-[11px] truncate flex-1"
                    style={{ color: "#c4c4c0" }}
                  >
                    {sr.report.summary.length > 60
                      ? sr.report.summary.slice(0, 60) + "\u2026"
                      : sr.report.summary}
                  </span>
                )}
                <span className="ml-auto text-[11px] shrink-0" style={{ color: "#c4c4c0" }}>
                  {timeAgo(sr.completed_at || sr.created_at)}
                </span>
              </Link>
            );
          })}
          <Link
            href={`/semantic-releases?project=${projectId}`}
            className="block text-[11px] mt-1 hover:underline"
            style={{ color: "#e8601a", fontFamily: "var(--font-dm-sans)" }}
          >
            View all &rarr;
          </Link>
        </div>
      ) : (
        <p className="text-[12px] italic" style={{ color: "#c4c4c0" }}>
          No semantic releases yet
        </p>
      )}
    </div>
  );
}

/* ---------- Project Card Logo ---------- */

function ProjectCardLogo({ projectId, name }: { projectId: string; name: string }) {
  const { data } = useSWR(`project-${projectId}-card-sources`, () =>
    sourcesApi.listByProject(projectId)
  );
  return <ProjectLogo name={name} sources={data?.data} size={40} />;
}

/* ---------- Project Card ---------- */

function ProjectCard({ project }: { project: Project }) {
  return (
    <div
      className="overflow-hidden rounded-md"
      style={{ border: "1px solid #e8e8e5", backgroundColor: "#ffffff" }}
    >
      {/* Header */}
      <div className="px-4 py-3">
        <div className="min-w-0 flex items-center gap-3">
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
                className="mt-0.5 text-[13px] truncate"
                style={{ color: "#6b7280", fontFamily: "var(--font-dm-sans)" }}
              >
                {project.description}
              </p>
            )}
          </div>
        </div>
      </div>

      {/* Content columns: sources | recent releases | semantic releases */}
      <div className="grid grid-cols-1 md:grid-cols-3 border-t" style={{ borderColor: "#e8e8e5" }}>
        <SourcesSection projectId={project.id} />
        <RecentReleasesSection projectId={project.id} />
        <SemanticReleasesSection projectId={project.id} />
      </div>
    </div>
  );
}

/* ---------- Compact Row ---------- */

function ProjectCompactRow({ project }: { project: Project }) {
  const router = useRouter();
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
  const [page, setPage] = useState(1);
  const { data, isLoading } = useSWR("projects", () => projectsApi.list(1, 100));
  const items = data?.data ?? [];

  const PAGE_SIZE = 12;
  const filtered = search
    ? items.filter((p) => p.name.toLowerCase().includes(search.toLowerCase()))
    : items;
  const totalPages = Math.max(1, Math.ceil(filtered.length / PAGE_SIZE));
  const currentPage = Math.min(page, totalPages);
  const paged = filtered.slice((currentPage - 1) * PAGE_SIZE, currentPage * PAGE_SIZE);

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
          <div className={viewMode === "cards" ? "space-y-4" : "space-y-1.5"}>
            {paged.map((project) =>
              viewMode === "cards" ? (
                <ProjectCard key={project.id} project={project} />
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
