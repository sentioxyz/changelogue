"use client";

import { useState, useRef, useEffect } from "react";
import { X, Plus, Search, Check } from "lucide-react";

/* Types */

export interface FilterOption {
  value: string;
  label: string;
}

export interface FilterConfig {
  key: string;
  label: string;
  type: "select" | "boolean" | "date-range";
  options?: FilterOption[];
  defaultValue?: string;
}

export interface FilterBarProps {
  filters: FilterConfig[];
  value: Record<string, string>;
  onChange: (value: Record<string, string>) => void;
}

/* Date presets */

const DATE_PRESETS: FilterOption[] = [
  { value: "7d", label: "Last 7 days" },
  { value: "30d", label: "Last 30 days" },
  { value: "90d", label: "Last 90 days" },
  { value: "1y", label: "Last year" },
];

export function expandDatePreset(preset: string): { date_from: string; date_to?: string } {
  const now = new Date();
  let from: Date;
  switch (preset) {
    case "7d":
      from = new Date(now.getTime() - 7 * 86400000);
      break;
    case "30d":
      from = new Date(now.getTime() - 30 * 86400000);
      break;
    case "90d":
      from = new Date(now.getTime() - 90 * 86400000);
      break;
    case "1y":
      from = new Date(now.getTime() - 365 * 86400000);
      break;
    default:
      return { date_from: preset };
  }
  return { date_from: from.toISOString().slice(0, 10) };
}

/* Chip sub-component */

function Chip({
  label,
  displayValue,
  onRemove,
  onClick,
}: {
  label: string;
  displayValue: string;
  onRemove: () => void;
  onClick: () => void;
}) {
  return (
    <button
      type="button"
      onClick={onClick}
      className="inline-flex items-center gap-1.5 rounded-full border border-border bg-surface-secondary px-2.5 py-1 text-xs transition-colors hover:bg-surface-tertiary"
      style={{ fontFamily: "var(--font-dm-sans)" }}
    >
      <span className="text-text-muted">{label}:</span>
      <span className="text-text-primary">{displayValue}</span>
      <span
        role="button"
        tabIndex={0}
        className="ml-0.5 text-text-muted hover:text-text-primary"
        onClick={(e) => { e.stopPropagation(); onRemove(); }}
        onKeyDown={(e) => { if (e.key === "Enter") { e.stopPropagation(); onRemove(); } }}
      >
        <X size={12} />
      </span>
    </button>
  );
}

/* FilterBar */

