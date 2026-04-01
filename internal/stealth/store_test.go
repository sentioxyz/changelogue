package stealth

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func testStore(t *testing.T) *Store {
	t.Helper()
	dbPath := filepath.Join(t.TempDir(), "test.db")
	store, err := NewStore(dbPath)
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}
	t.Cleanup(func() { store.Close() })
	return store
}

func TestNewStore(t *testing.T) {
	store := testStore(t)
	if err := store.PingDB(context.Background()); err != nil {
		t.Fatalf("PingDB: %v", err)
	}
}

func TestNewStore_CreatesDirectory(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "sub", "dir")
	dbPath := filepath.Join(dir, "test.db")
	store, err := NewStore(dbPath)
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}
	defer store.Close()
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		t.Fatal("expected directory to be created")
	}
}
