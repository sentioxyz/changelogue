// web/components/dashboard/discovery-section.tsx
"use client";

import { useState, useCallback } from "react";
import useSWR, { mutate } from "swr";
import { Star, Loader2, Check } from "lucide-react";
import { FaGithub, FaDocker } from "react-icons/fa";
import { discover, projects, sources } from "@/lib/api/client";
import type { DiscoverItem } from "@/lib/api/types";

type Tab = "github" | "dockerhub";

function formatStars(count: number): string {
  if (count >= 1000) {
    const k = count / 1000;
    return k >= 10 ? `${Math.round(k)}k` : `${Math.round(k * 10) / 10}k`;
  }
  return String(count);
}

export function DiscoverySection() {
  const [activeTab, setActiveTab] = useState<Tab>("github");
  const [trackingIds, setTrackingIds] = useState<Set<string>>(new Set());

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

  const trackedNames = new Set(
    (projectsData?.data ?? []).flatMap((p) => [p.name, p.name.split("/").pop()!]),
  );

  const items = activeTab === "github" ? githubData?.data : dockerData?.data;
  const isLoading = activeTab === "github" ? githubLoading : dockerLoading;

  const isTracked = useCallback(
    (item: DiscoverItem) =>
      trackedNames.has(item.full_name) || trackedNames.has(item.name),
    // eslint-disable-next-line react-hooks/exhaustive-deps
    [projectsData],
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
      } catch (err: unknown) {
        const msg = err instanceof Error ? err.message : "Failed to track project";
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

  return (
    <div
      className="rounded-lg bg-white"
      style={{ border: "1px solid #e8e8e5" }}
    >
      {/* Header */}
      <div className="flex items-center justify-between px-5 pt-4 pb-3">
        <div>
          <h3
            style={{
              fontFamily: "var(--font-fraunces)",
              fontSize: "16px",
              fontWeight: 600,
              color: "#111113",
            }}
          >
            Trending Projects
          </h3>
          <p
            className="mt-0.5"
            style={{
              fontFamily: "var(--font-dm-sans)",
              fontSize: "12px",
              color: "#6b7280",
            }}
          >
            Track trending repositories with one click
          </p>
        </div>

        {/* Tabs */}
        <div
          className="flex items-center gap-1 rounded-full p-1"
          style={{ backgroundColor: "#f5f5f3" }}
        >
          <TabButton
            active={activeTab === "github"}
            onClick={() => setActiveTab("github")}
            icon={<FaGithub className="h-3.5 w-3.5" />}
            label="GitHub"
          />
          <TabButton
            active={activeTab === "dockerhub"}
            onClick={() => setActiveTab("dockerhub")}
            icon={<FaDocker className="h-3.5 w-3.5" />}
            label="Docker Hub"
          />
        </div>
      </div>

      {/* Scrollable cards */}
      <div className="flex gap-3 overflow-x-auto px-5 pb-4" style={{ scrollbarWidth: "thin" }}>
        {isLoading
          ? Array.from({ length: 5 }).map((_, i) => <SkeletonCard key={i} />)
          : (items ?? []).map((item) => (
              <DiscoverCard
                key={item.full_name}
                item={item}
                tracked={isTracked(item)}
                tracking={trackingIds.has(item.full_name)}
                onTrack={handleTrack}
                showLanguage={activeTab === "github"}
              />
            ))}
      </div>
    </div>
  );
}

/* ------------------------------------------------------------------ */

function TabButton({
  active,
  onClick,
  icon,
  label,
}: {
  active: boolean;
  onClick: () => void;
  icon: React.ReactNode;
  label: string;
}) {
  return (
    <button
      onClick={onClick}
      className="flex items-center gap-1.5 rounded-full px-3 py-1 text-xs transition-all"
      style={{
        fontFamily: "var(--font-dm-sans)",
        backgroundColor: active ? "#ffffff" : "transparent",
        boxShadow: active ? "0 1px 2px rgba(0,0,0,0.06)" : "none",
        color: active ? "#111113" : "#6b7280",
        fontWeight: active ? 500 : 400,
      }}
    >
      {icon}
      {label}
    </button>
  );
}

/* ------------------------------------------------------------------ */

function DiscoverCard({
  item,
  tracked,
  tracking,
  onTrack,
  showLanguage,
}: {
  item: DiscoverItem;
  tracked: boolean;
  tracking: boolean;
  onTrack: (item: DiscoverItem) => void;
  showLanguage: boolean;
}) {
  return (
    <div
      className="flex flex-shrink-0 flex-col justify-between rounded-lg p-3.5"
      style={{
        width: 220,
        border: "1px solid #e8e8e5",
        minHeight: 160,
      }}
    >
      <div>
        <p
          className="truncate"
          style={{
            fontFamily: "var(--font-dm-sans)",
            fontSize: "14px",
            fontWeight: 500,
            color: "#111113",
          }}
          title={item.full_name}
        >
          {item.full_name}
        </p>
        <p
          className="mt-1"
          style={{
            fontFamily: "var(--font-dm-sans)",
            fontSize: "12px",
            color: "#6b7280",
            display: "-webkit-box",
            WebkitLineClamp: 2,
            WebkitBoxOrient: "vertical",
            overflow: "hidden",
          }}
        >
          {item.description || "No description"}
        </p>
      </div>

      <div className="mt-3 flex items-center justify-between">
        <div className="flex items-center gap-2">
          <span className="flex items-center gap-1">
            <Star className="h-3.5 w-3.5" style={{ color: "#e8601a" }} />
            <span
              style={{
                fontFamily: "var(--font-dm-sans)",
                fontSize: "12px",
                color: "#111113",
              }}
            >
              {formatStars(item.stars)}
            </span>
          </span>
          {showLanguage && item.language && (
            <span
              style={{
                fontFamily: "var(--font-dm-sans)",
                fontSize: "12px",
                color: "#9ca3af",
              }}
            >
              {item.language}
            </span>
          )}
        </div>

        {tracked ? (
          <span
            className="flex items-center gap-1 text-xs"
            style={{
              fontFamily: "var(--font-dm-sans)",
              color: "#16a34a",
              fontWeight: 500,
            }}
          >
            <Check className="h-3.5 w-3.5" />
            Tracked
          </span>
        ) : (
          <button
            onClick={() => onTrack(item)}
            disabled={tracking}
            className="flex items-center gap-1 rounded px-2.5 py-1 text-xs text-white transition-colors hover:opacity-90 disabled:opacity-70"
            style={{
              fontFamily: "var(--font-dm-sans)",
              backgroundColor: "#e8601a",
            }}
          >
            {tracking ? (
              <Loader2 className="h-3.5 w-3.5 animate-spin" />
            ) : null}
            Track
          </button>
        )}
      </div>
    </div>
  );
}

/* ------------------------------------------------------------------ */

function SkeletonCard() {
  return (
    <div
      className="flex flex-shrink-0 flex-col justify-between rounded-lg p-3.5"
      style={{
        width: 220,
        border: "1px solid #e8e8e5",
        minHeight: 160,
      }}
    >
      <div>
        <div className="h-4 w-3/4 animate-pulse rounded" style={{ backgroundColor: "#e8e8e5" }} />
        <div className="mt-2 h-3 w-full animate-pulse rounded" style={{ backgroundColor: "#f0f0ed" }} />
        <div className="mt-1 h-3 w-2/3 animate-pulse rounded" style={{ backgroundColor: "#f0f0ed" }} />
      </div>
      <div className="mt-3 flex items-center justify-between">
        <div className="h-3 w-10 animate-pulse rounded" style={{ backgroundColor: "#e8e8e5" }} />
        <div className="h-6 w-14 animate-pulse rounded" style={{ backgroundColor: "#e8e8e5" }} />
      </div>
    </div>
  );
}
