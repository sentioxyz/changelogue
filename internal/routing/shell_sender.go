package routing

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/sentioxyz/changelogue/internal/models"
)

type shellChannelConfig struct {
	TimeoutSeconds int    `json:"timeout_seconds"`
	WorkingDir     string `json:"working_dir"`
}

type shellSubscriptionConfig struct {
	Command        string `json:"command"`
	TimeoutSeconds int    `json:"timeout_seconds"`
	WorkingDir     string `json:"working_dir"`
}

// ShellSender executes a shell command when a notification fires.
type ShellSender struct{}

// Send implements the Sender interface. For shell channels, the subscription
// config is not available via the Sender interface, so this is a no-op that
// logs a warning. Use SendWithConfig for the full implementation.
func (s *ShellSender) Send(ctx context.Context, ch *models.NotificationChannel, msg Notification) error {
	slog.Warn("ShellSender.Send called without subscription config — use SendWithConfig")
	return nil
}

// SendWithConfig executes the shell command from the subscription config with
// environment variable substitution from the notification payload.
func (s *ShellSender) SendWithConfig(ctx context.Context, ch *models.NotificationChannel, msg Notification, subConfig json.RawMessage) error {
	var chCfg shellChannelConfig
	if len(ch.Config) > 0 {
		if err := json.Unmarshal(ch.Config, &chCfg); err != nil {
			return fmt.Errorf("parse shell channel config: %w", err)
		}
	}
	if chCfg.TimeoutSeconds == 0 {
		chCfg.TimeoutSeconds = 30
	}

	var subCfg shellSubscriptionConfig
	if len(subConfig) > 0 {
		if err := json.Unmarshal(subConfig, &subCfg); err != nil {
			return fmt.Errorf("parse shell subscription config: %w", err)
		}
	}
	if subCfg.Command == "" {
		return fmt.Errorf("shell subscription config: command is required")
	}

	timeout := chCfg.TimeoutSeconds
	if subCfg.TimeoutSeconds > 0 {
		timeout = subCfg.TimeoutSeconds
	}
	workingDir := chCfg.WorkingDir
	if subCfg.WorkingDir != "" {
		workingDir = subCfg.WorkingDir
	}

	env := append(os.Environ(),
		"CHANGELOGUE_VERSION="+msg.Version,
		"CHANGELOGUE_REPOSITORY="+msg.Repository,
		"CHANGELOGUE_PROVIDER="+msg.Provider,
		"CHANGELOGUE_PROJECT="+msg.ProjectName,
		"CHANGELOGUE_RELEASE_ID="+msg.ReleaseURL,
		"CHANGELOGUE_SOURCE_ID="+msg.SourceURL,
		"CHANGELOGUE_RAW_DATA="+msg.Body,
	)

	command := os.Expand(subCfg.Command, func(key string) string {
		for _, e := range env {
			if strings.HasPrefix(e, key+"=") {
				return e[len(key)+1:]
			}
		}
		return ""
	})

	go func() {
		execCtx, cancel := context.WithTimeout(context.Background(), time.Duration(timeout)*time.Second)
		defer cancel()

		cmd := exec.CommandContext(execCtx, "sh", "-c", command)
		cmd.Env = env
		if workingDir != "" {
			expanded := workingDir
			if strings.HasPrefix(expanded, "~") {
				home, _ := os.UserHomeDir()
				expanded = home + expanded[1:]
			}
			cmd.Dir = expanded
		}

		output, err := cmd.CombinedOutput()
		if err != nil {
			slog.Error("shell callback failed", "command", command, "err", err, "output", string(output))
		} else {
			slog.Debug("shell callback completed", "command", command, "output", string(output))
		}
	}()

	return nil
}
