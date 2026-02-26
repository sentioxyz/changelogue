# Frontend Redesign Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Fully redesign the ReleaseBeacon frontend from shadcn defaults to a distinctive editorial / data-journalism aesthetic with a dark charcoal sidebar and warm white content area.

**Architecture:** Pure visual redesign — no API changes, no routing changes, no data model changes. We rewrite CSS tokens, fonts, and component markup to match the approved design spec in `docs/plans/2026-02-25-frontend-redesign-design.md`. All existing shadcn/ui primitives are kept; we restyle them with the new token system.

**Tech Stack:** Next.js 16 (static export), React 19, Tailwind CSS 4, shadcn/ui, `next/font/google` (Fraunces + DM Sans), `@fontsource/jetbrains-mono`, Lucide React icons.

---

## Design Reference Quick-Look

All design decisions come from `docs/plans/2026-02-25-frontend-redesign-design.md`. Key tokens:

| Token | Value | Usage |
|-------|-------|-------|
| `--bg` | `#fafaf9` | Main content background |
| `--surface` | `#ffffff` | Cards, panels, header |
| `--sidebar` | `#16181c` | Sidebar background |
| `--accent` | `#e8601a` | Active indicators, links, primary buttons |
| `--border` | `#e8e8e5` | All dividers |
| `--mono-bg` | `#f3f3f1` | Version/repo chips |
| `--text-primary` | `#111113` | Headings, body |
| `--text-secondary` | `#6b7280` | Metadata |
| `--text-muted` | `#9ca3af` | Labels, inactive nav |

Fonts: **Fraunces** (display serif, 600–700) / **DM Sans** (body, 400–500) / **JetBrains Mono** (mono, 400).

Nav items (5, no collapse): Dashboard · Projects · Releases · Channels · Subscriptions.

---

## Task 1: Install JetBrains Mono Package

**Files:**
- Modify: `web/package.json` (indirectly, via npm install)

**Step 1: Install the npm package**

```bash
cd web && npm install @fontsource/jetbrains-mono
```

**Step 2: Verify installation**

Run: `grep "jetbrains-mono" web/package.json`
Expected: `"@fontsource/jetbrains-mono": "^x.x.x"` appears in `dependencies`.

**Step 3: Commit**

```bash
cd web && git add package.json package-lock.json
git commit -m "feat(web): add @fontsource/jetbrains-mono"
```

---

## Task 2: Global Design Tokens and Typography

**Files:**
- Modify: `web/app/globals.css`
- Modify: `web/app/layout.tsx`

### Step 1: Rewrite `globals.css`

Replace the entire file with the following. This keeps Tailwind 4's `@theme inline` structure, updates the shadcn variable values to match the new design, and adds new design-specific CSS custom properties.

```css
@import "tailwindcss";
@import "tw-animate-css";
@import "shadcn/tailwind.css";
@import "@fontsource/jetbrains-mono/400.css";

@custom-variant dark (&:is(.dark *));

@theme inline {
  --radius-sm: calc(var(--radius) - 4px);
  --radius-md: calc(var(--radius) - 2px);
  --radius-lg: var(--radius);
  --radius-xl: calc(var(--radius) + 4px);

  --color-background: var(--background);
  --color-foreground: var(--foreground);
  --color-card: var(--card);
  --color-card-foreground: var(--card-foreground);
  --color-primary: var(--primary);
  --color-primary-foreground: var(--primary-foreground);
  --color-secondary: var(--secondary);
  --color-secondary-foreground: var(--secondary-foreground);
  --color-muted: var(--muted);
  --color-muted-foreground: var(--muted-foreground);
  --color-accent: var(--accent);
  --color-accent-foreground: var(--accent-foreground);
  --color-destructive: #dc2626;
  --color-border: var(--border);
  --color-input: var(--input);
  --color-ring: var(--ring);

  /* Design-system extensions */
  --color-sidebar: var(--sidebar-bg);
  --color-sidebar-text: var(--sidebar-text);
  --color-beacon-accent: var(--beacon-accent);
  --color-mono-bg: var(--mono-bg);
  --color-text-secondary: var(--text-secondary);
  --color-text-muted: var(--text-muted);
  --color-surface: var(--surface);
}

:root {
  --radius: 0.375rem; /* 6px */

  /* Core layout colors */
  --background: #fafaf9;
  --foreground: #111113;
  --surface: #ffffff;
  --sidebar-bg: #16181c;
  --sidebar-text: #9ca3af;
  --beacon-accent: #e8601a;
  --border: #e8e8e5;
  --mono-bg: #f3f3f1;
  --text-secondary: #6b7280;
  --text-muted: #9ca3af;

  /* shadcn variable mappings */
  --card: #ffffff;
  --card-foreground: #111113;
  --primary: #e8601a;
  --primary-foreground: #ffffff;
  --secondary: #f3f3f1;
  --secondary-foreground: #374151;
  --muted: #f3f3f1;
  --muted-foreground: #6b7280;
  --accent: #f3f3f1;
  --accent-foreground: #111113;
  --input: #e8e8e5;
  --ring: #e8601a;

  /* Status colors */
  --status-completed: #16a34a;
  --status-running: #2563eb;
  --status-pending: #d97706;
  --status-failed: #dc2626;
}

@layer base {
  * {
    @apply border-border outline-ring/50;
  }
  body {
    @apply bg-background text-foreground;
    font-family: var(--font-dm-sans), system-ui, sans-serif;
  }
}

/* Motion primitives */
@layer utilities {
  .fade-in {
    animation: fadeIn 150ms ease both;
  }
  @keyframes fadeIn {
    from { opacity: 0; }
    to { opacity: 1; }
  }
}
```

### Step 2: Rewrite `layout.tsx`

Replace Geist fonts with Fraunces + DM Sans. JetBrains Mono is loaded via fontsource CSS import in globals.css.

```tsx
import type { Metadata } from "next";
import { Fraunces, DM_Sans } from "next/font/google";
import { Sidebar } from "@/components/layout/sidebar";
import { Header } from "@/components/layout/header";
import "./globals.css";

const fraunces = Fraunces({
  variable: "--font-fraunces",
  subsets: ["latin"],
  axes: ["SOFT", "WONK"],
  display: "swap",
});

const dmSans = DM_Sans({
  variable: "--font-dm-sans",
  subsets: ["latin"],
  display: "swap",
});

export const metadata: Metadata = {
  title: "ReleaseBeacon",
  description: "Agent-driven release intelligence platform",
};

export default function RootLayout({
  children,
}: Readonly<{
  children: React.ReactNode;
}>) {
  return (
    <html lang="en">
      <body className={`${fraunces.variable} ${dmSans.variable} antialiased`}>
        <div className="flex h-screen">
          <Sidebar />
          <div className="flex flex-1 flex-col overflow-hidden">
            <Header />
            <main className="flex-1 overflow-y-auto p-6 fade-in">{children}</main>
          </div>
        </div>
      </body>
    </html>
  );
}
```

### Step 3: Type-check

Run: `cd web && npx tsc --noEmit`
Expected: No errors.

### Step 4: Build check

Run: `cd web && npm run build`
Expected: Build succeeds.

### Step 5: Commit

```bash
cd web && git add app/globals.css app/layout.tsx
git commit -m "feat(web): redesign — global design tokens, Fraunces + DM Sans fonts"
```

---

## Task 3: Shared Micro-Components

**Files:**
- Create: `web/components/ui/provider-badge.tsx`
- Create: `web/components/ui/status-dot.tsx`
- Create: `web/components/ui/version-chip.tsx`
- Create: `web/components/ui/section-label.tsx`
- Create: `web/components/ui/urgency-callout.tsx`

These five primitives will be used throughout every redesigned page.

### Step 1: Create `provider-badge.tsx`

```tsx
// web/components/ui/provider-badge.tsx
import { cn } from "@/lib/utils";

const PROVIDER_STYLES: Record<string, { bg: string; text: string; label: string }> = {
  github: { bg: "#1a1a1a", text: "#ffffff", label: "GitHub" },
  dockerhub: { bg: "#2496ed", text: "#ffffff", label: "Docker Hub" },
};

interface ProviderBadgeProps {
  provider: string;
  className?: string;
}

export function ProviderBadge({ provider, className }: ProviderBadgeProps) {
  const style = PROVIDER_STYLES[provider.toLowerCase()] ?? {
    bg: "#6b7280",
    text: "#ffffff",
    label: provider,
  };
  return (
    <span
      className={cn("inline-flex items-center rounded-full px-2 py-0.5 text-[11px] font-medium leading-none", className)}
      style={{ backgroundColor: style.bg, color: style.text }}
    >
      {style.label}
    </span>
  );
}
```

### Step 2: Create `status-dot.tsx`

