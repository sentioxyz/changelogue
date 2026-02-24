package models

import "time"

type Subscription struct {
	ID             int       `json:"id"`
	ProjectID      int       `json:"project_id"`
	ChannelType    string    `json:"channel_type"`
	ChannelID      int       `json:"channel_id"`
	VersionPattern string    `json:"version_pattern,omitempty"`
	Frequency      string    `json:"frequency"`
	Enabled        bool      `json:"enabled"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}
