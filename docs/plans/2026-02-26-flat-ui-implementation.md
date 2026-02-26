# Flat UI Redesign Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Replace all standalone create/edit/delete pages with Dialog-based modals on their parent list pages, flattening the URL structure.

**Architecture:** Each list page manages its own dialog state (`createOpen`, `editingId`, `deletingId`) via `useState`. Existing form components are refactored to remove `<Card>` wrappers and accept `onSuccess`/`onCancel` callbacks instead of using `router.push`. A shared `ConfirmDialog` component handles all delete confirmations.

**Tech Stack:** React, shadcn/ui Dialog (`@/components/ui/dialog`), SWR for cache invalidation

---

### Task 1: Create ConfirmDialog Component

**Files:**
- Create: `web/components/ui/confirm-dialog.tsx`

**Step 1: Create the component**

```tsx
"use client";

import { useState } from "react";
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
  DialogDescription,
  DialogFooter,
} from "@/components/ui/dialog";
import { Button } from "@/components/ui/button";

interface ConfirmDialogProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  title: string;
  description: string;
  onConfirm: () => Promise<void>;
  confirmLabel?: string;
}

export function ConfirmDialog({
  open,
  onOpenChange,
  title,
  description,
  onConfirm,
  confirmLabel = "Delete",
}: ConfirmDialogProps) {
  const [loading, setLoading] = useState(false);

  const handleConfirm = async () => {
    setLoading(true);
    try {
      await onConfirm();
      onOpenChange(false);
    } finally {
      setLoading(false);
    }
  };

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="sm:max-w-md">
        <DialogHeader>
          <DialogTitle>{title}</DialogTitle>
          <DialogDescription>{description}</DialogDescription>
        </DialogHeader>
        <DialogFooter>
          <Button variant="outline" onClick={() => onOpenChange(false)} disabled={loading}>
            Cancel
          </Button>
          <Button variant="destructive" onClick={handleConfirm} disabled={loading}>
            {loading ? "Deleting..." : confirmLabel}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
```

**Step 2: Verify it compiles**

Run: `cd web && npx next build 2>&1 | head -20` or just `npx tsc --noEmit`
Expected: No type errors

**Step 3: Commit**

```bash
git add web/components/ui/confirm-dialog.tsx
git commit -m "feat(web): add reusable ConfirmDialog component"
```

---

### Task 2: Refactor ProjectForm to Support Dialog Mode

The form currently wraps itself in a `<Card>` and calls `router.push` after save. We need it to optionally work without the Card wrapper and call an `onSuccess` callback instead.

**Files:**
- Modify: `web/components/projects/project-form.tsx`

**Step 1: Refactor ProjectForm**

Changes:
1. Add optional `onSuccess` and `onCancel` callbacks
2. When `onSuccess` is provided, call it instead of `router.push`
3. When `onCancel` is provided, call it instead of `router.back()`
4. Add optional `variant` prop: `"page"` (default, with Card) or `"dialog"` (no Card)

The refactored form:

```tsx
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
  hideSource?: boolean;
  onSuccess?: () => void;
  onCancel?: () => void;
}

export function ProjectForm({ initial, onSubmit, title, hideSource, onSuccess, onCancel }: ProjectFormProps) {
  const router = useRouter();
  const [saving, setSaving] = useState(false);
  const [name, setName] = useState(initial?.name ?? "");
  const [description, setDescription] = useState(initial?.description ?? "");
  const [error, setError] = useState("");

  const [showSource, setShowSource] = useState(false);
  const [provider, setProvider] = useState("github");
  const [repository, setRepository] = useState("");
  const [pollInterval, setPollInterval] = useState("300");

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    setError("");
    setSaving(true);
    try {
      const result: ProjectFormResult = {
        project: { name, description: description || undefined },
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

  const handleCancel = () => {
    if (onCancel) {
      onCancel();
    } else {
      router.back();
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

  // Dialog mode: return form without Card wrapper
  if (onSuccess) {
    return formContent;
  }

  // Page mode: wrap in Card (fallback for any remaining page usage)
  return (
    <Card className="mx-auto max-w-2xl">
      <CardHeader><CardTitle>{title}</CardTitle></CardHeader>
      <CardContent>{formContent}</CardContent>
    </Card>
  );
}
```

**Step 2: Verify it compiles**

Run: `cd web && npx tsc --noEmit`

**Step 3: Commit**

