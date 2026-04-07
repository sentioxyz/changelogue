"use client";

import { usePathname } from "next/navigation";
import { useTranslation } from "@/lib/i18n/context";

const SEGMENT_KEYS: Record<string, string> = {
  "": "header.breadcrumb.dashboard",
  projects: "header.breadcrumb.projects",
  releases: "header.breadcrumb.releases",
  sources: "header.breadcrumb.sources",
  subscriptions: "header.breadcrumb.subscriptions",
  channels: "header.breadcrumb.channels",
  agent: "header.breadcrumb.agent",
  "semantic-releases": "header.breadcrumb.semanticReleases",
  "context-sources": "header.breadcrumb.contextSources",
  "api-keys": "header.breadcrumb.apiKeys",
  new: "header.breadcrumb.new",
  edit: "header.breadcrumb.edit",
};

export function Header() {
  const pathname = usePathname();
  const { t } = useTranslation();
  const segments = pathname.split("/").filter(Boolean);

  const uuidRe = /^[0-9a-f-]{8,}$/i;
  const breadcrumbs = segments
    .filter((s) => !uuidRe.test(s))
    .map((seg) => {
      const key = SEGMENT_KEYS[seg];
      return key ? t(key) : seg;
    });

  const display =
    breadcrumbs.length === 0
      ? t("header.breadcrumb.dashboard")
      : breadcrumbs.join(" / ");

  return (
    <header
      className="flex h-12 items-center border-b border-border bg-surface px-6"
    >
      <p
        className="text-[14px] font-medium text-foreground"
        style={{ fontFamily: "var(--font-dm-sans)" }}
      >
        {display}
      </p>
    </header>
  );
}
