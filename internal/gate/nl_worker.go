package gate

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

	"github.com/riverqueue/river"
	"github.com/sentioxyz/changelogue/internal/models"
	"github.com/sentioxyz/changelogue/internal/queue"
)

// NLEvaluator evaluates a natural language gate rule. Returns (passed, reason, error).
type NLEvaluator interface {
	Evaluate(ctx context.Context, rule string, readiness *models.VersionReadiness) (bool, string, error)
}

type GateNLEvalWorker struct {
	river.WorkerDefaults[queue.GateNLEvalJobArgs]
	store     GateStore
	evaluator NLEvaluator
}

func NewGateNLEvalWorker(store GateStore, evaluator NLEvaluator) *GateNLEvalWorker {
	return &GateNLEvalWorker{store: store, evaluator: evaluator}
}

// Work is the River entry point for NL gate evaluation jobs.
func (w *GateNLEvalWorker) Work(ctx context.Context, job *river.Job[queue.GateNLEvalJobArgs]) error {
	readinessID := job.Args.VersionReadinessID
	projectID := job.Args.ProjectID
	version := job.Args.Version

	// 1. Load version readiness; skip if already beyond pending.
	vr, err := w.store.GetVersionReadiness(ctx, readinessID)
	if err != nil {
		return fmt.Errorf("get version readiness: %w", err)
	}
	if vr == nil || vr.Status != "pending" {
		return nil
	}

	// 2. Load gate config.
	gate, err := w.store.GetReleaseGate(ctx, projectID)
	if err != nil {
		return fmt.Errorf("get release gate: %w", err)
	}
	if gate == nil {
		return fmt.Errorf("no release gate found for project %s", projectID)
	}

	// 3. Record nl_eval_started event.
	if recErr := w.store.RecordGateEvent(ctx, readinessID, projectID, version, "nl_eval_started", nil, nil); recErr != nil {
		slog.Error("record nl_eval_started event", "err", recErr)
	}

	// 4. Evaluate the NL rule via the LLM evaluator.
	passed, reason, err := w.evaluator.Evaluate(ctx, gate.NLRule, vr)
	if err != nil {
		return fmt.Errorf("nl evaluator: %w", err)
	}

	// 5. Persist the evaluation result.
	if err := w.store.UpdateNLRulePassed(ctx, readinessID, passed); err != nil {
		return fmt.Errorf("update nl_rule_passed: %w", err)
	}

	// 6. Record nl_eval_passed or nl_eval_failed event.
	eventType := "nl_eval_failed"
	if passed {
		eventType = "nl_eval_passed"
	}
	details, _ := json.Marshal(map[string]string{"reason": reason})
	if recErr := w.store.RecordGateEvent(ctx, readinessID, projectID, version, eventType, nil, details); recErr != nil {
		slog.Error("record nl_eval event", "event_type", eventType, "err", recErr)
	}

	// 7. If failed, log and return — gate stays pending for potential re-evaluation.
	if !passed {
		slog.Info("nl gate rule did not pass", "readiness_id", readinessID, "reason", reason)
		return nil
	}

	// 8. Open the gate (pending → ready).
	opened, err := w.store.OpenGate(ctx, readinessID, "ready")
	if err != nil {
		return fmt.Errorf("open gate: %w", err)
	}
	if !opened {
		// Already transitioned by a concurrent worker.
		return nil
	}

	// 9. Record gate_opened event.
	if recErr := w.store.RecordGateEvent(ctx, readinessID, projectID, version, "gate_opened", nil, nil); recErr != nil {
		slog.Error("record gate_opened event", "err", recErr)
	}

	// 10. Enqueue agent run — log errors, do not propagate.
	trigger := fmt.Sprintf("gate:%s", readinessID)
	if err := w.store.EnqueueAgentRun(ctx, projectID, trigger, version); err != nil {
		slog.Error("enqueue agent run for nl gate", "readiness_id", readinessID, "project_id", projectID, "err", err)
		return nil
	}

	// 11. Mark agent triggered.
	if err := w.store.MarkAgentTriggered(ctx, readinessID); err != nil {
		slog.Error("mark agent triggered", "readiness_id", readinessID, "err", err)
	}

	// 12. Record agent_triggered event.
	agentDetails, _ := json.Marshal(map[string]string{"trigger": trigger})
	if recErr := w.store.RecordGateEvent(ctx, readinessID, projectID, version, "agent_triggered", nil, agentDetails); recErr != nil {
		slog.Error("record agent_triggered event", "err", recErr)
	}

	return nil
}
