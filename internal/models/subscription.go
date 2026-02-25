package models

import "time"

type Subscription struct {
	ID            string    `json:"id"`
	ChannelID     string    `json:"channel_id"`
	Type          string    `json:"type"`
	SourceID      *string   `json:"source_id,omitempty"`
	ProjectID     *string   `json:"project_id,omitempty"`
	VersionFilter string    `json:"version_filter,omitempty"`
	CreatedAt     time.Time `json:"created_at"`
}
