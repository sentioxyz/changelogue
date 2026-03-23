"use client";

import { useState } from "react";
import { StarsTab } from "./stars-tab";
import { DepsTab } from "./deps-tab";

type Tab = "stars" | "deps";

export function SuggestionsSection() {
  const [tab, setTab] = useState<Tab>("stars");

  return (
    <div
      className="rounded-lg bg-white"
      style={{ border: "1px solid #e8e8e5", overflow: "hidden" }}
    >
      <div
        className="flex gap-0 px-5"
        style={{ borderBottom: "1px solid #e8e8e5" }}
      >
        <button
          onClick={() => setTab("stars")}
          style={{
            padding: "10px 16px",
            fontFamily: "var(--font-dm-sans)",
            fontSize: "13px",
            fontWeight: tab === "stars" ? 600 : 400,
            color: tab === "stars" ? "#e8601a" : "#6b7280",
            borderBottom: tab === "stars" ? "2px solid #e8601a" : "2px solid transparent",
            background: "none",
            cursor: "pointer",
            transition: "color 0.15s",
          }}
        >
          Your Stars
        </button>
        <button
          onClick={() => setTab("deps")}
          style={{
            padding: "10px 16px",
            fontFamily: "var(--font-dm-sans)",
            fontSize: "13px",
            fontWeight: tab === "deps" ? 600 : 400,
            color: tab === "deps" ? "#e8601a" : "#6b7280",
            borderBottom: tab === "deps" ? "2px solid #e8601a" : "2px solid transparent",
            background: "none",
            cursor: "pointer",
            transition: "color 0.15s",
          }}
        >
          Your Dependencies
        </button>
      </div>
      <div className="px-5 py-4">
        {tab === "stars" ? <StarsTab /> : <DepsTab />}
      </div>
    </div>
  );
}
