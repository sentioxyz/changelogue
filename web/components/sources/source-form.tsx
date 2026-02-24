"use client";

import { useState } from "react";
import { useRouter } from "next/navigation";
import useSWR from "swr";
import { projects as projectsApi, system } from "@/lib/api/client";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Switch } from "@/components/ui/switch";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import type { Source, SourceInput } from "@/lib/api/types";

interface SourceFormProps {
  initial?: Source;
  defaultProjectId?: number;
  onSubmit: (input: SourceInput) => Promise<void>;
  title: string;
}

export function SourceForm({ initial, defaultProjectId, onSubmit, title }: SourceFormProps) {
  const router = useRouter();
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState("");
  const [projectId, setProjectId] = useState(String(initial?.project_id ?? defaultProjectId ?? ""));
  const [type, setType] = useState(initial?.type ?? "dockerhub");
  const [repository, setRepository] = useState(initial?.repository ?? "");
  const [pollInterval, setPollInterval] = useState(String(initial?.poll_interval_seconds ?? 300));
  const [enabled, setEnabled] = useState(initial?.enabled ?? true);
  const [excludeRegexp, setExcludeRegexp] = useState(initial?.exclude_version_regexp ?? "");
  const [excludePrereleases, setExcludePrereleases] = useState(initial?.exclude_prereleases ?? false);

  const { data: projectsData } = useSWR("projects-for-form", () => projectsApi.list());
  const { data: providersData } = useSWR("providers", () => system.providers());

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    setError("");
    setSaving(true);
    try {
      await onSubmit({
        project_id: Number(projectId),
        type,
        repository,
        poll_interval_seconds: Number(pollInterval),
        enabled,
        exclude_version_regexp: excludeRegexp || undefined,
        exclude_prereleases: excludePrereleases || undefined,
      });
      router.push("/sources");
    } catch (err: unknown) {
      setError(err instanceof Error ? err.message : "Failed to save");
    } finally {
      setSaving(false);
    }
  };

  return (
    <Card className="mx-auto max-w-2xl">
      <CardHeader><CardTitle>{title}</CardTitle></CardHeader>
      <CardContent>
        <form onSubmit={handleSubmit} className="space-y-4">
          {error && <div className="rounded-md bg-red-50 p-3 text-sm text-red-700">{error}</div>}
          <div className="space-y-2">
            <Label>Project</Label>
            <Select value={projectId} onValueChange={setProjectId} required>
              <SelectTrigger><SelectValue placeholder="Select project" /></SelectTrigger>
              <SelectContent>
                {projectsData?.data.map((p) => (
                  <SelectItem key={p.id} value={String(p.id)}>{p.name}</SelectItem>
                ))}
              </SelectContent>
            </Select>
          </div>
          <div className="space-y-2">
            <Label>Source Type</Label>
            <Select value={type} onValueChange={setType}>
              <SelectTrigger><SelectValue /></SelectTrigger>
              <SelectContent>
                {(providersData?.data ?? [{ type: "dockerhub", name: "Docker Hub" }, { type: "github", name: "GitHub" }]).map((p) => (
                  <SelectItem key={p.type} value={p.type}>{p.name}</SelectItem>
                ))}
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
            <Label htmlFor="exclude_regexp">Exclude Version Pattern (regex)</Label>
            <Input id="exclude_regexp" value={excludeRegexp} onChange={(e) => setExcludeRegexp(e.target.value)} placeholder="e.g. -(alpha|beta|nightly)" />
          </div>
          <div className="flex items-center gap-3">
            <Switch checked={excludePrereleases} onCheckedChange={setExcludePrereleases} />
            <Label>Exclude pre-releases</Label>
          </div>
          <div className="flex items-center gap-3">
            <Switch checked={enabled} onCheckedChange={setEnabled} />
            <Label>Enabled</Label>
          </div>
          <div className="flex justify-end gap-2">
            <Button type="button" variant="outline" onClick={() => router.back()}>Cancel</Button>
            <Button type="submit" disabled={saving}>{saving ? "Saving..." : "Save"}</Button>
          </div>
        </form>
      </CardContent>
    </Card>
  );
}
