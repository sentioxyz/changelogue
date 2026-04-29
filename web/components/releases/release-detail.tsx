"use client";

import { useMemo, useState } from "react";
import useSWR from "swr";
import Link from "next/link";
import { useRouter } from "next/navigation";
import { marked } from "marked";
import {
  releases as releasesApi,
  sources as sourcesApi,
  semanticReleases as srApi,
  projects as projectsApi,
} from "@/lib/api/client";
import { ProviderBadge } from "@/components/ui/provider-badge";
import { VersionChip } from "@/components/ui/version-chip";
import { Tabs, TabsList, TabsTrigger, TabsContent } from "@/components/ui/tabs";
import { SemanticReleaseReport } from "@/components/semantic-releases/semantic-release-report";
import { UrgencyPill } from "@/components/ui/urgency-pill";
import type { SemanticRelease, Source, Project } from "@/lib/api/types";
import { ArrowLeft, ExternalLink, ChevronDown, ChevronRight } from "lucide-react";
import { useTranslation } from "@/lib/i18n/context";

import { timeAgo } from "@/lib/format";
import { getPathSegment } from "@/lib/path";
import { getProviderUrl } from "@/lib/provider-urls";

function changelogToHtml(raw: string): string {
  return marked.parse(raw, { async: false }) as string;
}

function getProviderLabel(provider: string): string {
  switch (provider) {
    case "github":
      return "GitHub";
    case "dockerhub":
      return "Docker Hub";
    case "ecr-public":
      return "ECR Public";
    case "gitlab":
      return "GitLab";
    case "pypi":
      return "PyPI";
    case "npm":
      return "npm";
    default:
      return provider;
  }
}

