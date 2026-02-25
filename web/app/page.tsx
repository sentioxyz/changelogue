// web/app/page.tsx
"use client";

import useSWR from "swr";
import Link from "next/link";
import { StatsCards } from "@/components/dashboard/stats-cards";
import { RecentReleases } from "@/components/dashboard/recent-releases";
import { ActivityFeed } from "@/components/dashboard/activity-feed";
import { projects as projectsApi, semanticReleases as srApi } from "@/lib/api/client";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Badge } from "@/components/ui/badge";
import type { SemanticRelease } from "@/lib/api/types";

const statusColors: Record<string, string> = {
  completed: "bg-green-100 text-green-800",
  running: "bg-blue-100 text-blue-800",
  pending: "bg-gray-100 text-gray-800",
  failed: "bg-red-100 text-red-800",
};

function RecentSemanticReleases() {
  const { data: projectsData } = useSWR("projects-for-sr-dashboard", () => projectsApi.list());

  const { data: allSRs, isLoading } = useSWR(
    projectsData ? "recent-semantic-releases" : null,
    async () => {
      if (!projectsData?.data?.length) return [];
      const results = await Promise.all(
        projectsData.data.slice(0, 10).map((p) =>
          srApi.list(p.id, 1).catch(() => null)
        )
      );
      return results
        .filter((r): r is NonNullable<typeof r> => r !== null)
        .flatMap((r) => r.data)
        .sort((a, b) => new Date(b.created_at).getTime() - new Date(a.created_at).getTime())
        .slice(0, 6);
    }
  );

  return (
    <Card>
      <CardHeader>
        <CardTitle>Recent Semantic Releases</CardTitle>
      </CardHeader>
      <CardContent>
        {isLoading ? (
          <div className="py-8 text-center text-muted-foreground">Loading...</div>
        ) : allSRs && allSRs.length > 0 ? (
          <div className="space-y-3">
            {allSRs.map((sr: SemanticRelease) => (
              <Link
                key={sr.id}
                href={`/projects/${sr.project_id}/semantic-releases/${sr.id}`}
                className="block rounded-md border p-3 transition-colors hover:bg-muted/50"
              >
                <div className="flex items-center justify-between">
                  <div className="flex items-center gap-2">
                    <span className="font-mono font-medium">{sr.version}</span>
                    <Badge className={statusColors[sr.status] ?? ""}>{sr.status}</Badge>
                  </div>
                  <span className="text-xs text-muted-foreground">
                    {new Date(sr.created_at).toLocaleDateString()}
                  </span>
                </div>
                {sr.report && (
                  <p className="mt-1 text-sm text-muted-foreground line-clamp-1">{sr.report.summary}</p>
                )}
              </Link>
            ))}
          </div>
        ) : (
          <div className="py-8 text-center text-muted-foreground">No semantic releases yet</div>
        )}
      </CardContent>
    </Card>
  );
}

export default function DashboardPage() {
  return (
    <div className="space-y-6">
      <StatsCards />
      <div className="grid gap-6 lg:grid-cols-2">
        <RecentReleases />
        <RecentSemanticReleases />
      </div>
      <ActivityFeed />
    </div>
  );
}
