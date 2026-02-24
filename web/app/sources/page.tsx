"use client";

import useSWR from "swr";
import Link from "next/link";
import { sources as sourcesApi } from "@/lib/api/client";
import { Button } from "@/components/ui/button";
import { Card, CardContent } from "@/components/ui/card";
import { Badge } from "@/components/ui/badge";
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table";
import { Plus } from "lucide-react";

export default function SourcesPage() {
  const { data, isLoading } = useSWR("sources", () => sourcesApi.list());
  return (
    <div className="space-y-4">
      <div className="flex items-center justify-between">
        <div>
          <h2 className="text-lg font-semibold">All Sources</h2>
          <p className="text-sm text-muted-foreground">Ingestion sources that poll upstream registries.</p>
        </div>
        <Link href="/sources/new"><Button><Plus className="mr-2 h-4 w-4" />Add Source</Button></Link>
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
                  <TableHead>Type</TableHead>
                  <TableHead>Poll Interval</TableHead>
                  <TableHead>Last Polled</TableHead>
                  <TableHead>Status</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {data?.data.map((source) => (
                  <TableRow key={source.id}>
                    <TableCell>
                      <Link href={`/sources/${source.id}/edit`} className="font-mono text-sm text-primary hover:underline">{source.repository}</Link>
                    </TableCell>
                    <TableCell><Badge variant="outline">{source.type}</Badge></TableCell>
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