export function ReleaseDetail() {
  const { t } = useTranslation();
  const router = useRouter();
  const id = getPathSegment(1);

  const { data: releaseData, isLoading } = useSWR(`release-${id}`, () =>
    releasesApi.get(id)
  );
  const release = releaseData?.data;

  const { data: sourceData } = useSWR(
    release ? `source-${release.source_id}` : null,
    () => (release ? sourcesApi.get(release.source_id) : null)
  );
  const source: Source | undefined = sourceData?.data;

  const { data: projectData } = useSWR(
    source ? `project-${source.project_id}` : null,
    () => (source ? projectsApi.get(source.project_id) : null)
  );
  const project: Project | undefined = projectData?.data;

  const { data: linkedSRData } = useSWR(
    release?.semantic_release_id ? `sr-${release.semantic_release_id}` : null,
    () => (release?.semantic_release_id ? srApi.get(release.semantic_release_id) : null)
  );
  const linkedSRs: SemanticRelease[] = linkedSRData?.data ? [linkedSRData.data] : [];

  const changelogHtml = useMemo(() => {
    const raw = release?.raw_data?.changelog;
    if (!raw) return null;
    return changelogToHtml(String(raw));
  }, [release?.raw_data?.changelog]);

  if (isLoading) {
    return (
      <div
        className="py-16 text-center"
        style={{
          fontFamily: "var(--font-dm-sans)",
          fontSize: "13px",
          color: "var(--text-secondary)",
        }}
      >
        {t("releases.loading")}
      </div>
    );
  }

  if (!release) {
    return (
      <div className="py-16 text-center">
        <p
          style={{
            fontFamily: "var(--font-raleway)",
            fontStyle: "italic",
            fontSize: "15px",
            color: "var(--text-muted)",
          }}
        >
          {t("releases.notFound")}
        </p>
      </div>
    );
  }

  return (
    <div className="space-y-8">
      {/* Back link */}
      <button
        onClick={() => window.history.length > 1 ? router.back() : router.push("/releases")}
        className="inline-flex items-center gap-1.5 transition-colors hover:opacity-70 cursor-pointer"
        style={{
          fontFamily: "var(--font-dm-sans)",
          fontSize: "13px",
          color: "var(--text-secondary)",
        }}
      >
        <ArrowLeft size={14} />
        {t("releases.back")}
      </button>

      {/* Header */}
      <div>
        <h1
          style={{
            fontFamily: "var(--font-raleway)",
            fontSize: "24px",
            fontWeight: 700,
            color: "var(--foreground)",
          }}
        >
          {t("releases.releaseVersion")} {release.version}
        </h1>
        <div className="mt-2 flex items-center gap-3">
          {source && <ProviderBadge provider={source.provider} />}
          {source && (
            <span
              style={{
                fontFamily: "'JetBrains Mono', monospace",
                fontSize: "12px",
                color: "var(--secondary-foreground)",
              }}
            >
              {source.repository}
            </span>
          )}
          <VersionChip version={release.version} />
          {source && (() => {
            const url = getProviderUrl(source.provider, source.repository, release.version);
            return url ? (
              <a
                href={url}
                target="_blank"
                rel="noopener noreferrer"
                className="inline-flex items-center gap-1 transition-colors hover:opacity-70"
                style={{
                  fontFamily: "var(--font-dm-sans)",
                  fontSize: "12px",
                  color: "var(--beacon-accent)",
                }}
              >
                {t("releases.viewOn")} {getProviderLabel(source.provider)}
                <ExternalLink size={12} />
              </a>
            ) : null;
          })()}
        </div>
        {project && (
          <p
            className="mt-2"
            style={{
              fontFamily: "var(--font-dm-sans)",
              fontSize: "13px",
              color: "var(--text-secondary)",
            }}
          >
            {t("releases.project")}:{" "}
            <Link
              href={`/projects/${project.id}`}
              className="hover:underline"
              style={{ color: "var(--beacon-accent)" }}
            >
              {project.name}
            </Link>
          </p>
        )}
      </div>

      {/* Tabs */}
      <Tabs defaultValue="basic">
        <TabsList variant="line">
          <TabsTrigger value="basic">{t("releases.tabBasic")}</TabsTrigger>
          <TabsTrigger value="report">
            {t("releases.tabReport")}
            {linkedSRs.length > 0 && (
              <span
                style={{
                  fontFamily: "var(--font-dm-sans)",
                  fontSize: "10px",
                  color: "var(--text-muted)",
                  marginLeft: "-1px",
                }}
              >
                +{linkedSRs.length}
              </span>
            )}
          </TabsTrigger>
        </TabsList>

        {/* Basic tab */}
        <TabsContent value="basic" className="space-y-6 pt-6">
          <div className="grid gap-6 lg:grid-cols-3">
            {/* Version Details card */}
            <div
              className="rounded-lg bg-surface lg:col-span-1"
              style={{ border: "1px solid var(--border)" }}
            >
            <div
              className="px-5 py-4"
              style={{ borderBottom: "1px solid var(--border)" }}
            >
              <h2
                style={{
                  fontFamily: "var(--font-raleway)",
                  fontSize: "16px",
                  fontWeight: 600,
                  color: "var(--foreground)",
                }}
              >
                {t("releases.versionDetails")}
              </h2>
            </div>
            <div className="space-y-3 px-5 py-4">
              <DetailRow label={t("releases.detail.version")} value={release.version} mono />
              <DetailRow
                label={t("releases.detail.sourceId")}
                value={release.source_id}
                mono
                small
              />
              {source && (
                <>
                  <DetailRow label={t("releases.detail.provider")} value={source.provider} />
                  <DetailRow label={t("releases.detail.repository")} value={source.repository} mono />
                </>
              )}
              {release.released_at && (
                <DetailRow
                  label={t("releases.detail.releasedAt")}
                  value={new Date(release.released_at).toLocaleString()}
                />
              )}
              <DetailRow
                label={t("releases.detail.ingestedAt")}
                value={new Date(release.created_at).toLocaleString()}
              />
              <DetailRow
                label={t("releases.detail.age")}
                value={timeAgo(release.released_at ?? release.created_at)}
              />
            </div>
          </div>

          {/* Release Notes */}
          <div
            className="rounded-lg bg-surface lg:col-span-2"
            style={{ border: "1px solid var(--border)" }}
          >
            <div
              className="px-5 py-4"
              style={{ borderBottom: "1px solid var(--border)" }}
            >
              <h2
                style={{
                  fontFamily: "var(--font-raleway)",
                  fontSize: "16px",
                  fontWeight: 600,
                  color: "var(--foreground)",
                }}
              >
                {t("releases.releaseNotes")}
              </h2>
            </div>
            <div className="px-5 py-4">
              {changelogHtml ? (
                <div
                  className="release-notes-content"
                  style={{
                    fontFamily: "var(--font-dm-sans)",
                    fontSize: "13px",
                    lineHeight: 1.7,
                    color: "var(--secondary-foreground)",
                  }}
                  dangerouslySetInnerHTML={{
                    __html: changelogHtml,
                  }}
                />
              ) : (
                <p
                  style={{
                    fontFamily: "var(--font-raleway)",
                    fontStyle: "italic",
                    fontSize: "14px",
                    color: "var(--text-muted)",
                  }}
                >
                  {t("releases.noReleaseNotes")}
                </p>
              )}
            </div>
          </div>
          </div>
        </TabsContent>

        {/* Semantic Releases tab */}
        <TabsContent value="report" className="pt-6">
          {linkedSRs.length > 0 ? (
            <div className="space-y-4">
              {linkedSRs.map((sr, index) => (
                <SRCollapsibleItem
                  key={sr.id}
                  sr={sr}
                  defaultExpanded={index === 0}
                />
              ))}
            </div>
          ) : (
            <div className="py-12 text-center">
              <p
                style={{
                  fontFamily: "var(--font-raleway)",
                  fontStyle: "italic",
                  fontSize: "14px",
                  color: "var(--text-muted)",
                }}
              >
                {t("releases.noSemanticReleases")}
              </p>
            </div>
          )}
        </TabsContent>
      </Tabs>
    </div>
  );
}

