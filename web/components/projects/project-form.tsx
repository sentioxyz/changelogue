"use client";

import { useState } from "react";
import { useRouter } from "next/navigation";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Textarea } from "@/components/ui/textarea";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import type { Project, ProjectInput } from "@/lib/api/types";

interface ProjectFormProps {
  initial?: Project;
  onSubmit: (input: ProjectInput) => Promise<void>;
  title: string;
}

export function ProjectForm({ initial, onSubmit, title }: ProjectFormProps) {
  const router = useRouter();
  const [saving, setSaving] = useState(false);
  const [name, setName] = useState(initial?.name ?? "");
  const [description, setDescription] = useState(initial?.description ?? "");
  const [url, setUrl] = useState(initial?.url ?? "");
  const [pipelineConfig, setPipelineConfig] = useState(
    JSON.stringify(initial?.pipeline_config ?? { changelog_summarizer: {}, urgency_scorer: {} }, null, 2)
  );
  const [error, setError] = useState("");

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    setError("");
    let parsedConfig: Record<string, unknown>;
    try {
      parsedConfig = JSON.parse(pipelineConfig);
    } catch {
      setError("Pipeline config must be valid JSON");
      return;
    }
    setSaving(true);
    try {
      await onSubmit({ name, description, url, pipeline_config: parsedConfig });
      router.push("/projects");
    } catch (err: unknown) {
      setError(err instanceof Error ? err.message : "Failed to save");
    } finally {
      setSaving(false);
    }
  };

  return (
    <Card className="mx-auto max-w-2xl">
      <CardHeader>
        <CardTitle>{title}</CardTitle>
      </CardHeader>
      <CardContent>
        <form onSubmit={handleSubmit} className="space-y-4">
          {error && <div className="rounded-md bg-red-50 p-3 text-sm text-red-700">{error}</div>}
          <div className="space-y-2">
            <Label htmlFor="name">Name</Label>
            <Input id="name" value={name} onChange={(e) => setName(e.target.value)} required />
          </div>
          <div className="space-y-2">
            <Label htmlFor="description">Description</Label>
            <Textarea id="description" value={description} onChange={(e) => setDescription(e.target.value)} rows={2} />
          </div>
          <div className="space-y-2">
            <Label htmlFor="url">URL</Label>
            <Input id="url" type="url" value={url} onChange={(e) => setUrl(e.target.value)} />
          </div>
          <div className="space-y-2">
            <Label htmlFor="pipeline_config">Pipeline Config (JSON)</Label>
            <Textarea
              id="pipeline_config"
              value={pipelineConfig}
              onChange={(e) => setPipelineConfig(e.target.value)}
              rows={8}
              className="font-mono text-sm"
            />
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
