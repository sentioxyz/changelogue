"use client";

import useSWR from "swr";
import Link from "next/link";
import { useRouter } from "next/navigation";
import { projects as projectsApi } from "@/lib/api/client";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Badge } from "@/components/ui/badge";
import { Pencil, Trash2, ExternalLink, ArrowLeft } from "lucide-react";

export function ProjectDetail({ id }: { id: string }) {
  const router = useRouter();
  const { data, isLoading } = useSWR(`project-${id}`, () => projectsApi.get(Number(id)));

  const handleDelete = async () => {
    if (!confirm("Delete this project? This will cascade to sources and subscriptions.")) return;
    await projectsApi.delete(Number(id));
    router.push("/projects");
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
          <p className="mt-1 text-muted-foreground">{project.description}</p>
          {project.url && (
            <a
              href={project.url}
              target="_blank"
              rel="noopener noreferrer"
              className="mt-1 inline-flex items-center gap-1 text-sm text-primary hover:underline"
            >
              {project.url} <ExternalLink className="h-3 w-3" />
            </a>
          )}
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

      <div className="grid gap-6 lg:grid-cols-2">
        {/* Pipeline Config */}
        <Card>
          <CardHeader>
            <CardTitle className="text-base">Pipeline Configuration</CardTitle>
          </CardHeader>
          <CardContent>
            <div className="space-y-3">
              <div className="text-xs text-muted-foreground">
                Always-on: regex_normalizer, subscription_router
              </div>
              {Object.entries(project.pipeline_config).map(([node, config]) => (
                <div key={node} className="rounded-md border p-3">
                  <div className="flex items-center justify-between">
                    <span className="font-medium text-sm">{node.replace(/_/g, " ")}</span>
                    <Badge variant="secondary" className="text-xs">enabled</Badge>
                  </div>
                  {config != null && typeof config === "object" && Object.keys(config).length > 0 && (
                    <pre className="mt-2 rounded bg-muted p-2 text-xs overflow-x-auto">
                      {JSON.stringify(config, null, 2)}
                    </pre>
                  )}
                </div>
              ))}
            </div>
          </CardContent>
        </Card>

        {/* Sources */}
        <Card>
          <CardHeader className="flex flex-row items-center justify-between">
            <CardTitle className="text-base">Sources</CardTitle>
            <Link href={`/sources/new?project_id=${id}`}>
              <Button variant="outline" size="sm">Add Source</Button>
            </Link>
          </CardHeader>
          <CardContent>
            {project.sources && project.sources.length > 0 ? (
              <div className="space-y-2">
                {project.sources.map((source) => (
                  <Link
                    key={source.id}
                    href={`/sources/${source.id}/edit`}
                    className="flex items-center justify-between rounded-md border p-3 hover:bg-muted/50"
                  >
                    <div>
                      <div className="flex items-center gap-2">
                        <Badge variant="outline">{source.type}</Badge>
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
      </div>
    </div>
  );
}
