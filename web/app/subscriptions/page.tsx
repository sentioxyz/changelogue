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
import { Plus, Pencil, Trash2 } from "lucide-react";
import { Checkbox } from "@/components/ui/checkbox";
import { Dialog, DialogContent, DialogHeader, DialogTitle } from "@/components/ui/dialog";
import { ConfirmDialog } from "@/components/ui/confirm-dialog";
import { SubscriptionForm } from "@/components/subscriptions/subscription-form";

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

  const subscriptions = data?.data ?? [];

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
        <h1
          style={{
            fontFamily: "var(--font-fraunces)",
            fontSize: "24px",
            fontWeight: 700,
            color: "#111113",
          }}
        >
          Subscriptions
        </h1>
        <button
          onClick={() => setCreateOpen(true)}
          className="inline-flex items-center gap-1.5 rounded-md px-3.5 py-2 transition-colors hover:opacity-90"
          style={{
            backgroundColor: "#e8601a",
            color: "#ffffff",
            fontFamily: "var(--font-dm-sans)",
            fontSize: "13px",
            fontWeight: 500,
          }}
        >
          <Plus className="h-4 w-4" />
          New Subscription
        </button>
      </div>

      {/* Table card */}
      <div
        className="overflow-hidden rounded-lg bg-white"
        style={{ border: "1px solid #e8e8e5" }}
      >
        {isLoading ? (
          <div
            className="py-16 text-center"
            style={{
              fontFamily: "var(--font-dm-sans)",
              fontSize: "13px",
              color: "#6b7280",
            }}
          >
            Loading...
          </div>
        ) : subscriptions.length === 0 ? (
          <div className="py-16 text-center">
            <p
              style={{
                fontFamily: "var(--font-fraunces)",
                fontStyle: "italic",
                fontSize: "15px",
                color: "#9ca3af",
              }}
            >
              No subscriptions configured yet
            </p>
          </div>
        ) : (
          <>
            <table className="w-full">
            <thead>
              <tr style={{ backgroundColor: "#fafaf9" }}>
                <th
                  className="w-10 px-5 py-3"
                  style={{ borderBottom: "1px solid #e8e8e5" }}
                >
                  <Checkbox
                    checked={isAllSelected ? true : isSomeSelected ? "indeterminate" : false}
                    onCheckedChange={toggleSelectAll}
                  />
                </th>
                {["Type", "Target", "Channel", "Version Filter", "Actions"].map(
                  (heading) => (
                    <th
                      key={heading}
                      className="px-5 py-3 text-left"
                      style={{
                        fontFamily: "var(--font-dm-sans)",
                        fontSize: "11px",
                        fontWeight: 500,
                        textTransform: "uppercase",
                        letterSpacing: "0.08em",
                        color: "#9ca3af",
                        borderBottom: "1px solid #e8e8e5",
                      }}
                    >
                      {heading}
                    </th>
                  )
                )}
              </tr>
            </thead>
            <tbody>
              {subscriptions.map((sub) => (
                <tr
                  key={sub.id}
                  className="transition-colors hover:bg-[#fafaf9]"
                  style={{ borderBottom: "1px solid #e8e8e5" }}
                >
                  {/* Checkbox */}
                  <td className="w-10 px-5 py-3.5">
                    <Checkbox
                      checked={selectedIds.has(sub.id)}
                      onCheckedChange={() => toggleSelect(sub.id)}
                    />
                  </td>

                  {/* Type */}
                  <td className="px-5 py-3.5">
                    <SubTypeBadge type={sub.type} />
                  </td>

                  {/* Target */}
                  <td
                    className="px-5 py-3.5"
                    style={{
                      fontFamily: "var(--font-dm-sans)",
                      fontSize: "14px",
                      fontWeight: 500,
                      color: "#111113",
                    }}
                  >
                    {sub.type === "source_release"
                      ? getSourceLabel(sub.source_id!)
                      : getProjectName(sub.project_id!)}
                  </td>

                  {/* Channel */}
                  <td
                    className="px-5 py-3.5"
                    style={{
                      fontFamily: "var(--font-dm-sans)",
                      fontSize: "14px",
                      color: "#111113",
                    }}
                  >
                    {getChannelName(sub.channel_id)}
                  </td>

                  {/* Version Filter */}
                  <td
                    className="px-5 py-3.5"
                    style={{
                      fontFamily: sub.version_filter
                        ? "'JetBrains Mono', monospace"
                        : "var(--font-dm-sans)",
                      fontSize: "13px",
                      color: sub.version_filter ? "#111113" : "#9ca3af",
                    }}
                  >
                    {sub.version_filter || "\u2014"}
                  </td>

                  {/* Actions */}
                  <td className="px-5 py-3.5">
                    <div className="flex items-center gap-2">
                      <button
                        onClick={() => setEditingSub(sub)}
                        className="rounded p-1 text-[#9ca3af] transition-colors hover:bg-[#f3f3f1] hover:text-[#111113]"
                      >
                        <Pencil className="h-4 w-4" />
                      </button>
                      <button
                        onClick={() => setDeletingId(sub.id)}
                        className="rounded p-1 text-[#9ca3af] transition-colors hover:bg-red-50 hover:text-red-600"
                      >
                        <Trash2 className="h-4 w-4" />
                      </button>
                    </div>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
            {selectedIds.size > 0 && (
              <div
                className="flex items-center justify-between px-5 py-2.5"
                style={{
                  backgroundColor: "#fef2f2",
                  borderTop: "1px solid #fecaca",
                }}
              >
                <span
                  style={{
                    fontFamily: "var(--font-dm-sans)",
                    fontSize: "13px",
                    fontWeight: 500,
                    color: "#111113",
                  }}
                >
                  {selectedIds.size} selected
                </span>
                <button
                  onClick={() => setBatchDeleteOpen(true)}
                  className="inline-flex items-center gap-1.5 rounded-md px-3 py-1.5 transition-colors hover:opacity-90"
                  style={{
                    backgroundColor: "#dc2626",
                    color: "#ffffff",
                    fontFamily: "var(--font-dm-sans)",
                    fontSize: "13px",
                    fontWeight: 500,
                  }}
                >
                  <Trash2 className="h-3.5 w-3.5" />
                  Delete Selected
                </button>
              </div>
            )}
          </>
        )}
      </div>

      {/* Create dialog */}
      <Dialog open={createOpen} onOpenChange={setCreateOpen}>
        <DialogContent className="sm:max-w-lg">
          <DialogHeader>
            <DialogTitle>Create Subscription</DialogTitle>
          </DialogHeader>
          <SubscriptionForm
            title="Create Subscription"
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
            <DialogTitle>Edit Subscription</DialogTitle>
          </DialogHeader>
          {editingSub && (
            <SubscriptionForm
              key={editingSub.id}
              title="Edit Subscription"
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
        title="Delete Subscription"
        description="This will permanently delete this subscription. This cannot be undone."
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
        title="Delete Subscriptions"
        description={`This will permanently delete ${selectedIds.size} subscription${selectedIds.size === 1 ? "" : "s"}. This cannot be undone.`}
        onConfirm={async () => {
          await subsApi.batchDelete({ ids: [...selectedIds] });
          mutate("subscriptions");
          clearSelection();
        }}
      />
    </div>
  );
}