```bash
git add web/components/projects/project-form.tsx
git commit -m "refactor(web): add dialog mode support to ProjectForm"
```

---

### Task 3: Refactor SourceForm to Support Dialog Mode

Same pattern as ProjectForm.

**Files:**
- Modify: `web/components/sources/source-form.tsx`

**Step 1: Refactor SourceForm**

Add `onSuccess` and `onCancel` optional callbacks. When `onSuccess` is provided, skip the Card wrapper and call callbacks instead of router navigation.

```tsx
"use client";

import { useState } from "react";
import { useRouter } from "next/navigation";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Switch } from "@/components/ui/switch";
import { Textarea } from "@/components/ui/textarea";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import type { Source, SourceInput } from "@/lib/api/types";

interface SourceFormProps {
  initial?: Source;
  projectId?: string;
  onSubmit: (input: SourceInput) => Promise<void>;
  title: string;
  redirectTo?: string;
  onSuccess?: () => void;
  onCancel?: () => void;
}

export function SourceForm({ initial, onSubmit, title, redirectTo, onSuccess, onCancel }: SourceFormProps) {
  const router = useRouter();
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState("");
  const [provider, setProvider] = useState(initial?.provider ?? "dockerhub");
  const [repository, setRepository] = useState(initial?.repository ?? "");
  const [pollInterval, setPollInterval] = useState(String(initial?.poll_interval_seconds ?? 300));
  const [enabled, setEnabled] = useState(initial?.enabled ?? true);
  const [configJson, setConfigJson] = useState(
    JSON.stringify(initial?.config ?? {}, null, 2)
  );

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    setError("");

    let parsedConfig: Record<string, unknown> | undefined;
    if (configJson.trim() && configJson.trim() !== "{}") {
      try {
        parsedConfig = JSON.parse(configJson);
      } catch {
        setError("Config must be valid JSON");
        return;
      }
    }

    setSaving(true);
    try {
      await onSubmit({
        provider,
        repository,
        poll_interval_seconds: Number(pollInterval),
        enabled,
        config: parsedConfig,
      });
      if (onSuccess) {
        onSuccess();
      } else {
        router.push(redirectTo ?? "/sources");
      }
    } catch (err: unknown) {
      setError(err instanceof Error ? err.message : "Failed to save");
    } finally {
      setSaving(false);
    }
  };

  const handleCancel = () => {
    if (onCancel) {
      onCancel();
    } else {
      router.back();
    }
  };

  const formContent = (
    <form onSubmit={handleSubmit} className="space-y-4">
      {error && <div className="rounded-md bg-red-50 p-3 text-sm text-red-700">{error}</div>}
      <div className="space-y-2">
        <Label>Provider</Label>
        <Select value={provider} onValueChange={setProvider}>
          <SelectTrigger><SelectValue /></SelectTrigger>
          <SelectContent>
            <SelectItem value="dockerhub">Docker Hub</SelectItem>
            <SelectItem value="github">GitHub</SelectItem>
          </SelectContent>
        </Select>
      </div>
      <div className="space-y-2">
        <Label htmlFor="repository">Repository</Label>
        <Input id="repository" value={repository} onChange={(e) => setRepository(e.target.value)} placeholder="e.g. library/golang or ethereum/go-ethereum" required />
      </div>
      <div className="space-y-2">
        <Label htmlFor="poll_interval">Poll Interval (seconds)</Label>
        <Input id="poll_interval" type="number" min={60} value={pollInterval} onChange={(e) => setPollInterval(e.target.value)} />
      </div>
      <div className="space-y-2">
        <Label htmlFor="config">Config (JSON, optional)</Label>
        <Textarea id="config" value={configJson} onChange={(e) => setConfigJson(e.target.value)} rows={4} className="font-mono text-sm" placeholder="{}" />
      </div>
      <div className="flex items-center gap-3">
        <Switch checked={enabled} onCheckedChange={setEnabled} />
        <Label>Enabled</Label>
      </div>
      <div className="flex justify-end gap-2">
        <Button type="button" variant="outline" onClick={handleCancel}>Cancel</Button>
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
      <CardContent>{formContent}</CardContent>
    </Card>
  );
}
```

**Step 2: Verify it compiles**

Run: `cd web && npx tsc --noEmit`

**Step 3: Commit**

```bash
git add web/components/sources/source-form.tsx
git commit -m "refactor(web): add dialog mode support to SourceForm"
```