```tsx
// web/components/ui/status-dot.tsx
import { cn } from "@/lib/utils";

const STATUS_COLORS: Record<string, string> = {
  completed: "#16a34a",
  running: "#2563eb",
  pending: "#d97706",
  failed: "#dc2626",
};

interface StatusDotProps {
  status: string;
  className?: string;
}

export function StatusDot({ status, className }: StatusDotProps) {
  const color = STATUS_COLORS[status.toLowerCase()] ?? "#6b7280";
  return (
    <span
      className={cn("inline-block h-2 w-2 rounded-full shrink-0", className)}
      style={{ backgroundColor: color }}
      title={status}
    />
  );
}
```

### Step 3: Create `version-chip.tsx`

```tsx
// web/components/ui/version-chip.tsx
import { cn } from "@/lib/utils";

interface VersionChipProps {
  version: string;
  className?: string;
}

export function VersionChip({ version, className }: VersionChipProps) {
  return (
    <span
      className={cn(
        "inline-flex items-center rounded px-1.5 py-0.5 text-[12px] leading-none text-[#374151]",
        className
      )}
      style={{ backgroundColor: "#f3f3f1", fontFamily: "'JetBrains Mono', monospace" }}
    >
      {version}
    </span>
  );
}
```

### Step 4: Create `section-label.tsx`

```tsx
// web/components/ui/section-label.tsx
import { cn } from "@/lib/utils";

interface SectionLabelProps {
  children: React.ReactNode;
  className?: string;
}

export function SectionLabel({ children, className }: SectionLabelProps) {
  return (
    <p
      className={cn("text-[11px] font-medium uppercase tracking-[0.12em] text-[#9ca3af]", className)}
    >
      {children}
    </p>
  );
}
```

### Step 5: Create `urgency-callout.tsx`

```tsx
// web/components/ui/urgency-callout.tsx
import { AlertTriangle, AlertCircle } from "lucide-react";
import { cn } from "@/lib/utils";

interface UrgencyCalloutProps {
  urgency: string;
  description?: string;
  className?: string;
}

export function UrgencyCallout({ urgency, description, className }: UrgencyCalloutProps) {
  const upper = urgency?.toUpperCase();
  if (upper !== "HIGH" && upper !== "CRITICAL") return null;

  const isCritical = upper === "CRITICAL";
  const bg = isCritical ? "#fff1f2" : "#fff8f0";
  const border = isCritical ? "#dc2626" : "#d97706";
  const Icon = isCritical ? AlertCircle : AlertTriangle;

  return (
    <div
      className={cn("rounded px-4 py-3 text-sm", className)}
      style={{ backgroundColor: bg, borderLeft: `3px solid ${border}` }}
    >
      <div className="flex items-start gap-2">
        <Icon className="h-4 w-4 mt-0.5 shrink-0" style={{ color: border }} />
        <div>
          <span className="font-semibold" style={{ color: border }}>
            {upper} URGENCY
          </span>
          {description && (
            <p className="mt-0.5 text-[#374151]">{description}</p>
          )}
        </div>
      </div>
    </div>
  );
}
```

### Step 6: Type-check and commit

Run: `cd web && npx tsc --noEmit`
Expected: No errors.

```bash
cd web && git add components/ui/provider-badge.tsx components/ui/status-dot.tsx \
  components/ui/version-chip.tsx components/ui/section-label.tsx \
  components/ui/urgency-callout.tsx
git commit -m "feat(web): redesign — shared micro-components (badge, status, version, label, urgency)"
```

---

## Task 4: Sidebar Redesign

**Files:**
- Modify: `web/components/layout/sidebar.tsx`

**Key changes vs current:**
- Remove collapse toggle (fixed width, always visible)
- Remove "Sources" top-level nav item (now nested under Projects)
- Dark `#16181c` background (no border — contrast is the boundary)
- Active state: 3px left orange border + white text + subtle white bg
- Logo: "ReleaseBeacon" in Fraunces italic + filled circle in accent color

### Step 1: Rewrite `sidebar.tsx`

```tsx
// web/components/layout/sidebar.tsx
"use client";

import Link from "next/link";
import { usePathname } from "next/navigation";
import {
  LayoutDashboard,
  FolderKanban,
  Package,
  Bell,
  Megaphone,
} from "lucide-react";
import { cn } from "@/lib/utils";

const navItems = [
  { href: "/", label: "Dashboard", icon: LayoutDashboard },
  { href: "/projects", label: "Projects", icon: FolderKanban },
  { href: "/releases", label: "Releases", icon: Package },
  { href: "/channels", label: "Channels", icon: Megaphone },
  { href: "/subscriptions", label: "Subscriptions", icon: Bell },
];

export function Sidebar() {
  const pathname = usePathname();

  return (
    <aside
      className="flex w-[200px] shrink-0 flex-col"
      style={{ backgroundColor: "#16181c" }}
    >
      {/* Logo */}
      <div className="flex h-12 items-center gap-2 px-4">
        <span
          className="h-2.5 w-2.5 rounded-full shrink-0"
          style={{ backgroundColor: "#e8601a" }}
        />
        <Link
          href="/"
          className="text-[16px] italic text-white"
          style={{ fontFamily: "var(--font-fraunces)" }}
        >
          ReleaseBeacon
        </Link>
      </div>

      {/* Nav */}
      <nav className="flex-1 px-0 pt-2">
        {navItems.map((item) => {
          const isActive =
            item.href === "/"
              ? pathname === "/"
              : pathname.startsWith(item.href);
          return (
            <Link
              key={item.href}
              href={item.href}
              className={cn(
                "flex items-center gap-3 py-2 pl-4 pr-3 text-[13px] transition-colors duration-150",
                isActive
                  ? "text-white"
                  : "text-[#9ca3af] hover:text-white"
              )}
              style={
                isActive
                  ? {
                      borderLeft: "3px solid #e8601a",
                      backgroundColor: "rgba(255,255,255,0.06)",
                      paddingLeft: "13px", /* 16px - 3px border */
                    }
                  : { borderLeft: "3px solid transparent" }
              }
            >
              <item.icon className="h-4 w-4 shrink-0" />
              <span style={{ fontFamily: "var(--font-dm-sans)" }}>{item.label}</span>
            </Link>
          );
        })}
      </nav>
    </aside>
  );
}
```

### Step 2: Type-check

Run: `cd web && npx tsc --noEmit`
Expected: No errors.

### Step 3: Commit

```bash
cd web && git add components/layout/sidebar.tsx
git commit -m "feat(web): redesign — sidebar (dark bg, accent border, 5 nav items)"
```

---

## Task 5: Header Redesign

**Files:**
- Modify: `web/components/layout/header.tsx`

**Key changes vs current:**
- White background, `1px solid #e8e8e5` bottom border, 48px height
- Left: breadcrumb for nested pages (e.g. `Projects / Geth / Agent`)
- Right: `children` slot for contextual action button (each page passes it in)
- Pages pass their action button via context or a layout pattern — use a simple `data-header-action` portal approach... actually, given the static export constraint, the cleanest approach is to keep Header as-is for title and let each page page render its own action button at page level. The header just shows the breadcrumb.

**Design note:** The header title comes from the URL. For nested pages like `/projects/[id]`, the header shows a breadcrumb. Since the header can't know project names without extra API calls, we'll show segment-based breadcrumbs (e.g. "Projects / Detail") and let full names appear in the page header zone.

### Step 1: Rewrite `header.tsx`

```tsx
// web/components/layout/header.tsx
"use client";

import { usePathname } from "next/navigation";

const SEGMENT_LABELS: Record<string, string> = {
  "": "Dashboard",
  projects: "Projects",
  releases: "Releases",
  sources: "Sources",
  subscriptions: "Subscriptions",
  channels: "Channels",
  agent: "Agent",
  "semantic-releases": "Semantic Releases",
  "context-sources": "Context Sources",
  new: "New",
  edit: "Edit",
};

function segmentLabel(seg: string): string {
  return SEGMENT_LABELS[seg] ?? seg;
}

export function Header() {
  const pathname = usePathname();
  const segments = pathname.split("/").filter(Boolean);

  // Build breadcrumb skipping dynamic ID segments (UUIDs)
  const uuidRe = /^[0-9a-f-]{8,}$/i;
  const breadcrumbs = segments
    .filter((s) => !uuidRe.test(s))
    .map(segmentLabel);

  const display =
    breadcrumbs.length === 0
      ? "Dashboard"
      : breadcrumbs.join(" / ");

  return (
    <header
      className="flex h-12 items-center px-6"
      style={{ borderBottom: "1px solid #e8e8e5", backgroundColor: "#ffffff" }}
    >
      <p
        className="text-[14px] font-medium text-[#111113]"
        style={{ fontFamily: "var(--font-dm-sans)" }}
      >
        {display}
      </p>
    </header>
  );
}
```

