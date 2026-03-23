"use client";

import { useState, useEffect, useCallback } from "react";
import useSWR, { mutate } from "swr";
import { Loader2, Check, PackageSearch } from "lucide-react";
import { suggestions, onboard, projects as projectsApi } from "@/lib/api/client";
import type { RepoItem, OnboardScan, OnboardSelection, Project, Source } from "@/lib/api/types";
import { ScanResultsTable } from "./shared/scan-results-table";
import { useTranslation } from "@/lib/i18n/context";

type Step = "pick" | "scanning" | "results" | "applied";

export function DepsTab() {
  const { t } = useTranslation();
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
      mutate("projects-for-dashboard");
      mutate("projects-list");
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
        <Loader2 className="h-4 w-4 animate-spin text-beacon-accent" />
        <span
          className="ml-2 text-text-secondary"
          style={{ fontFamily: "var(--font-dm-sans)", fontSize: "13px" }}
        >
          {t("dashboard.deps.loadingRepos")}
        </span>
      </div>
    );
  }

  if (fetchError) {
    return (
      <div className="flex items-center justify-center py-10">
        <span style={{ fontFamily: "var(--font-dm-sans)", fontSize: "13px", color: "#ef4444" }}>
          {t("dashboard.deps.loadError")}
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

      {/* Step 1: Repo picker -- single select */}
      {step === "pick" && (
        repos.length === 0 ? (
          <div className="flex items-center justify-center py-10">
            <span className="text-text-secondary" style={{ fontFamily: "var(--font-dm-sans)", fontSize: "13px" }}>
              {t("dashboard.deps.noRepos")}
            </span>
          </div>
        ) : (
          <div>
            <p
              className="mb-3 text-text-secondary"
              style={{ fontFamily: "var(--font-dm-sans)", fontSize: "13px" }}
            >
              {t("dashboard.deps.pickRepo")}
            </p>
            <div className="flex flex-col gap-2 max-h-80 overflow-y-auto">
              {repos.map((repo) => (
                <button
                  key={repo.full_name}
                  onClick={() => handleScanRepo(repo)}
                  className="flex items-center gap-3 rounded-lg p-3 text-left transition-colors border border-border bg-surface hover:border-beacon-accent hover:bg-beacon-accent/10"
                >
                  <div className="flex-1 min-w-0">
                    <div
                      className="truncate text-foreground"
                      style={{ fontFamily: "var(--font-dm-sans)", fontSize: "13px", fontWeight: 600 }}
                    >
                      {repo.full_name}
                    </div>
                    <div className="text-text-muted" style={{ fontFamily: "var(--font-dm-sans)", fontSize: "11px" }}>
                      {repo.pushed_at && `${t("dashboard.deps.pushed")} ${new Date(repo.pushed_at).toLocaleDateString()}`}
                      {repo.language && ` · ${repo.language}`}
                    </div>
                  </div>
                  <span
                    className="text-beacon-accent"
                    style={{ fontFamily: "var(--font-dm-sans)", fontSize: "12px", fontWeight: 500 }}
                  >
                    {t("dashboard.deps.scan")} →
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
              ? t("dashboard.deps.analyzingDeps")
              : t("dashboard.deps.waitingToStart")}
          </p>
          <p className="mt-2 text-[12px] text-text-muted">
            {elapsed > 0 && `${elapsed}s ${t("dashboard.deps.elapsed")} · `}
            <span style={{ fontFamily: "'JetBrains Mono', monospace" }}>{scanningRepo}</span>
          </p>
        </div>
      )}

      {/* Step 3: Results -- empty */}
      {step === "results" && deps.length === 0 && (
        <div className="flex flex-col items-center justify-center py-10">
          <PackageSearch className="h-8 w-8 mb-3 text-text-muted" />
          <p className="text-text-secondary" style={{ fontFamily: "var(--font-dm-sans)", fontSize: "13px" }}>
            {t("dashboard.deps.noDepsDetected")} <span className="text-foreground" style={{ fontWeight: 600 }}>{scanningRepo}</span>.
          </p>
          <button
            onClick={resetToPick}
            className="mt-3 text-beacon-accent"
            style={{ fontFamily: "var(--font-dm-sans)", fontSize: "12px", background: "none", border: "none", cursor: "pointer" }}
          >
            ← {t("dashboard.deps.pickAnother")}
          </button>
        </div>
      )}

      {/* Step 3: Results -- with deps (same table as Quick Onboard) */}
      {step === "results" && deps.length > 0 && (
        <>
          <div className="flex items-center justify-between mb-3">
            <p className="text-text-secondary" style={{ fontFamily: "var(--font-dm-sans)", fontSize: "13px" }}>
              {t("dashboard.deps.foundDeps")} <span className="text-foreground" style={{ fontWeight: 600 }}>{deps.length}</span> {t("dashboard.deps.dependenciesIn")}{" "}
              <span className="text-foreground" style={{ fontFamily: "'JetBrains Mono', monospace", fontWeight: 500 }}>{scanningRepo}</span>.
              {" "}{t("dashboard.deps.selected")} <span className="text-foreground" style={{ fontWeight: 600 }}>{selectedCount}</span>
            </p>
            <button
              onClick={applySelections}
              disabled={selectedCount === 0 || applying}
              className="flex items-center gap-2 rounded-md px-4 py-2 text-[13px] font-medium text-white transition-opacity hover:opacity-90 disabled:opacity-40 disabled:cursor-not-allowed bg-beacon-accent"
              style={{ fontFamily: "var(--font-dm-sans)" }}
            >
              {applying ? <Loader2 className="h-4 w-4 animate-spin" /> : <PackageSearch className="h-4 w-4" />}
              {t("dashboard.deps.trackSelected")} ({selectedCount})
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
              onClick={resetToPick}
              className="text-text-secondary"
              style={{ fontFamily: "var(--font-dm-sans)", fontSize: "12px", background: "none", border: "none", cursor: "pointer" }}
            >
              ← {t("dashboard.deps.pickAnother")}
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
              {t("dashboard.deps.successCreated")} {applyResult.created_sources.length} {applyResult.created_sources.length !== 1 ? t("dashboard.deps.sources") : t("dashboard.deps.source")}
              {applyResult.created_projects.length > 0 && ` ${t("dashboard.deps.and")} ${applyResult.created_projects.length} ${applyResult.created_projects.length !== 1 ? t("dashboard.deps.projects") : t("dashboard.deps.project")}`}.
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
              <p className="font-medium mb-1">{t("dashboard.deps.skipped")} ({applyResult.skipped.length}):</p>
              <ul className="list-disc pl-5 text-[12px]">
                {applyResult.skipped.map((s, i) => <li key={i}>{s}</li>)}
              </ul>
            </div>
          )}

          <button
            onClick={resetToPick}
            className="mt-3 text-beacon-accent"
            style={{ fontFamily: "var(--font-dm-sans)", fontSize: "12px", background: "none", border: "none", cursor: "pointer" }}
          >
            {t("dashboard.deps.scanAnother")} →
          </button>
        </div>
      )}
    </div>
  );
}
