# Version-Focused Agent Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Make the agent focus on a specific release version, add Google Search via sub-agents, expand the report for SRE needs, and add configurable multi-source waiting.

**Architecture:** Version is passed through `AgentJobArgs` → orchestrator substitutes `{{VERSION}}` in the prompt → root agent delegates to data sub-agent (DB tools) and search sub-agent (Google Search) → expanded `SemanticReport` with SRE fields. Multi-source waiting uses `river.JobSnooze` with a new `"waiting"` status.

**Tech Stack:** Go 1.25, ADK-Go v0.5.0 (`llmagent`, `agenttool`, `geminitool`), River for job queue, PostgreSQL

---

### Task 1: Expand SemanticReport model with SRE fields

**Files:**
- Modify: `internal/models/semantic_release.go`

**Step 1: Write the failing test**

Create `internal/models/semantic_release_test.go`:

```go
package models

import (
	"encoding/json"
	"testing"
)

func TestSemanticReportJSON(t *testing.T) {
	report := SemanticReport{
		Subject:          "🚀 Ready to Deploy: Geth v1.10.15 (Critical Update)",
		RiskLevel:        "CRITICAL",
		RiskReason:       "Hard Fork detected in Discord #announcements",
		StatusChecks:     []string{"Docker Image Verified", "Binaries Available"},
		ChangelogSummary: "Fixes sync bug in block 14,000,000",
		Availability:     "GA",
		Adoption:         "12% of network updated",
		Urgency:          "Critical",
		Recommendation:   "Wait for 25% adoption unless urgent.",
		DownloadCommands: []string{"docker pull ethereum/client-go:v1.10.15"},
		DownloadLinks:    []string{"https://github.com/ethereum/go-ethereum/releases/tag/v1.10.15"},
	}

	data, err := json.Marshal(report)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var decoded SemanticReport
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if decoded.Subject != report.Subject {
		t.Errorf("subject: got %q, want %q", decoded.Subject, report.Subject)
	}
	if decoded.RiskLevel != report.RiskLevel {
		t.Errorf("risk_level: got %q, want %q", decoded.RiskLevel, report.RiskLevel)
	}
	if len(decoded.StatusChecks) != 2 {
		t.Errorf("status_checks: got %d items, want 2", len(decoded.StatusChecks))
	}
	if len(decoded.DownloadCommands) != 1 {
		t.Errorf("download_commands: got %d items, want 1", len(decoded.DownloadCommands))
	}
	if len(decoded.DownloadLinks) != 1 {
		t.Errorf("download_links: got %d items, want 1", len(decoded.DownloadLinks))
	}
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/models/ -run TestSemanticReportJSON -v`
Expected: FAIL — `SemanticReport` doesn't have `Subject`, `RiskLevel`, etc.

**Step 3: Update the SemanticReport struct**

In `internal/models/semantic_release.go`, replace the `SemanticReport` struct:

```go
type SemanticReport struct {
	// SRE-focused fields
	Subject          string   `json:"subject"`
	RiskLevel        string   `json:"risk_level"`
	RiskReason       string   `json:"risk_reason"`
	StatusChecks     []string `json:"status_checks"`
	ChangelogSummary string   `json:"changelog_summary"`
	DownloadCommands []string `json:"download_commands,omitempty"`
	DownloadLinks    []string `json:"download_links,omitempty"`

	// Existing fields
	Summary        string `json:"summary,omitempty"`
	Availability   string `json:"availability"`
	Adoption       string `json:"adoption"`
	Urgency        string `json:"urgency"`
	Recommendation string `json:"recommendation"`
}
```

**Step 4: Run test to verify it passes**

Run: `go test ./internal/models/ -run TestSemanticReportJSON -v`
Expected: PASS

**Step 5: Commit**

```bash
git add internal/models/semantic_release.go internal/models/semantic_release_test.go
git commit -m "feat(models): expand SemanticReport with SRE-focused fields"
```

---

