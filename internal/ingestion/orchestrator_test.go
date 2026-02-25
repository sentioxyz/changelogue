package ingestion

import (
	"context"
	"sync/atomic"
	"testing"
	"time"
)

type fakeSource struct {
	name     string
	sourceID string
	calls    atomic.Int32
	results  []IngestionResult
}

func (f *fakeSource) Name() string     { return f.name }
func (f *fakeSource) SourceID() string { return f.sourceID }

func (f *fakeSource) FetchNewReleases(_ context.Context) ([]IngestionResult, error) {
	f.calls.Add(1)
	return f.results, nil
}

func TestOrchestratorPollsOnInterval(t *testing.T) {
	store := &mockStore{}
	svc := NewService(store)

	src := &fakeSource{
		name:    "test",
		results: []IngestionResult{{Repository: "r", RawVersion: "v1", Timestamp: time.Now()}},
	}

	orch := NewOrchestratorWithSources(svc, []IIngestionSource{src}, 50*time.Millisecond)

	ctx, cancel := context.WithTimeout(context.Background(), 180*time.Millisecond)
	defer cancel()

	orch.Run(ctx)

	// With 50ms interval and 180ms timeout, expect: immediate poll + ~3 ticks = ~4 calls.
	// At minimum 2 calls proves the interval loop works.
	calls := int(src.calls.Load())
	if calls < 2 {
		t.Errorf("expected at least 2 poll calls, got %d", calls)
	}
}

func TestOrchestratorMultipleSources(t *testing.T) {
	store := &mockStore{}
	svc := NewService(store)

	src1 := &fakeSource{name: "a", results: []IngestionResult{{Repository: "r1", RawVersion: "v1", Timestamp: time.Now()}}}
	src2 := &fakeSource{name: "b", results: []IngestionResult{{Repository: "r2", RawVersion: "v2", Timestamp: time.Now()}}}

	orch := NewOrchestratorWithSources(svc, []IIngestionSource{src1, src2}, 50*time.Millisecond)

	ctx, cancel := context.WithTimeout(context.Background(), 80*time.Millisecond)
	defer cancel()

	orch.Run(ctx)

	if src1.calls.Load() < 1 {
		t.Error("source 1 was not polled")
	}
	if src2.calls.Load() < 1 {
		t.Error("source 2 was not polled")
	}
}
