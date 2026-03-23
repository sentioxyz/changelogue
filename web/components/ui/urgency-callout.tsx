"use client";

import { AlertTriangle, AlertCircle } from "lucide-react";
import { cn } from "@/lib/utils";
import { useTranslation } from "@/lib/i18n/context";

interface UrgencyCalloutProps {
  urgency: string;
  description?: string;
  className?: string;
}

export function UrgencyCallout({ urgency, description, className }: UrgencyCalloutProps) {
  const { t } = useTranslation();
  const upper = urgency?.toUpperCase();
  if (upper !== "HIGH" && upper !== "CRITICAL") return null;

  const isCritical = upper === "CRITICAL";
  const bg = isCritical ? "var(--destructive-bg, #fff1f2)" : "var(--warning-bg, #fff8f0)";
  const border = isCritical ? "var(--color-destructive, #dc2626)" : "var(--status-pending, #d97706)";
  const Icon = isCritical ? AlertCircle : AlertTriangle;

  const urgencyLabel = isCritical ? t("urgency.critical") : t("urgency.high");

  return (
    <div
      className={cn("rounded px-4 py-3 text-sm", className)}
      style={{ backgroundColor: bg, borderLeft: `3px solid ${border}` }}
    >
      <div className="flex items-start gap-2">
        <Icon className="h-4 w-4 mt-0.5 shrink-0" style={{ color: border }} />
        <div>
          <span className="font-semibold" style={{ color: border }}>
            {urgencyLabel}
          </span>
          {description && (
            <p className="mt-0.5 text-secondary-foreground">{description}</p>
          )}
        </div>
      </div>
    </div>
  );
}
