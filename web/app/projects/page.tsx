"use client";

import { useState } from "react";
import useSWR from "swr";
import Link from "next/link";
import {
  projects as projectsApi,
  releases as releasesApi,
  semanticReleases as srApi,
} from "@/lib/api/client";
import { timeAgo } from "@/lib/format";
import { Plus } from "lucide-react";
import type { Project } from "@/lib/api/types";

const CHUNK_SIZE = 3;

function ReleaseChips({ projectId }: { projectId: string }) {
  const [limit, setLimit] = useState(CHUNK_SIZE);
  const { data } = useSWR(
    `projects/${projectId}/releases?limit=${limit + 1}`,
    () => releasesApi.listByProject(projectId, 1, limit + 1)
  );
  const items = data?.data ?? [];
  const hasMore = items.length > limit;
  const shown = items.slice(0, limit);

  if (!data) return <span className="text-[12px] text-[#c4c4c0] italic">loading…</span>;
  if (shown.length === 0) return <span className="text-[12px] text-[#c4c4c0]">—</span>;

  return (
    <div className="flex flex-wrap items-center gap-1">
      {shown.map((r) => (
        <Link
          key={r.id}
          href={`/releases/${r.id}`}
          className="inline-flex items-center rounded px-1.5 py-0.5 text-[12px] leading-none text-[#374151] transition-colors hover:bg-[#e8e8e5]"
          style={{ backgroundColor: "#f3f3f1", fontFamily: "'JetBrains Mono', monospace" }}
        >
          {r.version}
        </Link>
      ))}
      {hasMore && (
        <button
          onClick={() => setLimit((n) => n + CHUNK_SIZE)}
          className="text-[11px] text-[#e8601a] hover:underline"
          style={{ fontFamily: "var(--font-dm-sans)" }}
        >
          more…
        </button>
      )}
    </div>
  );
}

function SemanticReleaseChips({ projectId }: { projectId: string }) {
  const [limit, setLimit] = useState(CHUNK_SIZE);
  const { data } = useSWR(
    `projects/${projectId}/sr?limit=${limit + 1}`,
    () => srApi.list(projectId, 1, limit + 1)
  );
  const items = data?.data ?? [];
  const hasMore = items.length > limit;
  const shown = items.slice(0, limit);

  if (!data) return <span className="text-[12px] text-[#c4c4c0] italic">loading…</span>;
  if (shown.length === 0) return <span className="text-[12px] text-[#c4c4c0]">—</span>;

  return (
    <div className="flex flex-wrap items-center gap-1">
      {shown.map((sr) => (
        <Link
          key={sr.id}
          href={`/projects/${projectId}/semantic-releases/${sr.id}`}
          className="inline-flex items-center rounded px-1.5 py-0.5 text-[12px] leading-none text-[#1d4ed8] transition-colors hover:bg-[#dbeafe]"
          style={{ backgroundColor: "#eff6ff", fontFamily: "'JetBrains Mono', monospace" }}
        >
          {sr.version}
        </Link>
      ))}
      {hasMore && (
        <button
          onClick={() => setLimit((n) => n + CHUNK_SIZE)}
          className="text-[11px] text-[#e8601a] hover:underline"
          style={{ fontFamily: "var(--font-dm-sans)" }}
        >
          more…
        </button>
      )}
    </div>
  );
}

function ProjectRow({ project, index }: { project: Project; index: number }) {
  return (
    <tr
      className="transition-colors duration-100 hover:bg-[#fafaf9]"
      style={index > 0 ? { borderTop: "1px solid #e8e8e5" } : undefined}
    >
      <td className="px-4 py-2.5">
        <Link
          href={`/projects/${project.id}`}
          className="font-medium text-[#e8601a] hover:underline"
        >
          {project.name}
        </Link>
      </td>
      <td className="max-w-[220px] truncate px-4 py-2.5 text-[#6b7280]">
        {project.description || "—"}
      </td>
      <td className="px-4 py-2.5">
        <ReleaseChips projectId={project.id} />
      </td>
      <td className="px-4 py-2.5">
        <SemanticReleaseChips projectId={project.id} />
      </td>
      <td className="px-4 py-2.5 text-[12px] text-[#9ca3af]">
        {timeAgo(project.created_at)}
      </td>
    </tr>
  );
}

export default function ProjectsPage() {
  const { data, isLoading } = useSWR("projects", () => projectsApi.list());
  const items = data?.data ?? [];

  return (
    <div className="flex flex-col gap-4 fade-in">
      <div className="flex items-center justify-between">
        <div>
          <h1
            className="text-[24px] font-semibold text-[#111113]"
            style={{ fontFamily: "var(--font-fraunces)" }}
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

      <div
        className="overflow-hidden rounded-md"
        style={{ border: "1px solid #e8e8e5", backgroundColor: "#ffffff" }}
      >
        {isLoading ? (
          <p
            className="px-4 py-8 text-center text-[14px] italic text-[#9ca3af]"
            style={{ fontFamily: "var(--font-fraunces)" }}
          >
            Loading…
          </p>
        ) : items.length === 0 ? (
          <p
            className="px-4 py-8 text-center text-[14px] italic text-[#9ca3af]"
            style={{ fontFamily: "var(--font-fraunces)" }}
          >
            No projects yet — create one to start tracking releases
          </p>
        ) : (
          <table className="w-full text-[13px]" style={{ fontFamily: "var(--font-dm-sans)" }}>
            <thead>
              <tr style={{ borderBottom: "1px solid #e8e8e5", backgroundColor: "#fafaf9" }}>
                {["Name", "Description", "Recent Releases", "Semantic Releases", "Created"].map(
                  (h) => (
                    <th
                      key={h}
                      className="px-4 py-2.5 text-left text-[11px] font-medium uppercase tracking-[0.08em] text-[#9ca3af]"
                    >
                      {h}
                    </th>
                  )
                )}
              </tr>
            </thead>
            <tbody>
              {items.map((project, i) => (
                <ProjectRow key={project.id} project={project} index={i} />
              ))}
            </tbody>
          </table>
        )}
      </div>
    </div>
  );
}
