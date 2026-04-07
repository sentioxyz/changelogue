"use client";

import { useState } from "react";
import useSWR, { mutate } from "swr";
import { apiKeys as apiKeysApi } from "@/lib/api/client";
import { Plus, Trash2, Copy, Check } from "lucide-react";
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import { ConfirmDialog } from "@/components/ui/confirm-dialog";
import { useTranslation } from "@/lib/i18n/context";

export default function ApiKeysPage() {
  const { t } = useTranslation();
  const { data, isLoading } = useSWR("api-keys", () => apiKeysApi.list());


  const [createOpen, setCreateOpen] = useState(false);
  const [createdKey, setCreatedKey] = useState<string | null>(null);
  const [deletingId, setDeletingId] = useState<string | null>(null);
  const [name, setName] = useState("");
  const [creating, setCreating] = useState(false);
  const [copied, setCopied] = useState(false);

  const keys = data?.data ?? [];

  const handleCreate = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!name.trim()) return;
    setCreating(true);
    try {
      const res = await apiKeysApi.create({ name: name.trim() });
      setCreatedKey(res.data.key ?? null);
      mutate("api-keys");
    } catch (err) {
      console.error("Failed to create API key:", err);
    } finally {
      setCreating(false);
    }
  };

  const handleCopy = async () => {
    if (!createdKey) return;
    await navigator.clipboard.writeText(createdKey);
    setCopied(true);
    setTimeout(() => setCopied(false), 2000);
  };

  const handleCloseCreate = () => {
    setCreateOpen(false);
    setCreatedKey(null);
    setName("");
    setCopied(false);
  };

  const tableHeaders = [
    t("apiKeys.col.name"),
    t("apiKeys.col.prefix"),
    t("apiKeys.col.lastUsed"),
    t("apiKeys.col.created"),
    t("apiKeys.col.actions"),
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
            {t("apiKeys.title")}
          </h1>
          <p
            className="mt-1 text-[13px] text-text-secondary"
            style={{ fontFamily: "var(--font-dm-sans)" }}
          >
            {t("apiKeys.description")}
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
          {t("apiKeys.create")}
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
            {t("apiKeys.loading")}
          </div>
        ) : keys.length === 0 ? (
          <div className="py-16 text-center">
            <p
              style={{
                fontFamily: "var(--font-raleway)",
                fontStyle: "italic",
                fontSize: "15px",
                color: "var(--text-muted)",
              }}
            >
              {t("apiKeys.empty")}
            </p>
          </div>
        ) : (
          <table className="w-full">
            <thead>
              <tr style={{ backgroundColor: "var(--background)" }}>
                {tableHeaders.map((heading) => (
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
                ))}
              </tr>
            </thead>
            <tbody>
              {keys.map((k) => (
                <tr
                  key={k.id}
                  className="transition-colors hover:bg-background"
                  style={{ borderBottom: "1px solid var(--border)" }}
                >
                  <td
                    className="px-5 py-3.5"
                    style={{
                      fontFamily: "var(--font-dm-sans)",
                      fontSize: "14px",
                      fontWeight: 500,
                      color: "var(--foreground)",
                    }}
                  >
                    {k.name}
                  </td>
                  <td
                    className="px-5 py-3.5"
                    style={{
                      fontFamily: "'JetBrains Mono', monospace",
                      fontSize: "12px",
                      color: "var(--text-secondary)",
                    }}
                  >
                    {k.prefix}...
                  </td>
                  <td
                    className="px-5 py-3.5"
                    style={{
                      fontFamily: "var(--font-dm-sans)",
                      fontSize: "13px",
                      color: "var(--text-secondary)",
                    }}
                  >
                    {k.last_used_at
                      ? new Date(k.last_used_at).toLocaleDateString()
                      : t("apiKeys.never")}
                  </td>
                  <td
                    className="px-5 py-3.5"
                    style={{
                      fontFamily: "var(--font-dm-sans)",
                      fontSize: "13px",
                      color: "var(--text-secondary)",
                    }}
                  >
                    {new Date(k.created_at).toLocaleDateString()}
                  </td>
                  <td className="px-5 py-3.5">
                    <button
                      onClick={() => setDeletingId(k.id)}
                      className="rounded p-1 text-text-muted transition-colors hover:bg-red-50 hover:text-red-600"
                    >
                      <Trash2 className="h-4 w-4" />
                    </button>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        )}
      </div>

      {/* Create dialog */}
      <Dialog open={createOpen} onOpenChange={(open) => { if (!open) handleCloseCreate(); else setCreateOpen(true); }}>
        <DialogContent className="sm:max-w-md">
          <DialogHeader>
            <DialogTitle>
              {createdKey ? t("apiKeys.createdTitle") : t("apiKeys.createTitle")}
            </DialogTitle>
          </DialogHeader>

          {createdKey ? (
            <div className="space-y-4">
              <p
                className="text-sm text-amber-600"
                style={{ fontFamily: "var(--font-dm-sans)" }}
              >
                {t("apiKeys.createdWarning")}
              </p>
              <div className="flex items-start gap-2 rounded-md border border-border bg-background p-3">
                <code
                  className="min-w-0 flex-1 break-all text-sm"
                  style={{ fontFamily: "'JetBrains Mono', monospace" }}
                >
                  {createdKey}
                </code>
                <button
                  onClick={handleCopy}
                  className="shrink-0 rounded p-1.5 transition-colors hover:bg-accent"
                >
                  {copied ? (
                    <Check className="h-4 w-4 text-green-600" />
                  ) : (
                    <Copy className="h-4 w-4 text-text-muted" />
                  )}
                </button>
              </div>
              <button
                onClick={handleCloseCreate}
                className="w-full rounded-md border border-border px-4 py-2 text-sm font-medium transition-colors hover:bg-accent"
                style={{ fontFamily: "var(--font-dm-sans)" }}
              >
                {t("apiKeys.done")}
              </button>
            </div>
          ) : (
            <form onSubmit={handleCreate} className="space-y-4">
              <div className="space-y-2">
                <label
                  className="text-sm font-medium"
                  style={{ fontFamily: "var(--font-dm-sans)" }}
                >
                  {t("apiKeys.nameLabel")}
                </label>
                <input
                  type="text"
                  value={name}
                  onChange={(e) => setName(e.target.value)}
                  placeholder={t("apiKeys.namePlaceholder")}
                  className="w-full rounded-md border border-border bg-background px-3 py-2 text-sm outline-none transition-colors focus:border-[var(--beacon-accent)]"
                  style={{ fontFamily: "var(--font-dm-sans)" }}
                  autoFocus
                />
              </div>
              <div className="flex justify-end gap-2">
                <button
                  type="button"
                  onClick={handleCloseCreate}
                  className="rounded-md border border-border px-4 py-2 text-sm font-medium transition-colors hover:bg-accent"
                  style={{ fontFamily: "var(--font-dm-sans)" }}
                >
                  {t("apiKeys.cancel")}
                </button>
                <button
                  type="submit"
                  disabled={creating || !name.trim()}
                  className="rounded-md px-4 py-2 text-sm font-medium text-white transition-colors hover:opacity-90 disabled:opacity-50"
                  style={{
                    backgroundColor: "var(--beacon-accent)",
                    fontFamily: "var(--font-dm-sans)",
                  }}
                >
                  {creating ? t("apiKeys.creating") : t("apiKeys.create")}
                </button>
              </div>
            </form>
          )}
        </DialogContent>
      </Dialog>

      {/* Delete dialog */}
      <ConfirmDialog
        open={!!deletingId}
        onOpenChange={(open) => {
          if (!open) setDeletingId(null);
        }}
        title={t("apiKeys.deleteKey")}
        description={t("apiKeys.deleteConfirm")}
        onConfirm={async () => {
          if (deletingId) {
            await apiKeysApi.delete(deletingId);
            mutate("api-keys");
          }
        }}
      />
    </div>
  );
}
