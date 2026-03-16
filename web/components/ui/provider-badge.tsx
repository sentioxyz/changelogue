import { cn } from "@/lib/utils";
import { FaGithub, FaDocker, FaAws, FaGitlab, FaPython, FaNpm } from "react-icons/fa";
import type { IconType } from "react-icons";

const PROVIDER_STYLES: Record<string, { bg: string; text: string; label: string; icon: IconType }> = {
  github: { bg: "#1a1a1a", text: "#ffffff", label: "GitHub", icon: FaGithub },
  dockerhub: { bg: "#2496ed", text: "#ffffff", label: "Docker Hub", icon: FaDocker },
  "ecr-public": { bg: "#ff9900", text: "#ffffff", label: "ECR Public", icon: FaAws },
  gitlab: { bg: "#fc6d26", text: "#ffffff", label: "GitLab", icon: FaGitlab },
  pypi: { bg: "#006DAD", text: "#ffffff", label: "PyPI", icon: FaPython },
  npm: { bg: "#CB3837", text: "#ffffff", label: "npm", icon: FaNpm },
};

export function getProviderIcon(provider: string): IconType | undefined {
  return PROVIDER_STYLES[provider.toLowerCase()]?.icon;
}

interface ProviderBadgeProps {
  provider: string;
  className?: string;
}

export function ProviderBadge({ provider, className }: ProviderBadgeProps) {
  const style = PROVIDER_STYLES[provider.toLowerCase()];
  const resolved = style ?? {
    bg: "#6b7280",
    text: "#ffffff",
    label: provider,
  };
  const Icon = style?.icon;
  return (
    <span
      className={cn("inline-flex items-center gap-1 rounded-full px-2 py-0.5 text-[11px] font-medium leading-none", className)}
      style={{ backgroundColor: resolved.bg, color: resolved.text }}
    >
      {Icon && <Icon size={11} />}
      {resolved.label}
    </span>
  );
}
