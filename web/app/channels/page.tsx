"use client";

import { useState } from "react";
import useSWR, { mutate } from "swr";
import { channels as channelsApi } from "@/lib/api/client";
import { Plus, Pencil, Trash2 } from "lucide-react";
import { FaSlack, FaDiscord } from "react-icons/fa";
import { TbWebhook } from "react-icons/tb";
import type { IconType } from "react-icons";
import { Dialog, DialogContent, DialogHeader, DialogTitle } from "@/components/ui/dialog";
import { ConfirmDialog } from "@/components/ui/confirm-dialog";
import { ChannelForm } from "@/components/channels/channel-form";
import type { NotificationChannel } from "@/lib/api/types";

const TYPE_STYLES: Record<string, { bg: string; text: string; icon: IconType }> = {
  slack: { bg: "#4A154B", text: "#ffffff", icon: FaSlack },
  discord: { bg: "#5865F2", text: "#ffffff", icon: FaDiscord },
  webhook: { bg: "#1a1a1a", text: "#ffffff", icon: TbWebhook },
};

function TypeBadge({ type }: { type: string }) {
  const style = TYPE_STYLES[type.toLowerCase()];
  const colors = style ?? { bg: "#6b7280", text: "#ffffff" };
  const Icon = style?.icon;
  return (
    <span
      className="inline-flex items-center gap-1.5 rounded-full px-2.5 py-0.5"
      style={{
        backgroundColor: colors.bg,
        color: colors.text,
        fontFamily: "var(--font-dm-sans)",
        fontSize: "12px",
        fontWeight: 500,
        lineHeight: "16px",
      }}
    >
      {Icon && <Icon size={12} />}
      {type}
    </span>
  );
}

export default function ChannelsPage() {
  const { data, isLoading } = useSWR("channels", () => channelsApi.list());

  const [createOpen, setCreateOpen] = useState(false);
  const [editingChannel, setEditingChannel] = useState<NotificationChannel | null>(null);
  const [deletingId, setDeletingId] = useState<string | null>(null);

  const channels = data?.data ?? [];

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
          Channels
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
          New Channel
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
        ) : channels.length === 0 ? (
          <div className="py-16 text-center">
            <p
              style={{
                fontFamily: "var(--font-fraunces)",
                fontStyle: "italic",
                fontSize: "15px",
                color: "#9ca3af",
              }}
            >
              No notification channels configured yet
            </p>
          </div>
        ) : (
          <table className="w-full">
            <thead>
              <tr style={{ backgroundColor: "#fafaf9" }}>
                {["Name", "Type", "Config", "Created", "Actions"].map(
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
              {channels.map((ch) => (
                <tr
                  key={ch.id}
                  className="transition-colors hover:bg-[#fafaf9]"
                  style={{ borderBottom: "1px solid #e8e8e5" }}
                >
                  {/* Name */}
                  <td
                    className="px-5 py-3.5"
                    style={{
                      fontFamily: "var(--font-dm-sans)",
                      fontSize: "14px",
                      fontWeight: 500,
                      color: "#111113",
                    }}
                  >
                    {ch.name}
                  </td>

                  {/* Type */}
                  <td className="px-5 py-3.5">
                    <TypeBadge type={ch.type} />
                  </td>

                  {/* Config */}
                  <td
                    className="max-w-xs truncate px-5 py-3.5"
                    style={{
                      fontFamily: "'JetBrains Mono', monospace",
                      fontSize: "12px",
                      color: "#6b7280",
                    }}
                  >
                    {Object.entries(ch.config)
                      .map(([k, v]) => `${k}: ${String(v)}`)
                      .join(", ")}
                  </td>

                  {/* Created */}
                  <td
                    className="px-5 py-3.5"
                    style={{
                      fontFamily: "var(--font-dm-sans)",
                      fontSize: "13px",
                      color: "#6b7280",
                    }}
                  >
                    {new Date(ch.created_at).toLocaleDateString()}
                  </td>

                  {/* Actions */}
                  <td className="px-5 py-3.5">
                    <div className="flex items-center gap-2">
                      <button
                        onClick={() => setEditingChannel(ch)}
                        className="rounded p-1 text-[#9ca3af] transition-colors hover:bg-[#f3f3f1] hover:text-[#111113]"
                      >
                        <Pencil className="h-4 w-4" />
                      </button>
                      <button
                        onClick={() => setDeletingId(ch.id)}
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
        )}
      </div>

      {/* Create dialog */}
      <Dialog open={createOpen} onOpenChange={setCreateOpen}>
        <DialogContent className="sm:max-w-lg">
          <DialogHeader><DialogTitle>Add Channel</DialogTitle></DialogHeader>
          <ChannelForm
            title="Add Channel"
            onSubmit={async (input) => { await channelsApi.create(input); }}
            onSuccess={() => { setCreateOpen(false); mutate("channels"); }}
            onCancel={() => setCreateOpen(false)}
          />
        </DialogContent>
      </Dialog>

      {/* Edit dialog */}
      <Dialog open={!!editingChannel} onOpenChange={(open) => { if (!open) setEditingChannel(null); }}>
        <DialogContent className="sm:max-w-lg">
          <DialogHeader><DialogTitle>Edit Channel</DialogTitle></DialogHeader>
          {editingChannel && (
            <ChannelForm
              key={editingChannel.id}
              title="Edit Channel"
              initial={editingChannel}
              onSubmit={async (input) => { await channelsApi.update(editingChannel.id, input); }}
              onSuccess={() => { setEditingChannel(null); mutate("channels"); }}
              onCancel={() => setEditingChannel(null)}
            />
          )}
        </DialogContent>
      </Dialog>

      {/* Delete dialog */}
      <ConfirmDialog
        open={!!deletingId}
        onOpenChange={(open) => { if (!open) setDeletingId(null); }}
        title="Delete Channel"
        description="This will permanently delete this notification channel. This cannot be undone."
        onConfirm={async () => { if (deletingId) { await channelsApi.delete(deletingId); mutate("channels"); } }}
      />
    </div>
  );
}
