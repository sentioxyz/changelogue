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
