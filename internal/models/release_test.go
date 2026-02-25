package models

import (
	"encoding/json"
	"testing"
	"time"
)

func TestReleaseJSON(t *testing.T) {
	releasedAt := time.Date(2024, 1, 15, 10, 0, 0, 0, time.UTC)
	release := Release{
		ID:         "550e8400-e29b-41d4-a716-446655440000",
		SourceID:   "src-dockerhub-golang",
		Version:    "1.21.0",
		RawData:    json.RawMessage(`{"digest":"sha256:abc123"}`),
		ReleasedAt: &releasedAt,
		CreatedAt:  time.Date(2024, 1, 16, 12, 0, 0, 0, time.UTC),
	}

	data, err := json.Marshal(release)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var got Release
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if got.ID != release.ID {
		t.Errorf("ID = %q, want %q", got.ID, release.ID)
	}
	if got.SourceID != release.SourceID {
		t.Errorf("SourceID = %q, want %q", got.SourceID, release.SourceID)
	}
	if got.Version != release.Version {
		t.Errorf("Version = %q, want %q", got.Version, release.Version)
	}
	if string(got.RawData) != string(release.RawData) {
		t.Errorf("RawData = %s, want %s", got.RawData, release.RawData)
	}
	if got.ReleasedAt == nil {
		t.Fatal("ReleasedAt = nil, want non-nil")
	}
	if !got.ReleasedAt.Equal(*release.ReleasedAt) {
		t.Errorf("ReleasedAt = %v, want %v", got.ReleasedAt, release.ReleasedAt)
	}
	if !got.CreatedAt.Equal(release.CreatedAt) {
		t.Errorf("CreatedAt = %v, want %v", got.CreatedAt, release.CreatedAt)
	}
}

func TestReleaseJSON_NilReleasedAt(t *testing.T) {
	release := Release{
		ID:        "rel-001",
		SourceID:  "src-github-react",
		Version:   "18.2.0",
		CreatedAt: time.Date(2024, 6, 1, 8, 30, 0, 0, time.UTC),
	}

	data, err := json.Marshal(release)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	// When ReleasedAt is nil, omitempty should exclude it from JSON output.
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatalf("unmarshal to map: %v", err)
	}
	if _, exists := raw["released_at"]; exists {
		t.Error("expected released_at to be omitted when nil, but it was present")
	}

	// RawData is nil too, should also be omitted.
	if _, exists := raw["raw_data"]; exists {
		t.Error("expected raw_data to be omitted when nil, but it was present")
	}

	// Round-trip: unmarshal back into a Release and confirm ReleasedAt is nil.
	var got Release
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if got.ReleasedAt != nil {
		t.Errorf("ReleasedAt = %v, want nil", got.ReleasedAt)
	}
	if got.RawData != nil {
		t.Errorf("RawData = %s, want nil", got.RawData)
	}
}

func TestReleaseJSON_FromRawString(t *testing.T) {
	input := `{
		"id": "rel-002",
		"source_id": "src-dockerhub-nginx",
		"version": "1.25.3",
		"raw_data": {"tag": "stable"},
		"released_at": "2024-03-10T14:00:00Z",
		"created_at": "2024-03-11T09:00:00Z"
	}`

	var got Release
	if err := json.Unmarshal([]byte(input), &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if got.ID != "rel-002" {
		t.Errorf("ID = %q, want %q", got.ID, "rel-002")
	}
	if got.SourceID != "src-dockerhub-nginx" {
		t.Errorf("SourceID = %q, want %q", got.SourceID, "src-dockerhub-nginx")
	}
	if got.Version != "1.25.3" {
		t.Errorf("Version = %q, want %q", got.Version, "1.25.3")
	}
	if string(got.RawData) != `{"tag": "stable"}` {
		t.Errorf("RawData = %s, want %s", got.RawData, `{"tag": "stable"}`)
	}
	if got.ReleasedAt == nil {
		t.Fatal("ReleasedAt = nil, want non-nil")
	}

	wantReleasedAt := time.Date(2024, 3, 10, 14, 0, 0, 0, time.UTC)
	if !got.ReleasedAt.Equal(wantReleasedAt) {
		t.Errorf("ReleasedAt = %v, want %v", got.ReleasedAt, wantReleasedAt)
	}

	wantCreatedAt := time.Date(2024, 3, 11, 9, 0, 0, 0, time.UTC)
	if !got.CreatedAt.Equal(wantCreatedAt) {
		t.Errorf("CreatedAt = %v, want %v", got.CreatedAt, wantCreatedAt)
	}
}
