"use client";

import { SectionLabel } from "@/components/ui/section-label";
import { useTranslation } from "@/lib/i18n/context";
import type { SemanticReport } from "@/lib/api/types";
import {
  Check,
  ExternalLink,
  Copy,
  ShieldAlert,
  Download,
} from "lucide-react";

export function getRiskColors(riskLevel?: string) {
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

  const isArchive = /\.(tar\.gz|tar\.xz|zip|deb|rpm|dmg|msi|exe|pkg|appimage)(\?|$)/i.test(url);

  if (isArchive) {
    for (const [pattern, label] of platforms) {
      if (pattern.test(filename) || pattern.test(url)) {
        return { label, isDirect: true };
      }
    }
    const cleanName = (url.split("/").pop() ?? "").split("?")[0];
    return { label: cleanName || safeHostname(url), isDirect: true };
  }

  return { label: safeHostname(url), isDirect: false };
}

export function SemanticReleaseReport({
  report,
  error,
  compact = false,
}: {
  report?: SemanticReport;
  error?: string;
  compact?: boolean;
}) {
  const { t } = useTranslation();

  const riskLevel = (report?.urgency ?? report?.risk_level)?.toUpperCase();
  const hasRiskOrUrgency = riskLevel || report?.urgency;
  const riskColors = getRiskColors(riskLevel);

  const statusChecks = report?.status_checks ?? [];
  const downloadLinks = report?.download_links ?? [];
  const downloadCommands = report?.download_commands ?? [];
  const hasAvailabilitySection =
    statusChecks.length > 0 ||
    downloadLinks.length > 0 ||
    downloadCommands.length > 0;

  if (compact) {
    return (
      <>
        {error && (
          <div
            className="mb-4 rounded-md px-4 py-3 text-[14px] text-[#991b1b]"
            style={{
              border: "1px solid #fca5a5",
              backgroundColor: "#fef2f2",
              fontFamily: "var(--font-dm-sans)",
            }}
          >
            {error}
          </div>
        )}
        {report && (
          <div
            className="overflow-hidden rounded-lg bg-surface"
            style={{ border: "1px solid var(--border)" }}
          >
            {(report.urgency_reason ?? report.risk_reason) && (
              <div className="px-5 py-4" style={{ borderBottom: "1px solid var(--border)" }}>
                <h2
                  className="mb-2"
                  style={{ fontFamily: "var(--font-raleway)", fontSize: "16px", fontWeight: 600, color: "var(--foreground)" }}
                >
                  {t("sr.detail.urgency")}
                </h2>
                <p
                  className="text-[13px] leading-[1.7] text-secondary-foreground"
                  style={{ fontFamily: "var(--font-dm-sans)" }}
                >
                  {report.urgency_reason ?? report.risk_reason}
                </p>
              </div>
            )}

            {hasAvailabilitySection && (
              <div className="px-5 py-4" style={{ borderBottom: "1px solid var(--border)" }}>
                <h2
                  className="mb-2"
                  style={{ fontFamily: "var(--font-raleway)", fontSize: "16px", fontWeight: 600, color: "var(--foreground)" }}
                >
                  {t("sr.detail.availabilityDownloads")}
                </h2>
                <div className="space-y-3">
                  {statusChecks.length > 0 && (
                    <p
                      className="text-[13px] text-secondary-foreground"
                      style={{ fontFamily: "var(--font-dm-sans)" }}
                    >
                      {statusChecks.join(" · ")}
                    </p>
                  )}
                  {downloadLinks.length > 0 && (
                    <div className="flex flex-wrap gap-x-4 gap-y-1">
                      {downloadLinks.map((link) => {
                        const { label } = getDownloadLabel(link);
                        return (
                          <a
                            key={link}
                            href={link}
                            target="_blank"
                            rel="noopener noreferrer"
                            className="text-[13px] hover:underline"
                            style={{ color: "var(--beacon-accent)", fontFamily: "var(--font-dm-sans)" }}
                          >
                            {label}
                          </a>
                        );
                      })}
                    </div>
                  )}
                  {downloadCommands.length > 0 && (
                    <div className="space-y-1.5">
                      {downloadCommands.map((cmd) => (
                        <div
                          key={cmd}
                          className="group flex items-center gap-2 rounded px-3 py-1.5"
                          style={{ backgroundColor: "var(--background)" }}
                        >
                          <code
                            className="flex-1 text-[12px] text-foreground"
                            style={{ fontFamily: "'JetBrains Mono', monospace" }}
                          >
                            {cmd}
                          </code>
                          <button
                            onClick={() => navigator.clipboard.writeText(cmd)}
                            className="shrink-0 rounded p-1 text-text-muted opacity-0 transition-opacity group-hover:opacity-100 hover:text-secondary-foreground"
                            title={t("sr.detail.copy")}
                          >
                            <Copy size={12} />
                          </button>
                        </div>
                      ))}
                    </div>
                  )}
                </div>
              </div>
            )}

            {report.adoption && (
              <div className="px-5 py-4" style={{ borderBottom: "1px solid var(--border)" }}>
                <h2
                  className="mb-2"
                  style={{ fontFamily: "var(--font-raleway)", fontSize: "16px", fontWeight: 600, color: "var(--foreground)" }}
                >
                  {t("sr.detail.adoption")}
                </h2>
                <p
                  className="text-[13px] leading-[1.7] text-secondary-foreground"
                  style={{ fontFamily: "var(--font-dm-sans)" }}
                >
                  {report.adoption}
                </p>
              </div>
            )}

            {report.changelog_summary && (
              <div className="px-5 py-4">
                <h2
                  className="mb-2"
                  style={{ fontFamily: "var(--font-raleway)", fontSize: "16px", fontWeight: 600, color: "var(--foreground)" }}
                >
                  {t("sr.detail.changelogSummary")}
                </h2>
                <p
                  className="text-[13px] leading-[1.7] text-secondary-foreground"
                  style={{ fontFamily: "var(--font-dm-sans)" }}
                >
                  {report.changelog_summary}
                </p>
              </div>
            )}
          </div>
        )}
      </>
    );
  }

  return (
    <>
      {error && (
        <div
          className={`${compact ? "mb-4" : "mb-8"} rounded-md px-4 py-3 text-[14px] text-[#991b1b]`}
          style={{
            border: "1px solid #fca5a5",
            backgroundColor: "#fef2f2",
            fontFamily: "var(--font-dm-sans)",
          }}
        >
          {error}
        </div>
      )}

      {report && (
        <div className={compact ? "space-y-4" : "space-y-10"}>
          {hasRiskOrUrgency && (
            <div
              className={`rounded-md px-4 ${compact ? "py-3" : "py-4"}`}
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
                        {riskLevel} {t("sr.detail.urgency")}
                      </span>
                    )}
                  </div>
                  {(report.urgency_reason ?? report.risk_reason) && (
                    <p
                      className="text-[14px] leading-[1.6]"
                      style={{
                        color: riskColors.text,
                        fontFamily: "var(--font-dm-sans)",
                      }}
                    >
                      {report.urgency_reason ?? report.risk_reason}
                    </p>
                  )}
                </div>
              </div>
            </div>
          )}

          {hasAvailabilitySection && (
            <section>
              {!compact && (
                <SectionLabel className="mb-3">
                  {t("sr.detail.availabilityDownloads")}
                </SectionLabel>
              )}

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
                          backgroundColor: "var(--mono-bg)",
                          color: "var(--secondary-foreground)",
                          border: "1px solid var(--border)",
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

              {downloadCommands.length > 0 && (
                <div className="space-y-2">
                  {downloadCommands.map((cmd) => (
                    <div
                      key={cmd}
                      className="group flex items-center gap-2 rounded-md px-3 py-2"
                      style={{
                        backgroundColor: "var(--background)",
                        border: "1px solid var(--border)",
                      }}
                    >
                      <code
                        className="flex-1 text-[13px] text-foreground"
                        style={{ fontFamily: "'JetBrains Mono', monospace" }}
                      >
                        {cmd}
                      </code>
                      <button
                        onClick={() => navigator.clipboard.writeText(cmd)}
                        className="shrink-0 rounded p-1 text-text-muted opacity-0 transition-opacity group-hover:opacity-100 hover:text-secondary-foreground"
                        title={t("sr.detail.copy")}
                      >
                        <Copy size={14} />
                      </button>
                    </div>
                  ))}
                </div>
              )}
            </section>
          )}

          {report.adoption && (
            <section>
              {!compact && (
                <SectionLabel className="mb-2">{t("sr.detail.adoption")}</SectionLabel>
              )}
              <div
                className="rounded-md p-4"
                style={{
                  border: "1px solid var(--border)",
                  backgroundColor: "var(--surface)",
                }}
              >
                <p
                  className="text-[14px] leading-[1.6] text-foreground"
                  style={{ fontFamily: "var(--font-dm-sans)" }}
                >
                  {report.adoption}
                </p>
              </div>
            </section>
          )}

          {report.changelog_summary && (
            <section>
              {!compact && (
                <SectionLabel className="mb-3">{t("sr.detail.changelogSummary")}</SectionLabel>
              )}
              <p
                className="text-[16px] leading-[1.7] text-foreground"
                style={{ fontFamily: "var(--font-dm-sans)" }}
              >
                {report.changelog_summary}
              </p>
            </section>
          )}
        </div>
      )}
    </>
  );
}

