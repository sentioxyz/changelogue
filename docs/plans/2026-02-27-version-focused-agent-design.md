# Version-Focused Agent with SRE Report + Multi-Source Waiting

**Date:** 2026-02-27
**Status:** Approved

## Problem

1. **Agent reads ALL releases** — no version scoping; wastes tokens and loses focus
2. **No web search** — agent can't check community sentiment, security advisories, or adoption stats
3. **Report format is too generic** — SREs need actionable fields: subject line, risk level, download commands
4. **No multi-source waiting** — agent fires per-source, doesn't cross-reference availability across sources

## Design

### 1. Version-Scoped Prompt

Add `{{VERSION}}` placeholder to `DefaultInstruction`. The orchestrator extracts the version from the trigger string (`auto:version:v1.10.15`) or from `AgentJobArgs.Version` and substitutes it before building the agent.

**New DefaultInstruction:**

```
You are a release intelligence agent analyzing version {{VERSION}} of a software project.

Focus ONLY on version {{VERSION}}. Cross-check this version across all available sources.

Use tools to:
1. Fetch releases and find the one matching {{VERSION}} from each source.
2. Inspect release details (changelogs, commit data, raw payloads) for {{VERSION}} only.
3. Check binary/image availability directly from the source data.
4. Review context sources for relevant background.
5. Use web search ONLY when you need additional context not available from sources
   (e.g., community sentiment, security advisories, network adoption stats, known issues).

CRITICAL: Your final response MUST be a single JSON object and nothing else.
Do not include any explanation, commentary, or markdown formatting — just the raw JSON.

The JSON object must have exactly these fields:
{
  "subject": "🚀 Ready to Deploy: <Project> <Version> (<Risk Summary>)",
  "risk_level": "CRITICAL|HIGH|MEDIUM|LOW",
  "risk_reason": "Why this risk level (e.g., 'Hard Fork detected in Discord #announcements')",
  "status_checks": ["Docker Image Verified", "Binaries Available"],
  "changelog_summary": "One-line summary of key changes (e.g., 'Fixes sync bug in block 14,000,000')",
  "availability": "GA|RC|Beta",
  "adoption": "Percentage or recommendation (e.g., '12% of network updated (Wait recommended if not urgent)')",
  "urgency": "Critical|High|Medium|Low",
  "recommendation": "Actionable 1-2 sentence recommendation for the SRE team",
  "download_commands": ["docker pull ethereum/client-go:v1.10.15"],
  "download_links": ["https://github.com/ethereum/go-ethereum/releases/tag/v1.10.15"]
}
```

**User message** changes from generic to version-specific:
```
Analyze version {{VERSION}} for this project. Cross-check all sources and produce a semantic release report.
```

### 2. Sub-Agent Architecture (Google Search)

The current single `release_analyst` agent becomes a root agent delegating to two sub-agents. This is the documented ADK-Go pattern for mixing Gemini grounding tools with function tools.

```
root_agent (release_analyst)
  ├── data_agent     — has: get_releases, get_release_detail, list_context_sources
  └── search_agent   — has: geminitool.GoogleSearch{}
```

- **Root agent**: Receives the version-scoped instruction, orchestrates the analysis workflow
- **Data agent**: Handles all DB queries (existing function tools, scoped to project)
- **Search agent**: Handles web searches when root agent needs external context
- `agenttool.New()` wraps each sub-agent as a tool callable by the root

**Imports added:**
```go
import (
    "google.golang.org/adk/tool/agenttool"
    "google.golang.org/adk/tool/geminitool"
)
```

**BuildAgent changes:**
```go
// Data sub-agent with function tools
dataAgent := llmagent.New(llmagent.Config{
    Name:        "data_agent",
    Description: "Query project releases and context sources from the database.",
    Model:       llmModel,
    Tools:       functionTools, // get_releases, get_release_detail, list_context_sources
})

// Search sub-agent with Google Search grounding
searchAgent := llmagent.New(llmagent.Config{
    Name:        "search_agent",
    Description: "Search the web for additional context about a release.",
    Model:       llmModel,
    Tools:       []tool.Tool{geminitool.GoogleSearch{}},
})

// Root agent delegates to both
rootAgent := llmagent.New(llmagent.Config{
    Name:        "release_analyst",
    Description: "Analyzes upstream releases and produces semantic release reports.",
    Model:       llmModel,
    Instruction: instruction, // version-scoped
    Tools: []tool.Tool{
        agenttool.New("data_agent", dataAgent),
        agenttool.New("search_agent", searchAgent),
    },
})
```

