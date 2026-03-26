package api

import (
	"net/http"
	"time"
)

// ReleaseFilter holds optional filter parameters for release listing endpoints.
type ReleaseFilter struct {
	Provider string
	Urgency  string
	DateFrom *time.Time
	DateTo   *time.Time
}

// TodoFilter holds optional filter parameters for the todo listing endpoint.
type TodoFilter struct {
	ProjectID string
	Provider  string
	Urgency   string
	DateFrom  *time.Time
	DateTo    *time.Time
}

// ParseReleaseFilters extracts release filter params from the request query string.
func ParseReleaseFilters(r *http.Request) ReleaseFilter {
	q := r.URL.Query()
	f := ReleaseFilter{
		Provider: q.Get("provider"),
		Urgency:  q.Get("urgency"),
	}
	if v := q.Get("date_from"); v != "" {
		if t, err := time.Parse("2006-01-02", v); err == nil {
			f.DateFrom = &t
		}
	}
	if v := q.Get("date_to"); v != "" {
		if t, err := time.Parse("2006-01-02", v); err == nil {
			end := t.Add(24*time.Hour - time.Nanosecond)
			f.DateTo = &end
		}
	}
	return f
}

// ParseTodoFilters extracts todo filter params from the request query string.
func ParseTodoFilters(r *http.Request) TodoFilter {
	q := r.URL.Query()
	f := TodoFilter{
		ProjectID: q.Get("project"),
		Provider:  q.Get("provider"),
		Urgency:   q.Get("urgency"),
	}
	if v := q.Get("date_from"); v != "" {
		if t, err := time.Parse("2006-01-02", v); err == nil {
			f.DateFrom = &t
		}
	}
	if v := q.Get("date_to"); v != "" {
		if t, err := time.Parse("2006-01-02", v); err == nil {
			end := t.Add(24*time.Hour - time.Nanosecond)
			f.DateTo = &end
		}
	}
	return f
}
