package gate

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

	"github.com/jackc/pgx/v5"
	"github.com/riverqueue/river"
	"github.com/sentioxyz/changelogue/internal/queue"
)

// GateCheckWorker is a River worker that evaluates gate readiness each time a
// new release is ingested. It runs in the transactional outbox pattern —
// GateCheckJobArgs is enqueued alongside NotifyJobArgs whenever a release is
// ingested.
type GateCheckWorker struct {
	river.WorkerDefaults[queue.GateCheckJobArgs]
	store       GateStore
	riverClient *river.Client[pgx.Tx]
}

// NewGateCheckWorker creates a GateCheckWorker.
func NewGateCheckWorker(store GateStore, riverClient *river.Client[pgx.Tx]) *GateCheckWorker {
	return &GateCheckWorker{store: store, riverClient: riverClient}
}

// SetRiverClient sets the River client (used for enqueuing NL eval jobs).
func (w *GateCheckWorker) SetRiverClient(c *river.Client[pgx.Tx]) {
	w.riverClient = c
}

// Work is the River entry point.
func (w *GateCheckWorker) Work(ctx context.Context, job *river.Job[queue.GateCheckJobArgs]) error {
	return w.work(ctx, job.Args.SourceID, job.Args.ReleaseID, job.Args.Version)
}

// work contains the core gate-check logic and is called by Work and by tests.
func (w *GateCheckWorker) work(ctx context.Context, sourceID, _ /* releaseID */, rawVersion string) error {
	// 1. Load gate config for the project owning this source.
	gate, err := w.store.GetReleaseGateBySource(ctx, sourceID)
	if err != nil {
		return fmt.Errorf("get release gate by source: %w", err)
	}
	if gate == nil || !gate.Enabled {
		return nil
	}

	// 2. Normalize version using per-source mapping from the gate config.
	version := NormalizeVersionForSource(rawVersion, sourceID, gate.VersionMapping)

	// 3. Determine required sources.
	requiredSources := gate.RequiredSources
	if len(requiredSources) == 0 {
		sources, _, err := w.store.ListSourcesByProject(ctx, gate.ProjectID, 1, 1000)
		if err != nil {
			return fmt.Errorf("list sources by project: %w", err)
		}
		requiredSources = make([]string, len(sources))
		for i, s := range sources {
			requiredSources[i] = s.ID
		}
	}

	// 4. Upsert version readiness — atomically mark this source as met.
	vr, allMet, err := w.store.UpsertVersionReadiness(ctx, gate.ProjectID, version, sourceID, requiredSources, gate.TimeoutHours)
	if err != nil {
		return fmt.Errorf("upsert version readiness: %w", err)
	}

	// Record source_met event.
	sid := sourceID
	if recErr := w.store.RecordGateEvent(ctx, vr.ID, gate.ProjectID, version, "source_met", &sid, nil); recErr != nil {
		slog.Error("record source_met event", "err", recErr)
	}

	if !allMet {
		// Not all sources met yet — stay pending.
		return nil
	}

	// 5. All structured sources are met. Check NL rule if present.
	if gate.NLRule != "" {
		if vr.NLRulePassed == nil || !*vr.NLRulePassed {
			// NL eval not done yet (or failed). Enqueue GateNLEvalJob so the
			// LLM worker can evaluate and call back.
			if w.riverClient != nil {
				_, insertErr := w.riverClient.Insert(ctx, queue.GateNLEvalJobArgs{
					VersionReadinessID: vr.ID,
					ProjectID:          gate.ProjectID,
					Version:            version,
				}, nil)
				if insertErr != nil {
					slog.Error("enqueue gate_nl_eval job", "readiness_id", vr.ID, "err", insertErr)
				} else {
					slog.Info("enqueued gate_nl_eval job", "readiness_id", vr.ID)
				}
			}
			return nil
		}
		// NLRulePassed == true — fall through to open gate.
	}

	// 6. Open the gate (transitions from pending → ready).
	opened, err := w.store.OpenGate(ctx, vr.ID, "ready")
	if err != nil {
		return fmt.Errorf("open gate: %w", err)
	}
	if !opened {
		// Already transitioned by a concurrent worker — safe to stop.
		return nil
	}

	// 7. Record gate_opened event.
	if recErr := w.store.RecordGateEvent(ctx, vr.ID, gate.ProjectID, version, "gate_opened", nil, nil); recErr != nil {
		slog.Error("record gate_opened event", "err", recErr)
	}

	// 8. Trigger agent run.
	w.triggerAgent(ctx, vr.ID, gate.ProjectID, version)
	return nil
}

// triggerAgent enqueues an agent run and marks the readiness row as triggered.
// Errors from EnqueueAgentRun are only logged — we do not want gate jobs to
// fail because of agent-enqueue issues.
func (w *GateCheckWorker) triggerAgent(ctx context.Context, readinessID, projectID, version string) {
	trigger := fmt.Sprintf("gate:%s", readinessID)
	if err := w.store.EnqueueAgentRun(ctx, projectID, trigger, version); err != nil {
		slog.Error("enqueue agent run for gate", "readiness_id", readinessID, "project_id", projectID, "err", err)
		return
	}

	if err := w.store.MarkAgentTriggered(ctx, readinessID); err != nil {
		slog.Error("mark agent triggered", "readiness_id", readinessID, "err", err)
	}

	details, _ := json.Marshal(map[string]string{"trigger": trigger})
	if recErr := w.store.RecordGateEvent(ctx, readinessID, projectID, version, "agent_triggered", nil, details); recErr != nil {
		slog.Error("record agent_triggered event", "err", recErr)
	}
}
