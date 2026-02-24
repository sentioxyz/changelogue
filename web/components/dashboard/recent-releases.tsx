// web/components/dashboard/recent-releases.tsx
"use client";

import useSWR from "swr";
import Link from "next/link";
import { releases as releasesApi } from "@/lib/api/client";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Badge } from "@/components/ui/badge";

const statusColors: Record<string, string> = {
  completed: "bg-green-100 text-green-800",
  running: "bg-blue-100 text-blue-800",
  available: "bg-gray-100 text-gray-800",
  retry: "bg-yellow-100 text-yellow-800",
  discarded: "bg-red-100 text-red-800",
};

export function RecentReleases() {
  const { data, isLoading } = useSWR("recent-releases", () =>
    releasesApi.list({ per_page: 8, order: "desc" })
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
        ) : (
          <div className="space-y-3">
            {data?.data.map((release) => (
              <Link
                key={release.id}
                href={`/releases/${release.id}`}
                className="flex items-center justify-between rounded-md border p-3 transition-colors hover:bg-muted/50"
              >
                <div className="min-w-0 flex-1">
                  <div className="flex items-center gap-2">
                    <span className="font-medium">{release.project_name}</span>
                    <span className="font-mono text-sm text-muted-foreground">
                      {release.raw_version}
                    </span>
                    {release.is_pre_release && (
                      <Badge variant="outline" className="text-xs">pre-release</Badge>
                    )}
                  </div>
                  <div className="mt-1 text-xs text-muted-foreground">
                    {release.source_type} &middot; {release.repository} &middot;{" "}
                    {new Date(release.created_at).toLocaleDateString()}
                  </div>
                </div>
                <Badge className={statusColors[release.pipeline_status] ?? ""}>
                  {release.pipeline_status}
                </Badge>
              </Link>
            ))}
          </div>
        )}
      </CardContent>
    </Card>
  );
}
