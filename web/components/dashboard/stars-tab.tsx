"use client";

import { useState, useCallback } from "react";
import useSWR, { mutate } from "swr";
import { Star, Loader2, Check, Plus } from "lucide-react";
import { FaGithub } from "react-icons/fa";
import { suggestions, projects, sources } from "@/lib/api/client";
import type { SuggestionItem } from "@/lib/api/types";
import { useTranslation } from "@/lib/i18n/context";

const INITIAL_SHOW = 6;

export function StarsTab() {
  const { t } = useTranslation();
  const [trackingIds, setTrackingIds] = useState<Set<string>>(new Set());
  const [showAll, setShowAll] = useState(false);

  const { data, isLoading, error } = useSWR(
    "suggestions-stars",
    () => suggestions.stars(),
    { revalidateOnFocus: false }
  );

  const items = data?.data ?? [];
  const visible = showAll ? items : items.slice(0, INITIAL_SHOW);

  const handleTrack = useCallback(async (item: SuggestionItem) => {
    const key = item.full_name;
    setTrackingIds((prev) => new Set(prev).add(key));
    try {
      const projRes = await projects.create({
        name: item.full_name,
        description: item.description,
      });
      const project = projRes.data;
      const sourceRes = await sources.create(project.id, {
        provider: "github",
        repository: item.full_name,
        poll_interval_seconds: 86400,
        enabled: true,
      });
      await sources.poll(sourceRes.data.id);
      await mutate("suggestions-stars");
      await mutate("projects-list");
      await mutate("projects-for-dashboard");
    } catch (err: unknown) {
      const msg = err instanceof Error ? err.message : "Failed to track";
      alert(msg);
    } finally {
      setTrackingIds((prev) => {
        const next = new Set(prev);
        next.delete(key);
        return next;
      });
    }
  }, []);

  if (isLoading) {
    return (
      <div className="flex items-center justify-center py-10">
        <Loader2
          className="h-4 w-4 animate-spin text-beacon-accent"
        />
        <span
          className="ml-2 text-text-secondary"
          style={{
            fontFamily: "var(--font-dm-sans)",
            fontSize: "13px",
          }}
        >
          {t("dashboard.stars.loading")}
        </span>
      </div>
    );
  }

  if (error) {
    return (
      <div className="flex items-center justify-center py-10">
        <span
          style={{
            fontFamily: "var(--font-dm-sans)",
            fontSize: "13px",
            color: "#ef4444",
          }}
        >
          {t("dashboard.stars.loadError")}
        </span>
      </div>
    );
  }

  if (items.length === 0) {
    return (
      <div className="flex items-center justify-center py-10">
        <span
          className="text-text-secondary"
          style={{
            fontFamily: "var(--font-dm-sans)",
            fontSize: "13px",
          }}
        >
          {t("dashboard.stars.noStars")}
        </span>
      </div>
    );
  }

  return (
    <div>
      <div className="grid grid-cols-1 gap-3 sm:grid-cols-2 lg:grid-cols-3">
        {visible.map((item) => {
          const tracked = item.tracked;
          const tracking = trackingIds.has(item.full_name);

          return (
            <div
              key={item.full_name}
              className="rounded-lg border border-border"
              style={{
                padding: "12px",
                backgroundColor: tracked ? "var(--background)" : "var(--surface)",
                opacity: tracked ? 0.6 : 1,
              }}
            >
              <div className="flex items-start justify-between gap-2 mb-2">
                <div className="flex items-center gap-1.5 min-w-0">
                  <FaGithub
                    className="h-3.5 w-3.5 flex-shrink-0 text-text-secondary"
                  />
                  <span
                    className="truncate text-foreground"
                    style={{
                      fontFamily: "var(--font-dm-sans)",
                      fontSize: "13px",
                      fontWeight: 600,
                    }}
                  >
                    {item.full_name}
                  </span>
                </div>
                {tracked ? (
                  <span
                    className="flex items-center gap-1 shrink-0"
                    style={{
                      fontFamily: "var(--font-dm-sans)",
                      fontSize: "11px",
                      color: "#16a34a",
                    }}
                  >
                    <Check className="h-3 w-3" />
                    {t("dashboard.stars.tracked")}
                  </span>
                ) : (
                  <button
                    onClick={() => handleTrack(item)}
                    disabled={tracking}
                    className="shrink-0 flex items-center gap-1 rounded-md px-2.5 py-1 text-white transition-opacity hover:opacity-90 disabled:opacity-50 bg-beacon-accent"
                    style={{
                      fontFamily: "var(--font-dm-sans)",
                      fontSize: "11px",
                      fontWeight: 500,
                      cursor: tracking ? "wait" : "pointer",
                    }}
                  >
                    {tracking ? (
                      <Loader2 className="h-3 w-3 animate-spin" />
                    ) : (
                      <Plus className="h-3 w-3" />
                    )}
                    {tracking ? t("dashboard.stars.tracking") : t("dashboard.stars.track")}
                  </button>
                )}
              </div>
              {item.description && (
                <p
                  className="line-clamp-2 mb-2 text-text-secondary"
                  style={{
                    fontFamily: "var(--font-dm-sans)",
                    fontSize: "12px",
                    lineHeight: "1.4",
                  }}
                >
                  {item.description}
                </p>
              )}
              <div className="flex items-center gap-3">
                <span
                  className="flex items-center gap-1 text-text-muted"
                  style={{
                    fontFamily: "var(--font-dm-sans)",
                    fontSize: "11px",
                  }}
                >
                  <Star className="h-3 w-3 text-beacon-accent" />
                  {item.stars.toLocaleString()}
                </span>
                {item.language && (
                  <span
                    className="text-text-muted"
                    style={{
                      fontFamily: "var(--font-dm-sans)",
                      fontSize: "11px",
                    }}
                  >
                    {item.language}
                  </span>
                )}
              </div>
            </div>
          );
        })}
      </div>
      {items.length > INITIAL_SHOW && !showAll && (
        <div className="text-center mt-3">
          <button
            onClick={() => setShowAll(true)}
            className="text-beacon-accent"
            style={{
              fontFamily: "var(--font-dm-sans)",
              fontSize: "12px",
              background: "none",
              border: "none",
              cursor: "pointer",
            }}
          >
            {t("dashboard.stars.showAll")} {items.length} {t("dashboard.stars.starredRepos")} →
          </button>
        </div>
      )}
    </div>
  );
}
