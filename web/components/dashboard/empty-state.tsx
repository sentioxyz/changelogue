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
