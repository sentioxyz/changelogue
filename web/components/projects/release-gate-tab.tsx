"use client";

import { useState, useEffect, useMemo } from "react";
import useSWR from "swr";
import { gates as gatesApi } from "@/lib/api/client";
import type {
  Source,
  ReleaseGateInput,
  VersionMapping,
  VersionReadiness,
  GateEvent,
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
import { timeAgo } from "@/lib/format";
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

  // --- Version readiness data (with Load More accumulation) ---
  const [readinessPage, setReadinessPage] = useState(1);
  const [allReadiness, setAllReadiness] = useState<VersionReadiness[]>([]);
  const {
    data: readinessData,
  } = useSWR(
    gate?.enabled ? `project-${projectId}-readiness-${readinessPage}` : null,
    () => gatesApi.listReadiness(projectId, readinessPage)
  );

  // Accumulate pages
  useEffect(() => {
    if (readinessData?.data) {
      setAllReadiness((prev) =>
        readinessPage === 1 ? readinessData.data! : [...prev, ...readinessData.data!]
      );
    }
  }, [readinessData, readinessPage]);

  // Reset on gate toggle
  useEffect(() => {
    if (!gate?.enabled) {
      setAllReadiness([]);
      setReadinessPage(1);
    }
  }, [gate?.enabled]);

  const hasMoreReadiness = (readinessData?.data?.length ?? 0) === 25;

  // --- Events version filter (set by readiness table "Events" button) ---
  const [eventsVersionFilter, setEventsVersionFilter] = useState<string | null>(null);

  // --- Gate events data (with Load More accumulation) ---
  const [eventsPage, setEventsPage] = useState(1);
  const [allEvents, setAllEvents] = useState<GateEvent[]>([]);
  const { data: eventsData } = useSWR(
    gate
      ? eventsVersionFilter
        ? `project-${projectId}-gate-events-v-${eventsVersionFilter}-${eventsPage}`
        : `project-${projectId}-gate-events-${eventsPage}`
      : null,
    () =>
      eventsVersionFilter
        ? gatesApi.listEventsByVersion(projectId, eventsVersionFilter, eventsPage)
        : gatesApi.listEvents(projectId, eventsPage)
  );

  // Accumulate event pages
  useEffect(() => {
    if (eventsData?.data) {
      setAllEvents((prev) =>
        eventsPage === 1 ? eventsData.data! : [...prev, ...eventsData.data!]
      );
    }
  }, [eventsData, eventsPage]);

  // Reset events when filter changes
  useEffect(() => {
    setAllEvents([]);
    setEventsPage(1);
  }, [eventsVersionFilter]);

  const hasMoreEvents = (eventsData?.data?.length ?? 0) === 25;

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
    mutateGate();
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

  // Relative time formatter for timeout countdown
  const formatTimeRemaining = (timeoutAt: string, status: string): string => {
    if (status === "ready") return "—";
    if (status === "timed_out") return t("projects.detail.vrExpired");
    const diff = new Date(timeoutAt).getTime() - Date.now();
    if (diff <= 0) return t("projects.detail.vrExpired");
    const hours = Math.floor(diff / 3600000);
    const mins = Math.floor((diff % 3600000) / 60000);
    if (hours > 24) return `${Math.floor(hours / 24)}d ${hours % 24}h`;
    if (hours > 0) return `${hours}h ${mins}m`;
    return `${mins}m`;
  };

  const statusBadge = (status: string) => {
    switch (status) {
      case "ready":
        return (
          <span className="inline-flex items-center gap-1 px-2 py-0.5 rounded-full text-xs bg-green-500/10 text-green-500">
            {t("projects.detail.vrReady")}
          </span>
        );
      case "timed_out":
        return (
          <span className="inline-flex items-center gap-1 px-2 py-0.5 rounded-full text-xs bg-red-500/10 text-red-500">
            {t("projects.detail.vrTimedOut")}
          </span>
        );
      default:
        return (
          <span className="inline-flex items-center gap-1 px-2 py-0.5 rounded-full text-xs bg-amber-500/10 text-amber-500">
            {t("projects.detail.vrPending")}
          </span>
        );
    }
  };

  // Event dot color mapping
  const eventDotColor = (eventType: string): string => {
    switch (eventType) {
      case "gate_opened":
      case "nl_eval_passed":
        return "bg-green-500";
      case "source_met":
      case "agent_triggered":
        return "bg-blue-500";
      case "gate_timed_out":
        return "bg-amber-500";
      case "nl_eval_failed":
        return "bg-red-500";
      default:
        return "bg-muted-foreground";
    }
  };

  // Event description from event_type
  const eventDescription = (event: GateEvent): string => {
    switch (event.event_type) {
      case "gate_opened":
        return t("projects.detail.gateEventGateOpened");
      case "source_met":
        return t("projects.detail.gateEventSourceMet").replace(
          "{source}",
          event.source_id ? (sourceNames[event.source_id] ?? event.source_id.slice(0, 8)) : "unknown"
        );
      case "gate_timed_out":
        return t("projects.detail.gateEventTimedOut");
      case "nl_eval_started":
        return t("projects.detail.gateEventNLStarted");
      case "nl_eval_passed":
        return t("projects.detail.gateEventNLPassed");
      case "nl_eval_failed":
        return t("projects.detail.gateEventNLFailed");
      case "agent_triggered":
        return t("projects.detail.gateEventAgentTriggered");
      default:
        return event.event_type;
    }
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

      {/* Section 2: Version Readiness */}
      <div className="rounded-lg border p-5 bg-surface">
        <SectionLabel>{t("projects.detail.versionReadiness")}</SectionLabel>
        <p className="text-sm text-muted-foreground mt-1 mb-4">
          {t("projects.detail.versionReadinessDesc")}
        </p>

        {!gate?.enabled ? (
          <p className="text-sm text-muted-foreground italic">
            {t("projects.detail.gateDisabled")}
          </p>
        ) : allReadiness.length === 0 ? (
          <p className="text-sm text-muted-foreground italic">
            {t("projects.detail.vrEmpty")}
          </p>
        ) : (
          <div className="rounded-md border overflow-hidden">
            <div className="grid grid-cols-[1.5fr_1fr_2fr_2fr_1fr_0.5fr] gap-2 px-3 py-2 text-xs text-muted-foreground bg-muted/30 border-b">
              <div>{t("projects.detail.vrVersion")}</div>
              <div>{t("projects.detail.vrStatus")}</div>
              <div>{t("projects.detail.vrSourcesMet")}</div>
              <div>{t("projects.detail.vrSourcesMissing")}</div>
              <div>{t("projects.detail.vrTimeout")}</div>
              <div />
            </div>
            {allReadiness.map((vr) => (
              <div
                key={vr.id}
                className="grid grid-cols-[1.5fr_1fr_2fr_2fr_1fr_0.5fr] gap-2 px-3 py-2 items-center border-b last:border-b-0 text-sm"
              >
                <div className="font-medium">{vr.version}</div>
                <div>{statusBadge(vr.status)}</div>
                <div className="text-xs truncate">
                  {vr.sources_met.map((id) => sourceNames[id] ?? id.slice(0, 8)).join(", ") || "—"}
                </div>
                <div className="text-xs text-muted-foreground truncate">
                  {vr.sources_missing.map((id) => sourceNames[id] ?? id.slice(0, 8)).join(", ") || "—"}
                </div>
                <div className="text-xs">
                  {formatTimeRemaining(vr.timeout_at, vr.status)}
                </div>
                <div>
                  <Button
                    variant="ghost"
                    size="sm"
                    className="h-6 text-xs"
                    onClick={() => {
                      setEventsVersionFilter(vr.version);
                    }}
                  >
                    {t("projects.detail.vrEvents")}
                  </Button>
                </div>
              </div>
            ))}
          </div>
        )}

        {hasMoreReadiness && (
          <div className="mt-3 text-center">
            <Button
              variant="outline"
              size="sm"
              onClick={() => setReadinessPage((p) => p + 1)}
            >
              {t("projects.detail.loadMore")}
            </Button>
          </div>
        )}
      </div>

      {/* Section 3: Gate Events */}
      <div className="rounded-lg border p-5 bg-surface">
        <SectionLabel>{t("projects.detail.gateEvents")}</SectionLabel>
        <p className="text-sm text-muted-foreground mt-1 mb-4">
          {t("projects.detail.gateEventsDesc")}
        </p>

        {!gate ? (
          <p className="text-sm text-muted-foreground italic">
            {t("projects.detail.gateNoConfig")}
          </p>
        ) : allEvents.length === 0 ? (
          <p className="text-sm text-muted-foreground italic">
            {t("projects.detail.gateEventsEmpty")}
          </p>
        ) : (
          <div className="flex flex-col">
            {eventsVersionFilter && (
              <div className="flex items-center gap-2 mb-2 text-sm text-muted-foreground">
                <span>{t("projects.detail.gateEventsFiltered").replace("{version}", eventsVersionFilter)}</span>
                <Button
                  variant="ghost"
                  size="sm"
                  className="h-5 px-1"
                  onClick={() => setEventsVersionFilter(null)}
                >
                  <X className="size-3" />
                </Button>
              </div>
            )}
            {allEvents.map((ev) => (
              <div
                key={ev.id}
                className="flex gap-3 py-2.5 border-b last:border-b-0 items-start"
              >
                <div
                  className={`size-2 rounded-full mt-1.5 shrink-0 ${eventDotColor(ev.event_type)}`}
                />
                <div className="flex-1 min-w-0">
                  <div className="text-sm">
                    <span className="font-medium">{ev.version}</span>
                    {" — "}
                    {eventDescription(ev)}
                  </div>
                  <div className="text-xs text-muted-foreground mt-0.5">
                    {ev.event_type} • {timeAgo(ev.created_at)}
                  </div>
                </div>
              </div>
            ))}
          </div>
        )}

        {hasMoreEvents && (
          <div className="mt-3 text-center">
            <Button
              variant="outline"
              size="sm"
              onClick={() => setEventsPage((p) => p + 1)}
            >
              {t("projects.detail.loadMore")}
            </Button>
          </div>
        )}
      </div>

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