### Task 2: Add Version field to AgentJobArgs and AgentRun

**Files:**
- Modify: `internal/queue/jobs.go`
- Modify: `internal/models/agent_run.go`
- Modify: `internal/db/migrations.go`

**Step 1: Add `Version` to AgentJobArgs**

In `internal/queue/jobs.go`, add the `Version` field:

```go
type AgentJobArgs struct {
	AgentRunID string `json:"agent_run_id"`
	ProjectID  string `json:"project_id"`
	Version    string `json:"version"`
}
```

**Step 2: Add `Version` to AgentRun model**

In `internal/models/agent_run.go`:

```go
type AgentRun struct {
	ID                string     `json:"id"`
	ProjectID         string     `json:"project_id"`
	SemanticReleaseID *string    `json:"semantic_release_id,omitempty"`
	Trigger           string     `json:"trigger"`
	Version           string     `json:"version,omitempty"`
	Status            string     `json:"status"`
	PromptUsed        string     `json:"prompt_used,omitempty"`
	Error             string     `json:"error,omitempty"`
	StartedAt         *time.Time `json:"started_at,omitempty"`
	CompletedAt       *time.Time `json:"completed_at,omitempty"`
	CreatedAt         time.Time  `json:"created_at"`
}
```

**Step 3: Add `version` column to agent_runs migration**

In `internal/db/migrations.go`, add after the `agent_runs` CREATE TABLE statement:

```sql
ALTER TABLE agent_runs ADD COLUMN IF NOT EXISTS version VARCHAR(100);
```

**Step 4: Run vet and compile**

Run: `go vet ./internal/...`
Expected: PASS (no errors)

**Step 5: Commit**

```bash
git add internal/queue/jobs.go internal/models/agent_run.go internal/db/migrations.go
git commit -m "feat(models): add version field to AgentJobArgs and AgentRun"
```

---

### Task 3: Add WaitForAllSources to AgentRules

**Files:**
- Modify: `internal/models/project.go`

**Step 1: Add `WaitForAllSources` field**

In `internal/models/project.go`, update `AgentRules`:

```go
type AgentRules struct {
	OnMajorRelease    bool   `json:"on_major_release,omitempty"`
	OnMinorRelease    bool   `json:"on_minor_release,omitempty"`
	OnSecurityPatch   bool   `json:"on_security_patch,omitempty"`
	VersionPattern    string `json:"version_pattern,omitempty"`
	WaitForAllSources bool   `json:"wait_for_all_sources,omitempty"`
}
```

**Step 2: Run vet**

Run: `go vet ./internal/models/...`
Expected: PASS

**Step 3: Commit**

```bash
git add internal/models/project.go
git commit -m "feat(models): add WaitForAllSources to AgentRules"
```

---

### Task 4: Update EnqueueAgentRun to pass version

**Files:**
- Modify: `internal/routing/worker.go` (NotifyStore interface + checkAgentRules caller)
- Modify: `internal/api/pgstore.go` (EnqueueAgentRun implementation)
- Modify: `internal/routing/worker_test.go` (mock + tests)

**Step 1: Update the `NotifyStore` interface**

In `internal/routing/worker.go`, change the signature:

```go
EnqueueAgentRun(ctx context.Context, projectID, trigger, version string) error
```

**Step 2: Update the `checkAgentRules` caller**

In `internal/routing/worker.go:checkAgentRules`, update the call (approx line 132):

```go
if err := w.store.EnqueueAgentRun(ctx, source.ProjectID, trigger, release.Version); err != nil {
```

**Step 3: Update `PgStore.EnqueueAgentRun`**

In `internal/api/pgstore.go`, update the function to accept and store the version:

