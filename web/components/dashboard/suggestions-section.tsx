"use client";

import { useState } from "react";
import { StarsTab } from "./stars-tab";
import { DepsTab } from "./deps-tab";
import { ScanUrlTab } from "./scan-url-tab";

type Tab = "scan" | "stars" | "deps";

interface SuggestionsSectionProps {
  showAuthTabs?: boolean;
}

export function SuggestionsSection({ showAuthTabs }: SuggestionsSectionProps) {
  const [tab, setTab] = useState<Tab>("scan");

  const tabs: { key: Tab; label: string }[] = [
    { key: "scan", label: "Scan URL" },
    ...(showAuthTabs
      ? [
          { key: "stars" as Tab, label: "Your Stars" },
          { key: "deps" as Tab, label: "Your Dependencies" },
        ]
      : []),
  ];

  return (
    <div
      className="rounded-lg bg-white"
      style={{ border: "1px solid #e8e8e5", overflow: "hidden" }}
    >
      <div
        className="flex items-center gap-0 px-5"
        style={{ borderBottom: "1px solid #e8e8e5" }}
      >
        <span
          className="flex-shrink-0 mr-4"
          style={{
            fontFamily: "var(--font-fraunces)",
            fontSize: "14px",
            fontWeight: 600,
            color: "#111113",
          }}
        >
          Quick Onboard
        </span>
        {tabs.map((t) => (
          <button
            key={t.key}
            onClick={() => setTab(t.key)}
            style={{
              padding: "10px 16px",
              fontFamily: "var(--font-dm-sans)",
              fontSize: "13px",
              fontWeight: tab === t.key ? 600 : 400,
              color: tab === t.key ? "#e8601a" : "#6b7280",
              borderBottom: tab === t.key ? "2px solid #e8601a" : "2px solid transparent",
              background: "none",
              cursor: "pointer",
              transition: "color 0.15s",
            }}
          >
            {t.label}
          </button>
        ))}
      </div>
      <div className="px-5 py-4">
        {tab === "scan" && <ScanUrlTab />}
        {tab === "stars" && <StarsTab />}
        {tab === "deps" && <DepsTab />}
      </div>
    </div>
  );
}
