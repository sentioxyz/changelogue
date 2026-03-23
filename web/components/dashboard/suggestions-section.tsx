"use client";

import { useState } from "react";
import { StarsTab } from "./stars-tab";
import { DepsTab } from "./deps-tab";

type Tab = "stars" | "deps";

export function SuggestionsSection() {
  const [tab, setTab] = useState<Tab>("stars");

  return (
    <div className="rounded-xl border border-zinc-800 bg-zinc-900 p-4">
      <div className="flex gap-0 mb-4 border-b border-zinc-700">
        <button
          onClick={() => setTab("stars")}
          className={`px-5 py-2 text-sm font-medium transition-colors ${
            tab === "stars"
              ? "border-b-2 border-purple-500 text-purple-400"
              : "text-zinc-400 hover:text-zinc-200"
          }`}
        >
          ⭐ Your Stars
        </button>
        <button
          onClick={() => setTab("deps")}
          className={`px-5 py-2 text-sm font-medium transition-colors ${
            tab === "deps"
              ? "border-b-2 border-purple-500 text-purple-400"
              : "text-zinc-400 hover:text-zinc-200"
          }`}
        >
          📦 Your Dependencies
        </button>
      </div>
      {tab === "stars" ? <StarsTab /> : <DepsTab />}
    </div>
  );
}