---

### Task 4: Refactor ChannelForm to Support Dialog Mode

**Files:**
- Modify: `web/components/channels/channel-form.tsx`

**Step 1: Refactor ChannelForm**

Same pattern: add `onSuccess`/`onCancel`, conditionally wrap in Card.

```tsx
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
  onSuccess?: () => void;
  onCancel?: () => void;
}

export function ChannelForm({ initial, onSubmit, title, onSuccess, onCancel }: ChannelFormProps) {
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
      if (onSuccess) {
        onSuccess();
      } else {
        router.push("/channels");
      }
    } catch (err: unknown) {
      setError(err instanceof Error ? err.message : "Failed to save");
    } finally {
      setSaving(false);
    }
  };

  const handleCancel = () => {
    if (onCancel) {
      onCancel();
    } else {
      router.back();
    }
  };

  const formContent = (
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
        <Button type="button" variant="outline" onClick={handleCancel}>Cancel</Button>
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
      <CardContent>{formContent}</CardContent>
    </Card>
  );
}
```

**Step 2: Verify, then commit**

```bash
git add web/components/channels/channel-form.tsx
git commit -m "refactor(web): add dialog mode support to ChannelForm"
```

---

### Task 5: Refactor SubscriptionForm to Support Dialog Mode

**Files:**
- Modify: `web/components/subscriptions/subscription-form.tsx`

**Step 1: Refactor SubscriptionForm**

Same pattern: add `onSuccess`/`onCancel`, conditionally wrap in Card.

```tsx
"use client";

import { useState } from "react";
import { useRouter } from "next/navigation";
import useSWR from "swr";
import { projects as projectsApi, channels as channelsApi, sources as sourcesApi } from "@/lib/api/client";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import type { Subscription, SubscriptionInput } from "@/lib/api/types";

interface SubscriptionFormProps {
  initial?: Subscription;
  onSubmit: (input: SubscriptionInput) => Promise<void>;
  title: string;
  onSuccess?: () => void;
  onCancel?: () => void;
}

export function SubscriptionForm({ initial, onSubmit, title, onSuccess, onCancel }: SubscriptionFormProps) {
  const router = useRouter();
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState("");
  const [type, setType] = useState<"source" | "project">(initial?.type ?? "project");
  const [channelId, setChannelId] = useState(initial?.channel_id ?? "");
  const [projectId, setProjectId] = useState(initial?.project_id ?? "");
  const [sourceId, setSourceId] = useState(initial?.source_id ?? "");
  const [versionFilter, setVersionFilter] = useState(initial?.version_filter ?? "");

  const { data: projectsData } = useSWR("projects-for-sub", () => projectsApi.list());
  const { data: channelsData } = useSWR("channels-for-sub", () => channelsApi.list());
  const { data: sourcesData } = useSWR(
    type === "source" && projectId ? `sources-for-sub-${projectId}` : null,
    () => sourcesApi.listByProject(projectId)
  );

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    setError("");
    setSaving(true);
    try {
      await onSubmit({
        channel_id: channelId,
        type,
        source_id: type === "source" ? sourceId : undefined,
        project_id: type === "project" ? projectId : undefined,
        version_filter: versionFilter || undefined,
      });
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

  const handleCancel = () => {
    if (onCancel) {
      onCancel();
    } else {
      router.back();
    }
  };

  const formContent = (
    <form onSubmit={handleSubmit} className="space-y-4">
      {error && <div className="rounded-md bg-red-50 p-3 text-sm text-red-700">{error}</div>}
      <div className="space-y-2">
        <Label>Subscription Type</Label>
        <Select value={type} onValueChange={(v) => setType(v as "source" | "project")}>
          <SelectTrigger><SelectValue /></SelectTrigger>
          <SelectContent>
            <SelectItem value="project">Project</SelectItem>
            <SelectItem value="source">Source</SelectItem>
          </SelectContent>
        </Select>
      </div>
      {type === "project" && (
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
      {type === "source" && (
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
        <Button type="button" variant="outline" onClick={handleCancel}>Cancel</Button>
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
      <CardContent>{formContent}</CardContent>
    </Card>
  );
}
```

**Step 2: Verify, then commit**

```bash
git add web/components/subscriptions/subscription-form.tsx
git commit -m "refactor(web): add dialog mode support to SubscriptionForm"
```

---

### Task 6: Refactor NewContextSourceForm to Support Dialog Mode

