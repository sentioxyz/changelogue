"use client";

import { useState } from "react";
import useSWR from "swr";
import Link from "next/link";
import { agent as agentApi, projects as projectsApi } from "@/lib/api/client";
import { getPathSegment } from "@/lib/path";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Badge } from "@/components/ui/badge";
import { ArrowLeft, Play, Bot } from "lucide-react";
import { useTranslation } from "@/lib/i18n/context";

const statusColors: Record<string, string> = {
  completed: "bg-green-100 text-green-800 dark:bg-green-900/30 dark:text-green-400",
  running: "bg-blue-100 text-blue-800 dark:bg-blue-900/30 dark:text-blue-400",
  pending: "bg-mono-bg text-secondary-foreground dark:bg-mono-bg dark:text-text-muted",
  failed: "bg-red-100 text-red-800 dark:bg-red-900/30 dark:text-red-400",
};

export function AgentPageContent() {
  // Read ID from URL path — useParams() returns stale "0" in static export
  const projectId = getPathSegment(1); // /projects/{id}/agent
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
  const { t } = useTranslation();

  return (
    <div className="space-y-4">
      <div className="flex items-center gap-4">
        <Link href={`/projects/${projectId}`}>
          <Button variant="ghost" size="sm">
            <ArrowLeft className="mr-2 h-4 w-4" /> {t("agent.backToProject")}
          </Button>
        </Link>
      </div>

      <div className="flex items-center justify-between">
        <div>
          <h2 className="text-lg font-semibold flex items-center gap-2">
            <Bot className="h-5 w-5" /> {t("agent.configuration")}
          </h2>
          <p className="text-sm text-muted-foreground">{t("agent.configDescription")}</p>
        </div>
        <Button onClick={handleTrigger} disabled={triggering}>
          <Play className="mr-2 h-4 w-4" />
          {triggering ? t("agent.triggering") : t("agent.triggerRun")}
        </Button>
      </div>

      {/* Agent Config Summary */}
      {project && (
        <Card>
          <CardHeader><CardTitle className="text-base">{t("agent.configuration")}</CardTitle></CardHeader>
          <CardContent className="space-y-3">
            {project.agent_prompt ? (
              <div>
                <div className="text-xs text-muted-foreground mb-1">{t("agent.customPrompt")}</div>
                <pre className="rounded bg-muted p-3 text-sm whitespace-pre-wrap">{project.agent_prompt}</pre>
              </div>
            ) : (
              <p className="text-sm text-muted-foreground">{t("agent.defaultPrompt")}</p>
            )}
            {project.agent_rules && (
              <div className="flex flex-wrap gap-2">
                {project.agent_rules.on_major_release && <Badge variant="secondary">{t("agent.ruleMajor")}</Badge>}
                {project.agent_rules.on_minor_release && <Badge variant="secondary">{t("agent.ruleMinor")}</Badge>}
                {project.agent_rules.on_security_patch && <Badge variant="secondary">{t("agent.ruleSecurity")}</Badge>}
                {project.agent_rules.version_pattern && (
                  <Badge variant="outline" className="font-mono text-xs">{project.agent_rules.version_pattern}</Badge>
                )}
              </div>
            )}
            <Link href={`/projects/${projectId}/edit`}>
              <Button variant="outline" size="sm">{t("agent.editConfiguration")}</Button>
            </Link>
          </CardContent>
        </Card>
      )}

      {/* Run History */}
      <Card>
        <CardHeader><CardTitle className="text-base">{t("agent.runHistory")}</CardTitle></CardHeader>
        <CardContent>
          {isLoading ? (
            <div className="py-8 text-center text-muted-foreground">{t("agent.loading")}</div>
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
                      <p className="text-xs text-muted-foreground line-clamp-1">{t("agent.prompt")}: {run.prompt_used}</p>
                    )}
                  </div>
                  <div className="text-right text-xs text-muted-foreground">
                    {run.started_at && <div>{t("agent.started")}: {new Date(run.started_at).toLocaleString()}</div>}
                    {run.completed_at && <div>{t("agent.completed")}: {new Date(run.completed_at).toLocaleString()}</div>}
                    {!run.started_at && <div>{t("agent.pending")}</div>}
                  </div>
                </div>
              ))}
            </div>
          ) : (
            <div className="py-8 text-center text-muted-foreground">{t("agent.noRunsYet")}</div>
          )}
        </CardContent>
      </Card>
    </div>
  );
}
