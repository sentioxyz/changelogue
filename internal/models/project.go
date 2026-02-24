package models

import (
	"encoding/json"
	"time"
)

type Project struct {
	ID             int             `json:"id"`
	Name           string          `json:"name"`
	Description    string          `json:"description,omitempty"`
	URL            string          `json:"url,omitempty"`
	PipelineConfig json.RawMessage `json:"pipeline_config"`
	CreatedAt      time.Time       `json:"created_at"`
	UpdatedAt      time.Time       `json:"updated_at"`
}
