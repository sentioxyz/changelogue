"use client";

import useSWR from "swr";
import Link from "next/link";
import { semanticReleases as srApi } from "@/lib/api/client";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { ArrowLeft } from "lucide-react";

const statusColors: Record<string, string> = {
  completed: "bg-green-100 text-green-800",
  running: "bg-blue-100 text-blue-800",
  pending: "bg-gray-100 text-gray-800",
  failed: "bg-red-100 text-red-800",
};

export function SemanticReleaseDetail({ projectId, srId }: { projectId: string; srId: string }) {
  const { data, isLoading } = useSWR(`sr-${srId}`, () => srApi.get(srId));

  if (isLoading) return <div className="py-12 text-center text-muted-foreground">Loading...</div>;

  const sr = data?.data;
  if (!sr) return <div className="py-12 text-center">Semantic release not found</div>;

  return (
    <div className="space-y-6">
      <Link href={`/projects/${projectId}/semantic-releases`}>
        <Button variant="ghost" size="sm">
          <ArrowLeft className="mr-2 h-4 w-4" /> Back to Semantic Releases
        </Button>
      </Link>

      <div className="flex items-center gap-3">
        <h2 className="text-2xl font-bold font-mono">{sr.version}</h2>
        <Badge className={statusColors[sr.status] ?? ""}>{sr.status}</Badge>
      </div>

      <div className="flex gap-4 text-sm text-muted-foreground">
        <span>Created: {new Date(sr.created_at).toLocaleString()}</span>
        {sr.completed_at && <span>Completed: {new Date(sr.completed_at).toLocaleString()}</span>}
      </div>

      {sr.error && (
        <Card className="border-red-200">
          <CardContent className="p-4">
            <div className="text-sm text-red-700">{sr.error}</div>
          </CardContent>
        </Card>
      )}

      {sr.report && (
        <div className="grid gap-6 lg:grid-cols-2">
          <Card className="lg:col-span-2">
            <CardHeader><CardTitle className="text-base">Summary</CardTitle></CardHeader>
            <CardContent>
              <p className="text-sm leading-relaxed">{sr.report.summary}</p>
            </CardContent>
          </Card>

          <Card>
            <CardHeader><CardTitle className="text-base">Availability</CardTitle></CardHeader>
            <CardContent>
              <p className="text-sm leading-relaxed">{sr.report.availability}</p>
            </CardContent>
          </Card>

          <Card>
            <CardHeader><CardTitle className="text-base">Adoption</CardTitle></CardHeader>
            <CardContent>
              <p className="text-sm leading-relaxed">{sr.report.adoption}</p>
            </CardContent>
          </Card>

          <Card>
            <CardHeader><CardTitle className="text-base">Urgency</CardTitle></CardHeader>
            <CardContent>
              <p className="text-sm leading-relaxed">{sr.report.urgency}</p>
            </CardContent>
          </Card>

          <Card>
            <CardHeader><CardTitle className="text-base">Recommendation</CardTitle></CardHeader>
            <CardContent>
              <p className="text-sm leading-relaxed">{sr.report.recommendation}</p>
            </CardContent>
          </Card>
        </div>
      )}
    </div>
  );
}
