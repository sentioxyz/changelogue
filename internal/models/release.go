package models

import (
	"fmt"
	"time"
)

// SemanticData holds parsed semantic version components.
// Populated by the pipeline's Regex Normalizer node, not by ingestion.
type SemanticData struct {
	Major      int    `json:"major"`
	Minor      int    `json:"minor"`
	Patch      int    `json:"patch"`
	PreRelease string `json:"pre_release,omitempty"`
}

func (s SemanticData) String() string {
	v := fmt.Sprintf("%d.%d.%d", s.Major, s.Minor, s.Patch)
	if s.PreRelease != "" {
		v += "-" + s.PreRelease
	}
	return v
}

// ReleaseEvent is the Intermediate Representation (IR) for a detected release.
// See DESIGN.md Section 2.1 for the canonical definition.
type ReleaseEvent struct {
	ID              string            `json:"id"`
	Source          string            `json:"source"`
	Repository      string            `json:"repository"`
	RawVersion      string            `json:"raw_version"`
	SemanticVersion SemanticData      `json:"semantic_version"`
	Changelog       string            `json:"changelog"`
	IsPreRelease    bool              `json:"is_pre_release"`
	Metadata        map[string]string `json:"metadata"`
	Timestamp       time.Time         `json:"timestamp"`
}
