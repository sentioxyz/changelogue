# Semantic Release Settings Tab Redesign — Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Redesign the Agent tab to "Semantic Release Settings" with clear trigger rules, custom prompt config, and a test run picker that lets users select a source + version.

**Architecture:** Backend: add `version` field to trigger API request. Frontend: rename tab, remove header button, reorganize agent tab into 4 cards (trigger rules, prompt, test run with source/version pickers, run history).

**Tech Stack:** Go (backend API), Next.js + React + Tailwind (frontend)

---

### Task 1: Backend — Add version field to trigger API

**Files:**
- Modify: `internal/api/agent.go:29-31` (triggerRequest struct)
- Modify: `internal/api/agent.go:34-55` (TriggerRun handler)
- Modify: `internal/api/pgstore.go:589-613` (TriggerAgentRun)
- Modify: `internal/api/agent.go:12-16` (AgentStore interface)

**Step 1: Update triggerRequest struct**

In `internal/api/agent.go`, change the struct at line 29:

```go
type triggerRequest struct {
	Trigger string `json:"trigger"`
	Version string `json:"version"`
}
```

**Step 2: Update AgentStore interface**

In `internal/api/agent.go`, change the interface at line 12-16:

```go
type AgentStore interface {
	TriggerAgentRun(ctx context.Context, projectID, trigger, version string) (*models.AgentRun, error)
	ListAgentRuns(ctx context.Context, projectID string, page, perPage int) ([]models.AgentRun, int, error)
	GetAgentRun(ctx context.Context, id string) (*models.AgentRun, error)
}
```

**Step 3: Update TriggerRun handler**

In `internal/api/agent.go`, update lines 44-49 to pass version:

```go
	trigger := strings.TrimSpace(req.Trigger)
	if trigger == "" {
		trigger = "manual"
	}
	version := strings.TrimSpace(req.Version)
	run, err := h.store.TriggerAgentRun(r.Context(), projectID, trigger, version)
```

**Step 4: Update PgStore.TriggerAgentRun**

In `internal/api/pgstore.go`, update the function signature and SQL at lines 589-613:

```go
func (s *PgStore) TriggerAgentRun(ctx context.Context, projectID, trigger, version string) (*models.AgentRun, error) {
	var run models.AgentRun
	err := s.pool.QueryRow(ctx,
		`INSERT INTO agent_runs (project_id, trigger, version, status)
		 VALUES ($1, $2, $3, 'pending')
		 RETURNING id, project_id, trigger, COALESCE(version,''), status, created_at`,
		projectID, trigger, nilIfEmpty(version),
	).Scan(&run.ID, &run.ProjectID, &run.Trigger, &run.Version, &run.Status, &run.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("insert agent run: %w", err)
	}

	if s.river != nil {
		_, err = s.river.Insert(ctx, queue.AgentJobArgs{
			AgentRunID: run.ID,
			ProjectID:  projectID,
			Version:    version,
		}, nil)
		if err != nil {
			return nil, fmt.Errorf("enqueue agent job: %w", err)
		}
	}

	return &run, nil
}
```

Add `nilIfEmpty` helper if not already present:

```go
func nilIfEmpty(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}
```

**Step 5: Check AgentJobArgs has Version field**

Verify in `internal/queue/` that `AgentJobArgs` already has a `Version` field. If not, add it.

**Step 6: Check any other callers of TriggerAgentRun**

Search for other callers of `TriggerAgentRun` — the routing worker uses `EnqueueAgentRun` (different method), so this should only affect the API handler. Verify and update any other callers.

**Step 7: Run tests**

Run: `go vet ./internal/api/... && go build ./...`
Expected: builds cleanly

**Step 8: Commit**

```bash
git add internal/api/agent.go internal/api/pgstore.go
git commit -m "feat(api): add version field to agent trigger request"
```

---

### Task 2: Frontend — Update API client to pass version

**Files:**
- Modify: `web/lib/api/client.ts:130-132` (agent.triggerRun)