### Step 2: Type-check and commit

Run: `cd web && npx tsc --noEmit`

```bash
cd web && git add components/layout/header.tsx
git commit -m "feat(web): redesign — header (48px, breadcrumb, warm white)"
```

---

## Task 6: Dashboard Redesign

**Files:**
- Modify: `web/app/page.tsx`
- Modify: `web/components/dashboard/stats-cards.tsx`
- Modify: `web/components/dashboard/recent-releases.tsx`
- Delete (effectively supersede): `web/components/dashboard/activity-feed.tsx` (design removes activity feed)

**Layout:** Stat strip (4 cards) on top, then two-column: Recent Source Releases (left) + Recent Semantic Releases (right).

### Step 1: Rewrite `stats-cards.tsx`

Read current file first, then replace with:

```tsx
// web/components/dashboard/stats-cards.tsx
"use client";

import { Package, Radio, Clock, AlertCircle } from "lucide-react";
import useSWR from "swr";
import { system } from "@/lib/api/client";

interface Stats {
  total_releases: number;
  active_sources: number;
  pending_jobs: number;
  failed_jobs: number;
}

interface StatCardProps {
  label: string;
  value: number | undefined;
  icon: React.ElementType;
}

function StatCard({ label, value, icon: Icon }: StatCardProps) {
  return (
    <div
      className="flex flex-col gap-3 rounded-md p-5"
      style={{ backgroundColor: "#ffffff", border: "1px solid #e8e8e5" }}
    >
      <div className="flex items-start justify-between">
        <p
          className="text-[12px] uppercase tracking-[0.08em] text-[#6b7280]"
          style={{ fontFamily: "var(--font-dm-sans)" }}
        >
          {label}
        </p>
        <Icon className="h-4 w-4 text-[#9ca3af]" />
      </div>
      <p
        className="text-[32px] font-semibold leading-none text-[#111113]"
        style={{ fontFamily: "var(--font-fraunces)" }}
      >
        {value ?? "—"}
      </p>
    </div>
  );
}

export function StatsCards() {
  const { data } = useSWR("stats", () => system.stats());

  const stats: Stats = data ?? {
    total_releases: undefined as unknown as number,
    active_sources: undefined as unknown as number,
    pending_jobs: undefined as unknown as number,
    failed_jobs: undefined as unknown as number,
  };

  return (
    <div className="grid grid-cols-4 gap-4">
      <StatCard label="Total Releases" value={stats.total_releases} icon={Package} />
      <StatCard label="Active Sources" value={stats.active_sources} icon={Radio} />
      <StatCard label="Pending Jobs" value={stats.pending_jobs} icon={Clock} />
      <StatCard label="Failed Jobs" value={stats.failed_jobs} icon={AlertCircle} />
    </div>
  );
}
```

### Step 2: Rewrite `recent-releases.tsx`

```tsx
// web/components/dashboard/recent-releases.tsx
"use client";

import Link from "next/link";
import useSWR from "swr";
import { releases, projects } from "@/lib/api/client";
import { VersionChip } from "@/components/ui/version-chip";

function timeAgo(dateStr: string): string {
  const diff = Date.now() - new Date(dateStr).getTime();
  const mins = Math.floor(diff / 60000);
  if (mins < 60) return `${mins}m ago`;
  const hrs = Math.floor(mins / 60);
  if (hrs < 24) return `${hrs}h ago`;
  return `${Math.floor(hrs / 24)}d ago`;
}

export function RecentReleases() {
  const { data: projectsData } = useSWR("projects", () => projects.list());
  const firstProjectId = projectsData?.data?.[0]?.id;
  const { data: releasesData } = useSWR(
    firstProjectId ? `releases-recent-${firstProjectId}` : null,
    () => releases.listByProject(firstProjectId!, { limit: 8 })
  );

  const items = releasesData?.data ?? [];

  return (
    <div
      className="rounded-md overflow-hidden"
      style={{ border: "1px solid #e8e8e5", backgroundColor: "#ffffff" }}
    >
      <div
        className="flex items-center justify-between px-4 py-3"
        style={{ borderBottom: "1px solid #e8e8e5" }}
      >
        <p
          className="text-[13px] font-medium text-[#111113]"
          style={{ fontFamily: "var(--font-dm-sans)" }}
        >
          Recent Source Releases
        </p>
        <Link
          href="/releases"
          className="text-[12px] transition-colors duration-100 hover:opacity-70"
          style={{ color: "#e8601a", fontFamily: "var(--font-dm-sans)" }}
        >
          View all →
        </Link>
      </div>
      {items.length === 0 ? (
        <p
          className="px-4 py-6 text-center text-[14px] italic text-[#9ca3af]"
          style={{ fontFamily: "var(--font-fraunces)" }}
        >
          No releases yet
        </p>
      ) : (
        <table className="w-full">
          <tbody>
            {items.map((r, i) => (
              <tr
                key={r.id}
                className="transition-colors duration-100 hover:bg-[#fafaf9]"
                style={i > 0 ? { borderTop: "1px solid #e8e8e5" } : undefined}
              >
                <td
                  className="px-4 py-2.5 text-[13px] text-[#6b7280]"
                  style={{ fontFamily: "'JetBrains Mono', monospace" }}
                >
                  {r.repository ?? r.source_id}
                </td>
                <td className="px-4 py-2.5">
                  <Link href={`/releases/${r.id}`}>
                    <VersionChip version={r.version} />
                  </Link>
                </td>
                <td
                  className="px-4 py-2.5 text-right text-[12px] text-[#9ca3af]"
                  style={{ fontFamily: "var(--font-dm-sans)" }}
                >
                  {r.released_at ? timeAgo(r.released_at) : "—"}
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      )}
    </div>
  );
}
```

### Step 3: Rewrite `app/page.tsx`

```tsx
// web/app/page.tsx
"use client";

import Link from "next/link";
import useSWR from "swr";
import { projects, semanticReleases as srApi } from "@/lib/api/client";
import { StatsCards } from "@/components/dashboard/stats-cards";
import { RecentReleases } from "@/components/dashboard/recent-releases";
import { StatusDot } from "@/components/ui/status-dot";
import { VersionChip } from "@/components/ui/version-chip";

function timeAgo(dateStr: string): string {
  const diff = Date.now() - new Date(dateStr).getTime();
  const mins = Math.floor(diff / 60000);
  if (mins < 60) return `${mins}m ago`;
  const hrs = Math.floor(mins / 60);
  if (hrs < 24) return `${hrs}h ago`;
  return `${Math.floor(hrs / 24)}d ago`;
}

export default function DashboardPage() {
  const { data: projectsData } = useSWR("projects", () => projects.list());
  const firstProjectId = projectsData?.data?.[0]?.id;
  const { data: srData } = useSWR(
    firstProjectId ? `sr-dashboard-${firstProjectId}` : null,
    () => srApi.list(firstProjectId!, { limit: 6 })
  );

  const semanticItems = srData?.data ?? [];

  return (
    <div className="flex flex-col gap-6">
      <StatsCards />

      <div className="grid grid-cols-2 gap-4">
        <RecentReleases />

        {/* Semantic Releases column */}
        <div
          className="rounded-md overflow-hidden"
          style={{ border: "1px solid #e8e8e5", backgroundColor: "#ffffff" }}
        >
          <div
            className="flex items-center justify-between px-4 py-3"
            style={{ borderBottom: "1px solid #e8e8e5" }}
          >
            <p
              className="text-[13px] font-medium text-[#111113]"
              style={{ fontFamily: "var(--font-dm-sans)" }}
            >
              Semantic Releases
            </p>
            {firstProjectId && (
              <Link
                href={`/projects/${firstProjectId}/semantic-releases`}
                className="text-[12px] transition-colors duration-100 hover:opacity-70"
                style={{ color: "#e8601a", fontFamily: "var(--font-dm-sans)" }}
              >
                View all →
              </Link>
            )}
          </div>
          {semanticItems.length === 0 ? (
            <p
              className="px-4 py-6 text-center text-[14px] italic text-[#9ca3af]"
              style={{ fontFamily: "var(--font-fraunces)" }}
            >
              No semantic releases yet
            </p>
          ) : (
            <div className="divide-y" style={{ borderColor: "#e8e8e5" }}>
              {semanticItems.map((sr) => (
                <Link
                  key={sr.id}
                  href={`/projects/${sr.project_id}/semantic-releases/${sr.id}`}
                  className="flex flex-col gap-1 px-4 py-3 transition-colors duration-100 hover:bg-[#fafaf9]"
                >
                  <div className="flex items-center gap-2">
                    <p
                      className="text-[15px] font-semibold text-[#111113]"
                      style={{ fontFamily: "var(--font-fraunces)" }}
                    >
                      {sr.project_id}
                    </p>
                    <VersionChip version={sr.version} />
                  </div>
                  {sr.report?.summary && (
                    <p
                      className="line-clamp-1 text-[13px] italic text-[#6b7280]"
                      style={{ fontFamily: "var(--font-dm-sans)" }}
                    >
                      {sr.report.summary}
                    </p>
                  )}
                  <div className="flex items-center gap-1.5">
                    <StatusDot status={sr.status} />
                    <span
                      className="text-[12px] text-[#9ca3af]"
                      style={{ fontFamily: "var(--font-dm-sans)" }}
                    >
                      {sr.status} · {timeAgo(sr.created_at)}
                    </span>
                  </div>
                </Link>
              ))}
            </div>
          )}
        </div>
      </div>
    </div>
  );
}
```

