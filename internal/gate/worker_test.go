package gate

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/sentioxyz/changelogue/internal/models"
)

// --- mock store ---

type mockGateStore struct {
	gate             *models.ReleaseGate
	readiness        *models.VersionReadiness
	gateOpened       bool
	agentTriggered   bool
	events           []mockGateEvent
	agentRunEnqueued bool
	nlRuleUpdated    *bool
	expiredGates     []models.VersionReadiness
	upsertReady      bool // controls UpsertVersionReadiness return
	openResult       bool // controls OpenGate return

	// Track calls for assertions.
	openGateCalled        bool
	markAgentCalledWith   string
	enqueueAgentCalledFor string
}

type mockGateEvent struct {
	eventType string
	sourceID  *string
}

func (m *mockGateStore) GetReleaseGateBySource(_ context.Context, _ string) (*models.ReleaseGate, error) {
	return m.gate, nil
}

func (m *mockGateStore) GetReleaseGate(_ context.Context, _ string) (*models.ReleaseGate, error) {
	return m.gate, nil
}

func (m *mockGateStore) UpsertVersionReadiness(_ context.Context, _, _, _ string, _ []string, _ int) (*models.VersionReadiness, bool, error) {
	return m.readiness, m.upsertReady, nil
}

func (m *mockGateStore) OpenGate(_ context.Context, _, _ string) (bool, error) {
	m.openGateCalled = true
	return m.openResult, nil
}

func (m *mockGateStore) MarkAgentTriggered(_ context.Context, readinessID string) error {
	m.agentTriggered = true
	m.markAgentCalledWith = readinessID
	return nil
}

func (m *mockGateStore) RecordGateEvent(_ context.Context, _, _, _, eventType string, sourceID *string, _ json.RawMessage) error {
	m.events = append(m.events, mockGateEvent{eventType: eventType, sourceID: sourceID})
	return nil
}

func (m *mockGateStore) ListExpiredGates(_ context.Context, _ int) ([]models.VersionReadiness, error) {
	return m.expiredGates, nil
}

func (m *mockGateStore) GetSource(_ context.Context, id string) (*models.Source, error) {
	return &models.Source{ID: id, ProjectID: "proj-1"}, nil
}

func (m *mockGateStore) GetProject(_ context.Context, id string) (*models.Project, error) {
	return &models.Project{ID: id, Name: "test-project"}, nil
}

func (m *mockGateStore) ListSourcesByProject(_ context.Context, _ string, _, _ int) ([]models.Source, int, error) {
	return []models.Source{
		{ID: "src-1", ProjectID: "proj-1"},
		{ID: "src-2", ProjectID: "proj-1"},
	}, 2, nil
}

func (m *mockGateStore) EnqueueAgentRun(_ context.Context, projectID, _, _ string) error {
	m.agentRunEnqueued = true
	m.enqueueAgentCalledFor = projectID
	return nil
}

func (m *mockGateStore) GetVersionReadiness(_ context.Context, _ string) (*models.VersionReadiness, error) {
	return m.readiness, nil
}

func (m *mockGateStore) UpdateNLRulePassed(_ context.Context, _ string, passed bool) error {
	m.nlRuleUpdated = &passed
	return nil
}

// --- helpers ---

func boolPtr(b bool) *bool { return &b }

func countEvents(events []mockGateEvent, eventType string) int {
	n := 0
	for _, e := range events {
		if e.eventType == eventType {
			n++
		}
	}
	return n
}

// --- tests ---

// TestGateCheckWorker_NoGate: No gate exists → worker returns nil (no-op)
func TestGateCheckWorker_NoGate(t *testing.T) {
	store := &mockGateStore{
		gate: nil, // no gate configured
	}
	w := NewGateCheckWorker(store, nil)

	err := w.work(context.Background(), "src-1", "rel-1", "v1.0.0")
	if err != nil {
		t.Fatalf("expected nil error when no gate exists, got %v", err)
	}
	if store.openGateCalled {
		t.Error("expected OpenGate not to be called when no gate")
	}
	if store.agentRunEnqueued {
		t.Error("expected EnqueueAgentRun not to be called when no gate")
	}
}

