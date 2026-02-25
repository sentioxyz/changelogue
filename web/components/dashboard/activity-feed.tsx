// web/components/dashboard/activity-feed.tsx
"use client";

import { useEffect, useState, useRef } from "react";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Badge } from "@/components/ui/badge";
import type { SSEEvent } from "@/lib/api/types";

const BASE = process.env.NEXT_PUBLIC_API_URL || "/api/v1";

const eventColors: Record<string, string> = {
  release: "bg-green-100 text-green-800",
  semantic_release: "bg-blue-100 text-blue-800",
};

function formatEvent(event: SSEEvent): string {
  switch (event.type) {
    case "release":
      return `New release: ${event.data.version}`;
    case "semantic_release":
      return `Semantic release: ${event.data.version} (${event.data.status})`;
    default:
      return "Unknown event";
  }
}

export function ActivityFeed() {
  const [events, setEvents] = useState<SSEEvent[]>([]);
  const [connected, setConnected] = useState(false);
  const retryRef = useRef<ReturnType<typeof setTimeout> | null>(null);

  useEffect(() => {
    let es: EventSource | null = null;

    function connect() {
      try {
        es = new EventSource(`${BASE}/events`);

        es.onopen = () => setConnected(true);

        es.onmessage = (msg) => {
          try {
            const event = JSON.parse(msg.data) as SSEEvent;
            setEvents((prev) => [event, ...prev].slice(0, 20));
          } catch {
            // ignore parse errors
          }
        };

        es.onerror = () => {
          setConnected(false);
          es?.close();
          // Retry connection after 5 seconds
          retryRef.current = setTimeout(connect, 5000);
        };
      } catch {
        setConnected(false);
        retryRef.current = setTimeout(connect, 5000);
      }
    }

    connect();

    return () => {
      es?.close();
      if (retryRef.current) clearTimeout(retryRef.current);
    };
  }, []);

  return (
    <Card>
      <CardHeader>
        <CardTitle className="flex items-center gap-2">
          Live Activity
          <span className="relative flex h-2 w-2">
            {connected ? (
              <>
                <span className="absolute inline-flex h-full w-full animate-ping rounded-full bg-green-400 opacity-75" />
                <span className="relative inline-flex h-2 w-2 rounded-full bg-green-500" />
              </>
            ) : (
              <span className="relative inline-flex h-2 w-2 rounded-full bg-gray-400" />
            )}
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
            {events.map((event) => (
              <div key={event.id} className="flex items-start gap-2 text-sm">
                <Badge className={`shrink-0 text-xs ${eventColors[event.type] ?? ""}`}>
                  {event.type}
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
