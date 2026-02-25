"use client";

import useSWR from "swr";
import Link from "next/link";
import { semanticReleases as srApi, projects as projectsApi, releases as releasesApi, sources as sourcesApi } from "@/lib/api/client";
import { StatusDot } from "@/components/ui/status-dot";
import { VersionChip } from "@/components/ui/version-chip";
import { SectionLabel } from "@/components/ui/section-label";
import { UrgencyCallout } from "@/components/ui/urgency-callout";
import { ProviderBadge } from "@/components/ui/provider-badge";
import { timeAgo } from "@/lib/format";

export function SemanticReleaseDetail({ projectId, srId }: { projectId: string; srId: string }) {
  const { data, isLoading } = useSWR(`sr-${srId}`, () => srApi.get(srId));
  const { data: projectData } = useSWR(`project-${projectId}`, () => projectsApi.get(projectId));
  const { data: releasesData } = useSWR(`project-releases-${projectId}`, () => releasesApi.listByProject(projectId));
  const { data: sourcesData } = useSWR(`project-sources-${projectId}`, () => sourcesApi.listByProject(projectId));

  if (isLoading) {
    return (
      <div className="flex justify-center py-20">
        <div className="text-[13px] text-[#9ca3af]" style={{ fontFamily: "var(--font-dm-sans)" }}>
          Loading...
        </div>
      </div>
    );
  }

  const sr = data?.data;
  if (!sr) {
    return (
      <div className="flex justify-center py-20">
        <div className="text-[13px] text-[#9ca3af]" style={{ fontFamily: "var(--font-dm-sans)" }}>
          Semantic release not found
        </div>
      </div>
    );
  }

  const project = projectData?.data;
  const releasesList = releasesData?.data ?? [];
  const sourcesList = sourcesData?.data ?? [];
  const sourcesById = Object.fromEntries(sourcesList.map((s) => [s.id, s]));

  return (
    <div className="fade-in mx-auto max-w-[760px]">
      {/* 1. Breadcrumb */}
      <nav
        className="mb-6 flex items-center gap-1.5 text-[12px] text-[#9ca3af]"
        style={{ fontFamily: "var(--font-dm-sans)" }}
      >
        <Link href="/projects" className="hover:text-[#111113] transition-colors">
          Projects
        </Link>
        <span>/</span>
        <Link href={`/projects/${projectId}`} className="hover:text-[#111113] transition-colors">
          {project?.name ?? projectId}
        </Link>
        <span>/</span>
        <Link href={`/projects/${projectId}/semantic-releases`} className="hover:text-[#111113] transition-colors">
          Semantic Releases
        </Link>
      </nav>

      {/* 2. Byline */}
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

      {/* 4. Meta line */}
      <div
        className="mt-3 flex items-center gap-2 text-[13px] text-[#6b7280]"
        style={{ fontFamily: "var(--font-dm-sans)" }}
      >
        <StatusDot status={sr.status} />
        <span>
          {sr.status}
          {sr.completed_at && ` \u00b7 generated ${timeAgo(sr.completed_at)}`}
        </span>
      </div>

      {/* 5. Divider */}
      <hr className="my-8 border-0" style={{ borderTop: "1px solid #e8e8e5" }} />

      {/* 6. Error state */}
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

      {sr.report && (
        <div className="space-y-10">
          {/* 7. Summary section */}
          <section>
            <SectionLabel className="mb-3">Summary</SectionLabel>
            <p
              className="text-[16px] leading-[1.7] text-[#111113]"
              style={{ fontFamily: "var(--font-dm-sans)" }}
            >
              {sr.report.summary}
            </p>
          </section>

          {/* 8. Urgency callout */}
          <UrgencyCallout urgency={sr.report.urgency} />

          {/* 9. Availability + Adoption cards */}
          <div className="grid grid-cols-2 gap-4">
            <div
              className="rounded-md p-5"
              style={{ border: "1px solid #e8e8e5", backgroundColor: "#ffffff" }}
            >
              <SectionLabel className="mb-2">Availability</SectionLabel>
              <p
                className="text-[14px] leading-[1.6] text-[#111113]"
                style={{ fontFamily: "var(--font-dm-sans)" }}
              >
                {sr.report.availability}
              </p>
            </div>
            <div
              className="rounded-md p-5"
              style={{ border: "1px solid #e8e8e5", backgroundColor: "#ffffff" }}
            >
              <SectionLabel className="mb-2">Adoption</SectionLabel>
              <p
                className="text-[14px] leading-[1.6] text-[#111113]"
                style={{ fontFamily: "var(--font-dm-sans)" }}
              >
                {sr.report.adoption}
              </p>
            </div>
          </div>

          {/* 10. Recommendation pull-quote */}
          {sr.report.recommendation && (
            <blockquote
              className="rounded-md px-5 py-4 text-[18px] italic leading-[1.6] text-[#16181c]"
              style={{
                fontFamily: "var(--font-fraunces)",
                borderLeft: "3px solid #e8601a",
                backgroundColor: "#fafaf9",
              }}
            >
              {sr.report.recommendation}
            </blockquote>
          )}
        </div>
      )}

      {/* 11. Source Releases section */}
      {releasesList.length > 0 && (
        <section className="mt-10">
          <SectionLabel className="mb-4">Source Releases</SectionLabel>
          <div
            className="overflow-hidden rounded-md"
            style={{ border: "1px solid #e8e8e5" }}
          >
            <table className="w-full text-left">
              <thead>
                <tr style={{ backgroundColor: "#fafaf9", borderBottom: "1px solid #e8e8e5" }}>
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
                  return (
                    <tr
                      key={rel.id}
                      style={{ borderBottom: "1px solid #e8e8e5" }}
                      className="last:border-b-0"
                    >
                      <td className="px-4 py-3">
                        {source ? (
                          <ProviderBadge provider={source.provider} />
                        ) : (
                          <span className="text-[12px] text-[#9ca3af]">{"\u2014"}</span>
                        )}
                      </td>
                      <td className="px-4 py-3">
                        <span
                          className="text-[13px] text-[#374151]"
                          style={{ fontFamily: "'JetBrains Mono', monospace" }}
                        >
                          {source?.repository ?? "\u2014"}
                        </span>
                      </td>
                      <td className="px-4 py-3">
                        <VersionChip version={rel.version} />
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
    </div>
  );
}
