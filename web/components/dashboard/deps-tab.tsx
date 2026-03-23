"use client";

import { useState, useCallback } from "react";
import useSWR from "swr";
import { Loader2, Check, AlertCircle } from "lucide-react";
import { suggestions, onboard } from "@/lib/api/client";
import type { RepoItem, OnboardScan, ScannedDependency, OnboardSelection } from "@/lib/api/types";

type Step = "pick" | "scanning" | "results" | "applied";

interface ScanState {
  repoName: string;
  scanId: string;
  status: string;
  results?: ScannedDependency[];
  error?: string;
}

export function DepsTab() {
  const [step, setStep] = useState<Step>("pick");
  const [selectedRepos, setSelectedRepos] = useState<Set<string>>(new Set());
  const [scans, setScans] = useState<ScanState[]>([]);
  const [appliedCount, setAppliedCount] = useState(0);

  const { data, isLoading, error } = useSWR(
    "suggestions-repos",
    () => suggestions.repos(),
    { revalidateOnFocus: false }
  );

  const repos = data?.data ?? [];

  const toggleRepo = useCallback((fullName: string) => {
    setSelectedRepos((prev) => {
      const next = new Set(prev);
      if (next.has(fullName)) next.delete(fullName);
      else next.add(fullName);
      return next;
    });
  }, []);

  const handleScan = useCallback(async () => {
    const repoNames = Array.from(selectedRepos);
    setStep("scanning");

    const scanStates: ScanState[] = repoNames.map((name) => ({
      repoName: name,
      scanId: "",
      status: "queued",
    }));
    setScans([...scanStates]);

    const batchSize = 5;
    for (let i = 0; i < repoNames.length; i += batchSize) {
      const batch = repoNames.slice(i, i + batchSize);
      await Promise.all(
        batch.map(async (repoName, batchIdx) => {
          const idx = i + batchIdx;
          try {
            const repoUrl = `https://github.com/${repoName}`;
            const scanRes = await onboard.scan(repoUrl);
            const scanId = scanRes.data.id;
            scanStates[idx] = { ...scanStates[idx], scanId, status: "processing" };
            setScans([...scanStates]);

            let scan: OnboardScan;
            do {
              await new Promise((r) => setTimeout(r, 2000));
              const pollRes = await onboard.getScan(scanId);
              scan = pollRes.data;
            } while (scan.status === "pending" || scan.status === "processing");

            if (scan.status === "completed" && scan.results) {
              scanStates[idx] = { ...scanStates[idx], status: "completed", results: scan.results };
            } else {
              scanStates[idx] = { ...scanStates[idx], status: "failed", error: scan.error || "Unknown error" };
            }
          } catch (err: unknown) {
            const msg = err instanceof Error ? err.message : "Scan failed";
            scanStates[idx] = { ...scanStates[idx], status: "failed", error: msg };
          }
          setScans([...scanStates]);
        })
      );
    }

    setStep("results");
  }, [selectedRepos]);

  const handleApply = useCallback(
    async (selections: { scanId: string; items: OnboardSelection[] }[]) => {
      let total = 0;
      for (const { scanId, items } of selections) {
        if (items.length === 0) continue;
        try {
          const res = await onboard.apply(scanId, items);
          total += res.data.created_sources.length;
        } catch {
          // continue with other scans
        }
      }
      setAppliedCount(total);
      setStep("applied");
    },
    []
  );

  if (isLoading) {
    return (
      <div className="flex items-center justify-center py-10">
        <Loader2
          className="h-4 w-4 animate-spin"
          style={{ color: "#e8601a" }}
        />
        <span
          className="ml-2"
          style={{
            fontFamily: "var(--font-dm-sans)",
            fontSize: "13px",
            color: "#6b7280",
          }}
        >
          Loading your repos...
        </span>
      </div>
    );
  }

  if (error) {
    return (
      <div className="flex items-center justify-center py-10">
        <span
          style={{
            fontFamily: "var(--font-dm-sans)",
            fontSize: "13px",
            color: "#ef4444",
          }}
        >
          Failed to load repos. Try again later.
        </span>
      </div>
    );
  }

  if (step === "pick") {
    return <RepoPicker repos={repos} selected={selectedRepos} onToggle={toggleRepo} onScan={handleScan} />;
  }

  if (step === "scanning") {
    return <ScanProgress scans={scans} />;
  }

  if (step === "results") {
    return <ScanResults scans={scans.filter((s) => s.status === "completed")} onApply={handleApply} />;
  }

  return (
    <div className="flex flex-col items-center justify-center py-10 gap-3">
      <div className="flex items-center gap-2">
        <Check className="h-4 w-4" style={{ color: "#16a34a" }} />
        <span
          style={{
            fontFamily: "var(--font-dm-sans)",
            fontSize: "13px",
            color: "#16a34a",
          }}
        >
          Successfully started tracking {appliedCount} dependencies.
        </span>
      </div>
      <button
        onClick={() => {
          setStep("pick");
          setScans([]);
          setSelectedRepos(new Set());
        }}
        style={{
          fontFamily: "var(--font-dm-sans)",
          fontSize: "12px",
          color: "#e8601a",
          background: "none",
          border: "none",
          cursor: "pointer",
        }}
      >
        Scan more repos →
      </button>
    </div>
  );
}

