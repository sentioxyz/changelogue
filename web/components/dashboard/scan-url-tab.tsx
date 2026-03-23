"use client";

import { useState, useEffect, useCallback } from "react";
import useSWR, { mutate } from "swr";
import { Loader2, Check, Search, PackageSearch } from "lucide-react";
import { onboard, projects as projectsApi } from "@/lib/api/client";
import type { OnboardScan, OnboardSelection, Project, Source } from "@/lib/api/types";
import { ScanResultsTable } from "./shared/scan-results-table";
import { useTranslation } from "@/lib/i18n/context";

type Step = "input" | "scanning" | "results" | "applied";

export function ScanUrlTab() {
  const { t } = useTranslation();
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
              placeholder={t("dashboard.scan.placeholder")}
              className="flex-1 rounded-md border border-border px-3 py-2 text-[13px] text-foreground bg-surface focus:outline-none focus:ring-2 focus:ring-beacon-accent"
              style={{
                fontFamily: "'JetBrains Mono', monospace",
              }}
              onKeyDown={(e) => e.key === "Enter" && repoUrl.trim() && beginScan(repoUrl)}
            />
            <button
              onClick={() => repoUrl.trim() && beginScan(repoUrl)}
              disabled={!repoUrl.trim()}
              className="flex items-center gap-2 rounded-md px-4 py-2 text-[13px] font-medium text-white transition-opacity hover:opacity-90 disabled:opacity-40 disabled:cursor-not-allowed bg-beacon-accent"
              style={{ fontFamily: "var(--font-dm-sans)" }}
            >
              <Search className="h-4 w-4" />
              {t("dashboard.scan.scanButton")}
            </button>
          </div>
          <p className="mt-2 text-[12px] text-text-muted">
            {t("dashboard.scan.description")}
          </p>
        </div>
      )}

      {/* Scanning */}
      {step === "scanning" && (
        <div className="flex flex-col items-center justify-center py-10">
          <div
            className="flex items-center justify-center rounded-full mb-4 bg-beacon-accent/10"
            style={{ width: 48, height: 48 }}
          >
            <Loader2 className="h-6 w-6 animate-spin text-beacon-accent" />
          </div>
          <p
            className="text-[14px] font-medium text-foreground"
            style={{ fontFamily: "var(--font-dm-sans)" }}
          >
            {scan?.status === "processing"
              ? t("dashboard.scan.analyzingDeps")
              : t("dashboard.scan.waitingToStart")}
          </p>
          <p className="mt-2 text-[12px] text-text-muted">
            {elapsed > 0 && `${elapsed}s ${t("dashboard.scan.elapsed")} · `}
            <span style={{ fontFamily: "'JetBrains Mono', monospace" }}>{repoUrl}</span>
          </p>
        </div>
      )}

      {/* Results -- empty */}
      {step === "results" && deps.length === 0 && (
        <div className="flex flex-col items-center justify-center py-10">
          <PackageSearch className="h-8 w-8 mb-3 text-text-muted" />
          <p className="text-text-secondary" style={{ fontFamily: "var(--font-dm-sans)", fontSize: "13px" }}>
            {t("dashboard.scan.noDepsDetected")}
          </p>
          <button
            onClick={resetToInput}
            className="mt-3 text-beacon-accent"
            style={{ fontFamily: "var(--font-dm-sans)", fontSize: "12px", background: "none", border: "none", cursor: "pointer" }}
          >
            {t("dashboard.scan.tryAnother")}
          </button>
        </div>
      )}

      {/* Results -- with deps */}
      {step === "results" && deps.length > 0 && (
        <>
          <div className="flex items-center justify-between mb-3">
            <p className="text-text-secondary" style={{ fontFamily: "var(--font-dm-sans)", fontSize: "13px" }}>
              {t("dashboard.scan.foundDeps")} <span className="text-foreground" style={{ fontWeight: 600 }}>{deps.length}</span> {t("dashboard.scan.dependencies")}
              {" "}{t("dashboard.scan.selected")} <span className="text-foreground" style={{ fontWeight: 600 }}>{selectedCount}</span>
            </p>
            <button
              onClick={applySelections}
              disabled={selectedCount === 0 || applying}
              className="flex items-center gap-2 rounded-md px-4 py-2 text-[13px] font-medium text-white transition-opacity hover:opacity-90 disabled:opacity-40 disabled:cursor-not-allowed bg-beacon-accent"
              style={{ fontFamily: "var(--font-dm-sans)" }}
            >
              {applying ? <Loader2 className="h-4 w-4 animate-spin" /> : <PackageSearch className="h-4 w-4" />}
              {t("dashboard.scan.trackSelected")} ({selectedCount})
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
              className="text-text-secondary"
              style={{ fontFamily: "var(--font-dm-sans)", fontSize: "12px", background: "none", border: "none", cursor: "pointer" }}
            >
              ← {t("dashboard.scan.tryAnotherShort")}
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
              {t("dashboard.scan.successCreated")} {applyResult.created_sources.length} {applyResult.created_sources.length !== 1 ? t("dashboard.scan.sources") : t("dashboard.scan.source")}
              {applyResult.created_projects.length > 0 && ` ${t("dashboard.scan.and")} ${applyResult.created_projects.length} ${applyResult.created_projects.length !== 1 ? t("dashboard.scan.projects") : t("dashboard.scan.project")}`}.
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
              <p className="font-medium mb-1">{t("dashboard.scan.skipped")} ({applyResult.skipped.length}):</p>
              <ul className="list-disc pl-5 text-[12px]">
                {applyResult.skipped.map((s, i) => <li key={i}>{s}</li>)}
              </ul>
            </div>
          )}

          <button
            onClick={resetToInput}
            className="mt-3 text-beacon-accent"
            style={{ fontFamily: "var(--font-dm-sans)", fontSize: "12px", background: "none", border: "none", cursor: "pointer" }}
          >
            {t("dashboard.scan.scanAnother")} →
          </button>
        </div>
      )}
    </div>
  );
}
