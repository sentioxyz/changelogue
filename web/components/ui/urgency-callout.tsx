import { AlertTriangle, AlertCircle } from "lucide-react";
import { cn } from "@/lib/utils";

interface UrgencyCalloutProps {
  urgency: string;
  description?: string;
  className?: string;
}

export function UrgencyCallout({ urgency, description, className }: UrgencyCalloutProps) {
  const upper = urgency?.toUpperCase();
  if (upper !== "HIGH" && upper !== "CRITICAL") return null;

  const isCritical = upper === "CRITICAL";
  const bg = isCritical ? "#fff1f2" : "#fff8f0";
  const border = isCritical ? "#dc2626" : "#d97706";
  const Icon = isCritical ? AlertCircle : AlertTriangle;

  return (
    <div
      className={cn("rounded px-4 py-3 text-sm", className)}
      style={{ backgroundColor: bg, borderLeft: `3px solid ${border}` }}
    >
      <div className="flex items-start gap-2">
        <Icon className="h-4 w-4 mt-0.5 shrink-0" style={{ color: border }} />
        <div>
          <span className="font-semibold" style={{ color: border }}>
            {upper} URGENCY
          </span>
          {description && (
            <p className="mt-0.5 text-[#374151]">{description}</p>
          )}
        </div>
      </div>
    </div>
  );
}
