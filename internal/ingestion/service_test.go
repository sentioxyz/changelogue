package ingestion

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/sentioxyz/releaseguard/internal/models"
)

type mockStore struct {
	ingested []*models.ReleaseEvent
	err      error
}

func (m *mockStore) IngestRelease(_ context.Context, event *models.ReleaseEvent) error {
	if m.err != nil {
		return m.err
	}
	m.ingested = append(m.ingested, event)
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

	err := svc.ProcessResults(context.Background(), "dockerhub", results)
	if err != nil {
		t.Fatalf("ProcessResults: %v", err)
	}

	if len(store.ingested) != 1 {
		t.Fatalf("ingested %d events, want 1", len(store.ingested))
	}

	event := store.ingested[0]
	if event.Source != "dockerhub" {
		t.Errorf("Source = %q, want %q", event.Source, "dockerhub")
	}
	if event.Repository != "library/golang" {
		t.Errorf("Repository = %q, want %q", event.Repository, "library/golang")
	}
	if event.RawVersion != "1.21.0" {
		t.Errorf("RawVersion = %q, want %q", event.RawVersion, "1.21.0")
	}
	if event.ID == "" {
		t.Error("ID should not be empty")
	}
}

func TestServiceProcessResultsDuplicateSkipped(t *testing.T) {
	store := &mockStore{err: errors.New("unique_violation")}
	svc := NewService(store)

	results := []IngestionResult{
		{Repository: "lib/go", RawVersion: "1.0.0", Timestamp: time.Now()},
	}

	// Duplicates should not cause a top-level error — they're expected.
	err := svc.ProcessResults(context.Background(), "dockerhub", results)
	if err != nil {
		t.Fatalf("ProcessResults should not fail on duplicates: %v", err)
	}
}
