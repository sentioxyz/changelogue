"use client";

import useSWR from "swr";
import Link from "next/link";
import { releases as releasesApi } from "@/lib/api/client";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { ArrowLeft } from "lucide-react";
import { PipelineVisualization } from "@/components/releases/pipeline-status";

export function ReleaseDetail({ id }: { id: string }) {
  const { data: releaseData, isLoading: loadingRelease } = useSWR(
    `release-${id}`, () => releasesApi.get(id)
  );
  const { data: pipelineData, isLoading: loadingPipeline } = useSWR(
    `pipeline-${id}`, () => releasesApi.pipeline(id)
  );
  const { data: notesData } = useSWR(`notes-${id}`, () => releasesApi.notes(id));

  if (loadingRelease) return <div className="py-12 text-center text-muted-foreground">Loading...</div>;

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
        <div className="flex items-center gap-3">
          <h2 className="text-2xl font-bold font-mono">{release.raw_version}</h2>
          {release.is_pre_release && <Badge variant="outline">pre-release</Badge>}
        </div>
        <div className="mt-1 flex items-center gap-2 text-muted-foreground">
          <Link href={`/projects/${release.project_id}`} className="text-primary hover:underline">
            {release.project_name}
          </Link>
          <span>&middot;</span>
          <Badge variant="outline">{release.source_type}</Badge>
          <span className="font-mono text-sm">{release.repository}</span>
          <span>&middot;</span>
          <span>{new Date(release.created_at).toLocaleString()}</span>
        </div>
      </div>

      <div className="grid gap-6 lg:grid-cols-3">
        {/* Left column: Metadata + Notes */}
        <div className="space-y-6 lg:col-span-1">
          <Card>
            <CardHeader><CardTitle className="text-base">Version Details</CardTitle></CardHeader>
            <CardContent className="space-y-2 text-sm">
              <div className="flex justify-between">
                <span className="text-muted-foreground">Semantic</span>
                <span className="font-mono">
                  {release.semantic_version.major}.{release.semantic_version.minor}.{release.semantic_version.patch}
                  {release.semantic_version.pre_release && `-${release.semantic_version.pre_release}`}
                </span>
              </div>
              <div className="flex justify-between">
                <span className="text-muted-foreground">Source Type</span>
                <span>{release.source_type}</span>
              </div>
              <div className="flex justify-between">
                <span className="text-muted-foreground">Repository</span>
                <span className="font-mono text-xs">{release.repository}</span>
              </div>
              {Object.entries(release.metadata).map(([k, v]) => (
                <div key={k} className="flex justify-between">
                  <span className="text-muted-foreground">{k}</span>
                  <span className="font-mono text-xs truncate max-w-[200px]">{v}</span>
                </div>
              ))}
            </CardContent>
          </Card>

          {notesData?.data && (
            <Card>
              <CardHeader><CardTitle className="text-base">Release Notes</CardTitle></CardHeader>
              <CardContent>
                <div className="prose prose-sm max-w-none">
                  <pre className="whitespace-pre-wrap text-sm">{notesData.data}</pre>
                </div>
              </CardContent>
            </Card>
          )}
        </div>

        {/* Right column: Pipeline */}
        <div className="lg:col-span-2">
          {loadingPipeline ? (
            <div className="py-8 text-center text-muted-foreground">Loading pipeline...</div>
          ) : pipelineData?.data ? (
            <PipelineVisualization pipeline={pipelineData.data} />
          ) : (
            <Card>
              <CardContent className="py-8 text-center text-muted-foreground">
                No pipeline data available
              </CardContent>
            </Card>
          )}
        </div>
      </div>
    </div>
  );
}