**Step 1: Update triggerRun to accept trigger and version**

In `web/lib/api/client.ts`, change line 131:

```typescript
export const agent = {
  triggerRun: (projectId: string, version?: string) =>
    request<ApiResponse<AgentRun>>(`/projects/${projectId}/agent/run`, {
      method: "POST",
      body: JSON.stringify({ trigger: "test", version: version || undefined }),
    }),
  listRuns: (projectId: string, page = 1) =>
    request<ApiResponse<AgentRun[]>>(`/projects/${projectId}/agent/runs?page=${page}`),
  getRun: (id: string) =>
    request<ApiResponse<AgentRun>>(`/agent-runs/${id}`),
};
```

**Step 2: Commit**

```bash
git add web/lib/api/client.ts
git commit -m "feat(web): update agent API client to accept version parameter"
```

---

### Task 3: Frontend — Rename tab and remove Run Agent button from header

**Files:**
- Modify: `web/components/projects/project-detail.tsx`

**Step 1: Rename the tab**

Change line 29 from:
```typescript
  { key: "agent", label: "Agent" },
```
to:
```typescript
  { key: "agent", label: "Semantic Release Settings" },
```

**Step 2: Remove the Run Agent button from header**

Delete lines 310-321 (the Run Agent button in the header action area):

```typescript
            <button
              onClick={handleTriggerRun}
              disabled={triggering}
              className="inline-flex items-center gap-1.5 rounded-md px-3 py-1.5 text-[13px] font-medium text-white transition-colors disabled:opacity-60"
              style={{
                fontFamily: "var(--font-dm-sans), sans-serif",
                backgroundColor: "#e8601a",
              }}
            >
              <Play className="h-3.5 w-3.5" />
              {triggering ? "Running..." : "Run Agent"}
            </button>
```

**Step 3: Remove the old handleTriggerRun function**

Remove lines 136-144 (the old `handleTriggerRun` that just called `agentApi.triggerRun(id)` without a version). It will be replaced in Task 4.

**Step 4: Remove unused `triggering` state**

Remove line 56: `const [triggering, setTriggering] = useState(false);` — this will be re-added with a different name in the test run card.

**Step 5: Remove unused `Play` import if no longer needed**

Check if `Play` from lucide-react is still used elsewhere. If not, remove from imports on line 22.

**Step 6: Verify it compiles**

Run: `cd web && npx next build --no-lint` or just check for TS errors
Expected: No errors (agent tab content still renders, just without the header button)

**Step 7: Commit**

```bash
git add web/components/projects/project-detail.tsx
git commit -m "refactor(web): rename Agent tab and remove Run Agent header button"
```

---

### Task 4: Frontend — Redesign agent tab with 4 cards

**Files:**
- Modify: `web/components/projects/project-detail.tsx:537-695` (agent tab content)
- Modify: `web/components/projects/project-detail.tsx` (imports and state)

This is the main UI change. Replace the agent tab content (lines 537-695) with four card sections.

**Step 1: Add new state and data fetching for test run**

Add these state vars near the top of the `ProjectDetail` component (after existing state):

```typescript
  /* Test run state */
  const [testSourceId, setTestSourceId] = useState<string>("");
  const [testVersion, setTestVersion] = useState<string>("");
  const [testRunning, setTestRunning] = useState(false);
```

Add SWR for releases by source (inside component, after existing SWR calls):

```typescript
  const { data: testReleasesData } = useSWR(
    testSourceId ? `source-${testSourceId}-releases` : null,
    () => releases.listBySource(testSourceId, 1),
  );
```

Add import for `releases` from `@/lib/api/client` (add to existing import at line 8-12):

```typescript
import {
  projects as projectsApi,
  sources as sourcesApi,
  contextSources as ctxApi,
  agent as agentApi,
  releases as releasesApi,
} from "@/lib/api/client";
```

**Step 2: Add test run handler**

