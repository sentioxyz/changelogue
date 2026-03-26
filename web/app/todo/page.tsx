"use client";

import { useState, useEffect, useCallback, Suspense } from "react";
import useSWR from "swr";
import Link from "next/link";
import { todos as todosApi, projects as projectsApi } from "@/lib/api/client";
import type { TodoFilters } from "@/lib/api/client";
import { ProviderBadge } from "@/components/ui/provider-badge";
import { VersionChip } from "@/components/ui/version-chip";
import type { Todo } from "@/lib/api/types";
import { timeAgo } from "@/lib/format";
import { useTranslation } from "@/lib/i18n/context";
import { FilterBar, FilterConfig, expandDatePreset } from "@/components/filters/filter-bar";
import { useFilterParams } from "@/components/filters/use-filter-params";

const PER_PAGE = 15;

const URGENCY_COLORS: Record<string, { bg: string; text: string }> = {
  LOW: { bg: "#dcfce7", text: "#166534" },
  MEDIUM: { bg: "#fef9c3", text: "#854d0e" },
  HIGH: { bg: "#fee2e2", text: "#991b1b" },
  CRITICAL: { bg: "#1f2937", text: "#ffffff" },
};

/* ------------------------------------------------------------------ */
/*  Page                                                               */
/* ------------------------------------------------------------------ */

export default function TodoPage() {
  return (
    <Suspense>
      <TodoPageInner />
    </Suspense>
  );
}

