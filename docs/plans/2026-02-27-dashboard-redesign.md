# Dashboard Redesign: Unified Timeline — Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Replace the ops-centric dashboard with a unified timeline showing rethought stats + chronological activity feed mixing releases and semantic releases.

**Architecture:** Client-side only change. Three new stats computed from existing APIs (total_projects from `/stats`, releases_this-week and attention_needed computed client-side from existing release/SR data). Unified feed merges `/releases` + `/semantic-releases` client-side via SWR, sorted by timestamp. Backend `/stats` endpoint extended with two new fields for efficiency.

**Tech Stack:** Next.js 16, React, SWR, Tailwind CSS v4, Lucide icons, existing design system (Fraunces + DM Sans fonts, `#e8601a` accent).

---

### Task 1: Extend Backend Stats Endpoint

Add `releases_this_week` and `attention_needed` fields to the stats response.

**Files:**
- Modify: `internal/api/health.go:9-14`
- Modify: `internal/api/pgstore.go:838-852`
- Modify: `web/lib/api/types.ts:167-172`

**Step 1: Add fields to DashboardStats struct**

In `internal/api/health.go`, update:

```go
type DashboardStats struct {
	TotalReleases    int `json:"total_releases"`
	ActiveSources    int `json:"active_sources"`
	TotalProjects    int `json:"total_projects"`
	PendingAgentRuns int `json:"pending_agent_runs"`
	ReleasesThisWeek int `json:"releases_this_week"`
	AttentionNeeded  int `json:"attention_needed"`
}
```

**Step 2: Add queries to GetStats**

In `internal/api/pgstore.go`, append to `GetStats` before the return:

```go
if err := s.pool.QueryRow(ctx, `SELECT COUNT(*) FROM releases WHERE created_at >= NOW() - INTERVAL '7 days'`).Scan(&stats.ReleasesThisWeek); err != nil {
    return nil, fmt.Errorf("count releases this week: %w", err)
}
if err := s.pool.QueryRow(ctx, `SELECT COUNT(*) FROM semantic_releases WHERE status = 'completed' AND report->>'urgency' IN ('critical', 'high', 'CRITICAL', 'HIGH')`).Scan(&stats.AttentionNeeded); err != nil {
    return nil, fmt.Errorf("count attention needed: %w", err)
}
```

**Step 3: Update TypeScript Stats type**

In `web/lib/api/types.ts`, update:

```typescript
export interface Stats {
  total_projects: number;
  active_sources: number;
  total_releases: number;
  pending_agent_runs: number;
  releases_this_week: number;
  attention_needed: number;
}
```

**Step 4: Run tests**

Run: `go test ./internal/api/...`
Expected: PASS (existing tests should still pass, new fields are additive)

**Step 5: Commit**

```bash
git add internal/api/health.go internal/api/pgstore.go web/lib/api/types.ts
git commit -m "feat(api): add releases_this_week and attention_needed to stats endpoint"
```

---

### Task 2: Rewrite Stats Cards Component

Replace the 4 ops-metric cards with 3 user-centric stats.

**Files:**
- Modify: `web/components/dashboard/stats-cards.tsx`

**Step 1: Rewrite stats-cards.tsx**

```tsx
// web/components/dashboard/stats-cards.tsx
"use client";

import useSWR from "swr";
import { system } from "@/lib/api/client";
import { FolderKanban, TrendingUp, AlertTriangle } from "lucide-react";
import type { LucideIcon } from "lucide-react";

interface StatItem {
  label: string;
  value: number | string;
  icon: LucideIcon;
  accent?: boolean;
}

export function StatsCards() {
  const { data, isLoading } = useSWR("stats", () => system.stats());

  const stats = data?.data;
  const attentionCount = stats?.attention_needed ?? 0;

  const items: StatItem[] = [
    { label: "Projects Tracked", value: stats?.total_projects ?? "—", icon: FolderKanban },
    { label: "Releases This Week", value: stats?.releases_this_week ?? "—", icon: TrendingUp },
    { label: "Needs Attention", value: attentionCount, icon: AlertTriangle, accent: attentionCount > 0 },
  ];

  return (
    <div className="grid gap-4 sm:grid-cols-3">
      {items.map((item) => (
        <div
          key={item.label}
          className="relative rounded-lg bg-white px-5 py-4"
          style={{
            border: item.accent ? "1px solid #e8601a" : "1px solid #e8e8e5",
            backgroundColor: item.accent ? "#fff8f0" : "#ffffff",
          }}
        >
          <item.icon
            className="absolute right-4 top-4 h-4 w-4"
            style={{ color: item.accent ? "#e8601a" : "#b0b0a8" }}
          />
          <p
            className="text-xs uppercase tracking-[0.08em]"
            style={{
              fontFamily: "var(--font-dm-sans)",
              fontSize: "12px",
              color: item.accent ? "#e8601a" : "#6b7280",
            }}
          >
            {item.label}
          </p>
          <p
            className="mt-1 font-bold"
            style={{
              fontFamily: "var(--font-fraunces)",
              fontSize: "32px",
              lineHeight: 1.1,
              color: item.accent ? "#e8601a" : "#111113",
            }}
          >
            {isLoading ? "···" : item.value}
          </p>
        </div>
      ))}
    </div>
  );
}
```

**Step 2: Verify visually**

Run: `cd web && npm run dev` (or `make frontend-dev`)
Navigate to `/` and confirm 3 cards render with correct labels.

**Step 3: Commit**

```bash
git add web/components/dashboard/stats-cards.tsx
git commit -m "feat(dashboard): redesign stats cards with user-centric metrics"
```

---

