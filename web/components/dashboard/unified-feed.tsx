// web/components/dashboard/unified-feed.tsx
"use client";

import useSWR from "swr";
import Link from "next/link";
import {
  projects as projectsApi,
  releases as releasesApi,
  sources as sourcesApi,
  semanticReleases as srApi,
} from "@/lib/api/client";
import { VersionChip } from "@/components/ui/version-chip";
import { getProviderIcon } from "@/components/ui/provider-badge";
import { Sparkles } from "lucide-react";
import type { Release, SemanticRelease, Source } from "@/lib/api/types";
import { timeAgo } from "@/lib/format";

type FeedItemType =
  | { kind: "release"; data: Release; repository?: string; provider?: string; projectName?: string }
  | { kind: "semantic"; data: SemanticRelease; projectName?: string };

function getTimestamp(item: FeedItemType): number {
  if (item.kind === "release") {
    return new Date(item.data.released_at ?? item.data.created_at).getTime();
  }
  return new Date(item.data.created_at).getTime();
}

function getTimeStr(item: FeedItemType): string {
  if (item.kind === "release") {
    return item.data.released_at ?? item.data.created_at;
  }
  return item.data.created_at;
}

export function UnifiedFeed() {
  const { data: projectsData } = useSWR("projects-for-dashboard", () =>
    projectsApi.list()
  );

  const { data: feedItems, isLoading } = useSWR(
    projectsData ? "unified-feed" : null,
    async () => {
      if (!projectsData?.data?.length) return [];

      const projectMap = new Map(
        projectsData.data.map((p) => [p.id, p.name])
      );
      const projectSlice = projectsData.data.slice(0, 10);

      // Fetch releases, sources, and semantic releases in parallel
      const [releaseResults, sourceResults, srResults] = await Promise.all([
        Promise.all(
          projectSlice.map((p) =>
            releasesApi.listByProject(p.id, 1).catch(() => null)
          )
        ),
        Promise.all(
          projectSlice.map((p) =>
            sourcesApi.listByProject(p.id, 1).catch(() => null)
          )
        ),
        Promise.all(
          projectSlice.map((p) =>
            srApi.list(p.id, 1).catch(() => null)
          )
        ),
      ]);

      // Build source lookup maps
      const sourceMap = new Map<string, string>();
      const providerMap = new Map<string, string>();
      const sourceProjectMap = new Map<string, string>();
      sourceResults
        .filter((r): r is NonNullable<typeof r> => r !== null)
        .flatMap((r) => r.data)
        .forEach((s: Source) => {
          sourceMap.set(s.id, s.repository);
          providerMap.set(s.id, s.provider);
          sourceProjectMap.set(s.id, s.project_id);
        });

      // Build feed items
      const items: FeedItemType[] = [];

      releaseResults
        .filter((r): r is NonNullable<typeof r> => r !== null)
        .flatMap((r) => r.data)
        .forEach((rel) => {
          const projectId = sourceProjectMap.get(rel.source_id);
          items.push({
            kind: "release",
            data: rel,
            repository: sourceMap.get(rel.source_id),
            provider: providerMap.get(rel.source_id),
            projectName: projectId ? projectMap.get(projectId) : undefined,
          });
        });

      srResults
        .filter((r): r is NonNullable<typeof r> => r !== null)
        .flatMap((r) => r.data)
        .forEach((sr) => {
          items.push({
            kind: "semantic",
            data: sr,
            projectName: projectMap.get(sr.project_id),
          });
        });

      // Sort by timestamp descending, take first 15
      items.sort((a, b) => getTimestamp(b) - getTimestamp(a));
      return items.slice(0, 15);
    }
  );

  if (isLoading) {
    return (
      <div
        className="rounded-lg bg-white py-16 text-center"
        style={{ border: "1px solid #e8e8e5" }}
      >
        <p
          style={{
            fontFamily: "var(--font-dm-sans)",
            fontSize: "13px",
            color: "#6b7280",
          }}
        >
          Loading activity...
        </p>
      </div>
    );
  }

  if (!feedItems || feedItems.length === 0) {
    return null; // Empty state handled by parent
  }

  return (
    <div
      className="rounded-lg bg-white"
      style={{ border: "1px solid #e8e8e5" }}
    >
      {/* Header */}
      <div
        className="flex items-center justify-between px-5 py-4"
        style={{ borderBottom: "1px solid #e8e8e5" }}
      >
        <h3
          className="font-medium"
          style={{
            fontFamily: "var(--font-dm-sans)",
            fontSize: "13px",
            color: "#111113",
          }}
        >
          Recent Activity
        </h3>
        <Link
          href="/releases"
          className="text-sm hover:underline"
          style={{
            fontFamily: "var(--font-dm-sans)",
            fontSize: "13px",
            color: "#e8601a",
          }}
        >
          View all &rarr;
        </Link>
      </div>

      {/* Feed items */}
      <div>
        {feedItems.map((item, idx) => (
          <FeedEntry
            key={item.kind === "release" ? `r-${item.data.id}` : `sr-${item.data.id}`}
            item={item}
            isLast={idx === feedItems.length - 1}
          />
        ))}
      </div>
    </div>
  );
}

