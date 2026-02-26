package agent

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/riverqueue/river"
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

// Work processes a single AgentJobArgs job. It loads the agent run from the
// database and runs the LLM agent through the orchestrator.
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
