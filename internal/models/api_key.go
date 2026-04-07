package models

import "time"

type ApiKey struct {
	ID         string     `json:"id"`
	Name       string     `json:"name"`
	KeyPrefix  string     `json:"prefix"`
	Key        string     `json:"key,omitempty"` // only returned on create
	CreatedAt  time.Time  `json:"created_at"`
	LastUsedAt *time.Time `json:"last_used_at"`
}
