"use client";

import { useState } from "react";
import useSWR from "swr";
import Link from "next/link";
import { releases as releasesApi, type ListReleasesParams } from "@/lib/api/client";
import { projects as projectsApi } from "@/lib/api/client";
import { Card, CardContent } from "@/components/ui/card";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import {
  Select, SelectContent, SelectItem, SelectTrigger, SelectValue,
} from "@/components/ui/select";
import {
  Table, TableBody, TableCell, TableHead, TableHeader, TableRow,
} from "@/components/ui/table";
import { ChevronLeft, ChevronRight } from "lucide-react";

const statusColors: Record<string, string> = {
  completed: "bg-green-100 text-green-800",
  running: "bg-blue-100 text-blue-800",
  available: "bg-gray-100 text-gray-800",
  retry: "bg-yellow-100 text-yellow-800",
  discarded: "bg-red-100 text-red-800",
};

export default function ReleasesPage() {
  const [page, setPage] = useState(1);
  const [projectFilter, setProjectFilter] = useState<string>("all");
  const [preReleaseFilter, setPreReleaseFilter] = useState<string>("all");

  const { data: projectsData } = useSWR("projects-for-filter", () => projectsApi.list());

  const params: ListReleasesParams = { page, per_page: 15 };
  if (projectFilter !== "all") params.project_id = Number(projectFilter);
  if (preReleaseFilter !== "all") params.pre_release = preReleaseFilter === "true";

  const { data, isLoading } = useSWR(
    ["releases", page, projectFilter, preReleaseFilter],
    () => releasesApi.list(params)
  );

  const total = data?.meta.total ?? 0;
  const totalPages = Math.ceil(total / 15);

  return (
    <div className="space-y-4">
      <div>
        <h2 className="text-lg font-semibold">All Releases</h2>
        <p className="text-sm text-muted-foreground">Browse releases ingested from all sources.</p>
      </div>

      {/* Filters */}
      <div className="flex gap-3">
        <Select value={projectFilter} onValueChange={(v) => { setProjectFilter(v); setPage(1); }}>
          <SelectTrigger className="w-48">
            <SelectValue placeholder="All Projects" />
          </SelectTrigger>
          <SelectContent>
            <SelectItem value="all">All Projects</SelectItem>
            {projectsData?.data.map((p) => (
              <SelectItem key={p.id} value={String(p.id)}>{p.name}</SelectItem>
            ))}
          </SelectContent>
        </Select>

        <Select value={preReleaseFilter} onValueChange={(v) => { setPreReleaseFilter(v); setPage(1); }}>
          <SelectTrigger className="w-40">
            <SelectValue placeholder="All Types" />
          </SelectTrigger>
          <SelectContent>
            <SelectItem value="all">All Types</SelectItem>
            <SelectItem value="false">Stable Only</SelectItem>
            <SelectItem value="true">Pre-release Only</SelectItem>
          </SelectContent>
        </Select>
      </div>

      <Card>
        <CardContent className="p-0">
          {isLoading ? (
            <div className="py-12 text-center text-muted-foreground">Loading...</div>
          ) : (
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead>Version</TableHead>
                  <TableHead>Project</TableHead>
                  <TableHead>Source</TableHead>
                  <TableHead>Pipeline</TableHead>
                  <TableHead>Date</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {data?.data.map((release) => (
                  <TableRow key={release.id}>
                    <TableCell>
                      <Link
                        href={`/releases/${release.id}`}
                        className="flex items-center gap-2 font-mono text-sm text-primary hover:underline"
                      >
                        {release.raw_version}
                        {release.is_pre_release && (
                          <Badge variant="outline" className="text-xs">pre</Badge>
                        )}
                      </Link>
                    </TableCell>
                    <TableCell>
                      <Link href={`/projects/${release.project_id}`} className="hover:underline">
                        {release.project_name}
                      </Link>
                    </TableCell>
                    <TableCell>
                      <div className="flex items-center gap-1.5">
                        <Badge variant="outline" className="text-xs">{release.source_type}</Badge>
                        <span className="text-xs text-muted-foreground">{release.repository}</span>
                      </div>
                    </TableCell>
                    <TableCell>
                      <Badge className={statusColors[release.pipeline_status] ?? ""}>
                        {release.pipeline_status}
                      </Badge>
                    </TableCell>
                    <TableCell className="text-sm text-muted-foreground">
                      {new Date(release.created_at).toLocaleDateString()}
                    </TableCell>
                  </TableRow>
                ))}
              </TableBody>
            </Table>
          )}
        </CardContent>
      </Card>

      {/* Pagination */}
      {totalPages > 1 && (
        <div className="flex items-center justify-between">
          <span className="text-sm text-muted-foreground">{total} releases</span>
          <div className="flex items-center gap-2">
            <Button variant="outline" size="sm" disabled={page <= 1} onClick={() => setPage(page - 1)}>
              <ChevronLeft className="h-4 w-4" />
            </Button>
            <span className="text-sm">Page {page} of {totalPages}</span>
            <Button variant="outline" size="sm" disabled={page >= totalPages} onClick={() => setPage(page + 1)}>
              <ChevronRight className="h-4 w-4" />
            </Button>
          </div>
        </div>
      )}
    </div>
  );
}
