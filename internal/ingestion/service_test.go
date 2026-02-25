package ingestion

import (
	"context"
	"errors"
	"testing"
	"time"
)

type mockStore struct {
	ingested []*IngestionResult
	err      error
}

func (m *mockStore) IngestRelease(_ context.Context, _ string, result *IngestionResult) error {
	if m.err != nil {
		return m.err
	}
	m.ingested = append(m.ingested, result)
	return nil
}

func TestServiceProcessResults(t *testing.T) {
	store := &mockStore{}
	svc := NewService(store)

	results := []IngestionResult{
		{
			Repository: "library/golang",
			RawVersion: "1.21.0",
			Changelog:  "Bug fixes",
			Timestamp:  time.Date(2024, 1, 15, 0, 0, 0, 0, time.UTC),
		},
	}

	err := svc.ProcessResults(context.Background(), "src-1", "dockerhub", results)
	if err != nil {
		t.Fatalf("ProcessResults: %v", err)
	}

	if len(store.ingested) != 1 {
		t.Fatalf("ingested %d results, want 1", len(store.ingested))
	}

	result := store.ingested[0]
	if result.Repository != "library/golang" {
		t.Errorf("Repository = %q, want %q", result.Repository, "library/golang")
	}
	if result.RawVersion != "1.21.0" {
		t.Errorf("RawVersion = %q, want %q", result.RawVersion, "1.21.0")
	}
}

func TestServiceProcessResultsDuplicateSkipped(t *testing.T) {
	store := &mockStore{err: errors.New("unique_violation")}
	svc := NewService(store)

	results := []IngestionResult{
		{Repository: "lib/go", RawVersion: "1.0.0", Timestamp: time.Now()},
	}

	// Duplicates should not cause a top-level error — they're expected.
	err := svc.ProcessResults(context.Background(), "src-1", "dockerhub", results)
	if err != nil {
		t.Fatalf("ProcessResults should not fail on duplicates: %v", err)
	}
}
