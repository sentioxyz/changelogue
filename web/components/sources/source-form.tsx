"use client";

import { useState } from "react";
import { useRouter } from "next/navigation";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Switch } from "@/components/ui/switch";
import { Textarea } from "@/components/ui/textarea";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import type { Source, SourceInput } from "@/lib/api/types";
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
