# Release Gate UX Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add a "Release Gate" tab to the project detail page with gate configuration, version readiness monitoring, and gate events timeline.

**Architecture:** A new `<ReleaseGateTab>` component renders inside the project detail page as a 4th tab. It receives `projectId` and `sources` as props, manages its own SWR data fetching and form state. The API client gets a `gates` namespace, and TypeScript types + i18n keys are added for all gate-related models.

**Tech Stack:** Next.js (React 19), TypeScript, SWR v2, Radix UI (Switch, Checkbox, Select), Tailwind CSS v4, i18n (en/zh)

---

## File Structure

| File | Action | Responsibility |
|------|--------|----------------|
| `web/lib/api/types.ts` | Modify | Add `VersionMapping`, `ReleaseGate`, `ReleaseGateInput`, `VersionReadiness`, `GateEvent` |
| `web/lib/api/client.ts` | Modify | Add `gates` namespace (7 methods, `get` uses `fetch` directly for 404 handling) |
| `web/lib/i18n/messages/en.json` | Modify | Add ~40 gate-related i18n keys |
| `web/lib/i18n/messages/zh.json` | Modify | Add corresponding Chinese translations |
| `web/components/projects/release-gate-tab.tsx` | Create | Gate config form + version readiness table + events timeline |
| `web/components/projects/project-detail.tsx` | Modify | Add `"gates"` tab key, import + render `<ReleaseGateTab>` |

---

### Task 1: TypeScript Types

**Files:**
- Modify: `web/lib/api/types.ts:331` (append after `RepoItem`)

- [ ] **Step 1: Add gate-related TypeScript interfaces**

Append these interfaces to the end of `web/lib/api/types.ts`, before the final empty line:

```typescript
// --- Release Gate Types ---

export interface VersionMapping {
  pattern: string;
  template: string;
}

export interface ReleaseGate {
  id: string;
  project_id: string;
  required_sources?: string[];
  timeout_hours: number;
  version_mapping?: Record<string, VersionMapping>;
  nl_rule?: string;
  enabled: boolean;
  created_at: string;
  updated_at: string;
}

export interface ReleaseGateInput {
  required_sources?: string[];
  timeout_hours: number;
  version_mapping?: Record<string, VersionMapping>;
  nl_rule?: string;
  enabled: boolean;
}

export interface VersionReadiness {
  id: string;
  project_id: string;
  version: string;
  status: "pending" | "ready" | "timed_out";
  sources_met: string[];
  sources_missing: string[];
  nl_rule_passed?: boolean;
  timeout_at: string;
  opened_at?: string;
  agent_triggered: boolean;
  created_at: string;
  updated_at: string;
}

export interface GateEvent {
  id: string;
  version_readiness_id: string;
  project_id: string;
  version: string;
  event_type: string;
  source_id?: string;
  details?: Record<string, unknown>;
  created_at: string;
}
```

- [ ] **Step 2: Verify TypeScript compiles**

Run: `cd web && npx tsc --noEmit --pretty 2>&1 | head -20`
Expected: No errors related to the new types.

- [ ] **Step 3: Commit**

```bash
git add web/lib/api/types.ts
git commit -m "feat(web): add Release Gate TypeScript types"
```

---

### Task 2: API Client

**Files:**
- Modify: `web/lib/api/client.ts:1-2` (add import for new types)
- Modify: `web/lib/api/client.ts:293` (append `gates` namespace after `onboard`)

- [ ] **Step 1: Add gate type imports**

In `web/lib/api/client.ts`, add these to the existing import block from `"./types"` (line 2):

```typescript
  ReleaseGate,
  ReleaseGateInput,
  VersionReadiness,
  GateEvent,
```

- [ ] **Step 2: Add the `gates` namespace**

Append the following after the `onboard` namespace (after line 292), before the final empty line:

