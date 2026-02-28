"use client";

import useSWR from "swr";
import Link from "next/link";
import {
  semanticReleases as srApi,
  projects as projectsApi,
  sources as sourcesApi,
  contextSources,
} from "@/lib/api/client";
import { useRouter } from "next/navigation";
import { StatusDot } from "@/components/ui/status-dot";
import { VersionChip } from "@/components/ui/version-chip";
import { SectionLabel } from "@/components/ui/section-label";
import { ProviderBadge } from "@/components/ui/provider-badge";
import { timeAgo } from "@/lib/format";
import {
  ArrowLeft,
  Check,
  ExternalLink,
  Copy,
  ShieldAlert,
  BookOpen,
  Download,
} from "lucide-react";

function getRiskColors(riskLevel?: string) {
  switch (riskLevel?.toUpperCase()) {
    case "CRITICAL":
      return { border: "#dc2626", bg: "#fff1f2", text: "#991b1b" };
    case "HIGH":
      return { border: "#d97706", bg: "#fff8f0", text: "#92400e" };
    case "MEDIUM":
      return { border: "#ca8a04", bg: "#fefce8", text: "#854d0e" };
    case "LOW":
    default:
      return { border: "#16a34a", bg: "#f0fdf4", text: "#166534" };
  }
}

function safeHostname(url: string): string {
  try {
    return new URL(url).hostname.replace("www.", "");
  } catch {
    return url;
  }
}

function getDownloadLabel(url: string): { label: string; isDirect: boolean } {
  const lower = url.toLowerCase();
  const filename = lower.split("/").pop() ?? "";

  // Detect platform from common patterns in the URL or filename
  const platforms: [RegExp, string][] = [
    [/linux.*amd64|amd64.*linux/, "Linux x64"],
    [/linux.*arm64|arm64.*linux|linux.*aarch64/, "Linux ARM64"],
    [/linux.*386|linux.*i386/, "Linux x86"],
    [/darwin.*arm64|arm64.*darwin|macos.*arm64|osx.*arm64/, "macOS ARM64"],
    [/darwin.*amd64|amd64.*darwin|macos.*amd64|osx.*amd64/, "macOS x64"],
    [/darwin|macos|osx/, "macOS"],
    [/windows.*amd64|amd64.*windows|win64/, "Windows x64"],
    [/windows.*386|win32/, "Windows x86"],
    [/windows/, "Windows"],
  ];

  // Check if this is a direct binary (archive or executable)
  const isArchive = /\.(tar\.gz|tar\.xz|zip|deb|rpm|dmg|msi|exe|pkg|appimage)(\?|$)/i.test(url);

  if (isArchive) {
    for (const [pattern, label] of platforms) {
      if (pattern.test(filename) || pattern.test(url)) {
        return { label, isDirect: true };
      }
    }
    // Direct download but platform unknown — use filename
    const cleanName = (url.split("/").pop() ?? "").split("?")[0];
    return { label: cleanName || safeHostname(url), isDirect: true };
  }

  // Not a direct binary — generic link
  return { label: safeHostname(url), isDirect: false };
}

