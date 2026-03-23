"use client";

import { useState, useCallback } from "react";
import useSWR from "swr";
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
  const [loaded, setLoaded] = useState(false);
  const [step, setStep] = useState<Step>("pick");
  const [selectedRepos, setSelectedRepos] = useState<Set<string>>(new Set());
  const [scans, setScans] = useState<ScanState[]>([]);
  const [appliedCount, setAppliedCount] = useState(0);

  const { data, isLoading, error } = useSWR(
    loaded ? "suggestions-repos" : null,
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

  if (!loaded) {
    return (
      <div className="flex items-center justify-center py-12">
        <button
          onClick={() => setLoaded(true)}
          className="rounded-lg bg-purple-600 px-6 py-3 text-sm font-semibold text-white hover:bg-purple-700 transition-colors"
        >
          Load your repos
        </button>
      </div>
    );
  }

  if (isLoading) {
    return (
      <div className="flex items-center justify-center py-12">
        <div className="text-sm text-zinc-400">Loading your repos...</div>
      </div>
    );
  }

  if (error) {
    return (
      <div className="flex items-center justify-center py-12">
        <div className="text-sm text-red-400">Failed to load repos. Try again later.</div>
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
    <div className="flex flex-col items-center justify-center py-12 gap-3">
      <div className="text-sm text-green-400">
        Successfully started tracking {appliedCount} dependencies.
      </div>
      <button
        onClick={() => {
          setStep("pick");
          setScans([]);
          setSelectedRepos(new Set());
        }}
        className="text-xs text-zinc-400 hover:text-zinc-200 underline"
      >
        Scan more repos
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
      <div className="flex items-center justify-center py-12">
        <div className="text-sm text-zinc-400">No public repos found.</div>
      </div>
    );
  }

  return (
    <div>
      <p className="text-sm text-zinc-400 mb-3">Select repos to scan for dependencies:</p>
      <div className="flex flex-col gap-2 max-h-80 overflow-y-auto">
        {repos.map((repo) => (
          <label
            key={repo.full_name}
            className="flex items-center gap-3 rounded-lg border border-zinc-700 bg-zinc-800 p-3 cursor-pointer hover:border-zinc-600 transition-colors"
          >
            <input
              type="checkbox"
              checked={selected.has(repo.full_name)}
              onChange={() => onToggle(repo.full_name)}
              className="accent-purple-600"
            />
            <div className="flex-1 min-w-0">
              <div className="text-sm font-semibold text-zinc-200 truncate">{repo.full_name}</div>
              <div className="text-xs text-zinc-500">
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
          className="rounded-lg bg-purple-600 px-5 py-2 text-sm font-semibold text-white hover:bg-purple-700 disabled:opacity-50 transition-colors"
        >
          Scan Selected ({selected.size})
        </button>
      </div>
    </div>
  );
}

function ScanProgress({ scans }: { scans: ScanState[] }) {
  return (
    <div className="space-y-2 py-4">
      <p className="text-sm text-zinc-400 mb-3">Scanning repositories for dependencies...</p>
      {scans.map((scan) => (
        <div key={scan.repoName} className="flex items-center gap-3 text-sm">
          <span className="w-5 text-center">
            {scan.status === "completed" && "\u2713"}
            {scan.status === "failed" && "\u2717"}
            {(scan.status === "queued" || scan.status === "processing") && "\u23f3"}
          </span>
          <span className={scan.status === "failed" ? "text-red-400" : "text-zinc-300"}>
            {scan.repoName}
          </span>
          {scan.status === "failed" && scan.error && (
            <span className="text-xs text-red-400/70">({scan.error})</span>
          )}
        </div>
      ))}
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
      <div className="flex items-center justify-center py-12">
        <div className="text-sm text-zinc-400">No dependencies found in the selected repos.</div>
      </div>
    );
  }

  return (
    <div>
      {scans.map((scan) => {
        if (!scan.results?.length) return null;
        return (
          <div key={scan.scanId} className="mb-4">
            <h4 className="text-sm font-semibold text-zinc-300 mb-2">
              From {scan.repoName}
            </h4>
            <div className="flex flex-col gap-1">
              {scan.results.map((dep) => {
                const key = `${scan.scanId}:${dep.name}`;
                return (
                  <label
                    key={key}
                    className="flex items-center gap-3 rounded border border-zinc-700 bg-zinc-800/50 px-3 py-2 cursor-pointer text-sm"
                  >
                    <input
                      type="checkbox"
                      checked={selected.has(key)}
                      onChange={() => toggle(key)}
                      className="accent-purple-600"
                    />
                    <span className="text-zinc-200">{dep.name}</span>
                    {dep.version && (
                      <span className="text-xs text-zinc-500">{dep.version}</span>
                    )}
                    <span className="ml-auto text-xs text-zinc-500">{dep.ecosystem}</span>
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
          className="rounded-lg bg-purple-600 px-5 py-2 text-sm font-semibold text-white hover:bg-purple-700 disabled:opacity-50 transition-colors"
        >
          Track Selected ({totalSelected})
        </button>
      </div>
    </div>
  );
}
