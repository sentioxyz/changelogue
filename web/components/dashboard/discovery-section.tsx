// web/components/dashboard/discovery-section.tsx
"use client";

import { useState, useCallback, useMemo } from "react";
import useSWR, { mutate } from "swr";
import { Star, Loader2, Check, Plus } from "lucide-react";
import { FaGithub, FaDocker } from "react-icons/fa";
import { discover, projects, sources } from "@/lib/api/client";
import type { DiscoverItem } from "@/lib/api/types";

function formatStars(count: number): string {
  if (count >= 1000) {
    const k = count / 1000;
    return k >= 10 ? `${Math.round(k)}k` : `${Math.round(k * 10) / 10}k`;
  }
  return String(count);
}

export function DiscoverySection() {
  const [trackingIds, setTrackingIds] = useState<Set<string>>(new Set());
  const [paused, setPaused] = useState(false);

  const { data: githubData, isLoading: githubLoading } = useSWR(
    "discover-github",
    () => discover.github(),
    { revalidateOnFocus: false },
  );

  const { data: dockerData, isLoading: dockerLoading } = useSWR(
    "discover-dockerhub",
    () => discover.dockerhub(),
    { revalidateOnFocus: false },
  );

  const { data: projectsData } = useSWR(
    "projects-list",
    () => projects.list(1, 200),
    { revalidateOnFocus: false },
  );

  const trackedNames = useMemo(
    () =>
      new Set(
        (projectsData?.data ?? []).flatMap((p) => [
          p.name,
          p.name.split("/").pop()!,
        ]),
      ),
    [projectsData],
  );

  // Interleave GitHub and Docker Hub items
  const allItems = useMemo(() => {
    const gh = githubData?.data ?? [];
    const dh = dockerData?.data ?? [];
    const merged: DiscoverItem[] = [];
    const maxLen = Math.max(gh.length, dh.length);
    for (let i = 0; i < maxLen; i++) {
      if (i < gh.length) merged.push(gh[i]);
      if (i < dh.length) merged.push(dh[i]);
    }
    return merged;
  }, [githubData, dockerData]);

  const isLoading = githubLoading && dockerLoading;

  const isTracked = useCallback(
    (item: DiscoverItem) =>
      trackedNames.has(item.full_name) || trackedNames.has(item.name),
    [trackedNames],
  );

  const handleTrack = useCallback(
    async (item: DiscoverItem) => {
      const key = item.full_name;
      setTrackingIds((prev) => new Set(prev).add(key));
      try {
        const projRes = await projects.create({
          name: item.full_name,
          description: item.description,
        });
        const project = projRes.data;

        const provider = item.provider === "github" ? "github" : "dockerhub";
        const sourceRes = await sources.create(project.id, {
          provider,
          repository: item.full_name,
          poll_interval_seconds: 86400,
          enabled: true,
        });

        await sources.poll(sourceRes.data.id);
        await mutate("projects-list");
        await mutate("projects-for-dashboard");
      } catch (err: unknown) {
        const msg =
          err instanceof Error ? err.message : "Failed to track project";
        alert(msg);
      } finally {
        setTrackingIds((prev) => {
          const next = new Set(prev);
          next.delete(key);
          return next;
        });
      }
    },
    [],
  );

  if (isLoading) {
    return (
      <div
        className="rounded-lg bg-white px-5 py-4"
        style={{ border: "1px solid #e8e8e5" }}
      >
        <div className="flex items-center gap-3">
          <span
            style={{
              fontFamily: "var(--font-fraunces)",
              fontSize: "14px",
              fontWeight: 600,
              color: "#111113",
            }}
          >
            Trending
          </span>
          <div className="flex gap-3 overflow-hidden">
            {Array.from({ length: 6 }).map((_, i) => (
              <div
                key={i}
                className="h-8 w-48 flex-shrink-0 animate-pulse rounded-full"
                style={{ backgroundColor: "#f0f0ed" }}
              />
            ))}
          </div>
        </div>
      </div>
    );
  }

  if (allItems.length === 0) return null;

  return (
    <div
      className="rounded-lg bg-white"
      style={{ border: "1px solid #e8e8e5", overflow: "hidden" }}
    >
      <div className="flex items-center gap-4 px-5 py-3">
        {/* Label */}
        <span
          className="flex-shrink-0"
          style={{
            fontFamily: "var(--font-fraunces)",
            fontSize: "14px",
            fontWeight: 600,
            color: "#111113",
          }}
        >
          Trending
        </span>

        {/* Marquee container */}
        <div
          className="relative flex-1 overflow-hidden"
          onMouseEnter={() => setPaused(true)}
          onMouseLeave={() => setPaused(false)}
        >
          {/* Fade edges */}
          <div
            className="pointer-events-none absolute left-0 top-0 z-10 h-full w-8"
            style={{
              background:
                "linear-gradient(to right, white, transparent)",
            }}
          />
          <div
            className="pointer-events-none absolute right-0 top-0 z-10 h-full w-8"
            style={{
              background:
                "linear-gradient(to left, white, transparent)",
            }}
          />

          {/* Scrolling track — duplicated for seamless loop */}
          <div
            className="flex gap-2.5"
            style={{
              animation: `marquee ${Math.max(allItems.length * 3, 30)}s linear infinite`,
              animationPlayState: paused ? "paused" : "running",
              width: "max-content",
            }}
          >
            {[...allItems, ...allItems].map((item, idx) => {
              const tracked = isTracked(item);
              const tracking = trackingIds.has(item.full_name);

              return (
                <MarqueeChip
                  key={`${item.full_name}-${idx}`}
                  item={item}
                  tracked={tracked}
                  tracking={tracking}
                  onTrack={handleTrack}
                />
              );
            })}
          </div>
        </div>
      </div>

      {/* Keyframes injected via style tag */}
      <style>{`
        @keyframes marquee {
          0% { transform: translateX(0); }
          100% { transform: translateX(-50%); }
        }
      `}</style>
    </div>
  );
}

