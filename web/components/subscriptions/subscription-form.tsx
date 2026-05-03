"use client";

import { useState, useMemo, useEffect } from "react";
import { useRouter } from "next/navigation";
import useSWR from "swr";
import { useTranslation } from "@/lib/i18n/context";
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
  const { t } = useTranslation();
  const isEditing = !!initial;
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState("");
  const [type] = useState<"source_release">("source_release");
  const [channelId, setChannelId] = useState(initial?.channel_id ?? "");
  // Single-select state (edit mode)
  const [projectId, setProjectId] = useState(initial?.project_id ?? "");
  const [sourceId, setSourceId] = useState(initial?.source_id ?? "");
  // Multi-select state (create mode)
  const [selectedSourceIds, setSelectedSourceIds] = useState<string[]>([]);
  const [sourceSearch, setSourceSearch] = useState("");
  const [versionFilter, setVersionFilter] = useState(initial?.version_filter ?? "");

  const { data: projectsData } = useSWR("projects-for-sub", () => projectsApi.list(1, 100));
  const { data: channelsData } = useSWR("channels-for-sub", () => channelsApi.list());

  // When editing, resolve the source to find its project_id
  const { data: initialSourceData } = useSWR(
    isEditing && initial?.source_id ? `source-detail-${initial.source_id}` : null,
    () => sourcesApi.get(initial!.source_id!)
  );
  useEffect(() => {
    if (initialSourceData?.data && !projectId) {
      setProjectId(initialSourceData.data.project_id);
    }
  }, [initialSourceData]);

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

  const filteredSourcesByProject = useMemo(() => {
    if (!sourceSearch.trim()) return sourcesByProject;
    const q = sourceSearch.toLowerCase();
    return sourcesByProject
      .map((group) => {
        if (group.projectName.toLowerCase().includes(q)) return group;
        const filtered = group.sources.filter(
          (s) => s.repository.toLowerCase().includes(q) || s.provider.toLowerCase().includes(q)
        );
        if (filtered.length === 0) return null;
        return { ...group, sources: filtered };
      })
      .filter(Boolean) as typeof sourcesByProject;
  }, [sourcesByProject, sourceSearch]);

  const toggleProjectSources = (group: { sources: Source[] }) => {
    const ids = group.sources.map((s) => s.id);
    const allSelected = ids.every((id) => selectedSourceIds.includes(id));
    if (allSelected) {
      setSelectedSourceIds((prev) => prev.filter((id) => !ids.includes(id)));
    } else {
      setSelectedSourceIds((prev) => [...new Set([...prev, ...ids])]);
    }
  };

  const toggleSourceId = (id: string) => {
    setSelectedSourceIds((prev) =>
      prev.includes(id) ? prev.filter((x) => x !== id) : [...prev, id]
    );
  };

  const allSourceIds = allSourcesData?.map((s) => s.id) ?? [];

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
          source_id: sourceId,
          version_filter: versionFilter || undefined,
        });
      } else if (onBatchSubmit) {
        await onBatchSubmit({
          channel_id: channelId,
          type,
          source_ids: selectedSourceIds,
          version_filter: versionFilter || undefined,
        });
      }
      if (onSuccess) {
        onSuccess();
      } else {
        router.push("/subscriptions");
      }
    } catch (err: unknown) {
      setError(err instanceof Error ? err.message : t("subscriptionForm.failedToSave"));
    } finally {
      setSaving(false);
    }
  };

  const formContent = (
    <form onSubmit={handleSubmit} className="space-y-4">
      {error && <div className="rounded-md bg-red-50 p-3 text-sm text-red-700">{error}</div>}

      {/* SOURCE SELECTION */}
      {type === "source_release" && isEditing && (
        <>
          <div className="space-y-2">
            <Label>{t("subscriptionForm.projectToListSources")}</Label>
            <Select value={projectId} onValueChange={(v) => { setProjectId(v); setSourceId(""); }}>
              <SelectTrigger><SelectValue placeholder={t("subscriptionForm.selectProject")} /></SelectTrigger>
              <SelectContent>
                {projectsData?.data.map((p) => (
                  <SelectItem key={p.id} value={p.id}>{p.name}</SelectItem>
                ))}
              </SelectContent>
            </Select>
          </div>
          <div className="space-y-2">
            <Label>{t("subscriptionForm.source")}</Label>
            <Select value={sourceId} onValueChange={setSourceId} required>
              <SelectTrigger><SelectValue placeholder={t("subscriptionForm.selectSource")} /></SelectTrigger>
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
            <Label>{t("subscriptionForm.sources")}</Label>
            {allSourceIds.length > 0 && (
              <button
                type="button"
                onClick={toggleAllSources}
                className="text-xs text-blue-600 hover:text-blue-800"
              >
                {selectedSourceIds.length === allSourceIds.length ? t("subscriptionForm.deselectAll") : t("subscriptionForm.selectAll")}
              </button>
            )}
          </div>
          <Input
            placeholder="Search projects or sources..."
            value={sourceSearch}
            onChange={(e) => setSourceSearch(e.target.value)}
          />
          <div className="max-h-64 overflow-y-auto rounded-md border border-input p-2 space-y-3">
            {filteredSourcesByProject.map((group) => {
              const groupIds = group.sources.map((s) => s.id);
              const allGroupSelected = groupIds.every((id) => selectedSourceIds.includes(id));
              const someGroupSelected = !allGroupSelected && groupIds.some((id) => selectedSourceIds.includes(id));
              return (
                <div key={group.projectName}>
                  <label className="flex items-center gap-2 px-2 pb-1 cursor-pointer">
                    <Checkbox
                      checked={allGroupSelected ? true : someGroupSelected ? "indeterminate" : false}
                      onCheckedChange={() => toggleProjectSources(group)}
                    />
                    <span className="text-xs font-medium text-muted-foreground uppercase tracking-wider">{group.projectName}</span>
                  </label>
                  <div className="space-y-1">
                    {group.sources.map((s) => (
                      <label key={s.id} className="flex items-center gap-2 rounded px-2 py-1.5 hover:bg-accent cursor-pointer ml-4">
                        <Checkbox
                          checked={selectedSourceIds.includes(s.id)}
                          onCheckedChange={() => toggleSourceId(s.id)}
                        />
                        <span className="text-sm">{s.provider}: {s.repository}</span>
                      </label>
                    ))}
                  </div>
                </div>
              );
            })}
            {filteredSourcesByProject.length === 0 && (
              <p className="text-sm text-muted-foreground px-2 py-1">{sourceSearch ? "No matches found" : t("subscriptionForm.noSourcesFound")}</p>
            )}
          </div>
          {selectedSourceIds.length > 0 && (
            <p className="text-xs text-muted-foreground">{selectedSourceIds.length} {t("subscriptionForm.selected")}</p>
          )}
        </div>
      )}

      <div className="space-y-2">
        <Label>{t("subscriptionForm.notificationChannel")}</Label>
        <Select value={channelId} onValueChange={setChannelId} required>
          <SelectTrigger><SelectValue placeholder={t("subscriptionForm.selectChannel")} /></SelectTrigger>
          <SelectContent>
            {channelsData?.data.map((ch) => (
              <SelectItem key={ch.id} value={ch.id}>{ch.name} ({ch.type})</SelectItem>
            ))}
          </SelectContent>
        </Select>
      </div>
      <div className="space-y-2">
        <Label htmlFor="version_filter">{t("subscriptionForm.versionFilter")}</Label>
        <Input id="version_filter" value={versionFilter} onChange={(e) => setVersionFilter(e.target.value)} placeholder={t("subscriptionForm.versionFilterPlaceholder")} />
      </div>
      <div className="flex justify-end gap-2">
        <Button type="button" variant="outline" onClick={() => onCancel ? onCancel() : router.back()}>{t("subscriptionForm.cancel")}</Button>
        <Button type="submit" disabled={saving}>{saving ? t("subscriptionForm.saving") : t("subscriptionForm.save")}</Button>
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
