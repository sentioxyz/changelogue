// internal/pipeline/regex_normalizer.go
package pipeline

import (
	"context"
	"encoding/json"
	"regexp"
	"strconv"

	"github.com/sentioxyz/releaseguard/internal/models"
)

var semverRegex = regexp.MustCompile(`^v?(\d+)(?:\.(\d+))?(?:\.(\d+))?(?:[+-]([\w.]+))?$`)

// RegexNormalizerResult is the output of the Regex Normalizer node.
type RegexNormalizerResult struct {
	SemanticVersion models.SemanticData `json:"semantic_version"`
	IsPreRelease    bool                `json:"is_pre_release"`
	Parsed          bool                `json:"parsed"`
}

// RegexNormalizer parses RawVersion into SemanticData and detects pre-releases.
// Always-on node — runs for every release regardless of pipeline_config.
type RegexNormalizer struct{}

func NewRegexNormalizer() *RegexNormalizer { return &RegexNormalizer{} }

func (n *RegexNormalizer) Name() string { return "regex_normalizer" }

func (n *RegexNormalizer) Execute(_ context.Context, event *models.ReleaseEvent, _ json.RawMessage, _ map[string]json.RawMessage) (json.RawMessage, error) {
	result := RegexNormalizerResult{}

	matches := semverRegex.FindStringSubmatch(event.RawVersion)
	if matches != nil {
		result.Parsed = true
		result.SemanticVersion.Major, _ = strconv.Atoi(matches[1])
		if matches[2] != "" {
			result.SemanticVersion.Minor, _ = strconv.Atoi(matches[2])
		}
		if matches[3] != "" {
			result.SemanticVersion.Patch, _ = strconv.Atoi(matches[3])
		}
		if matches[4] != "" {
			result.SemanticVersion.PreRelease = matches[4]
			result.IsPreRelease = true
		}
	}

	// Mutate event for downstream nodes
	event.SemanticVersion = result.SemanticVersion
	event.IsPreRelease = result.IsPreRelease

	return json.Marshal(result)
}

var _ PipelineNode = (*RegexNormalizer)(nil)