**Files:**
- Modify: `web/components/context-sources/new-context-source-form.tsx`

**Step 1: Refactor the component**

Add `onSuccess`/`onCancel` callbacks. When `onSuccess` is provided, call it instead of `router.push` and render without Card wrapper.

```tsx
"use client";

import { useState } from "react";
import { useRouter } from "next/navigation";
import { contextSources as ctxApi } from "@/lib/api/client";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Textarea } from "@/components/ui/textarea";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";

interface NewContextSourceFormProps {
  projectId: string;
  onSuccess?: () => void;
  onCancel?: () => void;
}

export function NewContextSourceForm({ projectId, onSuccess, onCancel }: NewContextSourceFormProps) {
  const router = useRouter();
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState("");
  const [type, setType] = useState("documentation");
  const [name, setName] = useState("");
  const [configJson, setConfigJson] = useState("{}");

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    setError("");

    let parsedConfig: Record<string, unknown>;
    try {
      parsedConfig = JSON.parse(configJson);
    } catch {
      setError("Config must be valid JSON");
      return;
    }

    setSaving(true);
    try {
      await ctxApi.create(projectId, { type, name, config: parsedConfig });
      if (onSuccess) {
        onSuccess();
      } else {
        router.push(`/projects/${projectId}`);
      }
    } catch (err: unknown) {
      setError(err instanceof Error ? err.message : "Failed to save");
    } finally {
      setSaving(false);
    }
  };

  const handleCancel = () => {
    if (onCancel) {
      onCancel();
    } else {
      router.back();
    }
  };

  const formContent = (
    <form onSubmit={handleSubmit} className="space-y-4">
      {error && <div className="rounded-md bg-red-50 p-3 text-sm text-red-700">{error}</div>}
      <div className="space-y-2">
        <Label htmlFor="name">Name</Label>
        <Input id="name" value={name} onChange={(e) => setName(e.target.value)} placeholder="e.g. Go Release Notes" required />
      </div>
      <div className="space-y-2">
        <Label>Type</Label>
        <Select value={type} onValueChange={setType}>
          <SelectTrigger><SelectValue /></SelectTrigger>
          <SelectContent>
            <SelectItem value="documentation">Documentation</SelectItem>
            <SelectItem value="changelog">Changelog</SelectItem>
            <SelectItem value="github_issues">GitHub Issues</SelectItem>
            <SelectItem value="custom">Custom</SelectItem>
          </SelectContent>
        </Select>
      </div>
      <div className="space-y-2">
        <Label htmlFor="config">Config (JSON)</Label>
        <Textarea
          id="config"
          value={configJson}
          onChange={(e) => setConfigJson(e.target.value)}
          rows={6}
          className="font-mono text-sm"
          placeholder='{"url": "https://..."}'
        />
      </div>
      <div className="flex justify-end gap-2">
        <Button type="button" variant="outline" onClick={handleCancel}>Cancel</Button>
        <Button type="submit" disabled={saving}>{saving ? "Saving..." : "Save"}</Button>
      </div>
    </form>
  );

  if (onSuccess) {
    return formContent;
  }

  return (
    <Card className="mx-auto max-w-2xl">
      <CardHeader><CardTitle>Add Context Source</CardTitle></CardHeader>
      <CardContent>{formContent}</CardContent>
    </Card>
  );
}
```

**Step 2: Verify, then commit**

```bash
git add web/components/context-sources/new-context-source-form.tsx
git commit -m "refactor(web): add dialog mode support to NewContextSourceForm"
```

---

### Task 7: Projects Page — Add Create/Edit Dialogs

**Files:**
- Modify: `web/app/projects/page.tsx`

**Step 1: Add dialog-based create to ProjectsPage**

Replace `<Link href="/projects/new">` with a button that opens a Dialog containing `<ProjectForm>`. The projects page already has an inline source form pattern; we'll add the project create/edit dialogs alongside it.

Key changes:
1. Import `Dialog`, `DialogContent`, `DialogHeader`, `DialogTitle` from shadcn
2. Import `ProjectForm` from `@/components/projects/project-form`
3. Import `projects as projectsApi, sources as sourcesApi` (already imported)
4. Add state: `const [createOpen, setCreateOpen] = useState(false)`
5. Replace `<Link href="/projects/new">` with `<button onClick={() => setCreateOpen(true)}>`
6. Add `<Dialog>` at the end of the component with `ProjectForm` inside
7. On success: close dialog + `mutate("projects")`