/* ------------------------------------------------------------------ */

function MarqueeChip({
  item,
  tracked,
  tracking,
  onTrack,
}: {
  item: DiscoverItem;
  tracked: boolean;
  tracking: boolean;
  onTrack: (item: DiscoverItem) => void;
}) {
  const ProviderIcon =
    item.provider === "github" ? FaGithub : FaDocker;

  if (tracked) {
    return (
      <span
        className="inline-flex flex-shrink-0 items-center gap-1.5 rounded-full px-3 py-1.5"
        style={{
          fontFamily: "var(--font-dm-sans)",
          fontSize: "12px",
          color: "#16a34a",
          backgroundColor: "#f0fdf4",
          border: "1px solid #bbf7d0",
          whiteSpace: "nowrap",
        }}
      >
        <ProviderIcon className="h-3 w-3" style={{ color: "#6b7280" }} />
        {item.full_name}
        <Check className="h-3 w-3" />
      </span>
    );
  }

  return (
    <button
      onClick={() => onTrack(item)}
      disabled={tracking}
      className="inline-flex flex-shrink-0 items-center gap-1.5 rounded-full px-3 py-1.5 transition-all hover:shadow-sm disabled:opacity-70"
      style={{
        fontFamily: "var(--font-dm-sans)",
        fontSize: "12px",
        color: "#111113",
        backgroundColor: "#fafaf9",
        border: "1px solid #e8e8e5",
        whiteSpace: "nowrap",
        cursor: tracking ? "wait" : "pointer",
      }}
    >
      <ProviderIcon className="h-3 w-3" style={{ color: "#6b7280" }} />
      <span>{item.full_name}</span>
      <Star className="h-3 w-3" style={{ color: "#e8601a" }} />
      <span style={{ color: "#6b7280", fontSize: "11px" }}>
        {formatStars(item.stars)}
      </span>
      {tracking ? (
        <Loader2
          className="h-3 w-3 animate-spin"
          style={{ color: "#e8601a" }}
        />
      ) : (
        <Plus className="h-3 w-3" style={{ color: "#e8601a" }} />
      )}
    </button>
  );
}