```go
func (s *PgStore) EnqueueAgentRun(ctx context.Context, projectID, trigger, version string) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback(ctx)

	var runID string
	err = tx.QueryRow(ctx,
		`INSERT INTO agent_runs (project_id, trigger, version, status)
		 VALUES ($1, $2, $3, 'pending')
		 RETURNING id`, projectID, trigger, version,
	).Scan(&runID)
	if err != nil {
		return fmt.Errorf("insert agent run: %w", err)
	}

	if s.river != nil {
		_, err = s.river.InsertTx(ctx, tx, queue.AgentJobArgs{
			AgentRunID: runID,
			ProjectID:  projectID,
			Version:    version,
		}, nil)
		if err != nil {
			return fmt.Errorf("enqueue agent job: %w", err)
		}
	}

	return tx.Commit(ctx)
}
```

**Step 4: Update mock in `worker_test.go`**

In `internal/routing/worker_test.go`, update the mock's `EnqueueAgentRun` method:

```go
func (m *mockNotifyStore) EnqueueAgentRun(_ context.Context, projectID, trigger, version string) error {
```

**Step 5: Run tests**

Run: `go test ./internal/routing/ -v`
Expected: PASS

**Step 6: Commit**

```bash
git add internal/routing/worker.go internal/api/pgstore.go internal/routing/worker_test.go
git commit -m "feat(routing): pass version through EnqueueAgentRun"
```

---

### Task 5: Add HasReleaseForVersion store method

**Files:**
- Modify: `internal/api/pgstore.go`
- Create: `internal/api/pgstore_agent_test.go` (or add to existing test file)

**Step 1: Write the failing test**

Create or add to `internal/api/pgstore_agent_test.go`. Since this requires a real DB, add the query to the `OrchestratorStore` interface and test via the mock pattern. Instead, add to the `OrchestratorStore` interface in `internal/agent/orchestrator.go`:

```go
type OrchestratorStore interface {
	AgentDataStore
	GetProject(ctx context.Context, id string) (*models.Project, error)
	GetAgentRun(ctx context.Context, id string) (*models.AgentRun, error)
	UpdateAgentRunStatus(ctx context.Context, id, status string) error
	CreateSemanticRelease(ctx context.Context, sr *models.SemanticRelease, releaseIDs []string) error
	UpdateAgentRunResult(ctx context.Context, id string, semanticReleaseID string) error
	ListProjectSubscriptions(ctx context.Context, projectID string) ([]models.Subscription, error)
	GetChannel(ctx context.Context, id string) (*models.NotificationChannel, error)
	ListSourcesByProject(ctx context.Context, projectID string, page, perPage int) ([]models.Source, int, error)
	HasReleaseForVersion(ctx context.Context, sourceID, version string) (bool, error)
}
```

**Step 2: Implement `HasReleaseForVersion` in PgStore**

In `internal/api/pgstore.go`, add:

```go
// HasReleaseForVersion checks if a source has a release matching the given version.
func (s *PgStore) HasReleaseForVersion(ctx context.Context, sourceID, version string) (bool, error) {
	var exists bool
	err := s.pool.QueryRow(ctx,
		`SELECT EXISTS(SELECT 1 FROM releases WHERE source_id = $1 AND version = $2)`,
		sourceID, version,
	).Scan(&exists)
	if err != nil {
		return false, fmt.Errorf("check release for version: %w", err)
	}
	return exists, nil
}
```

**Step 3: Add mock method to orchestrator test**

In `internal/agent/orchestrator_test.go`, add to `mockOrchestratorStore`:

```go
func (m *mockOrchestratorStore) ListSourcesByProject(_ context.Context, projectID string, page, perPage int) ([]models.Source, int, error) {
	// Return empty by default; individual tests can populate
	return nil, 0, nil
}

func (m *mockOrchestratorStore) HasReleaseForVersion(_ context.Context, sourceID, version string) (bool, error) {
	return true, nil
}
```

**Step 4: Run vet + compile**

Run: `go vet ./internal/...`
Expected: PASS

**Step 5: Commit**

```bash
git add internal/agent/orchestrator.go internal/api/pgstore.go internal/agent/orchestrator_test.go
git commit -m "feat(store): add HasReleaseForVersion and ListSourcesByProject to OrchestratorStore"
```

