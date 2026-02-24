"use client";

import { useState } from "react";
import { useRouter } from "next/navigation";
import useSWR from "swr";
import { projects as projectsApi, channels as channelsApi } from "@/lib/api/client";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Switch } from "@/components/ui/switch";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import type { Subscription, SubscriptionInput } from "@/lib/api/types";

interface SubscriptionFormProps {
  initial?: Subscription;
  onSubmit: (input: SubscriptionInput) => Promise<void>;
  title: string;
}

export function SubscriptionForm({ initial, onSubmit, title }: SubscriptionFormProps) {
  const router = useRouter();
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState("");
  const [projectId, setProjectId] = useState(String(initial?.project_id ?? ""));
  const [channelType, setChannelType] = useState(initial?.channel_type ?? "stable");
  const [channelId, setChannelId] = useState(String(initial?.channel_id ?? ""));
  const [versionPattern, setVersionPattern] = useState(initial?.version_pattern ?? "");
  const [frequency, setFrequency] = useState<string>(initial?.frequency ?? "instant");
  const [enabled, setEnabled] = useState(initial?.enabled ?? true);

  const { data: projectsData } = useSWR("projects-for-sub", () => projectsApi.list());
  const { data: channelsData } = useSWR("channels-for-sub", () => channelsApi.list());

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    setError("");
    setSaving(true);
    try {
      await onSubmit({
        project_id: Number(projectId),
        channel_type: channelType,
        channel_id: Number(channelId),
        version_pattern: versionPattern || undefined,
        frequency: frequency as SubscriptionInput["frequency"],
        enabled,
      });
      router.push("/subscriptions");
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
            <Label>Channel Type</Label>
            <Select value={channelType} onValueChange={setChannelType}>
              <SelectTrigger><SelectValue /></SelectTrigger>
              <SelectContent>
                <SelectItem value="stable">Stable</SelectItem>
                <SelectItem value="pre-release">Pre-release</SelectItem>
                <SelectItem value="security">Security</SelectItem>
              </SelectContent>
            </Select>
          </div>
          <div className="space-y-2">
            <Label>Notification Channel</Label>
            <Select value={channelId} onValueChange={setChannelId} required>
              <SelectTrigger><SelectValue placeholder="Select channel" /></SelectTrigger>
              <SelectContent>
                {channelsData?.data.map((ch) => (
                  <SelectItem key={ch.id} value={String(ch.id)}>{ch.name} ({ch.type})</SelectItem>
                ))}
              </SelectContent>
            </Select>
          </div>
          <div className="space-y-2">
            <Label htmlFor="version_pattern">Version Pattern (regex, optional)</Label>
            <Input id="version_pattern" value={versionPattern} onChange={(e) => setVersionPattern(e.target.value)} placeholder='e.g. ^v\d+\.\d+\.\d+$' />
          </div>
          <div className="space-y-2">
            <Label>Frequency</Label>
            <Select value={frequency} onValueChange={setFrequency}>
              <SelectTrigger><SelectValue /></SelectTrigger>
              <SelectContent>
                <SelectItem value="instant">Instant</SelectItem>
                <SelectItem value="hourly">Hourly Digest</SelectItem>
                <SelectItem value="daily">Daily Digest</SelectItem>
                <SelectItem value="weekly">Weekly Digest</SelectItem>
              </SelectContent>
            </Select>
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
