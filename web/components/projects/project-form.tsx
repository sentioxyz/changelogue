"use client";

import { useState } from "react";
import { useRouter } from "next/navigation";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Textarea } from "@/components/ui/textarea";
import { Switch } from "@/components/ui/switch";
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
  const [agentPrompt, setAgentPrompt] = useState(initial?.agent_prompt ?? "");
  const [onMajor, setOnMajor] = useState(initial?.agent_rules?.on_major_release ?? true);
  const [onMinor, setOnMinor] = useState(initial?.agent_rules?.on_minor_release ?? false);
  const [onSecurity, setOnSecurity] = useState(initial?.agent_rules?.on_security_patch ?? true);
  const [versionPattern, setVersionPattern] = useState(initial?.agent_rules?.version_pattern ?? "");
  const [error, setError] = useState("");

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    setError("");
    setSaving(true);
    try {
      await onSubmit({
        name,
        description: description || undefined,
        agent_prompt: agentPrompt || undefined,
        agent_rules: {
          on_major_release: onMajor,
          on_minor_release: onMinor,
          on_security_patch: onSecurity,
          version_pattern: versionPattern || undefined,
        },
      });
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
            <Label htmlFor="agent_prompt">Agent Prompt</Label>
            <Textarea
              id="agent_prompt"
              value={agentPrompt}
              onChange={(e) => setAgentPrompt(e.target.value)}
              rows={6}
              placeholder="Custom instructions for the agent when analyzing releases for this project..."
              className="font-mono text-sm"
            />
          </div>
          <div className="space-y-3">
            <Label>Agent Rules</Label>
            <div className="rounded-md border p-4 space-y-3">
              <div className="flex items-center gap-3">
                <Switch checked={onMajor} onCheckedChange={setOnMajor} />
                <Label>Trigger on major releases</Label>
              </div>
              <div className="flex items-center gap-3">
                <Switch checked={onMinor} onCheckedChange={setOnMinor} />
                <Label>Trigger on minor releases</Label>
              </div>
              <div className="flex items-center gap-3">
                <Switch checked={onSecurity} onCheckedChange={setOnSecurity} />
                <Label>Trigger on security patches</Label>
              </div>
              <div className="space-y-2">
                <Label htmlFor="version_pattern">Version Pattern (regex, optional)</Label>
                <Input
                  id="version_pattern"
                  value={versionPattern}
                  onChange={(e) => setVersionPattern(e.target.value)}
                  placeholder='e.g. ^v\d+\.\d+\.\d+$'
                />
              </div>
            </div>
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
