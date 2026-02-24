// internal/pipeline/urgency_scorer.go
package pipeline

import (
	"context"
	"encoding/json"
	"strings"

	"github.com/sentioxyz/releaseguard/internal/models"
)

var securityKeywords = []string{"security", "cve", "vulnerability", "critical", "exploit", "rce", "injection"}

// UrgencyScorerResult is the output of the Urgency Scorer node.
type UrgencyScorerResult struct {
	Score   string   `json:"score"`
	Factors []string `json:"factors"`
}

// UrgencyScorer computes a composite urgency level from prior node results and
// the release changelog. Configurable node — only runs when enabled in pipeline_config.
type UrgencyScorer struct{}

func NewUrgencyScorer() *UrgencyScorer { return &UrgencyScorer{} }

func (n *UrgencyScorer) Name() string { return "urgency_scorer" }

func (n *UrgencyScorer) Execute(_ context.Context, event *models.ReleaseEvent, _ json.RawMessage, prior map[string]json.RawMessage) (json.RawMessage, error) {
	var normResult RegexNormalizerResult
	if raw, ok := prior["regex_normalizer"]; ok {
		json.Unmarshal(raw, &normResult)
	}

	score := "MEDIUM"
	var factors []string

	// Security keywords override everything
	changelogLower := strings.ToLower(event.Changelog)
	for _, kw := range securityKeywords {
		if strings.Contains(changelogLower, kw) {
			score = "CRITICAL"
			factors = append(factors, "security_keyword_"+kw)
			break
		}
	}

	if score != "CRITICAL" {
		switch {
		case normResult.IsPreRelease:
			score = "LOW"
			factors = append(factors, "pre_release")
		case normResult.Parsed && normResult.SemanticVersion.Minor == 0 && normResult.SemanticVersion.Patch == 0:
			score = "HIGH"
			factors = append(factors, "major_version")
		case normResult.Parsed && normResult.SemanticVersion.Patch > 0:
			score = "LOW"
			factors = append(factors, "patch_version")
		default:
			factors = append(factors, "minor_version")
		}
	}

	return json.Marshal(UrgencyScorerResult{Score: score, Factors: factors})
}

var _ PipelineNode = (*UrgencyScorer)(nil)