function RepoPicker({
  repos,
  selected,
  onToggle,
  onScan,
}: {
  repos: RepoItem[];
  selected: Set<string>;
  onToggle: (name: string) => void;
  onScan: () => void;
}) {
  if (repos.length === 0) {
    return (
      <div className="flex items-center justify-center py-10">
        <span
          style={{
            fontFamily: "var(--font-dm-sans)",
            fontSize: "13px",
            color: "#6b7280",
          }}
        >
          No public repos found.
        </span>
      </div>
    );
  }

  return (
    <div>
      <p
        className="mb-3"
        style={{
          fontFamily: "var(--font-dm-sans)",
          fontSize: "13px",
          color: "#6b7280",
        }}
      >
        Select repos to scan for dependencies:
      </p>
      <div className="flex flex-col gap-2 max-h-80 overflow-y-auto">
        {repos.map((repo) => (
          <label
            key={repo.full_name}
            className="flex items-center gap-3 rounded-lg p-3 cursor-pointer transition-colors"
            style={{
              border: selected.has(repo.full_name)
                ? "1px solid #e8601a"
                : "1px solid #e8e8e5",
              backgroundColor: selected.has(repo.full_name)
                ? "#fff7ed"
                : "#ffffff",
            }}
          >
            <input
              type="checkbox"
              checked={selected.has(repo.full_name)}
              onChange={() => onToggle(repo.full_name)}
              style={{ accentColor: "#e8601a" }}
            />
            <div className="flex-1 min-w-0">
              <div
                className="truncate"
                style={{
                  fontFamily: "var(--font-dm-sans)",
                  fontSize: "13px",
                  fontWeight: 600,
                  color: "#111113",
                }}
              >
                {repo.full_name}
              </div>
              <div
                style={{
                  fontFamily: "var(--font-dm-sans)",
                  fontSize: "11px",
                  color: "#9ca3af",
                }}
              >
                {repo.pushed_at && `Pushed ${new Date(repo.pushed_at).toLocaleDateString()}`}
                {repo.language && ` · ${repo.language}`}
              </div>
            </div>
          </label>
        ))}
      </div>
      <div className="flex justify-end mt-3">
        <button
          onClick={onScan}
          disabled={selected.size === 0}
          className="flex items-center gap-1.5 rounded-md px-4 py-2 text-white transition-opacity hover:opacity-90 disabled:opacity-40 disabled:cursor-not-allowed"
          style={{
            backgroundColor: "#e8601a",
            fontFamily: "var(--font-dm-sans)",
            fontSize: "13px",
            fontWeight: 600,
          }}
        >
          Scan Selected ({selected.size})
        </button>
      </div>
    </div>
  );
}

function ScanProgress({ scans }: { scans: ScanState[] }) {
  return (
    <div className="py-4">
      <p
        className="mb-3"
        style={{
          fontFamily: "var(--font-dm-sans)",
          fontSize: "13px",
          color: "#6b7280",
        }}
      >
        Scanning repositories for dependencies...
      </p>
      <div className="space-y-2">
        {scans.map((scan) => (
          <div
            key={scan.repoName}
            className="flex items-center gap-3"
            style={{
              fontFamily: "var(--font-dm-sans)",
              fontSize: "13px",
            }}
          >
            <span className="w-5 text-center flex-shrink-0">
              {scan.status === "completed" && (
                <Check className="h-3.5 w-3.5 inline" style={{ color: "#16a34a" }} />
              )}
              {scan.status === "failed" && (
                <AlertCircle className="h-3.5 w-3.5 inline" style={{ color: "#ef4444" }} />
              )}
              {(scan.status === "queued" || scan.status === "processing") && (
                <Loader2
                  className="h-3.5 w-3.5 inline animate-spin"
                  style={{ color: "#e8601a" }}
                />
              )}
            </span>
            <span style={{ color: scan.status === "failed" ? "#ef4444" : "#111113" }}>
              {scan.repoName}
            </span>
            {scan.status === "failed" && scan.error && (
              <span style={{ fontSize: "11px", color: "#9ca3af" }}>
                ({scan.error})
              </span>
            )}
          </div>
        ))}
      </div>
    </div>
  );
}

