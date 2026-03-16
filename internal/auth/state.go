package auth

import (
	"crypto/rand"
	"encoding/hex"
	"sync"
	"time"
)

type stateEntry struct {
	expiresAt time.Time
}

// StateStore holds OAuth state parameters in memory with TTL and size limit.
type StateStore struct {
	mu      sync.Mutex
	entries map[string]stateEntry
	maxSize int
	ttl     time.Duration
}

// NewStateStore creates a state store with the given max size and TTL per entry.
func NewStateStore(maxSize int, ttl time.Duration) *StateStore {
	return &StateStore{
		entries: make(map[string]stateEntry),
		maxSize: maxSize,
		ttl:     ttl,
	}
}

// Create generates a random state string and stores it. Returns "" if the store is full.
func (s *StateStore) Create() string {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Sweep expired before checking size
	now := time.Now()
	for k, v := range s.entries {
		if now.After(v.expiresAt) {
			delete(s.entries, k)
		}
	}

	if len(s.entries) >= s.maxSize {
		return ""
	}

	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return ""
	}
	state := hex.EncodeToString(b)
	s.entries[state] = stateEntry{expiresAt: now.Add(s.ttl)}
	return state
}

// Consume validates and removes a state parameter. Returns true if valid.
func (s *StateStore) Consume(state string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	entry, ok := s.entries[state]
	if !ok {
		return false
	}
	delete(s.entries, state)
	return time.Now().Before(entry.expiresAt)
}
