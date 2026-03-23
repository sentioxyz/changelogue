"use client";

import { useState, useEffect, useCallback } from "react";
import useSWR, { mutate } from "swr";
import { Loader2, Check, Search, PackageSearch } from "lucide-react";
import { onboard, projects as projectsApi } from "@/lib/api/client";
import type { OnboardScan, OnboardSelection, Project, Source } from "@/lib/api/types";
import { ScanResultsTable } from "./shared/scan-results-table";

type Step = "input" | "scanning" | "results" | "applied";

export function ScanUrlTab() {
  const [step, setStep] = useState<Step>("input");
  const [repoUrl, setRepoUrl] = useState("");
  const [scanId, setScanId] = useState<string | null>(null);
  const [scan, setScan] = useState<OnboardScan | null>(null);
  const [selections, setSelections] = useState<Record<number, boolean>>({});
  const [projectAssignments, setProjectAssignments] = useState<Record<number, { mode: "new" | "existing"; projectId?: string; newName?: string }>>({});
  const [applying, setApplying] = useState(false);
  const [applyResult, setApplyResult] = useState<{ created_projects: Project[]; created_sources: Source[]; skipped: string[] } | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [elapsed, setElapsed] = useState(0);

  const { data: projectsResp } = useSWR("projects-all", () => projectsApi.list(1, 200));
  const existingProjects = projectsResp?.data ?? [];

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
          setStep("input");
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

  const beginScan = useCallback(async (url: string) => {
    setError(null);
    try {
      const resp = await onboard.scan(url);
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
        setStep("input");
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
      mutate("projects-for-dashboard");
      mutate("projects-list");
    } catch (e: unknown) {
      setError(e instanceof Error ? e.message : "Failed to apply selections");
    } finally {
      setApplying(false);
    }
  }, [scan, scanId, selections, projectAssignments]);

  const resetToInput = () => {
    setStep("input");
    setRepoUrl("");
    setScanId(null);
    setScan(null);
    setSelections({});
    setProjectAssignments({});
    setApplyResult(null);
    setError(null);
  };

  const deps = scan?.results ?? [];
  const selectedCount = Object.values(selections).filter(Boolean).length;

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

      {/* Input */}
      {step === "input" && (
        <div>
          <div className="flex gap-2">
            <input
              type="text"
              value={repoUrl}
              onChange={(e) => setRepoUrl(e.target.value)}
              placeholder="owner/repo or https://github.com/owner/repo"
              className="flex-1 rounded-md border px-3 py-2 text-[13px] focus:outline-none focus:ring-2"
              style={{
                borderColor: "#e8e8e5",
                color: "#111113",
                fontFamily: "'JetBrains Mono', monospace",
                backgroundColor: "#ffffff",
              }}
              onFocus={(e) => { e.currentTarget.style.borderColor = "#e8601a"; }}
              onBlur={(e) => { e.currentTarget.style.borderColor = "#e8e8e5"; }}
              onKeyDown={(e) => e.key === "Enter" && repoUrl.trim() && beginScan(repoUrl)}
            />
            <button
              onClick={() => repoUrl.trim() && beginScan(repoUrl)}
              disabled={!repoUrl.trim()}
              className="flex items-center gap-2 rounded-md px-4 py-2 text-[13px] font-medium text-white transition-opacity hover:opacity-90 disabled:opacity-40 disabled:cursor-not-allowed"
              style={{ backgroundColor: "#e8601a", fontFamily: "var(--font-dm-sans)" }}
            >
              <Search className="h-4 w-4" />
              Scan
            </button>
          </div>
          <p className="mt-2 text-[12px]" style={{ color: "#9ca3af" }}>
            We&apos;ll scan for go.mod, package.json, requirements.txt, Cargo.toml, and other dependency files.
          </p>
        </div>
      )}

      {/* Scanning */}
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
            <span style={{ fontFamily: "'JetBrains Mono', monospace" }}>{repoUrl}</span>
          </p>
        </div>
      )}

      {/* Results — empty */}
      {step === "results" && deps.length === 0 && (
        <div className="flex flex-col items-center justify-center py-10">
          <PackageSearch className="h-8 w-8 mb-3" style={{ color: "#c4c4c0" }} />
          <p style={{ fontFamily: "var(--font-dm-sans)", fontSize: "13px", color: "#6b7280" }}>
            No dependencies detected in this repository.
          </p>
          <button
            onClick={resetToInput}
            className="mt-3"
            style={{ fontFamily: "var(--font-dm-sans)", fontSize: "12px", color: "#e8601a", background: "none", border: "none", cursor: "pointer" }}
          >
            Try another repository
          </button>
        </div>
      )}

      {/* Results — with deps */}
      {step === "results" && deps.length > 0 && (
        <>
          <div className="flex items-center justify-between mb-3">
            <p style={{ fontFamily: "var(--font-dm-sans)", fontSize: "13px", color: "#6b7280" }}>
              Found <span style={{ color: "#111113", fontWeight: 600 }}>{deps.length}</span> dependencies.
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

          <ScanResultsTable
            deps={deps}
            selections={selections}
            onSelectionsChange={setSelections}
            projectAssignments={projectAssignments}
            onProjectAssignmentsChange={setProjectAssignments}
            existingProjects={existingProjects}
          />

          <div className="mt-3">
            <button
              onClick={resetToInput}
              style={{ fontFamily: "var(--font-dm-sans)", fontSize: "12px", color: "#6b7280", background: "none", border: "none", cursor: "pointer" }}
            >
              ← Try another
            </button>
          </div>
        </>
      )}

      {/* Applied */}
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
            onClick={resetToInput}
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
