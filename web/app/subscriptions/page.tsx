"use client";

import { useState, useMemo } from "react";
import useSWR, { mutate } from "swr";
import {
  subscriptions as subsApi,
  channels as channelsApi,
  projects as projectsApi,
  sources as sourcesApi,
} from "@/lib/api/client";
import type { Source, Subscription } from "@/lib/api/types";
import { Plus, Pencil, Trash2, ChevronRight } from "lucide-react";
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
  const { t } = useTranslation();
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
  const [collapsedChannels, setCollapsedChannels] = useState<Set<string>>(new Set());

  const getChannelName = (id: string) =>
    channelsData?.data.find((c) => c.id === id)?.name ?? id;

  const getChannelType = (id: string) =>
    channelsData?.data.find((c) => c.id === id)?.type ?? "";

  const getProjectName = (id: string) =>
    projectsData?.data.find((p) => p.id === id)?.name ?? id;

  const getSourceLabel = (id: string) => {
    const source = sourcesMap?.[id];
    return source ? `${source.provider}: ${source.repository}` : id;
  };

  const subscriptions = data?.data ?? [];

  // Group subscriptions by channel_id
  const grouped = useMemo(() => {
    const map = new Map<string, Subscription[]>();
    for (const sub of subscriptions) {
      const list = map.get(sub.channel_id) ?? [];
      list.push(sub);
      map.set(sub.channel_id, list);
    }
    return map;
  }, [subscriptions]);

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

  const toggleChannel = (channelId: string) => {
    setCollapsedChannels((prev) => {
      const next = new Set(prev);
      if (next.has(channelId)) next.delete(channelId);
      else next.add(channelId);
      return next;
    });
  };

  const toggleSelectChannel = (channelId: string) => {
    const channelSubs = grouped.get(channelId) ?? [];
    const channelSubIds = channelSubs.map((s) => s.id);
    const allSelected = channelSubIds.every((id) => selectedIds.has(id));
    setSelectedIds((prev) => {
      const next = new Set(prev);
      if (allSelected) {
        channelSubIds.forEach((id) => next.delete(id));
      } else {
        channelSubIds.forEach((id) => next.add(id));
      }
      return next;
    });
  };

  const clearSelection = () => setSelectedIds(new Set());

  return (
    <div className="space-y-6">
      {/* Header */}
      <div className="flex items-center justify-between">
        <h1
          className="text-foreground"
          style={{
            fontFamily: "var(--font-fraunces)",
            fontSize: "24px",
            fontWeight: 700,
          }}
        >
          {t("subscriptions.title")}
        </h1>
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
              fontFamily: "var(--font-fraunces)",
              fontStyle: "italic",
              fontSize: "15px",
            }}
          >
            {t("subscriptions.empty")}
          </p>
        </div>
      ) : (
        <>
          {/* Select all / batch bar */}
          <div
            className="flex items-center gap-3 rounded-lg bg-surface px-5 py-2.5"
            style={{ border: "1px solid var(--border)" }}
          >
            <Checkbox
              checked={isAllSelected ? true : isSomeSelected ? "indeterminate" : false}
              onCheckedChange={toggleSelectAll}
            />
            <span
              className="text-text-secondary"
              style={{
                fontFamily: "var(--font-dm-sans)",
                fontSize: "13px",
              }}
            >
              {selectedIds.size > 0
                ? t("subscriptions.selected").replace("{count}", String(selectedIds.size)).replace("{total}", String(subscriptions.length))
                : subscriptions.length === 1
                  ? t("subscriptions.countLabel").replace("{count}", String(subscriptions.length))
                  : t("subscriptions.countLabelPlural").replace("{count}", String(subscriptions.length))}
            </span>
            {selectedIds.size > 0 && (
              <button
                onClick={() => setBatchDeleteOpen(true)}
                className="ml-auto inline-flex items-center gap-1.5 rounded-md px-3 py-1.5 transition-colors hover:opacity-90"
                style={{
                  backgroundColor: "#dc2626",
                  color: "#ffffff",
                  fontFamily: "var(--font-dm-sans)",
                  fontSize: "13px",
                  fontWeight: 500,
                }}
              >
                <Trash2 className="h-3.5 w-3.5" />
                {t("subscriptions.deleteSelected")}
              </button>
            )}
          </div>

          {/* Grouped by channel */}
          <div className="space-y-3">
            {Array.from(grouped.entries()).map(([channelId, channelSubs]) => {
              const isCollapsed = collapsedChannels.has(channelId);
              const channelSubIds = channelSubs.map((s) => s.id);
              const allChannelSelected = channelSubIds.every((id) => selectedIds.has(id));
              const someChannelSelected = channelSubIds.some((id) => selectedIds.has(id)) && !allChannelSelected;
              const channelType = getChannelType(channelId);

              return (
                <div
                  key={channelId}
                  className="overflow-hidden rounded-lg bg-surface"
                  style={{ border: "1px solid var(--border)" }}
                >
                  {/* Channel header */}
                  <div
                    className="flex items-center gap-3 px-5 py-3 cursor-pointer select-none bg-background"
                    style={{ borderBottom: isCollapsed ? "none" : "1px solid var(--border)" }}
                    onClick={() => toggleChannel(channelId)}
                  >
                    <div onClick={(e) => e.stopPropagation()}>
                      <Checkbox
                        checked={allChannelSelected ? true : someChannelSelected ? "indeterminate" : false}
                        onCheckedChange={() => toggleSelectChannel(channelId)}
                      />
                    </div>
                    <ChevronRight
                      className="h-4 w-4 transition-transform text-text-muted"
                      style={{
                        transform: isCollapsed ? "rotate(0deg)" : "rotate(90deg)",
                      }}
                    />
                    <span
                      className="text-foreground"
                      style={{
                        fontFamily: "var(--font-dm-sans)",
                        fontSize: "14px",
                        fontWeight: 600,
                      }}
                    >
                      {getChannelName(channelId)}
                    </span>
                    {channelType && (
                      <span
                        className="rounded-full px-2 py-0.5 text-text-secondary bg-mono-bg"
                        style={{
                          fontFamily: "var(--font-dm-sans)",
                          fontSize: "11px",
                          fontWeight: 500,
                        }}
                      >
                        {channelType}
                      </span>
                    )}
                    <span
                      className="text-text-muted"
                      style={{
                        fontFamily: "var(--font-dm-sans)",
                        fontSize: "12px",
                        marginLeft: "auto",
                      }}
                    >
                      {channelSubs.length === 1
                        ? t("subscriptions.channelSubscription").replace("{count}", String(channelSubs.length))
                        : t("subscriptions.channelSubscriptionPlural").replace("{count}", String(channelSubs.length))}
                    </span>
                  </div>

                  {/* Subscription rows */}
                  {!isCollapsed && (
                    <table className="w-full">
                      <thead>
                        <tr>
                          {["", t("subscriptions.thType"), t("subscriptions.thTarget"), t("subscriptions.thVersionFilter"), ""].map(
                            (heading, i) => (
                              <th
                                key={i}
                                className={`py-2 text-left ${i === 0 ? "w-10 px-5" : i === 4 ? "w-20 px-5" : "px-5"}`}
                                style={{
                                  fontFamily: "var(--font-dm-sans)",
                                  fontSize: "11px",
                                  fontWeight: 500,
                                  textTransform: "uppercase",
                                  letterSpacing: "0.08em",
                                  color: "var(--text-muted)",
                                  borderBottom: "1px solid var(--border)",
                                }}
                              >
                                {heading}
                              </th>
                            )
                          )}
                        </tr>
                      </thead>
                      <tbody>
                        {channelSubs.map((sub) => (
                          <tr
                            key={sub.id}
                            className="transition-colors hover:bg-background"
                            style={{ borderBottom: "1px solid var(--border)" }}
                          >
                            {/* Checkbox */}
                            <td className="w-10 px-5 py-3">
                              <Checkbox
                                checked={selectedIds.has(sub.id)}
                                onCheckedChange={() => toggleSelect(sub.id)}
                              />
                            </td>

                            {/* Type */}
                            <td className="px-5 py-3">
                              <SubTypeBadge type={sub.type} />
                            </td>

                            {/* Target */}
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

                            {/* Version Filter */}
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

                            {/* Actions */}
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
                  )}
                </div>
              );
            })}
          </div>
        </>
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
