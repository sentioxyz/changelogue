"use client";

import { useState, useEffect, useCallback, Suspense } from "react";
import { useSearchParams } from "next/navigation";
import useSWR, { mutate } from "swr";
import Link from "next/link";
import {
  releases as releasesApi,
  projects as projectsApi,
  agent,
} from "@/lib/api/client";
import { ProviderBadge } from "@/components/ui/provider-badge";
import { VersionChip } from "@/components/ui/version-chip";
import type { Release, Project } from "@/lib/api/types";
import { ExternalLink, Sparkles, Loader2 } from "lucide-react";
import { URGENCY_STYLES } from "@/components/ui/urgency-pill";
import { useTranslation } from "@/lib/i18n/context";

import { timeAgo } from "@/lib/format";

const PER_PAGE = 15;
const SSE_BASE = process.env.NEXT_PUBLIC_API_URL || "/api/v1";

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
    case "ecr-public":
      return `https://gallery.ecr.aws/${repository}`;
    case "gitlab":
      return `https://gitlab.com/${repository}/-/releases/${version}`;
    default:
      return null;
  }
}

/* ------------------------------------------------------------------ */
/*  Page                                                               */
/* ------------------------------------------------------------------ */

export default function ReleasesPage() {
  return (
    <Suspense>
      <ReleasesPageInner />
    </Suspense>
  );
}

