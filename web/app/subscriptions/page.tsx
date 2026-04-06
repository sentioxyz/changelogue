"use client";

import { useState, useMemo, Suspense } from "react";
import useSWR, { mutate } from "swr";
import {
  subscriptions as subsApi,
  channels as channelsApi,
  projects as projectsApi,
  sources as sourcesApi,
} from "@/lib/api/client";
import type { Source, Subscription } from "@/lib/api/types";
import { Plus, Pencil, Trash2 } from "lucide-react";
import { FilterBar, FilterConfig } from "@/components/filters/filter-bar";
import { useFilterParams } from "@/components/filters/use-filter-params";
import { Checkbox } from "@/components/ui/checkbox";
import { Dialog, DialogContent, DialogHeader, DialogTitle } from "@/components/ui/dialog";
import { ConfirmDialog } from "@/components/ui/confirm-dialog";
import { SubscriptionForm } from "@/components/subscriptions/subscription-form";
import { useTranslation } from "@/lib/i18n/context";

const SUB_TYPE_COLORS: Record<string, { bg: string; text: string }> = {
  source_release: { bg: "#1a1a1a", text: "#ffffff" },
  semantic_release: { bg: "#2563eb", text: "#ffffff" },
};

function SubTypeBadge({ type }: { type: string }) {
  const colors = SUB_TYPE_COLORS[type] ?? { bg: "#6b7280", text: "#ffffff" };
  return (
    <span
      className="inline-flex items-center rounded-full px-2.5 py-0.5"
      style={{
        backgroundColor: colors.bg,
        color: colors.text,
        fontFamily: "var(--font-dm-sans)",
        fontSize: "12px",
        fontWeight: 500,
        lineHeight: "16px",
      }}
    >
      {type}
    </span>
  );
}

export default function SubscriptionsPage() {
  return (
    <Suspense>
      <SubscriptionsPageInner />
    </Suspense>
  );
}

