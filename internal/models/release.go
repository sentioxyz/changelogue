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
}
