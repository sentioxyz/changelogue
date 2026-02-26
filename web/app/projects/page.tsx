"use client";

import { useState } from "react";
import useSWR, { mutate } from "swr";
import Link from "next/link";
import {
  projects as projectsApi,
  releases as releasesApi,
  sources as sourcesApi,
  semanticReleases as srApi,
} from "@/lib/api/client";
import { getProviderIcon } from "@/components/ui/provider-badge";
import { timeAgo } from "@/lib/format";
import { Plus, X, Pencil, Check } from "lucide-react";
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
            href={`/projects/${projectId}`}
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

/* ---------- Project Card ---------- */

function ProjectCard({ project }: { project: Project }) {
  const [editing, setEditing] = useState(false);
  const [name, setName] = useState(project.name);
  const [description, setDescription] = useState(project.description ?? "");
  const [saving, setSaving] = useState(false);

  const handleSave = async () => {
    if (!name.trim()) return;
    setSaving(true);
    try {
      await projectsApi.update(project.id, {
        name: name.trim(),
        description: description.trim() || undefined,
      });
      mutate("projects");
      setEditing(false);
    } catch {
      // keep editing on error
    } finally {
      setSaving(false);
    }
  };

  const handleCancel = () => {
    setName(project.name);
    setDescription(project.description ?? "");
    setEditing(false);
  };

  return (
    <div
      className="overflow-hidden rounded-md"
      style={{ border: "1px solid #e8e8e5", backgroundColor: "#ffffff" }}
    >
      {/* Header */}
      <div className="px-4 py-3">
        {editing ? (
          <div className="space-y-2 max-w-sm">
            <input
              type="text"
              value={name}
              onChange={(e) => setName(e.target.value)}
              className="w-full rounded-md border px-2 py-1 text-[16px] font-bold focus:outline-none focus:ring-1"
              style={{
                fontFamily: "var(--font-fraunces)",
                color: "#111113",
                borderColor: "#e8e8e5",
              }}
              autoFocus
            />
            <input
              type="text"
              value={description}
              onChange={(e) => setDescription(e.target.value)}
              placeholder="Description (optional)"
              className="w-full rounded-md border px-2 py-1 text-[13px] focus:outline-none focus:ring-1"
              style={{
                fontFamily: "var(--font-dm-sans)",
                color: "#6b7280",
                borderColor: "#e8e8e5",
              }}
            />
            <div className="flex items-center gap-2">
              <button
                onClick={handleSave}
                disabled={saving || !name.trim()}
                className="inline-flex items-center gap-1 rounded-md px-2 py-1 text-[12px] font-medium text-white disabled:opacity-40"
                style={{ backgroundColor: "#e8601a", fontFamily: "var(--font-dm-sans)" }}
              >
                <Check className="h-3 w-3" />
                {saving ? "Saving..." : "Save"}
              </button>
              <button
                onClick={handleCancel}
                className="text-[12px] text-[#9ca3af] hover:text-[#6b7280]"
                style={{ fontFamily: "var(--font-dm-sans)" }}
              >
                Cancel
              </button>
            </div>
          </div>
        ) : (
          <div className="flex items-center gap-3">
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
            <button
              onClick={() => setEditing(true)}
              className="shrink-0 inline-flex items-center gap-1 rounded-md border px-2 py-1 text-[12px] font-medium transition-colors hover:bg-[#f3f3f1]"
              style={{
                fontFamily: "var(--font-dm-sans)",
                borderColor: "#e8e8e5",
                color: "#6b7280",
              }}
            >
              <Pencil className="h-3 w-3" />
              Edit
            </button>
          </div>
        )}
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
