// internal/pipeline/regex_normalizer_test.go
package pipeline

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/sentioxyz/releaseguard/internal/models"
)

func TestRegexNormalizerName(t *testing.T) {
	n := NewRegexNormalizer()
	if got := n.Name(); got != "regex_normalizer" {
		t.Errorf("Name() = %q, want %q", got, "regex_normalizer")
	}
}

func TestRegexNormalizerParsesVersions(t *testing.T) {
	tests := []struct {
		name       string
		rawVersion string
		wantMajor  int
		wantMinor  int
		wantPatch  int
		wantPre    string
		wantPreRel bool
		wantParsed bool
	}{
		{"basic", "1.21.0", 1, 21, 0, "", false, true},
		{"v-prefix", "v1.21.0", 1, 21, 0, "", false, true},
		{"prerelease", "v2.0.0-rc.1", 2, 0, 0, "rc.1", true, true},
		{"beta", "1.0.0-beta.3", 1, 0, 0, "beta.3", true, true},
		{"two-part", "1.21", 1, 21, 0, "", false, true},
		{"major-only", "v3", 3, 0, 0, "", false, true},
		{"unparseable", "latest", 0, 0, 0, "", false, false},
		{"date-version", "20240115", 20240115, 0, 0, "", false, true},
	}

	n := NewRegexNormalizer()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			event := &models.ReleaseEvent{
				ID:         "test-id",
				RawVersion: tt.rawVersion,
				Timestamp:  time.Now(),
			}

			result, err := n.Execute(context.Background(), event, nil, nil)
			if err != nil {
				t.Fatalf("Execute: %v", err)
			}

			var res RegexNormalizerResult
			if err := json.Unmarshal(result, &res); err != nil {
				t.Fatalf("unmarshal: %v", err)
			}

			if res.Parsed != tt.wantParsed {
				t.Errorf("Parsed = %v, want %v", res.Parsed, tt.wantParsed)
			}
			if res.SemanticVersion.Major != tt.wantMajor {
				t.Errorf("Major = %d, want %d", res.SemanticVersion.Major, tt.wantMajor)
			}
			if res.SemanticVersion.Minor != tt.wantMinor {
				t.Errorf("Minor = %d, want %d", res.SemanticVersion.Minor, tt.wantMinor)
			}
			if res.SemanticVersion.Patch != tt.wantPatch {
				t.Errorf("Patch = %d, want %d", res.SemanticVersion.Patch, tt.wantPatch)
			}
			if res.SemanticVersion.PreRelease != tt.wantPre {
				t.Errorf("PreRelease = %q, want %q", res.SemanticVersion.PreRelease, tt.wantPre)
			}
			if res.IsPreRelease != tt.wantPreRel {
				t.Errorf("IsPreRelease = %v, want %v", res.IsPreRelease, tt.wantPreRel)
			}

			// Verify event was mutated for downstream nodes
			if event.SemanticVersion.Major != tt.wantMajor {
				t.Errorf("event.SemanticVersion.Major = %d, want %d", event.SemanticVersion.Major, tt.wantMajor)
			}
			if event.IsPreRelease != tt.wantPreRel {
				t.Errorf("event.IsPreRelease = %v, want %v", event.IsPreRelease, tt.wantPreRel)
			}
		})
	}
}
