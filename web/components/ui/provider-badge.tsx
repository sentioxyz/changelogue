import { cn } from "@/lib/utils";

const PROVIDER_STYLES: Record<string, { bg: string; text: string; label: string }> = {
  github: { bg: "#1a1a1a", text: "#ffffff", label: "GitHub" },
  dockerhub: { bg: "#2496ed", text: "#ffffff", label: "Docker Hub" },
};

interface ProviderBadgeProps {
  provider: string;
  className?: string;
}

export function ProviderBadge({ provider, className }: ProviderBadgeProps) {
  const style = PROVIDER_STYLES[provider.toLowerCase()] ?? {
    bg: "#6b7280",
    text: "#ffffff",
    label: provider,
  };
  return (
    <span
      className={cn("inline-flex items-center rounded-full px-2 py-0.5 text-[11px] font-medium leading-none", className)}
      style={{ backgroundColor: style.bg, color: style.text }}
    >
      {style.label}
    </span>
  );
}