function SRCollapsibleItem({
  sr,
  defaultExpanded,
}: {
  sr: SemanticRelease;
  defaultExpanded: boolean;
}) {
  const { t } = useTranslation();
  const [expanded, setExpanded] = useState(defaultExpanded);
  const riskLevel = (sr.report?.urgency ?? sr.report?.risk_level)?.toLowerCase();

  return (
    <div
      className="rounded-lg bg-surface overflow-hidden"
      style={{ border: "1px solid var(--border)" }}
    >
      <button
        onClick={() => setExpanded(!expanded)}
        className="flex w-full items-center gap-3 px-5 py-4 text-left transition-colors hover:bg-background cursor-pointer"
      >
        {expanded ? (
          <ChevronDown size={16} className="shrink-0 text-text-muted" />
        ) : (
          <ChevronRight size={16} className="shrink-0 text-text-muted" />
        )}
        {riskLevel && (
          <UrgencyPill urgency={riskLevel} variant="text" />
        )}
        {sr.report?.subject && (
          <span
            className="flex-1 truncate text-[13px] text-secondary-foreground"
            style={{ fontFamily: "var(--font-dm-sans)" }}
          >
            {sr.report.subject.replace(/^Ready to Deploy:\s*/i, "")}
          </span>
        )}
        <span
          className="shrink-0 text-[12px] text-text-muted"
          style={{ fontFamily: "var(--font-dm-sans)" }}
        >
          {timeAgo(sr.completed_at ?? sr.created_at)}
        </span>
        <Link
          href={`/projects/${sr.project_id}/semantic-releases/${sr.id}`}
          className="shrink-0 text-text-muted hover:text-secondary-foreground transition-colors"
          onClick={(e) => e.stopPropagation()}
        >
          <ExternalLink size={14} />
        </Link>
      </button>
      {expanded && (
        <div
          className="px-5 pb-5"
          style={{ borderTop: "1px solid var(--border)" }}
        >
          <div className="pt-5">
            <SemanticReleaseReport report={sr.report} error={sr.error} compact />
          </div>
        </div>
      )}
    </div>
  );
}

function DetailRow({
  label,
  value,
  mono,
  small,
}: {
  label: string;
  value: string;
  mono?: boolean;
  small?: boolean;
}) {
  return (
    <div className="flex items-baseline justify-between gap-4">
      <span
        style={{
          fontFamily: "var(--font-dm-sans)",
          fontSize: "13px",
          color: "var(--text-muted)",
        }}
      >
        {label}
      </span>
      <span
        className="text-right"
        style={{
          fontFamily: mono ? "'JetBrains Mono', monospace" : "var(--font-dm-sans)",
          fontSize: small ? "11px" : "13px",
          color: "var(--secondary-foreground)",
        }}
      >
        {value}
      </span>
    </div>
  );
}
