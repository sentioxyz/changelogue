// web/components/dashboard/recent-releases.tsx
"use client";

import useSWR from "swr";
import Link from "next/link";
import {
  projects as projectsApi,
  releases as releasesApi,
  sources as sourcesApi,
} from "@/lib/api/client";
import { VersionChip } from "@/components/ui/version-chip";
import type { Release, Source } from "@/lib/api/types";

function timeAgo(dateStr: string): string {
  const diff = Date.now() - new Date(dateStr).getTime();
  const mins = Math.floor(diff / 60000);
  if (mins < 60) return `${mins}m ago`;
  const hrs = Math.floor(mins / 60);
  if (hrs < 24) return `${hrs}h ago`;
  return `${Math.floor(hrs / 24)}d ago`;
}

interface ReleaseRow extends Release {
  _repository?: string;
}

export function RecentReleases() {
  const { data: projectsData } = useSWR("projects-for-dashboard", () =>
    projectsApi.list()
  );

  const { data: allReleases, isLoading } = useSWR(
    projectsData ? "recent-releases" : null,
    async () => {
      if (!projectsData?.data?.length) return [];

      // Fetch releases and sources in parallel per project
      const projectSlice = projectsData.data.slice(0, 10);
      const [releaseResults, sourceResults] = await Promise.all([
        Promise.all(
          projectSlice.map((p) =>
            releasesApi.listByProject(p.id, 1).catch(() => null)
          )
        ),
        Promise.all(
          projectSlice.map((p) =>
            sourcesApi.listByProject(p.id, 1).catch(() => null)
          )
        ),
      ]);

      // Build source_id -> repository map
      const sourceMap = new Map<string, string>();
      sourceResults
        .filter((r): r is NonNullable<typeof r> => r !== null)
        .flatMap((r) => r.data)
        .forEach((s: Source) => sourceMap.set(s.id, s.repository));

      const releases: ReleaseRow[] = releaseResults
        .filter((r): r is NonNullable<typeof r> => r !== null)
        .flatMap((r) => r.data)
        .sort(
          (a, b) =>
            new Date(b.created_at).getTime() - new Date(a.created_at).getTime()
        )
        .slice(0, 8)
        .map((rel) => ({
          ...rel,
          _repository: sourceMap.get(rel.source_id),
        }));

      return releases;
    }
  );

  return (
    <div
      className="rounded-lg bg-white"
      style={{ border: "1px solid #e8e8e5" }}
    >
      {/* Header */}
      <div
        className="flex items-center justify-between px-5 py-4"
        style={{ borderBottom: "1px solid #e8e8e5" }}
      >
        <h3
          className="font-medium"
          style={{
            fontFamily: "var(--font-dm-sans)",
            fontSize: "13px",
            color: "#1a1a1a",
          }}
        >
          Recent Source Releases
        </h3>
        <Link
          href="/releases"
          className="text-sm hover:underline"
          style={{
            fontFamily: "var(--font-dm-sans)",
            fontSize: "13px",
            color: "#e8601a",
          }}
        >
          View all &rarr;
        </Link>
      </div>

      {/* Body */}
      <div>
        {isLoading ? (
          <div
            className="py-12 text-center"
            style={{
              fontFamily: "var(--font-dm-sans)",
              fontSize: "13px",
              color: "#6b7280",
            }}
          >
            Loading...
          </div>
        ) : allReleases && allReleases.length > 0 ? (
          allReleases.map((release: ReleaseRow, idx: number) => (
            <div
              key={release.id}
              className="flex items-center justify-between px-5 py-3"
              style={
                idx < allReleases.length - 1
                  ? { borderBottom: "1px solid #e8e8e5" }
                  : undefined
              }
            >
              <span
                className="min-w-0 flex-1 truncate"
                style={{
                  fontFamily: "'JetBrains Mono', monospace",
                  fontSize: "13px",
                  color: "#6b7280",
                }}
              >
                {release._repository ?? release.source_id.slice(0, 12)}
              </span>
              <div className="ml-3 flex items-center gap-3">
                <VersionChip version={release.version} />
                <span
                  className="whitespace-nowrap"
                  style={{
                    fontFamily: "var(--font-dm-sans)",
                    fontSize: "12px",
                    color: "#9ca3af",
                  }}
                >
                  {timeAgo(release.released_at ?? release.created_at)}
                </span>
              </div>
            </div>
          ))
        ) : (
          <div className="py-12 text-center">
            <p
              style={{
                fontFamily: "var(--font-fraunces)",
                fontStyle: "italic",
                fontSize: "14px",
                color: "#9ca3af",
              }}
            >
              No releases yet
            </p>
          </div>
        )}
      </div>
    </div>
  );
}
