package models

import (
	"encoding/json"
	"time"
)

type SourceRepo struct {
	Provider   string `json:"provider"`
	Repository string `json:"repository"`
}

type Source struct {
	ID                  string          `json:"id"`
	ProjectID           string          `json:"project_id"`
	Provider            string          `json:"provider"`
	Repository          string          `json:"repository"`
	PollIntervalSeconds int             `json:"poll_interval_seconds"`
	Enabled             bool            `json:"enabled"`
	Config               json.RawMessage `json:"config,omitempty"`
	VersionFilterInclude *string         `json:"version_filter_include,omitempty"`
	VersionFilterExclude *string         `json:"version_filter_exclude,omitempty"`
	ExcludePrereleases   bool            `json:"exclude_prereleases"`
	LastPolledAt         *time.Time      `json:"last_polled_at,omitempty"`
	LastError           *string         `json:"last_error,omitempty"`
	CreatedAt           time.Time       `json:"created_at"`
	UpdatedAt           time.Time       `json:"updated_at"`
}
