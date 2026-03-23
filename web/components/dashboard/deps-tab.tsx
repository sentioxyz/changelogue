"use client";

import { useState, useEffect, useCallback } from "react";
import useSWR from "swr";
import { Loader2, Check, PackageSearch } from "lucide-react";
import { suggestions, onboard, projects as projectsApi } from "@/lib/api/client";
import type { RepoItem, OnboardScan, OnboardSelection, Project, Source } from "@/lib/api/types";

type Step = "pick" | "scanning" | "results" | "applied";

const ecosystemColors: Record<string, { bg: string; text: string; border: string }> = {
  go: { bg: "#ecfeff", text: "#0e7490", border: "#a5f3fc" },
  npm: { bg: "#fef2f2", text: "#b91c1c", border: "#fecaca" },
  pypi: { bg: "#fefce8", text: "#a16207", border: "#fde68a" },
  cargo: { bg: "#fff7ed", text: "#c2410c", border: "#fed7aa" },
  rubygems: { bg: "#fdf2f8", text: "#be185d", border: "#fbcfe8" },
  maven: { bg: "#eff6ff", text: "#1d4ed8", border: "#bfdbfe" },
  gradle: { bg: "#eff6ff", text: "#1d4ed8", border: "#bfdbfe" },
  docker: { bg: "#eff6ff", text: "#1d4ed8", border: "#bfdbfe" },
  other: { bg: "#f3f3f1", text: "#6b7280", border: "#e8e8e5" },
};

