package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"strings"
	"time"

	"google.golang.org/genai"

	"google.golang.org/adk/agent"
	"google.golang.org/adk/agent/llmagent"
	"google.golang.org/adk/model/gemini"
	"google.golang.org/adk/runner"
	"google.golang.org/adk/session"
	"google.golang.org/adk/tool"

	"github.com/sentioxyz/releaseguard/internal/models"
	"github.com/sentioxyz/releaseguard/internal/routing"
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
}

const defaultModel = "gemini-2.5-flash"

const defaultInstruction = `You are a release intelligence agent for a software project.
Your job is to analyze recent upstream releases from the project's tracked sources
and produce a structured semantic release report.

Use the available tools to:
1. Fetch the list of recent releases for this project.
2. Inspect individual release details (changelogs, commit data, raw payloads).
3. Review the project's context sources (runbooks, documentation) for background.

After your research, produce a JSON report with exactly these fields:
{
  "summary": "A 2-3 sentence high-level summary of what changed across the releases.",
  "availability": "Are these releases available and stable? (e.g., 'GA', 'RC', 'Beta')",
  "adoption": "What is the recommended adoption timeline? (e.g., 'Immediate', 'Next sprint', 'Monitor')",
  "urgency": "How urgent is upgrading? (e.g., 'Critical', 'High', 'Medium', 'Low')",
  "recommendation": "A concrete 1-2 sentence recommendation for the team."
}

Return ONLY the JSON object, with no markdown formatting or extra text.`

// Orchestrator manages the lifecycle of an agent run: loading project config,
// creating an ADK-Go agent with project-specific tools and instructions,
// running the agent, and persisting the result as a semantic release.
type Orchestrator struct {
	store  OrchestratorStore
	apiKey string
}

// NewOrchestrator creates a new Orchestrator. It reads GOOGLE_API_KEY from the
// environment. If the key is missing, it returns an error so the caller can
// degrade gracefully (log a warning, skip registering the agent worker).
func NewOrchestrator(store OrchestratorStore) (*Orchestrator, error) {
	apiKey := os.Getenv("GOOGLE_API_KEY")
	if apiKey == "" {
		return nil, fmt.Errorf("GOOGLE_API_KEY is not set; agent orchestrator requires an LLM API key")
	}
	return &Orchestrator{
		store:  store,
		apiKey: apiKey,
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
		// Best-effort: mark the run as failed (ignore status update error).
		_ = o.store.UpdateAgentRunStatus(ctx, run.ID, "failed")
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
	project, err := o.store.GetProject(ctx, run.ProjectID)
	if err != nil {
		return nil, fmt.Errorf("get project: %w", err)
	}

	// Build instruction from project's agent_prompt or use default.
	instruction := defaultInstruction
	if project.AgentPrompt != "" {
		instruction = project.AgentPrompt + "\n\n" + defaultInstruction
	}

	// Create the Gemini model.
	llmModel, err := gemini.NewModel(ctx, defaultModel, &genai.ClientConfig{
		APIKey: o.apiKey,
	})
	if err != nil {
		return nil, fmt.Errorf("create LLM model: %w", err)
	}

	// Create project-scoped tools.
	tools, err := NewTools(o.store, run.ProjectID)
	if err != nil {
		return nil, fmt.Errorf("create agent tools: %w", err)
	}

	// Create the ADK-Go LLM agent.
	agentInstance, err := llmagent.New(llmagent.Config{
		Name:        "release_analyst",
		Description: "Analyzes upstream releases and produces semantic release reports.",
		Model:       llmModel,
		Instruction: instruction,
		Tools:       tools,
	})
	if err != nil {
		return nil, fmt.Errorf("create LLM agent: %w", err)
	}

	// Create in-memory session service and a new session.
	sessionService := session.InMemoryService()
	createResp, err := sessionService.Create(ctx, &session.CreateRequest{
		AppName: "releaseguard",
		UserID:  "system",
	})
	if err != nil {
		return nil, fmt.Errorf("create session: %w", err)
	}
	sess := createResp.Session

	// Create the runner.
	r, err := runner.New(runner.Config{
		AppName:        "releaseguard",
		Agent:          agentInstance,
		SessionService: sessionService,
	})
	if err != nil {
		return nil, fmt.Errorf("create runner: %w", err)
	}

	// Run the agent with a prompt requesting analysis.
	userMsg := genai.NewContentFromText(
		"Analyze the recent releases for this project and produce a semantic release report.",
		"user",
	)

	var finalText string
	for event, err := range r.Run(ctx, "system", sess.ID(), userMsg, agent.RunConfig{}) {
		if err != nil {
			return nil, fmt.Errorf("agent run event error: %w", err)
		}
		if event != nil && event.IsFinalResponse() && event.Content != nil {
			for _, part := range event.Content.Parts {
				if part.Text != "" {
					finalText += part.Text
				}
			}
		}
	}

	if finalText == "" {
		return nil, fmt.Errorf("agent produced no output")
	}

	// Parse the agent's response into a SemanticReport.
	report, err := parseReport(finalText)
	if err != nil {
		slog.Warn("agent output was not valid JSON report, storing raw",
			"run_id", run.ID, "parse_err", err)
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

	// Determine a version label from the most recent release.
	version := "unknown"
	if len(releases) > 0 {
		version = releases[0].Version
	}

	now := time.Now()
	sr := &models.SemanticRelease{
		ProjectID:   run.ProjectID,
		Version:     version,
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
		version:           version,
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

	if report.Summary == "" {
		return nil, fmt.Errorf("report is missing required 'summary' field")
	}

	return &report, nil
}

// toolsToSlice is a helper to convert the tool.Tool interface slice for use
// with llmagent.Config. This is kept for API compatibility in case the
// function signature changes.
func toolsToSlice(tools []tool.Tool) []tool.Tool {
	return tools
}
