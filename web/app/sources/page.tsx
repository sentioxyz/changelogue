"use client";

import useSWR from "swr";
import Link from "next/link";
import { projects as projectsApi, sources as sourcesApi } from "@/lib/api/client";
import { Card, CardContent } from "@/components/ui/card";
import { Badge } from "@/components/ui/badge";
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table";
import type { Source } from "@/lib/api/types";

interface SourceWithProject extends Source {
  projectName?: string;
}

export default function SourcesPage() {
  const { data: projectsData } = useSWR("projects-for-sources", () => projectsApi.list());

  const { data: allSources, isLoading } = useSWR(
    projectsData ? "all-sources" : null,
    async () => {
      if (!projectsData?.data?.length) return [];
      const results = await Promise.all(
        projectsData.data.map(async (p) => {
          const res = await sourcesApi.listByProject(p.id).catch(() => null);
          return (res?.data ?? []).map((s) => ({ ...s, projectName: p.name }));
        })
      );
      return results.flat() as SourceWithProject[];
    }
  );

  return (
    <div className="space-y-4">
      <div>
        <h1
          style={{
            fontFamily: "var(--font-fraunces)",
            fontSize: "24px",
            fontWeight: 700,
            color: "#111113",
          }}
        >
          Sources
        </h1>
        <p
          className="mt-1 text-[13px] text-[#6b7280]"
          style={{ fontFamily: "var(--font-dm-sans)" }}
        >
          Ingestion sources across all projects. Sources are managed within their project.
        </p>
      </div>
      <Card>
        <CardContent className="p-0">
          {isLoading ? (
            <div className="py-12 text-center text-muted-foreground">Loading...</div>
          ) : (
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead>Repository</TableHead>
                  <TableHead>Provider</TableHead>
                  <TableHead>Project</TableHead>
                  <TableHead>Poll Interval</TableHead>
                  <TableHead>Last Polled</TableHead>
                  <TableHead>Status</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {(allSources ?? []).map((source) => (
                  <TableRow key={source.id}>
                    <TableCell>
                      <Link href={`/sources/${source.id}/edit`} className="font-mono text-sm text-primary hover:underline">{source.repository}</Link>
                    </TableCell>
                    <TableCell><Badge variant="outline">{source.provider}</Badge></TableCell>
                    <TableCell>
                      <Link href={`/projects/${source.project_id}`} className="text-primary hover:underline">
                        {source.projectName ?? source.project_id}
                      </Link>
                    </TableCell>
                    <TableCell className="text-sm">{source.poll_interval_seconds}s</TableCell>
                    <TableCell className="text-sm text-muted-foreground">{source.last_polled_at ? new Date(source.last_polled_at).toLocaleString() : "Never"}</TableCell>
                    <TableCell>
                      {source.last_error ? (
                        <Badge variant="destructive" className="text-xs">{source.last_error}</Badge>
                      ) : (
                        <Badge variant={source.enabled ? "default" : "secondary"}>{source.enabled ? "active" : "disabled"}</Badge>
                      )}
                    </TableCell>
                  </TableRow>
                ))}
              </TableBody>
            </Table>
          )}
        </CardContent>
      </Card>
    </div>
  );
}
