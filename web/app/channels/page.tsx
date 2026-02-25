"use client";

import useSWR from "swr";
import Link from "next/link";
import { channels as channelsApi } from "@/lib/api/client";
import { Button } from "@/components/ui/button";
import { Card, CardContent } from "@/components/ui/card";
import { Badge } from "@/components/ui/badge";
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table";
import { Plus } from "lucide-react";

export default function ChannelsPage() {
  const { data, isLoading } = useSWR("channels", () => channelsApi.list());
  return (
    <div className="space-y-4">
      <div className="flex items-center justify-between">
        <div>
          <h2 className="text-lg font-semibold">Notification Channels</h2>
          <p className="text-sm text-muted-foreground">Output targets for release notifications.</p>
        </div>
        <Link href="/channels/new"><Button><Plus className="mr-2 h-4 w-4" />Add Channel</Button></Link>
      </div>
      <Card>
        <CardContent className="p-0">
          {isLoading ? (
            <div className="py-12 text-center text-muted-foreground">Loading...</div>
          ) : (
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead>Name</TableHead>
                  <TableHead>Type</TableHead>
                  <TableHead>Config</TableHead>
                  <TableHead>Created</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {data?.data.map((ch) => (
                  <TableRow key={ch.id}>
                    <TableCell><Link href={`/channels/${ch.id}/edit`} className="font-medium text-primary hover:underline">{ch.name}</Link></TableCell>
                    <TableCell><Badge variant="outline">{ch.type}</Badge></TableCell>
                    <TableCell className="max-w-xs truncate text-xs text-muted-foreground font-mono">
                      {Object.entries(ch.config).map(([k, v]) => `${k}: ${String(v)}`).join(", ")}
                    </TableCell>
                    <TableCell className="text-sm text-muted-foreground">
                      {new Date(ch.created_at).toLocaleDateString()}
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
