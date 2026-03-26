package models

import "time"

// ReleaseFilter holds optional filter parameters for release listing.
type ReleaseFilter struct {
	Provider string
	Urgency  string
	DateFrom *time.Time
	DateTo   *time.Time
}

// TodoFilter holds optional filter parameters for todo listing.
type TodoFilter struct {
	ProjectID string
	Provider  string
	Urgency   string
	DateFrom  *time.Time
	DateTo    *time.Time
}