### Step 4: Type-check and build

Run: `cd web && npx tsc --noEmit`
Run: `cd web && npm run build`
Expected: Both succeed.

### Step 5: Commit

```bash
cd web && git add app/page.tsx components/dashboard/stats-cards.tsx \
  components/dashboard/recent-releases.tsx
git commit -m "feat(web): redesign — dashboard (stat strip, two-column layout)"
```

---

## Task 7: Project Detail Redesign

**Files:**
- Modify: `web/components/projects/project-detail.tsx`

**Key changes:**
- 80px white header zone (project name in Fraunces 28px, meta line, Run Agent button)
- 4 tabs: Sources · Context Sources · Semantic Releases · Agent
- Remove "Overview" tab (agent config moves into Agent tab)
- Sources tab: provider badge, repo (mono), poll interval, status dot, last polled, kebab actions
- Context Sources tab: type badge, name, config URL truncated, kebab actions
- Semantic Releases tab: card list with version chip, status dot, urgency chip, excerpt
- Agent tab: Prompt textarea + Rules checkboxes + Run History table

### Step 1: Read the current file

Read: `web/components/projects/project-detail.tsx` (to understand current API usage and data shapes)

### Step 2: Rewrite `project-detail.tsx`

This is the largest single component. Replace with the following complete implementation:

