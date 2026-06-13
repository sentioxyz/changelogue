"use client";

import { useTheme } from "next-themes";
import { useEffect, useState } from "react";
import { Github, Moon, Monitor, RefreshCw, Sun } from "lucide-react";
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import { useTranslation } from "@/lib/i18n/context";
import { cn } from "@/lib/utils";
import { githubApp } from "@/lib/api/client";
import type { GitHubAppStatus } from "@/lib/api/types";
import { Button } from "@/components/ui/button";

interface SettingsDialogProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
}

export function SettingsDialog({ open, onOpenChange }: SettingsDialogProps) {
  const { theme, setTheme } = useTheme();
  const { t, locale, setLocale } = useTranslation();
  const [githubStatus, setGithubStatus] = useState<GitHubAppStatus | null>(null);
  const [syncing, setSyncing] = useState(false);
  const [githubError, setGithubError] = useState("");

  useEffect(() => {
    if (!open) return;
    githubApp.status().then((res) => setGithubStatus(res.data)).catch((err) => setGithubError(err instanceof Error ? err.message : "Failed to load GitHub App status"));
  }, [open]);

  const syncGitHub = async () => {
    setGithubError("");
    setSyncing(true);
    try {
      await githubApp.sync();
      const status = await githubApp.status();
      setGithubStatus(status.data);
    } catch (err) {
      setGithubError(err instanceof Error ? err.message : "Failed to sync GitHub App");
    } finally {
      setSyncing(false);
    }
  };

  const themeOptions = [
    { value: "light", label: t("settings.theme.light"), icon: Sun },
    { value: "dark", label: t("settings.theme.dark"), icon: Moon },
    { value: "system", label: t("settings.theme.system"), icon: Monitor },
  ] as const;
  const installationCount = githubStatus?.installations?.length ?? 0;

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="sm:max-w-[520px]">
        <DialogHeader>
          <DialogTitle>{t("settings.title")}</DialogTitle>
        </DialogHeader>

        <div className="space-y-6 pt-2">
          {/* Theme */}
          <div className="space-y-2">
            <label className="text-sm font-medium">{t("settings.theme")}</label>
            <div className="flex gap-2">
              {themeOptions.map((opt) => (
                <button
                  key={opt.value}
                  onClick={() => setTheme(opt.value)}
                  className={cn(
                    "flex flex-1 items-center justify-center gap-2 rounded-md border px-3 py-2 text-sm transition-colors",
                    theme === opt.value
                      ? "border-[var(--beacon-accent)] bg-[var(--beacon-accent)]/10 text-[var(--beacon-accent)]"
                      : "border-border hover:bg-accent"
                  )}
                >
                  <opt.icon className="h-4 w-4" />
                  {opt.label}
                </button>
              ))}
            </div>
          </div>

          {/* Language */}
          <div className="space-y-2">
            <label className="text-sm font-medium">{t("settings.language")}</label>
            <Select value={locale} onValueChange={(v) => setLocale(v as "en" | "zh")}>
              <SelectTrigger className="w-full">
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                <SelectItem value="en">{t("settings.language.en")}</SelectItem>
                <SelectItem value="zh">{t("settings.language.zh")}</SelectItem>
              </SelectContent>
            </Select>
          </div>

          <div className="space-y-3 border-t pt-4">
            <div className="flex items-center justify-between gap-3">
              <div className="flex items-center gap-2">
                <Github className="h-4 w-4" />
                <label className="text-sm font-medium">GitHub App</label>
              </div>
              <Button type="button" size="sm" variant="outline" onClick={syncGitHub} disabled={syncing || !githubStatus?.configured}>
                <RefreshCw className={cn("mr-2 h-3.5 w-3.5", syncing && "animate-spin")} />
                Sync
              </Button>
            </div>
            {githubError && <div className="rounded-md bg-red-50 p-2 text-xs text-red-700">{githubError}</div>}
            <div className="rounded-md border p-3 text-sm">
              <div className="flex items-center justify-between">
                <span className="text-muted-foreground">Status</span>
                <span className={githubStatus?.configured ? "text-green-700" : "text-muted-foreground"}>
                  {githubStatus?.configured ? "Configured" : "Not configured"}
                </span>
              </div>
              {githubStatus?.app_id && (
                <div className="mt-2 flex items-center justify-between">
                  <span className="text-muted-foreground">App ID</span>
                  <span className="font-mono text-xs">{githubStatus.app_id}</span>
                </div>
              )}
              <div className="mt-2 flex items-center justify-between">
                <span className="text-muted-foreground">Installations</span>
                <span>{installationCount}</span>
              </div>
            </div>
            {githubStatus?.install_url && (
              <a className="inline-flex text-sm text-[var(--beacon-accent)] hover:underline" href={githubStatus.install_url} target="_blank" rel="noreferrer">
                Install GitHub App
              </a>
            )}
          </div>
        </div>
      </DialogContent>
    </Dialog>
  );
}
