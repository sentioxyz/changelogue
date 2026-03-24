package gate

import (
	"context"
	"testing"

	"github.com/riverqueue/river"
	"github.com/sentioxyz/changelogue/internal/models"
	"github.com/sentioxyz/changelogue/internal/queue"
)

func TestGateTimeoutWorker_SweepsExpired(t *testing.T) {
	store := &mockGateStore{
		expiredGates: []models.VersionReadiness{
			{ID: "vr-1", ProjectID: "proj-1", Version: "1.0.0", Status: "pending"},
		},
		openResult: true,
	}
	w := NewGateTimeoutWorker(store)
	job := &river.Job[queue.GateTimeoutJobArgs]{Args: queue.GateTimeoutJobArgs{}}
	if err := w.Work(context.Background(), job); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !store.gateOpened {
		t.Fatal("expired gate should have been opened")
	}
	found := false
	for _, e := range store.events {
		if e.eventType == "gate_timed_out" {
			found = true
		}
	}
	if !found {
		t.Fatal("gate_timed_out event should have been recorded")
	}
	if !store.agentRunEnqueued {
		t.Fatal("agent should have been enqueued after timeout")
	}
}

func TestGateTimeoutWorker_NoExpired(t *testing.T) {
	store := &mockGateStore{expiredGates: nil}
	w := NewGateTimeoutWorker(store)
	job := &river.Job[queue.GateTimeoutJobArgs]{Args: queue.GateTimeoutJobArgs{}}
	if err := w.Work(context.Background(), job); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if store.gateOpened {
		t.Fatal("no gates should have been opened")
	}
}
