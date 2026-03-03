// web/components/dashboard/release-trend-chart.tsx
"use client";

import { useState } from "react";
import useSWR from "swr";
import { system } from "@/lib/api/client";
import {
  BarChart,
  Bar,
  XAxis,
  YAxis,
  Tooltip,
  ResponsiveContainer,
  CartesianGrid,
} from "recharts";
import type { TrendBucket } from "@/lib/api/types";

type Granularity = "daily" | "weekly" | "monthly";

const RANGE_OPTIONS = [
  { label: "7d", days: 7, granularity: "daily" as Granularity },
  { label: "30d", days: 30, granularity: "daily" as Granularity },
  { label: "90d", days: 90, granularity: "weekly" as Granularity },
  { label: "1y", days: 365, granularity: "monthly" as Granularity },
];

function formatPeriod(period: string, granularity: Granularity): string {
  const d = new Date(period + "T00:00:00");
  switch (granularity) {
    case "daily":
      return d.toLocaleDateString("en-US", { month: "short", day: "numeric" });
    case "weekly":
      return d.toLocaleDateString("en-US", { month: "short", day: "numeric" });
    case "monthly":
      return d.toLocaleDateString("en-US", { month: "short", year: "2-digit" });
  }
}

export function ReleaseTrendChart() {
  const [rangeIdx, setRangeIdx] = useState(0);
  const range = RANGE_OPTIONS[rangeIdx];

  const { data, isLoading } = useSWR(
    `trend-${range.granularity}-${range.days}`,
    () => system.trend(range.granularity, range.days),
    { refreshInterval: 30_000 }
  );

  const buckets: TrendBucket[] = data?.data?.buckets ?? [];

  const chartData = buckets.map((b) => ({
    ...b,
    label: formatPeriod(b.period, range.granularity),
  }));

  return (
    <div
      className="flex flex-col rounded-lg bg-white px-5 py-4"
      style={{ border: "1px solid #e8e8e5", height: "336px" }}
    >
      <div className="flex items-center justify-between">
        <p
          className="text-xs uppercase tracking-[0.08em]"
          style={{
            fontFamily: "var(--font-dm-sans)",
            fontSize: "12px",
            color: "#6b7280",
          }}
        >
          Release Trend
        </p>
        <div className="flex gap-1">
          {RANGE_OPTIONS.map((opt, idx) => (
            <button
              key={opt.label}
              onClick={() => setRangeIdx(idx)}
              className="rounded px-2 py-0.5 text-xs font-medium transition-colors"
              style={{
                fontFamily: "var(--font-dm-sans)",
                fontSize: "11px",
                backgroundColor:
                  rangeIdx === idx ? "#111113" : "transparent",
                color: rangeIdx === idx ? "#ffffff" : "#6b7280",
                border:
                  rangeIdx === idx
                    ? "1px solid #111113"
                    : "1px solid #e8e8e5",
              }}
            >
              {opt.label}
            </button>
          ))}
        </div>
      </div>

      <div className="relative mt-3 flex-1" style={{ minHeight: "100px" }}>
        {isLoading ? (
          <div
            className="flex h-full items-center justify-center"
            style={{
              fontFamily: "var(--font-dm-sans)",
              fontSize: "13px",
              color: "#6b7280",
            }}
          >
            Loading...
          </div>
        ) : chartData.length === 0 ? (
          <div
            className="flex h-full items-center justify-center"
            style={{
              fontFamily: "var(--font-dm-sans)",
              fontSize: "13px",
              color: "#b0b0a8",
            }}
          >
            No data yet
          </div>
        ) : (
          <div className="absolute inset-0">
            <ResponsiveContainer width="100%" height="100%">
              <BarChart
                data={chartData}
                margin={{ top: 4, right: 0, bottom: 0, left: -20 }}
                barGap={1}
                barCategoryGap="20%"
              >
                <CartesianGrid
                  strokeDasharray="3 3"
                  stroke="#f0f0ed"
                  vertical={false}
                />
                <XAxis
                  dataKey="label"
                  tick={{ fontSize: 10, fill: "#b0b0a8" }}
                  tickLine={false}
                  axisLine={{ stroke: "#e8e8e5" }}
                  interval="preserveStartEnd"
                />
                <YAxis
                  tick={{ fontSize: 10, fill: "#b0b0a8" }}
                  tickLine={false}
                  axisLine={false}
                  allowDecimals={false}
                />
                <Tooltip
                  contentStyle={{
                    fontFamily: "var(--font-dm-sans)",
                    fontSize: "12px",
                    borderRadius: "6px",
                    border: "1px solid #e8e8e5",
                    boxShadow: "0 2px 8px rgba(0,0,0,0.06)",
                  }}
                  labelStyle={{ fontWeight: 600, color: "#111113" }}
                  cursor={{ fill: "rgba(0,0,0,0.03)" }}
                />
                <Bar
                  dataKey="releases"
                  name="Releases"
                  fill="#e8601a"
                  radius={[2, 2, 0, 0]}
                  maxBarSize={20}
                />
                <Bar
                  dataKey="semantic_releases"
                  name="Semantic Releases"
                  fill="#f4a261"
                  radius={[2, 2, 0, 0]}
                  maxBarSize={20}
                />
              </BarChart>
            </ResponsiveContainer>
          </div>
        )}
      </div>
    </div>
  );
}
