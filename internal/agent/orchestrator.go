package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"google.golang.org/genai"

	"google.golang.org/adk/agent"
	"google.golang.org/adk/agent/llmagent"
	"google.golang.org/adk/runner"
	"google.golang.org/adk/session"
	"google.golang.org/adk/tool"
	"google.golang.org/adk/tool/agenttool"
	"google.golang.org/adk/tool/geminitool"

	"github.com/sentioxyz/changelogue/internal/models"
	"github.com/sentioxyz/changelogue/internal/routing"
)

// OrchestratorStore defines all data access methods required by the agent
// orchestrator to load project configuration, run the agent, and persist
// the resulting semantic release.
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
  "subject": "Ready to Deploy: <Project> <Version> (<Risk Summary>)",
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

// BuildAgent creates the ADK-Go LLM agent for a given project. This is used
// by both the production orchestrator and the dev entrypoint to ensure the
// same agent configuration is used everywhere.
func BuildAgent(ctx context.Context, store AgentDataStore, project *models.Project, llmConfig LLMConfig, version string) (agent.Agent, error) {
	instruction := DefaultInstruction
	if project.AgentPrompt != "" {
		instruction = project.AgentPrompt + "\n\n" + instruction
	}
	// Substitute version placeholder
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

	// For Gemini: use sub-agent pattern (required because Gemini can't mix
	// grounding tools like GoogleSearch with function tools on the same agent).
	// For OpenAI: flat architecture — give function tools directly to root agent
	// (OpenAI doesn't support the agenttool schema).
	if llmConfig.Provider == "openai" {
		return llmagent.New(llmagent.Config{
			Name:        "release_analyst",
			Description: "Analyzes upstream releases and produces semantic release reports.",
			Model:       llmModel,
			Instruction: instruction,
			Tools:       functionTools,
		})
	}

	// Gemini path: sub-agent architecture.

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

	// Search sub-agent: Google Search grounding.
	searchAgent, err := llmagent.New(llmagent.Config{
		Name:        "search_agent",
		Description: "Search the web for additional context about a release. Use this ONLY when you need information not available from the project's sources, such as community sentiment, security advisories, network adoption statistics, or known issues.",
		Model:       llmModel,
		Tools:       []tool.Tool{geminitool.GoogleSearch{}},
	})
	if err != nil {
		return nil, fmt.Errorf("create search sub-agent: %w", err)
	}

	// Root agent orchestrates data lookup and web search via sub-agents.
	return llmagent.New(llmagent.Config{
		Name:        "release_analyst",
		Description: "Analyzes upstream releases and produces semantic release reports.",
		Model:       llmModel,
		Instruction: instruction,
		Tools: []tool.Tool{
			agenttool.New(dataAgent, nil),
			agenttool.New(searchAgent, nil),
		},
	})
}

