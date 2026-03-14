package api

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/sentioxyz/changelogue/internal/models"
)

// mockOnboardStore implements OnboardStore for testing.
type mockOnboardStore struct {
	scan       *models.OnboardScan
	activeScan *models.OnboardScan
	createErr  error
	getErr     error
	applyRes   *OnboardApplyResult
	applyErr   error
}

func (m *mockOnboardStore) CreateOnboardScan(_ context.Context, repoURL string) (*models.OnboardScan, error) {
	if m.createErr != nil {
		return nil, m.createErr
	}
	return m.scan, nil
}

func (m *mockOnboardStore) GetOnboardScan(_ context.Context, id string) (*models.OnboardScan, error) {
	if m.getErr != nil {
		return nil, m.getErr
	}
	return m.scan, nil
}

func (m *mockOnboardStore) UpdateOnboardScanStatus(_ context.Context, _, _ string, _ json.RawMessage, _ string) error {
	return nil
}

func (m *mockOnboardStore) ActiveScanForRepo(_ context.Context, _ string) (*models.OnboardScan, error) {
	if m.activeScan != nil {
		return m.activeScan, nil
	}
	return nil, nil
}

func (m *mockOnboardStore) ApplyOnboardScan(_ context.Context, _ string, _ []OnboardSelection) (*OnboardApplyResult, error) {
	if m.applyErr != nil {
		return nil, m.applyErr
	}
	return m.applyRes, nil
}

func TestOnboardHandler_Scan(t *testing.T) {
	scan := &models.OnboardScan{ID: "test-id", RepoURL: "owner/repo", Status: "pending"}
	store := &mockOnboardStore{scan: scan}
	handler := NewOnboardHandler(store)

	body := bytes.NewBufferString(`{"repo_url": "owner/repo"}`)
	req := httptest.NewRequest("POST", "/api/v1/onboard/scan", body)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	handler.Scan(rec, req)

	if rec.Code != http.StatusCreated {
		t.Errorf("expected 201, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestOnboardHandler_Scan_EmptyURL(t *testing.T) {
	store := &mockOnboardStore{}
	handler := NewOnboardHandler(store)

	body := bytes.NewBufferString(`{"repo_url": ""}`)
	req := httptest.NewRequest("POST", "/api/v1/onboard/scan", body)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	handler.Scan(rec, req)

	if rec.Code != http.StatusUnprocessableEntity {
		t.Errorf("expected 422, got %d", rec.Code)
	}
}

func TestOnboardHandler_Scan_Conflict(t *testing.T) {
	existing := &models.OnboardScan{ID: "existing", Status: "processing"}
	store := &mockOnboardStore{activeScan: existing}
	handler := NewOnboardHandler(store)

	body := bytes.NewBufferString(`{"repo_url": "owner/repo"}`)
	req := httptest.NewRequest("POST", "/api/v1/onboard/scan", body)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	handler.Scan(rec, req)

	if rec.Code != http.StatusConflict {
		t.Errorf("expected 409, got %d", rec.Code)
	}
}

func TestOnboardHandler_Apply_Validation(t *testing.T) {
	store := &mockOnboardStore{}
	handler := NewOnboardHandler(store)

	// Both project_id and new_project_name set
	body := bytes.NewBufferString(`{"selections": [{"dep_name": "test", "upstream_repo": "github.com/test/test", "provider": "github", "project_id": "id", "new_project_name": "name"}]}`)
	req := httptest.NewRequest("POST", "/api/v1/onboard/scans/test-id/apply", body)
	req.SetPathValue("id", "test-id")
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	handler.Apply(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d: %s", rec.Code, rec.Body.String())
	}
}
