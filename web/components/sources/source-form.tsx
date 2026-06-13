"use client";

import { useEffect, useState } from "react";
import { useRouter } from "next/navigation";
import { Github, Lock, RefreshCw } from "lucide-react";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Switch } from "@/components/ui/switch";
import { Textarea } from "@/components/ui/textarea";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import type { GitHubAppRepository, Source, SourceInput } from "@/lib/api/types";
import { githubApp } from "@/lib/api/client";
import { validateRepository } from "@/lib/format";
import { useTranslation } from "@/lib/i18n/context";

interface SourceFormProps {
  initial?: Source;
  projectId?: string;
  onSubmit: (input: SourceInput) => Promise<void>;
  title: string;
  redirectTo?: string;
  onSuccess?: () => void;
  onCancel?: () => void;
}


const POLL_INTERVAL_KEYS = [
    { labelKey: "sourceForm.intervalHourly", value: "3600" },
    { labelKey: "sourceForm.intervalDaily", value: "86400" },
    { labelKey: "sourceForm.intervalWeekly", value: "604800" },
    { labelKey: "sourceForm.intervalMonthly", value: "2592000" },
  ];

  function nearestPollInterval(seconds: number): string {
    const values = POLL_INTERVAL_KEYS.map((o) => Number(o.value));
    let closest = values[0];
    for (const v of values) {
      if (Math.abs(v - seconds) < Math.abs(closest - seconds)) closest = v;
    }
    return String(closest);
  }