```tsx
// web/components/projects/project-detail.tsx
"use client";

import { useState } from "react";
import Link from "next/link";
import { useRouter } from "next/navigation";
import useSWR from "swr";
import {
  MoreHorizontal, Plus, Play, Trash2, Pencil,
} from "lucide-react";
import { projects, sources, contextSources, semanticReleases as srApi, agent } from "@/lib/api/client";
import { ProviderBadge } from "@/components/ui/provider-badge";
import { StatusDot } from "@/components/ui/status-dot";
import { VersionChip } from "@/components/ui/version-chip";
import { SectionLabel } from "@/components/ui/section-label";
import { UrgencyCallout } from "@/components/ui/urgency-callout";

// ---- helpers ----

function timeAgo(dateStr?: string | null): string {
  if (!dateStr) return "never";
  const diff = Date.now() - new Date(dateStr).getTime();
  const mins = Math.floor(diff / 60000);
  if (mins < 60) return `${mins}m ago`;
  const hrs = Math.floor(mins / 60);
  if (hrs < 24) return `${hrs}h ago`;
  return `${Math.floor(hrs / 24)}d ago`;
}

function duration(start?: string | null, end?: string | null): string {
  if (!start || !end) return "—";
  const ms = new Date(end).getTime() - new Date(start).getTime();
  if (ms < 1000) return `${ms}ms`;
  return `${(ms / 1000).toFixed(1)}s`;
}

// ---- tab types ----

type TabId = "sources" | "context-sources" | "semantic-releases" | "agent";

const TABS: { id: TabId; label: string }[] = [
  { id: "sources", label: "Sources" },
  { id: "context-sources", label: "Context Sources" },
  { id: "semantic-releases", label: "Semantic Releases" },
  { id: "agent", label: "Agent" },
];

// ---- sub-components ----

function TabBar({ active, onChange }: { active: TabId; onChange: (t: TabId) => void }) {
  return (
    <div
      className="flex gap-0 px-6"
      style={{ borderBottom: "1px solid #e8e8e5" }}
    >
      {TABS.map((t) => (
        <button
          key={t.id}
          onClick={() => onChange(t.id)}
          className="py-3 pr-5 text-[13px] transition-colors duration-100"
          style={{
            fontFamily: "var(--font-dm-sans)",
            color: active === t.id ? "#111113" : "#9ca3af",
            borderBottom: active === t.id ? "2px solid #e8601a" : "2px solid transparent",
            fontWeight: active === t.id ? 500 : 400,
            marginBottom: "-1px",
          }}
        >
          {t.label}
        </button>
      ))}
    </div>
  );
}

function TableContainer({ children }: { children: React.ReactNode }) {
  return (
    <div className="overflow-hidden" style={{ border: "1px solid #e8e8e5", borderRadius: "6px" }}>
      <table className="w-full text-[13px]" style={{ fontFamily: "var(--font-dm-sans)" }}>
        {children}
      </table>
    </div>
  );
}

function Th({ children }: { children: React.ReactNode }) {
  return (
    <th
      className="px-4 py-2.5 text-left text-[11px] font-medium uppercase tracking-[0.08em] text-[#9ca3af]"
      style={{ borderBottom: "1px solid #e8e8e5", backgroundColor: "#fafaf9" }}
    >
      {children}
    </th>
  );
}

function Td({ children, mono }: { children: React.ReactNode; mono?: boolean }) {
  return (
    <td
      className="px-4 py-2.5 text-[#374151]"
      style={mono ? { fontFamily: "'JetBrains Mono', monospace", fontSize: "12px" } : undefined}
    >
      {children}
    </td>
  );
}

// ---- main component ----

interface ProjectDetailProps {
  projectId: string;
}

export function ProjectDetail({ projectId }: ProjectDetailProps) {
  const router = useRouter();
  const [tab, setTab] = useState<TabId>("sources");
  const [agentPrompt, setAgentPrompt] = useState<string>("");
  const [promptSaved, setPromptSaved] = useState(false);

  const { data: project } = useSWR(`project-${projectId}`, () =>
    projects.get(projectId)
  );
  const { data: sourcesData, mutate: mutateSources } = useSWR(
    `sources-${projectId}`,
    () => sources.listByProject(projectId)
  );
  const { data: ctxData, mutate: mutateCtx } = useSWR(
    `ctx-${projectId}`,
    () => contextSources.list(projectId)
  );
  const { data: srData } = useSWR(`sr-${projectId}`, () =>
    srApi.list(projectId, { limit: 20 })
  );
  const { data: runsData } = useSWR(`runs-${projectId}`, () =>
    agent.listRuns(projectId, { limit: 20 })
  );

  const proj = project?.data;
  const srcList = sourcesData?.data ?? [];
  const ctxList = ctxData?.data ?? [];
  const srList = srData?.data ?? [];
  const runList = runsData?.data ?? [];

  // Initialise prompt from project data
  if (proj?.agent_prompt && agentPrompt === "") {
    setAgentPrompt(proj.agent_prompt);
  }

  async function handleDelete() {
    if (!confirm(`Delete project "${proj?.name}"?`)) return;
    await projects.delete(projectId);
    router.push("/projects");
  }

  async function handleTriggerRun() {
    await agent.triggerRun(projectId);
  }

  async function handleSavePrompt() {
    await projects.update(projectId, { agent_prompt: agentPrompt });
    setPromptSaved(true);
    setTimeout(() => setPromptSaved(false), 2000);
  }

  async function handleDeleteSource(srcId: string) {
    if (!confirm("Delete this source?")) return;
    await sources.delete(srcId);
    mutateSources();
  }

  async function handleDeleteCtx(ctxId: string) {
    if (!confirm("Delete this context source?")) return;
    await contextSources.delete(projectId, ctxId);
    mutateCtx();
  }

  if (!proj) {
    return (
      <div className="flex h-40 items-center justify-center">
        <p className="text-[14px] italic text-[#9ca3af]" style={{ fontFamily: "var(--font-fraunces)" }}>
          Loading…
        </p>
      </div>
    );
  }

  return (
    <div className="flex flex-col" style={{ backgroundColor: "#ffffff", border: "1px solid #e8e8e5", borderRadius: "6px", overflow: "hidden" }}>
      {/* Header zone */}
      <div
        className="px-6 py-5"
        style={{ borderBottom: "1px solid #e8e8e5" }}
      >
        <div className="flex items-start justify-between">
          <div className="flex flex-col gap-1">
            <h1
              className="text-[28px] font-semibold leading-tight text-[#111113]"
              style={{ fontFamily: "var(--font-fraunces)" }}
            >
              {proj.name}
            </h1>
            {proj.description && (
              <p className="text-[14px] text-[#6b7280]" style={{ fontFamily: "var(--font-dm-sans)" }}>
                {proj.description}
              </p>
            )}
            <p
              className="mt-1 text-[12px] text-[#9ca3af]"
              style={{ fontFamily: "var(--font-dm-sans)" }}
            >
              {srcList.length} source{srcList.length !== 1 ? "s" : ""} ·{" "}
              {ctxList.length} context source{ctxList.length !== 1 ? "s" : ""}
            </p>
          </div>
          <div className="flex items-center gap-2">
            <button
              onClick={() => router.push(`/projects/${projectId}/edit`)}
              className="flex items-center gap-1.5 rounded px-3 py-1.5 text-[13px] transition-colors duration-120 hover:bg-[#f3f3f1]"
              style={{ border: "1px solid #e8e8e5", fontFamily: "var(--font-dm-sans)", color: "#374151" }}
            >
              <Pencil className="h-3.5 w-3.5" />
              Edit
            </button>
            <button
              onClick={handleDelete}
              className="flex items-center gap-1.5 rounded px-3 py-1.5 text-[13px] transition-colors duration-120"
              style={{ border: "1px solid #dc2626", color: "#dc2626", fontFamily: "var(--font-dm-sans)" }}
            >
              <Trash2 className="h-3.5 w-3.5" />
              Delete
            </button>
            <button
              onClick={handleTriggerRun}
              className="flex items-center gap-1.5 rounded px-3 py-1.5 text-[13px] text-white transition-colors duration-120 hover:opacity-90"
              style={{ backgroundColor: "#e8601a", fontFamily: "var(--font-dm-sans)" }}
            >
              <Play className="h-3.5 w-3.5" />
              Run Agent
            </button>
          </div>
        </div>
      </div>

      {/* Tabs */}
      <TabBar active={tab} onChange={setTab} />

      {/* Tab content */}
      <div className="p-6">
        {/* ---- SOURCES TAB ---- */}
        {tab === "sources" && (
          <div className="flex flex-col gap-4">
            <div className="flex justify-end">
              <Link
                href={`/projects/${projectId}/sources/new`}
                className="flex items-center gap-1.5 rounded px-3 py-1.5 text-[13px] text-white transition-opacity hover:opacity-90"
                style={{ backgroundColor: "#e8601a", fontFamily: "var(--font-dm-sans)" }}
              >
                <Plus className="h-3.5 w-3.5" />
                Add Source
              </Link>
            </div>
            {srcList.length === 0 ? (
              <p className="text-center text-[14px] italic text-[#9ca3af] py-8" style={{ fontFamily: "var(--font-fraunces)" }}>
                No sources yet — add one to start tracking releases
              </p>
            ) : (
              <TableContainer>
                <thead>
                  <tr>
                    <Th>Provider</Th>
                    <Th>Repository</Th>
                    <Th>Interval</Th>
                    <Th>Status</Th>
                    <Th>Last Polled</Th>
                    <Th></Th>
                  </tr>
                </thead>
                <tbody>
                  {srcList.map((src, i) => (
                    <tr
                      key={src.id}
                      className="transition-colors duration-100 hover:bg-[#fafaf9]"
                      style={i > 0 ? { borderTop: "1px solid #e8e8e5" } : undefined}
                    >
                      <Td><ProviderBadge provider={src.provider ?? src.source_type ?? ""} /></Td>
                      <Td mono>{src.repository}</Td>
                      <Td>{src.poll_interval_seconds}s</Td>
                      <Td>
                        <div className="flex items-center gap-1.5">
                          <StatusDot status={src.enabled ? "completed" : "pending"} />
                          <span>{src.enabled ? "active" : "disabled"}</span>
                        </div>
                      </Td>
                      <Td>{timeAgo(src.last_polled_at)}</Td>
                      <Td>
                        <button
                          onClick={() => handleDeleteSource(src.id)}
                          className="rounded p-1 text-[#9ca3af] transition-colors hover:text-[#dc2626]"
                        >
                          <MoreHorizontal className="h-4 w-4" />
                        </button>
                      </Td>
                    </tr>
                  ))}
                </tbody>
              </TableContainer>
            )}
          </div>
        )}

        {/* ---- CONTEXT SOURCES TAB ---- */}
        {tab === "context-sources" && (
          <div className="flex flex-col gap-4">
            <div className="flex justify-end">
              <Link
                href={`/projects/${projectId}/context-sources/new`}
                className="flex items-center gap-1.5 rounded px-3 py-1.5 text-[13px] text-white transition-opacity hover:opacity-90"
                style={{ backgroundColor: "#e8601a", fontFamily: "var(--font-dm-sans)" }}
              >
                <Plus className="h-3.5 w-3.5" />
                Add Context Source
              </Link>
            </div>
            {ctxList.length === 0 ? (
              <p className="text-center text-[14px] italic text-[#9ca3af] py-8" style={{ fontFamily: "var(--font-fraunces)" }}>
                No context sources yet
              </p>
            ) : (
              <TableContainer>
                <thead>
                  <tr>
                    <Th>Type</Th>
                    <Th>Name</Th>
                    <Th>Config</Th>
                    <Th></Th>
                  </tr>
                </thead>
                <tbody>
                  {ctxList.map((ctx, i) => (
                    <tr
                      key={ctx.id}
                      className="transition-colors duration-100 hover:bg-[#fafaf9]"
                      style={i > 0 ? { borderTop: "1px solid #e8e8e5" } : undefined}
                    >
                      <Td>
                        <span
                          className="rounded-full px-2 py-0.5 text-[11px]"
                          style={{ backgroundColor: "#f3f3f1", color: "#374151" }}
                        >
                          {ctx.source_type}
                        </span>
                      </Td>
                      <Td>{ctx.name}</Td>
                      <Td mono>
                        <span className="block max-w-[280px] truncate text-[#9ca3af]">
                          {ctx.config?.url ?? JSON.stringify(ctx.config)}
                        </span>
                      </Td>
                      <Td>
                        <button
                          onClick={() => handleDeleteCtx(ctx.id)}
                          className="rounded p-1 text-[#9ca3af] transition-colors hover:text-[#dc2626]"
                        >
                          <MoreHorizontal className="h-4 w-4" />
                        </button>
                      </Td>
                    </tr>
                  ))}
                </tbody>
              </TableContainer>
            )}
          </div>
        )}

        {/* ---- SEMANTIC RELEASES TAB ---- */}
        {tab === "semantic-releases" && (
          <div className="flex flex-col gap-3">
            {srList.length === 0 ? (
              <p className="text-center text-[14px] italic text-[#9ca3af] py-8" style={{ fontFamily: "var(--font-fraunces)" }}>
                No semantic releases yet
              </p>
            ) : (
              srList.map((sr) => (
                <Link
                  key={sr.id}
                  href={`/projects/${projectId}/semantic-releases/${sr.id}`}
                  className="flex items-center justify-between rounded-md px-4 py-3 transition-colors duration-100 hover:bg-[#fafaf9]"
                  style={{ border: "1px solid #e8e8e5" }}
                >
                  <div className="flex flex-col gap-1">
                    <div className="flex items-center gap-2">
                      <VersionChip version={sr.version} />
                      <StatusDot status={sr.status} />
                      <span className="text-[12px] text-[#9ca3af]" style={{ fontFamily: "var(--font-dm-sans)" }}>
                        {sr.status}
                      </span>
                      {sr.report?.urgency && sr.report.urgency !== "LOW" && (
                        <span
                          className="rounded-full px-2 py-0.5 text-[11px] font-medium"
                          style={{
                            backgroundColor: sr.report.urgency === "CRITICAL" ? "#fff1f2" : "#fff8f0",
                            color: sr.report.urgency === "CRITICAL" ? "#dc2626" : "#d97706",
                          }}
                        >
                          {sr.report.urgency}
                        </span>
                      )}
                    </div>
                    {sr.report?.summary && (
                      <p className="line-clamp-1 text-[13px] italic text-[#6b7280]" style={{ fontFamily: "var(--font-dm-sans)" }}>
                        {sr.report.summary}
                      </p>
                    )}
                  </div>
                  <span className="text-[12px] text-[#9ca3af]" style={{ fontFamily: "var(--font-dm-sans)" }}>
                    {timeAgo(sr.created_at)} →
                  </span>
                </Link>
              ))
            )}
          </div>
        )}

        {/* ---- AGENT TAB ---- */}
        {tab === "agent" && (
          <div className="flex flex-col gap-8">
            {/* Prompt */}
            <div className="flex flex-col gap-2">
              <SectionLabel>Agent Prompt</SectionLabel>
              <textarea
                value={agentPrompt}
                onChange={(e) => setAgentPrompt(e.target.value)}
                rows={6}
                className="w-full rounded-md p-3 text-[13px] text-[#374151] outline-none focus:ring-1 focus:ring-[#e8601a]"
                style={{
                  border: "1px solid #e8e8e5",
                  fontFamily: "var(--font-dm-sans)",
                  resize: "vertical",
                  backgroundColor: "#fafaf9",
                }}
                placeholder="Describe how the agent should analyze releases for this project…"
              />
              <div className="flex justify-end">
                <button
                  onClick={handleSavePrompt}
                  className="rounded px-3 py-1.5 text-[13px] text-white transition-opacity hover:opacity-90"
                  style={{ backgroundColor: "#e8601a", fontFamily: "var(--font-dm-sans)" }}
                >
                  {promptSaved ? "Saved ✓" : "Save"}
                </button>
              </div>
            </div>

            {/* Rules */}
            <div className="flex flex-col gap-3">
              <SectionLabel>Trigger Rules</SectionLabel>
              {[
                { key: "on_major_release", label: "On Major Release" },
                { key: "on_minor_release", label: "On Minor Release" },
                { key: "on_security_patch", label: "On Security Patch" },
              ].map(({ key, label }) => (
                <label key={key} className="flex items-center gap-2.5 text-[13px] text-[#374151] cursor-pointer" style={{ fontFamily: "var(--font-dm-sans)" }}>
                  <input
                    type="checkbox"
                    defaultChecked={(proj as Record<string, unknown>)[key] as boolean}
                    className="h-4 w-4 rounded accent-[#e8601a]"
                  />
                  {label}
                </label>
              ))}
              <div className="flex flex-col gap-1">
                <label className="text-[12px] text-[#9ca3af]" style={{ fontFamily: "var(--font-dm-sans)" }}>
                  Version Pattern (regex, optional)
                </label>
                <input
                  type="text"
                  defaultValue={(proj as Record<string, unknown>).version_pattern as string ?? ""}
                  placeholder="e.g. ^v\d+\.\d+\.0$"
                  className="rounded-md px-3 py-1.5 text-[13px] text-[#374151] outline-none focus:ring-1 focus:ring-[#e8601a]"
                  style={{
                    border: "1px solid #e8e8e5",
                    fontFamily: "'JetBrains Mono', monospace",
                    fontSize: "12px",
                    backgroundColor: "#fafaf9",
                    width: "320px",
                  }}
                />
              </div>
            </div>

            {/* Run History */}
            <div className="flex flex-col gap-3">
              <SectionLabel>Run History</SectionLabel>
              {runList.length === 0 ? (
                <p className="text-[14px] italic text-[#9ca3af]" style={{ fontFamily: "var(--font-fraunces)" }}>
                  No agent runs yet
                </p>
              ) : (
                <TableContainer>
                  <thead>
                    <tr>
                      <Th>Trigger</Th>
                      <Th>Status</Th>
                      <Th>Started</Th>
                      <Th>Duration</Th>
                      <Th>Semantic Release</Th>
                    </tr>
                  </thead>
                  <tbody>
                    {runList.map((run, i) => (
                      <tr
                        key={run.id}
                        className="transition-colors duration-100 hover:bg-[#fafaf9]"
                        style={i > 0 ? { borderTop: "1px solid #e8e8e5" } : undefined}
                      >
                        <Td>{run.trigger_reason ?? "manual"}</Td>
                        <Td>
                          <div className="flex items-center gap-1.5">
                            <StatusDot status={run.status} />
                            <span>{run.status}</span>
                          </div>
                        </Td>
                        <Td>{timeAgo(run.started_at)}</Td>
                        <Td>{duration(run.started_at, run.completed_at)}</Td>
                        <Td>
                          {run.semantic_release_id ? (
                            <Link
                              href={`/projects/${projectId}/semantic-releases/${run.semantic_release_id}`}
                              className="text-[#e8601a] hover:underline"
                              style={{ fontFamily: "'JetBrains Mono', monospace", fontSize: "12px" }}
                            >
                              View →
                            </Link>
                          ) : "—"}
                        </Td>
                      </tr>
                    ))}
                  </tbody>
                </TableContainer>
              )}
            </div>
          </div>
        )}
      </div>
    </div>
  );
}
```