function SubscriptionsPageInner() {
  const { t } = useTranslation();
  const FILTER_KEYS = ["channel", "type"];
  const { filters, setFilters } = useFilterParams(FILTER_KEYS);
  const { data, isLoading } = useSWR("subscriptions", () => subsApi.list());
  const { data: channelsData } = useSWR("channels-for-sub-list", () =>
    channelsApi.list()
  );
  const { data: projectsData } = useSWR("projects-for-sub-list", () =>
    projectsApi.list(1, 100)
  );

  // Collect unique source IDs from subscriptions and fetch each one
  const sourceIds = useMemo(
    () =>
      Array.from(
        new Set(
          (data?.data ?? [])
            .filter((s) => s.type === "source_release" && s.source_id)
            .map((s) => s.source_id!)
        )
      ),
    [data]
  );
  const { data: sourcesMap } = useSWR(
    sourceIds.length > 0 ? `sources-for-sub-list-${sourceIds.join(",")}` : null,
    async () => {
      const results = await Promise.all(
        sourceIds.map((id) => sourcesApi.get(id).catch(() => null))
      );
      const map: Record<string, Source> = {};
      for (const r of results) {
        if (r?.data) map[r.data.id] = r.data;
      }
      return map;
    }
  );

  const [createOpen, setCreateOpen] = useState(false);
  const [editingSub, setEditingSub] = useState<Subscription | null>(null);
  const [deletingId, setDeletingId] = useState<string | null>(null);
  const [selectedIds, setSelectedIds] = useState<Set<string>>(new Set());
  const [batchDeleteOpen, setBatchDeleteOpen] = useState(false);

  const getChannelName = (id: string) =>
    channelsData?.data.find((c) => c.id === id)?.name ?? id;

  const getProjectName = (id: string) =>
    projectsData?.data.find((p) => p.id === id)?.name ?? id;

  const getSourceLabel = (id: string) => {
    const source = sourcesMap?.[id];
    return source ? `${source.provider}: ${source.repository}` : id;
  };

  /* Build filter config */
  const filterConfig: FilterConfig[] = useMemo(() => [
    {
      key: "channel",
      label: "Channel",
      type: "select" as const,
      options: (channelsData?.data ?? []).map((c) => ({ value: c.id, label: c.name })),
    },
    {
      key: "type",
      label: "Type",
      type: "select" as const,
      options: [
        { value: "source_release", label: "Source Release" },
        { value: "semantic_release", label: "Semantic Release" },
      ],
    },
  ], [channelsData]);

  /* Client-side filtering */
  const allSubscriptions = data?.data ?? [];
  const subscriptions = useMemo(() => {
    let result = allSubscriptions;
    if (filters.channel) {
      result = result.filter((s) => s.channel_id === filters.channel);
    }
    if (filters.type) {
      result = result.filter((s) => s.type === filters.type);
    }
    return result;
  }, [allSubscriptions, filters.channel, filters.type]);

  const isAllSelected = subscriptions.length > 0 && selectedIds.size === subscriptions.length;
  const isSomeSelected = selectedIds.size > 0 && !isAllSelected;

  const toggleSelect = (id: string) => {
    setSelectedIds((prev) => {
      const next = new Set(prev);
      if (next.has(id)) next.delete(id);
      else next.add(id);
      return next;
    });
  };

  const toggleSelectAll = () => {
    if (isAllSelected) {
      setSelectedIds(new Set());
    } else {
      setSelectedIds(new Set(subscriptions.map((s) => s.id)));
    }
  };

  const clearSelection = () => setSelectedIds(new Set());

  return (
    <div className="space-y-6">
      {/* Header */}
      <div className="flex items-center justify-between">
        <div>
          <h1
            className="text-foreground"
            style={{
              fontFamily: "var(--font-raleway)",
              fontSize: "24px",
              fontWeight: 700,
            }}
          >
            {t("subscriptions.title")}
          </h1>
          <p className="mt-1 text-[13px] text-text-secondary" style={{ fontFamily: "var(--font-dm-sans)" }}>
            {t("subscriptions.description")}
          </p>
        </div>
        <button
          onClick={() => setCreateOpen(true)}
          className="inline-flex items-center gap-1.5 rounded-md px-3.5 py-2 transition-colors hover:opacity-90 bg-beacon-accent text-white"
          style={{
            fontFamily: "var(--font-dm-sans)",
            fontSize: "13px",
            fontWeight: 500,
          }}
        >
          <Plus className="h-4 w-4" />
          {t("subscriptions.newSubscription")}
        </button>
      </div>

      {/* Filter ribbon */}
      <FilterBar filters={filterConfig} value={filters} onChange={setFilters} />

      {isLoading ? (
        <div
          className="overflow-hidden rounded-lg bg-surface py-16 text-center border-border"
          style={{ border: "1px solid var(--border)", fontFamily: "var(--font-dm-sans)", fontSize: "13px", color: "var(--text-secondary)" }}
        >
          {t("subscriptions.loading")}
        </div>
      ) : subscriptions.length === 0 ? (
        <div
          className="overflow-hidden rounded-lg bg-surface py-16 text-center border-border"
          style={{ border: "1px solid var(--border)" }}
        >
          <p
            className="text-text-muted"
            style={{
              fontFamily: "var(--font-raleway)",
              fontStyle: "italic",
              fontSize: "15px",
            }}
          >
            {t("subscriptions.empty")}
          </p>
        </div>
      ) : (
        <div
          className="overflow-hidden rounded-lg bg-surface"
          style={{ border: "1px solid var(--border)" }}
        >
          <table className="w-full">
            <thead>
              <tr style={{ borderBottom: "1px solid var(--border)", backgroundColor: "var(--background)" }}>
                <th className="w-10 px-5 py-3 text-left">
                  <Checkbox
                    checked={isAllSelected ? true : isSomeSelected ? "indeterminate" : false}
                    onCheckedChange={toggleSelectAll}
                  />
                </th>
                {selectedIds.size > 0 ? (
                  <th colSpan={5} className="py-3 pr-5 text-left">
                    <div className="flex items-center">
                      <span
                        style={{
                          fontFamily: "var(--font-dm-sans)",
                          fontSize: "13px",
                          fontWeight: 500,
                          color: "var(--foreground)",
                        }}
                      >
                        {t("subscriptions.selected").replace("{count}", String(selectedIds.size)).replace("{total}", String(subscriptions.length))}
                      </span>
                      <span className="mx-3 h-4 w-px bg-border" />
                      <button
                        onClick={() => setBatchDeleteOpen(true)}
                        className="inline-flex items-center gap-1.5 rounded-md px-2.5 py-1 text-red-600 transition-colors hover:bg-red-50"
                        style={{
                          fontFamily: "var(--font-dm-sans)",
                          fontSize: "12px",
                          fontWeight: 500,
                        }}
                      >
                        <Trash2 className="h-3.5 w-3.5" />
                        {t("subscriptions.deleteSelected")}
                      </button>
                      <button
                        onClick={clearSelection}
                        className="ml-auto text-text-muted transition-colors hover:text-text-secondary"
                        style={{
                          fontFamily: "var(--font-dm-sans)",
                          fontSize: "12px",
                        }}
                      >
                        Cancel
                      </button>
                    </div>
                  </th>
                ) : (
                  [t("subscriptions.thType"), "Channel", t("subscriptions.thTarget"), t("subscriptions.thVersionFilter"), ""].map(
                    (heading, i) => (
                      <th
                        key={i}
                        className={`py-3 text-left ${i === 4 ? "w-20 px-5" : "px-5"}`}
                        style={{
                          fontFamily: "var(--font-dm-sans)",
                          fontSize: "11px",
                          fontWeight: 500,
                          textTransform: "uppercase",
                          letterSpacing: "0.08em",
                          color: "var(--text-muted)",
                        }}
                      >
                        {heading}
                      </th>
                    )
                  )
                )}
              </tr>
            </thead>
            <tbody>
              {subscriptions.map((sub) => (
                <tr
                  key={sub.id}
                  className="transition-colors hover:bg-background"
                  style={{ borderBottom: "1px solid var(--border)" }}
                >
                  <td className="w-10 px-5 py-3">
                    <Checkbox
                      checked={selectedIds.has(sub.id)}
                      onCheckedChange={() => toggleSelect(sub.id)}
                    />
                  </td>

                  <td className="px-5 py-3">
                    <SubTypeBadge type={sub.type} />
                  </td>

                  <td
                    className="px-5 py-3 text-foreground"
                    style={{
                      fontFamily: "var(--font-dm-sans)",
                      fontSize: "13px",
                      fontWeight: 500,
                    }}
                  >
                    {getChannelName(sub.channel_id)}
                  </td>

                  <td
                    className="px-5 py-3 text-foreground"
                    style={{
                      fontFamily: "var(--font-dm-sans)",
                      fontSize: "14px",
                      fontWeight: 500,
                    }}
                  >
                    {sub.type === "source_release"
                      ? getSourceLabel(sub.source_id!)
                      : getProjectName(sub.project_id!)}
                  </td>

                  <td
                    className="px-5 py-3"
                    style={{
                      fontFamily: sub.version_filter
                        ? "'JetBrains Mono', monospace"
                        : "var(--font-dm-sans)",
                      fontSize: "13px",
                      color: sub.version_filter ? "var(--foreground)" : "var(--text-muted)",
                    }}
                  >
                    {sub.version_filter || "\u2014"}
                  </td>

                  <td className="w-20 px-5 py-3">
                    <div className="flex items-center gap-2">
                      <button
                        onClick={() => setEditingSub(sub)}
                        className="rounded p-1 text-text-muted transition-colors hover:bg-mono-bg hover:text-foreground"
                      >
                        <Pencil className="h-4 w-4" />
                      </button>
                      <button
                        onClick={() => setDeletingId(sub.id)}
                        className="rounded p-1 text-text-muted transition-colors hover:bg-red-50 hover:text-red-600"
                      >
                        <Trash2 className="h-4 w-4" />
                      </button>
                    </div>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}

      {/* Create dialog */}
      <Dialog open={createOpen} onOpenChange={setCreateOpen}>
        <DialogContent className="sm:max-w-lg">
          <DialogHeader>
            <DialogTitle>{t("subscriptions.createSubscription")}</DialogTitle>
          </DialogHeader>
          <SubscriptionForm
            title={t("subscriptions.createSubscription")}
            onSubmit={async (input) => {
              await subsApi.create(input);
            }}
            onBatchSubmit={async (input) => {
              await subsApi.batchCreate(input);
            }}
            onSuccess={() => {
              setCreateOpen(false);
              mutate("subscriptions");
              clearSelection();
            }}
            onCancel={() => setCreateOpen(false)}
          />
        </DialogContent>
      </Dialog>

      {/* Edit dialog */}
      <Dialog
        open={!!editingSub}
        onOpenChange={(open) => {
          if (!open) setEditingSub(null);
        }}
      >
        <DialogContent className="sm:max-w-lg">
          <DialogHeader>
            <DialogTitle>{t("subscriptions.editSubscription")}</DialogTitle>
          </DialogHeader>
          {editingSub && (
            <SubscriptionForm
              key={editingSub.id}
              title={t("subscriptions.editSubscription")}
              initial={editingSub}
              onSubmit={async (input) => {
                await subsApi.update(editingSub.id, input);
              }}
              onSuccess={() => {
                setEditingSub(null);
                mutate("subscriptions");
                clearSelection();
              }}
              onCancel={() => setEditingSub(null)}
            />
          )}
        </DialogContent>
      </Dialog>

      {/* Delete dialog */}
      <ConfirmDialog
        open={!!deletingId}
        onOpenChange={(open) => {
          if (!open) setDeletingId(null);
        }}
        title={t("subscriptions.deleteSubscription")}
        description={t("subscriptions.deleteSubscriptionDesc")}
        onConfirm={async () => {
          if (deletingId) {
            await subsApi.delete(deletingId);
            mutate("subscriptions");
            clearSelection();
          }
        }}
      />

      {/* Batch delete dialog */}
      <ConfirmDialog
        open={batchDeleteOpen}
        onOpenChange={setBatchDeleteOpen}
        title={t("subscriptions.deleteSubscriptions")}
        description={t("subscriptions.deleteSubscriptionsDesc").replace("{count}", String(selectedIds.size))}
        onConfirm={async () => {
          await subsApi.batchDelete({ ids: [...selectedIds] });
          mutate("subscriptions");
          clearSelection();
        }}
      />
    </div>
  );
}