---

### Task 6: Update DefaultInstruction with version placeholder

**Files:**
- Modify: `internal/agent/orchestrator.go`

**Step 1: Write the failing test**

Add to `internal/agent/orchestrator_test.go`:

```go
func TestVersionPlaceholderSubstitution(t *testing.T) {
	instruction := DefaultInstruction
	if !strings.Contains(instruction, "{{VERSION}}") {
		t.Fatal("DefaultInstruction must contain {{VERSION}} placeholder")
	}

	replaced := strings.ReplaceAll(instruction, "{{VERSION}}", "v1.10.15")
	if strings.Contains(replaced, "{{VERSION}}") {
		t.Fatal("replacement failed: still contains {{VERSION}}")
	}
	if !strings.Contains(replaced, "v1.10.15") {
		t.Fatal("replacement failed: does not contain v1.10.15")
	}
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/agent/ -run TestVersionPlaceholderSubstitution -v`
Expected: FAIL — current `DefaultInstruction` has no `{{VERSION}}`

**Step 3: Replace DefaultInstruction**

In `internal/agent/orchestrator.go`, replace `DefaultInstruction`:

```go
const DefaultInstruction = `You are a release intelligence agent analyzing version {{VERSION}} of a software project.

Focus ONLY on version {{VERSION}}. Cross-check this version across all available sources.

Use the available tools to:
1. Fetch releases and find the one matching {{VERSION}} from each source.
2. Inspect release details (changelogs, commit data, raw payloads) for {{VERSION}} only.
3. Check binary/image availability directly from the source data.
4. Review the project's context sources (runbooks, documentation) for relevant background.
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
}`
```

**Step 4: Update the user message**

In `executeAgent()` (approx line 210), change the user message:

```go
userMsg := genai.NewContentFromText(
	fmt.Sprintf("Analyze version %s for this project. Cross-check all sources and produce a semantic release report.", version),
	"user",
)
```

**Step 5: Update version substitution in `BuildAgent`**

In `BuildAgent()`, add the version parameter and substitute:

```go
func BuildAgent(ctx context.Context, store AgentDataStore, project *models.Project, llmConfig LLMConfig, version string) (agent.Agent, error) {
	instruction := DefaultInstruction
	if project.AgentPrompt != "" {
		instruction = project.AgentPrompt + "\n\n" + instruction
	}
	// Substitute version placeholder
	instruction = strings.ReplaceAll(instruction, "{{VERSION}}", version)
	// ... rest unchanged
}
```

**Step 6: Run tests**

Run: `go test ./internal/agent/ -run TestVersionPlaceholderSubstitution -v`
Expected: PASS

**Step 7: Commit**

```bash
git add internal/agent/orchestrator.go internal/agent/orchestrator_test.go
git commit -m "feat(agent): version-scoped prompt with {{VERSION}} placeholder"
```

---

### Task 7: Refactor BuildAgent to sub-agent architecture with Google Search

**Files:**
- Modify: `internal/agent/orchestrator.go`

**Step 1: Write the test**

Add to `internal/agent/orchestrator_test.go`:

```go
func TestBuildAgentReturnsAgent(t *testing.T) {
	// BuildAgent requires a real LLM model which needs API keys.
	// We test that the function signature accepts the version parameter
	// and that it's callable. The actual LLM integration is tested via
	// integration tests.
	//
	// This test verifies the function compiles with the new signature.
	_ = func() {
		var store AgentDataStore
		var project models.Project
		var cfg LLMConfig
		_, _ = BuildAgent(context.Background(), store, &project, cfg, "v1.0.0")
	}
}
```

**Step 2: Update BuildAgent imports**

Add to the import block in `orchestrator.go`:

```go
"google.golang.org/adk/tool/agenttool"
"google.golang.org/adk/tool/geminitool"
```

**Step 3: Rewrite `BuildAgent` with sub-agent architecture**

