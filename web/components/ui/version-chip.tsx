import { cn } from "@/lib/utils";

interface VersionChipProps {
  version: string;
  className?: string;
}

export function VersionChip({ version, className }: VersionChipProps) {
  return (
    <span
      className={cn(
        "inline-flex items-center rounded px-1.5 py-0.5 text-[12px] leading-none text-[#374151]",
        className
      )}
      style={{ backgroundColor: "#f3f3f1", fontFamily: "'JetBrains Mono', monospace" }}
    >
      {version}
    </span>
  );
}
