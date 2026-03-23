"use client";

import { useState, useCallback } from "react";
import useSWR, { mutate } from "swr";
import { Star, Loader2, Check, Plus } from "lucide-react";
import { FaGithub } from "react-icons/fa";
import { suggestions, projects, sources } from "@/lib/api/client";
import type { SuggestionItem } from "@/lib/api/types";

const INITIAL_SHOW = 6;

export function StarsTab() {
  const [trackingIds, setTrackingIds] = useState<Set<string>>(new Set());
  const [showAll, setShowAll] = useState(false);

  const { data, isLoading, error } = useSWR(
    "suggestions-stars",
    () => suggestions.stars(),
    { revalidateOnFocus: false }
  );

  const items = data?.data ?? [];
  const visible = showAll ? items : items.slice(0, INITIAL_SHOW);

  const handleTrack = useCallback(async (item: SuggestionItem) => {
    const key = item.full_name;
    setTrackingIds((prev) => new Set(prev).add(key));
    try {
      const projRes = await projects.create({
        name: item.full_name,
        description: item.description,
      });
      const project = projRes.data;
      const sourceRes = await sources.create(project.id, {
        provider: "github",
        repository: item.full_name,
        poll_interval_seconds: 86400,
        enabled: true,
      });
      await sources.poll(sourceRes.data.id);
      await mutate("suggestions-stars");
      await mutate("projects-list");
      await mutate("projects-for-dashboard");
    } catch (err: unknown) {
      const msg = err instanceof Error ? err.message : "Failed to track";
      alert(msg);
    } finally {
      setTrackingIds((prev) => {
        const next = new Set(prev);
        next.delete(key);
        return next;
      });
    }
  }, []);

  if (isLoading) {
    return (
      <div className="flex items-center justify-center py-10">
        <Loader2
          className="h-4 w-4 animate-spin"
          style={{ color: "#e8601a" }}
        />
        <span
          className="ml-2"
          style={{
            fontFamily: "var(--font-dm-sans)",
            fontSize: "13px",
            color: "#6b7280",
          }}
        >
          Loading your starred repos...
        </span>
      </div>
    );
  }

  if (error) {
    return (
      <div className="flex items-center justify-center py-10">
        <span
          style={{
            fontFamily: "var(--font-dm-sans)",
            fontSize: "13px",
            color: "#ef4444",
          }}
        >
          Failed to load stars. Try again later.
        </span>
      </div>
    );
  }

  if (items.length === 0) {
    return (
      <div className="flex items-center justify-center py-10">
        <span
          style={{
            fontFamily: "var(--font-dm-sans)",
            fontSize: "13px",
            color: "#6b7280",
          }}
        >
          You haven&apos;t starred any public repos on GitHub yet.
        </span>
      </div>
    );
  }

  return (
    <div>
      <div className="grid grid-cols-1 gap-3 sm:grid-cols-2 lg:grid-cols-3">
        {visible.map((item) => {
          const tracked = item.tracked;
          const tracking = trackingIds.has(item.full_name);

          return (
            <div
              key={item.full_name}
              className="rounded-lg"
              style={{
                border: "1px solid #e8e8e5",
                padding: "12px",
                backgroundColor: tracked ? "#fafaf9" : "#ffffff",
                opacity: tracked ? 0.6 : 1,
              }}
            >
              <div className="flex items-start justify-between gap-2 mb-2">
                <div className="flex items-center gap-1.5 min-w-0">
                  <FaGithub
                    className="h-3.5 w-3.5 flex-shrink-0"
                    style={{ color: "#6b7280" }}
                  />
                  <span
                    className="truncate"
                    style={{
                      fontFamily: "var(--font-dm-sans)",
                      fontSize: "13px",
                      fontWeight: 600,
                      color: "#111113",
                    }}
                  >
                    {item.full_name}
                  </span>
                </div>
                {tracked ? (
                  <span
                    className="flex items-center gap-1 shrink-0"
                    style={{
                      fontFamily: "var(--font-dm-sans)",
                      fontSize: "11px",
                      color: "#16a34a",
                    }}
                  >
                    <Check className="h-3 w-3" />
                    Tracked
                  </span>
                ) : (
                  <button
                    onClick={() => handleTrack(item)}
                    disabled={tracking}
                    className="shrink-0 flex items-center gap-1 rounded-md px-2.5 py-1 text-white transition-opacity hover:opacity-90 disabled:opacity-50"
                    style={{
                      backgroundColor: "#e8601a",
                      fontFamily: "var(--font-dm-sans)",
                      fontSize: "11px",
                      fontWeight: 500,
                      cursor: tracking ? "wait" : "pointer",
                    }}
                  >
                    {tracking ? (
                      <Loader2 className="h-3 w-3 animate-spin" />
                    ) : (
                      <Plus className="h-3 w-3" />
                    )}
                    {tracking ? "Tracking..." : "Track"}
                  </button>
                )}
              </div>
              {item.description && (
                <p
                  className="line-clamp-2 mb-2"
                  style={{
                    fontFamily: "var(--font-dm-sans)",
                    fontSize: "12px",
                    color: "#6b7280",
                    lineHeight: "1.4",
                  }}
                >
                  {item.description}
                </p>
              )}
              <div className="flex items-center gap-3">
                <span
                  className="flex items-center gap-1"
                  style={{
                    fontFamily: "var(--font-dm-sans)",
                    fontSize: "11px",
                    color: "#9ca3af",
                  }}
                >
                  <Star className="h-3 w-3" style={{ color: "#e8601a" }} />
                  {item.stars.toLocaleString()}
                </span>
                {item.language && (
                  <span
                    style={{
                      fontFamily: "var(--font-dm-sans)",
                      fontSize: "11px",
                      color: "#9ca3af",
                    }}
                  >
                    {item.language}
                  </span>
                )}
              </div>
            </div>
          );
        })}
      </div>
      {items.length > INITIAL_SHOW && !showAll && (
        <div className="text-center mt-3">
          <button
            onClick={() => setShowAll(true)}
            style={{
              fontFamily: "var(--font-dm-sans)",
              fontSize: "12px",
              color: "#e8601a",
              background: "none",
              border: "none",
              cursor: "pointer",
            }}
          >
            Show all {items.length} starred repos →
          </button>
        </div>
      )}
    </div>
  );
}
