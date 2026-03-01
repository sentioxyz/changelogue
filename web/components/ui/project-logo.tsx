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
