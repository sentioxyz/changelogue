package models

import "time"

type AgentRun struct {
	ID                string     `json:"id"`
	ProjectID         string     `json:"project_id"`
	SemanticReleaseID *string    `json:"semantic_release_id,omitempty"`
	Trigger           string     `json:"trigger"`
	Status            string     `json:"status"`
	PromptUsed        string     `json:"prompt_used,omitempty"`
	Error             string     `json:"error,omitempty"`
	StartedAt         *time.Time `json:"started_at,omitempty"`
	CompletedAt       *time.Time `json:"completed_at,omitempty"`
	CreatedAt         time.Time  `json:"created_at"`
}