```go
func BuildAgent(ctx context.Context, store AgentDataStore, project *models.Project, llmConfig LLMConfig, version string) (agent.Agent, error) {
	instruction := DefaultInstruction
	if project.AgentPrompt != "" {
		instruction = project.AgentPrompt + "\n\n" + instruction
	}
	instruction = strings.ReplaceAll(instruction, "{{VERSION}}", version)

	llmModel, err := NewLLMModel(ctx, llmConfig)
	if err != nil {
		return nil, fmt.Errorf("create LLM model: %w", err)
	}

	// Create project-scoped function tools.
	functionTools, err := NewTools(store, project.ID)
	if err != nil {
		return nil, fmt.Errorf("create agent tools: %w", err)
	}

	// Data sub-agent: handles DB queries for releases and context sources.
	dataAgent, err := llmagent.New(llmagent.Config{
		Name:        "data_agent",
		Description: "Query project releases and context sources from the database. Use this to fetch release lists, release details, and context sources like runbooks and documentation.",
		Model:       llmModel,
		Tools:       functionTools,
	})
	if err != nil {
		return nil, fmt.Errorf("create data sub-agent: %w", err)
	}

	// Build the root agent tools list.
	rootTools := []tool.Tool{
		agenttool.New("data_agent", dataAgent),
	}

	// Search sub-agent: Google Search grounding (Gemini only).
	if llmConfig.Provider == "gemini" || llmConfig.Provider == "" {
		searchAgent, err := llmagent.New(llmagent.Config{
			Name:        "search_agent",
			Description: "Search the web for additional context about a release. Use this ONLY when you need information not available from the project's sources, such as community sentiment, security advisories, network adoption statistics, or known issues.",
			Model:       llmModel,
			Tools:       []tool.Tool{geminitool.GoogleSearch{}},
		})
		if err != nil {
			return nil, fmt.Errorf("create search sub-agent: %w", err)
		}
		rootTools = append(rootTools, agenttool.New("search_agent", searchAgent))
	}

	// Root agent orchestrates data lookup and optional web search.
	return llmagent.New(llmagent.Config{
		Name:        "release_analyst",
		Description: "Analyzes upstream releases and produces semantic release reports.",
		Model:       llmModel,
		Instruction: instruction,
		Tools:       rootTools,
	})
}
```

**Step 4: Run vet + compile**

Run: `go vet ./internal/agent/...`
Expected: PASS (unused import `geminitool` and `agenttool` now used)

**Step 5: Commit**

```bash
git add internal/agent/orchestrator.go internal/agent/orchestrator_test.go
git commit -m "feat(agent): sub-agent architecture with Google Search for Gemini"
```

---

### Task 8: Update orchestrator to use version from AgentRun

**Files:**
- Modify: `internal/agent/orchestrator.go`
- Modify: `internal/agent/worker.go`

**Step 1: Update `executeAgent` to use version**

In `orchestrator.go:executeAgent`, update the `BuildAgent` call and user message to use `run.Version`:

```go
func (o *Orchestrator) executeAgent(ctx context.Context, run *models.AgentRun) (*agentResult, error) {
	// ... load project (unchanged) ...

	version := run.Version
	if version == "" {
		// Fallback: parse from trigger "auto:version:v1.10.15"
		if strings.HasPrefix(run.Trigger, "auto:version:") {
			version = strings.TrimPrefix(run.Trigger, "auto:version:")
		}
	}

	// Build the agent using the shared constructor.
	agentInstance, err := BuildAgent(ctx, o.store, project, o.llmConfig, version)
	// ... rest unchanged, but update userMsg ...

	userMsg := genai.NewContentFromText(
		fmt.Sprintf("Analyze version %s for this project. Cross-check all sources and produce a semantic release report.", version),
		"user",
	)

	// ... run agent, parse report ...

	// Use the target version instead of guessing from latest release
	sr := &models.SemanticRelease{
		ProjectID: run.ProjectID,
		Version:   version,
		Report:    reportJSON,
		Status:    "completed",
		CompletedAt: &now,
	}

	// ... rest unchanged ...
}
```