export function SourceForm({ initial, onSubmit, title, redirectTo, onSuccess, onCancel }: SourceFormProps) {
  const router = useRouter();
  const { t } = useTranslation();
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState("");
  const [provider, setProvider] = useState(initial?.provider ?? "dockerhub");
  const [repository, setRepository] = useState(initial?.repository ?? "");
  const [pollInterval, setPollInterval] = useState(
    nearestPollInterval(initial?.poll_interval_seconds ?? 86400)
  );
  const [enabled, setEnabled] = useState(initial?.enabled ?? true);
  const [configJson, setConfigJson] = useState(
    JSON.stringify(initial?.config ?? {}, null, 2)
  );
  const [versionFilterInclude, setVersionFilterInclude] = useState(initial?.version_filter_include ?? "");
  const [versionFilterExclude, setVersionFilterExclude] = useState(initial?.version_filter_exclude ?? "");
  const [excludePrereleases, setExcludePrereleases] = useState(initial?.exclude_prereleases ?? false);
  const [githubRepos, setGithubRepos] = useState<GitHubAppRepository[]>([]);
  const [githubConfigured, setGithubConfigured] = useState(false);
  const [loadingGithubRepos, setLoadingGithubRepos] = useState(false);

  useEffect(() => {
    if (provider !== "github") return;
    let cancelled = false;
    setLoadingGithubRepos(true);
    Promise.all([githubApp.status(), githubApp.repositories()])
      .then(([status, repos]) => {
        if (cancelled) return;
        setGithubConfigured(status.data.configured);
        setGithubRepos(repos.data);
      })
      .catch(() => {
        if (cancelled) return;
        setGithubConfigured(false);
        setGithubRepos([]);
      })
      .finally(() => {
        if (!cancelled) setLoadingGithubRepos(false);
      });
    return () => {
      cancelled = true;
    };
  }, [provider]);

  const syncGithubRepos = async () => {
    setLoadingGithubRepos(true);
    try {
      await githubApp.sync();
      const repos = await githubApp.repositories();
      setGithubRepos(repos.data);
      setGithubConfigured(true);
    } catch (err) {
      setError(err instanceof Error ? err.message : "Failed to sync GitHub repositories");
    } finally {
      setLoadingGithubRepos(false);
    }
  };

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    setError("");

    const repoError = validateRepository(provider, repository);
    if (repoError) {
      setError(repoError);
      return;
    }

    let parsedConfig: Record<string, unknown> | undefined;
    if (configJson.trim() && configJson.trim() !== "{}") {
      try {
        parsedConfig = JSON.parse(configJson);
      } catch {
        setError(t("sourceForm.errorInvalidJson"));
        return;
      }
    }

    setSaving(true);
    try {
      await onSubmit({
        provider,
        repository: repository.trim(),
        poll_interval_seconds: Number(pollInterval),
        enabled,
        config: parsedConfig,
        version_filter_include: versionFilterInclude.trim() || undefined,
        version_filter_exclude: versionFilterExclude.trim() || undefined,
        exclude_prereleases: excludePrereleases || undefined,
      });
      if (onSuccess) {
        onSuccess();
      } else {
        router.push(redirectTo ?? "/sources");
      }
    } catch (err: unknown) {
      setError(err instanceof Error ? err.message : t("sourceForm.errorFailedToSave"));
    } finally {
      setSaving(false);
    }
  };

  const handleCancel = () => {
    if (onCancel) {
      onCancel();
    } else {
      router.back();
    }
  };

  const formContent = (
    <form onSubmit={handleSubmit} className="space-y-4">
      {error && <div className="rounded-md bg-red-50 p-3 text-sm text-red-700">{error}</div>}
      <div className="space-y-2">
        <Label>{t("sourceForm.provider")}</Label>
        <Select value={provider} onValueChange={setProvider}>
          <SelectTrigger><SelectValue /></SelectTrigger>
          <SelectContent>
            <SelectItem value="dockerhub">{t("sourceForm.providerDockerHub")}</SelectItem>
            <SelectItem value="github">{t("sourceForm.providerGitHub")}</SelectItem>
            <SelectItem value="ecr-public">{t("sourceForm.providerECR")}</SelectItem>
            <SelectItem value="gitlab">{t("sourceForm.providerGitLab")}</SelectItem>
            <SelectItem value="pypi">{t("sourceForm.providerPyPI")}</SelectItem>
            <SelectItem value="npm">{t("sourceForm.providerNpm")}</SelectItem>
          </SelectContent>
        </Select>
      </div>
      <div className="space-y-2">
        <Label htmlFor="repository">{t("sourceForm.repository")}</Label>
        <Input id="repository" value={repository} onChange={(e) => setRepository(e.target.value)} placeholder={t("sourceForm.repositoryPlaceholder")} required />
      </div>
      {provider === "github" && (
        <div className="space-y-2 rounded-md border p-3">
          <div className="flex items-center justify-between gap-3">
            <div className="flex items-center gap-2 text-sm font-medium">
              <Github className="h-4 w-4" />
              {t("sourceForm.githubAuthorizedRepos")}
            </div>
            <Button type="button" size="sm" variant="outline" onClick={syncGithubRepos} disabled={loadingGithubRepos || !githubConfigured}>
              <RefreshCw className={`mr-2 h-3.5 w-3.5 ${loadingGithubRepos ? "animate-spin" : ""}`} />
              {t("sourceForm.githubSync")}
            </Button>
          </div>
          {githubRepos.length > 0 ? (
            <Select value={githubRepos.some((repo) => repo.full_name === repository) ? repository : ""} onValueChange={setRepository}>
              <SelectTrigger><SelectValue placeholder={t("sourceForm.githubSelectRepo")} /></SelectTrigger>
              <SelectContent>
                {githubRepos.map((repo) => (
                  <SelectItem key={`${repo.installation_id}:${repo.full_name}`} value={repo.full_name}>
                    {repo.private ? "[private] " : ""}{repo.full_name}
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
          ) : (
            <div className="flex items-center gap-2 text-xs text-muted-foreground">
              <Lock className="h-3.5 w-3.5" />
              {githubConfigured ? t("sourceForm.githubNoRepos") : t("sourceForm.githubNotConfigured")}
            </div>
          )}
        </div>
      )}
      <div className="space-y-2">
        <Label>{t("sourceForm.pollInterval")}</Label>
        <Select value={pollInterval} onValueChange={setPollInterval}>
          <SelectTrigger><SelectValue /></SelectTrigger>
          <SelectContent>
            {POLL_INTERVAL_KEYS.map((opt) => (
              <SelectItem key={opt.value} value={opt.value}>{t(opt.labelKey)}</SelectItem>
            ))}
          </SelectContent>
        </Select>
      </div>
      <div className="space-y-2">
        <Label htmlFor="config">{t("sourceForm.config")}</Label>
        <Textarea
          id="config"
          value={configJson}
          onChange={(e) => setConfigJson(e.target.value)}
          rows={4}
          className="font-mono text-sm"
          placeholder="{}"
        />
      </div>
      <div className="space-y-2">
        <Label htmlFor="version_filter_include">{t("sourceForm.versionFilterInclude")}</Label>
        <Input
          id="version_filter_include"
          value={versionFilterInclude}
          onChange={(e) => setVersionFilterInclude(e.target.value)}
          placeholder={t("sourceForm.versionFilterIncludePlaceholder")}
          className="font-mono text-sm"
        />
        <p className="text-xs text-muted-foreground">{t("sourceForm.versionFilterIncludeHelper")}</p>
      </div>
      <div className="space-y-2">
        <Label htmlFor="version_filter_exclude">{t("sourceForm.versionFilterExclude")}</Label>
        <Input
          id="version_filter_exclude"
          value={versionFilterExclude}
          onChange={(e) => setVersionFilterExclude(e.target.value)}
          placeholder={t("sourceForm.versionFilterExcludePlaceholder")}
          className="font-mono text-sm"
        />
        <p className="text-xs text-muted-foreground">{t("sourceForm.versionFilterExcludeHelper")}</p>
      </div>
      {(provider === "github" || provider === "gitlab" || provider === "pypi" || provider === "npm") && (
        <div className="flex items-center gap-3">
          <Switch checked={excludePrereleases} onCheckedChange={setExcludePrereleases} />
          <Label>{t("sourceForm.excludePrereleases")}</Label>
        </div>
      )}
      <div className="flex items-center gap-3">
        <Switch checked={enabled} onCheckedChange={setEnabled} />
        <Label>{t("sourceForm.enabled")}</Label>
      </div>
      <div className="flex justify-end gap-2">
        <Button type="button" variant="outline" onClick={handleCancel}>{t("sourceForm.cancel")}</Button>
        <Button type="submit" disabled={saving}>{saving ? t("sourceForm.saving") : t("sourceForm.save")}</Button>
      </div>
    </form>
  );

  if (onSuccess) {
    return formContent;
  }

  return (
    <Card className="mx-auto max-w-2xl">
      <CardHeader><CardTitle>{title}</CardTitle></CardHeader>
      <CardContent>
        {formContent}
      </CardContent>
    </Card>
  );
}