// checkAllSourcesReady returns true if every source in the project has a
// release matching the target version.
func (o *Orchestrator) checkAllSourcesReady(ctx context.Context, projectID, version string) (bool, error) {
	// Use a high limit to fetch all sources; projects rarely exceed a handful.
	sources, _, err := o.store.ListSourcesByProject(ctx, projectID, 1, 1000)
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

// Orchestrator manages the lifecycle of an agent run: loading project config,
// creating an ADK-Go agent with project-specific tools and instructions,
// running the agent, and persisting the result as a semantic release.
type Orchestrator struct {
	store     OrchestratorStore
	llmConfig LLMConfig
}

// NewOrchestrator creates a new Orchestrator. It validates that the provided
// LLMConfig has the required API key for the configured provider. If the key
// is missing, it returns an error so the caller can degrade gracefully.
func NewOrchestrator(store OrchestratorStore, cfg LLMConfig) (*Orchestrator, error) {
	switch cfg.Provider {
	case "openai":
		if cfg.OpenAIAPIKey == "" {
			return nil, fmt.Errorf("OPENAI_API_KEY is not set; agent orchestrator requires an LLM API key")
		}
	default: // gemini
		if cfg.GoogleAPIKey == "" {
			return nil, fmt.Errorf("GOOGLE_API_KEY is not set; agent orchestrator requires an LLM API key")
		}
	}
	return &Orchestrator{
		store:     store,
		llmConfig: cfg,
	}, nil
}

// RunAgent executes the full agent lifecycle for a given agent run:
//  1. Marks the run as "running".
//  2. Loads the project and builds the agent prompt.
//  3. Creates an ADK-Go LLM agent with project-scoped tools.
//  4. Runs the agent and captures its final text response.
//  5. Parses the response into a SemanticReport.
//  6. Creates a SemanticRelease and links it to the agent run.
//  7. Marks the run as "completed" (or "failed" on error).
func (o *Orchestrator) RunAgent(ctx context.Context, run *models.AgentRun) error {
	// Mark as running.
	if err := o.store.UpdateAgentRunStatus(ctx, run.ID, "running"); err != nil {
		return fmt.Errorf("update agent run status to running: %w", err)
	}

	// Execute the agent; capture any error to mark the run as failed.
	result, err := o.executeAgent(ctx, run)
	if err != nil {
		slog.Error("agent run failed", "run_id", run.ID, "err", err)
		// Best-effort: mark the run as failed.
		if statusErr := o.store.UpdateAgentRunStatus(ctx, run.ID, "failed"); statusErr != nil {
			slog.Error("agent: failed to mark run as failed",
				"run_id", run.ID, "status_err", statusErr)
		}
		return err
	}

	// Link the semantic release to the agent run and mark completed.
	if err := o.store.UpdateAgentRunResult(ctx, run.ID, result.semanticReleaseID); err != nil {
		return fmt.Errorf("update agent run result: %w", err)
	}
	if err := o.store.UpdateAgentRunStatus(ctx, run.ID, "completed"); err != nil {
		return fmt.Errorf("update agent run status to completed: %w", err)
	}

	// Send project-level notifications (best-effort; errors are logged, not returned).
	o.sendProjectNotifications(ctx, run, result)

	return nil
}

// agentResult holds the output of a successful agent execution, used to
// construct notifications after the semantic release is persisted.
type agentResult struct {
	semanticReleaseID string
	version           string
	reportText        string
	projectName       string
}

// executeAgent performs the actual LLM agent interaction and returns the
// agentResult containing the semantic release ID and metadata for notifications.
func (o *Orchestrator) executeAgent(ctx context.Context, run *models.AgentRun) (*agentResult, error) {
	// Load project.
	slog.Info("agent: loading project", "run_id", run.ID, "project_id", run.ProjectID)
	project, err := o.store.GetProject(ctx, run.ProjectID)
	if err != nil {
		return nil, fmt.Errorf("get project: %w", err)
	}
	slog.Info("agent: project loaded", "run_id", run.ID, "project", project.Name)

	// Extract target version from the agent run.
	version := run.Version
	if version == "" {
		// Fallback: parse from trigger "auto:version:v1.10.15"
		if strings.HasPrefix(run.Trigger, "auto:version:") {
			version = strings.TrimPrefix(run.Trigger, "auto:version:")
		}
	}

	// Build the agent using the shared constructor.
	slog.Info("agent: building agent", "run_id", run.ID,
		"provider", o.llmConfig.Provider, "model", o.llmConfig.Model,
		"version", version)
	agentInstance, err := BuildAgent(ctx, o.store, project, o.llmConfig, version)
	if err != nil {
		return nil, fmt.Errorf("build agent: %w", err)
	}

	// Create in-memory session service and a new session.
	sessionService := session.InMemoryService()
	createResp, err := sessionService.Create(ctx, &session.CreateRequest{
		AppName: "changelogue",
		UserID:  "system",
	})
	if err != nil {
		return nil, fmt.Errorf("create session: %w", err)
	}
	sess := createResp.Session

	// Create the runner.
	r, err := runner.New(runner.Config{
		AppName:        "changelogue",
		Agent:          agentInstance,
		SessionService: sessionService,
	})
	if err != nil {
		return nil, fmt.Errorf("create runner: %w", err)
	}

	// Run the agent with a prompt requesting analysis.
	slog.Info("agent: starting LLM run", "run_id", run.ID, "project", project.Name, "version", version)
	var userPrompt string
	if version != "" {
		userPrompt = fmt.Sprintf("Analyze version %s for this project. Cross-check all sources and produce a semantic release report.", version)
	} else {
		userPrompt = "Analyze the recent releases for this project and produce a semantic release report."
	}
	userMsg := genai.NewContentFromText(userPrompt, "user")

	var finalText string
	eventCount := 0
	for event, err := range r.Run(ctx, "system", sess.ID(), userMsg, agent.RunConfig{}) {
		if err != nil {
			slog.Error("agent: run event error",
				"run_id", run.ID,
				"event_count", eventCount,
				"err", err,
			)
			return nil, fmt.Errorf("agent run event error: %w", err)
		}
		eventCount++
		if event != nil && event.IsFinalResponse() && event.Content != nil {
			for _, part := range event.Content.Parts {
				if part.Text != "" {
					finalText += part.Text
				}
			}
		}
	}
	slog.Info("agent: LLM run finished",
		"run_id", run.ID,
		"event_count", eventCount,
		"output_length", len(finalText),
	)

	if finalText == "" {
		slog.Error("agent: produced no output",
			"run_id", run.ID,
			"project_id", run.ProjectID,
			"event_count", eventCount,
		)
		return nil, fmt.Errorf("agent produced no output")
	}

	// Parse the agent's response into a SemanticReport.
	report, err := parseReport(finalText)
	if err != nil {
		slog.Warn("agent output was not valid JSON report, storing raw",
			"run_id", run.ID, "parse_err", err, "raw_output", finalText)
		// Fall back to storing the raw text as the summary.
		report = &models.SemanticReport{
			Summary: finalText,
		}
	}

	reportJSON, err := json.Marshal(report)
	if err != nil {
		return nil, fmt.Errorf("marshal report: %w", err)
	}

	// Gather release IDs for the semantic_release_sources join table.
	releases, _, err := o.store.ListReleasesByProject(ctx, run.ProjectID, 1, 50)
	if err != nil {
		return nil, fmt.Errorf("list releases for semantic release: %w", err)
	}
	releaseIDs := make([]string, 0, len(releases))
	for _, r := range releases {
		releaseIDs = append(releaseIDs, r.ID)
	}

	// Use the target version for the semantic release.
	srVersion := version
	if srVersion == "" {
		srVersion = "unknown"
		if len(releases) > 0 {
			srVersion = releases[0].Version
		}
	}

	now := time.Now()
	sr := &models.SemanticRelease{
		ProjectID:   run.ProjectID,
		Version:     srVersion,
		Report:      reportJSON,
		Status:      "completed",
		CompletedAt: &now,
	}

	if err := o.store.CreateSemanticRelease(ctx, sr, releaseIDs); err != nil {
		return nil, fmt.Errorf("create semantic release: %w", err)
	}

	slog.Info("agent run produced semantic release",
		"run_id", run.ID,
		"semantic_release_id", sr.ID,
		"version", sr.Version,
	)

	return &agentResult{
		semanticReleaseID: sr.ID,
		version:           srVersion,
		reportText:        finalText,
		projectName:       project.Name,
	}, nil
}

// defaultSenders returns a map of channel type to Sender for the supported
// notification channel types (webhook, slack, discord).
func defaultSenders() map[string]routing.Sender {
	httpClient := &http.Client{Timeout: 10 * time.Second}
	return map[string]routing.Sender{
		"webhook": &routing.WebhookSender{Client: httpClient},
		"slack":   &routing.SlackSender{Client: httpClient},
		"discord": &routing.DiscordSender{Client: httpClient},
	}
}

// sendProjectNotifications looks up project-level subscriptions and sends a
// notification to each channel with the semantic release report. This is
// best-effort: errors are logged but do not propagate to the caller.
func (o *Orchestrator) sendProjectNotifications(ctx context.Context, run *models.AgentRun, result *agentResult) {
	subs, err := o.store.ListProjectSubscriptions(ctx, run.ProjectID)
	if err != nil {
		slog.Error("list project subscriptions for notification",
			"project_id", run.ProjectID, "err", err)
		return
	}
	if len(subs) == 0 {
		slog.Debug("agent: no project subscriptions configured",
			"project_id", run.ProjectID)
		return
	}

	senders := defaultSenders()

	msg := routing.Notification{
		Title:   fmt.Sprintf("Semantic Release Report: %s %s", result.projectName, result.version),
		Body:    result.reportText,
		Version: result.version,
	}

	for _, sub := range subs {
		ch, err := o.store.GetChannel(ctx, sub.ChannelID)
		if err != nil {
			slog.Error("get channel for project notification",
				"channel_id", sub.ChannelID, "err", err)
			continue
		}

		sender, ok := senders[ch.Type]
		if !ok {
			slog.Warn("unknown channel type for project notification", "type", ch.Type)
			continue
		}

		if err := sender.Send(ctx, ch, msg); err != nil {
			slog.Error("send project notification failed",
				"channel", ch.Name, "type", ch.Type, "err", err)
		} else {
			slog.Info("project notification sent",
				"channel", ch.Name, "project_id", run.ProjectID, "version", result.version)
		}
	}
}

// parseReport attempts to parse the agent's text output as a SemanticReport JSON.
// It handles cases where the agent wraps the JSON in markdown code blocks.
func parseReport(text string) (*models.SemanticReport, error) {
	// Strip markdown code fences if present.
	cleaned := strings.TrimSpace(text)
	if strings.HasPrefix(cleaned, "```") {
		// Remove opening fence (with optional language tag).
		if idx := strings.Index(cleaned, "\n"); idx != -1 {
			cleaned = cleaned[idx+1:]
		}
		// Remove closing fence.
		if idx := strings.LastIndex(cleaned, "```"); idx != -1 {
			cleaned = cleaned[:idx]
		}
		cleaned = strings.TrimSpace(cleaned)
	}

	var report models.SemanticReport
	if err := json.Unmarshal([]byte(cleaned), &report); err != nil {
		return nil, fmt.Errorf("parse report JSON: %w", err)
	}

	if report.Subject == "" && report.Summary == "" {
		return nil, fmt.Errorf("report is missing required 'subject' or 'summary' field")
	}

	return &report, nil
}
