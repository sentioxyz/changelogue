// web/components/dashboard/recent-releases.tsx
"use client";

import useSWR from "swr";
import Link from "next/link";
import { projects as projectsApi, releases as releasesApi } from "@/lib/api/client";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import type { Release } from "@/lib/api/types";

export function RecentReleases() {
  const { data: projectsData } = useSWR("projects-for-dashboard", () => projectsApi.list());

  const { data: allReleases, isLoading } = useSWR(
    projectsData ? "recent-releases" : null,
    async () => {
      if (!projectsData?.data?.length) return [];
      const results = await Promise.all(
        projectsData.data.slice(0, 10).map((p) =>
          releasesApi.listByProject(p.id, 1).catch(() => null)
        )
      );
      return results
        .filter((r): r is NonNullable<typeof r> => r !== null)
        .flatMap((r) => r.data)
        .sort((a, b) => new Date(b.created_at).getTime() - new Date(a.created_at).getTime())
        .slice(0, 8);
    }
  );

  return (
    <Card>
      <CardHeader className="flex flex-row items-center justify-between">
        <CardTitle>Recent Releases</CardTitle>
        <Link href="/releases" className="text-sm text-primary hover:underline">
          View all
        </Link>
      </CardHeader>
      <CardContent>
        {isLoading ? (
          <div className="py-8 text-center text-muted-foreground">Loading...</div>
        ) : allReleases && allReleases.length > 0 ? (
          <div className="space-y-3">
            {allReleases.map((release: Release) => (
              <Link
                key={release.id}
                href={`/releases/${release.id}`}
                className="flex items-center justify-between rounded-md border p-3 transition-colors hover:bg-muted/50"
              >
                <div className="min-w-0 flex-1">
                  <span className="font-mono text-sm font-medium">{release.version}</span>
                  <div className="mt-1 text-xs text-muted-foreground">
                    {release.released_at
                      ? new Date(release.released_at).toLocaleDateString()
                      : new Date(release.created_at).toLocaleDateString()}
                  </div>
                </div>
                <span className="text-xs text-muted-foreground font-mono">
                  {release.source_id.slice(0, 8)}...
                </span>
              </Link>
            ))}
          </div>
        ) : (
          <div className="py-8 text-center text-muted-foreground">No releases yet</div>
        )}
      </CardContent>
    </Card>
  );
}
