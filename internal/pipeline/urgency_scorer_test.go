// internal/pipeline/urgency_scorer_test.go
package pipeline

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/sentioxyz/releaseguard/internal/models"
)

func mustMarshal(v any) json.RawMessage {
	data, _ := json.Marshal(v)
	return data
}

func TestUrgencyScorerName(t *testing.T) {
	n := NewUrgencyScorer()
	if got := n.Name(); got != "urgency_scorer" {
		t.Errorf("Name() = %q, want %q", got, "urgency_scorer")
	}
}

func TestUrgencyScorerPreRelease(t *testing.T) {
	n := NewUrgencyScorer()
	event := &models.ReleaseEvent{
		RawVersion:      "v1.0.0-rc.1",
		IsPreRelease:    true,
		SemanticVersion: models.SemanticData{Major: 1, PreRelease: "rc.1"},
		Timestamp:       time.Now(),
	}
	prior := map[string]json.RawMessage{
		"regex_normalizer": mustMarshal(RegexNormalizerResult{
			SemanticVersion: event.SemanticVersion,
			IsPreRelease:    true,
			Parsed:          true,
		}),
	}

	result, err := n.Execute(context.Background(), event, json.RawMessage(`{}`), prior)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}

	var res UrgencyScorerResult
	json.Unmarshal(result, &res)
	if res.Score != "LOW" {
		t.Errorf("Score = %q, want %q", res.Score, "LOW")
	}
}

func TestUrgencyScorerMajorVersion(t *testing.T) {
	n := NewUrgencyScorer()
	event := &models.ReleaseEvent{
		RawVersion:      "v2.0.0",
		SemanticVersion: models.SemanticData{Major: 2},
		Timestamp:       time.Now(),
	}
	prior := map[string]json.RawMessage{
		"regex_normalizer": mustMarshal(RegexNormalizerResult{
			SemanticVersion: event.SemanticVersion,
			Parsed:          true,
		}),
	}

	result, err := n.Execute(context.Background(), event, json.RawMessage(`{}`), prior)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}

	var res UrgencyScorerResult
	json.Unmarshal(result, &res)
	if res.Score != "HIGH" {
		t.Errorf("Score = %q, want %q", res.Score, "HIGH")
	}
}

func TestUrgencyScorerSecurityKeywords(t *testing.T) {
	n := NewUrgencyScorer()
	event := &models.ReleaseEvent{
		RawVersion:      "v1.0.1",
		Changelog:       "Fixes critical CVE-2024-1234 vulnerability",
		SemanticVersion: models.SemanticData{Major: 1, Patch: 1},
		Timestamp:       time.Now(),
	}
	prior := map[string]json.RawMessage{
		"regex_normalizer": mustMarshal(RegexNormalizerResult{
			SemanticVersion: event.SemanticVersion,
			Parsed:          true,
		}),
	}

	result, err := n.Execute(context.Background(), event, json.RawMessage(`{}`), prior)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}

	var res UrgencyScorerResult
	json.Unmarshal(result, &res)
	if res.Score != "CRITICAL" {
		t.Errorf("Score = %q, want %q", res.Score, "CRITICAL")
	}
}

func TestUrgencyScorerPatchVersion(t *testing.T) {
	n := NewUrgencyScorer()
	event := &models.ReleaseEvent{
		RawVersion:      "v1.21.3",
		SemanticVersion: models.SemanticData{Major: 1, Minor: 21, Patch: 3},
		Timestamp:       time.Now(),
	}
	prior := map[string]json.RawMessage{
		"regex_normalizer": mustMarshal(RegexNormalizerResult{
			SemanticVersion: event.SemanticVersion,
			Parsed:          true,
		}),
	}

	result, err := n.Execute(context.Background(), event, json.RawMessage(`{}`), prior)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}

	var res UrgencyScorerResult
	json.Unmarshal(result, &res)
	if res.Score != "LOW" {
		t.Errorf("Score = %q, want %q", res.Score, "LOW")
	}
}

func TestUrgencyScorerMinorVersion(t *testing.T) {
	n := NewUrgencyScorer()
	event := &models.ReleaseEvent{
		RawVersion:      "v1.22.0",
		SemanticVersion: models.SemanticData{Major: 1, Minor: 22},
		Timestamp:       time.Now(),
	}
	prior := map[string]json.RawMessage{
		"regex_normalizer": mustMarshal(RegexNormalizerResult{
			SemanticVersion: event.SemanticVersion,
			Parsed:          true,
		}),
	}

	result, err := n.Execute(context.Background(), event, json.RawMessage(`{}`), prior)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}

	var res UrgencyScorerResult
	json.Unmarshal(result, &res)
	if res.Score != "MEDIUM" {
		t.Errorf("Score = %q, want %q", res.Score, "MEDIUM")
	}
}
