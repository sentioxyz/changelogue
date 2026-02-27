// web/app/page.tsx
"use client";

import useSWR from "swr";
import { StatsCards } from "@/components/dashboard/stats-cards";
import { UnifiedFeed } from "@/components/dashboard/unified-feed";
import { DashboardEmptyState } from "@/components/dashboard/empty-state";
import { projects as projectsApi } from "@/lib/api/client";

export default function DashboardPage() {
  const { data: projectsData, isLoading } = useSWR("projects-for-dashboard", () =>
    projectsApi.list()
  );

  const hasProjects = !isLoading && projectsData?.data && projectsData.data.length > 0;

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

      {hasProjects ? (
        <>
          <StatsCards />
          <UnifiedFeed />
        </>
      ) : isLoading ? (
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
      ) : (
        <DashboardEmptyState />
      )}
    </div>
  );
}