function TodoPageInner() {
  const { t } = useTranslation();

  const FILTER_KEYS = ["status", "project", "provider", "urgency", "date", "aggregated"];
  const { filters, setFilters, page, setPage } = useFilterParams(FILTER_KEYS, { status: "pending", aggregated: "true" });

  const [confirmDialog, setConfirmDialog] = useState<{
    action: string;
    projectName?: string;
    version?: string;
    onConfirm: () => void;
  } | null>(null);

  /* Fetch projects for the Project filter dropdown */
  const { data: projectsData } = useSWR("projects-for-todo-filters", () =>
    projectsApi.list(1, 100)
  );

  /* Build filter config */
  const filterConfig: FilterConfig[] = [
    {
      key: "status",
      label: "Status",
      type: "select",
      options: [
        { value: "pending", label: "Pending" },
        { value: "acknowledged", label: "Acknowledged" },
        { value: "resolved", label: "Resolved" },
      ],
    },
    {
      key: "project",
      label: "Project",
      type: "select",
      options: (projectsData?.data ?? []).map((p) => ({ value: p.id, label: p.name })),
    },
    {
      key: "provider",
      label: "Provider",
      type: "select",
      options: [
        { value: "github", label: "GitHub" },
        { value: "dockerhub", label: "Docker Hub" },
        { value: "ecr-public", label: "ECR Public" },
        { value: "gitlab", label: "GitLab" },
        { value: "pypi", label: "PyPI" },
        { value: "npm", label: "npm" },
      ],
    },
    {
      key: "urgency",
      label: "Urgency",
      type: "select",
      options: [
        { value: "critical", label: "Critical" },
        { value: "high", label: "High" },
        { value: "medium", label: "Medium" },
        { value: "low", label: "Low" },
      ],
    },
    { key: "date", label: "Date", type: "date-range" },
    { key: "aggregated", label: "Latest Only", type: "boolean" },
  ];

  /* Convert filters to API params */
  const apiFilters: TodoFilters = {
    status: filters.status,
    aggregated: filters.aggregated === "true",
    project: filters.project,
    provider: filters.provider,
    urgency: filters.urgency,
  };
  if (filters.date) {
    const expanded = expandDatePreset(filters.date);
    apiFilters.date_from = expanded.date_from;
    apiFilters.date_to = expanded.date_to;
  }

  /* Fetch todos */
  const { data, isLoading, mutate } = useSWR(
    ["todos", page, JSON.stringify(filters)],
    () => todosApi.list(page, PER_PAGE, apiFilters),
    { refreshInterval: 30_000 }
  );

  /* SSE revalidation */
  const revalidateAll = useCallback(() => {
    mutate();
  }, [mutate]);

  useEffect(() => {
    const baseUrl = process.env.NEXT_PUBLIC_API_URL || "/api/v1";
    const es = new EventSource(`${baseUrl}/events`);
    es.onmessage = () => revalidateAll();
    es.onerror = () => {
      /* EventSource auto-reconnects; nothing to do */
    };
    return () => es.close();
  }, [revalidateAll]);

  const items: Todo[] = data?.data ?? [];
  const total = data?.meta?.total ?? 0;
  const totalPages = Math.max(1, Math.ceil(total / PER_PAGE));
  const startRow = (page - 1) * PER_PAGE + 1;
  const endRow = Math.min(page * PER_PAGE, total);

  /* Active status for action buttons */
  const activeStatus = filters.status ?? "pending";

  /* Action handlers with optimistic updates */
  const handleAcknowledge = async (id: string) => {
    mutate(
      (prev) => {
        if (!prev) return prev;
        return {
          ...prev,
          data: prev.data.filter((t) => t.id !== id),
          meta: prev.meta ? { ...prev.meta, total: prev.meta.total - 1 } : prev.meta,
        };
      },
      { revalidate: false }
    );
    try {
      await todosApi.acknowledge(id, true);
    } catch {
      /* Revert on failure */
    }
    revalidateAll();
  };

  const handleResolve = async (id: string, cascade = true) => {
    mutate(
      (prev) => {
        if (!prev) return prev;
        return {
          ...prev,
          data: prev.data.filter((t) => t.id !== id),
          meta: prev.meta ? { ...prev.meta, total: prev.meta.total - 1 } : prev.meta,
        };
      },
      { revalidate: false }
    );
    try {
      await todosApi.resolve(id, cascade);
    } catch {
      /* Revert on failure */
    }
    revalidateAll();
  };

  const handleReopen = async (id: string) => {
    mutate(
      (prev) => {
        if (!prev) return prev;
        return {
          ...prev,
          data: prev.data.filter((t) => t.id !== id),
          meta: prev.meta ? { ...prev.meta, total: prev.meta.total - 1 } : prev.meta,
        };
      },
      { revalidate: false }
    );
    try {
      await todosApi.reopen(id);
    } catch {
      /* Revert on failure */
    }
    revalidateAll();
  };

  return (
    <div className="space-y-6">
      {/* Page title */}
      <div>
        <h1
          className="text-foreground"
          style={{
            fontFamily: "var(--font-raleway)",
            fontSize: "24px",
            fontWeight: 700,
          }}
        >
          {t("todo.title")}
        </h1>
        <p className="mt-1 text-[13px] text-text-secondary" style={{ fontFamily: "var(--font-dm-sans)" }}>
          {t("todo.description")}
        </p>
      </div>

      {/* Filter bar */}
      <FilterBar filters={filterConfig} value={filters} onChange={setFilters} />

      {/* Table card */}
      <div
        className="overflow-hidden rounded-lg bg-surface"
        style={{ border: "1px solid var(--border)" }}
      >
        {isLoading ? (
          <div
            className="py-16 text-center text-text-secondary"
            style={{
              fontFamily: "var(--font-dm-sans)",
              fontSize: "13px",
            }}
          >
            {t("todo.loading")}
          </div>
        ) : items.length === 0 ? (
          <div className="py-16 text-center">
            <p
              className="text-text-muted"
              style={{
                fontFamily: "var(--font-raleway)",
                fontStyle: "italic",
                fontSize: "15px",
              }}
            >
              {t("todo.noItems").replace("{status}", activeStatus)}
            </p>
          </div>
        ) : (
          <table className="w-full">
            <thead>
              <tr style={{ borderBottom: "1px solid var(--border)", backgroundColor: "var(--background)" }}>
                {[t("todo.thProject"), t("todo.thVersion"), t("todo.thType"), t("todo.thProvider"), t("todo.thUrgency"), t("todo.thCreated"), t("todo.thActions")].map(
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
              {items.map((todo) => (
                <tr
                  key={todo.id}
                  className="transition-colors hover:bg-background"
                  style={{ borderBottom: "1px solid var(--border)" }}
                >
                  {/* Project */}
                  <td className="px-4 py-3">
                    <span
                      className="text-foreground"
                      style={{
                        fontFamily: "var(--font-dm-sans)",
                        fontSize: "13px",
                        fontWeight: 500,
                      }}
                    >
                      {todo.project_name ?? "\u2014"}
                    </span>
                  </td>

                  {/* Version */}
                  <td className="px-4 py-3">
                    {todo.version ? (
                      <Link
                        href={
                          todo.todo_type === "semantic" && todo.semantic_release_id && todo.project_id
                            ? `/projects/${todo.project_id}/semantic-releases/${todo.semantic_release_id}`
                            : todo.release_id
                              ? `/releases/${todo.release_id}`
                              : "#"
                        }
                      >
                        <VersionChip version={todo.version} />
                      </Link>
                    ) : (
                      <span
                        className="text-text-muted"
                        style={{
                          fontFamily: "var(--font-dm-sans)",
                          fontSize: "13px",
                        }}
                      >
                        {"\u2014"}
                      </span>
                    )}
                  </td>

                  {/* Type */}
                  <td className="px-4 py-3">
                    <span
                      className="inline-flex items-center rounded-full px-2 py-0.5 text-[11px] font-medium leading-none"
                      style={{
                        backgroundColor: todo.todo_type === "semantic" ? "#ede9fe" : "#e0f2fe",
                        color: todo.todo_type === "semantic" ? "#6d28d9" : "#0369a1",
                      }}
                    >
                      {todo.todo_type === "semantic" ? t("todo.typeSemantic") : t("todo.typeRelease")}
                    </span>
                  </td>

                  {/* Provider */}
                  <td className="px-4 py-3">
                    {todo.provider ? (
                      <ProviderBadge provider={todo.provider} />
                    ) : (
                      <span
                        className="text-text-muted"
                        style={{
                          fontFamily: "var(--font-dm-sans)",
                          fontSize: "13px",
                        }}
                      >
                        {"\u2014"}
                      </span>
                    )}
                  </td>

                  {/* Urgency */}
                  <td className="px-4 py-3">
                    {todo.urgency && todo.todo_type === "semantic" ? (
                      <span
                        className="inline-flex items-center rounded-full px-2 py-0.5 text-[11px] font-medium leading-none"
                        style={{
                          backgroundColor:
                            URGENCY_COLORS[todo.urgency.toUpperCase()]?.bg ?? "var(--mono-bg)",
                          color:
                            URGENCY_COLORS[todo.urgency.toUpperCase()]?.text ?? "var(--secondary-foreground)",
                        }}
                      >
                        {todo.urgency.toUpperCase()}
                      </span>
                    ) : (
                      <span
                        className="text-text-muted"
                        style={{
                          fontFamily: "var(--font-dm-sans)",
                          fontSize: "13px",
                        }}
                      >
                        {"\u2014"}
                      </span>
                    )}
                  </td>

                  {/* Created */}
                  <td className="px-4 py-3">
                    <span
                      className="text-text-muted"
                      style={{
                        fontFamily: "var(--font-dm-sans)",
                        fontSize: "13px",
                      }}
                    >
                      {timeAgo(todo.created_at)}
                    </span>
                  </td>

                  {/* Actions */}
                  <td className="px-4 py-3">
                    <div className="flex items-center gap-1.5">
                      {activeStatus === "pending" && (
                        <>
                          <button
                            onClick={() => handleAcknowledge(todo.id)}
                            className="rounded-md px-2.5 py-1 transition-colors hover:opacity-80"
                            style={{
                              fontFamily: "var(--font-dm-sans)",
                              fontSize: "12px",
                              fontWeight: 500,
                              backgroundColor: "#dcfce7",
                              color: "#166534",
                              border: "1px solid #bbf7d0",
                            }}
                          >
                            {t("todo.acknowledge")}
                          </button>
                          <button
                            onClick={() =>
                              setConfirmDialog({
                                action: t("todo.dismiss"),
                                projectName: todo.project_name,
                                version: todo.version,
                                onConfirm: () => handleResolve(todo.id, false),
                              })
                            }
                            className="rounded-md px-2.5 py-1 transition-colors hover:opacity-80 text-text-secondary border-border"
                            style={{
                              fontFamily: "var(--font-dm-sans)",
                              fontSize: "12px",
                              fontWeight: 500,
                              border: "1px solid var(--border)",
                            }}
                          >
                            {t("todo.dismiss")}
                          </button>
                        </>
                      )}
                      {activeStatus === "acknowledged" && (
                        <>
                          <button
                            onClick={() => handleResolve(todo.id)}
                            className="rounded-md px-2.5 py-1 transition-colors hover:opacity-80"
                            style={{
                              fontFamily: "var(--font-dm-sans)",
                              fontSize: "12px",
                              fontWeight: 500,
                              backgroundColor: "#dbeafe",
                              color: "#1e40af",
                              border: "1px solid #bfdbfe",
                            }}
                          >
                            {t("todo.resolve")}
                          </button>
                          <button
                            onClick={() =>
                              setConfirmDialog({
                                action: t("todo.undo"),
                                projectName: todo.project_name,
                                version: todo.version,
                                onConfirm: () => handleReopen(todo.id),
                              })
                            }
                            className="rounded-md px-2.5 py-1 transition-colors hover:opacity-80 text-text-secondary"
                            style={{
                              fontFamily: "var(--font-dm-sans)",
                              fontSize: "12px",
                              fontWeight: 500,
                              border: "1px solid var(--border)",
                            }}
                          >
                            {t("todo.undo")}
                          </button>
                        </>
                      )}
                      {activeStatus === "resolved" && (
                        <button
                          onClick={() =>
                            setConfirmDialog({
                              action: t("todo.reopen"),
                              projectName: todo.project_name,
                              version: todo.version,
                              onConfirm: () => handleReopen(todo.id),
                            })
                          }
                          className="rounded-md px-2.5 py-1 transition-colors hover:opacity-80 text-text-secondary"
                          style={{
                            fontFamily: "var(--font-dm-sans)",
                            fontSize: "12px",
                            fontWeight: 500,
                            border: "1px solid var(--border)",
                          }}
                        >
                          {t("todo.reopen")}
                        </button>
                      )}
                    </div>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        )}
      </div>

      {/* Pagination */}
      {total > 0 && (
        <div className="flex items-center justify-between">
          <span
            className="text-text-muted"
            style={{
              fontFamily: "var(--font-dm-sans)",
              fontSize: "13px",
            }}
          >
            {startRow}&ndash;{endRow} of {total}
          </span>
          <div className="flex items-center gap-2">
            <button
              disabled={page <= 1}
              onClick={() => setPage(page - 1)}
              className="rounded-md bg-surface px-3 py-1.5 transition-colors hover:bg-background disabled:cursor-not-allowed disabled:opacity-40 text-secondary-foreground border-border"
              style={{
                fontFamily: "var(--font-dm-sans)",
                fontSize: "13px",
                border: "1px solid var(--border)",
              }}
            >
              {t("todo.previous")}
            </button>
            <button
              disabled={page >= totalPages}
              onClick={() => setPage(page + 1)}
              className="rounded-md bg-surface px-3 py-1.5 transition-colors hover:bg-background disabled:cursor-not-allowed disabled:opacity-40 text-secondary-foreground border-border"
              style={{
                fontFamily: "var(--font-dm-sans)",
                fontSize: "13px",
                border: "1px solid var(--border)",
              }}
            >
              {t("todo.next")}
            </button>
          </div>
        </div>
      )}

      {/* Confirmation dialog */}
      {confirmDialog && (
        <div
          className="fixed inset-0 z-50 flex items-center justify-center"
          style={{ backgroundColor: "rgba(0,0,0,0.4)" }}
          onClick={() => setConfirmDialog(null)}
        >
          <div
            className="rounded-lg bg-surface p-6 shadow-xl"
            style={{ maxWidth: "360px", width: "100%" }}
            onClick={(e) => e.stopPropagation()}
          >
            <p
              className="text-foreground"
              style={{
                fontFamily: "var(--font-dm-sans)",
                fontSize: "14px",
                fontWeight: 400,
                marginBottom: "20px",
                lineHeight: 1.6,
              }}
            >
              {confirmDialog.action}{" "}
              {confirmDialog.projectName && (
                <span style={{ fontWeight: 600 }}>{confirmDialog.projectName}</span>
              )}{" "}
              <span
                className="bg-mono-bg"
                style={{
                  fontFamily: "var(--font-mono, ui-monospace, monospace)",
                  fontSize: "13px",
                  borderRadius: "4px",
                  padding: "1px 6px",
                  color: "#6d28d9",
                }}
              >
                {confirmDialog.version ?? t("todo.thisItem")}
              </span>
              ?
            </p>
            <div className="flex items-center justify-end gap-2">
              <button
                onClick={() => setConfirmDialog(null)}
                className="rounded-md px-3 py-1.5 transition-colors hover:bg-mono-bg text-text-secondary"
                style={{
                  fontFamily: "var(--font-dm-sans)",
                  fontSize: "13px",
                  border: "1px solid var(--border)",
                }}
              >
                {t("todo.cancel")}
              </button>
              <button
                onClick={() => {
                  confirmDialog.onConfirm();
                  setConfirmDialog(null);
                }}
                className="rounded-md px-3 py-1.5 transition-colors hover:opacity-80"
                style={{
                  fontFamily: "var(--font-dm-sans)",
                  fontSize: "13px",
                  fontWeight: 500,
                  backgroundColor: "#fee2e2",
                  color: "#991b1b",
                  border: "1px solid #fecaca",
                }}
              >
                {t("todo.confirm")}
              </button>
            </div>
          </div>
        </div>
      )}
    </div>
  );
}
