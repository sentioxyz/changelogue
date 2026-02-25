"use client";

import { useState } from "react";
import useSWR from "swr";
import Link from "next/link";
import { agent as agentApi, projects as projectsApi } from "@/lib/api/client";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Badge } from "@/components/ui/badge";
import { ArrowLeft, Play, Bot } from "lucide-react";

const statusColors: Record<string, string> = {
  completed: "bg-green-100 text-green-800",
  running: "bg-blue-100 text-blue-800",
  pending: "bg-gray-100 text-gray-800",
  failed: "bg-red-100 text-red-800",
};

export function AgentPageContent({ projectId }: { projectId: string }) {
  const { data: projectData } = useSWR(`project-${projectId}`, () => projectsApi.get(projectId));
  const { data: runsData, isLoading, mutate } = useSWR(`project-${projectId}-runs`, () => agentApi.listRuns(projectId));
  const [triggering, setTriggering] = useState(false);

  const handleTrigger = async () => {
    setTriggering(true);
    try {
      await agentApi.triggerRun(projectId);
      mutate();
    } finally {
      setTriggering(false);
    }
  };

  const project = projectData?.data;

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
          <h2 className="text-lg font-semibold flex items-center gap-2">
            <Bot className="h-5 w-5" /> Agent Configuration
          </h2>
          <p className="text-sm text-muted-foreground">Manage agent settings and view run history.</p>
        </div>
        <Button onClick={handleTrigger} disabled={triggering}>
          <Play className="mr-2 h-4 w-4" />
          {triggering ? "Triggering..." : "Trigger Run"}
        </Button>
      </div>

      {/* Agent Config Summary */}
      {project && (
        <Card>
          <CardHeader><CardTitle className="text-base">Configuration</CardTitle></CardHeader>
          <CardContent className="space-y-3">
            {project.agent_prompt ? (
              <div>
                <div className="text-xs text-muted-foreground mb-1">Custom Prompt</div>
                <pre className="rounded bg-muted p-3 text-sm whitespace-pre-wrap">{project.agent_prompt}</pre>
              </div>
            ) : (
              <p className="text-sm text-muted-foreground">Using default agent prompt.</p>
            )}
            {project.agent_rules && (
              <div className="flex flex-wrap gap-2">
                {project.agent_rules.on_major_release && <Badge variant="secondary">major</Badge>}
                {project.agent_rules.on_minor_release && <Badge variant="secondary">minor</Badge>}
                {project.agent_rules.on_security_patch && <Badge variant="secondary">security</Badge>}
                {project.agent_rules.version_pattern && (
                  <Badge variant="outline" className="font-mono text-xs">{project.agent_rules.version_pattern}</Badge>
                )}
              </div>
            )}
            <Link href={`/projects/${projectId}/edit`}>
              <Button variant="outline" size="sm">Edit Configuration</Button>
            </Link>
          </CardContent>
        </Card>
      )}

      {/* Run History */}
      <Card>
        <CardHeader><CardTitle className="text-base">Run History</CardTitle></CardHeader>
        <CardContent>
          {isLoading ? (
            <div className="py-8 text-center text-muted-foreground">Loading...</div>
          ) : runsData?.data && runsData.data.length > 0 ? (
            <div className="space-y-2">
              {runsData.data.map((run) => (
                <div key={run.id} className="flex items-center justify-between rounded-md border p-3">
                  <div className="space-y-1">
                    <div className="flex items-center gap-2">
                      <Badge className={statusColors[run.status] ?? ""}>{run.status}</Badge>
                      <span className="text-sm">{run.trigger}</span>
                      {run.semantic_release_id && (
                        <span className="text-xs text-muted-foreground font-mono">
                          SR: {run.semantic_release_id.slice(0, 8)}...
                        </span>
                      )}
                    </div>
                    {run.error && (
                      <p className="text-xs text-red-600">{run.error}</p>
                    )}
                    {run.prompt_used && (
                      <p className="text-xs text-muted-foreground line-clamp-1">Prompt: {run.prompt_used}</p>
                    )}
                  </div>
                  <div className="text-right text-xs text-muted-foreground">
                    {run.started_at && <div>Started: {new Date(run.started_at).toLocaleString()}</div>}
                    {run.completed_at && <div>Completed: {new Date(run.completed_at).toLocaleString()}</div>}
                    {!run.started_at && <div>Pending</div>}
                  </div>
                </div>
              ))}
            </div>
          ) : (
            <div className="py-8 text-center text-muted-foreground">No agent runs yet</div>
          )}
        </CardContent>
      </Card>
    </div>
  );
}
