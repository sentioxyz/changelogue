"use client";

import useSWR from "swr";
import Link from "next/link";
import { useState, useRef, useEffect, useCallback } from "react";
import { useRouter } from "next/navigation";
import {
  projects as projectsApi,
  sources as sourcesApi,
  contextSources as ctxApi,
  agent as agentApi,
  releases as releasesApi,
} from "@/lib/api/client";
import type { AgentRules, Source } from "@/lib/api/types";
import { Dialog, DialogContent, DialogHeader, DialogTitle } from "@/components/ui/dialog";
import { ConfirmDialog } from "@/components/ui/confirm-dialog";
import { SourceForm } from "@/components/sources/source-form";
import { NewContextSourceForm } from "@/components/context-sources/new-context-source-form";
import { ProjectLogo } from "@/components/ui/project-logo";
import { ProviderBadge } from "@/components/ui/provider-badge";
import { StatusDot } from "@/components/ui/status-dot";
import { SectionLabel } from "@/components/ui/section-label";
import { formatInterval } from "@/lib/format";
import { getPathSegment } from "@/lib/path";
import { Pencil, Trash2, Play, Plus, ArrowLeft } from "lucide-react";

/* ---------- Tabs ---------- */

const tabs = [
  { key: "sources", label: "Sources" },
  { key: "context", label: "Context Sources" },
  { key: "agent", label: "Semantic Release Settings" },
] as const;

type TabKey = (typeof tabs)[number]["key"];

/* ---------- Helpers ---------- */

function formatDuration(startedAt?: string, completedAt?: string): string {
  if (!startedAt) return "--";
  const start = new Date(startedAt).getTime();
  const end = completedAt ? new Date(completedAt).getTime() : Date.now();
  const secs = Math.round((end - start) / 1000);
  if (secs < 60) return `${secs}s`;
  const mins = Math.floor(secs / 60);
  const rem = secs % 60;
  return `${mins}m ${rem}s`;
}

function truncate(str: string, max: number): string {
  return str.length > max ? str.slice(0, max) + "\u2026" : str;
}

/* ---------- Main component ---------- */

