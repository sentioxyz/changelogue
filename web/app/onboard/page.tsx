"use client";

import React, { useState, useEffect } from "react";
import useSWR from "swr";
import { useRouter } from "next/navigation";
import { onboard as onboardApi, projects as projectsApi } from "@/lib/api/client";
import type { OnboardScan, OnboardSelection, Project } from "@/lib/api/types";
import { Rocket, Loader2, Check, Search, ExternalLink } from "lucide-react";

type Step = "input" | "scanning" | "results" | "applied";

export default function OnboardPage() {
  const router = useRouter();
  const [step, setStep] = useState<Step>("input");
  const [repoUrl, setRepoUrl] = useState("");
  const [scanId, setScanId] = useState<string | null>(null);
  const [scan, setScan] = useState<OnboardScan | null>(null);
  const [selections, setSelections] = useState<Record<number, boolean>>({});
  const [projectAssignments, setProjectAssignments] = useState<Record<number, { mode: "new" | "existing"; projectId?: string; newName?: string }>>({});
  const [applying, setApplying] = useState(false);
  const [applyResult, setApplyResult] = useState<{ created_projects: Project[]; created_sources: any[]; skipped: string[] } | null>(null);
  const [error, setError] = useState<string | null>(null);

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
          // Select all by default
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

  const startScan = async () => {
    setError(null);
    try {
      const resp = await onboardApi.scan(repoUrl);
      setScanId(resp.data.id);
      setScan(resp.data);
      setStep("scanning");
    } catch (e: any) {
      setError(e.message || "Failed to start scan");
    }
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
    } catch (e: any) {
      setError(e.message || "Failed to apply selections");
    } finally {
      setApplying(false);
    }
  };

  const deps = scan?.results ?? [];
  const selectedCount = Object.values(selections).filter(Boolean).length;

  const ecosystemColors: Record<string, string> = {
    go: "bg-cyan-900/50 text-cyan-300",
    npm: "bg-red-900/50 text-red-300",
    pypi: "bg-yellow-900/50 text-yellow-300",
    cargo: "bg-orange-900/50 text-orange-300",
    rubygems: "bg-pink-900/50 text-pink-300",
    maven: "bg-blue-900/50 text-blue-300",
    docker: "bg-blue-900/50 text-blue-300",
    other: "bg-gray-700/50 text-gray-300",
  };

  return (
    <main className="flex flex-1 flex-col p-6">
      <div className="mb-6">
        <h1 className="text-xl font-semibold text-white">Quick Onboard</h1>
        <p className="mt-1 text-sm text-[#9ca3af]">
          Scan a GitHub repository to detect dependencies and start tracking their releases.
        </p>
      </div>

      {error && (
        <div className="mb-4 rounded-md bg-red-900/30 px-4 py-3 text-sm text-red-300 border border-red-800/50">
          {error}
        </div>
      )}

      {/* Step 1: Input */}
      {step === "input" && (
        <div className="max-w-xl">
          <label className="block text-sm font-medium text-[#d1d5db] mb-2">
            GitHub Repository URL
          </label>
          <div className="flex gap-2">
            <input
              type="text"
              value={repoUrl}
              onChange={(e) => setRepoUrl(e.target.value)}
              placeholder="owner/repo or https://github.com/owner/repo"
              className="flex-1 rounded-md border border-[#374151] bg-[#1f2937] px-3 py-2 text-sm text-white placeholder-[#6b7280] focus:border-[#e8601a] focus:outline-none"
              onKeyDown={(e) => e.key === "Enter" && repoUrl.trim() && startScan()}
            />
            <button
              onClick={startScan}
              disabled={!repoUrl.trim()}
              className="flex items-center gap-2 rounded-md bg-[#e8601a] px-4 py-2 text-sm font-medium text-white hover:bg-[#d4560f] disabled:opacity-50 disabled:cursor-not-allowed"
            >
              <Search className="h-4 w-4" />
              Scan
            </button>
          </div>
        </div>
      )}

      {/* Step 2: Scanning */}
      {step === "scanning" && (
        <div className="flex flex-col items-center justify-center py-20">
          <Loader2 className="h-8 w-8 animate-spin text-[#e8601a]" />
          <p className="mt-4 text-sm text-[#9ca3af]">
            Scanning repository for dependencies...
          </p>
          <p className="mt-1 text-xs text-[#6b7280]">
            This may take a moment while we analyze dependency files.
          </p>
        </div>
      )}

      {/* Step 3: Results */}
      {step === "results" && deps.length === 0 && (
        <div className="flex flex-col items-center justify-center py-20 text-center">
          <p className="text-sm text-[#9ca3af]">No dependencies detected in this repository.</p>
          <button
            onClick={() => { setStep("input"); setRepoUrl(""); }}
            className="mt-4 text-sm text-[#e8601a] hover:underline"
          >
            Try another repository
          </button>
        </div>
      )}

      {step === "results" && deps.length > 0 && (
        <div>
          <div className="mb-4 flex items-center justify-between">
            <p className="text-sm text-[#9ca3af]">
              Found <span className="text-white font-medium">{deps.length}</span> dependencies.
              Selected: <span className="text-white font-medium">{selectedCount}</span>
            </p>
            <button
              onClick={applySelections}
              disabled={selectedCount === 0 || applying}
              className="flex items-center gap-2 rounded-md bg-[#e8601a] px-4 py-2 text-sm font-medium text-white hover:bg-[#d4560f] disabled:opacity-50"
            >
              {applying ? <Loader2 className="h-4 w-4 animate-spin" /> : <Rocket className="h-4 w-4" />}
              Track Selected ({selectedCount})
            </button>
          </div>

          <div className="overflow-hidden rounded-lg border border-[#374151]">
            <table className="w-full text-sm">
              <thead>
                <tr className="border-b border-[#374151] bg-[#111827]">
                  <th className="w-10 px-3 py-2">
                    <input
                      type="checkbox"
                      checked={selectedCount === deps.length}
                      onChange={(e) => {
                        const val = e.target.checked;
                        const s: Record<number, boolean> = {};
                        deps.forEach((_, i) => { s[i] = val; });
                        setSelections(s);
                      }}
                      className="rounded border-[#374151]"
                    />
                  </th>
                  <th className="px-3 py-2 text-left font-medium text-[#9ca3af]">Dependency</th>
                  <th className="px-3 py-2 text-left font-medium text-[#9ca3af]">Version</th>
                  <th className="px-3 py-2 text-left font-medium text-[#9ca3af]">Ecosystem</th>
                  <th className="px-3 py-2 text-left font-medium text-[#9ca3af]">Source</th>
                  <th className="px-3 py-2 text-left font-medium text-[#9ca3af]">Project</th>
                </tr>
              </thead>
              <tbody>
                {deps.map((dep, i) => (
                  <tr key={i} className="border-b border-[#374151] hover:bg-[#1f2937]/50">
                    <td className="px-3 py-2">
                      <input
                        type="checkbox"
                        checked={!!selections[i]}
                        onChange={(e) => setSelections({ ...selections, [i]: e.target.checked })}
                        className="rounded border-[#374151]"
                      />
                    </td>
                    <td className="px-3 py-2 text-white font-mono text-xs">{dep.name}</td>
                    <td className="px-3 py-2 text-[#9ca3af] font-mono text-xs">{dep.version}</td>
                    <td className="px-3 py-2">
                      <span className={`inline-block rounded px-2 py-0.5 text-xs font-medium ${ecosystemColors[dep.ecosystem] || ecosystemColors.other}`}>
                        {dep.ecosystem}
                      </span>
                    </td>
                    <td className="px-3 py-2 text-[#9ca3af] text-xs">{dep.upstream_repo}</td>
                    <td className="px-3 py-2">
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
                        className="rounded border border-[#374151] bg-[#1f2937] px-2 py-1 text-xs text-white"
                      >
                        <option value="__new__">Create new project</option>
                        {existingProjects.map((p) => (
                          <option key={p.id} value={p.id}>{p.name}</option>
                        ))}
                      </select>
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        </div>
      )}

      {/* Step 4: Applied */}
      {step === "applied" && applyResult && (
        <div>
          <div className="mb-6 rounded-md bg-green-900/30 px-4 py-3 text-sm text-green-300 border border-green-800/50">
            <div className="flex items-center gap-2">
              <Check className="h-4 w-4" />
              Successfully created {applyResult.created_sources.length} sources
              {applyResult.created_projects.length > 0 && ` and ${applyResult.created_projects.length} projects`}.
            </div>
          </div>

          {applyResult.skipped.length > 0 && (
            <div className="mb-4 rounded-md bg-yellow-900/30 px-4 py-3 text-sm text-yellow-300 border border-yellow-800/50">
              <p className="font-medium mb-1">Skipped ({applyResult.skipped.length}):</p>
              <ul className="list-disc pl-5 text-xs">
                {applyResult.skipped.map((s, i) => <li key={i}>{s}</li>)}
              </ul>
            </div>
          )}

          <div className="flex gap-3">
            <button
              onClick={() => router.push("/projects")}
              className="flex items-center gap-2 rounded-md bg-[#e8601a] px-4 py-2 text-sm font-medium text-white hover:bg-[#d4560f]"
            >
              View Projects
              <ExternalLink className="h-3 w-3" />
            </button>
            <button
              onClick={() => { setStep("input"); setRepoUrl(""); setScanId(null); setScan(null); setApplyResult(null); }}
              className="rounded-md border border-[#374151] px-4 py-2 text-sm text-[#9ca3af] hover:text-white hover:border-[#4b5563]"
            >
              Scan Another Repo
            </button>
          </div>
        </div>
      )}
    </main>
  );
}