function ScanResults({
  scans,
  onApply,
}: {
  scans: ScanState[];
  onApply: (selections: { scanId: string; items: OnboardSelection[] }[]) => void;
}) {
  const [selected, setSelected] = useState<Set<string>>(() => {
    const all = new Set<string>();
    scans.forEach((scan) =>
      scan.results?.forEach((dep) => all.add(`${scan.scanId}:${dep.name}`))
    );
    return all;
  });

  const toggle = (key: string) => {
    setSelected((prev) => {
      const next = new Set(prev);
      if (next.has(key)) next.delete(key);
      else next.add(key);
      return next;
    });
  };

  const handleApply = () => {
    const selections = scans.map((scan) => ({
      scanId: scan.scanId,
      items: (scan.results ?? [])
        .filter((dep) => selected.has(`${scan.scanId}:${dep.name}`))
        .map((dep) => ({
          dep_name: dep.name,
          upstream_repo: dep.upstream_repo,
          provider: dep.provider,
          new_project_name: dep.upstream_repo || dep.name,
        })),
    }));
    onApply(selections);
  };

  const totalSelected = selected.size;

  if (scans.length === 0 || scans.every((s) => !s.results?.length)) {
    return (
      <div className="flex items-center justify-center py-10">
        <span
          style={{
            fontFamily: "var(--font-dm-sans)",
            fontSize: "13px",
            color: "#6b7280",
          }}
        >
          No dependencies found in the selected repos.
        </span>
      </div>
    );
  }

  return (
    <div>
      {scans.map((scan) => {
        if (!scan.results?.length) return null;
        return (
          <div key={scan.scanId} className="mb-4">
            <h4
              className="mb-2"
              style={{
                fontFamily: "var(--font-fraunces)",
                fontSize: "13px",
                fontWeight: 600,
                color: "#111113",
              }}
            >
              From {scan.repoName}
            </h4>
            <div className="flex flex-col gap-1">
              {scan.results.map((dep) => {
                const key = `${scan.scanId}:${dep.name}`;
                return (
                  <label
                    key={key}
                    className="flex items-center gap-3 rounded-md px-3 py-2 cursor-pointer"
                    style={{
                      border: "1px solid #e8e8e5",
                      backgroundColor: selected.has(key) ? "#fff7ed" : "#fafaf9",
                      fontFamily: "var(--font-dm-sans)",
                      fontSize: "13px",
                    }}
                  >
                    <input
                      type="checkbox"
                      checked={selected.has(key)}
                      onChange={() => toggle(key)}
                      style={{ accentColor: "#e8601a" }}
                    />
                    <span style={{ color: "#111113" }}>{dep.name}</span>
                    {dep.version && (
                      <span style={{ fontSize: "11px", color: "#9ca3af" }}>
                        {dep.version}
                      </span>
                    )}
                    <span
                      className="ml-auto"
                      style={{ fontSize: "11px", color: "#9ca3af" }}
                    >
                      {dep.ecosystem}
                    </span>
                  </label>
                );
              })}
            </div>
          </div>
        );
      })}
      <div className="flex justify-end mt-3">
        <button
          onClick={handleApply}
          disabled={totalSelected === 0}
          className="flex items-center gap-1.5 rounded-md px-4 py-2 text-white transition-opacity hover:opacity-90 disabled:opacity-40 disabled:cursor-not-allowed"
          style={{
            backgroundColor: "#e8601a",
            fontFamily: "var(--font-dm-sans)",
            fontSize: "13px",
            fontWeight: 600,
          }}
        >
          Track Selected ({totalSelected})
        </button>
      </div>
    </div>
  );
}
