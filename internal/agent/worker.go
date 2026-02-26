package agent

import (
	"context"
	"fmt"

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
	run, err := w.store.GetAgentRun(ctx, job.Args.AgentRunID)
	if err != nil {
		return fmt.Errorf("get agent run: %w", err)
	}
	return w.orchestrator.RunAgent(ctx, run)
}
