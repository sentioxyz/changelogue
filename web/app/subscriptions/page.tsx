"use client";

import useSWR, { mutate } from "swr";
import Link from "next/link";
import {
  subscriptions as subsApi,
  channels as channelsApi,
} from "@/lib/api/client";
import { Plus, Pencil, Trash2 } from "lucide-react";

const SUB_TYPE_COLORS: Record<string, { bg: string; text: string }> = {
  source: { bg: "#1a1a1a", text: "#ffffff" },
  project: { bg: "#2563eb", text: "#ffffff" },
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

  const getChannelName = (id: string) =>
    channelsData?.data.find((c) => c.id === id)?.name ?? id;

  const handleDelete = async (id: string) => {
    if (!confirm("Delete this subscription? This cannot be undone.")) return;
    await subsApi.delete(id);
    mutate("subscriptions");
  };

  const subscriptions = data?.data ?? [];

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
        <Link href="/subscriptions/new">
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
            New Subscription
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
          <table className="w-full">
            <thead>
              <tr style={{ backgroundColor: "#fafaf9" }}>
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
                    {sub.type === "source" ? sub.source_id : sub.project_id}
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
                      <Link
                        href={`/subscriptions/${sub.id}/edit`}
                        className="rounded p-1 text-[#9ca3af] transition-colors hover:bg-[#f3f3f1] hover:text-[#111113]"
                      >
                        <Pencil className="h-4 w-4" />
                      </Link>
                      <button
                        onClick={() => handleDelete(sub.id)}
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
