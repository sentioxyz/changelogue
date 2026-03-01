# Project Logos Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Show project logos (GitHub org/user avatars) on the projects list page and detail page, with initial-based placeholder fallback.

**Architecture:** Create a `ProjectLogo` component that resolves the best source by provider priority (GitHub > GitLab > Docker Hub > ECR), constructs the avatar URL client-side, and falls back to a deterministic initial-based colored circle. Integrate it into 3 locations: card view, compact view, and detail page header.

**Tech Stack:** React, TypeScript, plain `<img>` tag (no Next.js `<Image>` — the app uses static export in production which doesn't support external image optimization).

---

### Task 1: Create the ProjectLogo component

**Files:**
- Create: `web/components/ui/project-logo.tsx`

**Step 1: Create the component file**

```tsx
"use client";

import { useState } from "react";
import type { Source } from "@/lib/api/types";

const PROVIDER_PRIORITY: Record<string, number> = {
  github: 0,
  gitlab: 1,
  dockerhub: 2,
  "ecr-public": 3,
};

function getAvatarUrl(sources: Source[]): string | null {
  const sorted = [...sources]
    .filter((s) => s.provider in PROVIDER_PRIORITY)
    .sort((a, b) => (PROVIDER_PRIORITY[a.provider] ?? 99) - (PROVIDER_PRIORITY[b.provider] ?? 99));

  for (const source of sorted) {
    const owner = source.repository.split("/")[0];
    if (!owner) continue;

    if (source.provider === "github") {
      return `https://github.com/${owner}.png?size=64`;
    }
    // GitLab, Docker Hub, ECR don't have simple public avatar URLs
  }
  return null;
}

function hashCode(str: string): number {
  let hash = 0;
  for (let i = 0; i < str.length; i++) {
    hash = ((hash << 5) - hash + str.charCodeAt(i)) | 0;
  }
  return Math.abs(hash);
}

const PLACEHOLDER_COLORS = [
  "#e8601a", "#2496ed", "#16a34a", "#7c3aed",
  "#dc2626", "#0891b2", "#c026d3", "#ca8a04",
  "#4f46e5", "#059669", "#d97706", "#9333ea",
];

interface ProjectLogoProps {
  name: string;
  sources?: Source[];
  size?: number;
}

export function ProjectLogo({ name, sources = [], size = 40 }: ProjectLogoProps) {
  const [imgError, setImgError] = useState(false);
  const avatarUrl = getAvatarUrl(sources);
  const showImg = avatarUrl && !imgError;

  const initial = (name[0] ?? "?").toUpperCase();
  const color = PLACEHOLDER_COLORS[hashCode(name) % PLACEHOLDER_COLORS.length];
  const fontSize = Math.max(10, Math.round(size * 0.45));

  if (showImg) {
    return (
      <img
        src={avatarUrl}
        alt={`${name} logo`}
        width={size}
        height={size}
        onError={() => setImgError(true)}
        className="shrink-0 rounded-md object-cover"
        style={{ width: size, height: size }}
      />
    );
  }

  return (
    <div
      className="shrink-0 rounded-md flex items-center justify-center select-none"
      style={{
        width: size,
        height: size,
        backgroundColor: color,
        fontSize,
        fontFamily: "var(--font-fraunces), serif",
        fontWeight: 700,
        color: "#ffffff",
      }}
      aria-label={`${name} logo`}
    >
      {initial}
    </div>
  );
}
```

**Step 2: Commit**

```bash
git add web/components/ui/project-logo.tsx
git commit -m "feat(web): add ProjectLogo component with avatar resolution and placeholder fallback"
```

---

### Task 2: Integrate ProjectLogo into the projects list page (card view)

**Files:**
- Modify: `web/app/projects/page.tsx` — `ProjectCard` component (around line 397-433)

The card view already fetches sources in `SourcesSection`. However, `ProjectCard` itself doesn't have access to sources. We need to fetch sources at the card level and pass them to `ProjectLogo`.

**Step 1: Add import and update ProjectCard**

At the top of the file, add the import:
```tsx
import { ProjectLogo } from "@/components/ui/project-logo";
```

In the `ProjectCard` component, add a SWR call to get sources and add the logo next to the project name:

Replace the card's header `<div>` (lines 404-423):

```tsx
{/* Header */}
<div className="px-4 py-3">
  <div className="min-w-0 flex items-center gap-3">
    <ProjectCardLogo projectId={project.id} name={project.name} />
    <div className="min-w-0">
      <Link
        href={`/projects/${project.id}`}
        className="group inline-flex items-center gap-1.5 text-[16px] font-bold transition-colors"
        style={{ fontFamily: "var(--font-fraunces)", color: "#e8601a" }}
      >
        {project.name}
        <ArrowRight className="h-3.5 w-3.5 opacity-0 -translate-x-1 transition-all group-hover:opacity-100 group-hover:translate-x-0" />
      </Link>
      {project.description && (
        <p
          className="mt-0.5 text-[13px] truncate"
          style={{ color: "#6b7280", fontFamily: "var(--font-dm-sans)" }}
        >
          {project.description}
        </p>
      )}
    </div>
  </div>
