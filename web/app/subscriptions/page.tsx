"use client";

import useSWR from "swr";
import Link from "next/link";
import { subscriptions as subsApi, projects as projectsApi, channels as channelsApi } from "@/lib/api/client";
import { Button } from "@/components/ui/button";
import { Card, CardContent } from "@/components/ui/card";
import { Badge } from "@/components/ui/badge";
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table";
import { Plus } from "lucide-react";

export default function SubscriptionsPage() {
  const { data, isLoading } = useSWR("subscriptions", () => subsApi.list());
  const { data: projectsData } = useSWR("projects-for-sub-list", () => projectsApi.list());
  const { data: channelsData } = useSWR("channels-for-sub-list", () => channelsApi.list());

  const getProjectName = (id: number) => projectsData?.data.find((p) => p.id === id)?.name ?? `#${id}`;
  const getChannelName = (id: number) => channelsData?.data.find((c) => c.id === id)?.name ?? `#${id}`;

  return (
    <div className="space-y-4">
      <div className="flex items-center justify-between">
        <div>
          <h2 className="text-lg font-semibold">Subscriptions</h2>
          <p className="text-sm text-muted-foreground">Notification routing rules linking projects to channels.</p>
        </div>
        <Link href="/subscriptions/new"><Button><Plus className="mr-2 h-4 w-4" />New Subscription</Button></Link>
      </div>
      <Card>
        <CardContent className="p-0">
          {isLoading ? (
            <div className="py-12 text-center text-muted-foreground">Loading...</div>
          ) : (
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead>Project</TableHead>
                  <TableHead>Type</TableHead>
                  <TableHead>Channel</TableHead>
                  <TableHead>Frequency</TableHead>
                  <TableHead>Pattern</TableHead>
                  <TableHead>Status</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {data?.data.map((sub) => (
                  <TableRow key={sub.id}>
                    <TableCell className="font-medium">{getProjectName(sub.project_id)}</TableCell>
                    <TableCell><Badge variant="outline">{sub.channel_type}</Badge></TableCell>
                    <TableCell>{getChannelName(sub.channel_id)}</TableCell>
                    <TableCell className="text-sm">{sub.frequency}</TableCell>
                    <TableCell className="font-mono text-xs text-muted-foreground">{sub.version_pattern || "\u2014"}</TableCell>
                    <TableCell>
                      <Link href={`/subscriptions/${sub.id}/edit`}>
                        <Badge variant={sub.enabled ? "default" : "secondary"}>{sub.enabled ? "active" : "disabled"}</Badge>
                      </Link>
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