```typescript
  const handleTestRun = async () => {
    if (!testVersion) return;
    setTestRunning(true);
    try {
      await agentApi.triggerRun(id, testVersion);
      mutateRuns();
    } finally {
      setTestRunning(false);
    }
  };
```

**Step 3: Replace agent tab content**

Replace the entire agent tab content (lines 538-695) with the redesigned 4-card layout:

```tsx
{activeTab === "agent" && (
  <div className="space-y-6">
    {/* Card 1: Trigger Rules */}
    <div
      className="rounded-lg border p-5"
      style={{ borderColor: "#e8e8e5", backgroundColor: "#ffffff" }}
    >
      <SectionLabel className="mb-1">Trigger Rules</SectionLabel>
      <p className="mb-4 text-[12px]" style={{ color: "#9ca3af" }}>
        Automatically run the agent when new releases match these conditions.
      </p>
      <div className="space-y-3">
        <label className="flex items-center gap-2.5 text-[13px]" style={{ color: "#374151" }}>
          <input
            type="checkbox"
            checked={currentRules.on_major_release ?? false}
            onChange={(e) =>
              setRulesDraft({ ...currentRules, on_major_release: e.target.checked })
            }
            className="h-4 w-4 rounded border accent-[#e8601a]"
            style={{ borderColor: "#e8e8e5" }}
          />
          Major release
          <span className="text-[11px]" style={{ color: "#9ca3af" }}>
            (e.g. 1.x → 2.x)
          </span>
        </label>
        <label className="flex items-center gap-2.5 text-[13px]" style={{ color: "#374151" }}>
          <input
            type="checkbox"
            checked={currentRules.on_minor_release ?? false}
            onChange={(e) =>
              setRulesDraft({ ...currentRules, on_minor_release: e.target.checked })
            }
            className="h-4 w-4 rounded border accent-[#e8601a]"
            style={{ borderColor: "#e8e8e5" }}
          />
          Minor release
          <span className="text-[11px]" style={{ color: "#9ca3af" }}>
            (e.g. 1.1 → 1.2)
          </span>
        </label>
        <label className="flex items-center gap-2.5 text-[13px]" style={{ color: "#374151" }}>
          <input
            type="checkbox"
            checked={currentRules.on_security_patch ?? false}
            onChange={(e) =>
              setRulesDraft({ ...currentRules, on_security_patch: e.target.checked })
            }
            className="h-4 w-4 rounded border accent-[#e8601a]"
            style={{ borderColor: "#e8e8e5" }}
          />
          Security patch
          <span className="text-[11px]" style={{ color: "#9ca3af" }}>
            (contains security/CVE keywords)
          </span>
        </label>
        <div className="pt-2">
          <label className="mb-1.5 block text-[13px]" style={{ color: "#374151" }}>
            Version pattern
          </label>
          <input
            type="text"
            value={currentRules.version_pattern ?? ""}
            onChange={(e) =>
              setRulesDraft({ ...currentRules, version_pattern: e.target.value })
            }
            placeholder="e.g. ^v\\d+\\.\\d+\\.\\d+$"
            className="w-full max-w-md rounded-md border px-3 py-1.5 text-[13px] placeholder:text-[#9ca3af] focus:outline-none focus:ring-1"
            style={{
              fontFamily: "'JetBrains Mono', monospace",
              backgroundColor: "#fafaf9",
              borderColor: "#e8e8e5",
              color: "#111113",
            }}
          />
          <p className="mt-1 text-[11px]" style={{ color: "#9ca3af" }}>
            Optional regex to filter which versions trigger agent runs.
          </p>
        </div>
      </div>
    </div>

    {/* Card 2: Agent Prompt */}
    <div
      className="rounded-lg border p-5"
      style={{ borderColor: "#e8e8e5", backgroundColor: "#ffffff" }}
    >
      <SectionLabel className="mb-1">Agent Prompt</SectionLabel>
      <p className="mb-3 text-[12px]" style={{ color: "#9ca3af" }}>
        Custom instructions for the agent when analyzing releases.
      </p>
      <textarea
        value={currentPrompt}
        onChange={(e) => setPromptDraft(e.target.value)}
        rows={5}
        placeholder="Using default agent prompt."
        className="w-full resize-y rounded-md border px-3 py-2 text-[13px] placeholder:text-[#9ca3af] focus:outline-none focus:ring-1"
        style={{
          fontFamily: "var(--font-dm-sans), sans-serif",
          backgroundColor: "#fafaf9",
          borderColor: "#e8e8e5",
          color: "#111113",
        }}
      />
      <div className="mt-3 flex justify-end">
        <button
          onClick={handleSaveAgentConfig}
          disabled={saving || (promptDraft === null && rulesDraft === null)}
          className="rounded-md px-4 py-1.5 text-[13px] font-medium text-white transition-colors disabled:opacity-40"
          style={{ backgroundColor: "#e8601a" }}
        >
          {saving ? "Saving..." : "Save Settings"}
        </button>
      </div>
    </div>

    {/* Card 3: Test Run */}
    <div
      className="rounded-lg border p-5"
      style={{ borderColor: "#e8e8e5", backgroundColor: "#ffffff" }}
    >
      <SectionLabel className="mb-1">Test Run</SectionLabel>
      <p className="mb-4 text-[12px]" style={{ color: "#9ca3af" }}>
        Trigger a one-off agent run to test your configuration against a specific release.
      </p>
      <div className="flex flex-wrap items-end gap-3">
        <div className="min-w-[200px] flex-1">
          <label className="mb-1.5 block text-[13px]" style={{ color: "#374151" }}>
            Source
          </label>
          <select
            value={testSourceId}
            onChange={(e) => {
              setTestSourceId(e.target.value);
              setTestVersion("");
            }}
            className="w-full rounded-md border px-3 py-1.5 text-[13px] focus:outline-none focus:ring-1"
            style={{
              fontFamily: "var(--font-dm-sans), sans-serif",
              backgroundColor: "#fafaf9",
              borderColor: "#e8e8e5",
              color: testSourceId ? "#111113" : "#9ca3af",
            }}
          >
            <option value="">Select a source...</option>
            {sourcesData?.data?.map((s) => (
              <option key={s.id} value={s.id}>
                {s.provider}: {s.repository}
              </option>
            ))}
          </select>
        </div>
        <div className="min-w-[200px] flex-1">
          <label className="mb-1.5 block text-[13px]" style={{ color: "#374151" }}>
            Version
          </label>
          <select
            value={testVersion}
            onChange={(e) => setTestVersion(e.target.value)}
            disabled={!testSourceId}
            className="w-full rounded-md border px-3 py-1.5 text-[13px] focus:outline-none focus:ring-1 disabled:opacity-50"
            style={{
              fontFamily: "'JetBrains Mono', monospace",
              backgroundColor: "#fafaf9",
              borderColor: "#e8e8e5",
              color: testVersion ? "#111113" : "#9ca3af",
            }}
          >
            <option value="">
              {testSourceId ? "Select a version..." : "Choose a source first"}
            </option>
            {testReleasesData?.data?.map((r) => (
              <option key={r.id} value={r.version}>
                {r.version}
              </option>
            ))}
          </select>
        </div>
        <button
          onClick={handleTestRun}
          disabled={testRunning || !testVersion}
          className="inline-flex items-center gap-1.5 rounded-md px-4 py-1.5 text-[13px] font-medium text-white transition-colors disabled:opacity-40"
          style={{ backgroundColor: "#e8601a" }}
        >
          <Play className="h-3.5 w-3.5" />
          {testRunning ? "Running..." : "Run Test"}
        </button>
      </div>
    </div>

    {/* Card 4: Run History */}
    <div>
      <SectionLabel className="mb-3">Run History</SectionLabel>
      {runsData?.data && runsData.data.length > 0 ? (
        <div className="overflow-hidden rounded-md border" style={{ borderColor: "#e8e8e5" }}>
          <table className="w-full text-[13px]" style={{ fontFamily: "var(--font-dm-sans), sans-serif" }}>
            <thead>
              <tr style={{ backgroundColor: "#fafaf9" }}>
                <th className="px-4 py-2 text-left text-[11px] font-medium uppercase tracking-[0.08em]" style={{ color: "#9ca3af" }}>Trigger</th>
                <th className="px-4 py-2 text-left text-[11px] font-medium uppercase tracking-[0.08em]" style={{ color: "#9ca3af" }}>Status</th>
                <th className="px-4 py-2 text-left text-[11px] font-medium uppercase tracking-[0.08em]" style={{ color: "#9ca3af" }}>Started</th>
                <th className="px-4 py-2 text-left text-[11px] font-medium uppercase tracking-[0.08em]" style={{ color: "#9ca3af" }}>Duration</th>
                <th className="px-4 py-2 text-left text-[11px] font-medium uppercase tracking-[0.08em]" style={{ color: "#9ca3af" }}>Semantic Release</th>
              </tr>
            </thead>
            <tbody>
              {runsData.data.map((run) => (
                <tr key={run.id} className="border-t" style={{ borderColor: "#e8e8e5" }}>
                  <td className="px-4 py-3" style={{ color: "#374151" }}>
                    {run.trigger}
                  </td>
                  <td className="px-4 py-3">
                    <div className="flex items-center gap-2">
                      <StatusDot status={run.status} />
                      <span style={{ color: "#6b7280" }}>{run.status}</span>
                    </div>
                  </td>
                  <td className="px-4 py-3" style={{ color: "#9ca3af" }}>
                    {run.started_at
                      ? new Date(run.started_at).toLocaleString()
                      : "Pending"}
                  </td>
                  <td className="px-4 py-3" style={{ color: "#9ca3af" }}>
                    {formatDuration(run.started_at, run.completed_at)}
                  </td>
                  <td className="px-4 py-3">
                    {run.semantic_release_id ? (
                      <Link
                        href={`/projects/${id}/semantic-releases/${run.semantic_release_id}`}
                        className="text-[13px] font-medium underline"
                        style={{ color: "#e8601a" }}
                      >
                        View
                      </Link>
                    ) : (
                      <span style={{ color: "#9ca3af" }}>--</span>
                    )}
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      ) : (
        <div
          className="flex h-32 items-center justify-center rounded-md border text-[13px]"
          style={{ borderColor: "#e8e8e5", color: "#9ca3af" }}
        >
          No agent runs yet
        </div>
      )}
    </div>
  </div>
)}
```