See the full replacement content in the implementing agent — the page is large (469 lines). The core pattern:

```tsx
// At top of component:
const [createOpen, setCreateOpen] = useState(false);

// Replace Link to /projects/new:
<button onClick={() => setCreateOpen(true)} className="...">
  <Plus className="h-3.5 w-3.5" />
  New Project
</button>

// At end of return, before closing </div>:
<Dialog open={createOpen} onOpenChange={setCreateOpen}>
  <DialogContent className="sm:max-w-lg">
    <DialogHeader>
      <DialogTitle>Create Project</DialogTitle>
    </DialogHeader>
    <ProjectForm
      title="Create Project"
      onSubmit={async (result) => {
        const created = await projectsApi.create(result.project);
        if (result.source && created.data?.id) {
          await sourcesApi.create(created.data.id, result.source);
        }
      }}
      onSuccess={() => { setCreateOpen(false); mutate("projects"); }}
      onCancel={() => setCreateOpen(false)}
    />
  </DialogContent>
</Dialog>
```

**Step 2: Verify, then commit**

```bash
git add web/app/projects/page.tsx
git commit -m "feat(web): add dialog-based project creation on projects page"
```

---

### Task 8: Channels Page — Add Create/Edit/Delete Dialogs

**Files:**
- Modify: `web/app/channels/page.tsx`

**Step 1: Replace standalone page links with dialogs**

Add three dialog states: `createOpen`, `editingChannel` (stores the full channel object or null), `deletingId`.

Key changes:
1. Import `Dialog`, `DialogContent`, `DialogHeader`, `DialogTitle`, `ConfirmDialog`, `ChannelForm`
2. Import `useSWR` (already imported) — use `useSWR` to fetch individual channel for edit
3. Replace `<Link href="/channels/new">` → button opening create dialog
4. Replace `<Link href="/channels/${ch.id}/edit">` → button setting `editingChannel`
5. Replace `handleDelete` (which uses `confirm()`) → set `deletingId`, show `ConfirmDialog`

```tsx
// State
const [createOpen, setCreateOpen] = useState(false);
const [editingChannel, setEditingChannel] = useState<NotificationChannel | null>(null);
const [deletingId, setDeletingId] = useState<string | null>(null);

// Create dialog
<Dialog open={createOpen} onOpenChange={setCreateOpen}>
  <DialogContent className="sm:max-w-lg">
    <DialogHeader><DialogTitle>Add Channel</DialogTitle></DialogHeader>
    <ChannelForm
      title="Add Channel"
      onSubmit={async (input) => { await channelsApi.create(input); }}
      onSuccess={() => { setCreateOpen(false); mutate("channels"); }}
      onCancel={() => setCreateOpen(false)}
    />
  </DialogContent>
</Dialog>

// Edit dialog
<Dialog open={!!editingChannel} onOpenChange={(open) => { if (!open) setEditingChannel(null); }}>
  <DialogContent className="sm:max-w-lg">
    <DialogHeader><DialogTitle>Edit Channel</DialogTitle></DialogHeader>
    {editingChannel && (
      <ChannelForm
        title="Edit Channel"
        initial={editingChannel}
        onSubmit={async (input) => { await channelsApi.update(editingChannel.id, input); }}
        onSuccess={() => { setEditingChannel(null); mutate("channels"); }}
        onCancel={() => setEditingChannel(null)}
      />
    )}
  </DialogContent>
</Dialog>

// Delete dialog
<ConfirmDialog
  open={!!deletingId}
  onOpenChange={(open) => { if (!open) setDeletingId(null); }}
  title="Delete Channel"
  description="This will permanently delete this notification channel. This cannot be undone."
  onConfirm={async () => { if (deletingId) { await channelsApi.delete(deletingId); mutate("channels"); } }}
/>

// Edit button in row (replace Link):
<button onClick={() => setEditingChannel(ch)} className="...">
  <Pencil className="h-4 w-4" />
</button>

// Delete button in row:
<button onClick={() => setDeletingId(ch.id)} className="...">
  <Trash2 className="h-4 w-4" />
</button>
```

**Step 2: Verify, then commit**

```bash
git add web/app/channels/page.tsx
git commit -m "feat(web): dialog-based CRUD on channels page"
```

---

