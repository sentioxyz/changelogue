"use client";

import { useState } from "react";
import useSWR, { mutate } from "swr";
import { channels as channelsApi } from "@/lib/api/client";
import { Plus, Pencil, Trash2, Zap, Loader2 } from "lucide-react";
import { FaSlack, FaDiscord } from "react-icons/fa";
import { TbWebhook } from "react-icons/tb";
import { HiOutlineMail } from "react-icons/hi";
import type { IconType } from "react-icons";
import { Dialog, DialogContent, DialogHeader, DialogTitle } from "@/components/ui/dialog";
import { ConfirmDialog } from "@/components/ui/confirm-dialog";
import { ChannelForm } from "@/components/channels/channel-form";
import { useToast, ToastContainer } from "@/components/ui/toast";
import { useTranslation } from "@/lib/i18n/context";
import type { NotificationChannel } from "@/lib/api/types";

const TYPE_STYLES: Record<string, { bg: string; text: string; icon: IconType }> = {
  slack: { bg: "#4A154B", text: "#ffffff", icon: FaSlack },
  discord: { bg: "#5865F2", text: "#ffffff", icon: FaDiscord },
  webhook: { bg: "#1a1a1a", text: "#ffffff", icon: TbWebhook },
  email: { bg: "#2563EB", text: "#ffffff", icon: HiOutlineMail },
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
  const { t } = useTranslation();
  const { data, isLoading } = useSWR("channels", () => channelsApi.list());

  const [createOpen, setCreateOpen] = useState(false);
  const [editingChannel, setEditingChannel] = useState<NotificationChannel | null>(null);
  const [deletingId, setDeletingId] = useState<string | null>(null);
  const [testingId, setTestingId] = useState<string | null>(null);
  const { toasts, show: showToast, dismiss: dismissToast } = useToast();

  const channels = data?.data ?? [];

  const handleTest = async (ch: NotificationChannel) => {
    setTestingId(ch.id);
    try {
      await channelsApi.test(ch.id);
      showToast(`${t("channels.testSent")} "${ch.name}"`, "success");
    } catch (err) {
      showToast(
        err instanceof Error ? err.message : t("channels.testFailed"),
        "error"
      );
    } finally {
      setTestingId(null);
    }
  };

  const tableHeaders = [
    t("channels.col.name"),
    t("channels.col.type"),
    t("channels.col.config"),
    t("channels.col.created"),
    t("channels.col.actions"),
  ];

  return (
    <div className="space-y-6">
      {/* Header */}
      <div className="flex items-center justify-between">
        <div>
          <h1
            style={{
              fontFamily: "var(--font-raleway)",
              fontSize: "24px",
              fontWeight: 700,
              color: "var(--foreground)",
            }}
          >
            {t("channels.title")}
          </h1>
          <p className="mt-1 text-[13px] text-text-secondary" style={{ fontFamily: "var(--font-dm-sans)" }}>
            {t("channels.description")}
          </p>
        </div>
        <button
          onClick={() => setCreateOpen(true)}
          className="inline-flex items-center gap-1.5 rounded-md px-3.5 py-2 transition-colors hover:opacity-90"
          style={{
            backgroundColor: "var(--beacon-accent)",
            color: "#ffffff",
            fontFamily: "var(--font-dm-sans)",
            fontSize: "13px",
            fontWeight: 500,
          }}
        >
          <Plus className="h-4 w-4" />
          {t("channels.newChannel")}
        </button>
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
            {t("channels.loading")}
          </div>
        ) : channels.length === 0 ? (
          <div className="py-16 text-center">
            <p
              style={{
                fontFamily: "var(--font-raleway)",
                fontStyle: "italic",
                fontSize: "15px",
                color: "var(--text-muted)",
              }}
            >
              {t("channels.empty")}
            </p>
          </div>
        ) : (
          <table className="w-full">
            <thead>
              <tr style={{ backgroundColor: "var(--background)" }}>
                {tableHeaders.map(
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
              {channels.map((ch) => (
                <tr
                  key={ch.id}
                  className="transition-colors hover:bg-background"
                  style={{ borderBottom: "1px solid var(--border)" }}
                >
                  {/* Name */}
                  <td
                    className="px-5 py-3.5"
                    style={{
                      fontFamily: "var(--font-dm-sans)",
                      fontSize: "14px",
                      fontWeight: 500,
                      color: "var(--foreground)",
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
                      color: "var(--text-secondary)",
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
                      color: "var(--text-secondary)",
                    }}
                  >
                    {new Date(ch.created_at).toLocaleDateString()}
                  </td>

                  {/* Actions */}
                  <td className="px-5 py-3.5">
                    <div className="flex items-center gap-2">
                      <button
                        onClick={() => handleTest(ch)}
                        disabled={testingId === ch.id}
                        className="rounded p-1 text-text-muted transition-colors hover:bg-amber-50 hover:text-amber-600 disabled:opacity-50"
                        title={t("channels.sendTest")}
                      >
                        {testingId === ch.id ? (
                          <Loader2 className="h-4 w-4 animate-spin" />
                        ) : (
                          <Zap className="h-4 w-4" />
                        )}
                      </button>
                      <button
                        onClick={() => setEditingChannel(ch)}
                        className="rounded p-1 text-text-muted transition-colors hover:bg-mono-bg hover:text-foreground"
                      >
                        <Pencil className="h-4 w-4" />
                      </button>
                      <button
                        onClick={() => setDeletingId(ch.id)}
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

      {/* Create dialog */}
      <Dialog open={createOpen} onOpenChange={setCreateOpen}>
        <DialogContent className="sm:max-w-lg">
          <DialogHeader><DialogTitle>{t("channels.addChannel")}</DialogTitle></DialogHeader>
          <ChannelForm
            title={t("channels.addChannel")}
            onSubmit={async (input) => { await channelsApi.create(input); }}
            onSuccess={() => { setCreateOpen(false); mutate("channels"); }}
            onCancel={() => setCreateOpen(false)}
          />
        </DialogContent>
      </Dialog>

      {/* Edit dialog */}
      <Dialog open={!!editingChannel} onOpenChange={(open) => { if (!open) setEditingChannel(null); }}>
        <DialogContent className="sm:max-w-lg">
          <DialogHeader><DialogTitle>{t("channels.editChannel")}</DialogTitle></DialogHeader>
          {editingChannel && (
            <ChannelForm
              key={editingChannel.id}
              title={t("channels.editChannel")}
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
        title={t("channels.deleteChannel")}
        description={t("channels.deleteConfirm")}
        onConfirm={async () => { if (deletingId) { await channelsApi.delete(deletingId); mutate("channels"); } }}
      />

      {/* Toast notifications */}
      <ToastContainer toasts={toasts} onDismiss={dismissToast} />
    </div>
  );
}