export function SemanticReleaseDetail({
  projectId,
  srId,
}: {
  projectId: string;
  srId: string;
}) {
  const router = useRouter();
  const { data, isLoading } = useSWR(`sr-${srId}`, () => srApi.get(srId));
  const { data: projectData } = useSWR(`project-${projectId}`, () =>
    projectsApi.get(projectId),
  );
  const { data: srSourcesData } = useSWR(`sr-sources-${srId}`, () =>
    srApi.getSources(srId),
  );
  const { data: sourcesData } = useSWR(`project-sources-${projectId}`, () =>
    sourcesApi.listByProject(projectId),
  );
  const { data: contextSourcesData } = useSWR(
    `project-context-sources-${projectId}`,
    () => contextSources.list(projectId),
  );

  if (isLoading) {
    return (
      <div className="flex justify-center py-20">
        <div
          className="text-[13px] text-[#9ca3af]"
          style={{ fontFamily: "var(--font-dm-sans)" }}
        >
          Loading...
        </div>
      </div>
    );
  }

  const sr = data?.data;
  if (!sr) {
    return (
      <div className="flex justify-center py-20">
        <div
          className="text-[13px] text-[#9ca3af]"
          style={{ fontFamily: "var(--font-dm-sans)" }}
        >
          Semantic release not found
        </div>
      </div>
    );
  }

  const project = projectData?.data;
  const releasesList = srSourcesData?.data ?? [];
  const sourcesList = sourcesData?.data ?? [];
  const sourcesById = Object.fromEntries(sourcesList.map((s) => [s.id, s]));
  const contextSourcesList = contextSourcesData?.data ?? [];

  const handleDelete = async () => {
    if (!confirm("Delete this semantic release?")) return;
    try {
      await srApi.delete(srId);
      router.push("/semantic-releases");
    } catch {
      alert("Failed to delete semantic release.");
    }
  };

  const report = sr.report;
  const riskLevel = report?.risk_level?.toUpperCase();
  const hasRiskOrUrgency = riskLevel || report?.urgency;
  const riskColors = getRiskColors(riskLevel);

  const statusChecks = report?.status_checks ?? [];
  const downloadLinks = report?.download_links ?? [];
  const downloadCommands = report?.download_commands ?? [];
  const hasAvailabilitySection =
    statusChecks.length > 0 ||
    downloadLinks.length > 0 ||
    downloadCommands.length > 0;

  return (
    <div className="fade-in mx-auto max-w-[760px]">
      {/* 1. Back link */}
      <Link
        href="/semantic-releases"
        className="mb-6 inline-flex items-center gap-1.5 transition-colors hover:opacity-70"
        style={{
          fontFamily: "var(--font-dm-sans)",
          fontSize: "13px",
          color: "#6b7280",
        }}
      >
        <ArrowLeft size={14} />
        Back to Semantic Releases
      </Link>

      {/* 2. Project byline */}
      {project?.name && (
        <p
          className="mb-1 text-[13px] italic text-[#9ca3af]"
          style={{ fontFamily: "var(--font-fraunces)" }}
        >
          {project.name}
        </p>
      )}

      {/* 3. Version heading */}
      <h1
        className="text-[42px] font-bold tracking-tight text-[#111113] leading-[1.1]"
        style={{ fontFamily: "var(--font-fraunces)" }}
      >
        {sr.version}
      </h1>

      {/* 4. Subject line */}
      {report?.subject && (
        <p
          className="mt-2 text-[20px] leading-[1.4] text-[#374151]"
          style={{ fontFamily: "var(--font-dm-sans)" }}
        >
          {report.subject}
        </p>
      )}

      {/* 5. Meta line */}
      <div
        className="mt-3 flex items-center gap-2 text-[13px] text-[#6b7280]"
        style={{ fontFamily: "var(--font-dm-sans)" }}
      >
        <StatusDot status={sr.status} />
        <span className="flex-1">
          {sr.status}
          {sr.completed_at && ` \u00b7 generated ${timeAgo(sr.completed_at)}`}
        </span>
        <button
          onClick={handleDelete}
          className="rounded-md px-2.5 py-1 text-[12px] font-medium text-[#991b1b] transition-colors hover:bg-[#fef2f2]"
          style={{ fontFamily: "var(--font-dm-sans)" }}
        >
          Delete
        </button>
      </div>

      {/* 6. Divider */}
      <hr
        className="my-8 border-0"
        style={{ borderTop: "1px solid #e8e8e5" }}
      />

      {/* 7. Error state */}
      {sr.error && (
        <div
          className="mb-8 rounded-md px-4 py-3 text-[14px] text-[#991b1b]"
          style={{
            border: "1px solid #fca5a5",
            backgroundColor: "#fef2f2",
            fontFamily: "var(--font-dm-sans)",
          }}
        >
          {sr.error}
        </div>
      )}

      {report && (
        <div className="space-y-10">
          {/* 8. Risk & Urgency Banner */}
          {hasRiskOrUrgency && (
            <div
              className="rounded-md px-4 py-4"
              style={{
                backgroundColor: riskColors.bg,
                borderLeft: `3px solid ${riskColors.border}`,
              }}
            >
              <div className="flex items-start gap-3">
                <ShieldAlert
                  className="h-5 w-5 mt-0.5 shrink-0"
                  style={{ color: riskColors.border }}
                />
                <div className="flex-1">
                  <div className="flex items-center gap-2 mb-1">
                    {riskLevel && (
                      <span
                        className="rounded px-1.5 py-0.5 text-[11px] font-bold uppercase"
                        style={{
                          backgroundColor: riskColors.border,
                          color: "#ffffff",
                        }}
                      >
                        {riskLevel} RISK
                      </span>
                    )}
                    {report.urgency && (
                      <span
                        className="text-[13px] font-medium"
                        style={{ color: riskColors.text }}
                      >
                        {report.urgency} Urgency
                      </span>
                    )}
                  </div>
                  {report.risk_reason && (
                    <p
                      className="text-[14px] leading-[1.6]"
                      style={{
                        color: riskColors.text,
                        fontFamily: "var(--font-dm-sans)",
                      }}
                    >
                      {report.risk_reason}
                    </p>
                  )}
                </div>
              </div>
            </div>
          )}

          {/* 9. Status Checks & Downloads */}
          {hasAvailabilitySection && (
            <section>
              <SectionLabel className="mb-3">
                Availability & Downloads
              </SectionLabel>

              {/* Status checks as green pills */}
              {statusChecks.length > 0 && (
                <div className="flex flex-wrap gap-2 mb-4">
                  {statusChecks.map((check) => (
                    <span
                      key={check}
                      className="inline-flex items-center gap-1.5 rounded-full px-3 py-1 text-[13px] font-medium"
                      style={{
                        backgroundColor: "#f0fdf4",
                        color: "#166534",
                        border: "1px solid #bbf7d0",
                        fontFamily: "var(--font-dm-sans)",
                      }}
                    >
                      <Check size={14} />
                      {check}
                    </span>
                  ))}
                </div>
              )}

              {/* Download links — platform binaries + generic links */}
              {downloadLinks.length > 0 && (
                <div className="flex flex-wrap gap-2 mb-4">
                  {downloadLinks.map((link) => {
                    const { label, isDirect } = getDownloadLabel(link);
                    return isDirect ? (
                      <a
                        key={link}
                        href={link}
                        target="_blank"
                        rel="noopener noreferrer"
                        className="inline-flex items-center gap-1.5 rounded-md px-3 py-1.5 text-[13px] font-medium transition-colors hover:opacity-80"
                        style={{
                          backgroundColor: "#111113",
                          color: "#ffffff",
                          fontFamily: "var(--font-dm-sans)",
                        }}
                      >
                        <Download size={13} />
                        {label}
                      </a>
                    ) : (
                      <a
                        key={link}
                        href={link}
                        target="_blank"
                        rel="noopener noreferrer"
                        className="inline-flex items-center gap-1.5 rounded-full px-3 py-1 text-[13px] font-medium transition-colors hover:opacity-80"
                        style={{
                          backgroundColor: "#f3f3f1",
                          color: "#374151",
                          border: "1px solid #e8e8e5",
                          fontFamily: "var(--font-dm-sans)",
                        }}
                      >
                        <ExternalLink size={12} />
                        {label}
                      </a>
                    );
                  })}
                </div>
              )}

              {/* Download commands as copiable code */}
              {downloadCommands.length > 0 && (
                <div className="space-y-2">
                  {downloadCommands.map((cmd) => (
                    <div
                      key={cmd}
                      className="group flex items-center gap-2 rounded-md px-3 py-2"
                      style={{
                        backgroundColor: "#fafaf9",
                        border: "1px solid #e8e8e5",
                      }}
                    >
                      <code
                        className="flex-1 text-[13px] text-[#111113]"
                        style={{ fontFamily: "'JetBrains Mono', monospace" }}
                      >
                        {cmd}
                      </code>
                      <button
                        onClick={() => navigator.clipboard.writeText(cmd)}
                        className="shrink-0 rounded p-1 text-[#9ca3af] opacity-0 transition-opacity group-hover:opacity-100 hover:text-[#374151]"
                        title="Copy"
                      >
                        <Copy size={14} />
                      </button>
                    </div>
                  ))}
                </div>
              )}
            </section>
          )}

          {/* 10. Adoption */}
          {report.adoption && (
            <section>
              <SectionLabel className="mb-2">Adoption</SectionLabel>
              <div
                className="rounded-md p-4"
                style={{
                  border: "1px solid #e8e8e5",
                  backgroundColor: "#ffffff",
                }}
              >
                <p
                  className="text-[14px] leading-[1.6] text-[#111113]"
                  style={{ fontFamily: "var(--font-dm-sans)" }}
                >
                  {report.adoption}
                </p>
              </div>
            </section>
          )}

          {/* 11. Source Releases */}
          {releasesList.length > 0 && (
            <section>
              <SectionLabel className="mb-4">Source Releases</SectionLabel>
              <div
                className="overflow-hidden rounded-md"
                style={{ border: "1px solid #e8e8e5" }}
              >
                <table className="w-full text-left">
                  <thead>
                    <tr
                      style={{
                        backgroundColor: "#fafaf9",
                        borderBottom: "1px solid #e8e8e5",
                      }}
                    >
                      <th
                        className="px-4 py-2.5 text-[11px] font-medium uppercase tracking-[0.08em] text-[#9ca3af]"
                        style={{ fontFamily: "var(--font-dm-sans)" }}
                      >
                        Provider
                      </th>
                      <th
                        className="px-4 py-2.5 text-[11px] font-medium uppercase tracking-[0.08em] text-[#9ca3af]"
                        style={{ fontFamily: "var(--font-dm-sans)" }}
                      >
                        Repository
                      </th>
                      <th
                        className="px-4 py-2.5 text-[11px] font-medium uppercase tracking-[0.08em] text-[#9ca3af]"
                        style={{ fontFamily: "var(--font-dm-sans)" }}
                      >
                        Version
                      </th>
                      <th
                        className="px-4 py-2.5 text-[11px] font-medium uppercase tracking-[0.08em] text-[#9ca3af]"
                        style={{ fontFamily: "var(--font-dm-sans)" }}
                      >
                        Date
                      </th>
                    </tr>
                  </thead>
                  <tbody>
                    {releasesList.map((rel) => {
                      const source = sourcesById[rel.source_id];
                      const versionUrl =
                        source?.provider === "github"
                          ? `https://github.com/${source.repository}/releases/tag/${rel.version}`
                          : source?.provider === "dockerhub"
                            ? `https://hub.docker.com/r/${source.repository}/tags?name=${encodeURIComponent(rel.version)}`
                            : null;

                      return (
                        <tr
                          key={rel.id}
                          className="border-b border-[#e8e8e5] last:border-b-0"
                        >
                          <td className="px-4 py-3">
                            {source ? (
                              <ProviderBadge provider={source.provider} />
                            ) : (
                              <span className="text-[12px] text-[#9ca3af]">
                                {"\u2014"}
                              </span>
                            )}
                          </td>
                          <td className="px-4 py-3">
                            <span
                              className="text-[13px] text-[#374151]"
                              style={{
                                fontFamily: "'JetBrains Mono', monospace",
                              }}
                            >
                              {source?.repository ?? "\u2014"}
                            </span>
                          </td>
                          <td className="px-4 py-3">
                            {versionUrl ? (
                              <a
                                href={versionUrl}
                                target="_blank"
                                rel="noopener noreferrer"
                                className="inline-flex items-center gap-1 hover:opacity-70 transition-opacity"
                              >
                                <VersionChip version={rel.version} />
                                <ExternalLink
                                  size={11}
                                  className="text-[#9ca3af]"
                                />
                              </a>
                            ) : (
                              <VersionChip version={rel.version} />
                            )}
                          </td>
                          <td className="px-4 py-3">
                            <span
                              className="text-[13px] text-[#6b7280]"
                              style={{ fontFamily: "var(--font-dm-sans)" }}
                            >
                              {timeAgo(rel.released_at ?? rel.created_at)}
                            </span>
                          </td>
                        </tr>
                      );
                    })}
                  </tbody>
                </table>
              </div>
            </section>
          )}

          {/* 12. Context Sources */}
          {contextSourcesList.length > 0 && (
            <section>
              <SectionLabel className="mb-3">Context Sources</SectionLabel>
              <div className="space-y-2">
                {contextSourcesList.map((cs) => {
                  const url = cs.config?.url as string | undefined;
                  return (
                    <div
                      key={cs.id}
                      className="flex items-center gap-3 rounded-md px-4 py-3"
                      style={{
                        border: "1px solid #e8e8e5",
                        backgroundColor: "#ffffff",
                      }}
                    >
                      <BookOpen
                        size={16}
                        className="shrink-0 text-[#9ca3af]"
                      />
                      <div className="flex-1 min-w-0">
                        <span
                          className="text-[14px] font-medium text-[#111113]"
                          style={{ fontFamily: "var(--font-dm-sans)" }}
                        >
                          {cs.name}
                        </span>
                        <span
                          className="ml-2 rounded px-1.5 py-0.5 text-[11px] font-medium uppercase text-[#6b7280]"
                          style={{ backgroundColor: "#f3f3f1" }}
                        >
                          {cs.type}
                        </span>
                      </div>
                      {url && (
                        <a
                          href={url}
                          target="_blank"
                          rel="noopener noreferrer"
                          className="shrink-0 text-[#9ca3af] hover:text-[#374151] transition-colors"
                        >
                          <ExternalLink size={14} />
                        </a>
                      )}
                    </div>
                  );
                })}
              </div>
            </section>
          )}

          {/* 13. Changelog Summary */}
          {report.changelog_summary && (
            <section>
              <SectionLabel className="mb-3">Changelog Summary</SectionLabel>
              <p
                className="text-[16px] leading-[1.7] text-[#111113]"
                style={{ fontFamily: "var(--font-dm-sans)" }}
              >
                {report.changelog_summary}
              </p>
            </section>
          )}

          {/* 14. Recommendation pull-quote */}
          {report.recommendation && (
            <blockquote
              className="rounded-md px-5 py-4 text-[18px] italic leading-[1.6] text-[#16181c]"
              style={{
                fontFamily: "var(--font-fraunces)",
                borderLeft: "3px solid #e8601a",
                backgroundColor: "#fafaf9",
              }}
            >
              {report.recommendation}
            </blockquote>
          )}
        </div>
      )}
    </div>
  );
}
