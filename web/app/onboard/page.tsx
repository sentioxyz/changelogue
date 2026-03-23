"use client";

import React, { useState, useEffect, Suspense } from "react";
import useSWR from "swr";
import { useRouter, useSearchParams } from "next/navigation";
import { onboard as onboardApi, projects as projectsApi } from "@/lib/api/client";
import type { OnboardScan, OnboardSelection, Project, Source } from "@/lib/api/types";
import { Loader2, Check, Search, PackageSearch, ArrowRight } from "lucide-react";
import { ScanResultsTable } from "@/components/dashboard/shared/scan-results-table";
import { useTranslation } from "@/lib/i18n/context";

type Step = "input" | "scanning" | "results" | "applied";

export default function OnboardPage() {
  return (
    <Suspense>
      <OnboardPageInner />
    </Suspense>
  );
}

function OnboardPageInner() {
  const router = useRouter();
  const searchParams = useSearchParams();
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
  const [autoStarted, setAutoStarted] = useState(false);
  const [elapsed, setElapsed] = useState(0);

  // Fetch existing projects for the dropdown
  const { data: projectsResp } = useSWR("projects-all", () => projectsApi.list(1, 200));
  const existingProjects = projectsResp?.data ?? [];

  // Poll scan status
  useEffect(() => {
    if (step !== "scanning" || !scanId) return;
    const interval = setInterval(async () => {
      try {
        const resp = await onboardApi.getScan(scanId);
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
          setError(resp.data.error || t("onboard.error.scanFailed"));
          setStep("input");
          clearInterval(interval);
        }
      } catch {
        // Keep polling
      }
    }, 2000);
    return () => clearInterval(interval);
  }, [step, scanId, t]);

  // Elapsed time counter during scanning
  useEffect(() => {
    if (step !== "scanning") { setElapsed(0); return; }
    const start = Date.now();
    const timer = setInterval(() => setElapsed(Math.floor((Date.now() - start) / 1000)), 1000);
    return () => clearInterval(timer);
  }, [step]);

  // Auto-start scan from ?repo= query param
  useEffect(() => {
    const repo = searchParams.get("repo");
    if (repo && !autoStarted) {
      setAutoStarted(true);
      setRepoUrl(repo);
      beginScan(repo);
    }
  }, [searchParams, autoStarted]);

  const beginScan = async (url: string) => {
    setError(null);
    try {
      const resp = await onboardApi.scan(url);
      const scanData = resp.data;
      setScanId(scanData.id);
      setScan(scanData);
      // If scan is already completed (resuming a finished scan), go straight to results
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
        setError(scanData.error || t("onboard.error.scanFailed"));
        setStep("input");
      } else {
        setStep("scanning");
      }
    } catch (e: unknown) {
      setError(e instanceof Error ? e.message : t("onboard.error.failedToStart"));
    }
  };

  const startScan = () => {
    if (repoUrl.trim()) beginScan(repoUrl);
  };

  const applySelections = async () => {
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
      const resp = await onboardApi.apply(scanId, sels);
      setApplyResult(resp.data);
      setStep("applied");
    } catch (e: unknown) {
      setError(e instanceof Error ? e.message : t("onboard.error.failedToApply"));
    } finally {
      setApplying(false);
    }
  };

  const deps = scan?.results ?? [];
  const selectedCount = Object.values(selections).filter(Boolean).length;

  return (
    <div className="flex flex-col gap-4 fade-in">
      {/* Header */}
      <div>
        <h1
          style={{
            fontFamily: "var(--font-fraunces)",
            fontSize: "24px",
            fontWeight: 700,
            color: "var(--foreground)",
          }}
        >
          {t("onboard.title")}
        </h1>
        <p
          className="mt-1 text-[13px] text-text-secondary"
          style={{ fontFamily: "var(--font-dm-sans)" }}
        >
          {t("onboard.subtitle")}
        </p>
      </div>

      {error && (
        <div
          className="rounded-md px-4 py-3 text-[13px]"
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

      {/* Step 1: Input */}
      {step === "input" && (
        <div
          className="rounded-md px-6 py-8 border border-border bg-surface"
        >
          <div className="max-w-xl">
            <label
              className="block text-[13px] font-medium mb-2 text-secondary-foreground"
              style={{ fontFamily: "var(--font-dm-sans)" }}
            >
              {t("onboard.repoUrlLabel")}
            </label>
            <div className="flex gap-2">
              <input
                type="text"
                value={repoUrl}
                onChange={(e) => setRepoUrl(e.target.value)}
                placeholder={t("onboard.repoUrlPlaceholder")}
                className="flex-1 rounded-md border border-border px-3 py-2 text-[13px] text-foreground bg-surface focus:outline-none focus:ring-2 focus:ring-beacon-accent"
                style={{
                  fontFamily: "'JetBrains Mono', monospace",
                }}
                onFocus={(e) => { e.currentTarget.style.borderColor = "var(--beacon-accent)"; }}
                onBlur={(e) => { e.currentTarget.style.borderColor = "var(--border)"; }}
                onKeyDown={(e) => e.key === "Enter" && repoUrl.trim() && startScan()}
              />
              <button
                onClick={startScan}
                disabled={!repoUrl.trim()}
                className="flex items-center gap-2 rounded-md px-4 py-2 text-[13px] font-medium text-white bg-beacon-accent transition-opacity hover:opacity-90 disabled:opacity-40 disabled:cursor-not-allowed"
                style={{ fontFamily: "var(--font-dm-sans)" }}
              >
                <Search className="h-4 w-4" />
                {t("onboard.button.scan")}
              </button>
            </div>
            <p className="mt-2 text-[12px] text-text-muted">
              {t("onboard.scanHint")}
            </p>
          </div>
        </div>
      )}

      {/* Step 2: Scanning */}
      {step === "scanning" && (
        <div
          className="flex flex-col items-center justify-center rounded-md py-16 border border-border bg-surface"
        >
          <div
            className="flex items-center justify-center rounded-full mb-4"
            style={{ width: 48, height: 48, backgroundColor: "color-mix(in srgb, var(--beacon-accent) 10%, transparent)" }}
          >
            <Loader2 className="h-6 w-6 animate-spin text-beacon-accent" />
          </div>
          <p
            className="text-[14px] font-medium text-foreground"
            style={{ fontFamily: "var(--font-dm-sans)" }}
          >
            {scan?.status === "processing"
              ? t("onboard.scanning.analyzing")
              : t("onboard.scanning.waiting")}
          </p>
          <div className="mt-3 flex flex-col items-center gap-2">
            {/* Progress steps */}
            <div className="flex items-center gap-6">
              <ScanStep
                label={t("onboard.scanning.queued")}
                done={scan?.status === "processing"}
                active={scan?.status === "pending"}
              />
              <StepConnector done={scan?.status === "processing"} />
              <ScanStep
                label={t("onboard.scanning.fetching")}
                done={false}
                active={scan?.status === "processing"}
              />
              <StepConnector done={false} />
              <ScanStep
                label={t("onboard.scanning.extracting")}
                done={false}
                active={false}
              />
            </div>
            <p className="mt-2 text-[12px] text-text-muted">
              {elapsed > 0 && `${elapsed}${t("onboard.scanning.secondsShort")} ${t("onboard.scanning.elapsed")} · `}
              {scan?.repo_url && (
                <span style={{ fontFamily: "'JetBrains Mono', monospace" }}>{scan.repo_url}</span>
              )}
            </p>
          </div>
        </div>
      )}

      {/* Step 3: Results -- empty */}
      {step === "results" && deps.length === 0 && (
        <div
          className="flex flex-col items-center justify-center rounded-md py-16 border border-border bg-surface"
        >
          <PackageSearch className="h-8 w-8 mb-3 text-text-muted" />
          <p
            className="text-[14px] text-text-secondary"
            style={{ fontFamily: "var(--font-dm-sans)" }}
          >
            {t("onboard.results.noDeps")}
          </p>
          <button
            onClick={() => { setStep("input"); setRepoUrl(""); }}
            className="mt-4 text-[13px] font-medium text-beacon-accent hover:underline"
            style={{ fontFamily: "var(--font-dm-sans)" }}
          >
            {t("onboard.button.tryAnother")}
          </button>
        </div>
      )}

      {/* Step 3: Results -- with deps */}
      {step === "results" && deps.length > 0 && (
        <>
          <div className="flex items-center justify-between">
            <p className="text-[13px] text-text-secondary" style={{ fontFamily: "var(--font-dm-sans)" }}>
              {t("onboard.results.found")} <span className="text-foreground font-semibold">{deps.length}</span> {t("onboard.results.dependencies")}
              {" "}{t("onboard.results.selected")} <span className="text-foreground font-semibold">{selectedCount}</span>
            </p>
            <button
              onClick={applySelections}
              disabled={selectedCount === 0 || applying}
              className="flex items-center gap-2 rounded-md px-4 py-2 text-[13px] font-medium text-white bg-beacon-accent transition-opacity hover:opacity-90 disabled:opacity-40"
              style={{ fontFamily: "var(--font-dm-sans)" }}
            >
              {applying ? <Loader2 className="h-4 w-4 animate-spin" /> : <PackageSearch className="h-4 w-4" />}
              {t("onboard.button.trackSelected")} ({selectedCount})
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
        </>
      )}

      {/* Step 4: Applied */}
      {step === "applied" && applyResult && (
        <>
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
              {t("onboard.applied.successCreated")} {applyResult.created_sources.length} {t("onboard.applied.source")}{applyResult.created_sources.length !== 1 ? t("onboard.applied.plural") : ""}
              {applyResult.created_projects.length > 0 && ` ${t("onboard.applied.and")} ${applyResult.created_projects.length} ${t("onboard.applied.project")}${applyResult.created_projects.length !== 1 ? t("onboard.applied.plural") : ""}`}.
            </div>
          </div>

          {applyResult.skipped.length > 0 && (
            <div
              className="rounded-md px-4 py-3 text-[13px]"
              style={{
                backgroundColor: "#fefce8",
                border: "1px solid #fde68a",
                color: "#a16207",
                fontFamily: "var(--font-dm-sans)",
              }}
            >
              <p className="font-medium mb-1">{t("onboard.applied.skipped")} ({applyResult.skipped.length}):</p>
              <ul className="list-disc pl-5 text-[12px]">
                {applyResult.skipped.map((s, i) => <li key={i}>{s}</li>)}
              </ul>
            </div>
          )}

          <div className="flex gap-3">
            <button
              onClick={() => router.push("/projects")}
              className="flex items-center gap-2 rounded-md px-4 py-2 text-[13px] font-medium text-white bg-beacon-accent transition-opacity hover:opacity-90"
              style={{ fontFamily: "var(--font-dm-sans)" }}
            >
              {t("onboard.button.viewProjects")}
              <ArrowRight className="h-3.5 w-3.5" />
            </button>
            <button
              onClick={() => { setStep("input"); setRepoUrl(""); setScanId(null); setScan(null); setApplyResult(null); }}
              className="rounded-md border border-border px-4 py-2 text-[13px] text-secondary-foreground transition-colors hover:bg-mono-bg"
              style={{
                fontFamily: "var(--font-dm-sans)",
              }}
            >
              {t("onboard.button.scanAnother")}
            </button>
          </div>
        </>
      )}
    </div>
  );
}

/* --- Progress step components --- */

function ScanStep({ label, done, active }: { label: string; done: boolean; active: boolean }) {
  return (
    <div className="flex items-center gap-1.5">
      <div
        className="flex items-center justify-center rounded-full"
        style={{
          width: 18,
          height: 18,
          backgroundColor: done ? "#f0fdf4" : active ? "color-mix(in srgb, var(--beacon-accent) 10%, transparent)" : "var(--mono-bg)",
          border: `1.5px solid ${done ? "#16a34a" : active ? "var(--beacon-accent)" : "var(--border)"}`,
        }}
      >
        {done ? (
          <Check className="h-3 w-3" style={{ color: "#16a34a" }} />
        ) : active ? (
          <Loader2 className="h-3 w-3 animate-spin text-beacon-accent" />
        ) : (
          <div className="h-1.5 w-1.5 rounded-full" style={{ backgroundColor: "var(--text-muted)" }} />
        )}
      </div>
      <span
        className="text-[12px]"
        style={{
          color: done ? "#16a34a" : active ? "var(--foreground)" : "var(--text-muted)",
          fontFamily: "var(--font-dm-sans)",
          fontWeight: active ? 500 : 400,
        }}
      >
        {label}
      </span>
    </div>
  );
}

function StepConnector({ done }: { done: boolean }) {
  return (
    <div
      className="h-px"
      style={{
        width: 24,
        backgroundColor: done ? "#16a34a" : "var(--border)",
      }}
    />
  );
}
