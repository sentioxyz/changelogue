package gate

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

	"github.com/riverqueue/river"
	"github.com/sentioxyz/changelogue/internal/queue"
)

type GateTimeoutWorker struct {
	river.WorkerDefaults[queue.GateTimeoutJobArgs]
	store GateStore
}

func NewGateTimeoutWorker(store GateStore) *GateTimeoutWorker {
	return &GateTimeoutWorker{store: store}
}

func (w *GateTimeoutWorker) Work(ctx context.Context, _ *river.Job[queue.GateTimeoutJobArgs]) error {
	expired, err := w.store.ListExpiredGates(ctx, 100)
	if err != nil {
		return fmt.Errorf("list expired gates: %w", err)
	}

	for _, vr := range expired {
		opened, err := w.store.OpenGate(ctx, vr.ID, "timed_out")
		if err != nil {
			slog.Error("gate timeout: failed to open gate", "readiness_id", vr.ID, "err", err)
			continue
		}
		if !opened {
			continue
		}

		details, _ := json.Marshal(map[string]interface{}{
			"sources_missing": vr.SourcesMissing,
		})
		_ = w.store.RecordGateEvent(ctx, vr.ID, vr.ProjectID, vr.Version, "gate_timed_out", nil, details)

		slog.Info("gate timeout: gate force-opened",
			"project_id", vr.ProjectID,
			"version", vr.Version,
		)

		trigger := fmt.Sprintf("gate:timeout:%s", vr.Version)
		if err := w.store.EnqueueAgentRun(ctx, vr.ProjectID, trigger, vr.Version); err != nil {
			slog.Error("gate timeout: failed to enqueue agent", "err", err)
			continue
		}
		_ = w.store.MarkAgentTriggered(ctx, vr.ID)

		agentDetails, _ := json.Marshal(map[string]interface{}{"partial": true})
		_ = w.store.RecordGateEvent(ctx, vr.ID, vr.ProjectID, vr.Version, "agent_triggered", nil, agentDetails)
	}

	return nil
}
