"use client";

import { useState, useEffect, useMemo } from "react";
import useSWR from "swr";
import { gates as gatesApi } from "@/lib/api/client";
import type {
  Source,
  ReleaseGate,
  ReleaseGateInput,
  VersionMapping,
} from "@/lib/api/types";
import { Switch } from "@/components/ui/switch";
import { Checkbox } from "@/components/ui/checkbox";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Textarea } from "@/components/ui/textarea";
import { Button } from "@/components/ui/button";
import { SectionLabel } from "@/components/ui/section-label";
import { ConfirmDialog } from "@/components/ui/confirm-dialog";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import { X, Plus } from "lucide-react";
import { useTranslation } from "@/lib/i18n/context";

interface ReleaseGateTabProps {
  projectId: string;
  sources: Source[];
}

export function ReleaseGateTab({ projectId, sources }: ReleaseGateTabProps) {
  const { t } = useTranslation();

  // Source display name lookup
  const sourceNames = useMemo(() => {
    const map: Record<string, string> = {};
    for (const s of sources) {
      map[s.id] = `${s.provider}/${s.repository}`;
    }
    return map;
  }, [sources]);

  // --- Gate config data ---
  const {
    data: gateData,
    mutate: mutateGate,
    isLoading: gateLoading,
  } = useSWR(`project-${projectId}-gate`, () => gatesApi.get(projectId));

  const gate = gateData?.data ?? null;

  // --- Form state ---
  const [enabled, setEnabled] = useState(false);
  const [requiredSources, setRequiredSources] = useState<string[]>([]);
  const [timeoutHours, setTimeoutHours] = useState(168);
  const [nlRule, setNlRule] = useState("");
  const [versionMapping, setVersionMapping] = useState<
    Record<string, VersionMapping>
  >({});
  const [saving, setSaving] = useState(false);
  const [deleteOpen, setDeleteOpen] = useState(false);

  // Sync form state from SWR data
  useEffect(() => {
    if (gate) {
      setEnabled(gate.enabled);
      setRequiredSources(gate.required_sources ?? []);
      setTimeoutHours(gate.timeout_hours);
      setNlRule(gate.nl_rule ?? "");
      setVersionMapping(gate.version_mapping ?? {});
    } else {
      setEnabled(false);
      setRequiredSources([]);
      setTimeoutHours(168);
      setNlRule("");
      setVersionMapping({});
    }
  }, [gate]);

  // --- Handlers ---
  const handleSave = async () => {
    setSaving(true);
    try {
      const input: ReleaseGateInput = {
        enabled,
        required_sources: requiredSources.length > 0 ? requiredSources : undefined,
        timeout_hours: timeoutHours,
        version_mapping:
          Object.keys(versionMapping).length > 0 ? versionMapping : undefined,
        nl_rule: nlRule || undefined,
      };
      await gatesApi.upsert(projectId, input);
      mutateGate();
    } finally {
      setSaving(false);
    }
  };

  const handleDelete = async () => {
    await gatesApi.delete(projectId);
    mutateGate({ data: null } as ReturnType<typeof gatesApi.get> extends Promise<infer R> ? R : never, false);
  };

  const toggleSource = (sourceId: string) => {
    setRequiredSources((prev) =>
      prev.includes(sourceId)
        ? prev.filter((id) => id !== sourceId)
        : [...prev, sourceId]
    );
  };

  // Version mapping helpers
  const [addMappingSourceId, setAddMappingSourceId] = useState("");
  const mappedSourceIds = Object.keys(versionMapping);
  const unmappedSources = sources.filter((s) => !mappedSourceIds.includes(s.id));

  const addMapping = () => {
    if (!addMappingSourceId) return;
    setVersionMapping((prev) => ({
      ...prev,
      [addMappingSourceId]: { pattern: "", template: "" },
    }));
    setAddMappingSourceId("");
  };

  const removeMapping = (sourceId: string) => {
    setVersionMapping((prev) => {
      const next = { ...prev };
      delete next[sourceId];
      return next;
    });
  };

  const updateMapping = (
    sourceId: string,
    field: "pattern" | "template",
    value: string
  ) => {
    setVersionMapping((prev) => ({
      ...prev,
      [sourceId]: { ...prev[sourceId], [field]: value },
    }));
  };

  if (gateLoading) {
    return (
      <div className="text-sm text-muted-foreground py-8 text-center">
        {t("projects.detail.loading")}
      </div>
    );
  }

  return (
    <div className="space-y-6">
      {/* Section 1: Gate Configuration */}
      <div className="rounded-lg border p-5 bg-surface">
        <div className="flex items-start justify-between mb-4">
          <div>
            <SectionLabel>{t("projects.detail.gateConfig")}</SectionLabel>
            <p className="text-sm text-muted-foreground mt-1">
              {t("projects.detail.gateConfigDesc")}
            </p>
          </div>
          <div className="flex items-center gap-2">
            <Label htmlFor="gate-enabled" className="text-sm text-muted-foreground">
              {t("projects.detail.gateEnabled")}
            </Label>
            <Switch
              id="gate-enabled"
              checked={enabled}
              onCheckedChange={setEnabled}
            />
          </div>
        </div>

        {/* Required Sources */}
        <div className="mb-4">
          <Label className="text-sm font-medium">
            {t("projects.detail.gateRequiredSources")}
          </Label>
          <p className="text-xs text-muted-foreground mt-1 mb-2">
            {t("projects.detail.gateRequiredSourcesHint")}
          </p>
          {sources.length === 0 ? (
            <p className="text-sm text-muted-foreground italic">
              {t("projects.detail.gateNoSources")}
            </p>
          ) : (
            <div className="flex flex-wrap gap-3">
              {sources.map((s) => (
                <label
                  key={s.id}
                  className="flex items-center gap-2 px-3 py-1.5 rounded-md border text-sm cursor-pointer hover:bg-accent/50"
                >
                  <Checkbox
                    checked={requiredSources.includes(s.id)}
                    onCheckedChange={() => toggleSource(s.id)}
                  />
                  {sourceNames[s.id]}
                </label>
              ))}
            </div>
          )}
        </div>

        {/* Timeout Hours */}
        <div className="mb-4">
          <Label htmlFor="gate-timeout" className="text-sm font-medium">
            {t("projects.detail.gateTimeoutHours")}
          </Label>
          <p className="text-xs text-muted-foreground mt-1 mb-2">
            {t("projects.detail.gateTimeoutHoursHint")}
          </p>
          <Input
            id="gate-timeout"
            type="number"
            min={1}
            className="w-32"
            value={timeoutHours}
            onChange={(e) => setTimeoutHours(Number(e.target.value) || 168)}
          />
        </div>

        {/* NL Rule */}
        <div className="mb-4">
          <Label htmlFor="gate-nl-rule" className="text-sm font-medium">
            {t("projects.detail.gateNLRule")}{" "}
            <span className="text-muted-foreground font-normal">
              {t("projects.detail.gateNLRuleOptional")}
            </span>
          </Label>
          <p className="text-xs text-muted-foreground mt-1 mb-2">
            {t("projects.detail.gateNLRuleHint")}
          </p>
          <Textarea
            id="gate-nl-rule"
            className="min-h-[60px]"
            value={nlRule}
            onChange={(e) => setNlRule(e.target.value)}
          />
        </div>

        {/* Version Mapping */}
        <div className="mb-4">
          <Label className="text-sm font-medium">
            {t("projects.detail.gateVersionMapping")}
          </Label>
          <p className="text-xs text-muted-foreground mt-1 mb-2">
            {t("projects.detail.gateVersionMappingHint")}
          </p>

          {mappedSourceIds.length > 0 && (
            <div className="rounded-md border overflow-hidden mb-2">
              <div className="grid grid-cols-[2fr_3fr_3fr_40px] gap-2 px-3 py-2 text-xs text-muted-foreground bg-muted/30 border-b">
                <div>{t("projects.detail.gateVMSource")}</div>
                <div>{t("projects.detail.gateVMPattern")}</div>
                <div>{t("projects.detail.gateVMTemplate")}</div>
                <div />
              </div>
              {mappedSourceIds.map((sid) => (
                <div
                  key={sid}
                  className="grid grid-cols-[2fr_3fr_3fr_40px] gap-2 px-3 py-2 items-center border-b last:border-b-0"
                >
                  <div className="text-sm truncate">
                    {sourceNames[sid] ?? `${sid.slice(0, 8)}… ${t("projects.detail.gateDeleted")}`}
                  </div>
                  <Input
                    className="h-8 text-sm font-mono"
                    value={versionMapping[sid]?.pattern ?? ""}
                    onChange={(e) => updateMapping(sid, "pattern", e.target.value)}
                    placeholder="^v?(.+)$"
                  />
                  <Input
                    className="h-8 text-sm font-mono"
                    value={versionMapping[sid]?.template ?? ""}
                    onChange={(e) => updateMapping(sid, "template", e.target.value)}
                    placeholder="$1"
                  />
                  <button
                    className="text-muted-foreground hover:text-foreground p-1"
                    onClick={() => removeMapping(sid)}
                  >
                    <X className="size-4" />
                  </button>
                </div>
              ))}
            </div>
          )}

          {unmappedSources.length > 0 && (
            <div className="flex items-center gap-2">
              <Select
                value={addMappingSourceId}
                onValueChange={setAddMappingSourceId}
              >
                <SelectTrigger className="w-48 h-8 text-sm">
                  <SelectValue placeholder={t("projects.detail.gateVMSource")} />
                </SelectTrigger>
                <SelectContent>
                  {unmappedSources.map((s) => (
                    <SelectItem key={s.id} value={s.id}>
                      {sourceNames[s.id]}
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
              <Button
                variant="outline"
                size="sm"
                onClick={addMapping}
                disabled={!addMappingSourceId}
              >
                <Plus className="size-3.5 mr-1" />
                {t("projects.detail.gateAddMapping")}
              </Button>
            </div>
          )}
        </div>

        {/* Action Buttons */}
        <div className="flex justify-end gap-2 pt-2 border-t">
          {gate && (
            <Button
              variant="outline"
              onClick={() => setDeleteOpen(true)}
              className="text-destructive hover:text-destructive"
            >
              {t("projects.detail.gateDelete")}
            </Button>
          )}
          <Button onClick={handleSave} disabled={saving}>
            {saving
              ? t("projects.detail.gateSaving")
              : t("projects.detail.gateSave")}
          </Button>
        </div>
      </div>

      {/* Sections 2 & 3 will be added in Tasks 5 and 6 */}

      {/* Delete Confirm Dialog */}
      <ConfirmDialog
        open={deleteOpen}
        onOpenChange={setDeleteOpen}
        title={t("projects.detail.gateDeleteConfirm")}
        description={t("projects.detail.gateDeleteConfirmDesc")}
        onConfirm={handleDelete}
      />
    </div>
  );
}