```typescript
// --- Release Gates ---

export const gates = {
  get: async (projectId: string): Promise<ApiResponse<ReleaseGate | null>> => {
    const res = await fetch(`${BASE}/projects/${projectId}/release-gate`, {
      headers: { "Content-Type": "application/json" },
    });
    if (res.status === 404) {
      return { data: null };
    }
    if (!res.ok) {
      const body = await res.json().catch(() => null);
      throw new Error(body?.error?.message ?? `Request failed: ${res.status}`);
    }
    return res.json();
  },
  upsert: (projectId: string, input: ReleaseGateInput) =>
    request<ApiResponse<ReleaseGate>>(`/projects/${projectId}/release-gate`, {
      method: "PUT",
      body: JSON.stringify(input),
    }),
  delete: (projectId: string) =>
    request<ApiResponse<null>>(`/projects/${projectId}/release-gate`, {
      method: "DELETE",
    }),
  listReadiness: (projectId: string, page = 1, perPage = 25) =>
    request<ApiResponse<VersionReadiness[]>>(
      `/projects/${projectId}/version-readiness?page=${page}&per_page=${perPage}`
    ),
  getReadiness: (projectId: string, version: string) =>
    request<ApiResponse<VersionReadiness>>(
      `/projects/${projectId}/version-readiness/${encodeURIComponent(version)}`
    ),
  listEvents: (projectId: string, page = 1, perPage = 25) =>
    request<ApiResponse<GateEvent[]>>(
      `/projects/${projectId}/gate-events?page=${page}&per_page=${perPage}`
    ),
  listEventsByVersion: (projectId: string, version: string, page = 1, perPage = 25) =>
    request<ApiResponse<GateEvent[]>>(
      `/projects/${projectId}/version-readiness/${encodeURIComponent(version)}/events?page=${page}&per_page=${perPage}`
    ),
};
```

