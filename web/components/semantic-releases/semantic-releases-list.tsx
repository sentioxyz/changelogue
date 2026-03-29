"use client";

import { useState, Suspense } from "react";
import { useSearchParams } from "next/navigation";
import useSWR from "swr";
import Link from "next/link";
import {
  semanticReleases as srApi,
  projects as projectsApi,
} from "@/lib/api/client";
import { VersionChip } from "@/components/ui/version-chip";
import { ConfirmDialog } from "@/components/ui/confirm-dialog";
import { Trash2 } from "lucide-react";
import { timeAgo } from "@/lib/format";
import type { Project, SemanticRelease } from "@/lib/api/types";
import { useTranslation } from "@/lib/i18n/context";
import { UrgencyPill } from "@/components/ui/urgency-pill";

/* ---------- Helpers ---------- */

const PER_PAGE = 15;

/* ---------- Main component ---------- */

export function SemanticReleasesList() {
  return (
    <Suspense>
      <SemanticReleasesListInner />
    </Suspense>
  );
}

function SemanticReleasesListInner() {
  const searchParams = useSearchParams();
  const initialProject = searchParams.get("project") ?? "all";
  const [page, setPage] = useState(1);
  const [projectFilter, setProjectFilter] = useState<string>(initialProject);
  const [deletingId, setDeletingId] = useState<string | null>(null);
  const { t } = useTranslation();

  /* Fetch projects for the filter dropdown */
  const { data: projectsData } = useSWR("projects-for-sr-filter", () =>
    projectsApi.list(1, 100)
  );

  /* Fetch semantic releases — scoped by project or all */
  const { data: scopedData, isLoading: scopedLoading, mutate: mutateScopedData } = useSWR(
    projectFilter !== "all" ? ["sr-scoped", page, projectFilter] : null,
    () => srApi.list(projectFilter, page, PER_PAGE)
  );

  const { data: allData, isLoading: allLoading, mutate: mutateAllData } = useSWR(
    projectFilter === "all" ? ["sr-all", page] : null,
    () => srApi.listAll(page, PER_PAGE)
  );

  const isLoading = projectFilter !== "all" ? scopedLoading : allLoading;
  const releases: SemanticRelease[] =
    projectFilter !== "all"
      ? scopedData?.data ?? []
      : allData?.data ?? [];

  const mutateData = projectFilter !== "all" ? mutateScopedData : mutateAllData;

  /* Pagination math */
  const activeData = projectFilter !== "all" ? scopedData : allData;
  const total = activeData?.meta?.total ?? 0;
  const totalPages = Math.max(1, Math.ceil(total / PER_PAGE));
  const startRow = (page - 1) * PER_PAGE + 1;
  const endRow = Math.min(page * PER_PAGE, total);

  return (
    <div className="space-y-6 fade-in">
      {/* Page title */}
      <h1
        style={{
          fontFamily: "var(--font-raleway)",
          fontSize: "24px",
          fontWeight: 700,
          color: "var(--foreground)",
        }}
      >
        {t("sr.title")}
      </h1>

      {/* Project filter */}
      <div>
        <select
          value={projectFilter}
          onChange={(e) => {
            setProjectFilter(e.target.value);
            setPage(1);
          }}
          className="appearance-none rounded-md bg-surface px-3 py-2 pr-8 outline-none transition-shadow"
          style={{
            fontFamily: "var(--font-dm-sans)",
            fontSize: "13px",
            color: "var(--foreground)",
            border: "1px solid var(--border)",
          }}
          onFocus={(e) =>
            (e.currentTarget.style.boxShadow = "0 0 0 2px #e8601a40")
          }
          onBlur={(e) => (e.currentTarget.style.boxShadow = "none")}
        >
          <option value="all">{t("sr.allProjects")}</option>
          {projectsData?.data.map((p: Project) => (
            <option key={p.id} value={p.id}>
              {p.name}
            </option>
          ))}
        </select>
      </div>

      {/* Table card */}
      <div
        className="overflow-hidden rounded-lg bg-surface"
        style={{ border: "1px solid var(--border)" }}
      >
        {isLoading ? (
          <div
            className="py-16 text-center"
            style={{
              fontFamily: "var(--font-dm-sans)",
              fontSize: "13px",
              color: "var(--text-secondary)",
            }}
          >
            {t("sr.loading")}
          </div>
        ) : releases.length === 0 ? (
          <div className="py-16 text-center">
            <p
              style={{
                fontFamily: "var(--font-raleway)",
                fontStyle: "italic",
                fontSize: "15px",
                color: "var(--text-muted)",
              }}
            >
              {t("sr.noReleasesYet")}
            </p>
          </div>
        ) : (
          <table className="w-full">
            <thead>
              <tr style={{ borderBottom: "1px solid var(--border)", backgroundColor: "var(--background)" }}>
                {[t("sr.colProject"), t("sr.colVersion"), t("sr.colStatus"), t("sr.colUrgency"), t("sr.colAge"), ""].map(
                  (col) => (
                    <th
                      key={col}
                      className="px-4 py-3 text-left"
                      style={{
                        fontFamily: "var(--font-dm-sans)",
                        fontSize: "11px",
                        fontWeight: 600,
                        textTransform: "uppercase" as const,
                        letterSpacing: "0.08em",
                        color: "var(--text-muted)",
                      }}
                    >
                      {col}
                    </th>
                  )
                )}
              </tr>
            </thead>
            <tbody>
              {releases.map((sr) => {
                return (
                  <tr
                    key={sr.id}
                    className="transition-colors hover:bg-background"
                    style={{ borderBottom: "1px solid var(--border)" }}
                  >
                    {/* Project */}
                    <td className="px-4 py-3">
                      {sr.project_name ? (
                        <Link
                          href={`/projects/${sr.project_id}`}
                          className="hover:underline"
                          style={{
                            fontFamily: "var(--font-dm-sans)",
                            fontSize: "13px",
                            color: "var(--foreground)",
                            fontWeight: 500,
                          }}
                        >
                          {sr.project_name}
                        </Link>
                      ) : (
                        <span
                          style={{
                            fontFamily: "var(--font-dm-sans)",
                            fontSize: "13px",
                            color: "var(--text-muted)",
                          }}
                        >
                          {"\u2014"}
                        </span>
                      )}
                    </td>

                    {/* Version */}
                    <td className="px-4 py-3">
                      <Link href={`/projects/${sr.project_id}/semantic-releases/${sr.id}`}>
                        <VersionChip version={sr.version} />
                      </Link>
                    </td>

                    {/* Status */}
                    <td className="px-4 py-3">
                      <span
                        className="text-[13px] font-medium"
                        style={{
                          fontFamily: "var(--font-dm-sans)",
                          color: sr.status === "completed" ? "var(--status-completed)" : "var(--beacon-accent)",
                        }}
                      >
                        {sr.status}
                      </span>
                    </td>

                    {/* Urgency */}
                    <td className="px-4 py-3">
                      {sr.report?.urgency ? (
                        <UrgencyPill urgency={sr.report.urgency} variant="text" />
                      ) : (
                        <span
                          style={{
                            fontFamily: "var(--font-dm-sans)",
                            fontSize: "13px",
                            color: "var(--text-muted)",
                          }}
                        >
                          {"\u2014"}
                        </span>
                      )}
                    </td>

                    {/* Age */}
                    <td className="px-4 py-3">
                      <span
                        style={{
                          fontFamily: "var(--font-dm-sans)",
                          fontSize: "13px",
                          color: "var(--text-muted)",
                        }}
                      >
                        {timeAgo(sr.completed_at ?? sr.created_at)}
                      </span>
                    </td>

                    {/* Delete */}
                    <td className="px-4 py-3">
                      <button
                        onClick={() => setDeletingId(sr.id)}
                        className="rounded p-1 transition-colors hover:bg-red-50 hover:text-red-600 dark:hover:bg-red-900/20"
                        style={{ color: "var(--text-muted)" }}
                        title={t("sr.deleteRelease")}
                      >
                        <Trash2 className="h-4 w-4" />
                      </button>
                    </td>
                  </tr>
                );
              })}
            </tbody>
          </table>
        )}
      </div>

      {/* Pagination */}
      {total > 0 && (
        <div className="flex items-center justify-between">
          <span
            style={{
              fontFamily: "var(--font-dm-sans)",
              fontSize: "13px",
              color: "var(--text-muted)",
            }}
          >
            {startRow}&ndash;{endRow} of {total}
          </span>
          <div className="flex items-center gap-2">
            <button
              disabled={page <= 1}
              onClick={() => setPage(page - 1)}
              className="rounded-md bg-surface px-3 py-1.5 transition-colors hover:bg-background disabled:cursor-not-allowed disabled:opacity-40"
              style={{
                fontFamily: "var(--font-dm-sans)",
                fontSize: "13px",
                color: "var(--secondary-foreground)",
                border: "1px solid var(--border)",
              }}
            >
              {t("sr.previous")}
            </button>
            <button
              disabled={page >= totalPages}
              onClick={() => setPage(page + 1)}
              className="rounded-md bg-surface px-3 py-1.5 transition-colors hover:bg-background disabled:cursor-not-allowed disabled:opacity-40"
              style={{
                fontFamily: "var(--font-dm-sans)",
                fontSize: "13px",
                color: "var(--secondary-foreground)",
                border: "1px solid var(--border)",
              }}
            >
              {t("sr.next")}
            </button>
          </div>
        </div>
      )}

      {/* Delete confirmation */}
      <ConfirmDialog
        open={!!deletingId}
        onOpenChange={(open) => {
          if (!open) setDeletingId(null);
        }}
        title={t("sr.deleteTitle")}
        description={t("sr.deleteDescription")}
        onConfirm={async () => {
          if (deletingId) {
            await srApi.delete(deletingId);
            mutateData();
          }
        }}
      />
    </div>
  );
}