export function DepsTab() {
  const [step, setStep] = useState<Step>("pick");
  const [scanId, setScanId] = useState<string | null>(null);
  const [scan, setScan] = useState<OnboardScan | null>(null);
  const [selections, setSelections] = useState<Record<number, boolean>>({});
  const [projectAssignments, setProjectAssignments] = useState<Record<number, { mode: "new" | "existing"; projectId?: string; newName?: string }>>({});
  const [applying, setApplying] = useState(false);
  const [applyResult, setApplyResult] = useState<{ created_projects: Project[]; created_sources: Source[]; skipped: string[] } | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [scanningRepo, setScanningRepo] = useState<string>("");
  const [elapsed, setElapsed] = useState(0);

  const { data, isLoading, error: fetchError } = useSWR(
    "suggestions-repos",
    () => suggestions.repos(),
    { revalidateOnFocus: false }
  );

  const { data: projectsResp } = useSWR("projects-all", () => projectsApi.list(1, 200));
  const existingProjects = projectsResp?.data ?? [];

  const repos = data?.data ?? [];

  // Poll scan status
  useEffect(() => {
    if (step !== "scanning" || !scanId) return;
    const interval = setInterval(async () => {
      try {
        const resp = await onboard.getScan(scanId);
        setScan(resp.data);
        if (resp.data.status === "completed") {
          setStep("results");
          const deps = resp.data.results ?? [];
          const sel: Record<number, boolean> = {};
          const assign: Record<number, { mode: "new" | "existing"; newName?: string }> = {};
          deps.forEach((d, i) => {
            sel[i] = true;
            assign[i] = { mode: "new", newName: d.name.replace(/\//g, "-") };
          });
          setSelections(sel);
          setProjectAssignments(assign);
          clearInterval(interval);
        } else if (resp.data.status === "failed") {
          setError(resp.data.error || "Scan failed");
          setStep("pick");
          clearInterval(interval);
        }
      } catch {
        // Keep polling
      }
    }, 2000);
    return () => clearInterval(interval);
  }, [step, scanId]);

  // Elapsed time counter during scanning
  useEffect(() => {
    if (step !== "scanning") { setElapsed(0); return; }
    const start = Date.now();
    const timer = setInterval(() => setElapsed(Math.floor((Date.now() - start) / 1000)), 1000);
    return () => clearInterval(timer);
  }, [step]);

  const handleScanRepo = useCallback(async (repo: RepoItem) => {
    setError(null);
    setScanningRepo(repo.full_name);
    try {
      const repoUrl = `https://github.com/${repo.full_name}`;
      const resp = await onboard.scan(repoUrl);
      const scanData = resp.data;
      setScanId(scanData.id);
      setScan(scanData);

      if (scanData.status === "completed") {
        const deps = scanData.results ?? [];
        const sel: Record<number, boolean> = {};
        const assign: Record<number, { mode: "new" | "existing"; newName?: string }> = {};
        deps.forEach((d, i) => {
          sel[i] = true;
          assign[i] = { mode: "new", newName: d.name.replace(/\//g, "-") };
        });
        setSelections(sel);
        setProjectAssignments(assign);
        setStep("results");
      } else if (scanData.status === "failed") {
        setError(scanData.error || "Scan failed");
      } else {
        setStep("scanning");
      }
    } catch (e: unknown) {
      setError(e instanceof Error ? e.message : "Failed to start scan");
    }
  }, []);

  const applySelections = useCallback(async () => {
    if (!scan?.results || !scanId) return;
    setApplying(true);
    setError(null);

    const sels: OnboardSelection[] = [];
    scan.results.forEach((dep, i) => {
      if (!selections[i]) return;
      const assign = projectAssignments[i];
      sels.push({
        dep_name: dep.name,
        upstream_repo: dep.upstream_repo,
        provider: dep.provider,
        project_id: assign?.mode === "existing" ? assign.projectId : undefined,
        new_project_name: assign?.mode === "new" ? (assign.newName || dep.name.replace(/\//g, "-")) : undefined,
      });
    });

    try {
      const resp = await onboard.apply(scanId, sels);
      setApplyResult(resp.data);
      setStep("applied");
    } catch (e: unknown) {
      setError(e instanceof Error ? e.message : "Failed to apply selections");
    } finally {
      setApplying(false);
    }
  }, [scan, scanId, selections, projectAssignments]);

  const resetToPick = () => {
    setStep("pick");
    setScanId(null);
    setScan(null);
    setSelections({});
    setProjectAssignments({});
    setApplyResult(null);
    setError(null);
    setScanningRepo("");
  };

  const deps = scan?.results ?? [];
  const selectedCount = Object.values(selections).filter(Boolean).length;

  if (isLoading) {
    return (
      <div className="flex items-center justify-center py-10">
        <Loader2 className="h-4 w-4 animate-spin" style={{ color: "#e8601a" }} />
        <span
          className="ml-2"
          style={{ fontFamily: "var(--font-dm-sans)", fontSize: "13px", color: "#6b7280" }}
        >
          Loading your repos...
        </span>
      </div>
    );
  }

  if (fetchError) {
    return (
      <div className="flex items-center justify-center py-10">
        <span style={{ fontFamily: "var(--font-dm-sans)", fontSize: "13px", color: "#ef4444" }}>
          Failed to load repos. Try again later.
        </span>
      </div>
    );
  }

  return (
    <div>
      {error && (
        <div
          className="rounded-md px-4 py-3 text-[13px] mb-3"
          style={{
            backgroundColor: "#fef2f2",
            border: "1px solid #fecaca",
            color: "#b91c1c",
            fontFamily: "var(--font-dm-sans)",
          }}
        >
          {error}
        </div>
      )}

      {/* Step 1: Repo picker — single select */}
      {step === "pick" && (
        repos.length === 0 ? (
          <div className="flex items-center justify-center py-10">
            <span style={{ fontFamily: "var(--font-dm-sans)", fontSize: "13px", color: "#6b7280" }}>
              No public repos found.
            </span>
          </div>
        ) : (
          <div>
            <p
              className="mb-3"
              style={{ fontFamily: "var(--font-dm-sans)", fontSize: "13px", color: "#6b7280" }}
            >
              Pick a repo to scan for dependencies:
            </p>
            <div className="flex flex-col gap-2 max-h-80 overflow-y-auto">
              {repos.map((repo) => (
                <button
                  key={repo.full_name}
                  onClick={() => handleScanRepo(repo)}
                  className="flex items-center gap-3 rounded-lg p-3 text-left transition-colors"
                  style={{
                    border: "1px solid #e8e8e5",
                    backgroundColor: "#ffffff",
                    cursor: "pointer",
                  }}
                  onMouseEnter={(e) => {
                    e.currentTarget.style.borderColor = "#e8601a";
                    e.currentTarget.style.backgroundColor = "#fff7ed";
                  }}
                  onMouseLeave={(e) => {
                    e.currentTarget.style.borderColor = "#e8e8e5";
                    e.currentTarget.style.backgroundColor = "#ffffff";
                  }}
                >
                  <div className="flex-1 min-w-0">
                    <div
                      className="truncate"
                      style={{ fontFamily: "var(--font-dm-sans)", fontSize: "13px", fontWeight: 600, color: "#111113" }}
                    >
                      {repo.full_name}
                    </div>
                    <div style={{ fontFamily: "var(--font-dm-sans)", fontSize: "11px", color: "#9ca3af" }}>
                      {repo.pushed_at && `Pushed ${new Date(repo.pushed_at).toLocaleDateString()}`}
                      {repo.language && ` · ${repo.language}`}
                    </div>
                  </div>
                  <span
                    style={{ fontFamily: "var(--font-dm-sans)", fontSize: "12px", color: "#e8601a", fontWeight: 500 }}
                  >
                    Scan →
                  </span>
                </button>
              ))}
            </div>
          </div>
        )
      )}

      {/* Step 2: Scanning */}
      {step === "scanning" && (
        <div className="flex flex-col items-center justify-center py-10">
          <div
            className="flex items-center justify-center rounded-full mb-4"
            style={{ width: 48, height: 48, backgroundColor: "#fff7ed" }}
          >
            <Loader2 className="h-6 w-6 animate-spin" style={{ color: "#e8601a" }} />
          </div>
          <p
            className="text-[14px] font-medium"
            style={{ color: "#111113", fontFamily: "var(--font-dm-sans)" }}
          >
            {scan?.status === "processing"
              ? "Analyzing dependency files..."
              : "Waiting to start scan..."}
          </p>
          <p className="mt-2 text-[12px]" style={{ color: "#9ca3af" }}>
            {elapsed > 0 && `${elapsed}s elapsed · `}
            <span style={{ fontFamily: "'JetBrains Mono', monospace" }}>{scanningRepo}</span>
          </p>
        </div>
      )}

      {/* Step 3: Results — empty */}
      {step === "results" && deps.length === 0 && (
        <div className="flex flex-col items-center justify-center py-10">
          <PackageSearch className="h-8 w-8 mb-3" style={{ color: "#c4c4c0" }} />
          <p style={{ fontFamily: "var(--font-dm-sans)", fontSize: "13px", color: "#6b7280" }}>
            No dependencies detected in <span style={{ fontWeight: 600, color: "#111113" }}>{scanningRepo}</span>.
          </p>
          <button
            onClick={resetToPick}
            className="mt-3"
            style={{ fontFamily: "var(--font-dm-sans)", fontSize: "12px", color: "#e8601a", background: "none", border: "none", cursor: "pointer" }}
          >
            ← Pick another repo
          </button>
        </div>
      )}

      {/* Step 3: Results — with deps (same table as Quick Onboard) */}
      {step === "results" && deps.length > 0 && (
        <>
          <div className="flex items-center justify-between mb-3">
            <p style={{ fontFamily: "var(--font-dm-sans)", fontSize: "13px", color: "#6b7280" }}>
              Found <span style={{ color: "#111113", fontWeight: 600 }}>{deps.length}</span> dependencies in{" "}
              <span style={{ fontFamily: "'JetBrains Mono', monospace", color: "#111113", fontWeight: 500 }}>{scanningRepo}</span>.
              {" "}Selected: <span style={{ color: "#111113", fontWeight: 600 }}>{selectedCount}</span>
            </p>
            <button
              onClick={applySelections}
              disabled={selectedCount === 0 || applying}
              className="flex items-center gap-2 rounded-md px-4 py-2 text-[13px] font-medium text-white transition-opacity hover:opacity-90 disabled:opacity-40 disabled:cursor-not-allowed"
              style={{ backgroundColor: "#e8601a", fontFamily: "var(--font-dm-sans)" }}
            >
              {applying ? <Loader2 className="h-4 w-4 animate-spin" /> : <PackageSearch className="h-4 w-4" />}
              Track Selected ({selectedCount})
            </button>
          </div>

          <div
            className="overflow-hidden rounded-md"
            style={{ border: "1px solid #e8e8e5", backgroundColor: "#ffffff" }}
          >
            <table className="w-full text-[13px]" style={{ fontFamily: "var(--font-dm-sans)" }}>
              <thead>
                <tr style={{ borderBottom: "1px solid #e8e8e5", backgroundColor: "#fafaf9" }}>
                  <th className="w-10 px-3 py-2.5">
                    <input
                      type="checkbox"
                      checked={selectedCount === deps.length}
                      onChange={(e) => {
                        const val = e.target.checked;
                        const s: Record<number, boolean> = {};
                        deps.forEach((_, i) => { s[i] = val; });
                        setSelections(s);
                      }}
                      className="rounded"
                      style={{ accentColor: "#e8601a" }}
                    />
                  </th>
                  <th className="px-3 py-2.5 text-left text-[11px] font-semibold uppercase tracking-[0.08em]" style={{ color: "#9ca3af" }}>
                    Dependency
                  </th>
                  <th className="px-3 py-2.5 text-left text-[11px] font-semibold uppercase tracking-[0.08em]" style={{ color: "#9ca3af" }}>
                    Version
                  </th>
                  <th className="px-3 py-2.5 text-left text-[11px] font-semibold uppercase tracking-[0.08em]" style={{ color: "#9ca3af" }}>
                    Ecosystem
                  </th>
                  <th className="px-3 py-2.5 text-left text-[11px] font-semibold uppercase tracking-[0.08em]" style={{ color: "#9ca3af" }}>
                    Source
                  </th>
                  <th className="px-3 py-2.5 text-left text-[11px] font-semibold uppercase tracking-[0.08em]" style={{ color: "#9ca3af" }}>
                    Project
                  </th>
                </tr>
              </thead>
              <tbody>
                {deps.map((dep, i) => {
                  const eco = ecosystemColors[dep.ecosystem] || ecosystemColors.other;
                  return (
                    <tr
                      key={i}
                      className="transition-colors hover:bg-[#fafaf9]"
                      style={{
                        borderBottom: i < deps.length - 1 ? "1px solid #e8e8e5" : undefined,
                        opacity: selections[i] ? 1 : 0.5,
                      }}
                    >
                      <td className="px-3 py-2.5">
                        <input
                          type="checkbox"
                          checked={!!selections[i]}
                          onChange={(e) => setSelections({ ...selections, [i]: e.target.checked })}
                          className="rounded"
                          style={{ accentColor: "#e8601a" }}
                        />
                      </td>
                      <td className="px-3 py-2.5">
                        <span
                          className="text-[12px]"
                          style={{ fontFamily: "'JetBrains Mono', monospace", color: "#111113" }}
                        >
                          {dep.name}
                        </span>
                      </td>
                      <td className="px-3 py-2.5">
                        <span
                          className="inline-flex items-center rounded px-1.5 py-0.5 text-[11px]"
                          style={{ backgroundColor: "#f3f3f1", fontFamily: "'JetBrains Mono', monospace", color: "#374151" }}
                        >
                          {dep.version}
                        </span>
                      </td>
                      <td className="px-3 py-2.5">
                        <span
                          className="inline-flex items-center rounded-full px-2 py-0.5 text-[11px] font-medium"
                          style={{ backgroundColor: eco.bg, color: eco.text, border: `1px solid ${eco.border}` }}
                        >
                          {dep.ecosystem}
                        </span>
                      </td>
                      <td className="px-3 py-2.5">
                        <span className="text-[12px]" style={{ color: "#6b7280", fontFamily: "'JetBrains Mono', monospace" }}>
                          {dep.upstream_repo}
                        </span>
                      </td>
                      <td className="px-3 py-2.5">
                        <select
                          value={projectAssignments[i]?.mode === "existing" ? projectAssignments[i]?.projectId : "__new__"}
                          onChange={(e) => {
                            const val = e.target.value;
                            if (val === "__new__") {
                              setProjectAssignments({
                                ...projectAssignments,
                                [i]: { mode: "new", newName: dep.name.replace(/\//g, "-") },
                              });
                            } else {
                              setProjectAssignments({
                                ...projectAssignments,
                                [i]: { mode: "existing", projectId: val },
                              });
                            }
                          }}
                          className="rounded-md border px-2 py-1 text-[12px] bg-white"
                          style={{ borderColor: "#e8e8e5", color: "#374151", fontFamily: "var(--font-dm-sans)" }}
                        >
                          <option value="__new__">Create new project</option>
                          {existingProjects.map((p) => (
                            <option key={p.id} value={p.id}>{p.name}</option>
                          ))}
                        </select>
                      </td>
                    </tr>
                  );
                })}
              </tbody>
            </table>
          </div>

          <div className="mt-3">
            <button
              onClick={resetToPick}
              style={{ fontFamily: "var(--font-dm-sans)", fontSize: "12px", color: "#6b7280", background: "none", border: "none", cursor: "pointer" }}
            >
              ← Pick another repo
            </button>
          </div>
        </>
      )}

      {/* Step 4: Applied */}
      {step === "applied" && applyResult && (
        <div>
          <div
            className="rounded-md px-4 py-3 text-[13px]"
            style={{
              backgroundColor: "#f0fdf4",
              border: "1px solid #bbf7d0",
              color: "#15803d",
              fontFamily: "var(--font-dm-sans)",
            }}
          >
            <div className="flex items-center gap-2">
              <Check className="h-4 w-4" />
              Successfully created {applyResult.created_sources.length} source{applyResult.created_sources.length !== 1 ? "s" : ""}
              {applyResult.created_projects.length > 0 && ` and ${applyResult.created_projects.length} project${applyResult.created_projects.length !== 1 ? "s" : ""}`}.
            </div>
          </div>

          {applyResult.skipped.length > 0 && (
            <div
              className="rounded-md px-4 py-3 text-[13px] mt-3"
              style={{
                backgroundColor: "#fefce8",
                border: "1px solid #fde68a",
                color: "#a16207",
                fontFamily: "var(--font-dm-sans)",
              }}
            >
              <p className="font-medium mb-1">Skipped ({applyResult.skipped.length}):</p>
              <ul className="list-disc pl-5 text-[12px]">
                {applyResult.skipped.map((s, i) => <li key={i}>{s}</li>)}
              </ul>
            </div>
          )}

          <button
            onClick={resetToPick}
            className="mt-3"
            style={{ fontFamily: "var(--font-dm-sans)", fontSize: "12px", color: "#e8601a", background: "none", border: "none", cursor: "pointer" }}
          >
            Scan another repo →
          </button>
        </div>
      )}
    </div>
  );
}
