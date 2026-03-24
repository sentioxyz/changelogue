package gate

import (
	"testing"

	"github.com/sentioxyz/changelogue/internal/models"
)

func TestNormalizeVersion(t *testing.T) {
	tests := []struct {
		name     string
		raw      string
		mapping  *models.VersionMapping
		expected string
	}{
		{
			name:     "no mapping strips v prefix",
			raw:      "v1.21.0",
			mapping:  nil,
			expected: "1.21.0",
		},
		{
			name:     "no mapping lowercases",
			raw:      "V1.21.0-RC1",
			mapping:  nil,
			expected: "1.21.0-rc1",
		},
		{
			name:     "no mapping no v prefix",
			raw:      "1.21.0",
			mapping:  nil,
			expected: "1.21.0",
		},
		{
			name:     "mapping with capture group",
			raw:      "v1.21.0",
			mapping:  &models.VersionMapping{Pattern: `^v?(.+)$`, Template: "$1"},
			expected: "1.21.0",
		},
		{
			name:     "mapping extracts semver from complex tag",
			raw:      "1.21.0-alpine",
			mapping:  &models.VersionMapping{Pattern: `^(\d+\.\d+\.\d+)`, Template: "$1"},
			expected: "1.21.0",
		},
		{
			name:     "mapping with invalid regex falls back to default",
			raw:      "v2.0.0",
			mapping:  &models.VersionMapping{Pattern: `[invalid`, Template: "$1"},
			expected: "2.0.0",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := NormalizeVersion(tt.raw, tt.mapping)
			if got != tt.expected {
				t.Errorf("NormalizeVersion(%q) = %q, want %q", tt.raw, got, tt.expected)
			}
		})
	}
}
