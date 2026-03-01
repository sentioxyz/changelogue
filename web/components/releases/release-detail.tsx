"use client";

import useSWR from "swr";
import Link from "next/link";
import {
  releases as releasesApi,
  sources as sourcesApi,
  semanticReleases as srApi,
  projects as projectsApi,
} from "@/lib/api/client";
import { ProviderBadge } from "@/components/ui/provider-badge";
import { VersionChip } from "@/components/ui/version-chip";
import type { SemanticRelease, Source, Project } from "@/lib/api/types";
import { ArrowLeft, ExternalLink } from "lucide-react";

import { timeAgo } from "@/lib/format";
import { getPathSegment } from "@/lib/path";

/* ------------------------------------------------------------------ */
/*  Helpers                                                            */
/* ------------------------------------------------------------------ */

function getProviderUrl(
  provider: string,
  repository: string,
  version: string
): string | null {
  switch (provider) {
    case "github":
      return `https://github.com/${repository}/releases/tag/${version}`;
    case "dockerhub":
      return `https://hub.docker.com/r/${repository}/tags?name=${encodeURIComponent(version)}`;
    default:
      return null;
  }
}

function getProviderLabel(provider: string): string {
  switch (provider) {
    case "github":
      return "GitHub";
    case "dockerhub":
      return "Docker Hub";
    default:
      return provider;
  }
}

/* ------------------------------------------------------------------ */
/*  Component                                                          */
/* ------------------------------------------------------------------ */

