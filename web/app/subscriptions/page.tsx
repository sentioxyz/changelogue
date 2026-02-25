"use client";

import useSWR from "swr";
import Link from "next/link";
import { subscriptions as subsApi, channels as channelsApi } from "@/lib/api/client";
import { Button } from "@/components/ui/button";
import { Card, CardContent } from "@/components/ui/card";
import { Badge } from "@/components/ui/badge";
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table";
import { Plus } from "lucide-react";

export default function SubscriptionsPage() {
  const { data, isLoading } = useSWR("subscriptions", () => subsApi.list());
  const { data: channelsData } = useSWR("channels-for-sub-list", () => channelsApi.list());

  const getChannelName = (id: string) => channelsData?.data.find((c) => c.id === id)?.name ?? id;

  return (
    <div className="space-y-4">
      <div className="flex items-center justify-between">
        <div>
          <h2 className="text-lg font-semibold">Subscriptions</h2>
          <p className="text-sm text-muted-foreground">Notification routing rules linking sources or projects to channels.</p>
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
                  <TableHead>Type</TableHead>
                  <TableHead>Target</TableHead>
                  <TableHead>Channel</TableHead>
                  <TableHead>Version Filter</TableHead>
                  <TableHead>Actions</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {data?.data.map((sub) => (
                  <TableRow key={sub.id}>
                    <TableCell>
                      <Badge variant="outline">{sub.type}</Badge>
                    </TableCell>
                    <TableCell className="font-mono text-xs text-muted-foreground">
                      {sub.type === "source" ? sub.source_id : sub.project_id}
                    </TableCell>
                    <TableCell>{getChannelName(sub.channel_id)}</TableCell>
                    <TableCell className="font-mono text-xs text-muted-foreground">{sub.version_filter || "\u2014"}</TableCell>
                    <TableCell>
                      <Link href={`/subscriptions/${sub.id}/edit`}>
                        <Badge variant="secondary" className="cursor-pointer">edit</Badge>
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
