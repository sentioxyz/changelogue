package auth

import (
	"testing"
	"time"
)

func TestStateStoreCreateAndConsume(t *testing.T) {
	store := NewStateStore(100, 10*time.Minute)

	state := store.Create()
	if state == "" {
		t.Fatal("state should not be empty")
	}

	if !store.Consume(state) {
		t.Fatal("first consume should succeed")
	}

	if store.Consume(state) {
		t.Fatal("second consume should fail (already consumed)")
	}
}

func TestStateStoreMaxSize(t *testing.T) {
	store := NewStateStore(2, 10*time.Minute)

	store.Create()
	store.Create()
	third := store.Create()

	if third != "" {
		t.Fatal("should return empty string when max size reached")
	}
}

func TestStateStoreExpiry(t *testing.T) {
	store := NewStateStore(100, 1*time.Millisecond)

	state := store.Create()
	time.Sleep(5 * time.Millisecond)

	if store.Consume(state) {
		t.Fatal("expired state should not be consumable")
	}
}
