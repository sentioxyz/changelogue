/**
 * Urgency Pill — shared badge component for semantic release urgency.
 *
 * ## Urgency Levels
 *
 * | Level    | Icon            | Color   | Meaning                                    |
 * |----------|-----------------|---------|--------------------------------------------|
 * | critical | AlertOctagon    | Red     | Breaking changes, security patches          |
 * | high     | AlertTriangle   | Orange  | Significant API changes, deprecations       |
 * | medium   | Circle          | Amber   | Notable changes worth reviewing             |
 * | low      | CheckCircle     | Green   | Routine updates, dependency bumps           |
 *
 * ## Variants
 *
 * - **icon-only** (default): 18×18 circle with just the icon. Best for compact
 *   spaces like the projects page where multiple versions are listed inline.
 *   Urgency label is shown on hover via `title`.
 *
 * - **labeled**: Icon + text label in a rounded pill. Best for table rows
 *   (releases page) where there's dedicated column space.
 *
 * - **text**: Text-only pill without icon. Best for table cells and compact
 *   lists where space is tight but the label should be visible.
 */
import { AlertOctagon, AlertTriangle, Circle, CheckCircle } from "lucide-react";
import type { LucideIcon } from "lucide-react";

interface UrgencyStyle {
  icon: LucideIcon;
  bg: string;
  border: string;
  text: string;
}

export const URGENCY_STYLES: Record<string, UrgencyStyle> = {
  critical: { icon: AlertOctagon, bg: "rgba(220,38,38,0.10)", border: "rgba(220,38,38,0.20)", text: "#dc2626" },
  high:     { icon: AlertTriangle, bg: "rgba(249,115,22,0.10)", border: "rgba(249,115,22,0.20)", text: "#ea580c" },
  medium:   { icon: Circle,        bg: "rgba(245,158,11,0.10)", border: "rgba(245,158,11,0.20)", text: "#d97706" },
  low:      { icon: CheckCircle,   bg: "rgba(34,197,94,0.08)",  border: "rgba(34,197,94,0.18)",  text: "#16a34a" },
};

interface UrgencyPillProps {
  urgency: string;
  /** "icon-only" = compact circle, "labeled" = icon + text pill, "text" = text-only pill */
  variant?: "icon-only" | "labeled" | "text";
  className?: string;
}

export function UrgencyPill({ urgency, variant = "icon-only", className }: UrgencyPillProps) {
  const style = URGENCY_STYLES[urgency.toLowerCase()];
  if (!style) return null;

  const Icon = style.icon;

  if (variant === "icon-only") {
    return (
      <span
        className={`inline-flex items-center justify-center rounded-full ${className ?? ""}`}
        style={{ backgroundColor: style.bg, border: `1px solid ${style.border}`, color: style.text, width: 18, height: 18 }}
        title={urgency}
      >
        <Icon size={10} />
      </span>
    );
  }

  if (variant === "text") {
    return (
      <span
        className={`inline-flex items-center rounded-full px-2 py-0.5 text-[11px] font-medium leading-none ${className ?? ""}`}
        style={{ backgroundColor: style.bg, border: `1px solid ${style.border}`, color: style.text, fontFamily: "var(--font-dm-sans)" }}
        title={urgency}
      >
        {urgency}
      </span>
    );
  }

  return (
    <span
      className={`inline-flex items-center gap-0.5 rounded-full px-2 py-0.5 text-[10px] font-semibold ${className ?? ""}`}
      style={{ backgroundColor: style.bg, border: `1px solid ${style.border}`, color: style.text, fontFamily: "var(--font-dm-sans)" }}
      title={urgency}
    >
      <Icon size={10} /> {urgency}
    </span>
  );
}