export function FilterBar({ filters, value, onChange }: FilterBarProps) {
  const [popoverOpen, setPopoverOpen] = useState(false);
  const [selectedType, setSelectedType] = useState<string | null>(null);
  const [search, setSearch] = useState("");
  const popoverRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    function handleClick(e: MouseEvent) {
      if (popoverRef.current && !popoverRef.current.contains(e.target as Node)) {
        setPopoverOpen(false);
        setSelectedType(null);
        setSearch("");
      }
    }
    if (popoverOpen) {
      document.addEventListener("mousedown", handleClick);
      return () => document.removeEventListener("mousedown", handleClick);
    }
  }, [popoverOpen]);

  const activeFilters = filters.filter(
    (f) => value[f.key] !== undefined && value[f.key] !== ""
  );
  const availableFilters = filters.filter(
    (f) => value[f.key] === undefined || value[f.key] === ""
  );

  const getDisplayValue = (config: FilterConfig, val: string): string => {
    if (config.type === "boolean") return val === "true" ? "Yes" : "No";
    if (config.type === "date-range") {
      const preset = DATE_PRESETS.find((p) => p.value === val);
      if (preset) return preset.label;
      return val;
    }
    if (config.options) {
      const opt = config.options.find((o) => o.value === val);
      if (opt) return opt.label;
    }
    return val;
  };

  const removeFilter = (key: string) => {
    const next = { ...value };
    delete next[key];
    if (key === "date") {
      delete next["date_from"];
      delete next["date_to"];
    }
    onChange(next);
  };

  const setFilter = (key: string, val: string) => {
    onChange({ ...value, [key]: val });
    setPopoverOpen(false);
    setSelectedType(null);
    setSearch("");
  };

  const openFilterEdit = (key: string) => {
    setSelectedType(key);
    setPopoverOpen(true);
    setSearch("");
  };

  const clearAll = () => onChange({});

  const hasActiveFilters = activeFilters.length > 0;
  const selectedConfig = selectedType ? filters.find((f) => f.key === selectedType) : null;

  return (
    <div className="flex flex-wrap items-center gap-2 rounded-lg border border-border bg-surface px-3 py-2">
      {activeFilters.map((config) => (
        <Chip
          key={config.key}
          label={config.label}
          displayValue={getDisplayValue(config, value[config.key])}
          onRemove={() => removeFilter(config.key)}
          onClick={() => openFilterEdit(config.key)}
        />
      ))}

      {availableFilters.length > 0 && (
        <div className="relative" ref={popoverRef}>
          <button
            type="button"
            onClick={() => { setPopoverOpen(!popoverOpen); setSelectedType(null); setSearch(""); }}
            className="inline-flex items-center gap-1 rounded-full border border-dashed border-border px-2.5 py-1 text-xs text-text-muted transition-colors hover:border-border-strong hover:text-text-secondary"
            style={{ fontFamily: "var(--font-dm-sans)" }}
          >
            <Plus size={12} />
            Add filter
          </button>

          {popoverOpen && (
            <div className="absolute left-0 top-full z-50 mt-1 flex overflow-hidden rounded-lg border border-border bg-surface shadow-lg">
              {!selectedType && (
                <div className="w-44 py-1">
                  <div className="px-3 py-1.5 text-[10px] uppercase tracking-wider text-text-muted">
                    Filter by
                  </div>
                  {availableFilters.map((config) => (
                    <button
                      key={config.key}
                      type="button"
                      onClick={() => {
                        if (config.type === "boolean") {
                          setFilter(config.key, "true");
                        } else {
                          setSelectedType(config.key);
                        }
                      }}
                      className="flex w-full items-center px-3 py-1.5 text-left text-xs text-text-secondary transition-colors hover:bg-surface-secondary"
                      style={{ fontFamily: "var(--font-dm-sans)" }}
                    >
                      {config.label}
                    </button>
                  ))}
                </div>
              )}

              {selectedConfig && selectedConfig.type === "select" && (
                <div className="w-52 py-1">
                  <div className="px-2 pb-1">
                    <div className="flex items-center gap-1.5 rounded border border-border bg-background px-2 py-1">
                      <Search size={12} className="text-text-muted" />
                      <input
                        type="text"
                        value={search}
                        onChange={(e) => setSearch(e.target.value)}
                        placeholder={`Search ${selectedConfig.label.toLowerCase()}...`}
                        className="w-full bg-transparent text-xs text-text-primary outline-none placeholder:text-text-muted"
                        autoFocus
                      />
                    </div>
                  </div>
                  <div className="max-h-48 overflow-y-auto">
                    {(selectedConfig.options ?? [])
                      .filter((opt) => opt.label.toLowerCase().includes(search.toLowerCase()))
                      .map((opt) => {
                        const isActive = value[selectedConfig.key] === opt.value;
                        return (
                          <button
                            key={opt.value}
                            type="button"
                            onClick={() => setFilter(selectedConfig.key, opt.value)}
                            className="flex w-full items-center gap-2 px-3 py-1.5 text-left text-xs transition-colors hover:bg-surface-secondary"
                            style={{ fontFamily: "var(--font-dm-sans)" }}
                          >
                            <span className={isActive ? "text-text-primary font-medium" : "text-text-secondary"}>
                              {opt.label}
                            </span>
                            {isActive && <Check size={12} className="ml-auto text-accent" />}
                          </button>
                        );
                      })}
                  </div>
                </div>
              )}

              {selectedConfig && selectedConfig.type === "date-range" && (
                <div className="w-52 py-1">
                  <div className="px-3 py-1.5 text-[10px] uppercase tracking-wider text-text-muted">
                    Date range
                  </div>
                  {DATE_PRESETS.map((preset) => (
                    <button
                      key={preset.value}
                      type="button"
                      onClick={() => setFilter("date", preset.value)}
                      className="flex w-full items-center px-3 py-1.5 text-left text-xs text-text-secondary transition-colors hover:bg-surface-secondary"
                      style={{ fontFamily: "var(--font-dm-sans)" }}
                    >
                      {preset.label}
                    </button>
                  ))}
                </div>
              )}
            </div>
          )}
        </div>
      )}

      {hasActiveFilters && (
        <button
          type="button"
          onClick={clearAll}
          className="ml-auto text-[11px] text-text-muted transition-colors hover:text-text-secondary"
          style={{ fontFamily: "var(--font-dm-sans)" }}
        >
          Clear all
        </button>
      )}
    </div>
  );
}
