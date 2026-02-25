"use client";

import useSWR, { mutate } from "swr";
import Link from "next/link";
import { channels as channelsApi } from "@/lib/api/client";
import { Plus, Pencil, Trash2 } from "lucide-react";

const TYPE_COLORS: Record<string, { bg: string; text: string }> = {
  slack: { bg: "#4A154B", text: "#ffffff" },
  discord: { bg: "#5865F2", text: "#ffffff" },
  webhook: { bg: "#1a1a1a", text: "#ffffff" },
};

function TypeBadge({ type }: { type: string }) {
  const colors = TYPE_COLORS[type.toLowerCase()] ?? {
    bg: "#6b7280",
    text: "#ffffff",
  };
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

export default function ChannelsPage() {
  const { data, isLoading } = useSWR("channels", () => channelsApi.list());

  const handleDelete = async (id: string) => {
    if (!confirm("Delete this channel? This cannot be undone.")) return;
    await channelsApi.delete(id);
    mutate("channels");
  };

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
        <Link href="/channels/new">
          <button
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
        </Link>
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
                        letterSpacing: "0.05em",
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
                      fontFamily: "var(--font-jetbrains-mono)",
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
                      <Link
                        href={`/channels/${ch.id}/edit`}
                        className="rounded p-1 text-[#9ca3af] transition-colors hover:bg-[#f3f3f1] hover:text-[#111113]"
                      >
                        <Pencil className="h-4 w-4" />
                      </Link>
                      <button
                        onClick={() => handleDelete(ch.id)}
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
    </div>
  );
}
