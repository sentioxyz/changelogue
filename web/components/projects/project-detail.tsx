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
import { useTranslation } from "@/lib/i18n/context";

/* ---------- Tabs ---------- */

type TabKey = "sources" | "context" | "agent";

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
  const { t } = useTranslation();
  // Read ID from URL path — useParams() returns stale "0" in static export
  const id = getPathSegment(1); // /projects/{id}
  const router = useRouter();
  const [activeTab, setActiveTab] = useState<TabKey>("sources");

  const tabs = [
    { key: "sources" as TabKey, label: t("projects.detail.tabSources") },
    { key: "context" as TabKey, label: t("projects.detail.tabContext") },
    { key: "agent" as TabKey, label: t("projects.detail.tabAgent") },
  ];

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
      <div className="flex h-48 items-center justify-center text-text-muted">
        {t("projects.detail.loading")}
      </div>
    );
  }

  const project = data?.data;
  if (!project) {
    return (
      <div className="flex h-48 items-center justify-center text-text-secondary">
        {t("projects.detail.notFound")}
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
        className="-m-6 mb-0 border-b px-6 py-5 bg-surface"
        style={{ borderColor: "var(--border)" }}
      >
        {/* Back link */}
        <Link
          href="/projects"
          className="mb-4 inline-flex items-center gap-1.5 transition-colors hover:opacity-70 text-text-secondary"
          style={{
            fontFamily: "var(--font-dm-sans)",
            fontSize: "13px",
          }}
        >
          <ArrowLeft size={14} />
          {t("projects.detail.backToProjects")}
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
                className="w-full max-w-md rounded-md border px-2 py-1 text-[28px] font-bold leading-tight focus:outline-none focus:ring-1 border-border text-foreground"
                style={{
                  fontFamily: "var(--font-fraunces), serif",
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
                placeholder={t("projects.detail.addDescription")}
                className="mt-1 w-full max-w-lg rounded-md border px-2 py-1 text-[14px] focus:outline-none focus:ring-1 border-border text-text-secondary"
                style={{
                  fontFamily: "var(--font-dm-sans), sans-serif",
                }}
              />
            ) : (
              <p
                className="group mt-1 flex cursor-pointer items-center gap-2 text-[14px] text-text-secondary"
                style={{ fontFamily: "var(--font-dm-sans), sans-serif" }}
                onClick={() => {
                  setDescDraft(project.description ?? "");
                  setEditingDesc(true);
                }}
              >
                {project.description || t("projects.detail.addDescription")}
                <Pencil className="h-3 w-3 shrink-0 opacity-0 transition-opacity group-hover:opacity-40" />
              </p>
            )}
            <p
              className="mt-2 text-[12px] text-text-muted"
              style={{ fontFamily: "var(--font-dm-sans), sans-serif" }}
            >
              {sourceCount !== undefined ? t("projects.detail.sourcesCount").replace("{count}", String(sourceCount)) : `-- ${t("projects.detail.tabSources").toLowerCase()}`}
              {" \u00B7 "}
              {ctxCount !== undefined ? t("projects.detail.contextSourcesCount").replace("{count}", String(ctxCount)) : `-- ${t("projects.detail.tabContext").toLowerCase()}`}
            </p>
            </div>
          </div>

          {/* Right: actions */}
          <div className="flex shrink-0 items-center gap-2 ml-4">
            <button
              onClick={() => setDeletingProject(true)}
              className="inline-flex items-center gap-1.5 rounded-md border px-3 py-1.5 text-[13px] font-medium transition-colors hover:bg-error-bg"
              style={{
                fontFamily: "var(--font-dm-sans), sans-serif",
                borderColor: "var(--error-text)",
                color: "var(--error-text)",
              }}
            >
              <Trash2 className="h-3.5 w-3.5" />
              {t("projects.detail.delete")}
            </button>
          </div>
        </div>
      </div>

      {/* ===== Tab bar ===== */}
      <div
        className="-mx-6 border-b px-6 bg-surface"
        style={{ borderColor: "var(--border)" }}
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
                  color: isActive ? "var(--foreground)" : "var(--text-muted)",
                }}
              >
                {tab.label}
                {isActive && (
                  <span
                    className="absolute bottom-0 left-4 right-4 h-[2px] bg-beacon-accent"
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
              <SectionLabel>{t("projects.detail.tabSources")}</SectionLabel>
              <button
                onClick={() => setSourceCreateOpen(true)}
                className="inline-flex items-center gap-1.5 rounded-md border px-3 py-1.5 text-[13px] font-medium transition-colors hover:bg-mono-bg border-border text-secondary-foreground"
                style={{
                  fontFamily: "var(--font-dm-sans), sans-serif",
                }}
              >
                <Plus className="h-3.5 w-3.5" />
                {t("projects.detail.addSourceBtn")}
              </button>
            </div>

            {sourcesData?.data && sourcesData.data.length > 0 ? (
              <div className="overflow-hidden rounded-md border" style={{ borderColor: "var(--border)" }}>
                <table className="w-full text-[13px]" style={{ fontFamily: "var(--font-dm-sans), sans-serif" }}>
                  <thead>
                    <tr style={{ backgroundColor: "var(--background)" }}>
                      <th className="px-4 py-2 text-left text-[11px] font-medium uppercase tracking-[0.08em] text-text-muted">{t("projects.detail.provider")}</th>
                      <th className="px-4 py-2 text-left text-[11px] font-medium uppercase tracking-[0.08em] text-text-muted">{t("projects.detail.repository")}</th>
                      <th className="px-4 py-2 text-left text-[11px] font-medium uppercase tracking-[0.08em] text-text-muted">{t("projects.detail.interval")}</th>
                      <th className="px-4 py-2 text-left text-[11px] font-medium uppercase tracking-[0.08em] text-text-muted">{t("projects.detail.status")}</th>
                      <th className="px-4 py-2 text-left text-[11px] font-medium uppercase tracking-[0.08em] text-text-muted">{t("projects.detail.lastPolled")}</th>
                      <th className="px-4 py-2 text-right text-[11px] font-medium uppercase tracking-[0.08em] text-text-muted"></th>
                    </tr>
                  </thead>
                  <tbody>
                    {sourcesData.data.map((source) => (
                      <tr key={source.id} className="border-t" style={{ borderColor: "var(--border)" }}>
                        <td className="px-4 py-3">
                          <ProviderBadge provider={source.provider} />
                        </td>
                        <td className="px-4 py-3">
                          <span style={{ fontFamily: "'JetBrains Mono', monospace", fontSize: "12px" }}>
                            {source.repository}
                          </span>
                        </td>
                        <td className="px-4 py-3 text-text-secondary">
                          {formatInterval(source.poll_interval_seconds)}
                        </td>
                        <td className="px-4 py-3">
                          <div className="flex items-center gap-2">
                            <button
                              onClick={() => handleToggleSource(source)}
                              className="relative inline-flex h-5 w-9 shrink-0 cursor-pointer items-center rounded-full transition-colors duration-200"
                              style={{ backgroundColor: source.last_error ? "var(--error-text)" : source.enabled ? "var(--status-completed)" : "var(--text-muted)" }}
                              title={source.last_error ? `Error: ${source.last_error}` : source.enabled ? t("projects.detail.disablePolling") : t("projects.detail.enablePolling")}
                            >
                              <span
                                className="inline-block h-3.5 w-3.5 rounded-full bg-white shadow transition-transform duration-200"
                                style={{ transform: source.enabled ? "translateX(18px)" : "translateX(3px)" }}
                              />
                            </button>
                            {source.last_error ? (
                              <span className="text-[11px] text-error-text max-w-[200px] truncate" title={source.last_error}>
                                {source.last_error}
                              </span>
                            ) : null}
                          </div>
                        </td>
                        <td className="px-4 py-3 text-text-muted">
                          {source.last_polled_at
                            ? new Date(source.last_polled_at).toLocaleString()
                            : t("projects.detail.never")}
                        </td>
                        <td className="px-4 py-3 text-right">
                          <div className="flex items-center justify-end gap-2">
                            <button
                              onClick={() => setEditingSource(source)}
                              className="rounded p-1 text-text-muted transition-colors hover:bg-mono-bg hover:text-foreground"
                              title={t("projects.detail.editSource")}
                            >
                              <Pencil className="h-4 w-4" />
                            </button>
                            <button
                              onClick={() => setDeletingSourceId(source.id)}
                              className="rounded p-1 text-text-muted transition-colors hover:bg-red-50 hover:text-red-600"
                              title={t("projects.detail.deleteSource")}
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
                className="flex h-32 items-center justify-center rounded-md border text-[13px] border-border text-text-muted"
              >
                {t("projects.detail.noSources")}
              </div>
            )}
          </div>
        )}

        {/* --- Context Sources tab --- */}
        {activeTab === "context" && (
          <div>
            <div className="mb-4 flex items-center justify-between">
              <SectionLabel>{t("projects.detail.tabContext")}</SectionLabel>
              <button
                onClick={() => setCtxCreateOpen(true)}
                className="inline-flex items-center gap-1.5 rounded-md border px-3 py-1.5 text-[13px] font-medium transition-colors hover:bg-mono-bg border-border text-secondary-foreground"
                style={{
                  fontFamily: "var(--font-dm-sans), sans-serif",
                }}
              >
                <Plus className="h-3.5 w-3.5" />
                {t("projects.detail.addContextSource")}
              </button>
            </div>

            {ctxData?.data && ctxData.data.length > 0 ? (
              <div className="overflow-hidden rounded-md border" style={{ borderColor: "var(--border)" }}>
                <table className="w-full text-[13px]" style={{ fontFamily: "var(--font-dm-sans), sans-serif" }}>
                  <thead>
                    <tr style={{ backgroundColor: "var(--background)" }}>
                      <th className="px-4 py-2 text-left text-[11px] font-medium uppercase tracking-[0.08em] text-text-muted">{t("projects.detail.ctxType")}</th>
                      <th className="px-4 py-2 text-left text-[11px] font-medium uppercase tracking-[0.08em] text-text-muted">{t("projects.detail.ctxName")}</th>
                      <th className="px-4 py-2 text-left text-[11px] font-medium uppercase tracking-[0.08em] text-text-muted">{t("projects.detail.ctxConfigUrl")}</th>
                      <th className="px-4 py-2 text-right text-[11px] font-medium uppercase tracking-[0.08em] text-text-muted"></th>
                    </tr>
                  </thead>
                  <tbody>
                    {ctxData.data.map((ctx) => (
                      <tr key={ctx.id} className="border-t" style={{ borderColor: "var(--border)" }}>
                        <td className="px-4 py-3">
                          <span
                            className="inline-flex items-center rounded-full px-2 py-0.5 text-[11px] font-medium leading-none bg-mono-bg text-secondary-foreground"
                          >
                            {ctx.type}
                          </span>
                        </td>
                        <td className="px-4 py-3 font-medium text-foreground">
                          {ctx.name}
                        </td>
                        <td className="px-4 py-3 text-text-muted">
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
                            className="rounded p-1 text-text-muted transition-colors hover:bg-red-50 hover:text-red-600"
                            title={t("projects.detail.deleteContextSource")}
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
                className="flex h-32 items-center justify-center rounded-md border text-[13px] border-border text-text-muted"
              >
                {t("projects.detail.noContextSources")}
              </div>
            )}
          </div>
        )}

        {/* --- Semantic Release Settings tab --- */}
        {activeTab === "agent" && (
          <div className="space-y-6">
            {/* Card 1: Trigger Rules */}
            <div
              className="rounded-lg border p-5 bg-surface"
              style={{ borderColor: "var(--border)" }}
            >
              <SectionLabel className="mb-1">{t("projects.detail.triggerRules")}</SectionLabel>
              <p className="mb-4 text-[12px] text-text-muted">
                {t("projects.detail.triggerRulesDesc")}
              </p>
              <div className="space-y-3">
                <label className="flex items-center gap-2.5 text-[13px] text-secondary-foreground">
                  <input
                    type="checkbox"
                    checked={currentRules.on_major_release ?? false}
                    onChange={(e) =>
                      setRulesDraft({ ...currentRules, on_major_release: e.target.checked })
                    }
                    className="h-4 w-4 rounded border accent-[#e8601a]"
                    style={{ borderColor: "var(--border)" }}
                  />
                  {t("projects.detail.majorRelease")}
                  <span className="text-[11px] text-text-muted">
                    {t("projects.detail.majorReleaseHint")}
                  </span>
                </label>
                <label className="flex items-center gap-2.5 text-[13px] text-secondary-foreground">
                  <input
                    type="checkbox"
                    checked={currentRules.on_minor_release ?? false}
                    onChange={(e) =>
                      setRulesDraft({ ...currentRules, on_minor_release: e.target.checked })
                    }
                    className="h-4 w-4 rounded border accent-[#e8601a]"
                    style={{ borderColor: "var(--border)" }}
                  />
                  {t("projects.detail.minorRelease")}
                  <span className="text-[11px] text-text-muted">
                    {t("projects.detail.minorReleaseHint")}
                  </span>
                </label>
                <label className="flex items-center gap-2.5 text-[13px] text-secondary-foreground">
                  <input
                    type="checkbox"
                    checked={currentRules.on_security_patch ?? false}
                    onChange={(e) =>
                      setRulesDraft({ ...currentRules, on_security_patch: e.target.checked })
                    }
                    className="h-4 w-4 rounded border accent-[#e8601a]"
                    style={{ borderColor: "var(--border)" }}
                  />
                  {t("projects.detail.securityPatch")}
                  <span className="text-[11px] text-text-muted">
                    {t("projects.detail.securityPatchHint")}
                  </span>
                </label>
                <div className="pt-2">
                  <label className="mb-1.5 block text-[13px] text-secondary-foreground">
                    {t("projects.detail.versionPattern")}
                  </label>
                  <input
                    type="text"
                    value={currentRules.version_pattern ?? ""}
                    onChange={(e) =>
                      setRulesDraft({ ...currentRules, version_pattern: e.target.value })
                    }
                    placeholder="e.g. ^v\\d+\\.\\d+\\.\\d+$"
                    className="w-full max-w-md rounded-md border px-3 py-1.5 text-[13px] placeholder:text-text-muted focus:outline-none focus:ring-1 bg-background border-border text-foreground"
                    style={{
                      fontFamily: "'JetBrains Mono', monospace",
                    }}
                  />
                  <p className="mt-1 text-[11px] text-text-muted">
                    {t("projects.detail.versionPatternHint")}
                  </p>
                </div>
              </div>
            </div>

            {/* Card 2: Agent Prompt */}
            <div
              className="rounded-lg border p-5 bg-surface"
              style={{ borderColor: "var(--border)" }}
            >
              <SectionLabel className="mb-1">{t("projects.detail.agentPrompt")}</SectionLabel>
              <p className="mb-3 text-[12px] text-text-muted">
                {t("projects.detail.agentPromptDesc")}
              </p>
              <textarea
                value={currentPrompt}
                onChange={(e) => setPromptDraft(e.target.value)}
                rows={5}
                placeholder={t("projects.detail.agentPromptPlaceholder")}
                className="w-full resize-y rounded-md border px-3 py-2 text-[13px] placeholder:text-text-muted focus:outline-none focus:ring-1 bg-background border-border text-foreground"
                style={{
                  fontFamily: "var(--font-dm-sans), sans-serif",
                }}
              />
              <div className="mt-3 flex justify-end">
                <button
                  onClick={handleSaveAgentConfig}
                  disabled={saving || (promptDraft === null && rulesDraft === null)}
                  className="rounded-md px-4 py-1.5 text-[13px] font-medium text-white transition-colors disabled:opacity-40 bg-beacon-accent"
                >
                  {saving ? t("projects.detail.saving") : t("projects.detail.saveSettings")}
                </button>
              </div>
            </div>

            {/* Card 3: Test Run */}
            <div
              className="rounded-lg border p-5 bg-surface"
              style={{ borderColor: "var(--border)" }}
            >
              <SectionLabel className="mb-1">{t("projects.detail.testRun")}</SectionLabel>
              <p className="mb-4 text-[12px] text-text-muted">
                {t("projects.detail.testRunDesc")}
              </p>
              <div className="flex flex-wrap items-end gap-3">
                <div className="min-w-[200px] flex-1">
                  <label className="mb-1.5 block text-[13px] text-secondary-foreground">
                    {t("projects.detail.source")}
                  </label>
                  <select
                    value={testSourceId}
                    onChange={(e) => {
                      setTestSourceId(e.target.value);
                      setTestVersion("");
                    }}
                    className="w-full rounded-md border px-3 py-1.5 text-[13px] focus:outline-none focus:ring-1 bg-background border-border"
                    style={{
                      fontFamily: "var(--font-dm-sans), sans-serif",
                      color: testSourceId ? "var(--foreground)" : "var(--text-muted)",
                    }}
                  >
                    <option value="">{t("projects.detail.selectSource")}</option>
                    {sourcesData?.data?.map((s) => (
                      <option key={s.id} value={s.id}>
                        {s.provider}: {s.repository}
                      </option>
                    ))}
                  </select>
                </div>
                <div className="min-w-[200px] flex-1">
                  <label className="mb-1.5 block text-[13px] text-secondary-foreground">
                    {t("projects.detail.version")}
                  </label>
                  <select
                    value={testVersion}
                    onChange={(e) => setTestVersion(e.target.value)}
                    disabled={!testSourceId}
                    className="w-full rounded-md border px-3 py-1.5 text-[13px] focus:outline-none focus:ring-1 disabled:opacity-50 bg-background border-border"
                    style={{
                      fontFamily: "'JetBrains Mono', monospace",
                      color: testVersion ? "var(--foreground)" : "var(--text-muted)",
                    }}
                  >
                    <option value="">
                      {testSourceId ? t("projects.detail.selectVersion") : t("projects.detail.chooseSourceFirst")}
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
                  className="inline-flex items-center gap-1.5 rounded-md px-4 py-1.5 text-[13px] font-medium text-white transition-colors disabled:opacity-40 bg-beacon-accent"
                >
                  <Play className="h-3.5 w-3.5" />
                  {testRunning ? t("projects.detail.running") : t("projects.detail.runTest")}
                </button>
              </div>
            </div>

            {/* Card 4: Run History */}
            <div>
              <SectionLabel className="mb-3">{t("projects.detail.runHistory")}</SectionLabel>
              {runsData?.data && runsData.data.length > 0 ? (
                <div className="overflow-hidden rounded-md border" style={{ borderColor: "var(--border)" }}>
                  <table className="w-full text-[13px]" style={{ fontFamily: "var(--font-dm-sans), sans-serif" }}>
                    <thead>
                      <tr style={{ backgroundColor: "var(--background)" }}>
                        <th className="px-4 py-2 text-left text-[11px] font-medium uppercase tracking-[0.08em] text-text-muted">{t("projects.detail.trigger")}</th>
                        <th className="px-4 py-2 text-left text-[11px] font-medium uppercase tracking-[0.08em] text-text-muted">{t("projects.detail.status")}</th>
                        <th className="px-4 py-2 text-left text-[11px] font-medium uppercase tracking-[0.08em] text-text-muted">{t("projects.detail.started")}</th>
                        <th className="px-4 py-2 text-left text-[11px] font-medium uppercase tracking-[0.08em] text-text-muted">{t("projects.detail.duration")}</th>
                        <th className="px-4 py-2 text-left text-[11px] font-medium uppercase tracking-[0.08em] text-text-muted">{t("projects.detail.semanticRelease")}</th>
                      </tr>
                    </thead>
                    <tbody>
                      {runsData.data.map((run) => (
                        <tr key={run.id} className="border-t" style={{ borderColor: "var(--border)" }}>
                          <td className="px-4 py-3 text-secondary-foreground">
                            {run.trigger}
                          </td>
                          <td className="px-4 py-3">
                            <div className="flex items-center gap-2">
                              <StatusDot status={run.status} />
                              <span className="text-text-secondary">{run.status}</span>
                            </div>
                          </td>
                          <td className="px-4 py-3 text-text-muted">
                            {run.started_at
                              ? new Date(run.started_at).toLocaleString()
                              : t("projects.detail.pending")}
                          </td>
                          <td className="px-4 py-3 text-text-muted">
                            {formatDuration(run.started_at, run.completed_at)}
                          </td>
                          <td className="px-4 py-3">
                            {run.semantic_release_id ? (
                              <Link
                                href={`/projects/${id}/semantic-releases/${run.semantic_release_id}`}
                                className="text-[13px] font-medium underline text-beacon-accent"
                              >
                                {t("projects.detail.view")}
                              </Link>
                            ) : (
                              <span className="text-text-muted">--</span>
                            )}
                          </td>
                        </tr>
                      ))}
                    </tbody>
                  </table>
                </div>
              ) : (
                <div
                  className="flex h-32 items-center justify-center rounded-md border text-[13px] border-border text-text-muted"
                >
                  {t("projects.detail.noRuns")}
                </div>
              )}
            </div>
          </div>
        )}
      </div>

      {/* Source create dialog */}
      <Dialog open={sourceCreateOpen} onOpenChange={setSourceCreateOpen}>
        <DialogContent className="sm:max-w-lg">
          <DialogHeader><DialogTitle>{t("projects.detail.dialogAddSource")}</DialogTitle></DialogHeader>
          <SourceForm
            title={t("projects.detail.dialogAddSource")}
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
          <DialogHeader><DialogTitle>{t("projects.detail.dialogEditSource")}</DialogTitle></DialogHeader>
          {editingSource && (
            <SourceForm
              key={editingSource.id}
              title={t("projects.detail.dialogEditSource")}
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
        title={t("projects.detail.dialogDeleteSource")}
        description={t("projects.detail.dialogDeleteSourceDesc")}
        onConfirm={async () => { if (deletingSourceId) { await sourcesApi.delete(deletingSourceId); mutateSources(); } }}
      />

      {/* Context source create dialog */}
      <Dialog open={ctxCreateOpen} onOpenChange={setCtxCreateOpen}>
        <DialogContent className="sm:max-w-lg">
          <DialogHeader><DialogTitle>{t("projects.detail.dialogAddContextSource")}</DialogTitle></DialogHeader>
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
        title={t("projects.detail.dialogDeleteContextSource")}
        description={t("projects.detail.dialogDeleteContextSourceDesc")}
        onConfirm={async () => { if (deletingCtxId) { await ctxApi.delete(deletingCtxId); mutateCtx(); } }}
      />

      {/* Project delete dialog */}
      <ConfirmDialog
        open={deletingProject}
        onOpenChange={setDeletingProject}
        title={t("projects.detail.dialogDeleteProject")}
        description={t("projects.detail.dialogDeleteProjectDesc")}
        onConfirm={async () => { await projectsApi.delete(id); router.push("/projects"); }}
      />
    </div>
  );
}