**Note on field access:** If any field path doesn't match the actual API types (e.g., `src.provider` vs `src.source_type`, `run.trigger_reason`), adjust to match the types in `lib/api/types.ts`. The component above is written to the design spec; reconcile with live types.

### Step 3: Type-check

Run: `cd web && npx tsc --noEmit`
Fix any field-name mismatches against `lib/api/types.ts`.

### Step 4: Commit

```bash
cd web && git add components/projects/project-detail.tsx
git commit -m "feat(web): redesign — project detail (header zone, 4 tabs: sources/ctx/sr/agent)"
```

---

## Task 8: Semantic Release Detail Redesign

**Files:**
- Modify: `web/components/semantic-releases/semantic-release-detail.tsx`

**Design:** Single-column editorial layout, max-width 760px, centered.

### Step 1: Read the current file

Read: `web/components/semantic-releases/semantic-release-detail.tsx`

### Step 2: Rewrite `semantic-release-detail.tsx`

```tsx
// web/components/semantic-releases/semantic-release-detail.tsx
"use client";

import Link from "next/link";
import useSWR from "swr";
import { semanticReleases as srApi } from "@/lib/api/client";
import { StatusDot } from "@/components/ui/status-dot";
import { VersionChip } from "@/components/ui/version-chip";
import { SectionLabel } from "@/components/ui/section-label";
import { UrgencyCallout } from "@/components/ui/urgency-callout";
import { ProviderBadge } from "@/components/ui/provider-badge";

function timeAgo(dateStr?: string | null): string {
  if (!dateStr) return "—";
  const diff = Date.now() - new Date(dateStr).getTime();
  const mins = Math.floor(diff / 60000);
  if (mins < 60) return `${mins}m ago`;
  const hrs = Math.floor(mins / 60);
  if (hrs < 24) return `${hrs}h ago`;
  return `${Math.floor(hrs / 24)}d ago`;
}

interface SemanticReleaseDetailProps {
  projectId: string;
  srId: string;
}

export function SemanticReleaseDetail({ projectId, srId }: SemanticReleaseDetailProps) {
  const { data } = useSWR(`sr-${srId}`, () => srApi.get(projectId, srId));
  const sr = data?.data;

  if (!sr) {
    return (
      <div className="flex h-40 items-center justify-center">
        <p className="text-[14px] italic text-[#9ca3af]" style={{ fontFamily: "var(--font-fraunces)" }}>
          Loading…
        </p>
      </div>
    );
  }

  const report = sr.report;

  return (
    <div className="mx-auto max-w-[760px] fade-in">
      {/* Breadcrumb */}
      <p
        className="mb-4 text-[12px] text-[#9ca3af]"
        style={{ fontFamily: "var(--font-dm-sans)" }}
      >
        <Link href="/projects" className="hover:text-[#e8601a]">Projects</Link>
        {" / "}
        <Link href={`/projects/${projectId}`} className="hover:text-[#e8601a]">{projectId}</Link>
        {" / "}
        Semantic Releases
      </p>

      {/* Byline */}
      <p
        className="mb-1 text-[13px] text-[#9ca3af]"
        style={{ fontFamily: "var(--font-fraunces)", fontStyle: "italic" }}
      >
        {projectId}
      </p>

      {/* Version heading */}
      <h1
        className="mb-3 text-[42px] font-bold leading-none tracking-tight text-[#111113]"
        style={{ fontFamily: "var(--font-fraunces)" }}
      >
        {sr.version}
      </h1>

      {/* Meta line */}
      <div className="mb-5 flex items-center gap-2">
        <StatusDot status={sr.status} />
        <span
          className="text-[13px] text-[#6b7280]"
          style={{ fontFamily: "var(--font-dm-sans)" }}
        >
          {sr.status} · generated {timeAgo(sr.created_at)}
        </span>
      </div>

      {/* Divider */}
      <div className="mb-6" style={{ borderTop: "1px solid #e8e8e5" }} />

      {/* Error state */}
      {sr.error && (
        <div
          className="mb-6 rounded px-4 py-3 text-[13px] text-[#dc2626]"
          style={{ backgroundColor: "#fff1f2", border: "1px solid #dc2626" }}
        >
          {sr.error}
        </div>
      )}

      {report && (
        <div className="flex flex-col gap-6">
          {/* Summary */}
          <div className="flex flex-col gap-2">
            <SectionLabel>Summary</SectionLabel>
            <p
              className="text-[16px] leading-[1.7] text-[#111113]"
              style={{ fontFamily: "var(--font-dm-sans)" }}
            >
              {report.summary}
            </p>
          </div>

          {/* Urgency callout */}
          <UrgencyCallout urgency={report.urgency} description={report.recommendation} />

          {/* Availability + Adoption */}
          <div className="grid grid-cols-2 gap-4">
            <div
              className="rounded-md p-4"
              style={{ border: "1px solid #e8e8e5", backgroundColor: "#ffffff" }}
            >
              <SectionLabel className="mb-2">Availability</SectionLabel>
              <p
                className="text-[14px] text-[#374151]"
                style={{ fontFamily: "var(--font-dm-sans)" }}
              >
                {report.availability ?? "—"}
              </p>
            </div>
            <div
              className="rounded-md p-4"
              style={{ border: "1px solid #e8e8e5", backgroundColor: "#ffffff" }}
            >
              <SectionLabel className="mb-2">Adoption</SectionLabel>
              <p
                className="text-[14px] text-[#374151]"
                style={{ fontFamily: "var(--font-dm-sans)" }}
              >
                {report.adoption ?? "—"}
              </p>
            </div>
          </div>

          {/* Recommendation pull-quote */}
          {report.recommendation && (
            <blockquote
              className="rounded px-5 py-4 text-[18px] italic leading-relaxed text-[#16181c]"
              style={{
                fontFamily: "var(--font-fraunces)",
                borderLeft: "3px solid #e8601a",
                backgroundColor: "#fafaf9",
              }}
            >
              {report.recommendation}
            </blockquote>
          )}
        </div>
      )}

      {/* Source releases */}
      {sr.source_releases && sr.source_releases.length > 0 && (
        <div className="mt-8 flex flex-col gap-3">
          <div style={{ borderTop: "1px solid #e8e8e5", paddingTop: "24px" }}>
            <SectionLabel className="mb-3">Source Releases</SectionLabel>
            <div
              className="overflow-hidden rounded-md"
              style={{ border: "1px solid #e8e8e5" }}
            >
              <table className="w-full text-[13px]">
                <thead>
                  <tr style={{ borderBottom: "1px solid #e8e8e5", backgroundColor: "#fafaf9" }}>
                    <th className="px-4 py-2.5 text-left text-[11px] font-medium uppercase tracking-[0.08em] text-[#9ca3af]">Provider</th>
                    <th className="px-4 py-2.5 text-left text-[11px] font-medium uppercase tracking-[0.08em] text-[#9ca3af]">Repository</th>
                    <th className="px-4 py-2.5 text-left text-[11px] font-medium uppercase tracking-[0.08em] text-[#9ca3af]">Version</th>
                    <th className="px-4 py-2.5 text-left text-[11px] font-medium uppercase tracking-[0.08em] text-[#9ca3af]">Date</th>
                  </tr>
                </thead>
                <tbody>
                  {sr.source_releases.map((rel, i) => (
                    <tr
                      key={rel.id}
                      className="transition-colors duration-100 hover:bg-[#fafaf9]"
                      style={i > 0 ? { borderTop: "1px solid #e8e8e5" } : undefined}
                    >
                      <td className="px-4 py-2.5">
                        <ProviderBadge provider={rel.provider ?? ""} />
                      </td>
                      <td
                        className="px-4 py-2.5 text-[12px] text-[#6b7280]"
                        style={{ fontFamily: "'JetBrains Mono', monospace" }}
                      >
                        {rel.repository}
                      </td>
                      <td className="px-4 py-2.5">
                        <VersionChip version={rel.version} />
                      </td>
                      <td className="px-4 py-2.5 text-[12px] text-[#9ca3af]" style={{ fontFamily: "var(--font-dm-sans)" }}>
                        {timeAgo(rel.released_at)}
                      </td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          </div>
        </div>
      )}
    </div>
  );
}
```

