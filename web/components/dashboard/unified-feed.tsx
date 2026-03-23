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
import { useTranslation } from "@/lib/i18n/context";

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

function ProviderIcon({ provider }: { provider: string }) {
  const icon = getProviderIcon(provider);
  if (!icon) return <div className="h-3.5 w-3.5 shrink-0" />;
  return icon({ size: 14, className: "shrink-0 text-text-muted" });
}

export function UnifiedFeed() {
  const { t } = useTranslation();

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
    },
    { refreshInterval: 30_000 }
  );

  if (isLoading) {
    return (
      <div
        className="rounded-lg bg-surface py-16 text-center border border-border"
      >
        <p
          className="text-text-secondary"
          style={{
            fontFamily: "var(--font-dm-sans)",
            fontSize: "13px",
          }}
        >
          {t("dashboard.feed.loading")}
        </p>
      </div>
    );
  }

  if (!feedItems || feedItems.length === 0) {
    return null; // Empty state handled by parent
  }

  return (
    <div
      className="flex flex-col rounded-lg bg-surface border border-border"
      style={{ height: "336px" }}
    >
      {/* Header */}
      <div
        className="flex items-center justify-between px-5 py-4 shrink-0 border-b border-border"
      >
        <p
          className="text-xs uppercase tracking-[0.08em] text-text-secondary"
          style={{
            fontFamily: "var(--font-dm-sans)",
            fontSize: "12px",
          }}
        >
          {t("dashboard.feed.recentActivity")}
        </p>
      </div>

      {/* Feed items -- scrollable */}
      <div className="flex-1 overflow-y-auto">
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
  const { t } = useTranslation();

  if (item.kind === "semantic") {
    const sr = item.data;
    const urgency = sr.report?.urgency?.toUpperCase();
    const isUrgent = urgency === "CRITICAL" || urgency === "HIGH";

    return (
      <Link
        href={`/projects/${sr.project_id}/semantic-releases/${sr.id}`}
        className="flex items-center gap-3 px-5 py-3 transition-colors hover:bg-background"
        style={{
          borderBottom: isLast ? undefined : "1px solid var(--border)",
        }}
      >
        {/* AI icon */}
        <Sparkles className="h-3.5 w-3.5 shrink-0 text-text-muted" />

        {/* Content */}
        <div className="min-w-0 flex-1 flex items-center gap-2">
          <span
            className="truncate text-text-secondary"
            style={{
              fontFamily: "var(--font-dm-sans)",
              fontSize: "13px",
            }}
          >
            {item.projectName ?? t("dashboard.feed.unknownProject")}
          </span>
          {isUrgent && (
            <span
              className="inline-flex items-center rounded px-1 py-0.5 text-[10px] font-semibold leading-none"
              style={{
                backgroundColor: urgency === "CRITICAL" ? "#fff1f2" : "#fff8f0",
                color: urgency === "CRITICAL" ? "#dc2626" : "#d97706",
              }}
            >
              {urgency}
            </span>
          )}
        </div>

        {/* Version + timestamp */}
        <div className="flex items-center gap-3 shrink-0">
          <VersionChip version={sr.version} />
          <span
            className="whitespace-nowrap text-text-muted"
            style={{
              fontFamily: "var(--font-dm-sans)",
              fontSize: "12px",
            }}
          >
            {timeAgo(getTimeStr({ kind: "semantic", data: sr }))}
          </span>
        </div>
      </Link>
    );
  }

  // Raw release
  const release = item.data;

  return (
    <Link
      href={`/releases/${release.id}`}
      className="flex items-center gap-3 px-5 py-3 transition-colors hover:bg-background"
      style={{
        borderBottom: isLast ? undefined : "1px solid var(--border)",
      }}
    >
      {/* Provider icon */}
      {item.provider ? (
        <ProviderIcon provider={item.provider} />
      ) : (
        <div className="h-3.5 w-3.5 shrink-0" />
      )}

      {/* Content */}
      <div className="min-w-0 flex-1 flex items-center gap-2">
        <span
          className="truncate text-text-secondary"
          style={{
            fontFamily: "'JetBrains Mono', monospace",
            fontSize: "13px",
          }}
        >
          {item.repository ?? release.source_id.slice(0, 12)}
        </span>
        {item.projectName && (
          <span
            className="truncate text-text-muted"
            style={{
              fontFamily: "var(--font-dm-sans)",
              fontSize: "12px",
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
          className="whitespace-nowrap text-text-muted"
          style={{
            fontFamily: "var(--font-dm-sans)",
            fontSize: "12px",
          }}
        >
          {timeAgo(release.released_at ?? release.created_at)}
        </span>
      </div>
    </Link>
  );
}