function FeedEntry({ item, isLast }: { item: FeedItemType; isLast: boolean }) {
  if (item.kind === "semantic") {
    const sr = item.data;
    const urgency = sr.report?.urgency?.toUpperCase();
    const isUrgent = urgency === "CRITICAL" || urgency === "HIGH";

    return (
      <Link
        href={`/projects/${sr.project_id}/semantic-releases/${sr.id}`}
        className="flex items-start gap-3 px-5 py-3.5 transition-colors hover:bg-[#fafaf9]"
        style={{
          borderBottom: isLast ? undefined : "1px solid #e8e8e5",
          borderLeft: "3px solid #e8601a",
          backgroundColor: "#fffcfa",
        }}
      >
        {/* AI icon */}
        <Sparkles className="mt-0.5 h-4 w-4 shrink-0" style={{ color: "#e8601a" }} />

        {/* Content */}
        <div className="min-w-0 flex-1">
          <div className="flex items-center gap-2">
            <span
              className="font-semibold truncate"
              style={{
                fontFamily: "var(--font-fraunces)",
                fontSize: "14px",
                color: "#111113",
              }}
            >
              {item.projectName ?? "Unknown Project"}
            </span>
            <VersionChip version={sr.version} />
            {isUrgent && (
              <span
                className="inline-flex items-center rounded px-1.5 py-0.5 text-[11px] font-semibold leading-none"
                style={{
                  backgroundColor: urgency === "CRITICAL" ? "#fff1f2" : "#fff8f0",
                  color: urgency === "CRITICAL" ? "#dc2626" : "#d97706",
                }}
              >
                {urgency}
              </span>
            )}
          </div>
          {sr.report?.summary && (
            <p
              className="mt-1 line-clamp-1"
              style={{
                fontFamily: "var(--font-dm-sans)",
                fontStyle: "italic",
                fontSize: "13px",
                color: "#6b7280",
              }}
            >
              {sr.report.summary}
            </p>
          )}
        </div>

        {/* Timestamp */}
        <span
          className="mt-0.5 shrink-0 whitespace-nowrap"
          style={{
            fontFamily: "var(--font-dm-sans)",
            fontSize: "12px",
            color: "#9ca3af",
          }}
        >
          {timeAgo(getTimeStr({ kind: "semantic", data: sr }))}
        </span>
      </Link>
    );
  }

  // Raw release
  const release = item.data;
  const Icon = item.provider ? getProviderIcon(item.provider) : undefined;

  return (
    <Link
      href={`/releases/${release.id}`}
      className="flex items-center gap-3 px-5 py-3 transition-colors hover:bg-[#fafaf9]"
      style={{
        borderBottom: isLast ? undefined : "1px solid #e8e8e5",
      }}
    >
      {/* Provider icon */}
      {Icon ? (
        <Icon size={14} className="shrink-0" style={{ color: "#9ca3af" }} />
      ) : (
        <div className="h-3.5 w-3.5 shrink-0" />
      )}

      {/* Content */}
      <div className="min-w-0 flex-1 flex items-center gap-2">
        <span
          className="truncate"
          style={{
            fontFamily: "'JetBrains Mono', monospace",
            fontSize: "13px",
            color: "#6b7280",
          }}
        >
          {item.repository ?? release.source_id.slice(0, 12)}
        </span>
        {item.projectName && (
          <span
            className="truncate"
            style={{
              fontFamily: "var(--font-dm-sans)",
              fontSize: "12px",
              color: "#9ca3af",
            }}
          >
            · {item.projectName}
          </span>
        )}
      </div>

      {/* Version + timestamp */}
      <div className="flex items-center gap-3 shrink-0">
        <VersionChip version={release.version} />
        <span
          className="whitespace-nowrap"
          style={{
            fontFamily: "var(--font-dm-sans)",
            fontSize: "12px",
            color: "#9ca3af",
          }}
        >
          {timeAgo(release.released_at ?? release.created_at)}
        </span>
      </div>
    </Link>
  );
}
