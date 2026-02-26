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
import { Plus, X } from "lucide-react";

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
  const [saving, setSaving] = useState(false);
  const [name, setName] = useState(initial?.name ?? "");
  const [description, setDescription] = useState(initial?.description ?? "");
  const [error, setError] = useState("");

  /* Source fields (only for create) */
  const [showSource, setShowSource] = useState(false);
  const [provider, setProvider] = useState("github");
  const [repository, setRepository] = useState("");
  const [pollInterval, setPollInterval] = useState("300");

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
        result.source = {
          provider,
          repository: repository.trim(),
          poll_interval_seconds: Number(pollInterval) || 300,
          enabled: true,
        };
      }
      await onSubmit(result);
      if (onSuccess) {
        onSuccess();
      } else {
        router.push("/projects");
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
        <Label htmlFor="name">Name</Label>
        <Input id="name" value={name} onChange={(e) => setName(e.target.value)} required />
      </div>
      <div className="space-y-2">
        <Label htmlFor="description">Description</Label>
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
              style={{ color: "#e8601a" }}
            >
              <Plus className="h-3.5 w-3.5" />
              Add a Source
            </button>
          ) : (
            <div className="rounded-md border p-4 space-y-3" style={{ borderColor: "#e8e8e5" }}>
              <div className="flex items-center justify-between">
                <Label className="text-[13px] font-medium">Add a Source</Label>
                <button
                  type="button"
                  onClick={() => { setShowSource(false); setRepository(""); }}
                  className="text-[#9ca3af] hover:text-[#6b7280]"
                >
                  <X className="h-4 w-4" />
                </button>
              </div>
              <div className="space-y-2">
                <Label>Provider</Label>
                <Select value={provider} onValueChange={setProvider}>
                  <SelectTrigger><SelectValue /></SelectTrigger>
                  <SelectContent>
                    <SelectItem value="github">GitHub</SelectItem>
                    <SelectItem value="dockerhub">Docker Hub</SelectItem>
                  </SelectContent>
                </Select>
              </div>
              <div className="space-y-2">
                <Label htmlFor="repository">Repository</Label>
                <Input
                  id="repository"
                  value={repository}
                  onChange={(e) => setRepository(e.target.value)}
                  placeholder="e.g. ethereum/go-ethereum"
                />
              </div>
              <div className="space-y-2">
                <Label htmlFor="poll_interval">Poll Interval (seconds)</Label>
                <Input
                  id="poll_interval"
                  type="number"
                  min={60}
                  value={pollInterval}
                  onChange={(e) => setPollInterval(e.target.value)}
                />
              </div>
            </div>
          )}
        </div>
      )}

      <div className="flex justify-end gap-2">
        <Button type="button" variant="outline" onClick={handleCancel}>Cancel</Button>
        <Button type="submit" disabled={saving}>{saving ? "Saving..." : "Save"}</Button>
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
