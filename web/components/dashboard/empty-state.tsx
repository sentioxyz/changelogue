// web/components/dashboard/empty-state.tsx
"use client";

import Link from "next/link";
import { FolderPlus } from "lucide-react";
import { useTranslation } from "@/lib/i18n/context";

export function DashboardEmptyState() {
  const { t } = useTranslation();

  return (
    <div
      className="rounded-lg bg-surface px-8 py-16 text-center border border-border"
    >
      <FolderPlus className="mx-auto h-10 w-10 text-text-muted" />
      <h2
        className="mt-4 text-foreground"
        style={{
          fontFamily: "var(--font-fraunces)",
          fontSize: "18px",
          fontWeight: 600,
        }}
      >
        {t("dashboard.empty.title")}
      </h2>
      <p
        className="mt-2 text-text-secondary"
        style={{
          fontFamily: "var(--font-dm-sans)",
          fontSize: "14px",
        }}
      >
        {t("dashboard.empty.description")}
      </p>
      <Link
        href="/projects"
        className="mt-6 inline-flex items-center gap-2 rounded-lg bg-beacon-accent px-5 py-2.5 text-sm font-medium text-white transition-colors hover:opacity-90"
        style={{
          fontFamily: "var(--font-dm-sans)",
        }}
      >
        <FolderPlus className="h-4 w-4" />
        {t("dashboard.empty.createProject")}
      </Link>
    </div>
  );
}
