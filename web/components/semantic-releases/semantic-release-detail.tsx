"use client";

import useSWR from "swr";
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
import { getPathSegment } from "@/lib/path";
import { useTranslation } from "@/lib/i18n/context";
import { ArrowLeft, ExternalLink, BookOpen } from "lucide-react";
import { SemanticReleaseReport } from "./semantic-release-report";

export function SemanticReleaseDetail() {
  const { t } = useTranslation();
  // Read IDs from URL path — useParams() returns stale "0" in static export
  const projectId = getPathSegment(1); // /projects/{id}/semantic-releases/{srId}
  const srId = getPathSegment(3);
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
          className="text-[13px] text-text-muted"
          style={{ fontFamily: "var(--font-dm-sans)" }}
        >
          {t("sr.detail.loading")}
        </div>
      </div>
    );
  }

  const sr = data?.data;
  if (!sr) {
    return (
      <div className="flex justify-center py-20">
        <div
          className="text-[13px] text-text-muted"
          style={{ fontFamily: "var(--font-dm-sans)" }}
        >
          {t("sr.detail.notFound")}
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
    if (!confirm(t("sr.detail.deleteConfirm"))) return;
    try {
      await srApi.delete(srId);
      router.push("/releases");
    } catch {
      alert(t("sr.detail.deleteFailed"));
    }
  };

  const report = sr.report;

  return (
    <div className="fade-in mx-auto max-w-[760px]">
      {/* 1. Back link */}
      <button
        onClick={() => window.history.length > 1 ? router.back() : router.push("/releases")}
        className="mb-6 inline-flex items-center gap-1.5 transition-colors hover:opacity-70 cursor-pointer"
        style={{
          fontFamily: "var(--font-dm-sans)",
          fontSize: "13px",
          color: "var(--text-secondary)",
        }}
      >
        <ArrowLeft size={14} />
        {t("sr.detail.back")}
      </button>

      {/* 2. Project byline */}
      {project?.name && (
        <p
          className="mb-1 text-[13px] italic text-text-muted"
          style={{ fontFamily: "var(--font-raleway)" }}
        >
          {project.name}
        </p>
      )}

      {/* 3. Version heading */}
      <h1
        className="text-[42px] font-bold tracking-tight text-foreground leading-[1.1]"
        style={{ fontFamily: "var(--font-raleway)" }}
      >
        {sr.version}
      </h1>

      {/* 4. Subject line */}
      {report?.subject && (
        <p
          className="mt-2 text-[20px] leading-[1.4] text-secondary-foreground"
          style={{ fontFamily: "var(--font-dm-sans)" }}
        >
          {report.subject}
        </p>
      )}

      {/* 5. Meta line */}
      <div
        className="mt-3 flex items-center gap-2 text-[13px] text-text-secondary"
        style={{ fontFamily: "var(--font-dm-sans)" }}
      >
        <StatusDot status={sr.status} />
        <span className="flex-1">
          {sr.status}
          {sr.completed_at && ` \u00b7 ${t("sr.detail.generated")} ${timeAgo(sr.completed_at)}`}
        </span>
        <button
          onClick={handleDelete}
          className="rounded-md px-2.5 py-1 text-[12px] font-medium text-[#991b1b] transition-colors hover:bg-[#fef2f2]"
          style={{ fontFamily: "var(--font-dm-sans)" }}
        >
          {t("sr.detail.delete")}
        </button>
      </div>

      {/* 6. Divider */}
      <hr
        className="my-8 border-0"
        style={{ borderTop: "1px solid var(--border)" }}
      />

      {/* Report content */}
      <SemanticReleaseReport report={report} error={sr.error} />

      {/* Source Releases */}
      {releasesList.length > 0 && (
        <section className="mt-10">
          <SectionLabel className="mb-4">{t("sr.detail.sourceReleases")}</SectionLabel>
          <div
            className="overflow-hidden rounded-md"
            style={{ border: "1px solid var(--border)" }}
          >
            <table className="w-full text-left">
              <thead>
                <tr
                  style={{
                    backgroundColor: "var(--background)",
                    borderBottom: "1px solid var(--border)",
                  }}
                >
                  <th
                    className="px-4 py-2.5 text-[11px] font-medium uppercase tracking-[0.08em] text-text-muted"
                    style={{ fontFamily: "var(--font-dm-sans)" }}
                  >
                    {t("sr.detail.provider")}
                  </th>
                  <th
                    className="px-4 py-2.5 text-[11px] font-medium uppercase tracking-[0.08em] text-text-muted"
                    style={{ fontFamily: "var(--font-dm-sans)" }}
                  >
                    {t("sr.detail.repository")}
                  </th>
                  <th
                    className="px-4 py-2.5 text-[11px] font-medium uppercase tracking-[0.08em] text-text-muted"
                    style={{ fontFamily: "var(--font-dm-sans)" }}
                  >
                    {t("sr.detail.version")}
                  </th>
                  <th
                    className="px-4 py-2.5 text-[11px] font-medium uppercase tracking-[0.08em] text-text-muted"
                    style={{ fontFamily: "var(--font-dm-sans)" }}
                  >
                    {t("sr.detail.date")}
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
                        : source?.provider === "ecr-public"
                          ? `https://gallery.ecr.aws/${source.repository}`
                          : null;

                  return (
                    <tr
                      key={rel.id}
                      className="border-b border-border last:border-b-0"
                    >
                      <td className="px-4 py-3">
                        {source ? (
                          <ProviderBadge provider={source.provider} />
                        ) : (
                          <span className="text-[12px] text-text-muted">
                            {"\u2014"}
                          </span>
                        )}
                      </td>
                      <td className="px-4 py-3">
                        <span
                          className="text-[13px] text-secondary-foreground"
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
                              className="text-text-muted"
                            />
                          </a>
                        ) : (
                          <VersionChip version={rel.version} />
                        )}
                      </td>
                      <td className="px-4 py-3">
                        <span
                          className="text-[13px] text-text-secondary"
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

      {/* Context Sources */}
      {contextSourcesList.length > 0 && (
        <section className="mt-10">
          <SectionLabel className="mb-3">{t("sr.detail.contextSources")}</SectionLabel>
          <div className="space-y-2">
            {contextSourcesList.map((cs) => {
              const url = cs.config?.url as string | undefined;
              return (
                <div
                  key={cs.id}
                  className="flex items-center gap-3 rounded-md px-4 py-3"
                  style={{
                    border: "1px solid var(--border)",
                    backgroundColor: "var(--surface)",
                  }}
                >
                  <BookOpen
                    size={16}
                    className="shrink-0 text-text-muted"
                  />
                  <div className="flex-1 min-w-0">
                    <span
                      className="text-[14px] font-medium text-foreground"
                      style={{ fontFamily: "var(--font-dm-sans)" }}
                    >
                      {cs.name}
                    </span>
                    <span
                      className="ml-2 rounded px-1.5 py-0.5 text-[11px] font-medium uppercase text-text-secondary"
                      style={{ backgroundColor: "var(--mono-bg)" }}
                    >
                      {cs.type}
                    </span>
                  </div>
                  {url && (
                    <a
                      href={url}
                      target="_blank"
                      rel="noopener noreferrer"
                      className="shrink-0 text-text-muted hover:text-secondary-foreground transition-colors"
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
    </div>
  );
}
