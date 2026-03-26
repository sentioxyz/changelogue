"use client";

import { useState, useCallback, useEffect } from "react";

/**
 * useFilterParams syncs a Record<string, string> of filter values with URL query params.
 * Only reads/writes params whose keys are in `allowedKeys` to prevent cross-page leakage.
 */
export function useFilterParams(
  allowedKeys: string[],
  defaults?: Record<string, string>
): {
  filters: Record<string, string>;
  setFilters: (next: Record<string, string>) => void;
  page: number;
  setPage: (p: number) => void;
} {
  const [filters, setFiltersState] = useState<Record<string, string>>(() => {
    if (typeof window === "undefined") return defaults ?? {};
    const params = new URLSearchParams(window.location.search);
    const parsed: Record<string, string> = { ...(defaults ?? {}) };
    const allowed = new Set(allowedKeys);
    params.forEach((value, key) => {
      if (key !== "page" && allowed.has(key)) {
        parsed[key] = value;
      }
    });
    return parsed;
  });

  const [page, setPageState] = useState<number>(() => {
    if (typeof window === "undefined") return 1;
    const p = new URLSearchParams(window.location.search).get("page");
    return p ? Math.max(1, parseInt(p, 10) || 1) : 1;
  });

  useEffect(() => {
    const url = new URL(window.location.href);
    Array.from(url.searchParams.keys()).forEach((k) =>
      url.searchParams.delete(k)
    );
    for (const [key, value] of Object.entries(filters)) {
      if (value !== "" && value !== undefined) {
        url.searchParams.set(key, value);
      }
    }
    if (page > 1) {
      url.searchParams.set("page", String(page));
    }
    window.history.replaceState({}, "", url.toString());
  }, [filters, page]);

  const setFilters = useCallback((next: Record<string, string>) => {
    setFiltersState(next);
    setPageState(1);
  }, []);

  const setPage = useCallback((p: number) => {
    setPageState(p);
  }, []);

  return { filters, setFilters, page, setPage };
}
