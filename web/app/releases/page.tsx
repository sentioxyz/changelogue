"use client";

import { useState } from "react";
import useSWR from "swr";
import Link from "next/link";
import {
  releases as releasesApi,
  projects as projectsApi,
  sources as sourcesApi,
} from "@/lib/api/client";
import { ProviderBadge } from "@/components/ui/provider-badge";
import { VersionChip } from "@/components/ui/version-chip";
import type { Release, Source, Project } from "@/lib/api/types";

/* ------------------------------------------------------------------ */
/*  Helpers                                                            */
/* ------------------------------------------------------------------ */

function timeAgo(dateStr?: string | null): string {
  if (!dateStr) return "\u2014";
  const diff = Date.now() - new Date(dateStr).getTime();
  const mins = Math.floor(diff / 60000);
  if (mins < 60) return `${mins}m ago`;
  const hrs = Math.floor(mins / 60);
  if (hrs < 24) return `${hrs}h ago`;
  return `${Math.floor(hrs / 24)}d ago`;
}

interface ReleaseRow extends Release {
  _projectName?: string;
  _projectId?: string;
  _provider?: string;
  _repository?: string;
}

const PER_PAGE = 15;

/* ------------------------------------------------------------------ */
/*  Page                                                               */
/* ------------------------------------------------------------------ */

