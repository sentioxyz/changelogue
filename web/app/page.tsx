// web/app/page.tsx
"use client";

import { StatsCards } from "@/components/dashboard/stats-cards";
import { RecentReleases } from "@/components/dashboard/recent-releases";
import { ActivityFeed } from "@/components/dashboard/activity-feed";

export default function DashboardPage() {
  return (
    <div className="space-y-6">
      <StatsCards />
      <div className="grid gap-6 lg:grid-cols-2">
        <RecentReleases />
        <ActivityFeed />
      </div>
    </div>
  );
}
