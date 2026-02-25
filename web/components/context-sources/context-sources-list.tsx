"use client";

import useSWR from "swr";
import Link from "next/link";
import { contextSources as ctxApi } from "@/lib/api/client";
import { Button } from "@/components/ui/button";
import { Card, CardContent } from "@/components/ui/card";
import { Badge } from "@/components/ui/badge";
import { Plus, ArrowLeft } from "lucide-react";

export function ContextSourcesList({ projectId }: { projectId: string }) {
  const { data, isLoading } = useSWR(`project-${projectId}-ctx`, () => ctxApi.list(projectId));

  return (
    <div className="space-y-4">
      <div className="flex items-center gap-4">
        <Link href={`/projects/${projectId}`}>
          <Button variant="ghost" size="sm">
            <ArrowLeft className="mr-2 h-4 w-4" /> Back to Project
          </Button>
        </Link>
      </div>
      <div className="flex items-center justify-between">
        <div>
          <h2 className="text-lg font-semibold">Context Sources</h2>
          <p className="text-sm text-muted-foreground">External data sources providing context for agent analysis.</p>
        </div>
        <Link href={`/projects/${projectId}/context-sources/new`}>
          <Button><Plus className="mr-2 h-4 w-4" />Add Context Source</Button>
        </Link>
      </div>
      <Card>
        <CardContent className="p-0">
          {isLoading ? (
            <div className="py-12 text-center text-muted-foreground">Loading...</div>
          ) : data?.data && data.data.length > 0 ? (
            <div className="divide-y">
              {data.data.map((ctx) => (
                <div key={ctx.id} className="flex items-center justify-between p-4">
                  <div>
                    <div className="flex items-center gap-2">
                      <Badge variant="outline">{ctx.type}</Badge>
                      <span className="font-medium">{ctx.name}</span>
                    </div>
                    <div className="mt-1 text-xs text-muted-foreground">
                      Created {new Date(ctx.created_at).toLocaleDateString()}
                    </div>
                  </div>
                  <pre className="max-w-xs truncate text-xs text-muted-foreground font-mono">
                    {JSON.stringify(ctx.config)}
                  </pre>
                </div>
              ))}
            </div>
          ) : (
            <div className="py-12 text-center text-muted-foreground">No context sources configured</div>
          )}
        </CardContent>
      </Card>
    </div>
  );
}
