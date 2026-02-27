// web/components/dashboard/stats-cards.tsx
"use client";

import useSWR from "swr";
import { system } from "@/lib/api/client";
import { FolderKanban, TrendingUp, AlertTriangle } from "lucide-react";
import type { LucideIcon } from "lucide-react";

interface StatItem {
  label: string;
  value: number | string;
  icon: LucideIcon;
  accent?: boolean;
}

export function StatsCards() {
  const { data, isLoading } = useSWR("stats", () => system.stats());

  const stats = data?.data;
  const attentionCount = stats?.attention_needed ?? 0;

  const items: StatItem[] = [
    { label: "Projects Tracked", value: stats?.total_projects ?? "\u2014", icon: FolderKanban },
    { label: "Releases This Week", value: stats?.releases_this_week ?? "\u2014", icon: TrendingUp },
    { label: "Needs Attention", value: attentionCount, icon: AlertTriangle, accent: attentionCount > 0 },
  ];

  return (
    <div className="grid gap-4 sm:grid-cols-3">
      {items.map((item) => (
        <div
          key={item.label}
          className="relative rounded-lg bg-white px-5 py-4"
          style={{
            border: item.accent ? "1px solid #e8601a" : "1px solid #e8e8e5",
            backgroundColor: item.accent ? "#fff8f0" : "#ffffff",
          }}
        >
          <item.icon
            className="absolute right-4 top-4 h-4 w-4"
            style={{ color: item.accent ? "#e8601a" : "#b0b0a8" }}
          />
          <p
            className="text-xs uppercase tracking-[0.08em]"
            style={{
              fontFamily: "var(--font-dm-sans)",
              fontSize: "12px",
              color: item.accent ? "#e8601a" : "#6b7280",
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
              color: item.accent ? "#e8601a" : "#111113",
            }}
          >
            {isLoading ? "\u00B7\u00B7\u00B7" : item.value}
          </p>
        </div>
      ))}
    </div>
  );
}
