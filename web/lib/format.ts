export function timeAgo(dateStr?: string | null): string {
  if (!dateStr) return "\u2014";
  const diff = Date.now() - new Date(dateStr).getTime();
  const mins = Math.floor(diff / 60000);
  if (mins < 60) return `${mins}m ago`;
  const hrs = Math.floor(mins / 60);
  if (hrs < 24) return `${hrs}h ago`;
  return `${Math.floor(hrs / 24)}d ago`;
}

export function validateRepository(provider: string, repo: string): string | null {
  const trimmed = repo.trim();
  if (!trimmed) return null;
  if (provider === "github" && /^(https?:\/\/)?github\.com\//i.test(trimmed)) {
    return "Use owner/repo format (e.g. ethereum/go-ethereum), not a full URL";
  }
  if (provider === "dockerhub" && /^(https?:\/\/)?hub\.docker\.com\//i.test(trimmed)) {
    return "Use owner/image format (e.g. library/nginx), not a full URL";
  }
  if (/^https?:\/\//.test(trimmed)) {
    return "Use owner/repo format, not a full URL";
  }
  return null;
}

export function formatInterval(seconds: number): string {
  if (seconds < 60) return `${seconds}s`;
  if (seconds < 3600) return `${Math.round(seconds / 60)}m`;
  if (seconds < 86400) return `${(seconds / 3600).toFixed(1)}h`;
  return `${(seconds / 86400).toFixed(1)}d`;
}
