"use client";

import { useState } from "react";
import { useRouter } from "next/navigation";
import useSWR from "swr";
import { projects as projectsApi, channels as channelsApi, sources as sourcesApi } from "@/lib/api/client";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import type { Subscription, SubscriptionInput } from "@/lib/api/types";

interface SubscriptionFormProps {
  initial?: Subscription;
  onSubmit: (input: SubscriptionInput) => Promise<void>;
  title: string;
  onSuccess?: () => void;
  onCancel?: () => void;
}

export function SubscriptionForm({ initial, onSubmit, title, onSuccess, onCancel }: SubscriptionFormProps) {
  const router = useRouter();
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState("");
  const [type, setType] = useState<"source" | "project">(initial?.type ?? "project");
  const [channelId, setChannelId] = useState(initial?.channel_id ?? "");
  const [projectId, setProjectId] = useState(initial?.project_id ?? "");
  const [sourceId, setSourceId] = useState(initial?.source_id ?? "");
  const [versionFilter, setVersionFilter] = useState(initial?.version_filter ?? "");

  const { data: projectsData } = useSWR("projects-for-sub", () => projectsApi.list());
  const { data: channelsData } = useSWR("channels-for-sub", () => channelsApi.list());
  const { data: sourcesData } = useSWR(
    type === "source" && projectId ? `sources-for-sub-${projectId}` : null,
    () => sourcesApi.listByProject(projectId)
  );

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    setError("");
    setSaving(true);
    try {
      await onSubmit({
        channel_id: channelId,
        type,
        source_id: type === "source" ? sourceId : undefined,
        project_id: type === "project" ? projectId : undefined,
        version_filter: versionFilter || undefined,
      });
      if (onSuccess) {
        onSuccess();
      } else {
        router.push("/subscriptions");
      }
    } catch (err: unknown) {
      setError(err instanceof Error ? err.message : "Failed to save");
    } finally {
      setSaving(false);
    }
  };

  const formContent = (
    <form onSubmit={handleSubmit} className="space-y-4">
      {error && <div className="rounded-md bg-red-50 p-3 text-sm text-red-700">{error}</div>}
      <div className="space-y-2">
        <Label>Subscription Type</Label>
        <Select value={type} onValueChange={(v) => setType(v as "source" | "project")}>
          <SelectTrigger><SelectValue /></SelectTrigger>
          <SelectContent>
            <SelectItem value="project">Project</SelectItem>
            <SelectItem value="source">Source</SelectItem>
          </SelectContent>
        </Select>
      </div>
      {type === "project" && (
        <div className="space-y-2">
          <Label>Project</Label>
          <Select value={projectId} onValueChange={setProjectId} required>
            <SelectTrigger><SelectValue placeholder="Select project" /></SelectTrigger>
            <SelectContent>
              {projectsData?.data.map((p) => (
                <SelectItem key={p.id} value={p.id}>{p.name}</SelectItem>
              ))}
            </SelectContent>
          </Select>
        </div>
      )}
      {type === "source" && (
        <>
          <div className="space-y-2">
            <Label>Project (to list sources)</Label>
            <Select value={projectId} onValueChange={(v) => { setProjectId(v); setSourceId(""); }}>
              <SelectTrigger><SelectValue placeholder="Select project" /></SelectTrigger>
              <SelectContent>
                {projectsData?.data.map((p) => (
                  <SelectItem key={p.id} value={p.id}>{p.name}</SelectItem>
                ))}
              </SelectContent>
            </Select>
          </div>
          <div className="space-y-2">
            <Label>Source</Label>
            <Select value={sourceId} onValueChange={setSourceId} required>
              <SelectTrigger><SelectValue placeholder="Select source" /></SelectTrigger>
              <SelectContent>
                {sourcesData?.data.map((s) => (
                  <SelectItem key={s.id} value={s.id}>{s.provider}: {s.repository}</SelectItem>
                ))}
              </SelectContent>
            </Select>
          </div>
        </>
      )}
      <div className="space-y-2">
        <Label>Notification Channel</Label>
        <Select value={channelId} onValueChange={setChannelId} required>
          <SelectTrigger><SelectValue placeholder="Select channel" /></SelectTrigger>
          <SelectContent>
            {channelsData?.data.map((ch) => (
              <SelectItem key={ch.id} value={ch.id}>{ch.name} ({ch.type})</SelectItem>
            ))}
          </SelectContent>
        </Select>
      </div>
      <div className="space-y-2">
        <Label htmlFor="version_filter">Version Filter (regex, optional)</Label>
        <Input id="version_filter" value={versionFilter} onChange={(e) => setVersionFilter(e.target.value)} placeholder='e.g. ^v\d+\.\d+\.\d+$' />
      </div>
      <div className="flex justify-end gap-2">
        <Button type="button" variant="outline" onClick={() => onCancel ? onCancel() : router.back()}>Cancel</Button>
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