function ReleasesPageInner() {
  const { t } = useTranslation();
  const searchParams = useSearchParams();
  const initialProject = searchParams.get("project") ?? "all";
  const initialShowExcluded = searchParams.get("show_excluded") !== "false";
  const [page, setPage] = useState(1);
  const [projectFilter, setProjectFilter] = useState<string>(initialProject);
  const [showExcluded, setShowExcluded] = useState(initialShowExcluded);
  const [triggeringVersion, setTriggeringVersion] = useState<string | null>(null);

  /* Fetch projects for the filter dropdown */
  const { data: projectsData } = useSWR("projects-for-filter", async () => {
    const firstPage = await projectsApi.list(1, 100);
    return firstPage;
  });

  /* Fetch releases — scoped by project or all */
  const { data: scopedData, isLoading: scopedLoading } = useSWR(
    projectFilter !== "all" ? ["releases", page, projectFilter, showExcluded] : null,
    () => releasesApi.listByProject(projectFilter, page, PER_PAGE, showExcluded)
  );

  const { data: allReleasesData, isLoading: allLoading } = useSWR(
    projectFilter === "all" ? ["all-releases", page, showExcluded] : null,
    () => releasesApi.list(page, PER_PAGE, showExcluded)
  );

  const isLoading = projectFilter !== "all" ? scopedLoading : allLoading;

  /* Releases come pre-enriched with project/source metadata from the backend */
  const releases: Release[] =
    projectFilter !== "all"
      ? scopedData?.data ?? []
      : allReleasesData?.data ?? [];

  /* Pagination math */
  const activeData = projectFilter !== "all" ? scopedData : allReleasesData;
  const total = activeData?.meta?.total ?? 0;
  const totalPages = Math.max(1, Math.ceil(total / PER_PAGE));
  const startRow = (page - 1) * PER_PAGE + 1;
  const endRow = Math.min(page * PER_PAGE, total);

  /* SSE revalidation — refresh on semantic_release events */
  const revalidateReleases = useCallback(() => {
    return mutate((key) => {
      if (Array.isArray(key)) {
        return key[0] === "releases" || key[0] === "all-releases";
      }
      return false;
    }, undefined, { revalidate: true });
  }, []);

  useEffect(() => {
    let es: EventSource | null = null;
    let retryTimer: ReturnType<typeof setTimeout> | null = null;

    function connect() {
      try {
        es = new EventSource(`${SSE_BASE}/events`);
        es.onmessage = (event) => {
          try {
            const parsed = JSON.parse(event.data);
            if (parsed.type === "semantic_release") {
              setTriggeringVersion(null);
              revalidateReleases();
            }
          } catch {
            // ignore parse errors
          }
        };
        es.onerror = () => {
          es?.close();
          retryTimer = setTimeout(connect, 5000);
        };
      } catch {
        retryTimer = setTimeout(connect, 5000);
      }
    }

    connect();
    return () => {
      es?.close();
      if (retryTimer) clearTimeout(retryTimer);
    };
  }, [revalidateReleases]);

  /* Trigger agent run — keep spinner until SWR revalidation brings back the new status */
  const handleTrigger = async (projectId: string, version: string) => {
    setTriggeringVersion(version);
    try {
      await agent.triggerRun(projectId, version);
      await revalidateReleases();
    } catch {
      setTriggeringVersion(null);
    }
  };

  const tableHeaders = [
    t("releases.col.project"),
    t("releases.col.provider"),
    t("releases.col.repository"),
    t("releases.col.version"),
    t("releases.col.released"),
    t("releases.col.age"),
    t("releases.col.report"),
    "",
  ];

  return (
    <div className="space-y-6">
      {/* Page title */}
      <div>
        <h1
          style={{
            fontFamily: "var(--font-fraunces)",
            fontSize: "24px",
            fontWeight: 700,
            color: "var(--foreground)",
          }}
        >
          {t("releases.title")}
        </h1>
        <p className="mt-1 text-[13px] text-text-secondary" style={{ fontFamily: "var(--font-dm-sans)" }}>
          {t("releases.description")}
        </p>
      </div>

      {/* Filters */}
      <div className="flex items-center gap-3">
        <select
          value={projectFilter}
          onChange={(e) => {
            const val = e.target.value;
            setProjectFilter(val);
            setPage(1);
            const params = new URLSearchParams(window.location.search);
            if (val === "all") params.delete("project");
            else params.set("project", val);
            const qs = params.toString();
            window.history.pushState({}, "", qs ? `?${qs}` : window.location.pathname);
          }}
          className="appearance-none rounded-md bg-surface px-3 py-2 pr-8 outline-none transition-shadow"
          style={{
            fontFamily: "var(--font-dm-sans)",
            fontSize: "13px",
            color: "var(--foreground)",
            border: "1px solid var(--border)",
          }}
          onFocus={(e) =>
            (e.currentTarget.style.boxShadow = "0 0 0 2px color-mix(in srgb, var(--beacon-accent) 25%, transparent)")
          }
          onBlur={(e) => (e.currentTarget.style.boxShadow = "none")}
        >
          <option value="all">{t("releases.allProjects")}</option>
          {projectsData?.data.map((p: Project) => (
            <option key={p.id} value={p.id}>
              {p.name}
            </option>
          ))}
        </select>

        <label className="inline-flex items-center gap-2 cursor-pointer select-none">
          <span style={{
            fontFamily: "var(--font-dm-sans)",
            fontSize: "13px",
            color: "var(--text-secondary)",
          }}>
            {t("releases.showExcluded")}
          </span>
          <button
            role="switch"
            aria-checked={showExcluded}
            onClick={() => {
              const next = !showExcluded;
              setShowExcluded(next);
              setPage(1);
              const params = new URLSearchParams(window.location.search);
              if (next) params.delete("show_excluded");
              else params.set("show_excluded", "false");
              window.history.replaceState({}, "", `?${params.toString()}`);
            }}
            className="relative inline-flex h-5 w-9 items-center rounded-full transition-colors"
            style={{ backgroundColor: showExcluded ? "var(--beacon-accent)" : "#d1d5db" }}
          >
            <span
              className="inline-block h-3.5 w-3.5 rounded-full bg-white transition-transform"
              style={{ transform: showExcluded ? "translateX(18px)" : "translateX(3px)" }}
            />
          </button>
        </label>
      </div>

      {/* Table card */}
      <div
        className="overflow-hidden rounded-lg bg-surface"
        style={{ border: "1px solid var(--border)" }}
      >
        {isLoading ? (
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
        ) : releases.length === 0 ? (
          <div className="py-16 text-center">
            <p
              style={{
                fontFamily: "var(--font-fraunces)",
                fontStyle: "italic",
                fontSize: "15px",
                color: "var(--text-muted)",
              }}
            >
              {t("releases.empty")}
            </p>
          </div>
        ) : (
          <table className="w-full">
            <thead>
              <tr style={{ borderBottom: "1px solid var(--border)", backgroundColor: "var(--background)" }}>
                {tableHeaders.map(
                  (col) => (
                    <th
                      key={col}
                      className="px-4 py-3 text-left"
                      style={{
                        fontFamily: "var(--font-dm-sans)",
                        fontSize: "11px",
                        fontWeight: 600,
                        textTransform: "uppercase" as const,
                        letterSpacing: "0.08em",
                        color: "var(--text-muted)",
                      }}
                    >
                      {col}
                    </th>
                  )
                )}
              </tr>
            </thead>
            <tbody>
              {releases.map((release) => (
                <tr
                  key={release.id}
                  className={`transition-colors ${release.excluded ? '' : 'hover:bg-background'}`}
                  style={{
                    borderBottom: "1px solid var(--border)",
                    opacity: release.excluded ? 0.45 : 1,
                  }}
                >
                  {/* Project */}
                  <td className="px-4 py-3">
                    {release.project_id ? (
                      <Link
                        href={`/projects/${release.project_id}`}
                        className="hover:underline"
                        style={{
                          fontFamily: "var(--font-dm-sans)",
                          fontSize: "13px",
                          color: "var(--foreground)",
                          fontWeight: 500,
                        }}
                      >
                        {release.project_name ?? "\u2014"}
                      </Link>
                    ) : (
                      <span
                        style={{
                          fontFamily: "var(--font-dm-sans)",
                          fontSize: "13px",
                          color: "var(--text-muted)",
                        }}
                      >
                        {"\u2014"}
                      </span>
                    )}
                  </td>

                  {/* Provider */}
                  <td className="px-4 py-3">
                    {release.provider ? (
                      <ProviderBadge provider={release.provider} />
                    ) : (
                      <span
                        style={{
                          fontFamily: "var(--font-dm-sans)",
                          fontSize: "13px",
                          color: "var(--text-muted)",
                        }}
                      >
                        {"\u2014"}
                      </span>
                    )}
                  </td>

                  {/* Repository */}
                  <td className="px-4 py-3">
                    <span
                      style={{
                        fontFamily: "'JetBrains Mono', monospace",
                        fontSize: "12px",
                        color: "var(--secondary-foreground)",
                      }}
                    >
                      {release.repository ?? release.source_id}
                    </span>
                  </td>

                  {/* Version */}
                  <td className="px-4 py-3">
                    <Link href={`/releases/${release.id}`}>
                      <VersionChip version={release.version} />
                    </Link>
                  </td>

                  {/* Released date */}
                  <td className="px-4 py-3">
                    <span
                      style={{
                        fontFamily: "var(--font-dm-sans)",
                        fontSize: "13px",
                        color: "var(--text-secondary)",
                      }}
                    >
                      {release.released_at
                        ? new Date(release.released_at).toLocaleDateString()
                        : "\u2014"}
                    </span>
                  </td>

                  {/* Age */}
                  <td className="px-4 py-3">
                    <span
                      style={{
                        fontFamily: "var(--font-dm-sans)",
                        fontSize: "13px",
                        color: "var(--text-muted)",
                      }}
                    >
                      {timeAgo(release.released_at ?? release.created_at)}
                    </span>
                  </td>

                  {/* Report */}
                  <td className="px-4 py-3">
                    {release.semantic_release_status === "completed" && release.semantic_release_id && release.project_id ? (() => {
                      const pill = release.semantic_release_urgency
                        ? URGENCY_STYLES[release.semantic_release_urgency.toLowerCase()]
                        : undefined;
                      return pill ? (
                        <Link
                          href={`/projects/${release.project_id}/semantic-releases/${release.semantic_release_id}`}
                          className="inline-flex items-center gap-0.5 rounded-full px-2 py-0.5 text-[10px] font-semibold transition-colors"
                          style={{ backgroundColor: pill.bg, border: `1px solid ${pill.border}`, color: pill.text, fontFamily: "var(--font-dm-sans)" }}
                          title={t("releases.viewReport")}
                        >
                          <pill.icon size={10} /> {release.semantic_release_urgency}
                        </Link>
                      ) : (
                        <Link
                          href={`/projects/${release.project_id}/semantic-releases/${release.semantic_release_id}`}
                          className="inline-flex items-center gap-0.5 rounded-full px-2 py-0.5 text-[10px] font-semibold transition-colors bg-muted"
                          style={{ border: "1px solid color-mix(in srgb, var(--text-secondary) 18%, transparent)", color: "var(--text-secondary)", fontFamily: "var(--font-dm-sans)" }}
                          title={t("releases.viewReport")}
                        >
                          {t("releases.report")}
                        </Link>
                      );
                    })() : release.semantic_release_status === "pending" || release.semantic_release_status === "processing" || triggeringVersion === release.version ? (
                      <span
                        className="inline-flex items-center gap-1 rounded-full px-2 py-0.5 text-[10px] font-semibold"
                        style={{ color: "#2563eb", backgroundColor: "rgba(37,99,235,0.08)", border: "1px solid rgba(37,99,235,0.18)", fontFamily: "var(--font-dm-sans)" }}
                      >
                        <Loader2 size={10} className="animate-spin" />
                        {release.semantic_release_status || t("releases.analyzing")}
                      </span>
                    ) : release.project_id && !release.excluded ? (
                      <button
                        onClick={() => handleTrigger(release.project_id!, release.version)}
                        disabled={triggeringVersion === release.version}
                        className="inline-flex items-center gap-1 rounded-full px-2 py-0.5 text-[10px] font-semibold transition-colors hover:bg-mono-bg disabled:opacity-50 cursor-pointer"
                        style={{
                          color: "var(--text-secondary)",
                          backgroundColor: "color-mix(in srgb, var(--text-secondary) 6%, transparent)",
                          border: "1px solid color-mix(in srgb, var(--text-secondary) 18%, transparent)",
                          fontFamily: "var(--font-dm-sans)",
                        }}
                        title={t("releases.generateReport")}
                      >
                        <Sparkles size={10} />
                        {t("releases.analyze")}
                      </button>
                    ) : (
                      <span
                        style={{
                          fontFamily: "var(--font-dm-sans)",
                          fontSize: "13px",
                          color: "var(--text-muted)",
                        }}
                      >
                        {"\u2014"}
                      </span>
                    )}
                  </td>

                  {/* Provider link */}
                  <td className="px-4 py-3">
                    {release.provider && release.repository && (() => {
                      const url = getProviderUrl(release.provider, release.repository, release.version);
                      return url ? (
                        <a
                          href={url}
                          target="_blank"
                          rel="noopener noreferrer"
                          className="inline-flex items-center transition-colors hover:opacity-70"
                          style={{ color: "var(--text-muted)" }}
                          title={t("releases.viewOnProvider")}
                        >
                          <ExternalLink size={14} />
                        </a>
                      ) : null;
                    })()}
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        )}
      </div>

      {/* Pagination */}
      {total > 0 && (
        <div className="flex items-center justify-between">
          <span
            style={{
              fontFamily: "var(--font-dm-sans)",
              fontSize: "13px",
              color: "var(--text-muted)",
            }}
          >
            {startRow}&ndash;{endRow} of {total}
          </span>
          <div className="flex items-center gap-2">
            <button
              disabled={page <= 1}
              onClick={() => setPage(page - 1)}
              className="rounded-md bg-surface px-3 py-1.5 transition-colors hover:bg-background disabled:cursor-not-allowed disabled:opacity-40"
              style={{
                fontFamily: "var(--font-dm-sans)",
                fontSize: "13px",
                color: "var(--secondary-foreground)",
                border: "1px solid var(--border)",
              }}
            >
              {t("releases.previous")}
            </button>
            <button
              disabled={page >= totalPages}
              onClick={() => setPage(page + 1)}
              className="rounded-md bg-surface px-3 py-1.5 transition-colors hover:bg-background disabled:cursor-not-allowed disabled:opacity-40"
              style={{
                fontFamily: "var(--font-dm-sans)",
                fontSize: "13px",
                color: "var(--secondary-foreground)",
                border: "1px solid var(--border)",
              }}
            >
              {t("releases.next")}
            </button>
          </div>
        </div>
      )}
    </div>
  );
}