// TestGateCheckWorker_GateOpens: Gate exists, all sources met → gate opens, agent triggered
func TestGateCheckWorker_GateOpens(t *testing.T) {
	store := &mockGateStore{
		gate: &models.ReleaseGate{
			ID:              "gate-1",
			ProjectID:       "proj-1",
			RequiredSources: []string{"src-1", "src-2"},
			TimeoutHours:    24,
			Enabled:         true,
		},
		readiness: &models.VersionReadiness{
			ID:        "vr-1",
			ProjectID: "proj-1",
			Version:   "1.0.0",
			Status:    "pending",
		},
		upsertReady: true,  // all sources met
		openResult:  true,  // gate successfully opened
	}
	w := NewGateCheckWorker(store, nil)

	err := w.work(context.Background(), "src-1", "rel-1", "v1.0.0")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !store.openGateCalled {
		t.Error("expected OpenGate to be called")
	}
	if !store.agentRunEnqueued {
		t.Error("expected EnqueueAgentRun to be called")
	}
	if !store.agentTriggered {
		t.Error("expected MarkAgentTriggered to be called")
	}
	if countEvents(store.events, "gate_opened") != 1 {
		t.Errorf("expected 1 gate_opened event, got %d", countEvents(store.events, "gate_opened"))
	}
	if countEvents(store.events, "agent_triggered") != 1 {
		t.Errorf("expected 1 agent_triggered event, got %d", countEvents(store.events, "agent_triggered"))
	}
	if countEvents(store.events, "source_met") != 1 {
		t.Errorf("expected 1 source_met event, got %d", countEvents(store.events, "source_met"))
	}
}

// TestGateCheckWorker_GateDisabled: Gate disabled → no-op
func TestGateCheckWorker_GateDisabled(t *testing.T) {
	store := &mockGateStore{
		gate: &models.ReleaseGate{
			ID:        "gate-1",
			ProjectID: "proj-1",
			Enabled:   false, // disabled
		},
	}
	w := NewGateCheckWorker(store, nil)

	err := w.work(context.Background(), "src-1", "rel-1", "v1.0.0")
	if err != nil {
		t.Fatalf("expected nil error for disabled gate, got %v", err)
	}
	if store.openGateCalled {
		t.Error("expected OpenGate not to be called for disabled gate")
	}
	if len(store.events) != 0 {
		t.Errorf("expected no events for disabled gate, got %d", len(store.events))
	}
}

// TestGateCheckWorker_PendingWaitsForMore: Not all sources met → stays pending
func TestGateCheckWorker_PendingWaitsForMore(t *testing.T) {
	store := &mockGateStore{
		gate: &models.ReleaseGate{
			ID:              "gate-1",
			ProjectID:       "proj-1",
			RequiredSources: []string{"src-1", "src-2"},
			TimeoutHours:    24,
			Enabled:         true,
		},
		readiness: &models.VersionReadiness{
			ID:             "vr-1",
			ProjectID:      "proj-1",
			Version:        "1.0.0",
			Status:         "pending",
			SourcesMet:     []string{"src-1"},
			SourcesMissing: []string{"src-2"},
		},
		upsertReady: false, // not all sources met yet
	}
	w := NewGateCheckWorker(store, nil)

	err := w.work(context.Background(), "src-1", "rel-1", "v1.0.0")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if store.openGateCalled {
		t.Error("expected OpenGate not to be called when sources still missing")
	}
	if store.agentRunEnqueued {
		t.Error("expected EnqueueAgentRun not to be called when sources still missing")
	}
	// source_met event should still be recorded
	if countEvents(store.events, "source_met") != 1 {
		t.Errorf("expected 1 source_met event, got %d", countEvents(store.events, "source_met"))
	}
}

// TestGateCheckWorker_NLRuleEnqueuesEval: Structured rules pass, NL rule present →
// agent NOT triggered, gate NOT opened (NL eval needed first)
func TestGateCheckWorker_NLRuleEnqueuesEval(t *testing.T) {
	store := &mockGateStore{
		gate: &models.ReleaseGate{
			ID:              "gate-1",
			ProjectID:       "proj-1",
			RequiredSources: []string{"src-1"},
			TimeoutHours:    24,
			NLRule:          "Only open gate if the release includes security fixes",
			Enabled:         true,
		},
		readiness: &models.VersionReadiness{
			ID:           "vr-1",
			ProjectID:    "proj-1",
			Version:      "1.0.0",
			Status:       "pending",
			NLRulePassed: nil, // NL eval not done yet
		},
		upsertReady: true,  // all structured sources met
		openResult:  false, // gate should NOT open (NL not evaluated)
	}
	w := NewGateCheckWorker(store, nil)

	err := w.work(context.Background(), "src-1", "rel-1", "v1.0.0")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Gate must NOT be opened because NL eval is pending.
	if store.openGateCalled {
		t.Error("expected OpenGate NOT to be called while NL eval is pending")
	}
	// Agent must NOT be triggered.
	if store.agentRunEnqueued {
		t.Error("expected EnqueueAgentRun NOT to be called while NL eval is pending")
	}
	// source_met event should have been recorded.
	if countEvents(store.events, "source_met") != 1 {
		t.Errorf("expected 1 source_met event, got %d", countEvents(store.events, "source_met"))
	}
}
