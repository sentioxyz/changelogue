package cli

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/sentioxyz/changelogue/internal/models"
)

func TestGetGate(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/projects/p1/release-gate" {
			t.Errorf("expected /api/v1/projects/p1/release-gate, got %s", r.URL.Path)
		}
		json.NewEncoder(w).Encode(map[string]any{
			"data": models.ReleaseGate{
				ID:           "g1",
				ProjectID:    "p1",
				Enabled:      true,
				TimeoutHours: 168,
			},
			"meta": map[string]any{"request_id": "r1"},
		})
	}))
	defer srv.Close()

	c := NewClient(srv.URL, "key")
	gate, err := GetGate(c, "p1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gate.ID != "g1" {
		t.Errorf("expected g1, got %s", gate.ID)
	}
	if !gate.Enabled {
		t.Error("expected gate to be enabled")
	}
	if gate.TimeoutHours != 168 {
		t.Errorf("expected 168, got %d", gate.TimeoutHours)
	}
}

func TestUpsertGate(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPut {
			t.Errorf("expected PUT, got %s", r.Method)
		}
		if r.URL.Path != "/api/v1/projects/p1/release-gate" {
			t.Errorf("expected /api/v1/projects/p1/release-gate, got %s", r.URL.Path)
		}
		var body map[string]any
		json.NewDecoder(r.Body).Decode(&body)
		if body["enabled"] != true {
			t.Errorf("expected enabled=true, got %v", body["enabled"])
		}
		json.NewEncoder(w).Encode(map[string]any{
			"data": models.ReleaseGate{
				ID:           "g1",
				ProjectID:    "p1",
				Enabled:      true,
				TimeoutHours: 168,
			},
			"meta": map[string]any{"request_id": "r2"},
		})
	}))
	defer srv.Close()

	c := NewClient(srv.URL, "key")
	gate, err := UpsertGate(c, "p1", map[string]any{"enabled": true})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gate.ID != "g1" {
		t.Errorf("expected g1, got %s", gate.ID)
	}
}

func TestDeleteGate(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete {
			t.Errorf("expected DELETE, got %s", r.Method)
		}
		if r.URL.Path != "/api/v1/projects/p1/release-gate" {
			t.Errorf("expected /api/v1/projects/p1/release-gate, got %s", r.URL.Path)
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()

	c := NewClient(srv.URL, "key")
	err := DeleteGate(c, "p1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestListReadiness(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/projects/p1/version-readiness" {
			t.Errorf("expected /api/v1/projects/p1/version-readiness, got %s", r.URL.Path)
		}
		if r.URL.Query().Get("page") != "1" {
			t.Errorf("expected page=1, got %s", r.URL.Query().Get("page"))
		}
		json.NewEncoder(w).Encode(map[string]any{
			"data": []models.VersionReadiness{{
				ID:        "vr1",
				ProjectID: "p1",
				Version:   "v1.0.0",
				Status:    "pending",
			}},
			"meta": map[string]any{"request_id": "r3", "page": 1, "per_page": 25, "total": 1},
		})
	}))
	defer srv.Close()

	c := NewClient(srv.URL, "key")
	entries, meta, err := ListReadiness(c, "p1", 1, 25)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}
	if entries[0].Version != "v1.0.0" {
		t.Errorf("expected v1.0.0, got %s", entries[0].Version)
	}
	if meta.Total != 1 {
		t.Errorf("expected total=1, got %d", meta.Total)
	}
}

func TestListGateEvents(t *testing.T) {
	now := time.Now()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/projects/p1/gate-events" {
			t.Errorf("expected /api/v1/projects/p1/gate-events, got %s", r.URL.Path)
		}
		json.NewEncoder(w).Encode(map[string]any{
			"data": []models.GateEvent{{
				ID:        "ev1",
				ProjectID: "p1",
				Version:   "v1.0.0",
				EventType: "source_met",
				CreatedAt: now,
			}},
			"meta": map[string]any{"request_id": "r4", "page": 1, "per_page": 25, "total": 1},
		})
	}))
	defer srv.Close()

	c := NewClient(srv.URL, "key")
	events, meta, err := ListGateEvents(c, "p1", 1, 25)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(events))
	}
	if events[0].EventType != "source_met" {
		t.Errorf("expected source_met, got %s", events[0].EventType)
	}
	if meta.Total != 1 {
		t.Errorf("expected total=1, got %d", meta.Total)
	}
}

func TestListGateEventsByVersion(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/projects/p1/version-readiness/v1.0.0/events" {
			t.Errorf("expected .../version-readiness/v1.0.0/events, got %s", r.URL.Path)
		}
		json.NewEncoder(w).Encode(map[string]any{
			"data": []models.GateEvent{{
				ID:        "ev2",
				ProjectID: "p1",
				Version:   "v1.0.0",
				EventType: "gate_opened",
			}},
			"meta": map[string]any{"request_id": "r5", "page": 1, "per_page": 25, "total": 1},
		})
	}))
	defer srv.Close()

	c := NewClient(srv.URL, "key")
	events, _, err := ListGateEventsByVersion(c, "p1", "v1.0.0", 1, 25)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(events))
	}
	if events[0].EventType != "gate_opened" {
		t.Errorf("expected gate_opened, got %s", events[0].EventType)
	}
}