export function ReleaseDetail() {
  // Read ID from URL path — useParams() returns stale "0" in static export
  const id = getPathSegment(1); // /releases/{id}
  /* Fetch release */
  const { data: releaseData, isLoading } = useSWR(`release-${id}`, () =>
    releasesApi.get(id)
  );

  const release = releaseData?.data;

  /* Fetch source info once we have the release */
  const { data: sourceData } = useSWR(
    release ? `source-${release.source_id}` : null,
    () => (release ? sourcesApi.get(release.source_id) : null)
  );
  const source: Source | undefined = sourceData?.data;

  /* Fetch project info once we have the source */
  const { data: projectData } = useSWR(
    source ? `project-${source.project_id}` : null,
    () => (source ? projectsApi.get(source.project_id) : null)
  );
  const project: Project | undefined = projectData?.data;

  /* Fetch linked semantic releases (via project) */
  const { data: srData } = useSWR(
    source ? `sr-for-release-${id}` : null,
    async () => {
      if (!source) return [];
      const res = await srApi.list(source.project_id, 1).catch(() => null);
      if (!res?.data) return [];
      /* Filter semantic releases whose version matches this release version */
      return res.data.filter((sr: SemanticRelease) => sr.version === release?.version);
    }
  );

  const linkedSRs: SemanticRelease[] = srData ?? [];

  /* Loading state */
  if (isLoading) {
    return (
      <div
        className="py-16 text-center"
        style={{
          fontFamily: "var(--font-dm-sans)",
          fontSize: "13px",
          color: "#6b7280",
        }}
      >
        Loading...
      </div>
    );
  }

  if (!release) {
    return (
      <div className="py-16 text-center">
        <p
          style={{
            fontFamily: "var(--font-fraunces)",
            fontStyle: "italic",
            fontSize: "15px",
            color: "#9ca3af",
          }}
        >
          Release not found
        </p>
      </div>
    );
  }

  return (
    <div className="space-y-8">
      {/* Back link */}
      <Link
        href="/releases"
        className="inline-flex items-center gap-1.5 transition-colors hover:opacity-70"
        style={{
          fontFamily: "var(--font-dm-sans)",
          fontSize: "13px",
          color: "#6b7280",
        }}
      >
        <ArrowLeft size={14} />
        Back to Releases
      </Link>

      {/* Header */}
      <div>
        <h1
          style={{
            fontFamily: "var(--font-fraunces)",
            fontSize: "24px",
            fontWeight: 700,
            color: "#111113",
          }}
        >
          Release {release.version}
        </h1>
        <div className="mt-2 flex items-center gap-3">
          {source && <ProviderBadge provider={source.provider} />}
          {source && (
            <span
              style={{
                fontFamily: "'JetBrains Mono', monospace",
                fontSize: "12px",
                color: "#374151",
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
                  color: "#e8601a",
                }}
              >
                View on {getProviderLabel(source.provider)}
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
              color: "#6b7280",
            }}
          >
            Project:{" "}
            <Link
              href={`/projects/${project.id}`}
              className="hover:underline"
              style={{ color: "#e8601a" }}
            >
              {project.name}
            </Link>
          </p>
        )}
      </div>

      {/* Info grid */}
      <div className="grid gap-6 lg:grid-cols-2">
        {/* Version Details card */}
        <div
          className="rounded-lg bg-white"
          style={{ border: "1px solid #e8e8e5" }}
        >
          <div
            className="px-5 py-4"
            style={{ borderBottom: "1px solid #e8e8e5" }}
          >
            <h2
              style={{
                fontFamily: "var(--font-fraunces)",
                fontSize: "16px",
                fontWeight: 600,
                color: "#111113",
              }}
            >
              Version Details
            </h2>
          </div>
          <div className="space-y-3 px-5 py-4">
            <DetailRow label="Version" value={release.version} mono />
            <DetailRow
              label="Source ID"
              value={release.source_id}
              mono
              small
            />
            {source && (
              <>
                <DetailRow label="Provider" value={source.provider} />
                <DetailRow label="Repository" value={source.repository} mono />
              </>
            )}
            {release.released_at && (
              <DetailRow
                label="Released At"
                value={new Date(release.released_at).toLocaleString()}
              />
            )}
            <DetailRow
              label="Ingested At"
              value={new Date(release.created_at).toLocaleString()}
            />
            <DetailRow
              label="Age"
              value={timeAgo(release.released_at ?? release.created_at)}
            />
          </div>
        </div>

        {/* Linked Semantic Releases */}
        <div
          className="rounded-lg bg-white"
          style={{ border: "1px solid #e8e8e5" }}
        >
          <div
            className="px-5 py-4"
            style={{ borderBottom: "1px solid #e8e8e5" }}
          >
            <h2
              style={{
                fontFamily: "var(--font-fraunces)",
                fontSize: "16px",
                fontWeight: 600,
                color: "#111113",
              }}
            >
              Semantic Releases
            </h2>
          </div>
          <div className="px-5 py-4">
            {linkedSRs.length > 0 ? (
              <div className="space-y-3">
                {linkedSRs.map((sr) => (
                  <Link
                    key={sr.id}
                    href={`/projects/${sr.project_id}/semantic-releases/${sr.id}`}
                    className="block rounded-lg px-4 py-3 transition-colors hover:bg-[#fafaf9]"
                    style={{ border: "1px solid #e8e8e5" }}
                  >
                    <div className="flex items-center justify-between">
                      <VersionChip version={sr.version} />
                      <span
                        className="rounded-full px-2 py-0.5"
                        style={{
                          fontFamily: "var(--font-dm-sans)",
                          fontSize: "11px",
                          fontWeight: 500,
                          color:
                            sr.status === "completed" ? "#16a34a" : "#e8601a",
                          backgroundColor:
                            sr.status === "completed" ? "#f0fdf4" : "#fff7ed",
                        }}
                      >
                        {sr.status}
                      </span>
                    </div>
                    {sr.report?.summary && (
                      <p
                        className="mt-2 line-clamp-2"
                        style={{
                          fontFamily: "var(--font-dm-sans)",
                          fontStyle: "italic",
                          fontSize: "13px",
                          color: "#6b7280",
                        }}
                      >
                        {sr.report.summary}
                      </p>
                    )}
                    <p
                      className="mt-1"
                      style={{
                        fontFamily: "var(--font-dm-sans)",
                        fontSize: "12px",
                        color: "#9ca3af",
                      }}
                    >
                      {timeAgo(sr.created_at)}
                    </p>
                  </Link>
                ))}
              </div>
            ) : (
              <div className="py-6 text-center">
                <p
                  style={{
                    fontFamily: "var(--font-fraunces)",
                    fontStyle: "italic",
                    fontSize: "14px",
                    color: "#9ca3af",
                  }}
                >
                  No semantic releases linked
                </p>
              </div>
            )}
          </div>
        </div>
      </div>

      {/* Release Notes */}
      <div
        className="rounded-lg bg-white"
        style={{ border: "1px solid #e8e8e5" }}
      >
        <div
          className="px-5 py-4"
          style={{ borderBottom: "1px solid #e8e8e5" }}
        >
          <h2
            style={{
              fontFamily: "var(--font-fraunces)",
              fontSize: "16px",
              fontWeight: 600,
              color: "#111113",
            }}
          >
            Release Notes
          </h2>
        </div>
        <div className="px-5 py-4">
          {release.raw_data?.changelog ? (
            <div
              className="release-notes-content"
              style={{
                fontFamily: "var(--font-dm-sans)",
                fontSize: "13px",
                lineHeight: 1.7,
                color: "#374151",
              }}
              dangerouslySetInnerHTML={{
                __html: String(release.raw_data.changelog),
              }}
            />
          ) : (
            <p
              style={{
                fontFamily: "var(--font-fraunces)",
                fontStyle: "italic",
                fontSize: "14px",
                color: "#9ca3af",
              }}
            >
              No release notes available
            </p>
          )}
        </div>
      </div>
    </div>
  );
}

/* ------------------------------------------------------------------ */
/*  Sub-components                                                     */
/* ------------------------------------------------------------------ */

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
          color: "#9ca3af",
        }}
      >
        {label}
      </span>
      <span
        className="text-right"
        style={{
          fontFamily: mono ? "'JetBrains Mono', monospace" : "var(--font-dm-sans)",
          fontSize: small ? "11px" : "13px",
          color: "#374151",
        }}
      >
        {value}
      </span>
    </div>
  );
}
