"use client";

import { useState } from "react";
import { useRouter } from "next/navigation";
import { contextSources as ctxApi } from "@/lib/api/client";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Textarea } from "@/components/ui/textarea";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import { useTranslation } from "@/lib/i18n/context";

interface NewContextSourceFormProps {
  projectId: string;
  onSuccess?: () => void;
  onCancel?: () => void;
}

export function NewContextSourceForm({ projectId, onSuccess, onCancel }: NewContextSourceFormProps) {
  const router = useRouter();
  const { t } = useTranslation();
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState("");
  const [type, setType] = useState("documentation");
  const [name, setName] = useState("");
  const [configJson, setConfigJson] = useState("{}");

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    setError("");

    let parsedConfig: Record<string, unknown>;
    try {
      parsedConfig = JSON.parse(configJson);
    } catch {
      setError(t("contextSourceForm.invalidJson"));
      return;
    }

    setSaving(true);
    try {
      await ctxApi.create(projectId, { type, name, config: parsedConfig });
      if (onSuccess) {
        onSuccess();
      } else {
        router.push(`/projects/${projectId}`);
      }
    } catch (err: unknown) {
      setError(err instanceof Error ? err.message : t("contextSourceForm.failedToSave"));
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
        <Label htmlFor="name">{t("contextSourceForm.labelName")}</Label>
        <Input id="name" value={name} onChange={(e) => setName(e.target.value)} placeholder={t("contextSourceForm.placeholderName")} required />
      </div>
      <div className="space-y-2">
        <Label>{t("contextSourceForm.labelType")}</Label>
        <Select value={type} onValueChange={setType}>
          <SelectTrigger><SelectValue /></SelectTrigger>
          <SelectContent>
            <SelectItem value="documentation">{t("contextSourceForm.typeDocumentation")}</SelectItem>
            <SelectItem value="changelog">{t("contextSourceForm.typeChangelog")}</SelectItem>
            <SelectItem value="github_issues">{t("contextSourceForm.typeGithubIssues")}</SelectItem>
            <SelectItem value="custom">{t("contextSourceForm.typeCustom")}</SelectItem>
          </SelectContent>
        </Select>
      </div>
      <div className="space-y-2">
        <Label htmlFor="config">{t("contextSourceForm.labelConfig")}</Label>
        <Textarea
          id="config"
          value={configJson}
          onChange={(e) => setConfigJson(e.target.value)}
          rows={6}
          className="font-mono text-sm"
          placeholder={t("contextSourceForm.placeholderConfig")}
        />
      </div>
      <div className="flex justify-end gap-2">
        <Button type="button" variant="outline" onClick={handleCancel}>{t("contextSourceForm.cancel")}</Button>
        <Button type="submit" disabled={saving}>{saving ? t("contextSourceForm.saving") : t("contextSourceForm.save")}</Button>
      </div>
    </form>
  );

  if (onSuccess) {
    return formContent;
  }

  return (
    <Card className="mx-auto max-w-2xl">
      <CardHeader><CardTitle>{t("contextSourceForm.cardTitle")}</CardTitle></CardHeader>
      <CardContent>
        {formContent}
      </CardContent>
    </Card>
  );
}