### Task 3: Build Unified Activity Feed Component

Create the new activity feed that merges releases and semantic releases chronologically.

**Files:**
- Create: `web/components/dashboard/unified-feed.tsx`

**Step 1: Create the component**

```tsx
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
```

**Step 2: Verify the component compiles**

Run: `cd web && npx next build` (or just visit in dev mode)
Expected: No TypeScript errors.

**Step 3: Commit**

```bash
git add web/components/dashboard/unified-feed.tsx
git commit -m "feat(dashboard): add unified chronological activity feed component"
```

---

### Task 4: Build Empty State Component

Create the empty state shown when no projects exist.

**Files:**
- Create: `web/components/dashboard/empty-state.tsx`

**Step 1: Create the component**

```tsx
// web/components/dashboard/empty-state.tsx
"use client";

import Link from "next/link";
import { FolderPlus } from "lucide-react";

export function DashboardEmptyState() {
  return (
    <div
      className="rounded-lg bg-white px-8 py-16 text-center"
      style={{ border: "1px solid #e8e8e5" }}
    >
      <FolderPlus className="mx-auto h-10 w-10" style={{ color: "#b0b0a8" }} />
      <h2
        className="mt-4"
        style={{
          fontFamily: "var(--font-fraunces)",
          fontSize: "18px",
          fontWeight: 600,
          color: "#111113",
        }}
      >
        No projects yet
      </h2>
      <p
        className="mt-2"
        style={{
          fontFamily: "var(--font-dm-sans)",
          fontSize: "14px",
          color: "#6b7280",
        }}
      >
        Start tracking releases by creating your first project.
      </p>
      <Link
        href="/projects"
        className="mt-6 inline-flex items-center gap-2 rounded-lg px-5 py-2.5 text-sm font-medium text-white transition-colors hover:opacity-90"
        style={{
          fontFamily: "var(--font-dm-sans)",
          backgroundColor: "#e8601a",
        }}
      >
        <FolderPlus className="h-4 w-4" />
        Create a Project
      </Link>
    </div>
  );
}
```

**Step 2: Commit**

```bash
git add web/components/dashboard/empty-state.tsx
git commit -m "feat(dashboard): add empty state component for zero-project state"
```

---

### Task 5: Rewrite Dashboard Page

Wire everything together in the main page.

**Files:**
- Modify: `web/app/page.tsx`

**Step 1: Rewrite page.tsx**

```tsx
// web/app/page.tsx
"use client";

import useSWR from "swr";
import { StatsCards } from "@/components/dashboard/stats-cards";
import { UnifiedFeed } from "@/components/dashboard/unified-feed";
import { DashboardEmptyState } from "@/components/dashboard/empty-state";
import { projects as projectsApi } from "@/lib/api/client";

export default function DashboardPage() {
  const { data: projectsData, isLoading } = useSWR("projects-for-dashboard", () =>
    projectsApi.list()
  );

  const hasProjects = !isLoading && projectsData?.data && projectsData.data.length > 0;

  return (
    <div className="space-y-6">
      <h1
        style={{
          fontFamily: "var(--font-fraunces)",
          fontSize: "24px",
          fontWeight: 700,
          color: "#111113",
        }}
      >
        Dashboard
      </h1>

      {hasProjects ? (
        <>
          <StatsCards />
          <UnifiedFeed />
        </>
      ) : isLoading ? (
        <div
          className="py-16 text-center"
          style={{
            fontFamily: "var(--font-dm-sans)",
            fontSize: "13px",
            color: "#6b7280",
          }}
        >
          Loading...
        </div>
      ) : (
        <DashboardEmptyState />
      )}
    </div>
  );
}
```

**Step 2: Verify in dev mode**

Run: `cd web && npm run dev`
Navigate to `/`. Confirm:
- With projects: 3 stat cards + unified feed renders
- Without projects: empty state with CTA renders

**Step 3: Commit**

```bash
git add web/app/page.tsx
git commit -m "feat(dashboard): wire up unified timeline dashboard page"
```

---

### Task 6: Clean Up Old Components

Remove the old `recent-releases.tsx` component (no longer imported). Keep `activity-feed.tsx` as it serves SSE live events (not used on dashboard currently but may be useful).

**Files:**
- Delete: `web/components/dashboard/recent-releases.tsx`

**Step 1: Verify no other imports**

Search for `recent-releases` imports across the codebase. Only `app/page.tsx` imported it, and that import is now removed.

**Step 2: Delete the file**

```bash
rm web/components/dashboard/recent-releases.tsx
```

**Step 3: Run build to verify nothing breaks**

Run: `cd web && npx next build`
Expected: Build succeeds with no errors.

**Step 4: Commit**

```bash
git add -u web/components/dashboard/recent-releases.tsx
git commit -m "refactor(dashboard): remove old recent-releases component"
```

---

### Task 7: Final Verification

**Step 1: Run full Go tests**

Run: `go test ./...`
Expected: All pass.

**Step 2: Run frontend build**

Run: `cd web && npx next build`
Expected: Build succeeds.

**Step 3: Manual visual check**

Run: `make dev` (starts Postgres + server)
Navigate to `http://localhost:8080`
Confirm:
- 3 stat cards show correct numbers
- Unified feed shows mixed releases + semantic releases in chronological order
- Semantic releases have orange left border + AI sparkle icon + summary text
- Raw releases show provider icon + repository + version chip
- "Needs Attention" card highlights in orange accent when count > 0
- Empty state shows when no projects exist

**Step 4: Commit final state (if any tweaks needed)**

```bash
git add -A
git commit -m "feat(dashboard): complete unified timeline dashboard redesign"
```
