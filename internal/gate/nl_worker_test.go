package gate

import (
	"context"
	"testing"

	"github.com/riverqueue/river"
	"github.com/sentioxyz/changelogue/internal/models"
	"github.com/sentioxyz/changelogue/internal/queue"
)

func TestGateNLEvalWorker_Passes(t *testing.T) {
	store := &mockGateStore{
		gate: &models.ReleaseGate{
			ID:        "gate-1",
			ProjectID: "proj-1",
			NLRule:    "Docker image must have 100 pulls",
			Enabled:   true,
		},
		readiness:  &models.VersionReadiness{ID: "vr-1", ProjectID: "proj-1", Version: "1.0.0", Status: "pending"},
		openResult: true,
	}
	eval := &stubNLEvaluator{result: true}
	w := NewGateNLEvalWorker(store, eval)
	job := &river.Job[queue.GateNLEvalJobArgs]{
		Args: queue.GateNLEvalJobArgs{VersionReadinessID: "vr-1", ProjectID: "proj-1", Version: "1.0.0"},
	}
	if err := w.Work(context.Background(), job); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if store.nlRuleUpdated == nil || !*store.nlRuleUpdated {
		t.Fatal("nl_rule_passed should be true")
	}
	if !store.gateOpened {
		t.Fatal("gate should have been opened after NL rule passed")
	}
}

func TestGateNLEvalWorker_Fails(t *testing.T) {
	store := &mockGateStore{
		gate: &models.ReleaseGate{
			ID:        "gate-1",
			ProjectID: "proj-1",
			NLRule:    "Docker image must have 100 pulls",
			Enabled:   true,
		},
		readiness: &models.VersionReadiness{ID: "vr-1", ProjectID: "proj-1", Version: "1.0.0", Status: "pending"},
	}
	eval := &stubNLEvaluator{result: false}
	w := NewGateNLEvalWorker(store, eval)
	job := &river.Job[queue.GateNLEvalJobArgs]{
		Args: queue.GateNLEvalJobArgs{VersionReadinessID: "vr-1", ProjectID: "proj-1", Version: "1.0.0"},
	}
	if err := w.Work(context.Background(), job); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if store.nlRuleUpdated == nil || *store.nlRuleUpdated {
		t.Fatal("nl_rule_passed should be false")
	}
	if store.gateOpened {
		t.Fatal("gate should NOT be opened when NL rule fails")
	}
}

type stubNLEvaluator struct {
	result bool
}

func (s *stubNLEvaluator) Evaluate(_ context.Context, _ string, _ *models.VersionReadiness) (bool, string, error) {
	return s.result, "stub response", nil
}
