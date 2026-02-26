"use client";

import { useState } from "react";
import useSWR from "swr";
import Link from "next/link";
import { semanticReleases as srApi } from "@/lib/api/client";
import { VersionChip } from "@/components/ui/version-chip";
import { ConfirmDialog } from "@/components/ui/confirm-dialog";
import { ArrowLeft, Trash2 } from "lucide-react";
import { timeAgo } from "@/lib/format";

/* ---------- Urgency chip ---------- */

const URGENCY_COLORS: Record<string, { bg: string; text: string }> = {
  critical: { bg: "#dc2626", text: "#ffffff" },
  high: { bg: "#f97316", text: "#ffffff" },
};

function UrgencyChip({ urgency }: { urgency: string }) {
  const u = urgency.toLowerCase();
  const style = URGENCY_COLORS[u];
  if (!style) return null;
  return (
    <span
      className="inline-flex items-center rounded-full px-2 py-0.5 text-[11px] font-medium leading-none"
      style={{ backgroundColor: style.bg, color: style.text }}
    >
      {urgency}
    </span>
  );
}

/* ---------- Helpers ---------- */

function truncate(str: string, max: number): string {
  return str.length > max ? str.slice(0, max) + "\u2026" : str;
}

/* ---------- Main component ---------- */

export function SemanticReleasesList({ projectId }: { projectId: string }) {
  const { data, isLoading, mutate } = useSWR(`project-${projectId}-sr`, () =>
    srApi.list(projectId)
  );
  const [deletingId, setDeletingId] = useState<string | null>(null);

  return (
    <div className="space-y-6">
      {/* Back link */}
      <Link
        href={`/projects/${projectId}`}
        className="inline-flex items-center gap-1.5 transition-colors hover:opacity-70"
        style={{
          fontFamily: "var(--font-dm-sans), sans-serif",
          fontSize: "13px",
          color: "#6b7280",
        }}
      >
        <ArrowLeft className="h-3.5 w-3.5" />
        Back to Project
      </Link>

      {/* Page title */}
      <h1
        style={{
          fontFamily: "var(--font-fraunces), serif",
          fontSize: "24px",
          fontWeight: 700,
          color: "#111113",
        }}
      >
        Semantic Releases
      </h1>

      {/* List */}
      {isLoading ? (
        <div
          className="py-16 text-center"
          style={{
            fontFamily: "var(--font-dm-sans), sans-serif",
            fontSize: "13px",
            color: "#6b7280",
          }}
        >
          Loading...
        </div>
      ) : data?.data && data.data.length > 0 ? (
        <div className="space-y-3">
          {data.data.map((sr) => (
            <div
              key={sr.id}
              className="flex items-start justify-between rounded-md p-4 transition-colors hover:bg-[#fafaf9]"
              style={{ border: "1px solid #e8e8e5" }}
            >
              <Link
                href={`/projects/${projectId}/semantic-releases/${sr.id}`}
                className="min-w-0 flex-1"
              >
                <div className="flex items-center gap-2.5">
                  <VersionChip version={sr.version} />
                  <span
                    className="text-[13px] font-medium"
                    style={{
                      color: sr.status === "completed" ? "#16a34a" : "#e8601a",
                    }}
                  >
                    {sr.status}
                  </span>
                  {sr.report?.urgency && (
                    <UrgencyChip urgency={sr.report.urgency} />
                  )}
                  <span
                    className="ml-auto text-[12px]"
                    style={{
                      fontFamily: "var(--font-dm-sans), sans-serif",
                      color: "#9ca3af",
                    }}
                  >
                    {timeAgo(sr.created_at)}
                  </span>
                </div>
                {sr.report?.summary && (
                  <p
                    className="mt-2 text-[13px] italic leading-relaxed"
                    style={{
                      fontFamily: "var(--font-dm-sans), sans-serif",
                      color: "#6b7280",
                    }}
                  >
                    {truncate(sr.report.summary, 200)}
                  </p>
                )}
                {sr.error && (
                  <p
                    className="mt-2 text-[13px]"
                    style={{
                      fontFamily: "var(--font-dm-sans), sans-serif",
                      color: "#dc2626",
                    }}
                  >
                    {truncate(sr.error, 200)}
                  </p>
                )}
              </Link>
              <button
                onClick={(e) => {
                  e.preventDefault();
                  setDeletingId(sr.id);
                }}
                className="ml-3 shrink-0 rounded p-1 transition-colors hover:bg-red-50 hover:text-red-600"
                style={{ color: "#9ca3af" }}
                title="Delete semantic release"
              >
                <Trash2 className="h-4 w-4" />
              </button>
            </div>
          ))}
        </div>
      ) : (
        <div className="py-16 text-center">
          <p
            style={{
              fontFamily: "var(--font-fraunces), serif",
              fontStyle: "italic",
              fontSize: "15px",
              color: "#9ca3af",
            }}
          >
            No semantic releases yet
          </p>
        </div>
      )}

      {/* Delete confirmation */}
      <ConfirmDialog
        open={!!deletingId}
        onOpenChange={(open) => {
          if (!open) setDeletingId(null);
        }}
        title="Delete Semantic Release"
        description="This will permanently delete this semantic release and its report."
        onConfirm={async () => {
          if (deletingId) {
            await srApi.delete(deletingId);
            mutate();
          }
        }}
      />
    </div>
  );
}