**Important:** The `gates.get` method uses `fetch` directly (not the shared `request()` helper) so it can check `res.status === 404` and return `{ data: null }` instead of throwing. Use the existing `BASE` constant already defined on line 31 of client.ts — rename `BASE_URL` to `BASE` in the code above (the `BASE` const already exists, so don't redeclare it). Just reference it directly: `` `${BASE}/projects/${projectId}/release-gate` ``.

- [ ] **Step 3: Verify TypeScript compiles**

Run: `cd web && npx tsc --noEmit --pretty 2>&1 | head -20`
Expected: No errors.

- [ ] **Step 4: Commit**

```bash
git add web/lib/api/client.ts
git commit -m "feat(web): add gates API client namespace"
```

---

### Task 3: i18n Keys

**Files:**
- Modify: `web/lib/i18n/messages/en.json:315` (insert after last `projects.detail.*` key)
- Modify: `web/lib/i18n/messages/zh.json:315` (insert after last `projects.detail.*` key)

- [ ] **Step 1: Add English i18n keys**

In `web/lib/i18n/messages/en.json`, add the following keys after `"projects.detail.dialogDeleteProjectDesc"` (line 315) and before the `"subscriptions.*"` section (line 317):

```json
  "projects.detail.tabGates": "Release Gate",
  "projects.detail.gateConfig": "Gate Configuration",
  "projects.detail.gateConfigDesc": "Delay agent analysis until all required sources release the same version.",
  "projects.detail.gateEnabled": "Enabled",
  "projects.detail.gateRequiredSources": "Required Sources",
  "projects.detail.gateRequiredSourcesHint": "Select which sources must all release a version before the agent runs. Leave empty to require all sources.",
  "projects.detail.gateTimeoutHours": "Timeout (hours)",
  "projects.detail.gateTimeoutHoursHint": "How long to wait for all sources before timing out.",
  "projects.detail.gateNLRule": "Natural Language Rule",
  "projects.detail.gateNLRuleOptional": "(optional)",
  "projects.detail.gateNLRuleHint": "Extra constraint evaluated by AI. E.g., \"Only proceed if the Docker image tag is a stable release.\"",
  "projects.detail.gateVersionMapping": "Version Mapping",
  "projects.detail.gateVersionMappingHint": "Define how to normalize version strings per source for comparison.",
  "projects.detail.gateVMSource": "Source",
  "projects.detail.gateVMPattern": "Pattern (regex)",
  "projects.detail.gateVMTemplate": "Template",
  "projects.detail.gateAddMapping": "Add Mapping",
  "projects.detail.gateSave": "Save Configuration",
  "projects.detail.gateSaving": "Saving...",
  "projects.detail.gateDelete": "Delete Gate",
  "projects.detail.gateDeleteConfirm": "Delete Release Gate",
  "projects.detail.gateDeleteConfirmDesc": "This will remove the release gate configuration. Version readiness data will be preserved.",
  "projects.detail.gateNoConfig": "No release gate configured. Save to create one.",
  "projects.detail.gateDisabled": "Gate is disabled.",
  "projects.detail.gateNoSources": "Add sources to configure gate requirements.",
  "projects.detail.versionReadiness": "Version Readiness",
  "projects.detail.versionReadinessDesc": "Track which versions are waiting for sources and which are ready.",
  "projects.detail.vrVersion": "Version",
  "projects.detail.vrStatus": "Status",
  "projects.detail.vrSourcesMet": "Sources Met",
  "projects.detail.vrSourcesMissing": "Sources Missing",
  "projects.detail.vrTimeout": "Timeout",
  "projects.detail.vrPending": "pending",
  "projects.detail.vrReady": "ready",
  "projects.detail.vrTimedOut": "timed out",
  "projects.detail.vrExpired": "expired",
  "projects.detail.vrEvents": "Events",
  "projects.detail.vrEmpty": "No versions tracked yet.",
  "projects.detail.gateEvents": "Gate Events",
  "projects.detail.gateEventsDesc": "Audit log of gate activity across all versions.",
  "projects.detail.gateEventsEmpty": "No gate events yet.",
  "projects.detail.gateEventGateOpened": "Gate opened: all sources ready",
  "projects.detail.gateEventSourceMet": "Source released: {source}",
  "projects.detail.gateEventTimedOut": "Gate timed out",
  "projects.detail.gateEventNLStarted": "NL rule evaluation started",
  "projects.detail.gateEventNLPassed": "NL rule passed",
  "projects.detail.gateEventNLFailed": "NL rule failed",
  "projects.detail.gateEventAgentTriggered": "Agent analysis triggered",
  "projects.detail.gateDeleted": "(deleted)",
  "projects.detail.loadMore": "Load more",
```

- [ ] **Step 2: Add Chinese i18n keys**

In `web/lib/i18n/messages/zh.json`, add the following keys after `"projects.detail.dialogDeleteProjectDesc"` (line 315) and before `"subscriptions.*"`:

```json
  "projects.detail.tabGates": "发布门控",
  "projects.detail.gateConfig": "门控配置",
  "projects.detail.gateConfigDesc": "延迟代理分析，直到所有必需的来源发布相同版本。",
  "projects.detail.gateEnabled": "已启用",
  "projects.detail.gateRequiredSources": "必需来源",
  "projects.detail.gateRequiredSourcesHint": "选择哪些来源必须都发布某个版本后才运行代理。留空表示需要所有来源。",
  "projects.detail.gateTimeoutHours": "超时（小时）",
  "projects.detail.gateTimeoutHoursHint": "等待所有来源发布的最长时间。",
  "projects.detail.gateNLRule": "自然语言规则",
  "projects.detail.gateNLRuleOptional": "（可选）",
  "projects.detail.gateNLRuleHint": "由 AI 评估的额外约束。例如：\"仅在 Docker 镜像标签是稳定版本时继续。\"",
  "projects.detail.gateVersionMapping": "版本映射",
  "projects.detail.gateVersionMappingHint": "定义如何规范化每个来源的版本字符串以便比较。",
  "projects.detail.gateVMSource": "来源",
  "projects.detail.gateVMPattern": "匹配模式（正则）",
  "projects.detail.gateVMTemplate": "模板",
  "projects.detail.gateAddMapping": "添加映射",
  "projects.detail.gateSave": "保存配置",
  "projects.detail.gateSaving": "保存中...",
  "projects.detail.gateDelete": "删除门控",
  "projects.detail.gateDeleteConfirm": "删除发布门控",
  "projects.detail.gateDeleteConfirmDesc": "此操作将移除发布门控配置。版本就绪数据将保留。",
  "projects.detail.gateNoConfig": "尚未配置发布门控。保存以创建。",
  "projects.detail.gateDisabled": "门控已禁用。",
  "projects.detail.gateNoSources": "请先添加来源以配置门控需求。",
  "projects.detail.versionReadiness": "版本就绪状态",
  "projects.detail.versionReadinessDesc": "跟踪哪些版本正在等待来源，哪些已就绪。",
  "projects.detail.vrVersion": "版本",
  "projects.detail.vrStatus": "状态",
  "projects.detail.vrSourcesMet": "已就绪来源",
  "projects.detail.vrSourcesMissing": "缺少来源",
  "projects.detail.vrTimeout": "超时",
  "projects.detail.vrPending": "等待中",
  "projects.detail.vrReady": "就绪",
  "projects.detail.vrTimedOut": "已超时",
  "projects.detail.vrExpired": "已过期",
  "projects.detail.vrEvents": "事件",
  "projects.detail.vrEmpty": "暂无跟踪版本。",
  "projects.detail.gateEvents": "门控事件",
  "projects.detail.gateEventsDesc": "所有版本的门控活动审计日志。",
  "projects.detail.gateEventsEmpty": "暂无门控事件。",
  "projects.detail.gateEventGateOpened": "门控开启：所有来源已就绪",
  "projects.detail.gateEventSourceMet": "来源已发布：{source}",
  "projects.detail.gateEventTimedOut": "门控已超时",
  "projects.detail.gateEventNLStarted": "自然语言规则评估已开始",
  "projects.detail.gateEventNLPassed": "自然语言规则已通过",
  "projects.detail.gateEventNLFailed": "自然语言规则未通过",
  "projects.detail.gateEventAgentTriggered": "代理分析已触发",
  "projects.detail.gateDeleted": "（已删除）",
  "projects.detail.loadMore": "加载更多",
```

- [ ] **Step 3: Verify JSON is valid**

Run: `node -e "JSON.parse(require('fs').readFileSync('web/lib/i18n/messages/en.json','utf8')); console.log('en.json OK')" && node -e "JSON.parse(require('fs').readFileSync('web/lib/i18n/messages/zh.json','utf8')); console.log('zh.json OK')"`
Expected: Both print OK.

- [ ] **Step 4: Commit**

```bash
git add web/lib/i18n/messages/en.json web/lib/i18n/messages/zh.json
git commit -m "feat(web): add Release Gate i18n keys (en + zh)"
```

---

### Task 4: ReleaseGateTab Component — Gate Configuration Section

This is the main new component. It's the largest task, so we build it incrementally: first the config card, then readiness table, then events timeline.

**Files:**
- Create: `web/components/projects/release-gate-tab.tsx`

- [ ] **Step 1: Create the component with gate config card**

Create `web/components/projects/release-gate-tab.tsx` with the following content:

```tsx
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
```

- [ ] **Step 2: Verify TypeScript compiles**

Run: `cd web && npx tsc --noEmit --pretty 2>&1 | head -30`
Expected: No errors. (The component is not yet rendered anywhere — this just checks the file compiles.)

- [ ] **Step 3: Commit**

```bash
git add web/components/projects/release-gate-tab.tsx
git commit -m "feat(web): add ReleaseGateTab component with gate config card"
```

---

### Task 5: ReleaseGateTab — Version Readiness Table

**Files:**
- Modify: `web/components/projects/release-gate-tab.tsx` (add Section 2)

- [ ] **Step 1: Add the version readiness table section**

In `web/components/projects/release-gate-tab.tsx`, add `VersionReadiness` to the existing type import from `"@/lib/api/types"` (merge into the block added in Task 4):

```tsx
import type {
  Source,
  ReleaseGate,
  ReleaseGateInput,
  VersionMapping,
  VersionReadiness,
} from "@/lib/api/types";
```

Add this state and SWR hook inside the component function, after the existing `mutateGate` hook:

```tsx
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
```

Add this helper function inside the component, after the existing handler functions:

```tsx
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
```

Replace the `{/* Sections 2 & 3 will be added in Tasks 5 and 6 */}` comment with:

```tsx
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

      {/* Section 3 placeholder — will be added in Task 6 */}
```

- [ ] **Step 2: Verify TypeScript compiles**

Run: `cd web && npx tsc --noEmit --pretty 2>&1 | head -20`
Expected: No errors.

- [ ] **Step 3: Commit**

```bash
git add web/components/projects/release-gate-tab.tsx
git commit -m "feat(web): add version readiness table to ReleaseGateTab"
```

---

### Task 6: ReleaseGateTab — Gate Events Timeline

**Files:**
- Modify: `web/components/projects/release-gate-tab.tsx` (add Section 3)

- [ ] **Step 1: Add gate events timeline section**

Add `GateEvent` to the existing type import from `"@/lib/api/types"` (merge into the block, which now includes `VersionReadiness` from Task 5):

```tsx
import type {
  Source,
  ReleaseGate,
  ReleaseGateInput,
  VersionMapping,
  VersionReadiness,
  GateEvent,
} from "@/lib/api/types";
```

Add this SWR hook with pagination accumulation inside the component function, after the readiness hook:

```tsx
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
```

Add this helper function inside the component:

```tsx
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

  // Relative timestamp
  const relativeTime = (iso: string): string => {
    const diff = Date.now() - new Date(iso).getTime();
    const mins = Math.floor(diff / 60000);
    if (mins < 1) return "just now";
    if (mins < 60) return `${mins}m ago`;
    const hours = Math.floor(mins / 60);
    if (hours < 24) return `${hours}h ago`;
    const days = Math.floor(hours / 24);
    return `${days}d ago`;
  };
```

Replace the `{/* Section 3 placeholder — will be added in Task 6 */}` comment with:

```tsx
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
                <span>Filtered: {eventsVersionFilter}</span>
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
                    {ev.event_type} • {relativeTime(ev.created_at)}
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
```

- [ ] **Step 2: Verify TypeScript compiles**

Run: `cd web && npx tsc --noEmit --pretty 2>&1 | head -20`
Expected: No errors.

- [ ] **Step 3: Commit**

```bash
git add web/components/projects/release-gate-tab.tsx
git commit -m "feat(web): add gate events timeline to ReleaseGateTab"
```

---

### Task 7: Integrate Tab in project-detail.tsx

**Files:**
- Modify: `web/components/projects/project-detail.tsx:13` (add import)
- Modify: `web/components/projects/project-detail.tsx:30` (add tab key)
- Modify: `web/components/projects/project-detail.tsx:58-62` (add tab entry)
- Modify: `web/components/projects/project-detail.tsx` (add tab content render)

- [ ] **Step 1: Add import**

In `web/components/projects/project-detail.tsx`, add this import after the existing component imports (around line 18):

```tsx
import { ReleaseGateTab } from "./release-gate-tab";
```

- [ ] **Step 2: Extend TabKey type**

Change line 30 from:
```tsx
type TabKey = "sources" | "context" | "agent";
```
to:
```tsx
type TabKey = "sources" | "context" | "agent" | "gates";
```

- [ ] **Step 3: Add tab to tabs array**

In the `tabs` array (around lines 58-62), add the gates tab after the agent tab:

```tsx
    { key: "gates" as TabKey, label: t("projects.detail.tabGates") },
```

So the full array becomes:
```tsx
  const tabs = [
    { key: "sources" as TabKey, label: t("projects.detail.tabSources") },
    { key: "context" as TabKey, label: t("projects.detail.tabContext") },
    { key: "agent" as TabKey, label: t("projects.detail.tabAgent") },
    { key: "gates" as TabKey, label: t("projects.detail.tabGates") },
  ];
```

- [ ] **Step 4: Add tab content render**

Find the last tab content block (the `{activeTab === "agent" && (` block). After its closing `)}`, add:

```tsx
        {activeTab === "gates" && (
          <ReleaseGateTab
            projectId={id}
            sources={sourcesData?.data ?? []}
          />
        )}
```

- [ ] **Step 5: Verify TypeScript compiles**

Run: `cd web && npx tsc --noEmit --pretty 2>&1 | head -20`
Expected: No errors.

- [ ] **Step 6: Verify the dev server renders correctly**

Run: `cd web && npm run build 2>&1 | tail -20`
Expected: Build succeeds without errors.

- [ ] **Step 7: Commit**

```bash
git add web/components/projects/project-detail.tsx
git commit -m "feat(web): integrate Release Gate tab in project detail page"
```

---

### Task 8: Visual Smoke Test

**Files:** None (read-only verification)

- [ ] **Step 1: Start the dev environment**

Run: `make frontend-dev` in one terminal and (if not already running) `make dev` for the backend.

- [ ] **Step 2: Navigate to a project detail page**

Open a project in the browser. Verify:
- The "Release Gate" tab appears as the 4th tab
- Clicking it shows the gate configuration card with all form fields
- The enabled switch, required sources checkboxes, timeout input, NL rule textarea, and version mapping table all render correctly
- The version readiness section shows "Gate is disabled." when the toggle is off
- The gate events section shows "No release gate configured." when no gate exists

- [ ] **Step 3: Test save and delete flow**

- Toggle the gate to enabled, set a timeout, click Save — verify no JS errors
- After saving, verify the form persists data on tab switch and return
- Click Delete Gate — verify confirm dialog appears
- Confirm delete — verify the form resets to defaults

- [ ] **Step 4: Verify i18n**

Switch language to Chinese in settings. Verify all gate tab labels display in Chinese.
