// web/components/dashboard/stats-cards.tsx
"use client";

import useSWR from "swr";
import { system } from "@/lib/api/client";
import { FolderKanban, TrendingUp } from "lucide-react";
import type { LucideIcon } from "lucide-react";
import { useTranslation } from "@/lib/i18n/context";

interface StatItem {
  labelKey: string;
  value: number | string;
  icon: LucideIcon;
}

export function StatsCards() {
  const { t } = useTranslation();
  const { data, isLoading } = useSWR("stats", () => system.stats(), {
    refreshInterval: 30_000,
  });

  const stats = data?.data;

  const items: StatItem[] = [
    { labelKey: "dashboard.stats.trackedProjects", value: stats?.total_projects ?? "\u2014", icon: FolderKanban },
    { labelKey: "dashboard.stats.releasesThisWeek", value: stats?.releases_this_week ?? "\u2014", icon: TrendingUp },
  ];

  return (
    <div className="grid grid-cols-2 gap-4">
      {items.map((item) => (
        <div
          key={item.labelKey}
          className="relative rounded-lg bg-surface px-5 py-4 border border-border"
        >
          <item.icon
            className="absolute right-4 top-4 h-4 w-4 text-text-muted"
          />
          <p
            className="text-xs uppercase tracking-[0.08em] text-text-secondary"
            style={{
              fontFamily: "var(--font-dm-sans)",
              fontSize: "12px",
            }}
          >
            {t(item.labelKey)}
          </p>
          <p
            className="mt-1 font-bold text-foreground"
            style={{
              fontFamily: "var(--font-raleway)",
              fontSize: "32px",
              lineHeight: 1.1,
            }}
          >
            {isLoading ? "\u00B7\u00B7\u00B7" : item.value}
          </p>
        </div>
      ))}
    </div>
  );
}
