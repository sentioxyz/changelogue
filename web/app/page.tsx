// web/app/page.tsx
"use client";

import { useEffect } from "react";
import useSWR, { mutate } from "swr";
import { StatsCards } from "@/components/dashboard/stats-cards";
import { ReleaseTrendChart } from "@/components/dashboard/release-trend-chart";
import { UnifiedFeed } from "@/components/dashboard/unified-feed";
import { DashboardEmptyState } from "@/components/dashboard/empty-state";
import { DiscoverySection } from "@/components/dashboard/discovery-section";
import { SuggestionsSection } from "@/components/dashboard/suggestions-section";
import { projects as projectsApi } from "@/lib/api/client";
import { useAuth } from "@/lib/auth/context";
import { useTranslation } from "@/lib/i18n/context";

const SSE_BASE = process.env.NEXT_PUBLIC_API_URL || "/api/v1";

export default function DashboardPage() {
  const { user } = useAuth();
  const { t } = useTranslation();
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
      <div>
        <h1
          style={{
            fontFamily: "var(--font-fraunces)",
            fontSize: "24px",
            fontWeight: 700,
            color: "var(--foreground)",
          }}
        >
          {t("dashboard.title")}
        </h1>
        <p className="mt-1 text-[13px] text-text-secondary" style={{ fontFamily: "var(--font-dm-sans)" }}>
          {t("dashboard.description")}
        </p>
      </div>

      {user?.github_login && user.github_login !== "dev" ? (
        <SuggestionsSection showAuthTabs />
      ) : (
        <>
          <DiscoverySection />
          <SuggestionsSection />
        </>
      )}

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
          className="py-16 text-center text-text-secondary"
          style={{
            fontFamily: "var(--font-dm-sans)",
            fontSize: "13px",
          }}
        >
          {t("dashboard.loading")}
        </div>
      ) : (
        <DashboardEmptyState />
      )}
    </div>
  );
}
