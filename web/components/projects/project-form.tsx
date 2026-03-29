"use client";

import { useState } from "react";
import { useRouter } from "next/navigation";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Textarea } from "@/components/ui/textarea";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import type { Project, ProjectInput, SourceInput } from "@/lib/api/types";
import { validateRepository } from "@/lib/format";
import { Plus, X } from "lucide-react";
import { Checkbox } from "@/components/ui/checkbox";
import { useTranslation } from "@/lib/i18n/context";

export interface ProjectFormResult {
  project: ProjectInput;
  source?: SourceInput;
}

interface ProjectFormProps {
  initial?: Project;
  onSubmit: (result: ProjectFormResult) => Promise<void>;
  title: string;
  /** Hide the source section (used in edit mode) */
  hideSource?: boolean;
  /** Dialog mode: called after successful submit instead of router.push */
  onSuccess?: () => void;
  /** Dialog mode: called on cancel instead of router.back */
  onCancel?: () => void;
}

export function ProjectForm({ initial, onSubmit, title, hideSource, onSuccess, onCancel }: ProjectFormProps) {
  const router = useRouter();
  const { t } = useTranslation();
  const [saving, setSaving] = useState(false);
  const [name, setName] = useState(initial?.name ?? "");
  const [description, setDescription] = useState(initial?.description ?? "");
  const [error, setError] = useState("");

  /* Source fields (only for create) */
  const [showSource, setShowSource] = useState(false);
  const [provider, setProvider] = useState("github");
  const [repository, setRepository] = useState("");
  const [pollInterval, setPollInterval] = useState("86400");
  const [versionFilterInclude, setVersionFilterInclude] = useState("");
  const [versionFilterExclude, setVersionFilterExclude] = useState("");
  const [excludePrereleases, setExcludePrereleases] = useState(false);

  const handleCancel = () => {
    if (onCancel) {
      onCancel();
    } else {
      router.back();
    }
  };

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    setError("");
    setSaving(true);
    try {
      const result: ProjectFormResult = {
        project: {
          name,
          description: description || undefined,
        },
      };
      if (showSource && repository.trim()) {
        const repoError = validateRepository(provider, repository.trim());
        if (repoError) {
          setError(repoError);
          setSaving(false);
          return;
        }
        result.source = {
          provider,
          repository: repository.trim(),
          poll_interval_seconds: Number(pollInterval) || 86400,
          enabled: true,
          version_filter_include: versionFilterInclude.trim() || undefined,
          version_filter_exclude: versionFilterExclude.trim() || undefined,
          exclude_prereleases: excludePrereleases || undefined,
        };
      }
      await onSubmit(result);
      if (onSuccess) {
        onSuccess();
      } else {
        router.push("/projects");
      }
    } catch (err: unknown) {
      setError(err instanceof Error ? err.message : t("projectForm.errorFailedToSave"));
    } finally {
      setSaving(false);
    }
  };

  const formContent = (
    <form onSubmit={handleSubmit} className="space-y-4">
      {error && <div className="rounded-md bg-red-50 p-3 text-sm text-red-700">{error}</div>}
      <div className="space-y-2">
        <Label htmlFor="name">{t("projectForm.name")}</Label>
        <Input id="name" value={name} onChange={(e) => setName(e.target.value)} required />
      </div>
      <div className="space-y-2">
        <Label htmlFor="description">{t("projectForm.description")}</Label>
        <Textarea id="description" value={description} onChange={(e) => setDescription(e.target.value)} rows={2} />
      </div>

      {/* Optional source section — only in create mode */}
      {!hideSource && (
        <div className="space-y-3">
          {!showSource ? (
            <button
              type="button"
              onClick={() => setShowSource(true)}
              className="inline-flex items-center gap-1.5 text-[13px] font-medium transition-colors hover:opacity-80"
              style={{ color: "var(--beacon-accent)" }}
            >
              <Plus className="h-3.5 w-3.5" />
              {t("projectForm.addSource")}
            </button>
          ) : (
            <div className="rounded-md border p-4 space-y-3" style={{ borderColor: "var(--border)" }}>
              <div className="flex items-center justify-between">
                <Label className="text-[13px] font-medium">{t("projectForm.addSource")}</Label>
                <button
                  type="button"
                  onClick={() => { setShowSource(false); setRepository(""); }}
                  className="text-text-muted hover:text-text-secondary"
                >
                  <X className="h-4 w-4" />
                </button>
              </div>
              <div className="space-y-2">
                <Label>{t("projectForm.provider")}</Label>
                <Select value={provider} onValueChange={setProvider}>
                  <SelectTrigger><SelectValue /></SelectTrigger>
                  <SelectContent>
                    <SelectItem value="github">{t("projectForm.providerGitHub")}</SelectItem>
                    <SelectItem value="dockerhub">{t("projectForm.providerDockerHub")}</SelectItem>
                    <SelectItem value="ecr-public">{t("projectForm.providerECR")}</SelectItem>
                    <SelectItem value="gitlab">{t("projectForm.providerGitLab")}</SelectItem>
                    <SelectItem value="pypi">{t("projectForm.providerPyPI")}</SelectItem>
                    <SelectItem value="npm">{t("projectForm.providerNpm")}</SelectItem>
                  </SelectContent>
                </Select>
              </div>
              <div className="space-y-2">
                <Label htmlFor="repository">{t("projectForm.repository")}</Label>
                <Input
                  id="repository"
                  value={repository}
                  onChange={(e) => setRepository(e.target.value)}
                  placeholder={t("projectForm.repositoryPlaceholder")}
                />
              </div>
              <div className="space-y-2">
                <Label>{t("projectForm.pollInterval")}</Label>
                <Select value={pollInterval} onValueChange={setPollInterval}>
                  <SelectTrigger><SelectValue /></SelectTrigger>
                  <SelectContent>
                    <SelectItem value="3600">{t("projectForm.intervalHourly")}</SelectItem>
                    <SelectItem value="86400">{t("projectForm.intervalDaily")}</SelectItem>
                    <SelectItem value="604800">{t("projectForm.intervalWeekly")}</SelectItem>
                    <SelectItem value="2592000">{t("projectForm.intervalMonthly")}</SelectItem>
                  </SelectContent>
                </Select>
              </div>
              <div className="space-y-2">
                <Label htmlFor="version_filter_include">{t("projectForm.versionFilterInclude")}</Label>
                <Input
                  id="version_filter_include"
                  value={versionFilterInclude}
                  onChange={(e) => setVersionFilterInclude(e.target.value)}
                  placeholder={t("projectForm.versionFilterIncludePlaceholder")}
                  className="font-mono text-sm"
                />
                <p className="text-xs text-muted-foreground">{t("projectForm.versionFilterIncludeHelper")}</p>
              </div>
              <div className="space-y-2">
                <Label htmlFor="version_filter_exclude">{t("projectForm.versionFilterExclude")}</Label>
                <Input
                  id="version_filter_exclude"
                  value={versionFilterExclude}
                  onChange={(e) => setVersionFilterExclude(e.target.value)}
                  placeholder={t("projectForm.versionFilterExcludePlaceholder")}
                  className="font-mono text-sm"
                />
                <p className="text-xs text-muted-foreground">{t("projectForm.versionFilterExcludeHelper")}</p>
              </div>
              {(provider === "github" || provider === "gitlab" || provider === "pypi" || provider === "npm") && (
                <label className="flex items-center gap-2 text-sm text-muted-foreground">
                  <Checkbox
                    checked={excludePrereleases}
                    onCheckedChange={(checked) => setExcludePrereleases(!!checked)}
                  />
                  {t("projectForm.excludePrereleases")}
                </label>
              )}
            </div>
          )}
        </div>
      )}

      <div className="flex justify-end gap-2">
        <Button type="button" variant="outline" onClick={handleCancel}>{t("projectForm.cancel")}</Button>
        <Button type="submit" disabled={saving}>{saving ? t("projectForm.saving") : t("projectForm.save")}</Button>
      </div>
    </form>
  );

  // Dialog mode: return form content without Card wrapper
  if (onSuccess) {
    return formContent;
  }

  // Page mode: wrap in Card
  return (
    <Card className="mx-auto max-w-2xl">
      <CardHeader>
        <CardTitle>{title}</CardTitle>
      </CardHeader>
      <CardContent>
        {formContent}
      </CardContent>
    </Card>
  );
}