**Note:** `sr.source_releases` field name — verify against `lib/api/types.ts`. It may be `sr.sources` or similar; adjust accordingly.

### Step 3: Type-check and commit

Run: `cd web && npx tsc --noEmit`

```bash
cd web && git add components/semantic-releases/semantic-release-detail.tsx
git commit -m "feat(web): redesign — semantic release detail (editorial single-column layout)"
```

---

## Task 9: Releases Pages Redesign

**Files:**
- Modify: `web/app/releases/page.tsx`
- Modify: `web/components/releases/release-detail.tsx` (or the releases detail page file)

### Step 1: Read current files

Read: `web/app/releases/page.tsx`
Read: `web/app/releases/[id]/page.tsx`

### Step 2: Rewrite `app/releases/page.tsx`

```tsx
// web/app/releases/page.tsx
"use client";

import { useState } from "react";
import Link from "next/link";
import useSWR from "swr";
import { releases, projects } from "@/lib/api/client";
import { ProviderBadge } from "@/components/ui/provider-badge";
import { VersionChip } from "@/components/ui/version-chip";

function timeAgo(dateStr?: string | null): string {
  if (!dateStr) return "—";
  const diff = Date.now() - new Date(dateStr).getTime();
  const mins = Math.floor(diff / 60000);
  if (mins < 60) return `${mins}m ago`;
  const hrs = Math.floor(mins / 60);
  if (hrs < 24) return `${hrs}h ago`;
  return `${Math.floor(hrs / 24)}d ago`;
}

export default function ReleasesPage() {
  const [selectedProject, setSelectedProject] = useState<string>("all");
  const [page, setPage] = useState(0);
  const limit = 20;

  const { data: projectsData } = useSWR("projects", () => projects.list());
  const projectList = projectsData?.data ?? [];

  // Fetch releases: use first project when "all" selected (API limitation)
  const targetProjectId = selectedProject !== "all"
    ? selectedProject
    : projectList[0]?.id;

  const { data: relData } = useSWR(
    targetProjectId ? `releases-${targetProjectId}-${page}` : null,
    () => releases.listByProject(targetProjectId!, { limit, offset: page * limit })
  );

  const items = relData?.data ?? [];
  const total = relData?.meta?.total ?? 0;

  return (
    <div className="flex flex-col gap-4 fade-in">
      {/* Page header */}
      <div className="flex items-center justify-between">
        <h1
          className="text-[24px] font-semibold text-[#111113]"
          style={{ fontFamily: "var(--font-fraunces)" }}
        >
          Releases
        </h1>
        <select
          value={selectedProject}
          onChange={(e) => { setSelectedProject(e.target.value); setPage(0); }}
          className="rounded-md px-3 py-1.5 text-[13px] text-[#374151] outline-none focus:ring-1 focus:ring-[#e8601a]"
          style={{ border: "1px solid #e8e8e5", fontFamily: "var(--font-dm-sans)", backgroundColor: "#ffffff" }}
        >
          <option value="all">All Projects</option>
          {projectList.map((p) => (
            <option key={p.id} value={p.id}>{p.name}</option>
          ))}
        </select>
      </div>

      {/* Table */}
      <div
        className="overflow-hidden rounded-md"
        style={{ border: "1px solid #e8e8e5", backgroundColor: "#ffffff" }}
      >
        <table className="w-full text-[13px]">
          <thead>
            <tr style={{ borderBottom: "1px solid #e8e8e5", backgroundColor: "#fafaf9" }}>
              {["Project", "Provider", "Repository", "Version", "Released", "Age"].map((h) => (
                <th
                  key={h}
                  className="px-4 py-2.5 text-left text-[11px] font-medium uppercase tracking-[0.08em] text-[#9ca3af]"
                >
                  {h}
                </th>
              ))}
            </tr>
          </thead>
          <tbody>
            {items.length === 0 ? (
              <tr>
                <td colSpan={6} className="px-4 py-8 text-center text-[14px] italic text-[#9ca3af]" style={{ fontFamily: "var(--font-fraunces)" }}>
                  No releases found
                </td>
              </tr>
            ) : (
              items.map((r, i) => (
                <tr
                  key={r.id}
                  className="transition-colors duration-100 hover:bg-[#fafaf9]"
                  style={i > 0 ? { borderTop: "1px solid #e8e8e5" } : undefined}
                >
                  <td className="px-4 py-2.5 text-[#374151]" style={{ fontFamily: "var(--font-dm-sans)" }}>
                    {r.project_name ?? selectedProject}
                  </td>
                  <td className="px-4 py-2.5">
                    <ProviderBadge provider={r.provider ?? ""} />
                  </td>
                  <td
                    className="px-4 py-2.5 text-[12px] text-[#6b7280]"
                    style={{ fontFamily: "'JetBrains Mono', monospace" }}
                  >
                    {r.repository}
                  </td>
                  <td className="px-4 py-2.5">
                    <Link href={`/releases/${r.id}`}>
                      <VersionChip version={r.version} />
                    </Link>
                  </td>
                  <td className="px-4 py-2.5 text-[12px] text-[#9ca3af]" style={{ fontFamily: "var(--font-dm-sans)" }}>
                    {r.released_at ? new Date(r.released_at).toLocaleDateString() : "—"}
                  </td>
                  <td className="px-4 py-2.5 text-[12px] text-[#9ca3af]" style={{ fontFamily: "var(--font-dm-sans)" }}>
                    {timeAgo(r.released_at)}
                  </td>
                </tr>
              ))
            )}
          </tbody>
        </table>
      </div>

      {/* Pagination */}
      {total > limit && (
        <div className="flex items-center justify-between text-[13px] text-[#6b7280]" style={{ fontFamily: "var(--font-dm-sans)" }}>
          <span>
            {page * limit + 1}–{Math.min((page + 1) * limit, total)} of {total}
          </span>
          <div className="flex gap-2">
            <button
              disabled={page === 0}
              onClick={() => setPage(p => p - 1)}
              className="rounded px-3 py-1 transition-colors hover:bg-[#f3f3f1] disabled:opacity-40"
              style={{ border: "1px solid #e8e8e5" }}
            >
              Previous
            </button>
            <button
              disabled={(page + 1) * limit >= total}
              onClick={() => setPage(p => p + 1)}
              className="rounded px-3 py-1 transition-colors hover:bg-[#f3f3f1] disabled:opacity-40"
              style={{ border: "1px solid #e8e8e5" }}
            >
              Next
            </button>
          </div>
        </div>
      )}
    </div>
  );
}
```

