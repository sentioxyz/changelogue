"use client";

import useSWR from "swr";
import Link from "next/link";
import { semanticReleases as srApi } from "@/lib/api/client";
import { Button } from "@/components/ui/button";
import { Card, CardContent } from "@/components/ui/card";
import { Badge } from "@/components/ui/badge";
import { ArrowLeft } from "lucide-react";

const statusColors: Record<string, string> = {
  completed: "bg-green-100 text-green-800",
  running: "bg-blue-100 text-blue-800",
  pending: "bg-gray-100 text-gray-800",
  failed: "bg-red-100 text-red-800",
};

export function SemanticReleasesList({ projectId }: { projectId: string }) {
  const { data, isLoading } = useSWR(`project-${projectId}-sr`, () => srApi.list(projectId));

  return (
    <div className="space-y-4">
      <div className="flex items-center gap-4">
        <Link href={`/projects/${projectId}`}>
          <Button variant="ghost" size="sm">
            <ArrowLeft className="mr-2 h-4 w-4" /> Back to Project
          </Button>
        </Link>
      </div>
      <div>
        <h2 className="text-lg font-semibold">Semantic Releases</h2>
        <p className="text-sm text-muted-foreground">Agent-analyzed releases with intelligence reports.</p>
      </div>

      {isLoading ? (
        <div className="py-12 text-center text-muted-foreground">Loading...</div>
      ) : data?.data && data.data.length > 0 ? (
        <div className="space-y-3">
          {data.data.map((sr) => (
            <Link key={sr.id} href={`/projects/${projectId}/semantic-releases/${sr.id}`}>
              <Card className="hover:bg-muted/30 transition-colors cursor-pointer">
                <CardContent className="p-4">
                  <div className="flex items-center justify-between">
                    <div className="flex items-center gap-3">
                      <span className="font-mono font-medium text-lg">{sr.version}</span>
                      <Badge className={statusColors[sr.status] ?? ""}>{sr.status}</Badge>
                    </div>
                    <span className="text-sm text-muted-foreground">
                      {new Date(sr.created_at).toLocaleDateString()}
                    </span>
                  </div>
                  {sr.report && (
                    <p className="mt-2 text-sm text-muted-foreground line-clamp-2">{sr.report.summary}</p>
                  )}
                  {sr.error && (
                    <p className="mt-2 text-sm text-red-600">{sr.error}</p>
                  )}
                </CardContent>
              </Card>
            </Link>
          ))}
        </div>
      ) : (
        <Card>
          <CardContent className="py-12 text-center text-muted-foreground">
            No semantic releases yet. Releases will be analyzed once the agent runs.
          </CardContent>
        </Card>
      )}
    </div>
  );
}