export function ProjectDetail() {
  // Read ID from URL path — useParams() returns stale "0" in static export
  const id = getPathSegment(1); // /projects/{id}
  const router = useRouter();
  const [activeTab, setActiveTab] = useState<TabKey>("sources");

  /* Test run state */
  const [testSourceId, setTestSourceId] = useState<string>("");
  const [testVersion, setTestVersion] = useState<string>("");
  const [testRunning, setTestRunning] = useState(false);

  /* Agent config local state */
  const [promptDraft, setPromptDraft] = useState<string | null>(null);
  const [rulesDraft, setRulesDraft] = useState<AgentRules | null>(null);
  const [saving, setSaving] = useState(false);

  /* Inline edit state */
  const [editingName, setEditingName] = useState(false);
  const [editingDesc, setEditingDesc] = useState(false);
  const [nameDraft, setNameDraft] = useState("");
  const [descDraft, setDescDraft] = useState("");
  const nameRef = useRef<HTMLInputElement>(null);
  const descRef = useRef<HTMLInputElement>(null);

  /* Dialog state */
  const [sourceCreateOpen, setSourceCreateOpen] = useState(false);
  const [editingSource, setEditingSource] = useState<Source | null>(null);
  const [deletingSourceId, setDeletingSourceId] = useState<string | null>(null);
  const [ctxCreateOpen, setCtxCreateOpen] = useState(false);
  const [deletingCtxId, setDeletingCtxId] = useState<string | null>(null);
  const [deletingProject, setDeletingProject] = useState(false);

  /* Data fetching */
  const { data, isLoading, mutate: mutateProject } = useSWR(`project-${id}`, () => projectsApi.get(id));
  const { data: sourcesData, mutate: mutateSources } = useSWR(
    `project-${id}-sources`,
    () => sourcesApi.listByProject(id),
  );
  const { data: ctxData, mutate: mutateCtx } = useSWR(
    `project-${id}-ctx`,
    () => ctxApi.list(id),
  );
  const { data: runsData, mutate: mutateRuns } = useSWR(
    activeTab === "agent" ? `project-${id}-runs` : null,
    () => agentApi.listRuns(id),
  );
  const { data: testReleasesData } = useSWR(
    testSourceId ? `source-${testSourceId}-releases` : null,
    () => releasesApi.listBySource(testSourceId, 1),
  );

  const saveName = useCallback(async () => {
    const p = data?.data;
    if (!p) return;
    const trimmed = nameDraft.trim();
    if (trimmed && trimmed !== p.name) {
      await projectsApi.update(id, { name: trimmed, description: p.description });
      mutateProject();
    }
    setEditingName(false);
  }, [nameDraft, data, id, mutateProject]);

  const saveDesc = useCallback(async () => {
    const p = data?.data;
    if (!p) return;
    const trimmed = descDraft.trim();
    if (trimmed !== (p.description ?? "")) {
      await projectsApi.update(id, { name: p.name, description: trimmed || undefined });
      mutateProject();
    }
    setEditingDesc(false);
  }, [descDraft, data, id, mutateProject]);

  /* Click-outside handlers */
  useEffect(() => {
    if (!editingName) return;
    const handler = (e: MouseEvent) => {
      if (nameRef.current && !nameRef.current.contains(e.target as Node)) saveName();
    };
    document.addEventListener("mousedown", handler);
    return () => document.removeEventListener("mousedown", handler);
  }, [editingName, saveName]);

  useEffect(() => {
    if (!editingDesc) return;
    const handler = (e: MouseEvent) => {
      if (descRef.current && !descRef.current.contains(e.target as Node)) saveDesc();
    };
    document.addEventListener("mousedown", handler);
    return () => document.removeEventListener("mousedown", handler);
  }, [editingDesc, saveDesc]);

  /* Handlers */
  const handleToggleSource = async (source: { id: string; provider: string; repository: string; poll_interval_seconds: number; enabled: boolean; config?: Record<string, unknown> }) => {
    await sourcesApi.update(source.id, {
      provider: source.provider,
      repository: source.repository,
      poll_interval_seconds: source.poll_interval_seconds,
      enabled: !source.enabled,
      config: source.config,
    });
    mutateSources();
  };

  const handleSaveAgentConfig = async () => {
    if (!project) return;
    setSaving(true);
    try {
      const input = {
        name: project.name,
        description: project.description,
        agent_prompt: promptDraft ?? project.agent_prompt,
        agent_rules: rulesDraft ?? project.agent_rules,
      };
      await projectsApi.update(id, input);
      mutateProject();
      setPromptDraft(null);
      setRulesDraft(null);
    } finally {
      setSaving(false);
    }
  };

  const handleTestRun = async () => {
    if (!testVersion) return;
    setTestRunning(true);
    try {
      await agentApi.triggerRun(id, testVersion);
      mutateRuns();
    } finally {
      setTestRunning(false);
    }
  };

  /* Loading / not found states */
  if (isLoading) {
    return (
      <div className="flex h-48 items-center justify-center" style={{ color: "#9ca3af" }}>
        Loading...
      </div>
    );
  }

  const project = data?.data;
  if (!project) {
    return (
      <div className="flex h-48 items-center justify-center" style={{ color: "#6b7280" }}>
        Project not found
      </div>
    );
  }

  /* Derived counts */
  const sourceCount = sourcesData?.data?.length;
  const ctxCount = ctxData?.data?.length;

  /* Current effective values */
  const currentPrompt = promptDraft ?? project.agent_prompt ?? "";
  const currentRules: AgentRules = rulesDraft ?? project.agent_rules ?? {};

  return (
    <div className="space-y-0">
      {/* ===== Header zone ===== */}
      <div
        className="-m-6 mb-0 border-b px-6 py-5"
        style={{ backgroundColor: "#ffffff", borderColor: "#e8e8e5" }}
      >
        {/* Back link */}
        <Link
          href="/projects"
          className="mb-4 inline-flex items-center gap-1.5 transition-colors hover:opacity-70"
          style={{
            fontFamily: "var(--font-dm-sans)",
            fontSize: "13px",
            color: "#6b7280",
          }}
        >
          <ArrowLeft size={14} />
          Back to Projects
        </Link>
        <div className="flex items-start justify-between">
          {/* Left: project info */}
          <div className="min-w-0 flex-1 flex items-start gap-4">
            <ProjectLogo name={project.name} sources={sourcesData?.data} size={48} />
            <div className="min-w-0 flex-1">
            {editingName ? (
              <input
                ref={nameRef}
                value={nameDraft}
                onChange={(e) => setNameDraft(e.target.value)}
                onKeyDown={(e) => {
                  if (e.key === "Enter") saveName();
                  if (e.key === "Escape") setEditingName(false);
                }}
                autoFocus
                className="w-full max-w-md rounded-md border px-2 py-1 text-[28px] font-bold leading-tight focus:outline-none focus:ring-1"
                style={{
                  fontFamily: "var(--font-fraunces), serif",
                  borderColor: "#e8e8e5",
                  color: "#111113",
                }}
              />
            ) : (
              <h1
                className="group flex cursor-pointer items-center gap-2 text-[28px] font-bold leading-tight"
                style={{ fontFamily: "var(--font-fraunces), serif" }}
                onClick={() => {
                  setNameDraft(project.name);
                  setEditingName(true);
                }}
              >
                {project.name}
                <Pencil className="h-4 w-4 shrink-0 opacity-0 transition-opacity group-hover:opacity-40" />
              </h1>
            )}
            {editingDesc ? (
              <input
                ref={descRef}
                value={descDraft}
                onChange={(e) => setDescDraft(e.target.value)}
                onKeyDown={(e) => {
                  if (e.key === "Enter") saveDesc();
                  if (e.key === "Escape") setEditingDesc(false);
                }}
                autoFocus
                placeholder="Add a description..."
                className="mt-1 w-full max-w-lg rounded-md border px-2 py-1 text-[14px] focus:outline-none focus:ring-1"
                style={{
                  fontFamily: "var(--font-dm-sans), sans-serif",
                  borderColor: "#e8e8e5",
                  color: "#6b7280",
                }}
              />
            ) : (
              <p
                className="group mt-1 flex cursor-pointer items-center gap-2 text-[14px]"
                style={{ fontFamily: "var(--font-dm-sans), sans-serif", color: "#6b7280" }}
                onClick={() => {
                  setDescDraft(project.description ?? "");
                  setEditingDesc(true);
                }}
              >
                {project.description || "Add a description..."}
                <Pencil className="h-3 w-3 shrink-0 opacity-0 transition-opacity group-hover:opacity-40" />
              </p>
            )}
            <p
              className="mt-2 text-[12px]"
              style={{ fontFamily: "var(--font-dm-sans), sans-serif", color: "#9ca3af" }}
            >
              {sourceCount !== undefined ? `${sourceCount} sources` : "-- sources"}
              {" \u00B7 "}
              {ctxCount !== undefined ? `${ctxCount} context sources` : "-- context sources"}
            </p>
            </div>
          </div>

          {/* Right: actions */}
          <div className="flex shrink-0 items-center gap-2 ml-4">
            <button
              onClick={() => setDeletingProject(true)}
              className="inline-flex items-center gap-1.5 rounded-md border px-3 py-1.5 text-[13px] font-medium transition-colors hover:bg-red-50"
              style={{
                fontFamily: "var(--font-dm-sans), sans-serif",
                borderColor: "#dc2626",
                color: "#dc2626",
              }}
            >
              <Trash2 className="h-3.5 w-3.5" />
              Delete
            </button>
          </div>
        </div>
      </div>

      {/* ===== Tab bar ===== */}
      <div
        className="-mx-6 border-b px-6"
        style={{ borderColor: "#e8e8e5", backgroundColor: "#ffffff" }}
      >
        <div className="flex gap-0">
          {tabs.map((tab) => {
            const isActive = activeTab === tab.key;
            return (
              <button
                key={tab.key}
                onClick={() => setActiveTab(tab.key)}
                className="relative px-4 py-2.5 text-[13px] font-medium transition-colors"
                style={{
                  fontFamily: "var(--font-dm-sans), sans-serif",
                  color: isActive ? "#111113" : "#9ca3af",
                }}
              >
                {tab.label}
                {isActive && (
                  <span
                    className="absolute bottom-0 left-4 right-4 h-[2px]"
                    style={{ backgroundColor: "#e8601a" }}
                  />
                )}
              </button>
            );
          })}
        </div>
      </div>

      {/* ===== Tab content ===== */}
      <div className="pt-6">
        {/* --- Sources tab --- */}
        {activeTab === "sources" && (
          <div>
            <div className="mb-4 flex items-center justify-between">
              <SectionLabel>Sources</SectionLabel>
              <button
                onClick={() => setSourceCreateOpen(true)}
                className="inline-flex items-center gap-1.5 rounded-md border px-3 py-1.5 text-[13px] font-medium transition-colors hover:bg-[#f3f3f1]"
                style={{
                  fontFamily: "var(--font-dm-sans), sans-serif",
                  borderColor: "#e8e8e5",
                  color: "#374151",
                }}
              >
                <Plus className="h-3.5 w-3.5" />
                Add Source
              </button>
            </div>

            {sourcesData?.data && sourcesData.data.length > 0 ? (
              <div className="overflow-hidden rounded-md border" style={{ borderColor: "#e8e8e5" }}>
                <table className="w-full text-[13px]" style={{ fontFamily: "var(--font-dm-sans), sans-serif" }}>
                  <thead>
                    <tr style={{ backgroundColor: "#fafaf9" }}>
                      <th className="px-4 py-2 text-left text-[11px] font-medium uppercase tracking-[0.08em]" style={{ color: "#9ca3af" }}>Provider</th>
                      <th className="px-4 py-2 text-left text-[11px] font-medium uppercase tracking-[0.08em]" style={{ color: "#9ca3af" }}>Repository</th>
                      <th className="px-4 py-2 text-left text-[11px] font-medium uppercase tracking-[0.08em]" style={{ color: "#9ca3af" }}>Interval</th>
                      <th className="px-4 py-2 text-left text-[11px] font-medium uppercase tracking-[0.08em]" style={{ color: "#9ca3af" }}>Status</th>
                      <th className="px-4 py-2 text-left text-[11px] font-medium uppercase tracking-[0.08em]" style={{ color: "#9ca3af" }}>Last polled</th>
                      <th className="px-4 py-2 text-right text-[11px] font-medium uppercase tracking-[0.08em]" style={{ color: "#9ca3af" }}></th>
                    </tr>
                  </thead>
                  <tbody>
                    {sourcesData.data.map((source) => (
                      <tr key={source.id} className="border-t" style={{ borderColor: "#e8e8e5" }}>
                        <td className="px-4 py-3">
                          <ProviderBadge provider={source.provider} />
                        </td>
                        <td className="px-4 py-3">
                          <span style={{ fontFamily: "'JetBrains Mono', monospace", fontSize: "12px" }}>
                            {source.repository}
                          </span>
                        </td>
                        <td className="px-4 py-3" style={{ color: "#6b7280" }}>
                          {formatInterval(source.poll_interval_seconds)}
                        </td>
                        <td className="px-4 py-3">
                          <div className="flex items-center gap-2">
                            <button
                              onClick={() => handleToggleSource(source)}
                              className="relative inline-flex h-5 w-9 shrink-0 cursor-pointer items-center rounded-full transition-colors duration-200"
                              style={{ backgroundColor: source.last_error ? "#dc2626" : source.enabled ? "#16a34a" : "#d1d5db" }}
                              title={source.last_error ? `Error: ${source.last_error}` : source.enabled ? "Disable polling" : "Enable polling"}
                            >
                              <span
                                className="inline-block h-3.5 w-3.5 rounded-full bg-white shadow transition-transform duration-200"
                                style={{ transform: source.enabled ? "translateX(18px)" : "translateX(3px)" }}
                              />
                            </button>
                            {source.last_error ? (
                              <span className="text-[11px] text-red-600 max-w-[200px] truncate" title={source.last_error}>
                                {source.last_error}
                              </span>
                            ) : null}
                          </div>
                        </td>
                        <td className="px-4 py-3" style={{ color: "#9ca3af" }}>
                          {source.last_polled_at
                            ? new Date(source.last_polled_at).toLocaleString()
                            : "Never"}
                        </td>
                        <td className="px-4 py-3 text-right">
                          <div className="flex items-center justify-end gap-2">
                            <button
                              onClick={() => setEditingSource(source)}
                              className="rounded p-1 text-[#9ca3af] transition-colors hover:bg-[#f3f3f1] hover:text-[#111113]"
                              title="Edit source"
                            >
                              <Pencil className="h-4 w-4" />
                            </button>
                            <button
                              onClick={() => setDeletingSourceId(source.id)}
                              className="rounded p-1 text-[#9ca3af] transition-colors hover:bg-red-50 hover:text-red-600"
                              title="Delete source"
                            >
                              <Trash2 className="h-4 w-4" />
                            </button>
                          </div>
                        </td>
                      </tr>
                    ))}
                  </tbody>
                </table>
              </div>
            ) : (
              <div
                className="flex h-32 items-center justify-center rounded-md border text-[13px]"
                style={{ borderColor: "#e8e8e5", color: "#9ca3af" }}
              >
                No sources configured
              </div>
            )}
          </div>
        )}

        {/* --- Context Sources tab --- */}
        {activeTab === "context" && (
          <div>
            <div className="mb-4 flex items-center justify-between">
              <SectionLabel>Context Sources</SectionLabel>
              <button
                onClick={() => setCtxCreateOpen(true)}
                className="inline-flex items-center gap-1.5 rounded-md border px-3 py-1.5 text-[13px] font-medium transition-colors hover:bg-[#f3f3f1]"
                style={{
                  fontFamily: "var(--font-dm-sans), sans-serif",
                  borderColor: "#e8e8e5",
                  color: "#374151",
                }}
              >
                <Plus className="h-3.5 w-3.5" />
                Add Context Source
              </button>
            </div>

            {ctxData?.data && ctxData.data.length > 0 ? (
              <div className="overflow-hidden rounded-md border" style={{ borderColor: "#e8e8e5" }}>
                <table className="w-full text-[13px]" style={{ fontFamily: "var(--font-dm-sans), sans-serif" }}>
                  <thead>
                    <tr style={{ backgroundColor: "#fafaf9" }}>
                      <th className="px-4 py-2 text-left text-[11px] font-medium uppercase tracking-[0.08em]" style={{ color: "#9ca3af" }}>Type</th>
                      <th className="px-4 py-2 text-left text-[11px] font-medium uppercase tracking-[0.08em]" style={{ color: "#9ca3af" }}>Name</th>
                      <th className="px-4 py-2 text-left text-[11px] font-medium uppercase tracking-[0.08em]" style={{ color: "#9ca3af" }}>Config URL</th>
                      <th className="px-4 py-2 text-right text-[11px] font-medium uppercase tracking-[0.08em]" style={{ color: "#9ca3af" }}></th>
                    </tr>
                  </thead>
                  <tbody>
                    {ctxData.data.map((ctx) => (
                      <tr key={ctx.id} className="border-t" style={{ borderColor: "#e8e8e5" }}>
                        <td className="px-4 py-3">
                          <span
                            className="inline-flex items-center rounded-full px-2 py-0.5 text-[11px] font-medium leading-none"
                            style={{ backgroundColor: "#f3f3f1", color: "#374151" }}
                          >
                            {ctx.type}
                          </span>
                        </td>
                        <td className="px-4 py-3 font-medium" style={{ color: "#111113" }}>
                          {ctx.name}
                        </td>
                        <td className="px-4 py-3" style={{ color: "#9ca3af" }}>
                          <span
                            className="text-[12px]"
                            style={{ fontFamily: "'JetBrains Mono', monospace" }}
                            title={typeof ctx.config?.url === "string" ? ctx.config.url : ""}
                          >
                            {typeof ctx.config?.url === "string"
                              ? truncate(ctx.config.url, 50)
                              : "--"}
                          </span>
                        </td>
                        <td className="px-4 py-3 text-right">
                          <button
                            onClick={() => setDeletingCtxId(ctx.id)}
                            className="rounded p-1 text-[#9ca3af] transition-colors hover:bg-red-50 hover:text-red-600"
                            title="Delete context source"
                          >
                            <Trash2 className="h-4 w-4" />
                          </button>
                        </td>
                      </tr>
                    ))}
                  </tbody>
                </table>
              </div>
            ) : (
              <div
                className="flex h-32 items-center justify-center rounded-md border text-[13px]"
                style={{ borderColor: "#e8e8e5", color: "#9ca3af" }}
              >
                No context sources configured
              </div>
            )}
          </div>
        )}

        {/* --- Semantic Release Settings tab --- */}
        {activeTab === "agent" && (
          <div className="space-y-6">
            {/* Card 1: Trigger Rules */}
            <div
              className="rounded-lg border p-5"
              style={{ borderColor: "#e8e8e5", backgroundColor: "#ffffff" }}
            >
              <SectionLabel className="mb-1">Trigger Rules</SectionLabel>
              <p className="mb-4 text-[12px]" style={{ color: "#9ca3af" }}>
                Automatically run the agent when new releases match these conditions.
              </p>
              <div className="space-y-3">
                <label className="flex items-center gap-2.5 text-[13px]" style={{ color: "#374151" }}>
                  <input
                    type="checkbox"
                    checked={currentRules.on_major_release ?? false}
                    onChange={(e) =>
                      setRulesDraft({ ...currentRules, on_major_release: e.target.checked })
                    }
                    className="h-4 w-4 rounded border accent-[#e8601a]"
                    style={{ borderColor: "#e8e8e5" }}
                  />
                  Major release
                  <span className="text-[11px]" style={{ color: "#9ca3af" }}>
                    (e.g. 1.x &rarr; 2.x)
                  </span>
                </label>
                <label className="flex items-center gap-2.5 text-[13px]" style={{ color: "#374151" }}>
                  <input
                    type="checkbox"
                    checked={currentRules.on_minor_release ?? false}
                    onChange={(e) =>
                      setRulesDraft({ ...currentRules, on_minor_release: e.target.checked })
                    }
                    className="h-4 w-4 rounded border accent-[#e8601a]"
                    style={{ borderColor: "#e8e8e5" }}
                  />
                  Minor release
                  <span className="text-[11px]" style={{ color: "#9ca3af" }}>
                    (e.g. 1.1 &rarr; 1.2)
                  </span>
                </label>
                <label className="flex items-center gap-2.5 text-[13px]" style={{ color: "#374151" }}>
                  <input
                    type="checkbox"
                    checked={currentRules.on_security_patch ?? false}
                    onChange={(e) =>
                      setRulesDraft({ ...currentRules, on_security_patch: e.target.checked })
                    }
                    className="h-4 w-4 rounded border accent-[#e8601a]"
                    style={{ borderColor: "#e8e8e5" }}
                  />
                  Security patch
                  <span className="text-[11px]" style={{ color: "#9ca3af" }}>
                    (contains security/CVE keywords)
                  </span>
                </label>
                <div className="pt-2">
                  <label className="mb-1.5 block text-[13px]" style={{ color: "#374151" }}>
                    Version pattern
                  </label>
                  <input
                    type="text"
                    value={currentRules.version_pattern ?? ""}
                    onChange={(e) =>
                      setRulesDraft({ ...currentRules, version_pattern: e.target.value })
                    }
                    placeholder="e.g. ^v\\d+\\.\\d+\\.\\d+$"
                    className="w-full max-w-md rounded-md border px-3 py-1.5 text-[13px] placeholder:text-[#9ca3af] focus:outline-none focus:ring-1"
                    style={{
                      fontFamily: "'JetBrains Mono', monospace",
                      backgroundColor: "#fafaf9",
                      borderColor: "#e8e8e5",
                      color: "#111113",
                    }}
                  />
                  <p className="mt-1 text-[11px]" style={{ color: "#9ca3af" }}>
                    Optional regex to filter which versions trigger agent runs.
                  </p>
                </div>
              </div>
            </div>

            {/* Card 2: Agent Prompt */}
            <div
              className="rounded-lg border p-5"
              style={{ borderColor: "#e8e8e5", backgroundColor: "#ffffff" }}
            >
              <SectionLabel className="mb-1">Agent Prompt</SectionLabel>
              <p className="mb-3 text-[12px]" style={{ color: "#9ca3af" }}>
                Custom instructions for the agent when analyzing releases.
              </p>
              <textarea
                value={currentPrompt}
                onChange={(e) => setPromptDraft(e.target.value)}
                rows={5}
                placeholder="Using default agent prompt."
                className="w-full resize-y rounded-md border px-3 py-2 text-[13px] placeholder:text-[#9ca3af] focus:outline-none focus:ring-1"
                style={{
                  fontFamily: "var(--font-dm-sans), sans-serif",
                  backgroundColor: "#fafaf9",
                  borderColor: "#e8e8e5",
                  color: "#111113",
                }}
              />
              <div className="mt-3 flex justify-end">
                <button
                  onClick={handleSaveAgentConfig}
                  disabled={saving || (promptDraft === null && rulesDraft === null)}
                  className="rounded-md px-4 py-1.5 text-[13px] font-medium text-white transition-colors disabled:opacity-40"
                  style={{ backgroundColor: "#e8601a" }}
                >
                  {saving ? "Saving..." : "Save Settings"}
                </button>
              </div>
            </div>

            {/* Card 3: Test Run */}
            <div
              className="rounded-lg border p-5"
              style={{ borderColor: "#e8e8e5", backgroundColor: "#ffffff" }}
            >
              <SectionLabel className="mb-1">Test Run</SectionLabel>
              <p className="mb-4 text-[12px]" style={{ color: "#9ca3af" }}>
                Trigger a one-off agent run to test your configuration against a specific release.
              </p>
              <div className="flex flex-wrap items-end gap-3">
                <div className="min-w-[200px] flex-1">
                  <label className="mb-1.5 block text-[13px]" style={{ color: "#374151" }}>
                    Source
                  </label>
                  <select
                    value={testSourceId}
                    onChange={(e) => {
                      setTestSourceId(e.target.value);
                      setTestVersion("");
                    }}
                    className="w-full rounded-md border px-3 py-1.5 text-[13px] focus:outline-none focus:ring-1"
                    style={{
                      fontFamily: "var(--font-dm-sans), sans-serif",
                      backgroundColor: "#fafaf9",
                      borderColor: "#e8e8e5",
                      color: testSourceId ? "#111113" : "#9ca3af",
                    }}
                  >
                    <option value="">Select a source...</option>
                    {sourcesData?.data?.map((s) => (
                      <option key={s.id} value={s.id}>
                        {s.provider}: {s.repository}
                      </option>
                    ))}
                  </select>
                </div>
                <div className="min-w-[200px] flex-1">
                  <label className="mb-1.5 block text-[13px]" style={{ color: "#374151" }}>
                    Version
                  </label>
                  <select
                    value={testVersion}
                    onChange={(e) => setTestVersion(e.target.value)}
                    disabled={!testSourceId}
                    className="w-full rounded-md border px-3 py-1.5 text-[13px] focus:outline-none focus:ring-1 disabled:opacity-50"
                    style={{
                      fontFamily: "'JetBrains Mono', monospace",
                      backgroundColor: "#fafaf9",
                      borderColor: "#e8e8e5",
                      color: testVersion ? "#111113" : "#9ca3af",
                    }}
                  >
                    <option value="">
                      {testSourceId ? "Select a version..." : "Choose a source first"}
                    </option>
                    {testReleasesData?.data?.map((r) => (
                      <option key={r.id} value={r.version}>
                        {r.version}
                      </option>
                    ))}
                  </select>
                </div>
                <button
                  onClick={handleTestRun}
                  disabled={testRunning || !testVersion}
                  className="inline-flex items-center gap-1.5 rounded-md px-4 py-1.5 text-[13px] font-medium text-white transition-colors disabled:opacity-40"
                  style={{ backgroundColor: "#e8601a" }}
                >
                  <Play className="h-3.5 w-3.5" />
                  {testRunning ? "Running..." : "Run Test"}
                </button>
              </div>
            </div>

            {/* Card 4: Run History */}
            <div>
              <SectionLabel className="mb-3">Run History</SectionLabel>
              {runsData?.data && runsData.data.length > 0 ? (
                <div className="overflow-hidden rounded-md border" style={{ borderColor: "#e8e8e5" }}>
                  <table className="w-full text-[13px]" style={{ fontFamily: "var(--font-dm-sans), sans-serif" }}>
                    <thead>
                      <tr style={{ backgroundColor: "#fafaf9" }}>
                        <th className="px-4 py-2 text-left text-[11px] font-medium uppercase tracking-[0.08em]" style={{ color: "#9ca3af" }}>Trigger</th>
                        <th className="px-4 py-2 text-left text-[11px] font-medium uppercase tracking-[0.08em]" style={{ color: "#9ca3af" }}>Status</th>
                        <th className="px-4 py-2 text-left text-[11px] font-medium uppercase tracking-[0.08em]" style={{ color: "#9ca3af" }}>Started</th>
                        <th className="px-4 py-2 text-left text-[11px] font-medium uppercase tracking-[0.08em]" style={{ color: "#9ca3af" }}>Duration</th>
                        <th className="px-4 py-2 text-left text-[11px] font-medium uppercase tracking-[0.08em]" style={{ color: "#9ca3af" }}>Semantic Release</th>
                      </tr>
                    </thead>
                    <tbody>
                      {runsData.data.map((run) => (
                        <tr key={run.id} className="border-t" style={{ borderColor: "#e8e8e5" }}>
                          <td className="px-4 py-3" style={{ color: "#374151" }}>
                            {run.trigger}
                          </td>
                          <td className="px-4 py-3">
                            <div className="flex items-center gap-2">
                              <StatusDot status={run.status} />
                              <span style={{ color: "#6b7280" }}>{run.status}</span>
                            </div>
                          </td>
                          <td className="px-4 py-3" style={{ color: "#9ca3af" }}>
                            {run.started_at
                              ? new Date(run.started_at).toLocaleString()
                              : "Pending"}
                          </td>
                          <td className="px-4 py-3" style={{ color: "#9ca3af" }}>
                            {formatDuration(run.started_at, run.completed_at)}
                          </td>
                          <td className="px-4 py-3">
                            {run.semantic_release_id ? (
                              <Link
                                href={`/projects/${id}/semantic-releases/${run.semantic_release_id}`}
                                className="text-[13px] font-medium underline"
                                style={{ color: "#e8601a" }}
                              >
                                View
                              </Link>
                            ) : (
                              <span style={{ color: "#9ca3af" }}>--</span>
                            )}
                          </td>
                        </tr>
                      ))}
                    </tbody>
                  </table>
                </div>
              ) : (
                <div
                  className="flex h-32 items-center justify-center rounded-md border text-[13px]"
                  style={{ borderColor: "#e8e8e5", color: "#9ca3af" }}
                >
                  No agent runs yet
                </div>
              )}
            </div>
          </div>
        )}
      </div>

      {/* Source create dialog */}
      <Dialog open={sourceCreateOpen} onOpenChange={setSourceCreateOpen}>
        <DialogContent className="sm:max-w-lg">
          <DialogHeader><DialogTitle>Add Source</DialogTitle></DialogHeader>
          <SourceForm
            title="Add Source"
            projectId={id}
            onSubmit={async (input) => { const res = await sourcesApi.create(id, input); if (res.data?.id) sourcesApi.poll(res.data.id).catch(() => {}); }}
            onSuccess={() => { setSourceCreateOpen(false); mutateSources(); }}
            onCancel={() => setSourceCreateOpen(false)}
          />
        </DialogContent>
      </Dialog>

      {/* Source edit dialog */}
      <Dialog open={!!editingSource} onOpenChange={(open) => { if (!open) setEditingSource(null); }}>
        <DialogContent className="sm:max-w-lg">
          <DialogHeader><DialogTitle>Edit Source</DialogTitle></DialogHeader>
          {editingSource && (
            <SourceForm
              key={editingSource.id}
              title="Edit Source"
              initial={editingSource}
              onSubmit={async (input) => { await sourcesApi.update(editingSource.id, input); }}
              onSuccess={() => { setEditingSource(null); mutateSources(); }}
              onCancel={() => setEditingSource(null)}
            />
          )}
        </DialogContent>
      </Dialog>

      {/* Source delete dialog */}
      <ConfirmDialog
        open={!!deletingSourceId}
        onOpenChange={(open) => { if (!open) setDeletingSourceId(null); }}
        title="Delete Source"
        description="This will permanently delete this source and its releases."
        onConfirm={async () => { if (deletingSourceId) { await sourcesApi.delete(deletingSourceId); mutateSources(); } }}
      />

      {/* Context source create dialog */}
      <Dialog open={ctxCreateOpen} onOpenChange={setCtxCreateOpen}>
        <DialogContent className="sm:max-w-lg">
          <DialogHeader><DialogTitle>Add Context Source</DialogTitle></DialogHeader>
          <NewContextSourceForm
            projectId={id}
            onSuccess={() => { setCtxCreateOpen(false); mutateCtx(); }}
            onCancel={() => setCtxCreateOpen(false)}
          />
        </DialogContent>
      </Dialog>

      {/* Context source delete dialog */}
      <ConfirmDialog
        open={!!deletingCtxId}
        onOpenChange={(open) => { if (!open) setDeletingCtxId(null); }}
        title="Delete Context Source"
        description="This will permanently delete this context source."
        onConfirm={async () => { if (deletingCtxId) { await ctxApi.delete(deletingCtxId); mutateCtx(); } }}
      />

      {/* Project delete dialog */}
      <ConfirmDialog
        open={deletingProject}
        onOpenChange={setDeletingProject}
        title="Delete Project"
        description="This will permanently delete this project, including all sources, releases, and subscriptions."
        onConfirm={async () => { await projectsApi.delete(id); router.push("/projects"); }}
      />
    </div>
  );
}
