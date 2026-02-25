package queue

import "testing"

func TestNotifyJobArgsKind(t *testing.T) {
	args := NotifyJobArgs{ReleaseID: "test-id", SourceID: "src-1"}
	if got := args.Kind(); got != "notify_release" {
		t.Errorf("Kind() = %q, want %q", got, "notify_release")
	}
}

func TestAgentJobArgsKind(t *testing.T) {
	args := AgentJobArgs{AgentRunID: "run-1", ProjectID: "proj-1"}
	if got := args.Kind(); got != "agent_run" {
		t.Errorf("Kind() = %q, want %q", got, "agent_run")
	}
}
