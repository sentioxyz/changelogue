// internal/pipeline/runner_test.go
package pipeline

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/sentioxyz/releaseguard/internal/models"
)

// --- Mock Store ---

type mockPipelineStore struct {
	release       *models.ReleaseEvent
	getReleaseErr error
	jobID         int64
	createErr     error
	finalState    string
	finalResults  map[string]json.RawMessage
	skipReason    string
	failMsg       string
}

func (m *mockPipelineStore) GetReleasePayload(_ context.Context, _ string) (*models.ReleaseEvent, error) {
	return m.release, m.getReleaseErr
}
func (m *mockPipelineStore) CreatePipelineJob(_ context.Context, _ string) (int64, error) {
	return m.jobID, m.createErr
}
func (m *mockPipelineStore) UpdateNodeProgress(_ context.Context, _ int64, _ string, _ map[string]json.RawMessage) error {
	return nil
}
func (m *mockPipelineStore) CompletePipelineJob(_ context.Context, _ int64, nodeResults map[string]json.RawMessage) error {
	m.finalState = "completed"
	m.finalResults = nodeResults
	return nil
}
func (m *mockPipelineStore) SkipPipelineJob(_ context.Context, _ int64, reason string) error {
	m.finalState = "skipped"
	m.skipReason = reason
	return nil
}
func (m *mockPipelineStore) FailPipelineJob(_ context.Context, _ int64, errMsg string) error {
	m.finalState = "failed"
	m.failMsg = errMsg
	return nil
}

// --- Mock Node ---

type mockNode struct {
	name   string
	result json.RawMessage
	err    error
}

func (n *mockNode) Name() string { return n.name }
func (n *mockNode) Execute(_ context.Context, _ *models.ReleaseEvent, _ json.RawMessage, _ map[string]json.RawMessage) (json.RawMessage, error) {
	return n.result, n.err
}

// --- Tests ---

func TestRunnerHappyPath(t *testing.T) {
	store := &mockPipelineStore{
		release: &models.ReleaseEvent{ID: "r-1", RawVersion: "v1.0.0", Timestamp: time.Now()},
		jobID:   42,
	}

	alwaysOn := &mockNode{name: "normalizer", result: json.RawMessage(`{"parsed":true}`)}
	configurable := &mockNode{name: "scorer", result: json.RawMessage(`{"score":"HIGH"}`)}

	runner := NewRunner(store, []PipelineNode{alwaysOn}, []PipelineNode{configurable})

	config := map[string]json.RawMessage{"scorer": json.RawMessage(`{}`)}

	err := runner.Process(context.Background(), "r-1", config)
	if err != nil {
		t.Fatalf("Process: %v", err)
	}

	if store.finalState != "completed" {
		t.Errorf("state = %q, want %q", store.finalState, "completed")
	}
	if _, ok := store.finalResults["normalizer"]; !ok {
		t.Error("missing normalizer result")
	}
	if _, ok := store.finalResults["scorer"]; !ok {
		t.Error("missing scorer result")
	}
}

func TestRunnerNodeDropsEvent(t *testing.T) {
	store := &mockPipelineStore{
		release: &models.ReleaseEvent{ID: "r-1", Timestamp: time.Now()},
		jobID:   42,
	}

	dropper := &mockNode{name: "router", err: ErrEventDropped}

	runner := NewRunner(store, []PipelineNode{dropper}, nil)

	err := runner.Process(context.Background(), "r-1", nil)
	if err != nil {
		t.Fatalf("Process should not error on drop: %v", err)
	}

	if store.finalState != "skipped" {
		t.Errorf("state = %q, want %q", store.finalState, "skipped")
	}
}

func TestRunnerNodeError(t *testing.T) {
	store := &mockPipelineStore{
		release: &models.ReleaseEvent{ID: "r-1", Timestamp: time.Now()},
		jobID:   42,
	}

	failNode := &mockNode{name: "broken", err: errors.New("api timeout")}

	runner := NewRunner(store, []PipelineNode{failNode}, nil)

	err := runner.Process(context.Background(), "r-1", nil)
	if err == nil {
		t.Fatal("expected error")
	}

	if store.finalState != "failed" {
		t.Errorf("state = %q, want %q", store.finalState, "failed")
	}
}

func TestRunnerSkipsDisabledConfigurableNodes(t *testing.T) {
	store := &mockPipelineStore{
		release: &models.ReleaseEvent{ID: "r-1", Timestamp: time.Now()},
		jobID:   42,
	}

	alwaysOn := &mockNode{name: "always", result: json.RawMessage(`{}`)}
	optional := &mockNode{name: "optional", result: json.RawMessage(`{"should":"not appear"}`)}

	runner := NewRunner(store, []PipelineNode{alwaysOn}, []PipelineNode{optional})

	// Empty config — "optional" node is not enabled
	err := runner.Process(context.Background(), "r-1", nil)
	if err != nil {
		t.Fatalf("Process: %v", err)
	}

	if store.finalState != "completed" {
		t.Errorf("state = %q, want %q", store.finalState, "completed")
	}
	if _, ok := store.finalResults["optional"]; ok {
		t.Error("disabled node should not have results")
	}
}

func TestRunnerReleaseNotFound(t *testing.T) {
	store := &mockPipelineStore{getReleaseErr: errors.New("not found")}

	runner := NewRunner(store, nil, nil)

	err := runner.Process(context.Background(), "bad-id", nil)
	if err == nil {
		t.Fatal("expected error")
	}
}