**Step 2: Update worker to pass version from job args**

In `internal/agent/worker.go`, the worker already loads the `AgentRun` from DB which now has the `Version` field. The `AgentRun` is loaded by `GetAgentRun` which reads from the `agent_runs` table. Ensure the `GetAgentRun` query includes the `version` column.

Check `internal/api/pgstore.go:GetAgentRun` and add `version` to the SELECT clause:

```go
func (s *PgStore) GetAgentRun(ctx context.Context, id string) (*models.AgentRun, error) {
	// Add version to the SELECT and Scan
}
```

**Step 3: Run vet**

Run: `go vet ./internal/...`
Expected: PASS

**Step 4: Commit**

```bash
git add internal/agent/orchestrator.go internal/agent/worker.go internal/api/pgstore.go
git commit -m "feat(agent): use version from AgentRun for version-scoped analysis"
```

---

### Task 9: Implement multi-source waiting in the worker

**Files:**
- Modify: `internal/agent/worker.go`
- Modify: `internal/agent/orchestrator.go`

**Step 1: Write the test**

Add to `internal/agent/orchestrator_test.go`:

```go
func TestCheckAllSourcesReady(t *testing.T) {
	tests := []struct {
		name     string
		sources  []models.Source
		hasMap   map[string]bool
		version  string
		expected bool
	}{
		{
			name:     "no sources",
			sources:  nil,
			version:  "v1.0.0",
			expected: true,
		},
		{
			name: "all sources have version",
			sources: []models.Source{
				{ID: "s1"}, {ID: "s2"},
			},
			hasMap:   map[string]bool{"s1": true, "s2": true},
			version:  "v1.0.0",
			expected: true,
		},
		{
			name: "one source missing",
			sources: []models.Source{
				{ID: "s1"}, {ID: "s2"},
			},
			hasMap:   map[string]bool{"s1": true, "s2": false},
			version:  "v1.0.0",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			store := &mockOrchestratorStore{
				sources:       tt.sources,
				hasReleaseMap: tt.hasMap,
			}
			o := &Orchestrator{store: store}
			got, err := o.checkAllSourcesReady(context.Background(), "proj-1", tt.version)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tt.expected {
				t.Errorf("got %v, want %v", got, tt.expected)
			}
		})
	}
}
```

Update the mock store to support the new fields:

```go
type mockOrchestratorStore struct {
	// ... existing fields ...
	sources       []models.Source
	hasReleaseMap map[string]bool  // sourceID -> has release
}

func (m *mockOrchestratorStore) ListSourcesByProject(_ context.Context, projectID string, page, perPage int) ([]models.Source, int, error) {
	return m.sources, len(m.sources), nil
}

func (m *mockOrchestratorStore) HasReleaseForVersion(_ context.Context, sourceID, version string) (bool, error) {
	if m.hasReleaseMap != nil {
		return m.hasReleaseMap[sourceID], nil
	}
	return true, nil
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/agent/ -run TestCheckAllSourcesReady -v`
Expected: FAIL — `checkAllSourcesReady` doesn't exist

**Step 3: Implement `checkAllSourcesReady`**

Add to `internal/agent/orchestrator.go`:

```go
// checkAllSourcesReady returns true if every source in the project has a
// release matching the target version.
func (o *Orchestrator) checkAllSourcesReady(ctx context.Context, projectID, version string) (bool, error) {
	sources, _, err := o.store.ListSourcesByProject(ctx, projectID, 1, 100)
	if err != nil {
		return false, fmt.Errorf("list sources: %w", err)
	}
	if len(sources) == 0 {
		return true, nil
	}

	for _, src := range sources {
		has, err := o.store.HasReleaseForVersion(ctx, src.ID, version)
		if err != nil {
			return false, fmt.Errorf("check source %s: %w", src.ID, err)
		}
		if !has {
			slog.Info("agent: source not ready for version",
				"project_id", projectID,
				"source_id", src.ID,
				"version", version,
			)
			return false, nil
		}
	}
	return true, nil
}
```

