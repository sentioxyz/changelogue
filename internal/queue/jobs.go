package queue

import "github.com/riverqueue/river"

// NotifyJobArgs is enqueued when a new source release is detected.
// The worker sends notifications to source-level subscribers.
type NotifyJobArgs struct {
	ReleaseID string `json:"release_id"`
	SourceID  string `json:"source_id"`
}

func (NotifyJobArgs) Kind() string { return "notify_release" }

var _ river.JobArgs = NotifyJobArgs{}

// AgentJobArgs is enqueued when an agent run is triggered.
// The worker runs the LLM agent to produce a semantic release.
type AgentJobArgs struct {
	AgentRunID string `json:"agent_run_id"`
	ProjectID  string `json:"project_id"`
}

func (AgentJobArgs) Kind() string { return "agent_run" }

var _ river.JobArgs = AgentJobArgs{}