### Step 3: Redesign release detail page

Read `web/app/releases/[id]/page.tsx` (or `web/components/releases/release-detail.tsx`), then update the release detail to show:
- Source info: provider badge, repository (mono), version chip, released date
- Raw data as formatted JSON in a mono pre block
- Linked semantic releases (if `sr_id` is available)

Apply the same typography conventions (Fraunces headings, DM Sans body, JetBrains Mono code).

### Step 4: Type-check and build

Run: `cd web && npx tsc --noEmit && npm run build`

### Step 5: Commit

```bash
cd web && git add app/releases/ components/releases/
git commit -m "feat(web): redesign — releases list and detail pages"
```

---

## Task 10: Channels and Subscriptions Redesign

**Files:**
- Modify: `web/app/channels/page.tsx`
- Modify: `web/app/subscriptions/page.tsx`

### Step 1: Rewrite `app/channels/page.tsx`

Apply the same table pattern (no zebra, row dividers, white bg). Add type badges with distinct per-type colors.

```tsx
// web/app/channels/page.tsx
"use client";

import Link from "next/link";
import useSWR from "swr";
import { channels } from "@/lib/api/client";
import { Plus, Pencil, Trash2 } from "lucide-react";

const TYPE_COLORS: Record<string, { bg: string; text: string }> = {
  slack: { bg: "#4A154B", text: "#ffffff" },
  discord: { bg: "#5865F2", text: "#ffffff" },
  webhook: { bg: "#1a1a1a", text: "#ffffff" },
};

function TypeBadge({ type }: { type: string }) {
  const style = TYPE_COLORS[type.toLowerCase()] ?? { bg: "#6b7280", text: "#ffffff" };
  return (
    <span
      className="inline-flex items-center rounded-full px-2 py-0.5 text-[11px] font-medium capitalize"
      style={{ backgroundColor: style.bg, color: style.text }}
    >
      {type}
    </span>
  );
}

function configSummary(config: Record<string, unknown>): string {
  return Object.entries(config)
    .slice(0, 2)
    .map(([k, v]) => `${k}: ${String(v).slice(0, 30)}`)
    .join(", ");
}

export default function ChannelsPage() {
  const { data, mutate } = useSWR("channels", () => channels.list());
  const items = data?.data ?? [];

  async function handleDelete(id: string) {
    if (!confirm("Delete this channel?")) return;
    await channels.delete(id);
    mutate();
  }

  return (
    <div className="flex flex-col gap-4 fade-in">
      <div className="flex items-center justify-between">
        <h1
          className="text-[24px] font-semibold text-[#111113]"
          style={{ fontFamily: "var(--font-fraunces)" }}
        >
          Channels
        </h1>
        <Link
          href="/channels/new"
          className="flex items-center gap-1.5 rounded px-3 py-1.5 text-[13px] text-white transition-opacity hover:opacity-90"
          style={{ backgroundColor: "#e8601a", fontFamily: "var(--font-dm-sans)" }}
        >
          <Plus className="h-3.5 w-3.5" />
          New Channel
        </Link>
      </div>

      <div
        className="overflow-hidden rounded-md"
        style={{ border: "1px solid #e8e8e5", backgroundColor: "#ffffff" }}
      >
        <table className="w-full text-[13px]" style={{ fontFamily: "var(--font-dm-sans)" }}>
          <thead>
            <tr style={{ borderBottom: "1px solid #e8e8e5", backgroundColor: "#fafaf9" }}>
              {["Name", "Type", "Config", "Created", ""].map((h) => (
                <th key={h} className="px-4 py-2.5 text-left text-[11px] font-medium uppercase tracking-[0.08em] text-[#9ca3af]">
                  {h}
                </th>
              ))}
            </tr>
          </thead>
          <tbody>
            {items.length === 0 ? (
              <tr>
                <td colSpan={5} className="px-4 py-8 text-center text-[14px] italic text-[#9ca3af]" style={{ fontFamily: "var(--font-fraunces)" }}>
                  No channels yet — add one to receive notifications
                </td>
              </tr>
            ) : (
              items.map((ch, i) => (
                <tr
                  key={ch.id}
                  className="transition-colors duration-100 hover:bg-[#fafaf9]"
                  style={i > 0 ? { borderTop: "1px solid #e8e8e5" } : undefined}
                >
                  <td className="px-4 py-2.5 font-medium text-[#111113]">{ch.name}</td>
                  <td className="px-4 py-2.5"><TypeBadge type={ch.type} /></td>
                  <td
                    className="max-w-[280px] truncate px-4 py-2.5 text-[12px] text-[#9ca3af]"
                    style={{ fontFamily: "'JetBrains Mono', monospace" }}
                  >
                    {ch.config ? configSummary(ch.config as Record<string, unknown>) : "—"}
                  </td>
                  <td className="px-4 py-2.5 text-[12px] text-[#9ca3af]">
                    {new Date(ch.created_at).toLocaleDateString()}
                  </td>
                  <td className="px-4 py-2.5">
                    <div className="flex items-center gap-2">
                      <Link href={`/channels/${ch.id}/edit`} className="rounded p-1 text-[#9ca3af] transition-colors hover:text-[#374151]">
                        <Pencil className="h-3.5 w-3.5" />
                      </Link>
                      <button
                        onClick={() => handleDelete(ch.id)}
                        className="rounded p-1 text-[#9ca3af] transition-colors hover:text-[#dc2626]"
                      >
                        <Trash2 className="h-3.5 w-3.5" />
                      </button>
                    </div>
                  </td>
                </tr>
              ))
            )}
          </tbody>
        </table>
      </div>
    </div>
  );
}
```

### Step 2: Rewrite `app/subscriptions/page.tsx`

Apply the same table pattern with type badges (source/project), channel name lookup, mono version filter display, and a "+ New Subscription" button linking to `/subscriptions/new`.

Use the same structural pattern as channels above: `useSWR` for both `subscriptions.list()` and `channels.list()` (needed for channel name resolution). Apply identical table, badge, and empty state styling.

### Step 3: Type-check and final build

Run: `cd web && npx tsc --noEmit`
Run: `cd web && npm run build`
Expected: Zero errors, build succeeds.

### Step 4: Final commit

```bash
cd web && git add app/channels/page.tsx app/subscriptions/page.tsx
git commit -m "feat(web): redesign — channels and subscriptions pages"
```

---

## Final Verification

### Visual check

1. Start the dev server: `cd web && npm run dev`
2. Open `http://localhost:3000`
3. Verify against the design spec:
   - Sidebar: `#16181c` background, 5 items, 3px orange left border on active item, Fraunces italic logo
   - Header: 48px, white, breadcrumb
   - Dashboard: stat strip + two columns
   - `/projects/[id]`: header zone + 4 tabs with correct content
   - `/projects/[id]/semantic-releases/[srId]`: editorial single-column, max-width 760px

### Production build

Run: `cd web && npm run build`
Expected: Static export succeeds with no TypeScript errors.

---

## Plan complete and saved to `docs/plans/2026-02-25-frontend-redesign.md`.

**Two execution options:**

**1. Subagent-Driven (this session)** — I dispatch a fresh subagent per task, review between tasks, fast iteration

**2. Parallel Session (separate)** — Open a new session with executing-plans, batch execution with checkpoints

**Which approach?**
