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
import { useTranslation } from "@/lib/i18n/context";

interface SourceWithProject extends Source {
  projectName?: string;
}

export default function SourcesPage() {
  const { t } = useTranslation();
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
          className="text-foreground"
          style={{
            fontFamily: "var(--font-raleway)",
            fontSize: "24px",
            fontWeight: 700,
          }}
        >
          {t("sources.title")}
        </h1>
        <p
          className="mt-1 text-[13px] text-text-secondary"
          style={{ fontFamily: "var(--font-dm-sans)" }}
        >
          {t("sources.description")}
        </p>
      </div>
      <Card>
        <CardContent className="p-0">
          {isLoading ? (
            <div className="py-12 text-center text-muted-foreground">{t("sources.loading")}</div>
          ) : (
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead>{t("sources.thRepository")}</TableHead>
                  <TableHead>{t("sources.thProvider")}</TableHead>
                  <TableHead>{t("sources.thProject")}</TableHead>
                  <TableHead>{t("sources.thPollInterval")}</TableHead>
                  <TableHead>{t("sources.thLastPolled")}</TableHead>
                  <TableHead>{t("sources.thStatus")}</TableHead>
                  <TableHead>{t("sources.thActions")}</TableHead>
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
                    <TableCell className="text-sm text-muted-foreground">{source.last_polled_at ? new Date(source.last_polled_at).toLocaleString() : t("sources.never")}</TableCell>
                    <TableCell>
                      {source.last_error ? (
                        <Badge variant="destructive" className="text-xs max-w-[250px] truncate" title={source.last_error}>{source.last_error}</Badge>
                      ) : (
                        <Badge variant={source.enabled ? "default" : "secondary"}>{source.enabled ? t("sources.statusActive") : t("sources.statusDisabled")}</Badge>
                      )}
                    </TableCell>
                    <TableCell>
                      <div className="flex items-center gap-2">
                        <button
                          onClick={() => setEditingSource(source)}
                          className="rounded p-1 text-text-muted transition-colors hover:bg-mono-bg hover:text-foreground"
                        >
                          <Pencil className="h-4 w-4" />
                        </button>
                        <button
                          onClick={() => setDeletingId(source.id)}
                          className="rounded p-1 text-text-muted transition-colors hover:bg-red-50 hover:text-red-600"
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
          <DialogHeader><DialogTitle>{t("sources.editSource")}</DialogTitle></DialogHeader>
          {editingSource && (
            <SourceForm
              key={editingSource.id}
              title={t("sources.editSource")}
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
        title={t("sources.deleteSource")}
        description={t("sources.deleteSourceDesc")}
        onConfirm={async () => { if (deletingId) { await sourcesApi.delete(deletingId); mutate("all-sources"); } }}
      />
    </div>
  );
}