**Step 4: Remove the `Play` import cleanup**

Keep `Play` in imports since it's used in the Test Run button.

**Step 5: Verify it compiles**

Run: `cd web && npx tsc --noEmit`
Expected: No type errors

**Step 6: Commit**

```bash
git add web/components/projects/project-detail.tsx
git commit -m "feat(web): redesign agent tab as Semantic Release Settings with test run picker"
```

---

### Task 5: Verification — Visual check and end-to-end test

**Step 1: Start the dev server**

Run: `make dev` (starts Postgres + Go server)

In another terminal: `cd web && npm run dev`

**Step 2: Navigate to a project detail page**

Open a project that has at least one source with releases.

**Step 3: Verify**

1. Tab label reads "Semantic Release Settings" (not "Agent")
2. No "Run Agent" button in the project header (only Delete button)
3. Agent tab shows 4 cards: Trigger Rules, Agent Prompt, Test Run, Run History
4. Trigger rules card has 3 checkboxes + version pattern with help text
5. Agent prompt card has textarea + "Save Settings" button
6. Test Run card: source dropdown populates from project sources
7. Test Run card: selecting a source populates version dropdown with releases
8. Test Run card: "Run Test" button is disabled until version is selected
9. Clicking "Run Test" creates an agent run that shows in Run History

**Step 4: Final commit (if any fixes needed)**

```bash
git add -A && git commit -m "fix(web): polish semantic release settings tab"
```
