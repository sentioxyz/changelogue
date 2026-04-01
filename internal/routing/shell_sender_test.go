package routing

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/sentioxyz/changelogue/internal/models"
)

func TestShellSender_Send(t *testing.T) {
	tmpDir := t.TempDir()
	outFile := filepath.Join(tmpDir, "out.txt")

	chConfig, _ := json.Marshal(map[string]any{
		"timeout_seconds": 5,
	})

	subConfig, _ := json.Marshal(map[string]any{
		"command": "echo ${CHANGELOGUE_VERSION} > " + outFile,
	})

	ch := &models.NotificationChannel{
		ID:     "ch1",
		Name:   "test-shell",
		Type:   "shell",
		Config: chConfig,
	}

	msg := Notification{
		Version:    "v1.2.3",
		Repository: "owner/repo",
		Provider:   "github",
	}

	sender := &ShellSender{}
	err := sender.SendWithConfig(context.Background(), ch, msg, subConfig)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	time.Sleep(500 * time.Millisecond)

	data, err := os.ReadFile(outFile)
	if err != nil {
		t.Fatalf("output file not created: %v", err)
	}
	got := string(data)
	if got != "v1.2.3\n" {
		t.Errorf("got %q, want %q", got, "v1.2.3\n")
	}
}

func TestShellSender_SendMissingCommand(t *testing.T) {
	chConfig, _ := json.Marshal(map[string]any{})
	subConfig, _ := json.Marshal(map[string]any{})

	ch := &models.NotificationChannel{
		ID:     "ch1",
		Type:   "shell",
		Config: chConfig,
	}

	sender := &ShellSender{}
	err := sender.SendWithConfig(context.Background(), ch, Notification{}, subConfig)
	if err == nil {
		t.Fatal("expected error for missing command")
	}
}
