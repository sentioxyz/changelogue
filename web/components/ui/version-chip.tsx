import { cn } from "@/lib/utils";

interface VersionChipProps {
  version: string;
  className?: string;
}

export function VersionChip({ version, className }: VersionChipProps) {
  return (
    <span
      className={cn(
        "inline-flex items-center rounded bg-mono-bg px-1.5 py-0.5 text-[12px] leading-none text-secondary-foreground",
        className
      )}
      style={{ fontFamily: "'JetBrains Mono', monospace" }}
    >
      {version}
    </span>
  );
}
