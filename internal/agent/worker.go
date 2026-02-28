package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"github.com/riverqueue/river"
	"github.com/sentioxyz/changelogue/internal/models"
	"github.com/sentioxyz/changelogue/internal/queue"
)

// AgentWorker is a River worker that processes AgentJobArgs by running the
// LLM agent orchestrator. When a job is dequeued, it loads the corresponding
// agent run from the store and delegates to Orchestrator.RunAgent.
type AgentWorker struct {
	river.WorkerDefaults[queue.AgentJobArgs]
	orchestrator *Orchestrator
	store        OrchestratorStore
}

// NewAgentWorker creates a new AgentWorker backed by the given orchestrator
// and store.
func NewAgentWorker(orchestrator *Orchestrator, store OrchestratorStore) *AgentWorker {
	return &AgentWorker{
		orchestrator: orchestrator,
		store:        store,
	}
}

// Timeout overrides River's default 60-second job timeout. Agent runs involve
// multiple LLM round-trips (root agent + sub-agents), each of which can take
// 10-30 seconds, so we allow 5 minutes total.
func (w *AgentWorker) Timeout(_ *river.Job[queue.AgentJobArgs]) time.Duration {
	return 5 * time.Minute
}

// Work processes a single AgentJobArgs job. It loads the agent run from the
// database and runs the LLM agent through the orchestrator. If the project
// has WaitForAllSources enabled and not all sources have the target version,
// the job is snoozed for 5 minutes via river.JobSnooze.
func (w *AgentWorker) Work(ctx context.Context, job *river.Job[queue.AgentJobArgs]) error {
	slog.Info("agent worker picked up job",
		"job_id", job.ID,
		"agent_run_id", job.Args.AgentRunID,
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

	// Check multi-source waiting: if the run has a version and the project
	// has WaitForAllSources enabled, verify all sources have reported a
	// release for the target version before running the agent.
	if run.Version != "" {
		project, err := w.store.GetProject(ctx, run.ProjectID)
		if err != nil {
			slog.Error("agent worker failed to load project",
				"job_id", job.ID,
				"project_id", run.ProjectID,
				"err", err,
			)
			return fmt.Errorf("get project: %w", err)
		}

		var rules models.AgentRules
		if len(project.AgentRules) > 0 {
			if err := json.Unmarshal(project.AgentRules, &rules); err != nil {
				slog.Warn("agent worker: failed to unmarshal agent rules, proceeding without wait",
					"project_id", run.ProjectID, "err", err)
			}
		}

		if rules.WaitForAllSources {
			ready, err := w.orchestrator.checkAllSourcesReady(ctx, run.ProjectID, run.Version)
			if err != nil {
				slog.Error("agent worker: error checking source readiness",
					"job_id", job.ID,
					"project_id", run.ProjectID,
					"version", run.Version,
					"err", err,
				)
				return fmt.Errorf("check all sources ready: %w", err)
			}
			if !ready {
				// After maxWaitAttempts (12 × 5min = ~1 hour), proceed
				// with partial data rather than waiting forever.
				const maxWaitAttempts = 12
				if job.Attempt >= maxWaitAttempts {
					slog.Warn("agent worker: max wait attempts reached, proceeding with partial data",
						"job_id", job.ID,
						"project_id", run.ProjectID,
						"version", run.Version,
						"attempts", job.Attempt,
					)
				} else {
					slog.Info("agent worker: not all sources ready, snoozing",
						"job_id", job.ID,
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
	}

	slog.Info("agent worker starting run",
		"job_id", job.ID,
		"agent_run_id", run.ID,
		"project_id", run.ProjectID,
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
