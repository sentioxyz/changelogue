// web/components/dashboard/stats-cards.tsx
"use client";

import useSWR from "swr";
import { system } from "@/lib/api/client";
import { Package, Radio, Clock, FolderKanban } from "lucide-react";
import type { LucideIcon } from "lucide-react";

interface StatItem {
  label: string;
  value: number | string;
  icon: LucideIcon;
}

export function StatsCards() {
  const { data, isLoading } = useSWR("stats", () => system.stats());

  const stats = data?.data;
  const items: StatItem[] = [
    { label: "Total Releases", value: stats?.total_releases ?? "\u2014", icon: Package },
    { label: "Active Sources", value: stats?.total_sources ?? "\u2014", icon: Radio },
    { label: "Pending Jobs", value: stats?.pending_agent_runs ?? "\u2014", icon: Clock },
    { label: "Total Projects", value: stats?.total_projects ?? "\u2014", icon: FolderKanban },
  ];

  return (
    <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-4">
      {items.map((item) => (
        <div
          key={item.label}
          className="relative rounded-lg bg-white px-5 py-4"
          style={{ border: "1px solid #e8e8e5" }}
        >
          <item.icon
            className="absolute right-4 top-4 h-4 w-4"
            style={{ color: "#b0b0a8" }}
          />
          <p
            className="text-xs uppercase tracking-[0.08em]"
            style={{
              fontFamily: "var(--font-dm-sans)",
              fontSize: "12px",
              color: "#6b7280",
            }}
          >
            {item.label}
          </p>
          <p
            className="mt-1 font-bold"
            style={{
              fontFamily: "var(--font-fraunces)",
              fontSize: "32px",
              lineHeight: 1.1,
              color: "#111113",
            }}
          >
            {isLoading ? "\u00B7\u00B7\u00B7" : item.value}
          </p>
        </div>
      ))}
    </div>
  );
}
