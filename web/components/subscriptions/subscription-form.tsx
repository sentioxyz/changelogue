"use client";

import { useState, useMemo } from "react";
import { useRouter } from "next/navigation";
import useSWR from "swr";
import { projects as projectsApi, channels as channelsApi, sources as sourcesApi } from "@/lib/api/client";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Checkbox } from "@/components/ui/checkbox";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import type { Subscription, SubscriptionInput, BatchSubscriptionInput, Source } from "@/lib/api/types";

interface SubscriptionFormProps {
  initial?: Subscription;
  onSubmit: (input: SubscriptionInput) => Promise<void>;
  onBatchSubmit?: (input: BatchSubscriptionInput) => Promise<void>;
  title: string;
  onSuccess?: () => void;
  onCancel?: () => void;
}

export function SubscriptionForm({ initial, onSubmit, onBatchSubmit, title, onSuccess, onCancel }: SubscriptionFormProps) {
  const router = useRouter();
  const isEditing = !!initial;
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState("");
  const [type, setType] = useState<"source_release" | "semantic_release">(initial?.type ?? "semantic_release");
  const [channelId, setChannelId] = useState(initial?.channel_id ?? "");
  // Single-select state (edit mode)
  const [projectId, setProjectId] = useState(initial?.project_id ?? "");
  const [sourceId, setSourceId] = useState(initial?.source_id ?? "");
  // Multi-select state (create mode)
  const [selectedProjectIds, setSelectedProjectIds] = useState<string[]>([]);
  const [selectedSourceIds, setSelectedSourceIds] = useState<string[]>([]);
  const [versionFilter, setVersionFilter] = useState(initial?.version_filter ?? "");

  const { data: projectsData } = useSWR("projects-for-sub", () => projectsApi.list(1, 100));
  const { data: channelsData } = useSWR("channels-for-sub", () => channelsApi.list());

  // Edit mode: fetch sources for the selected project
  const { data: sourcesData } = useSWR(
    type === "source_release" && isEditing && projectId
      ? `sources-for-sub-${projectId}`
      : null,
    () => sourcesApi.listByProject(projectId)
  );

  // Create mode (source type): fetch sources for ALL projects
  const projectIds = projectsData?.data.map((p) => p.id) ?? [];
  const { data: allSourcesData } = useSWR(
    type === "source_release" && !isEditing && projectIds.length > 0
      ? `all-sources-for-sub-${projectIds.join(",")}`
      : null,
    async () => {
      const results = await Promise.all(
        projectIds.map((pid) => sourcesApi.listByProject(pid).catch(() => ({ data: [] as Source[] })))
      );
      return results.flatMap((r) => r.data);
    }
  );

  // Group all sources by project for display
  const sourcesByProject = useMemo(() => {
    if (!allSourcesData || !projectsData?.data) return [];
    const projectMap = new Map(projectsData.data.map((p) => [p.id, p.name]));
    const groups = new Map<string, { projectName: string; sources: Source[] }>();
    for (const s of allSourcesData) {
      const name = projectMap.get(s.project_id) ?? s.project_id;
      if (!groups.has(s.project_id)) {
        groups.set(s.project_id, { projectName: name, sources: [] });
      }
      groups.get(s.project_id)!.sources.push(s);
    }
    return Array.from(groups.values());
  }, [allSourcesData, projectsData]);

  const toggleProjectId = (id: string) => {
    setSelectedProjectIds((prev) =>
      prev.includes(id) ? prev.filter((x) => x !== id) : [...prev, id]
    );
  };

  const toggleSourceId = (id: string) => {
    setSelectedSourceIds((prev) =>
      prev.includes(id) ? prev.filter((x) => x !== id) : [...prev, id]
    );
  };

  const allProjectIds = projectsData?.data.map((p) => p.id) ?? [];
  const allSourceIds = allSourcesData?.map((s) => s.id) ?? [];

  const toggleAllProjects = () => {
    if (selectedProjectIds.length === allProjectIds.length) {
      setSelectedProjectIds([]);
    } else {
      setSelectedProjectIds([...allProjectIds]);
    }
  };

  const toggleAllSources = () => {
    if (selectedSourceIds.length === allSourceIds.length) {
      setSelectedSourceIds([]);
    } else {
      setSelectedSourceIds([...allSourceIds]);
    }
  };

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    setError("");
    setSaving(true);
    try {
      if (isEditing) {
        await onSubmit({
          channel_id: channelId,
          type,
          source_id: type === "source_release" ? sourceId : undefined,
          project_id: type === "semantic_release" ? projectId : undefined,
          version_filter: versionFilter || undefined,
        });
      } else if (onBatchSubmit) {
        await onBatchSubmit({
          channel_id: channelId,
          type,
          project_ids: type === "semantic_release" ? selectedProjectIds : undefined,
          source_ids: type === "source_release" ? selectedSourceIds : undefined,
          version_filter: versionFilter || undefined,
        });
      }
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
        <Select value={type} onValueChange={(v) => { setType(v as "source_release" | "semantic_release"); setSelectedProjectIds([]); setSelectedSourceIds([]); }}>
          <SelectTrigger><SelectValue /></SelectTrigger>
          <SelectContent>
            <SelectItem value="semantic_release">Semantic Release</SelectItem>
            <SelectItem value="source_release">Source Release</SelectItem>
          </SelectContent>
        </Select>
      </div>

      {/* PROJECT SELECTION */}
      {type === "semantic_release" && isEditing && (
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
      {type === "semantic_release" && !isEditing && (
        <div className="space-y-2">
          <div className="flex items-center justify-between">
            <Label>Projects</Label>
            <button
              type="button"
              onClick={toggleAllProjects}
              className="text-xs text-blue-600 hover:text-blue-800"
            >
              {selectedProjectIds.length === allProjectIds.length ? "Deselect all" : "Select all"}
            </button>
          </div>
          <div className="max-h-48 overflow-y-auto rounded-md border border-input p-2 space-y-1">
            {projectsData?.data.map((p) => (
              <label key={p.id} className="flex items-center gap-2 rounded px-2 py-1.5 hover:bg-accent cursor-pointer">
                <Checkbox
                  checked={selectedProjectIds.includes(p.id)}
                  onCheckedChange={() => toggleProjectId(p.id)}
                />
                <span className="text-sm">{p.name}</span>
              </label>
            ))}
            {(!projectsData?.data || projectsData.data.length === 0) && (
              <p className="text-sm text-muted-foreground px-2 py-1">No projects found</p>
            )}
          </div>
          {selectedProjectIds.length > 0 && (
            <p className="text-xs text-muted-foreground">{selectedProjectIds.length} selected</p>
          )}
        </div>
      )}

      {/* SOURCE SELECTION */}
      {type === "source_release" && isEditing && (
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
      {type === "source_release" && !isEditing && (
        <div className="space-y-2">
          <div className="flex items-center justify-between">
            <Label>Sources</Label>
            {allSourceIds.length > 0 && (
              <button
                type="button"
                onClick={toggleAllSources}
                className="text-xs text-blue-600 hover:text-blue-800"
              >
                {selectedSourceIds.length === allSourceIds.length ? "Deselect all" : "Select all"}
              </button>
            )}
          </div>
          <div className="max-h-64 overflow-y-auto rounded-md border border-input p-2 space-y-3">
            {sourcesByProject.map((group) => (
              <div key={group.projectName}>
                <p className="text-xs font-medium text-muted-foreground uppercase tracking-wider px-2 pb-1">{group.projectName}</p>
                <div className="space-y-1">
                  {group.sources.map((s) => (
                    <label key={s.id} className="flex items-center gap-2 rounded px-2 py-1.5 hover:bg-accent cursor-pointer">
                      <Checkbox
                        checked={selectedSourceIds.includes(s.id)}
                        onCheckedChange={() => toggleSourceId(s.id)}
                      />
                      <span className="text-sm">{s.provider}: {s.repository}</span>
                    </label>
                  ))}
                </div>
              </div>
            ))}
            {sourcesByProject.length === 0 && (
              <p className="text-sm text-muted-foreground px-2 py-1">No sources found</p>
            )}
          </div>
          {selectedSourceIds.length > 0 && (
            <p className="text-xs text-muted-foreground">{selectedSourceIds.length} selected</p>
          )}
        </div>
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
