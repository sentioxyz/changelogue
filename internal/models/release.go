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
}
