package models

import (
	"encoding/json"
	"time"
)

// VersionMapping defines a per-source regex/template for normalizing versions.
type VersionMapping struct {
	Pattern  string `json:"pattern"`
	Template string `json:"template"`
}

// ReleaseGate is a per-project gate configuration that controls when the
// LLM agent runs for multi-source projects.
type ReleaseGate struct {
	ID              string                    `json:"id"`
	ProjectID       string                    `json:"project_id"`
	RequiredSources []string                  `json:"required_sources,omitempty"` // source UUIDs; empty = all
	TimeoutHours    int                       `json:"timeout_hours"`
	VersionMapping  map[string]VersionMapping `json:"version_mapping,omitempty"` // keyed by source ID
	NLRule          string                    `json:"nl_rule,omitempty"`
	Enabled         bool                      `json:"enabled"`
	CreatedAt       time.Time                 `json:"created_at"`
	UpdatedAt       time.Time                 `json:"updated_at"`
}

// VersionReadiness tracks gate state for a specific version.
type VersionReadiness struct {
	ID             string     `json:"id"`
	ProjectID      string     `json:"project_id"`
	Version        string     `json:"version"` // normalized
	Status         string     `json:"status"`  // pending, ready, timed_out
	SourcesMet     []string   `json:"sources_met"`
	SourcesMissing []string   `json:"sources_missing"`
	NLRulePassed   *bool      `json:"nl_rule_passed,omitempty"`
	TimeoutAt      time.Time  `json:"timeout_at"`
	OpenedAt       *time.Time `json:"opened_at,omitempty"`
	AgentTriggered bool       `json:"agent_triggered"`
	CreatedAt      time.Time  `json:"created_at"`
	UpdatedAt      time.Time  `json:"updated_at"`
}

// GateEvent records a state transition in the gate lifecycle.
type GateEvent struct {
	ID                 string          `json:"id"`
	VersionReadinessID string          `json:"version_readiness_id"`
	ProjectID          string          `json:"project_id"`
	Version            string          `json:"version"`
	EventType          string          `json:"event_type"`
	SourceID           *string         `json:"source_id,omitempty"`
	Details            json.RawMessage `json:"details,omitempty"`
	CreatedAt          time.Time       `json:"created_at"`
}
