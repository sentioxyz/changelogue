"use client";

import useSWR from "swr";
import Link from "next/link";
import { useState } from "react";
import { useRouter } from "next/navigation";
import { projects as projectsApi, sources as sourcesApi, contextSources as ctxApi, semanticReleases as srApi, agent as agentApi } from "@/lib/api/client";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Badge } from "@/components/ui/badge";
import { Pencil, Trash2, ArrowLeft, Play, Bot, Radio, BookOpen, FileText } from "lucide-react";

const tabs = [
  { key: "overview", label: "Overview" },
  { key: "sources", label: "Sources" },
  { key: "context", label: "Context Sources" },
  { key: "semantic", label: "Semantic Releases" },
  { key: "agent", label: "Agent" },
] as const;

type TabKey = (typeof tabs)[number]["key"];

export function ProjectDetail({ id }: { id: string }) {
  const router = useRouter();
  const [activeTab, setActiveTab] = useState<TabKey>("overview");
  const { data, isLoading } = useSWR(`project-${id}`, () => projectsApi.get(id));
  const { data: sourcesData } = useSWR(activeTab === "sources" ? `project-${id}-sources` : null, () => sourcesApi.listByProject(id));
  const { data: ctxData } = useSWR(activeTab === "context" ? `project-${id}-ctx` : null, () => ctxApi.list(id));
  const { data: srData } = useSWR(activeTab === "semantic" ? `project-${id}-sr` : null, () => srApi.list(id));
  const { data: runsData, mutate: mutateRuns } = useSWR(activeTab === "agent" ? `project-${id}-runs` : null, () => agentApi.listRuns(id));
  const [triggering, setTriggering] = useState(false);

  const handleDelete = async () => {
    if (!confirm("Delete this project? This will cascade to sources and subscriptions.")) return;
    await projectsApi.delete(id);
    router.push("/projects");
  };

  const handleTriggerRun = async () => {
    setTriggering(true);
    try {
      await agentApi.triggerRun(id);
      mutateRuns();
    } finally {
      setTriggering(false);
    }
  };

  if (isLoading) return <div className="py-12 text-center text-muted-foreground">Loading...</div>;

  const project = data?.data;
  if (!project) return <div className="py-12 text-center">Project not found</div>;

  return (
    <div className="space-y-6">
      <div className="flex items-center gap-4">
        <Link href="/projects">
          <Button variant="ghost" size="sm">
            <ArrowLeft className="mr-2 h-4 w-4" />
            Back
          </Button>
        </Link>
      </div>

      <div className="flex items-start justify-between">
        <div>
          <h2 className="text-2xl font-bold">{project.name}</h2>
          {project.description && <p className="mt-1 text-muted-foreground">{project.description}</p>}
        </div>
        <div className="flex gap-2">
          <Link href={`/projects/${id}/edit`}>
            <Button variant="outline" size="sm">
              <Pencil className="mr-2 h-4 w-4" /> Edit
            </Button>
          </Link>
          <Button variant="destructive" size="sm" onClick={handleDelete}>
            <Trash2 className="mr-2 h-4 w-4" /> Delete
          </Button>
        </div>
      </div>

      {/* Tab bar */}
      <div className="flex gap-1 border-b">
        {tabs.map((tab) => (
          <button
            key={tab.key}
            onClick={() => setActiveTab(tab.key)}
            className={`px-4 py-2 text-sm font-medium border-b-2 transition-colors ${
              activeTab === tab.key
                ? "border-primary text-primary"
                : "border-transparent text-muted-foreground hover:text-foreground"
            }`}
          >
            {tab.label}
          </button>
        ))}
      </div>

      {/* Tab content */}
      {activeTab === "overview" && (
        <div className="grid gap-6 lg:grid-cols-2">
          <Card>
            <CardHeader>
              <CardTitle className="text-base">Agent Configuration</CardTitle>
            </CardHeader>
            <CardContent className="space-y-3">
              {project.agent_prompt ? (
                <div>
                  <div className="text-xs text-muted-foreground mb-1">Custom Prompt</div>
                  <pre className="rounded bg-muted p-3 text-sm whitespace-pre-wrap">{project.agent_prompt}</pre>
                </div>
              ) : (
                <p className="text-sm text-muted-foreground">No custom agent prompt configured.</p>
              )}
            </CardContent>
          </Card>

          <Card>
            <CardHeader>
              <CardTitle className="text-base">Agent Rules</CardTitle>
            </CardHeader>
            <CardContent>
              {project.agent_rules ? (
                <div className="space-y-2 text-sm">
                  <div className="flex justify-between">
                    <span className="text-muted-foreground">Major releases</span>
                    <Badge variant={project.agent_rules.on_major_release ? "default" : "secondary"}>
                      {project.agent_rules.on_major_release ? "enabled" : "disabled"}
                    </Badge>
                  </div>
                  <div className="flex justify-between">
                    <span className="text-muted-foreground">Minor releases</span>
                    <Badge variant={project.agent_rules.on_minor_release ? "default" : "secondary"}>
                      {project.agent_rules.on_minor_release ? "enabled" : "disabled"}
                    </Badge>
                  </div>
                  <div className="flex justify-between">
                    <span className="text-muted-foreground">Security patches</span>
                    <Badge variant={project.agent_rules.on_security_patch ? "default" : "secondary"}>
                      {project.agent_rules.on_security_patch ? "enabled" : "disabled"}
                    </Badge>
                  </div>
                  {project.agent_rules.version_pattern && (
                    <div className="flex justify-between">
                      <span className="text-muted-foreground">Version pattern</span>
                      <span className="font-mono text-xs">{project.agent_rules.version_pattern}</span>
                    </div>
                  )}
                </div>
              ) : (
                <p className="text-sm text-muted-foreground">No agent rules configured.</p>
              )}
            </CardContent>
          </Card>
        </div>
      )}

      {activeTab === "sources" && (
        <Card>
          <CardHeader className="flex flex-row items-center justify-between">
            <CardTitle className="flex items-center gap-2 text-base">
              <Radio className="h-4 w-4" /> Sources
            </CardTitle>
            <Link href={`/projects/${id}/sources/new`}>
              <Button variant="outline" size="sm">Add Source</Button>
            </Link>
          </CardHeader>
          <CardContent>
            {sourcesData?.data && sourcesData.data.length > 0 ? (
              <div className="space-y-2">
                {sourcesData.data.map((source) => (
                  <Link
                    key={source.id}
                    href={`/sources/${source.id}/edit`}
                    className="flex items-center justify-between rounded-md border p-3 hover:bg-muted/50"
                  >
                    <div>
                      <div className="flex items-center gap-2">
                        <Badge variant="outline">{source.provider}</Badge>
                        <span className="font-mono text-sm">{source.repository}</span>
                      </div>
                      <div className="mt-1 text-xs text-muted-foreground">
                        Poll every {source.poll_interval_seconds}s
                        {source.last_polled_at && ` · Last: ${new Date(source.last_polled_at).toLocaleString()}`}
                      </div>
                    </div>
                    <Badge variant={source.enabled ? "default" : "secondary"}>
                      {source.enabled ? "active" : "disabled"}
                    </Badge>
                  </Link>
                ))}
              </div>
            ) : (
              <p className="py-4 text-center text-sm text-muted-foreground">No sources configured</p>
            )}
          </CardContent>
        </Card>
      )}

      {activeTab === "context" && (
        <Card>
          <CardHeader className="flex flex-row items-center justify-between">
            <CardTitle className="flex items-center gap-2 text-base">
              <BookOpen className="h-4 w-4" /> Context Sources
            </CardTitle>
            <Link href={`/projects/${id}/context-sources/new`}>
              <Button variant="outline" size="sm">Add Context Source</Button>
            </Link>
          </CardHeader>
          <CardContent>
            {ctxData?.data && ctxData.data.length > 0 ? (
              <div className="space-y-2">
                {ctxData.data.map((ctx) => (
                  <div key={ctx.id} className="flex items-center justify-between rounded-md border p-3">
                    <div>
                      <div className="flex items-center gap-2">
                        <Badge variant="outline">{ctx.type}</Badge>
                        <span className="font-medium text-sm">{ctx.name}</span>
                      </div>
                      <div className="mt-1 text-xs text-muted-foreground">
                        Created {new Date(ctx.created_at).toLocaleDateString()}
                      </div>
                    </div>
                  </div>
                ))}
              </div>
            ) : (
              <p className="py-4 text-center text-sm text-muted-foreground">No context sources configured</p>
            )}
          </CardContent>
        </Card>
      )}

      {activeTab === "semantic" && (
        <Card>
          <CardHeader>
            <CardTitle className="flex items-center gap-2 text-base">
              <FileText className="h-4 w-4" /> Semantic Releases
            </CardTitle>
          </CardHeader>
          <CardContent>
            {srData?.data && srData.data.length > 0 ? (
              <div className="space-y-3">
                {srData.data.map((sr) => (
                  <Link
                    key={sr.id}
                    href={`/projects/${id}/semantic-releases/${sr.id}`}
                    className="block rounded-md border p-4 hover:bg-muted/50"
                  >
                    <div className="flex items-center justify-between">
                      <div className="flex items-center gap-2">
                        <span className="font-mono font-medium">{sr.version}</span>
                        <Badge
                          variant={sr.status === "completed" ? "default" : sr.status === "failed" ? "destructive" : "secondary"}
                        >
                          {sr.status}
                        </Badge>
                      </div>
                      <span className="text-xs text-muted-foreground">
                        {new Date(sr.created_at).toLocaleDateString()}
                      </span>
                    </div>
                    {sr.report && (
                      <p className="mt-2 text-sm text-muted-foreground line-clamp-2">{sr.report.summary}</p>
                    )}
                  </Link>
                ))}
              </div>
            ) : (
              <p className="py-4 text-center text-sm text-muted-foreground">No semantic releases yet</p>
            )}
          </CardContent>
        </Card>
      )}

      {activeTab === "agent" && (
        <div className="space-y-6">
          <Card>
            <CardHeader className="flex flex-row items-center justify-between">
              <CardTitle className="flex items-center gap-2 text-base">
                <Bot className="h-4 w-4" /> Agent
              </CardTitle>
              <Button size="sm" onClick={handleTriggerRun} disabled={triggering}>
                <Play className="mr-2 h-4 w-4" />
                {triggering ? "Triggering..." : "Trigger Run"}
              </Button>
            </CardHeader>
            <CardContent>
              <div className="space-y-3">
                <h4 className="text-sm font-medium">Run History</h4>
                {runsData?.data && runsData.data.length > 0 ? (
                  <div className="space-y-2">
                    {runsData.data.map((run) => (
                      <div key={run.id} className="flex items-center justify-between rounded-md border p-3">
                        <div>
                          <div className="flex items-center gap-2">
                            <Badge
                              variant={run.status === "completed" ? "default" : run.status === "failed" ? "destructive" : "secondary"}
                            >
                              {run.status}
                            </Badge>
                            <span className="text-sm text-muted-foreground">{run.trigger}</span>
                          </div>
                          {run.error && (
                            <p className="mt-1 text-xs text-red-600">{run.error}</p>
                          )}
                        </div>
                        <div className="text-xs text-muted-foreground">
                          {run.started_at ? new Date(run.started_at).toLocaleString() : "Pending"}
                        </div>
                      </div>
                    ))}
                  </div>
                ) : (
                  <p className="py-4 text-center text-sm text-muted-foreground">No agent runs yet</p>
                )}
              </div>
            </CardContent>
          </Card>
        </div>
      )}
    </div>
  );
}
