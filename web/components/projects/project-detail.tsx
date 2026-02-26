"use client";

import useSWR from "swr";
import Link from "next/link";
import { useState } from "react";
import { useRouter } from "next/navigation";
import {
  projects as projectsApi,
  sources as sourcesApi,
  contextSources as ctxApi,
  semanticReleases as srApi,
  agent as agentApi,
} from "@/lib/api/client";
import type { AgentRules } from "@/lib/api/types";
import { ProviderBadge } from "@/components/ui/provider-badge";
import { StatusDot } from "@/components/ui/status-dot";
import { VersionChip } from "@/components/ui/version-chip";
import { SectionLabel } from "@/components/ui/section-label";
import { Pencil, Trash2, Play, Plus } from "lucide-react";

/* ---------- Tabs ---------- */

const tabs = [
  { key: "sources", label: "Sources" },
  { key: "context", label: "Context Sources" },
  { key: "semantic", label: "Semantic Releases" },
  { key: "agent", label: "Agent" },
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

function formatInterval(seconds: number): string {
  if (seconds < 60) return `${seconds}s`;
  if (seconds < 3600) return `${Math.round(seconds / 60)}m`;
  return `${(seconds / 3600).toFixed(1)}h`;
}

/* ---------- Urgency chip ---------- */

const URGENCY_COLORS: Record<string, { bg: string; text: string }> = {
  critical: { bg: "#dc2626", text: "#ffffff" },
  high: { bg: "#f97316", text: "#ffffff" },
};

function UrgencyChip({ urgency }: { urgency: string }) {
  const u = urgency.toLowerCase();
  const style = URGENCY_COLORS[u];
  if (!style) return null;
  return (
    <span
      className="inline-flex items-center rounded-full px-2 py-0.5 text-[11px] font-medium leading-none"
      style={{ backgroundColor: style.bg, color: style.text }}
    >
      {urgency}
    </span>
  );
}

/* ---------- Main component ---------- */

export function ProjectDetail({ id }: { id: string }) {
  const router = useRouter();
  const [activeTab, setActiveTab] = useState<TabKey>("sources");
  const [triggering, setTriggering] = useState(false);

  /* Agent config local state */
  const [promptDraft, setPromptDraft] = useState<string | null>(null);
  const [rulesDraft, setRulesDraft] = useState<AgentRules | null>(null);
  const [saving, setSaving] = useState(false);

  /* Data fetching */
  const { data, isLoading, mutate: mutateProject } = useSWR(`project-${id}`, () => projectsApi.get(id));
  const { data: sourcesData, mutate: mutateSources } = useSWR(
    activeTab === "sources" ? `project-${id}-sources` : null,
    () => sourcesApi.listByProject(id),
  );
  const { data: ctxData, mutate: mutateCtx } = useSWR(
    activeTab === "context" ? `project-${id}-ctx` : null,
    () => ctxApi.list(id),
  );
  const { data: srData } = useSWR(
    activeTab === "semantic" ? `project-${id}-sr` : null,
    () => srApi.list(id),
  );
  const { data: runsData, mutate: mutateRuns } = useSWR(
    activeTab === "agent" ? `project-${id}-runs` : null,
    () => agentApi.listRuns(id),
  );

  /* Handlers */
  const handleDelete = async () => {
    if (!confirm("Delete this project? This will cascade to sources and subscriptions.")) return;
    await projectsApi.delete(id);
    router.push("/projects");
  };

  const handleTriggerRun = async () => {
    setTriggering(true);
    try {
      await agentApi.triggerRun(id);
      mutateRuns();
    } finally {
      setTriggering(false);
    }
  };

  const handleDeleteSource = async (sourceId: string) => {
    if (!confirm("Delete this source?")) return;
    await sourcesApi.delete(sourceId);
    mutateSources();
  };

  const handleDeleteCtx = async (ctxId: string) => {
    if (!confirm("Delete this context source?")) return;
    await ctxApi.delete(ctxId);
    mutateCtx();
  };

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
        <div className="flex items-start justify-between">
          {/* Left: project info */}
          <div className="min-w-0 flex-1">
            <h1
              className="text-[28px] font-bold leading-tight"
              style={{ fontFamily: "var(--font-fraunces), serif" }}
            >
              {project.name}
            </h1>
            {project.description && (
              <p
                className="mt-1 text-[14px]"
                style={{ fontFamily: "var(--font-dm-sans), sans-serif", color: "#6b7280" }}
              >
                {project.description}
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

          {/* Right: actions */}
          <div className="flex shrink-0 items-center gap-2 ml-4">
            <Link href={`/projects/${id}/edit`}>
              <button
                className="inline-flex items-center gap-1.5 rounded-md border px-3 py-1.5 text-[13px] font-medium transition-colors hover:bg-[#f3f3f1]"
                style={{
                  fontFamily: "var(--font-dm-sans), sans-serif",
                  borderColor: "#e8e8e5",
                  color: "#374151",
                }}
              >
                <Pencil className="h-3.5 w-3.5" />
                Edit
              </button>
            </Link>
            <button
              onClick={handleDelete}
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
            <button
              onClick={handleTriggerRun}
              disabled={triggering}
              className="inline-flex items-center gap-1.5 rounded-md px-3 py-1.5 text-[13px] font-medium text-white transition-colors disabled:opacity-60"
              style={{
                fontFamily: "var(--font-dm-sans), sans-serif",
                backgroundColor: "#e8601a",
              }}
            >
              <Play className="h-3.5 w-3.5" />
              {triggering ? "Running..." : "Run Agent"}
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
              <Link href={`/projects/${id}/sources/new`}>
                <button
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
              </Link>
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
                          <button
                            onClick={() => handleToggleSource(source)}
                            className="relative inline-flex h-5 w-9 shrink-0 cursor-pointer items-center rounded-full transition-colors duration-200"
                            style={{ backgroundColor: source.enabled ? "#16a34a" : "#d1d5db" }}
                            title={source.enabled ? "Disable polling" : "Enable polling"}
                          >
                            <span
                              className="inline-block h-3.5 w-3.5 rounded-full bg-white shadow transition-transform duration-200"
                              style={{ transform: source.enabled ? "translateX(18px)" : "translateX(3px)" }}
                            />
                          </button>
                        </td>
                        <td className="px-4 py-3" style={{ color: "#9ca3af" }}>
                          {source.last_polled_at
                            ? new Date(source.last_polled_at).toLocaleString()
                            : "Never"}
                        </td>
                        <td className="px-4 py-3 text-right">
                          <button
                            onClick={() => handleDeleteSource(source.id)}
                            className="rounded p-1 text-[#9ca3af] transition-colors hover:bg-red-50 hover:text-red-600"
                            title="Delete source"
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
              <Link href={`/projects/${id}/context-sources/new`}>
                <button
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
              </Link>
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
                            onClick={() => handleDeleteCtx(ctx.id)}
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

        {/* --- Semantic Releases tab --- */}
        {activeTab === "semantic" && (
          <div>
            <SectionLabel className="mb-4">Semantic Releases</SectionLabel>

            {srData?.data && srData.data.length > 0 ? (
              <div className="space-y-3">
                {srData.data.map((sr) => (
                  <Link
                    key={sr.id}
                    href={`/projects/${id}/semantic-releases/${sr.id}`}
                    className="block rounded-md border p-4 transition-colors hover:bg-[#fafaf9]"
                    style={{ borderColor: "#e8e8e5" }}
                  >
                    <div className="flex items-center gap-2.5">
                      <VersionChip version={sr.version} />
                      <StatusDot status={sr.status} />
                      <span className="text-[13px]" style={{ color: "#6b7280" }}>
                        {sr.status}
                      </span>
                      {sr.report?.urgency && <UrgencyChip urgency={sr.report.urgency} />}
                    </div>
                    {sr.report?.summary && (
                      <p
                        className="mt-2 text-[13px] italic leading-relaxed"
                        style={{ color: "#6b7280" }}
                      >
                        {truncate(sr.report.summary, 200)}
                      </p>
                    )}
                  </Link>
                ))}
              </div>
            ) : (
              <div
                className="flex h-32 items-center justify-center rounded-md border text-[13px]"
                style={{ borderColor: "#e8e8e5", color: "#9ca3af" }}
              >
                No semantic releases yet
              </div>
            )}
          </div>
        )}

        {/* --- Agent tab --- */}
        {activeTab === "agent" && (
          <div className="space-y-8">
            {/* Prompt section */}
            <div>
              <SectionLabel className="mb-3">Agent Prompt</SectionLabel>
              <textarea
                value={currentPrompt}
                onChange={(e) => setPromptDraft(e.target.value)}
                rows={5}
                placeholder="Custom instructions for the agent when analyzing releases..."
                className="w-full resize-y rounded-md border px-3 py-2 text-[13px] placeholder:text-[#9ca3af] focus:outline-none focus:ring-1"
                style={{
                  fontFamily: "var(--font-dm-sans), sans-serif",
                  backgroundColor: "#fafaf9",
                  borderColor: "#e8e8e5",
                  color: "#111113",
                }}
              />
              <div className="mt-2 flex justify-end">
                <button
                  onClick={handleSaveAgentConfig}
                  disabled={saving || (promptDraft === null && rulesDraft === null)}
                  className="rounded-md px-3 py-1.5 text-[13px] font-medium text-white transition-colors disabled:opacity-40"
                  style={{ backgroundColor: "#e8601a" }}
                >
                  {saving ? "Saving..." : "Save"}
                </button>
              </div>
            </div>

            {/* Rules section */}
            <div>
              <SectionLabel className="mb-3">Trigger Rules</SectionLabel>
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
                  On Major Release
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
                  On Minor Release
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
                  On Security Patch
                </label>
                <div className="mt-4">
                  <label className="mb-1.5 block text-[13px]" style={{ color: "#374151" }}>
                    Version Pattern
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
                </div>
              </div>
            </div>

            {/* Run History section */}
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
    </div>
  );
}
