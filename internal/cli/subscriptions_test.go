package cli

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/sentioxyz/changelogue/internal/models"
)

func TestSubscriptionsList(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/subscriptions" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		json.NewEncoder(w).Encode(map[string]any{
			"data": []models.Subscription{{ID: "sub1", ChannelID: "ch1", Type: "source_release"}},
			"meta": map[string]any{"request_id": "r1", "page": 1, "per_page": 25, "total": 1},
		})
	}))
	defer srv.Close()

	c := NewClient(srv.URL, "key")
	subs, _, err := ListSubscriptions(c, 1, 25)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(subs) != 1 || subs[0].Type != "source_release" {
		t.Errorf("unexpected subscriptions: %+v", subs)
	}
}

func TestSubscriptionsCreate(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/api/v1/subscriptions" {
			t.Errorf("unexpected %s %s", r.Method, r.URL.Path)
		}
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(map[string]any{
			"data": models.Subscription{ID: "sub2", ChannelID: "ch1", Type: "semantic_release"},
			"meta": map[string]any{"request_id": "r2"},
		})
	}))
	defer srv.Close()

	c := NewClient(srv.URL, "key")
	sub, err := CreateSubscription(c, map[string]any{"channel_id": "ch1", "type": "semantic_release", "project_id": "p1"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if sub.ID != "sub2" {
		t.Errorf("expected sub2, got %s", sub.ID)
	}
}

func TestSubscriptionsBatchDelete(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete || r.URL.Path != "/api/v1/subscriptions/batch" {
			t.Errorf("unexpected %s %s", r.Method, r.URL.Path)
		}
		var body map[string][]string
		json.NewDecoder(r.Body).Decode(&body)
		if len(body["ids"]) != 2 {
			t.Errorf("expected 2 ids, got %d", len(body["ids"]))
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()

	c := NewClient(srv.URL, "key")
	err := BatchDeleteSubscriptions(c, []string{"sub1", "sub2"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}
