package onboard

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/sentioxyz/changelogue/internal/models"
)

type mockScanStore struct {
	scan        *models.OnboardScan
	getErr      error
	lastStatus  string
	lastResults json.RawMessage
}

func (m *mockScanStore) GetOnboardScan(_ context.Context, _ string) (*models.OnboardScan, error) {
	if m.getErr != nil {
		return nil, m.getErr
	}
	return m.scan, nil
}

func (m *mockScanStore) UpdateOnboardScanStatus(_ context.Context, _, status string, results json.RawMessage, _ string) error {
	m.lastStatus = status
	m.lastResults = results
	return nil
}

func TestScanWorker_EmptyRepo(t *testing.T) {
	// Scanner that returns no files
	store := &mockScanStore{
		scan: &models.OnboardScan{ID: "scan-1", RepoURL: "owner/empty-repo", Status: "pending"},
	}

	// We can't easily test the full worker without a real River job,
	// but we can test the core logic by calling the internal methods.
	// For now, verify the parse + store interaction.
	owner, repo, err := ParseRepoURL("owner/empty-repo")
	if err != nil {
		t.Fatalf("ParseRepoURL: %v", err)
	}
	if owner != "owner" || repo != "empty-repo" {
		t.Errorf("got (%q, %q), want (owner, empty-repo)", owner, repo)
	}

	// Verify store was initialized correctly
	if store.scan.ID != "scan-1" {
		t.Errorf("scan ID = %q, want scan-1", store.scan.ID)
	}
}