export default function ReleasesPage() {
  const [page, setPage] = useState(1);
  const [projectFilter, setProjectFilter] = useState<string>("all");

  /* Fetch projects for the filter dropdown + source enrichment */
  const { data: projectsData } = useSWR("projects-for-filter", () =>
    projectsApi.list()
  );

  /* Build a map: sourceId -> { provider, repository, projectName, projectId } */
  const { data: sourceMap } = useSWR(
    projectsData ? "source-map-for-releases" : null,
    async () => {
      if (!projectsData?.data?.length) return new Map<string, { provider: string; repository: string; projectName: string; projectId: string }>();
      const map = new Map<string, { provider: string; repository: string; projectName: string; projectId: string }>();
      await Promise.all(
        projectsData.data.map(async (p: Project) => {
          const res = await sourcesApi.listByProject(p.id).catch(() => null);
          if (res?.data) {
            for (const s of res.data) {
              map.set(s.id, {
                provider: s.provider,
                repository: s.repository,
                projectName: p.name,
                projectId: p.id,
              });
            }
          }
        })
      );
      return map;
    }
  );

  /* Fetch releases — scoped by project or aggregated across all */
  const { data: scopedData, isLoading: scopedLoading } = useSWR(
    projectFilter !== "all" ? ["releases", page, projectFilter] : null,
    () => releasesApi.listByProject(projectFilter, page)
  );

  const { data: allReleasesData, isLoading: allLoading } = useSWR(
    projectFilter === "all" && projectsData
      ? ["all-releases", page]
      : null,
    async () => {
      if (!projectsData?.data?.length) return [];
      const results = await Promise.all(
        projectsData.data.map((p: Project) =>
          releasesApi.listByProject(p.id, page).catch(() => null)
        )
      );
      return results
        .filter((r): r is NonNullable<typeof r> => r !== null)
        .flatMap((r) => r.data)
        .sort(
          (a, b) =>
            new Date(b.created_at).getTime() - new Date(a.created_at).getTime()
        );
    }
  );

  const isLoading = projectFilter !== "all" ? scopedLoading : allLoading;

  /* Enrich releases with source metadata */
  const rawReleases: Release[] =
    projectFilter !== "all"
      ? scopedData?.data ?? []
      : allReleasesData ?? [];

  const releases: ReleaseRow[] = rawReleases.map((r) => {
    const info = sourceMap?.get(r.source_id);
    return {
      ...r,
      _projectName: info?.projectName,
      _projectId: info?.projectId,
      _provider: info?.provider,
      _repository: info?.repository,
    };
  });

  /* Pagination math */
  const total = projectFilter !== "all" ? (scopedData?.meta?.total ?? 0) : releases.length;
  const totalPages = Math.max(1, Math.ceil(total / PER_PAGE));
  const startRow = (page - 1) * PER_PAGE + 1;
  const endRow = Math.min(page * PER_PAGE, total);

  return (
    <div className="space-y-6">
      {/* Page title */}
      <h1
        style={{
          fontFamily: "var(--font-fraunces)",
          fontSize: "24px",
          fontWeight: 700,
          color: "#1a1a1a",
        }}
      >
        Releases
      </h1>

      {/* Project filter */}
      <div>
        <select
          value={projectFilter}
          onChange={(e) => {
            setProjectFilter(e.target.value);
            setPage(1);
          }}
          className="appearance-none rounded-md bg-white px-3 py-2 pr-8 outline-none transition-shadow"
          style={{
            fontFamily: "var(--font-dm-sans)",
            fontSize: "13px",
            color: "#1a1a1a",
            border: "1px solid #e8e8e5",
          }}
          onFocus={(e) =>
            (e.currentTarget.style.boxShadow = "0 0 0 2px #e8601a40")
          }
          onBlur={(e) => (e.currentTarget.style.boxShadow = "none")}
        >
          <option value="all">All Projects</option>
          {projectsData?.data.map((p: Project) => (
            <option key={p.id} value={p.id}>
              {p.name}
            </option>
          ))}
        </select>
      </div>

      {/* Table card */}
      <div
        className="overflow-hidden rounded-lg bg-white"
        style={{ border: "1px solid #e8e8e5" }}
      >
        {isLoading ? (
          <div
            className="py-16 text-center"
            style={{
              fontFamily: "var(--font-dm-sans)",
              fontSize: "13px",
              color: "#6b7280",
            }}
          >
            Loading...
          </div>
        ) : releases.length === 0 ? (
          <div className="py-16 text-center">
            <p
              style={{
                fontFamily: "var(--font-fraunces)",
                fontStyle: "italic",
                fontSize: "15px",
                color: "#9ca3af",
              }}
            >
              No releases ingested yet
            </p>
          </div>
        ) : (
          <table className="w-full">
            <thead>
              <tr style={{ borderBottom: "1px solid #e8e8e5", backgroundColor: "#fafaf9" }}>
                {["Project", "Provider", "Repository", "Version", "Released", "Age"].map(
                  (col) => (
                    <th
                      key={col}
                      className="px-4 py-3 text-left"
                      style={{
                        fontFamily: "var(--font-dm-sans)",
                        fontSize: "11px",
                        fontWeight: 600,
                        textTransform: "uppercase" as const,
                        letterSpacing: "0.05em",
                        color: "#9ca3af",
                      }}
                    >
                      {col}
                    </th>
                  )
                )}
              </tr>
            </thead>
            <tbody>
              {releases.map((release) => (
                <tr
                  key={release.id}
                  className="transition-colors hover:bg-[#fafaf9]"
                  style={{ borderBottom: "1px solid #e8e8e5" }}
                >
                  {/* Project */}
                  <td className="px-4 py-3">
                    {release._projectId ? (
                      <Link
                        href={`/projects/${release._projectId}`}
                        className="hover:underline"
                        style={{
                          fontFamily: "var(--font-dm-sans)",
                          fontSize: "13px",
                          color: "#1a1a1a",
                          fontWeight: 500,
                        }}
                      >
                        {release._projectName ?? "\u2014"}
                      </Link>
                    ) : (
                      <span
                        style={{
                          fontFamily: "var(--font-dm-sans)",
                          fontSize: "13px",
                          color: "#9ca3af",
                        }}
                      >
                        {"\u2014"}
                      </span>
                    )}
                  </td>

                  {/* Provider */}
                  <td className="px-4 py-3">
                    {release._provider ? (
                      <ProviderBadge provider={release._provider} />
                    ) : (
                      <span
                        style={{
                          fontFamily: "var(--font-dm-sans)",
                          fontSize: "13px",
                          color: "#9ca3af",
                        }}
                      >
                        {"\u2014"}
                      </span>
                    )}
                  </td>

                  {/* Repository */}
                  <td className="px-4 py-3">
                    <span
                      style={{
                        fontFamily: "'JetBrains Mono', monospace",
                        fontSize: "12px",
                        color: "#374151",
                      }}
                    >
                      {release._repository ?? release.source_id}
                    </span>
                  </td>

                  {/* Version */}
                  <td className="px-4 py-3">
                    <Link href={`/releases/${release.id}`}>
                      <VersionChip version={release.version} />
                    </Link>
                  </td>

                  {/* Released date */}
                  <td className="px-4 py-3">
                    <span
                      style={{
                        fontFamily: "var(--font-dm-sans)",
                        fontSize: "13px",
                        color: "#6b7280",
                      }}
                    >
                      {release.released_at
                        ? new Date(release.released_at).toLocaleDateString()
                        : "\u2014"}
                    </span>
                  </td>

                  {/* Age */}
                  <td className="px-4 py-3">
                    <span
                      style={{
                        fontFamily: "var(--font-dm-sans)",
                        fontSize: "13px",
                        color: "#9ca3af",
                      }}
                    >
                      {timeAgo(release.released_at ?? release.created_at)}
                    </span>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        )}
      </div>

      {/* Pagination */}
      {total > 0 && (
        <div className="flex items-center justify-between">
          <span
            style={{
              fontFamily: "var(--font-dm-sans)",
              fontSize: "13px",
              color: "#9ca3af",
            }}
          >
            {startRow}&ndash;{endRow} of {total}
          </span>
          <div className="flex items-center gap-2">
            <button
              disabled={page <= 1}
              onClick={() => setPage(page - 1)}
              className="rounded-md bg-white px-3 py-1.5 transition-colors hover:bg-[#fafaf9] disabled:cursor-not-allowed disabled:opacity-40"
              style={{
                fontFamily: "var(--font-dm-sans)",
                fontSize: "13px",
                color: "#374151",
                border: "1px solid #e8e8e5",
              }}
            >
              Previous
            </button>
            <button
              disabled={page >= totalPages}
              onClick={() => setPage(page + 1)}
              className="rounded-md bg-white px-3 py-1.5 transition-colors hover:bg-[#fafaf9] disabled:cursor-not-allowed disabled:opacity-40"
              style={{
                fontFamily: "var(--font-dm-sans)",
                fontSize: "13px",
                color: "#374151",
                border: "1px solid #e8e8e5",
              }}
            >
              Next
            </button>
          </div>
        </div>
      )}
    </div>
  );
}