### 3. Multi-Source Waiting

Configurable per-project via `AgentRules`. When enabled, the agent waits until ALL sources in the project have a release matching the target version before generating the report.

**AgentRules addition:**
```go
type AgentRules struct {
    OnMajorRelease    bool   `json:"on_major_release,omitempty"`
    OnMinorRelease    bool   `json:"on_minor_release,omitempty"`
    OnSecurityPatch   bool   `json:"on_security_patch,omitempty"`
    VersionPattern    string `json:"version_pattern,omitempty"`
    WaitForAllSources bool   `json:"wait_for_all_sources,omitempty"` // NEW
}
```

**Flow when `wait_for_all_sources` is true:**

1. `AgentWorker` picks up job
2. Orchestrator checks: does every source in this project have a release matching the target version?
3. If **yes** → proceed with agent execution
4. If **no** → set run status to `"waiting"`, return `river.JobSnooze(duration)` to re-enqueue after delay (e.g., 5 minutes)
5. Max retries before giving up (e.g., 12 attempts = ~1 hour), then run with partial data

**New store method needed:**
```go
ListSourcesByProject(ctx context.Context, projectID string) ([]models.Source, error)
HasReleaseForVersion(ctx context.Context, sourceID, version string) (bool, error)
```

**New agent_run status:** `"waiting"` (in addition to pending, running, completed, failed)

### 4. Expanded SemanticReport Model

```go
type SemanticReport struct {
    // New SRE-focused fields
    Subject          string   `json:"subject"`
    RiskLevel        string   `json:"risk_level"`
    RiskReason       string   `json:"risk_reason"`
    StatusChecks     []string `json:"status_checks"`
    ChangelogSummary string   `json:"changelog_summary"`
    DownloadCommands []string `json:"download_commands,omitempty"`
    DownloadLinks    []string `json:"download_links,omitempty"`

    // Existing fields (kept for backward compat)
    Summary        string `json:"summary,omitempty"`
    Availability   string `json:"availability"`
    Adoption       string `json:"adoption"`
    Urgency        string `json:"urgency"`
    Recommendation string `json:"recommendation"`
}
```

### 5. AgentJobArgs Version Field

```go
type AgentJobArgs struct {
    AgentRunID string `json:"agent_run_id"`
    ProjectID  string `json:"project_id"`
    Version    string `json:"version"` // NEW — target version to analyze
}
```

The version is passed explicitly rather than parsed from the trigger string.

**Propagation:** `EnqueueAgentRun` signature changes to accept version:
```go
EnqueueAgentRun(ctx context.Context, projectID, trigger, version string) error
```

### 6. AgentRun Model Change

Add `Version` field to `AgentRun` for persisting which version the run targets:
```go
type AgentRun struct {
    // ... existing fields ...
    Version string `json:"version,omitempty"` // NEW — target version
}
```

## Files Changed

| File | Change |
|------|--------|
| `internal/agent/orchestrator.go` | Version placeholder substitution in prompt, version-scoped user message, sub-agent architecture in `BuildAgent`, multi-source wait check before execution |
| `internal/agent/tools.go` | No change to tool implementations (tools stay the same) |
| `internal/agent/worker.go` | Pass version from job args, handle `"waiting"` status + `river.JobSnooze` |
| `internal/models/semantic_release.go` | Expand `SemanticReport` struct with SRE fields |
| `internal/models/project.go` | Add `WaitForAllSources` to `AgentRules` |
| `internal/models/agent_run.go` | Add `Version` field |
| `internal/queue/jobs.go` | Add `Version` to `AgentJobArgs` |
| `internal/routing/worker.go` | Pass version to `EnqueueAgentRun` |
| `internal/api/pgstore.go` | Update `EnqueueAgentRun` signature, add `HasReleaseForVersion` + `ListSourcesByProject` queries |
| `internal/db/migrations.go` | Add `version` column to `agent_runs` table |

## Non-Goals

- OpenAI provider web search (Gemini-only for now via `geminitool.GoogleSearch`)
- UI changes for the new report format
- Notification format changes (notification sends raw report text as before)
