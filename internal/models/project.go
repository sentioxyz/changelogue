package models

import (
	"encoding/json"
	"time"
)

type AgentRules struct {
	OnMajorRelease    bool   `json:"on_major_release,omitempty"`
	OnMinorRelease    bool   `json:"on_minor_release,omitempty"`
	OnSecurityPatch   bool   `json:"on_security_patch,omitempty"`
	VersionPattern    string `json:"version_pattern,omitempty"`
	WaitForAllSources bool   `json:"wait_for_all_sources,omitempty"`
}

type Project struct {
	ID          string          `json:"id"`
	Name        string          `json:"name"`
	Description string          `json:"description,omitempty"`
	AgentPrompt string          `json:"agent_prompt,omitempty"`
	AgentRules  json.RawMessage `json:"agent_rules,omitempty"`
	CreatedAt   time.Time       `json:"created_at"`
	UpdatedAt   time.Time       `json:"updated_at"`
}
