// web/app/page.tsx
"use client";

import { StatsCards } from "@/components/dashboard/stats-cards";
import { RecentReleases } from "@/components/dashboard/recent-releases";

export default function DashboardPage() {
  return (
    <div className="space-y-6">
      <StatsCards />
      <RecentReleases />
    </div>
  );
}
