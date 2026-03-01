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

interface SourceFormProps {
  initial?: Source;
  projectId?: string;
  onSubmit: (input: SourceInput) => Promise<void>;
  title: string;
  redirectTo?: string;
  onSuccess?: () => void;
  onCancel?: () => void;
}


export function SourceForm({ initial, onSubmit, title, redirectTo, onSuccess, onCancel }: SourceFormProps) {
  const router = useRouter();
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState("");
  const [provider, setProvider] = useState(initial?.provider ?? "dockerhub");
  const [repository, setRepository] = useState(initial?.repository ?? "");
  const [pollInterval, setPollInterval] = useState(String(initial?.poll_interval_seconds ?? 86400));
  const [enabled, setEnabled] = useState(initial?.enabled ?? true);
  const [configJson, setConfigJson] = useState(
    JSON.stringify(initial?.config ?? {}, null, 2)
  );
  const [versionFilterInclude, setVersionFilterInclude] = useState(initial?.version_filter_include ?? "");
  const [versionFilterExclude, setVersionFilterExclude] = useState(initial?.version_filter_exclude ?? "");

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
        setError("Config must be valid JSON");
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
      });
      if (onSuccess) {
        onSuccess();
      } else {
        router.push(redirectTo ?? "/sources");
      }
    } catch (err: unknown) {
      setError(err instanceof Error ? err.message : "Failed to save");
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
        <Label>Provider</Label>
        <Select value={provider} onValueChange={setProvider}>
          <SelectTrigger><SelectValue /></SelectTrigger>
          <SelectContent>
            <SelectItem value="dockerhub">Docker Hub</SelectItem>
            <SelectItem value="github">GitHub</SelectItem>
          </SelectContent>
        </Select>
      </div>
      <div className="space-y-2">
        <Label htmlFor="repository">Repository</Label>
        <Input id="repository" value={repository} onChange={(e) => setRepository(e.target.value)} placeholder="e.g. library/golang or ethereum/go-ethereum" required />
      </div>
      <div className="space-y-2">
        <Label htmlFor="poll_interval">Poll Interval (seconds)</Label>
        <Input id="poll_interval" type="number" min={60} value={pollInterval} onChange={(e) => setPollInterval(e.target.value)} />
      </div>
      <div className="space-y-2">
        <Label htmlFor="config">Config (JSON, optional)</Label>
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
        <Label htmlFor="version_filter_include">Version Filter — Include (regex, optional)</Label>
        <Input
          id="version_filter_include"
          value={versionFilterInclude}
          onChange={(e) => setVersionFilterInclude(e.target.value)}
          placeholder='e.g. ^v\d+\.\d+\.\d+$'
          className="font-mono text-sm"
        />
        <p className="text-xs text-muted-foreground">Only show/notify versions matching this pattern</p>
      </div>
      <div className="space-y-2">
        <Label htmlFor="version_filter_exclude">Version Filter — Exclude (regex, optional)</Label>
        <Input
          id="version_filter_exclude"
          value={versionFilterExclude}
          onChange={(e) => setVersionFilterExclude(e.target.value)}
          placeholder='e.g. -(alpha|beta|rc|nightly)'
          className="font-mono text-sm"
        />
        <p className="text-xs text-muted-foreground">Hide/suppress versions matching this pattern</p>
      </div>
      <div className="flex items-center gap-3">
        <Switch checked={enabled} onCheckedChange={setEnabled} />
        <Label>Enabled</Label>
      </div>
      <div className="flex justify-end gap-2">
        <Button type="button" variant="outline" onClick={handleCancel}>Cancel</Button>
        <Button type="submit" disabled={saving}>{saving ? "Saving..." : "Save"}</Button>
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
