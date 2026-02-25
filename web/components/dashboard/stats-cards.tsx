// web/components/dashboard/stats-cards.tsx
"use client";

import useSWR from "swr";
import { system } from "@/lib/api/client";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { FolderKanban, Radio, Package, Bot } from "lucide-react";

export function StatsCards() {
  const { data, isLoading } = useSWR("stats", () => system.stats());

  const stats = data?.data;
  const items = [
    { label: "Projects", value: stats?.total_projects ?? "\u2014", icon: FolderKanban, color: "text-blue-600" },
    { label: "Sources", value: stats?.total_sources ?? "\u2014", icon: Radio, color: "text-green-600" },
    { label: "Releases", value: stats?.total_releases ?? "\u2014", icon: Package, color: "text-purple-600" },
    { label: "Pending Agent Runs", value: stats?.pending_agent_runs ?? "\u2014", icon: Bot, color: "text-yellow-600" },
  ];

  return (
    <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-4">
      {items.map((item) => (
        <Card key={item.label}>
          <CardHeader className="flex flex-row items-center justify-between pb-2">
            <CardTitle className="text-sm font-medium text-muted-foreground">{item.label}</CardTitle>
            <item.icon className={`h-4 w-4 ${item.color}`} />
          </CardHeader>
          <CardContent>
            <div className="text-2xl font-bold">{isLoading ? "..." : item.value}</div>
          </CardContent>
        </Card>
      ))}
    </div>
  );
}
