import { cn } from "@/lib/utils";

interface SectionLabelProps {
  children: React.ReactNode;
  className?: string;
}

export function SectionLabel({ children, className }: SectionLabelProps) {
  return (
    <p
      className={cn("text-[11px] font-medium uppercase tracking-[0.12em] text-text-muted", className)}
    >
      {children}
    </p>
  );
}