**Step 4: Run test**

Run: `go test ./internal/agent/ -run TestCheckAllSourcesReady -v`
Expected: PASS

**Step 5: Implement worker waiting logic**

In `internal/agent/worker.go`, update the `Work` method:

```go
func (w *AgentWorker) Work(ctx context.Context, job *river.Job[queue.AgentJobArgs]) error {
	slog.Info("agent worker picked up job",
		"job_id", job.ID,
		"agent_run_id", job.Args.AgentRunID,
		"version", job.Args.Version,
		"attempt", job.Attempt,
	)

	run, err := w.store.GetAgentRun(ctx, job.Args.AgentRunID)
	if err != nil {
		slog.Error("agent worker failed to load agent run",
			"job_id", job.ID,
			"agent_run_id", job.Args.AgentRunID,
			"err", err,
		)
		return fmt.Errorf("get agent run: %w", err)
	}

	// Check multi-source waiting if configured.
	if run.Version != "" {
		project, err := w.store.GetProject(ctx, run.ProjectID)
		if err != nil {
			return fmt.Errorf("get project for wait check: %w", err)
		}

		var rules models.AgentRules
		if len(project.AgentRules) > 0 {
			if err := json.Unmarshal(project.AgentRules, &rules); err != nil {
				slog.Error("unmarshal agent rules for wait check", "err", err)
			}
		}

		if rules.WaitForAllSources {
			ready, err := w.orchestrator.checkAllSourcesReady(ctx, run.ProjectID, run.Version)
			if err != nil {
				return fmt.Errorf("check sources ready: %w", err)
			}
			if !ready {
				slog.Info("agent: not all sources ready, snoozing",
					"project_id", run.ProjectID,
					"version", run.Version,
					"attempt", job.Attempt,
				)
				if err := w.store.UpdateAgentRunStatus(ctx, run.ID, "waiting"); err != nil {
					slog.Error("failed to set waiting status", "err", err)
				}
				return river.JobSnooze(5 * time.Minute)
			}
		}
	}

	slog.Info("agent worker starting run",
		"job_id", job.ID,
		"agent_run_id", run.ID,
		"project_id", run.ProjectID,
		"version", run.Version,
		"trigger", run.Trigger,
	)

	if err := w.orchestrator.RunAgent(ctx, run); err != nil {
		slog.Error("agent worker run failed",
			"job_id", job.ID,
			"agent_run_id", run.ID,
			"project_id", run.ProjectID,
			"err", err,
		)
		return err
	}

	slog.Info("agent worker run completed",
		"job_id", job.ID,
		"agent_run_id", run.ID,
		"project_id", run.ProjectID,
	)
	return nil
}
```

Add `"encoding/json"` and `"time"` to the import block in `worker.go`.

**Step 6: Run all agent tests**

Run: `go test ./internal/agent/ -v`
Expected: PASS

**Step 7: Commit**

```bash
git add internal/agent/orchestrator.go internal/agent/orchestrator_test.go internal/agent/worker.go
git commit -m "feat(agent): multi-source waiting with river.JobSnooze"
```

---

### Task 10: Update parseReport for new schema + backward compat

**Files:**
- Modify: `internal/agent/orchestrator.go`

**Step 1: Write the test**

Add to `internal/agent/orchestrator_test.go`:

