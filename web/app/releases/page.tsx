"use client";

import { useState } from "react";
import useSWR from "swr";
import Link from "next/link";
import { releases as releasesApi, projects as projectsApi } from "@/lib/api/client";
import { Card, CardContent } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import {
  Select, SelectContent, SelectItem, SelectTrigger, SelectValue,
} from "@/components/ui/select";
import {
  Table, TableBody, TableCell, TableHead, TableHeader, TableRow,
} from "@/components/ui/table";
import { ChevronLeft, ChevronRight } from "lucide-react";

export default function ReleasesPage() {
  const [page, setPage] = useState(1);
  const [projectFilter, setProjectFilter] = useState<string>("all");

  const { data: projectsData } = useSWR("projects-for-filter", () => projectsApi.list());

  const { data, isLoading } = useSWR(
    ["releases", page, projectFilter],
    () => {
      if (projectFilter !== "all") {
        return releasesApi.listByProject(projectFilter, page);
      }
      // When no project filter, show all releases across all projects
      // The API doesn't have a global releases endpoint, so we use the first project or show empty
      return null;
    }
  );

  // For the "all projects" case, we need to fetch all project releases
  const { data: allReleasesData } = useSWR(
    projectFilter === "all" ? ["all-releases", page] : null,
    async () => {
      if (!projectsData?.data?.length) return null;
      // Fetch releases from the first project as a default view
      // In production, there would be a global /releases endpoint
      const results = await Promise.all(
        projectsData.data.slice(0, 5).map((p) => releasesApi.listByProject(p.id, page).catch(() => null))
      );
      const allReleases = results
        .filter((r): r is NonNullable<typeof r> => r !== null)
        .flatMap((r) => r.data);
      return allReleases;
    }
  );

  const displayReleases = projectFilter !== "all" ? data?.data : allReleasesData;
  const total = data?.meta?.total ?? 0;
  const totalPages = Math.max(1, Math.ceil(total / 15));

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
              <SelectItem key={p.id} value={p.id}>{p.name}</SelectItem>
            ))}
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
                  <TableHead>Source</TableHead>
                  <TableHead>Released</TableHead>
                  <TableHead>Ingested</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {(displayReleases ?? []).map((release) => (
                  <TableRow key={release.id}>
                    <TableCell>
                      <Link
                        href={`/releases/${release.id}`}
                        className="font-mono text-sm text-primary hover:underline"
                      >
                        {release.version}
                      </Link>
                    </TableCell>
                    <TableCell className="text-xs text-muted-foreground font-mono">
                      {release.source_id}
                    </TableCell>
                    <TableCell className="text-sm text-muted-foreground">
                      {release.released_at ? new Date(release.released_at).toLocaleDateString() : "\u2014"}
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
      {projectFilter !== "all" && totalPages > 1 && (
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
