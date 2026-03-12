package models

import "time"

// Todo represents a release TODO item for acknowledge/resolve tracking.
type Todo struct {
	ID                string     `json:"id"`
	ReleaseID         *string    `json:"release_id,omitempty"`
	SemanticReleaseID *string    `json:"semantic_release_id,omitempty"`
	Status            string     `json:"status"` // pending, acknowledged, resolved
	CreatedAt         time.Time  `json:"created_at"`
	AcknowledgedAt    *time.Time `json:"acknowledged_at,omitempty"`
	ResolvedAt        *time.Time `json:"resolved_at,omitempty"`
	// Enriched fields from JOINs (populated by list queries)
	ProjectID   string `json:"project_id,omitempty"`
	ProjectName string `json:"project_name,omitempty"`
	Version     string `json:"version,omitempty"`
	Provider    string `json:"provider,omitempty"`
	Repository  string `json:"repository,omitempty"`
	SourceURL   string `json:"source_url,omitempty"`
	ReleaseURL  string `json:"release_url,omitempty"`
	Urgency     string `json:"urgency,omitempty"`
	TodoType    string `json:"todo_type,omitempty"` // "release" or "semantic"
}