</div>
```

Add a small helper component above `ProjectCard` that fetches sources for the logo:

```tsx
function ProjectCardLogo({ projectId, name }: { projectId: string; name: string }) {
  const { data } = useSWR(`project-${projectId}-card-sources`, () =>
    sourcesApi.listByProject(projectId)
  );
  return <ProjectLogo name={name} sources={data?.data} size={40} />;
}
```

**Step 2: Commit**

```bash
git add web/app/projects/page.tsx
git commit -m "feat(web): add project logo to card view on projects page"
```

---

### Task 3: Integrate ProjectLogo into the projects list page (compact view)

**Files:**
- Modify: `web/app/projects/page.tsx` — `ProjectCompactRow` component (around line 437-541)

**Step 1: Add logo to compact row**

The compact row already fetches sources via SWR (`srcData`). Add the `ProjectLogo` next to the project name.

Replace the project name `<span>` (lines 471-476):

```tsx
{/* Project name */}
<span
  className="flex items-center gap-2 text-[14px] font-bold shrink-0"
  style={{ fontFamily: "var(--font-fraunces)", color: "#e8601a", minWidth: "160px" }}
>
  <ProjectLogo name={project.name} sources={sources} size={24} />
  {project.name}
</span>
```

**Step 2: Commit**

```bash
git add web/app/projects/page.tsx
git commit -m "feat(web): add project logo to compact view on projects page"
```

---

### Task 4: Integrate ProjectLogo into the project detail page

**Files:**
- Modify: `web/components/projects/project-detail.tsx` — header zone (around line 235-264)

**Step 1: Add import**

Add at top of file:
```tsx
import { ProjectLogo } from "@/components/ui/project-logo";
```

**Step 2: Add logo to header**

In the header zone, the project name is inside `<div className="min-w-0 flex-1">` (line 237). Wrap it with a flex container that includes the logo.

Replace lines 235-237:
```tsx
<div className="flex items-start justify-between">
  {/* Left: project info */}
  <div className="min-w-0 flex-1 flex items-start gap-4">
    <ProjectLogo name={project.name} sources={sourcesData?.data} size={48} />
    <div className="min-w-0 flex-1">
```

Add a closing `</div>` after the source/context counts paragraph (after line 306):
```tsx
            </p>
          </div>  {/* close inner flex-1 */}
```

**Step 3: Commit**

```bash
git add web/components/projects/project-detail.tsx
git commit -m "feat(web): add project logo to project detail page header"
```

---

### Task 5: Verify the feature works

**Step 1: Run the frontend dev server and verify visually**

```bash
cd web && npm run dev
```

Check these pages:
- `/projects` — card view should show 40px logos next to project names
- `/projects` — compact view should show 24px logos next to project names
- `/projects/{id}` — detail header should show 48px logo next to project name

Verify:
- GitHub-sourced projects show the org avatar
- Projects without GitHub sources show the initial-based placeholder
- Broken avatar URLs gracefully fall back to the placeholder (test by temporarily modifying the URL)

**Step 2: Run TypeScript type check**

```bash
cd web && npx tsc --noEmit
```

**Step 3: Final commit if any fixes needed**
