package models

import "time"

type Source struct {
	ID                   int        `json:"id"`
	ProjectID            int        `json:"project_id"`
	SourceType           string     `json:"type"`
	Repository           string     `json:"repository"`
	PollIntervalSeconds  int        `json:"poll_interval_seconds"`
	Enabled              bool       `json:"enabled"`
	ExcludeVersionRegexp string     `json:"exclude_version_regexp,omitempty"`
	ExcludePrereleases   bool       `json:"exclude_prereleases"`
	LastPolledAt         *time.Time `json:"last_polled_at,omitempty"`
	LastError            *string    `json:"last_error,omitempty"`
	CreatedAt            time.Time  `json:"created_at"`
	UpdatedAt            time.Time  `json:"updated_at"`
}
