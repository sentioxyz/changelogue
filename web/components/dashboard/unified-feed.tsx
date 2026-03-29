// web/components/dashboard/unified-feed.tsx
"use client";

import useSWR from "swr";
import Link from "next/link";
import {
  releases as releasesApi,
  semanticReleases as srApi,
} from "@/lib/api/client";
import { VersionChip } from "@/components/ui/version-chip";
import { getProviderIcon } from "@/components/ui/provider-badge";
import { Sparkles } from "lucide-react";
import type { Release, SemanticRelease } from "@/lib/api/types";
import { timeAgo } from "@/lib/format";
import { useTranslation } from "@/lib/i18n/context";
import { UrgencyPill } from "@/components/ui/urgency-pill";

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

  const { data: feedItems, isLoading } = useSWR(
    "unified-feed",
    async () => {
      // Fetch recent releases and semantic releases globally (server sorts by recency)
      const [releasesRes, srRes] = await Promise.all([
        releasesApi.list(1, 15).catch(() => null),
        srApi.listAll(1, 15).catch(() => null),
      ]);

      const items: FeedItemType[] = [];

      if (releasesRes?.data) {
        for (const rel of releasesRes.data) {
          items.push({
            kind: "release",
            data: rel,
            repository: rel.repository,
            provider: rel.provider,
            projectName: rel.project_name,
          });
        }
      }

      if (srRes?.data) {
        for (const sr of srRes.data) {
          items.push({
            kind: "semantic",
            data: sr,
            projectName: sr.project_name,
          });
        }
      }

      // Merge and sort by timestamp descending, take first 15
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
          {isUrgent && urgency && (
            <UrgencyPill urgency={urgency} variant="text" />
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
