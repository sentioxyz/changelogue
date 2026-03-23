"use client";

import { useState } from "react";
import { useTranslation } from "@/lib/i18n/context";
import { StarsTab } from "./stars-tab";
import { DepsTab } from "./deps-tab";
import { ScanUrlTab } from "./scan-url-tab";

type Tab = "scan" | "stars" | "deps";

interface SuggestionsSectionProps {
  showAuthTabs?: boolean;
}

export function SuggestionsSection({ showAuthTabs }: SuggestionsSectionProps) {
  const [tab, setTab] = useState<Tab>("scan");
  const { t } = useTranslation();

  const tabs: { key: Tab; labelKey: string }[] = [
    { key: "scan", labelKey: "dashboard.suggestions.tabScanUrl" },
    ...(showAuthTabs
      ? [
          { key: "stars" as Tab, labelKey: "dashboard.suggestions.tabYourStars" },
          { key: "deps" as Tab, labelKey: "dashboard.suggestions.tabYourDeps" },
        ]
      : []),
  ];

  return (
    <div
      className="rounded-lg bg-surface border border-border"
      style={{ overflow: "hidden" }}
    >
      <div
        className="flex items-center gap-0 px-5 border-b border-border"
      >
        <span
          className="flex-shrink-0 mr-4 text-foreground"
          style={{
            fontFamily: "var(--font-fraunces)",
            fontSize: "14px",
            fontWeight: 600,
          }}
        >
          {t("dashboard.suggestions.quickOnboard")}
        </span>
        {tabs.map((tabItem) => (
          <button
            key={tabItem.key}
            onClick={() => setTab(tabItem.key)}
            style={{
              padding: "10px 16px",
              fontFamily: "var(--font-dm-sans)",
              fontSize: "13px",
              fontWeight: tab === tabItem.key ? 600 : 400,
              color: tab === tabItem.key ? "var(--beacon-accent)" : "var(--text-secondary)",
              borderBottom: tab === tabItem.key ? "2px solid var(--beacon-accent)" : "2px solid transparent",
              background: "none",
              cursor: "pointer",
              transition: "color 0.15s",
            }}
          >
            {t(tabItem.labelKey)}
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
