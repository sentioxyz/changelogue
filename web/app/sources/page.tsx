"use client";

import { useState } from "react";
import useSWR, { mutate } from "swr";
import Link from "next/link";
import { Pencil, Trash2 } from "lucide-react";
import { projects as projectsApi, sources as sourcesApi } from "@/lib/api/client";
import { formatInterval } from "@/lib/format";
import { Card, CardContent } from "@/components/ui/card";
import { Badge } from "@/components/ui/badge";
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table";
import { Dialog, DialogContent, DialogHeader, DialogTitle } from "@/components/ui/dialog";
import { ConfirmDialog } from "@/components/ui/confirm-dialog";
import { SourceForm } from "@/components/sources/source-form";
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

  const [editingSource, setEditingSource] = useState<SourceWithProject | null>(null);
  const [deletingId, setDeletingId] = useState<string | null>(null);

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
                  <TableHead>Actions</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {(allSources ?? []).map((source) => (
                  <TableRow key={source.id}>
                    <TableCell>
                      <span className="font-mono text-sm">{source.repository}</span>
                    </TableCell>
                    <TableCell><Badge variant="outline">{source.provider}</Badge></TableCell>
                    <TableCell>
                      <Link href={`/projects/${source.project_id}`} className="text-primary hover:underline">
                        {source.projectName ?? source.project_id}
                      </Link>
                    </TableCell>
                    <TableCell className="text-sm">{formatInterval(source.poll_interval_seconds)}</TableCell>
                    <TableCell className="text-sm text-muted-foreground">{source.last_polled_at ? new Date(source.last_polled_at).toLocaleString() : "Never"}</TableCell>
                    <TableCell>
                      {source.last_error ? (
                        <Badge variant="destructive" className="text-xs max-w-[250px] truncate" title={source.last_error}>{source.last_error}</Badge>
                      ) : (
                        <Badge variant={source.enabled ? "default" : "secondary"}>{source.enabled ? "active" : "disabled"}</Badge>
                      )}
                    </TableCell>
                    <TableCell>
                      <div className="flex items-center gap-2">
                        <button
                          onClick={() => setEditingSource(source)}
                          className="rounded p-1 text-[#9ca3af] transition-colors hover:bg-[#f3f3f1] hover:text-[#111113]"
                        >
                          <Pencil className="h-4 w-4" />
                        </button>
                        <button
                          onClick={() => setDeletingId(source.id)}
                          className="rounded p-1 text-[#9ca3af] transition-colors hover:bg-red-50 hover:text-red-600"
                        >
                          <Trash2 className="h-4 w-4" />
                        </button>
                      </div>
                    </TableCell>
                  </TableRow>
                ))}
              </TableBody>
            </Table>
          )}
        </CardContent>
      </Card>

      {/* Edit dialog */}
      <Dialog open={!!editingSource} onOpenChange={(open) => { if (!open) setEditingSource(null); }}>
        <DialogContent className="sm:max-w-lg">
          <DialogHeader><DialogTitle>Edit Source</DialogTitle></DialogHeader>
          {editingSource && (
            <SourceForm
              key={editingSource.id}
              title="Edit Source"
              initial={editingSource}
              onSubmit={async (input) => { await sourcesApi.update(editingSource.id, input); }}
              onSuccess={() => { setEditingSource(null); mutate("all-sources"); }}
              onCancel={() => setEditingSource(null)}
            />
          )}
        </DialogContent>
      </Dialog>

      {/* Delete dialog */}
      <ConfirmDialog
        open={!!deletingId}
        onOpenChange={(open) => { if (!open) setDeletingId(null); }}
        title="Delete Source"
        description="This will permanently delete this source. This cannot be undone."
        onConfirm={async () => { if (deletingId) { await sourcesApi.delete(deletingId); mutate("all-sources"); } }}
      />
    </div>
  );
}
