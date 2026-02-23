package models

import (
	"encoding/json"
	"testing"
	"time"
)

func TestReleaseEventJSON(t *testing.T) {
	event := ReleaseEvent{
		ID:         "550e8400-e29b-41d4-a716-446655440000",
		Source:     "dockerhub",
		Repository: "library/golang",
		RawVersion: "1.21.0",
		Changelog:  "Bug fixes and improvements",
		Metadata:   map[string]string{"digest": "sha256:abc123"},
		Timestamp:  time.Date(2024, 1, 15, 10, 0, 0, 0, time.UTC),
	}

	data, err := json.Marshal(event)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var got ReleaseEvent
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if got.ID != event.ID {
		t.Errorf("ID = %q, want %q", got.ID, event.ID)
	}
	if got.Source != event.Source {
		t.Errorf("Source = %q, want %q", got.Source, event.Source)
	}
	if got.Repository != event.Repository {
		t.Errorf("Repository = %q, want %q", got.Repository, event.Repository)
	}
	if got.RawVersion != event.RawVersion {
		t.Errorf("RawVersion = %q, want %q", got.RawVersion, event.RawVersion)
	}
}

func TestSemanticDataString(t *testing.T) {
	tests := []struct {
		name string
		data SemanticData
		want string
	}{
		{"stable", SemanticData{Major: 1, Minor: 21, Patch: 0}, "1.21.0"},
		{"prerelease", SemanticData{Major: 2, Minor: 0, Patch: 0, PreRelease: "rc.1"}, "2.0.0-rc.1"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.data.String(); got != tt.want {
				t.Errorf("String() = %q, want %q", got, tt.want)
			}
		})
	}
}
