// web/app/page.tsx
"use client";

import useSWR from "swr";
import Link from "next/link";
import { StatsCards } from "@/components/dashboard/stats-cards";
import { RecentReleases } from "@/components/dashboard/recent-releases";
import {
  projects as projectsApi,
  semanticReleases as srApi,
} from "@/lib/api/client";
import { StatusDot } from "@/components/ui/status-dot";
import { VersionChip } from "@/components/ui/version-chip";
import type { SemanticRelease } from "@/lib/api/types";
import { timeAgo } from "@/lib/format";

interface SRRow extends SemanticRelease {
  _projectName?: string;
}

function SemanticReleasesColumn() {
  const { data: projectsData } = useSWR("projects-for-sr-dashboard", () =>
    projectsApi.list()
  );

  const { data: allSRs, isLoading } = useSWR(
    projectsData ? "recent-semantic-releases" : null,
    async () => {
      if (!projectsData?.data?.length) return [];

      const projectMap = new Map(
        projectsData.data.map((p) => [p.id, p.name])
      );

      const results = await Promise.all(
        projectsData.data.slice(0, 10).map((p) =>
          srApi.list(p.id, 1).catch(() => null)
        )
      );

      const srs: SRRow[] = results
        .filter((r): r is NonNullable<typeof r> => r !== null)
        .flatMap((r) => r.data)
        .sort(
          (a, b) =>
            new Date(b.created_at).getTime() - new Date(a.created_at).getTime()
        )
        .slice(0, 6)
        .map((sr) => ({
          ...sr,
          _projectName: projectMap.get(sr.project_id),
        }));

      return srs;
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
            color: "#111113",
          }}
        >
          Semantic Releases
        </h3>
        <Link
          href="/semantic-releases"
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
      <div className="p-4 space-y-3">
        {isLoading ? (
          <div
            className="py-8 text-center"
            style={{
              fontFamily: "var(--font-dm-sans)",
              fontSize: "13px",
              color: "#6b7280",
            }}
          >
            Loading...
          </div>
        ) : allSRs && allSRs.length > 0 ? (
          allSRs.map((sr: SRRow) => (
            <Link
              key={sr.id}
              href={`/projects/${sr.project_id}/semantic-releases/${sr.id}`}
              className="block rounded-lg px-4 py-3 transition-colors hover:bg-[#fafaf9]"
              style={{ border: "1px solid #e8e8e5" }}
            >
              <div className="flex items-center justify-between">
                <span
                  className="font-semibold"
                  style={{
                    fontFamily: "var(--font-fraunces)",
                    fontSize: "15px",
                    color: "#111113",
                  }}
                >
                  {sr._projectName ?? "Unknown Project"}
                </span>
                <VersionChip version={sr.version} />
              </div>
              {sr.report?.summary && (
                <p
                  className="mt-1 line-clamp-1"
                  style={{
                    fontFamily: "var(--font-dm-sans)",
                    fontStyle: "italic",
                    fontSize: "13px",
                    color: "#6b7280",
                  }}
                >
                  {sr.report.summary}
                </p>
              )}
              <div className="mt-2 flex items-center gap-2">
                <StatusDot status={sr.status} />
                <span
                  style={{
                    fontFamily: "var(--font-dm-sans)",
                    fontSize: "12px",
                    color: "#9ca3af",
                  }}
                >
                  {timeAgo(sr.created_at)}
                </span>
              </div>
            </Link>
          ))
        ) : (
          <div className="py-8 text-center">
            <p
              style={{
                fontFamily: "var(--font-fraunces)",
                fontStyle: "italic",
                fontSize: "14px",
                color: "#9ca3af",
              }}
            >
              No semantic releases yet
            </p>
          </div>
        )}
      </div>
    </div>
  );
}

export default function DashboardPage() {
  return (
    <div className="space-y-6">
      <h1
        style={{
          fontFamily: "var(--font-fraunces)",
          fontSize: "24px",
          fontWeight: 700,
          color: "#111113",
        }}
      >
        Dashboard
      </h1>
      <StatsCards />
      <div className="grid gap-6 lg:grid-cols-2">
        <RecentReleases />
        <SemanticReleasesColumn />
      </div>
    </div>
  );
}
