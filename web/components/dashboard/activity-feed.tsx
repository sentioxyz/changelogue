// web/components/dashboard/activity-feed.tsx
"use client";

import { useEffect, useState } from "react";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Badge } from "@/components/ui/badge";
import type { SSEEvent } from "@/lib/api/types";
import { mockSSE } from "@/lib/mock/sse";

const eventColors: Record<string, string> = {
  "release.created": "bg-green-100 text-green-800",
  "pipeline.node_completed": "bg-blue-100 text-blue-800",
  "pipeline.completed": "bg-green-100 text-green-800",
  "pipeline.failed": "bg-red-100 text-red-800",
  "source.error": "bg-red-100 text-red-800",
  "source.polled": "bg-gray-100 text-gray-800",
};

function formatEvent(event: SSEEvent): string {
  const d = event.data as Record<string, unknown>;
  switch (event.type) {
    case "release.created":
      return `New release: ${d.repository} ${d.raw_version}`;
    case "pipeline.node_completed":
      return `Pipeline node "${d.node}" completed for ${d.release_id}`;
    case "pipeline.completed":
      return `Pipeline completed for ${d.release_id}`;
    case "pipeline.failed":
      return `Pipeline failed for ${d.release_id}`;
    case "source.polled":
      return `Polled ${d.repository} — ${d.new_releases} new`;
    case "source.error":
      return `Source error: ${d.repository}`;
    default:
      return event.type;
  }
}

export function ActivityFeed() {
  const [events, setEvents] = useState<SSEEvent[]>([]);

  useEffect(() => {
    const unsubscribe = mockSSE.subscribe((event) => {
      setEvents((prev) => [event, ...prev].slice(0, 20));
    });
    return unsubscribe;
  }, []);

  return (
    <Card>
      <CardHeader>
        <CardTitle className="flex items-center gap-2">
          Live Activity
          <span className="relative flex h-2 w-2">
            <span className="absolute inline-flex h-full w-full animate-ping rounded-full bg-green-400 opacity-75" />
            <span className="relative inline-flex h-2 w-2 rounded-full bg-green-500" />
          </span>
        </CardTitle>
      </CardHeader>
      <CardContent>
        {events.length === 0 ? (
          <p className="py-4 text-center text-sm text-muted-foreground">
            Waiting for events...
          </p>
        ) : (
          <div className="space-y-2 max-h-80 overflow-y-auto">
            {events.map((event, i) => (
              <div key={`${event.timestamp}-${i}`} className="flex items-start gap-2 text-sm">
                <Badge className={`shrink-0 text-xs ${eventColors[event.type] ?? ""}`}>
                  {event.type.split(".")[1]}
                </Badge>
                <span className="flex-1 text-muted-foreground">{formatEvent(event)}</span>
                <span className="shrink-0 text-xs text-muted-foreground">
                  {new Date(event.timestamp).toLocaleTimeString()}
                </span>
              </div>
            ))}
          </div>
        )}
      </CardContent>
    </Card>
  );
}
