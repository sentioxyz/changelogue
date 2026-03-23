"use client";

import { useState, useCallback } from "react";
import useSWR, { mutate } from "swr";
import { suggestions, projects, sources } from "@/lib/api/client";
import type { SuggestionItem } from "@/lib/api/types";

export function StarsTab() {
  const [loaded, setLoaded] = useState(false);
  const [trackingIds, setTrackingIds] = useState<Set<string>>(new Set());

  const { data, isLoading, error } = useSWR(
    loaded ? "suggestions-stars" : null,
    () => suggestions.stars(),
    { revalidateOnFocus: false }
  );

  const items = data?.data ?? [];

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

  if (!loaded) {
    return (
      <div className="flex items-center justify-center py-12">
        <button
          onClick={() => setLoaded(true)}
          className="rounded-lg bg-purple-600 px-6 py-3 text-sm font-semibold text-white hover:bg-purple-700 transition-colors"
        >
          Load your starred repos
        </button>
      </div>
    );
  }

  if (isLoading) {
    return (
      <div className="flex items-center justify-center py-12">
        <div className="text-sm text-zinc-400">Loading your starred repos...</div>
      </div>
    );
  }

  if (error) {
    return (
      <div className="flex items-center justify-center py-12">
        <div className="text-sm text-red-400">Failed to load stars. Try again later.</div>
      </div>
    );
  }

  if (items.length === 0) {
    return (
      <div className="flex items-center justify-center py-12">
        <div className="text-sm text-zinc-400">
          You haven&apos;t starred any public repos on GitHub yet.
        </div>
      </div>
    );
  }

  return (
    <div className="grid grid-cols-1 gap-3 sm:grid-cols-2 lg:grid-cols-3">
      {items.map((item) => (
        <div
          key={item.full_name}
          className={`rounded-lg border p-3 ${
            item.tracked
              ? "border-zinc-700 bg-zinc-800/50 opacity-60"
              : "border-zinc-700 bg-zinc-800"
          }`}
        >
          <div className="flex items-start justify-between gap-2 mb-2">
            <span className="text-sm font-semibold text-zinc-200 truncate">
              {item.full_name}
            </span>
            {item.tracked ? (
              <span className="shrink-0 text-xs text-green-400">✓ Tracked</span>
            ) : (
              <button
                onClick={() => handleTrack(item)}
                disabled={trackingIds.has(item.full_name)}
                className="shrink-0 rounded bg-purple-600 px-2.5 py-1 text-xs font-medium text-white hover:bg-purple-700 disabled:opacity-50 transition-colors"
              >
                {trackingIds.has(item.full_name) ? "Tracking..." : "Track"}
              </button>
            )}
          </div>
          {item.description && (
            <p className="text-xs text-zinc-400 mb-2 line-clamp-2">
              {item.description}
            </p>
          )}
          <div className="flex items-center gap-3 text-xs text-zinc-500">
            <span>⭐ {item.stars.toLocaleString()}</span>
            {item.language && <span>● {item.language}</span>}
          </div>
        </div>
      ))}
    </div>
  );
}
