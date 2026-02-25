"use client";

import useSWR from "swr";
import Link from "next/link";
import { releases as releasesApi } from "@/lib/api/client";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { ArrowLeft } from "lucide-react";

export function ReleaseDetail({ id }: { id: string }) {
  const { data: releaseData, isLoading } = useSWR(
    `release-${id}`, () => releasesApi.get(id)
  );

  if (isLoading) return <div className="py-12 text-center text-muted-foreground">Loading...</div>;

  const release = releaseData?.data;
  if (!release) return <div className="py-12 text-center">Release not found</div>;

  return (
    <div className="space-y-6">
      <Link href="/releases">
        <Button variant="ghost" size="sm">
          <ArrowLeft className="mr-2 h-4 w-4" /> Back to Releases
        </Button>
      </Link>

      {/* Release Header */}
      <div>
        <h2 className="text-2xl font-bold font-mono">{release.version}</h2>
        <div className="mt-1 flex items-center gap-2 text-muted-foreground">
          <span className="text-sm">Source: {release.source_id}</span>
          <span>&middot;</span>
          <span>{new Date(release.created_at).toLocaleString()}</span>
        </div>
      </div>

      <div className="grid gap-6 lg:grid-cols-2">
        <Card>
          <CardHeader><CardTitle className="text-base">Version Details</CardTitle></CardHeader>
          <CardContent className="space-y-2 text-sm">
            <div className="flex justify-between">
              <span className="text-muted-foreground">Version</span>
              <span className="font-mono">{release.version}</span>
            </div>
            <div className="flex justify-between">
              <span className="text-muted-foreground">Source ID</span>
              <span className="font-mono text-xs">{release.source_id}</span>
            </div>
            {release.released_at && (
              <div className="flex justify-between">
                <span className="text-muted-foreground">Released At</span>
                <span>{new Date(release.released_at).toLocaleString()}</span>
              </div>
            )}
            <div className="flex justify-between">
              <span className="text-muted-foreground">Ingested At</span>
              <span>{new Date(release.created_at).toLocaleString()}</span>
            </div>
          </CardContent>
        </Card>

        {release.raw_data && Object.keys(release.raw_data).length > 0 && (
          <Card>
            <CardHeader><CardTitle className="text-base">Raw Data</CardTitle></CardHeader>
            <CardContent>
              <pre className="rounded bg-muted p-3 text-xs overflow-x-auto whitespace-pre-wrap">
                {JSON.stringify(release.raw_data, null, 2)}
              </pre>
            </CardContent>
          </Card>
        )}
      </div>
    </div>
  );
}
