"use client";

import { useState } from "react";
import { useRouter } from "next/navigation";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import type { NotificationChannel, ChannelInput } from "@/lib/api/types";

const channelFields: Record<string, { label: string; placeholder: string }[]> = {
  slack: [
    { label: "Webhook URL", placeholder: "https://hooks.slack.com/services/..." },
    { label: "Channel", placeholder: "#releases" },
  ],
  pagerduty: [
    { label: "Routing Key", placeholder: "R0xxxxx" },
  ],
  webhook: [
    { label: "URL", placeholder: "https://your-service.com/api/releases" },
    { label: "Headers", placeholder: "Authorization: Bearer token" },
  ],
};

interface ChannelFormProps {
  initial?: NotificationChannel;
  onSubmit: (input: ChannelInput) => Promise<void>;
  title: string;
}

export function ChannelForm({ initial, onSubmit, title }: ChannelFormProps) {
  const router = useRouter();
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState("");
  const [type, setType] = useState(initial?.type ?? "slack");
  const [name, setName] = useState(initial?.name ?? "");
  const [config, setConfig] = useState<Record<string, unknown>>(initial?.config ?? {});

  const fields = channelFields[type] ?? [];

  const handleConfigChange = (label: string, value: string) => {
    const key = label.toLowerCase().replace(/ /g, "_");
    setConfig((prev) => ({ ...prev, [key]: value }));
  };

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    setError("");
    setSaving(true);
    try {
      await onSubmit({ type, name, config });
      router.push("/channels");
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
            <Label htmlFor="name">Name</Label>
            <Input id="name" value={name} onChange={(e) => setName(e.target.value)} placeholder="e.g. Engineering Releases" required />
          </div>
          <div className="space-y-2">
            <Label>Type</Label>
            <Select value={type} onValueChange={(v) => { setType(v); setConfig({}); }}>
              <SelectTrigger><SelectValue /></SelectTrigger>
              <SelectContent>
                <SelectItem value="slack">Slack</SelectItem>
                <SelectItem value="pagerduty">PagerDuty</SelectItem>
                <SelectItem value="webhook">Webhook</SelectItem>
              </SelectContent>
            </Select>
          </div>
          {fields.map((field) => {
            const key = field.label.toLowerCase().replace(/ /g, "_");
            return (
              <div key={key} className="space-y-2">
                <Label htmlFor={key}>{field.label}</Label>
                <Input id={key} value={String(config[key] ?? "")} onChange={(e) => handleConfigChange(field.label, e.target.value)} placeholder={field.placeholder} />
              </div>
            );
          })}
          <div className="flex justify-end gap-2">
            <Button type="button" variant="outline" onClick={() => router.back()}>Cancel</Button>
            <Button type="submit" disabled={saving}>{saving ? "Saving..." : "Save"}</Button>
          </div>
        </form>
      </CardContent>
    </Card>
  );
}
