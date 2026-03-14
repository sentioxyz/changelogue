// web/app/page.tsx
"use client";

import { useEffect, useState } from "react";
import useSWR, { mutate } from "swr";
import { useRouter } from "next/navigation";
import { StatsCards } from "@/components/dashboard/stats-cards";
import { ReleaseTrendChart } from "@/components/dashboard/release-trend-chart";
import { UnifiedFeed } from "@/components/dashboard/unified-feed";
import { DashboardEmptyState } from "@/components/dashboard/empty-state";
import { DiscoverySection } from "@/components/dashboard/discovery-section";
import { projects as projectsApi } from "@/lib/api/client";
import { Search } from "lucide-react";

const SSE_BASE = process.env.NEXT_PUBLIC_API_URL || "/api/v1";

export default function DashboardPage() {
  const router = useRouter();
  const [repoUrl, setRepoUrl] = useState("");
  const { data: projectsData, isLoading } = useSWR("projects-for-dashboard", () =>
    projectsApi.list()
  );

  // Listen to SSE events and revalidate dashboard data on new releases
  useEffect(() => {
    let es: EventSource | null = null;
    let retryTimer: ReturnType<typeof setTimeout> | null = null;

    function connect() {
      try {
        es = new EventSource(`${SSE_BASE}/events`);
        es.onmessage = () => {
          // Revalidate all dashboard SWR keys on any event
          mutate((key) => typeof key === "string" && (
            key === "stats" ||
            key.startsWith("trend-") ||
            key === "unified-feed"
          ), undefined, { revalidate: true });
        };
        es.onerror = () => {
          es?.close();
          retryTimer = setTimeout(connect, 5000);
        };
      } catch {
        retryTimer = setTimeout(connect, 5000);
      }
    }

    connect();
    return () => {
      es?.close();
      if (retryTimer) clearTimeout(retryTimer);
    };
  }, []);

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

      <DiscoverySection />

      {/* Quick Onboard inline */}
      <div
        className="rounded-lg bg-white px-5 py-3"
        style={{ border: "1px solid #e8e8e5" }}
      >
        <div className="flex items-center gap-4">
          <span
            className="flex-shrink-0"
            style={{
              fontFamily: "var(--font-fraunces)",
              fontSize: "14px",
              fontWeight: 600,
              color: "#111113",
            }}
          >
            Quick Onboard
          </span>
          <div className="flex flex-1 items-center gap-2">
            <input
              type="text"
              value={repoUrl}
              onChange={(e) => setRepoUrl(e.target.value)}
              placeholder="owner/repo or https://github.com/owner/repo"
              className="flex-1 rounded-md border px-3 py-1.5 text-[13px] focus:outline-none"
              style={{
                borderColor: "#e8e8e5",
                color: "#111113",
                fontFamily: "'JetBrains Mono', monospace",
                maxWidth: 400,
              }}
              onFocus={(e) => { e.currentTarget.style.borderColor = "#e8601a"; }}
              onBlur={(e) => { e.currentTarget.style.borderColor = "#e8e8e5"; }}
              onKeyDown={(e) => {
                if (e.key === "Enter" && repoUrl.trim()) {
                  router.push(`/onboard?repo=${encodeURIComponent(repoUrl.trim())}`);
                }
              }}
            />
            <button
              onClick={() => {
                if (repoUrl.trim()) {
                  router.push(`/onboard?repo=${encodeURIComponent(repoUrl.trim())}`);
                }
              }}
              disabled={!repoUrl.trim()}
              className="flex items-center gap-1.5 rounded-md px-3 py-1.5 text-[13px] font-medium text-white transition-opacity hover:opacity-90 disabled:opacity-40 disabled:cursor-not-allowed"
              style={{ backgroundColor: "#e8601a", fontFamily: "var(--font-dm-sans)" }}
            >
              <Search className="h-3.5 w-3.5" />
              Scan
            </button>
          </div>
          <span className="text-[12px] flex-shrink-0" style={{ color: "#9ca3af" }}>
            Scan a repo for dependencies
          </span>
        </div>
      </div>

      {hasProjects ? (
        <>
          <StatsCards />
          <div className="grid gap-4 lg:grid-cols-2">
            <ReleaseTrendChart />
            <UnifiedFeed />
          </div>
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