```go
func TestParseReport_NewFormat(t *testing.T) {
	input := `{
		"subject": "🚀 Ready to Deploy: Geth v1.10.15",
		"risk_level": "CRITICAL",
		"risk_reason": "Hard Fork detected",
		"status_checks": ["Docker Image Verified"],
		"changelog_summary": "Fixes sync bug",
		"availability": "GA",
		"adoption": "12% updated",
		"urgency": "Critical",
		"recommendation": "Wait for 25% adoption.",
		"download_commands": ["docker pull ethereum/client-go:v1.10.15"],
		"download_links": ["https://example.com/release"]
	}`

	report, err := parseReport(input)
	if err != nil {
		t.Fatalf("parseReport: %v", err)
	}
	if report.Subject != "🚀 Ready to Deploy: Geth v1.10.15" {
		t.Errorf("subject: got %q", report.Subject)
	}
	if report.RiskLevel != "CRITICAL" {
		t.Errorf("risk_level: got %q", report.RiskLevel)
	}
	if len(report.StatusChecks) != 1 {
		t.Errorf("status_checks: got %d", len(report.StatusChecks))
	}
}

func TestParseReport_OldFormat(t *testing.T) {
	input := `{
		"summary": "Major changes across releases",
		"availability": "GA",
		"adoption": "Immediate",
		"urgency": "High",
		"recommendation": "Upgrade now"
	}`

	report, err := parseReport(input)
	if err != nil {
		t.Fatalf("parseReport: %v", err)
	}
	if report.Summary != "Major changes across releases" {
		t.Errorf("summary: got %q", report.Summary)
	}
}
```

**Step 2: Run tests**

Run: `go test ./internal/agent/ -run TestParseReport -v`
Expected: PASS (the new fields are in the struct, parseReport uses json.Unmarshal, and the old `summary` field check still validates)

Update `parseReport` to accept either `subject` or `summary` as the required field:

```go
func parseReport(text string) (*models.SemanticReport, error) {
	cleaned := strings.TrimSpace(text)
	if strings.HasPrefix(cleaned, "```") {
		if idx := strings.Index(cleaned, "\n"); idx != -1 {
			cleaned = cleaned[idx+1:]
		}
		if idx := strings.LastIndex(cleaned, "```"); idx != -1 {
			cleaned = cleaned[:idx]
		}
		cleaned = strings.TrimSpace(cleaned)
	}

	var report models.SemanticReport
	if err := json.Unmarshal([]byte(cleaned), &report); err != nil {
		return nil, fmt.Errorf("parse report JSON: %w", err)
	}

	// Accept either new format (subject) or old format (summary)
	if report.Subject == "" && report.Summary == "" {
		return nil, fmt.Errorf("report is missing required 'subject' or 'summary' field")
	}

	return &report, nil
}
```

**Step 3: Run all tests**

Run: `go test ./internal/agent/ -v`
Expected: PASS

**Step 4: Commit**

```bash
git add internal/agent/orchestrator.go internal/agent/orchestrator_test.go
git commit -m "feat(agent): parseReport accepts new SRE format and old format"
```

---

### Task 11: Update GetAgentRun query to include version column

**Files:**
- Modify: `internal/api/pgstore.go`

**Step 1: Find and update the GetAgentRun query**

Locate the `GetAgentRun` method in `internal/api/pgstore.go` and add `version` to the SELECT clause and `Scan` call.

The query should become:
```sql
SELECT id, project_id, semantic_release_id, trigger, version, status, prompt_used, error, started_at, completed_at, created_at
FROM agent_runs WHERE id = $1
```

And the Scan should include `&run.Version`.

**Step 2: Run vet**

Run: `go vet ./internal/...`
Expected: PASS

**Step 3: Commit**

```bash
git add internal/api/pgstore.go
git commit -m "fix(store): include version column in GetAgentRun query"
```

---

### Task 12: Full compilation check and existing test pass

**Files:** None (verification only)

**Step 1: Build**

Run: `go build ./cmd/server`
Expected: Compiles successfully

**Step 2: Run all tests**

Run: `go test ./...`
Expected: All tests pass

**Step 3: Run vet**

Run: `go vet ./...`
Expected: No issues

**Step 4: Final commit if any fixups needed**

```bash
git add -A
git commit -m "fix: compilation and test fixups for version-focused agent"
```
