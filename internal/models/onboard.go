package models

import (
	"encoding/json"
	"time"
)

// OnboardScan represents a GitHub repo dependency scan.
type OnboardScan struct {
	ID          string          `json:"id"`
	RepoURL     string          `json:"repo_url"`
	Status      string          `json:"status"` // pending, processing, completed, failed
	Results     json.RawMessage `json:"results,omitempty"`
	Error       string          `json:"error,omitempty"`
	CreatedAt   time.Time       `json:"created_at"`
	StartedAt   *time.Time      `json:"started_at,omitempty"`
	CompletedAt *time.Time      `json:"completed_at,omitempty"`
}

// ScannedDependency is one entry in OnboardScan.Results.
type ScannedDependency struct {
	Name         string `json:"name"`
	Version      string `json:"version"`
	Ecosystem    string `json:"ecosystem"`
	UpstreamRepo string `json:"upstream_repo"`
	Provider     string `json:"provider"`
}