### Task 9: Subscriptions Page — Add Create/Edit/Delete Dialogs

**Files:**
- Modify: `web/app/subscriptions/page.tsx`

**Step 1: Same pattern as channels**

Add dialog states, replace Links with buttons, add `<Dialog>` wrappers with `SubscriptionForm` and `ConfirmDialog`.

For edit: since we have the full subscription object in the list data, pass it directly as `initial` to `SubscriptionForm`.

```tsx
// State
const [createOpen, setCreateOpen] = useState(false);
const [editingSub, setEditingSub] = useState<Subscription | null>(null);
const [deletingId, setDeletingId] = useState<string | null>(null);

// Create dialog: SubscriptionForm with onSubmit={subsApi.create}
// Edit dialog: SubscriptionForm with initial={editingSub}, onSubmit={subsApi.update(editingSub.id, input)}
// Delete dialog: ConfirmDialog with onConfirm={subsApi.delete(deletingId)}
```

**Step 2: Verify, then commit**

```bash
git add web/app/subscriptions/page.tsx
git commit -m "feat(web): dialog-based CRUD on subscriptions page"
```

---

### Task 10: Sources Page — Add Edit/Delete Dialogs

The global sources page (`/sources`) is a read-only cross-project view. Currently each row links to `/sources/{id}/edit`. We'll add edit and delete dialogs here.

**Files:**
- Modify: `web/app/sources/page.tsx`

**Step 1: Add edit/delete dialog state**

Since the source data is already in the list with full objects, pass them directly to `SourceForm` as `initial`.

```tsx
// State
const [editingSource, setEditingSource] = useState<SourceWithProject | null>(null);
const [deletingId, setDeletingId] = useState<string | null>(null);

// Edit dialog: SourceForm with initial, onSubmit={sourcesApi.update(editingSource.id, input)}
// Delete dialog: ConfirmDialog with onConfirm={sourcesApi.delete(deletingId)}
// Row edit link → button: onClick={() => setEditingSource(source)}
// Add delete button to each row
```

**Step 2: Verify, then commit**

```bash
git add web/app/sources/page.tsx
git commit -m "feat(web): dialog-based edit/delete on sources page"
```

---

### Task 11: Project Detail — Replace Source/ContextSource Links with Dialogs

**Files:**
- Modify: `web/components/projects/project-detail.tsx`

**Step 1: Replace Links in Sources tab and Context Sources tab**

Key changes:
1. Import `Dialog`, `DialogContent`, `DialogHeader`, `DialogTitle`, `ConfirmDialog`
2. Import `SourceForm` and `NewContextSourceForm`
3. Add state: `sourceCreateOpen`, `editingSource`, `deletingSourceId`, `ctxCreateOpen`, `deletingCtxId`
4. Sources tab: Replace `<Link href="/projects/${id}/sources/new">` → button opening source create dialog
5. Sources tab: Replace `<Link href="/sources/${source.id}/edit">` → button opening source edit dialog
6. Sources tab: Replace `handleDeleteSource` (confirm()) → `setDeletingSourceId`, use `ConfirmDialog`
7. Context Sources tab: Replace `<Link href="/projects/${id}/context-sources/new">` → button opening ctx create dialog
8. Context Sources tab: Replace `handleDeleteCtx` (confirm()) → `setDeletingCtxId`, use `ConfirmDialog`
9. Header: Replace `handleDelete` (confirm()) → project delete `ConfirmDialog`

```tsx
// New state
const [sourceCreateOpen, setSourceCreateOpen] = useState(false);
const [editingSource, setEditingSource] = useState<Source | null>(null);
const [deletingSourceId, setDeletingSourceId] = useState<string | null>(null);
const [ctxCreateOpen, setCtxCreateOpen] = useState(false);
const [deletingCtxId, setDeletingCtxId] = useState<string | null>(null);
const [deletingProject, setDeletingProject] = useState(false);
```

This is the largest file change. The implementing agent should carefully replace only the Link/confirm patterns, keeping all other UI intact.

**Step 2: Verify, then commit**

```bash
git add web/components/projects/project-detail.tsx
git commit -m "feat(web): dialog-based CRUD in project detail page"
```

---

### Task 12: Delete Standalone Page Files and Edit Wrappers

