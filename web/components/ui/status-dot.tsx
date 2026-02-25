import { cn } from "@/lib/utils";

const STATUS_COLORS: Record<string, string> = {
  completed: "#16a34a",
  running: "#2563eb",
  pending: "#d97706",
  failed: "#dc2626",
};

interface StatusDotProps {
  status: string;
  className?: string;
}

export function StatusDot({ status, className }: StatusDotProps) {
  const color = STATUS_COLORS[status.toLowerCase()] ?? "#6b7280";
  return (
    <span
      className={cn("inline-block h-2 w-2 rounded-full shrink-0", className)}
      style={{ backgroundColor: color }}
      title={status}
    />
  );
}
