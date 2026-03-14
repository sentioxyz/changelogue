package models

import (
	"encoding/json"
	"time"
)

type Release struct {
	ID         string          `json:"id"`
	SourceID   string          `json:"source_id"`
	Version    string          `json:"version"`
	RawData    json.RawMessage `json:"raw_data,omitempty"`
	ReleasedAt *time.Time      `json:"released_at,omitempty"`
	CreatedAt  time.Time       `json:"created_at"`

	// Enriched fields from JOINed sources/projects tables.
	ProjectID   string `json:"project_id,omitempty"`
	ProjectName string `json:"project_name,omitempty"`
	Provider    string `json:"provider,omitempty"`
	Repository  string `json:"repository,omitempty"`
	Excluded    bool   `json:"excluded"` // true when filtered out by source version filters

	// Enriched fields from LEFT JOINed semantic_releases table.
	SemanticReleaseID      string `json:"semantic_release_id,omitempty"`
	SemanticReleaseStatus  string `json:"semantic_release_status,omitempty"`
	SemanticReleaseUrgency string `json:"semantic_release_urgency,omitempty"`
}
