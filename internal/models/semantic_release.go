package models

import (
	"encoding/json"
	"time"
)

type SemanticReport struct {
	// Primary fields
	Subject          string   `json:"subject"`
	Urgency          string   `json:"urgency"`
	UrgencyReason    string   `json:"urgency_reason,omitempty"`
	StatusChecks     []string `json:"status_checks"`
	ChangelogSummary string   `json:"changelog_summary"`
	DownloadCommands []string `json:"download_commands,omitempty"`
	DownloadLinks    []string `json:"download_links,omitempty"`

	// Existing fields
	Summary        string `json:"summary,omitempty"`
	Availability   string `json:"availability"`
	Adoption       string `json:"adoption"`
	Recommendation string `json:"recommendation"`

	// Backward compat — old reports may still have these in JSONB
	RiskLevel  string `json:"risk_level,omitempty"`
	RiskReason string `json:"risk_reason,omitempty"`
}

type SemanticRelease struct {
	ID          string          `json:"id"`
	ProjectID   string          `json:"project_id"`
	ProjectName string          `json:"project_name,omitempty"`
	Version     string          `json:"version"`
	Report      json.RawMessage `json:"report,omitempty"`
	Status      string          `json:"status"`
	Error       string          `json:"error,omitempty"`
	CreatedAt   time.Time       `json:"created_at"`
	CompletedAt *time.Time      `json:"completed_at,omitempty"`
}
