// web/lib/mock/sse.ts
import type { SSEEvent, SSEEventType } from "../api/types";

type SSEListener = (event: SSEEvent) => void;

class MockSSE {
  private listeners: SSEListener[] = [];
  private interval: ReturnType<typeof setInterval> | null = null;

  subscribe(listener: SSEListener) {
    this.listeners.push(listener);
    if (!this.interval) this.startEmitting();
    return () => {
      this.listeners = this.listeners.filter((l) => l !== listener);
      if (this.listeners.length === 0 && this.interval) {
        clearInterval(this.interval);
        this.interval = null;
      }
    };
  }

  private startEmitting() {
    const events: Array<() => SSEEvent> = [
      () => ({
        type: "release.created" as SSEEventType,
        data: {
          id: `r-live-${Date.now()}`,
          source: "dockerhub",
          repository: "library/golang",
          raw_version: `1.23.${Math.floor(Math.random() * 10)}`,
          created_at: new Date().toISOString(),
        },
        timestamp: new Date().toISOString(),
      }),
      () => ({
        type: "pipeline.node_completed" as SSEEventType,
        data: {
          release_id: "r-004",
          node: ["regex_normalizer", "changelog_summarizer", "urgency_scorer"][Math.floor(Math.random() * 3)],
          result: { status: "ok" },
        },
        timestamp: new Date().toISOString(),
      }),
      () => ({
        type: "pipeline.completed" as SSEEventType,
        data: {
          release_id: "r-004",
          state: "completed",
        },
        timestamp: new Date().toISOString(),
      }),
      () => ({
        type: "source.polled" as SSEEventType,
        data: {
          source_id: Math.floor(Math.random() * 6) + 1,
          repository: ["library/golang", "ethereum/client-go", "library/postgres"][Math.floor(Math.random() * 3)],
          new_releases: Math.floor(Math.random() * 2),
        },
        timestamp: new Date().toISOString(),
      }),
    ];

    this.interval = setInterval(() => {
      const eventFn = events[Math.floor(Math.random() * events.length)];
      const event = eventFn();
      this.listeners.forEach((l) => l(event));
    }, 5000); // Emit every 5 seconds
  }
}

export const mockSSE = new MockSSE();
