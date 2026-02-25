package models

import (
	"encoding/json"
	"time"
)

type ContextSource struct {
	ID        string          `json:"id"`
	ProjectID string          `json:"project_id"`
	Type      string          `json:"type"`
	Name      string          `json:"name"`
	Config    json.RawMessage `json:"config"`
	CreatedAt time.Time       `json:"created_at"`
	UpdatedAt time.Time       `json:"updated_at"`
}