**Files to delete:**
- `web/app/projects/new/page.tsx`
- `web/app/projects/[id]/edit/page.tsx`
- `web/app/projects/[id]/sources/new/page.tsx`
- `web/app/projects/[id]/context-sources/new/page.tsx`
- `web/app/sources/new/page.tsx`
- `web/app/sources/[id]/edit/page.tsx`
- `web/app/channels/new/page.tsx`
- `web/app/channels/[id]/edit/page.tsx`
- `web/app/subscriptions/new/page.tsx`
- `web/app/subscriptions/[id]/edit/page.tsx`
- `web/components/projects/project-edit.tsx`
- `web/components/sources/source-edit.tsx`
- `web/components/sources/new-project-source.tsx`
- `web/components/channels/channel-edit.tsx`
- `web/components/subscriptions/subscription-edit.tsx`
- `web/components/context-sources/context-sources-list.tsx` (now unused — context sources are shown inline in project detail)

Also delete any now-empty directories:
- `web/app/projects/new/`
- `web/app/projects/[id]/edit/`
- `web/app/projects/[id]/sources/`
- `web/app/projects/[id]/context-sources/` (keep if the list page is still used, delete `new/` subfolder)
- `web/app/sources/new/`
- `web/app/sources/[id]/`
- `web/app/channels/new/`
- `web/app/channels/[id]/`
- `web/app/subscriptions/new/`
- `web/app/subscriptions/[id]/`

**Step 1: Delete files**

```bash
# Pages
rm web/app/projects/new/page.tsx
rm web/app/projects/\[id\]/edit/page.tsx
rm web/app/projects/\[id\]/sources/new/page.tsx
rm web/app/projects/\[id\]/context-sources/new/page.tsx
rm web/app/sources/new/page.tsx
rm web/app/sources/\[id\]/edit/page.tsx
rm web/app/channels/new/page.tsx
rm web/app/channels/\[id\]/edit/page.tsx
rm web/app/subscriptions/new/page.tsx
rm web/app/subscriptions/\[id\]/edit/page.tsx

# Edit wrappers
rm web/components/projects/project-edit.tsx
rm web/components/sources/source-edit.tsx
rm web/components/sources/new-project-source.tsx
rm web/components/channels/channel-edit.tsx
rm web/components/subscriptions/subscription-edit.tsx
rm web/components/context-sources/context-sources-list.tsx

# Clean empty dirs
rmdir web/app/projects/new
rmdir web/app/projects/\[id\]/edit
rmdir web/app/projects/\[id\]/sources/new
rmdir web/app/projects/\[id\]/sources
rmdir web/app/projects/\[id\]/context-sources/new
rmdir web/app/sources/new
rmdir web/app/sources/\[id\]/edit
rmdir web/app/sources/\[id\]
rmdir web/app/channels/new
rmdir web/app/channels/\[id\]/edit
rmdir web/app/channels/\[id\]
rmdir web/app/subscriptions/new
rmdir web/app/subscriptions/\[id\]/edit
rmdir web/app/subscriptions/\[id\]
```

**Step 2: Grep for any remaining imports of deleted files**

```bash
cd web && grep -r "project-edit\|source-edit\|channel-edit\|subscription-edit\|new-project-source\|context-sources-list" --include="*.tsx" --include="*.ts" .
```

Expected: No matches (or only the files we already modified)

**Step 3: Verify build**

```bash
cd web && npx next build
```

**Step 4: Commit**

```bash
git add -A
git commit -m "chore(web): delete standalone CRUD pages and edit wrappers"
```

---

### Task 13: Final Verification

**Step 1: Full build**

```bash
cd web && npx next build
```

Expected: Build succeeds with no errors.

**Step 2: Manual smoke test**

```bash
make dev
```

Verify in browser:
- `/projects` — "New Project" button opens dialog, cancel closes it
- `/projects` — Create a project, verify it appears in list
- `/projects/{id}` — Sources tab: "Add Source" opens dialog, "Edit" icon opens edit dialog, "Delete" opens confirm dialog
- `/projects/{id}` — Context Sources tab: "Add Context Source" opens dialog, "Delete" opens confirm dialog
- `/channels` — "New Channel" opens dialog, edit/delete work via dialogs
- `/subscriptions` — "New Subscription" opens dialog, edit/delete work via dialogs
- `/sources` — Edit/delete work via dialogs
- Old URLs (`/projects/new`, `/channels/new`, etc.) return 404

**Step 3: Commit any fixes, then final commit**

```bash
git add -A
git commit -m "chore(web): flat UI redesign verification complete"
```
