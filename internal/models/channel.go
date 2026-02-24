package models

import (
	"encoding/json"
	"time"
)

type NotificationChannel struct {
	ID        int             `json:"id"`
	Type      string          `json:"type"`
	Name      string          `json:"name"`
	Config    json.RawMessage `json:"config"`
	Enabled   bool            `json:"enabled"`
	CreatedAt time.Time       `json:"created_at"`
}
