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
	Version    string `json:"version"`
}

func (AgentJobArgs) Kind() string { return "agent_run" }

var _ river.JobArgs = AgentJobArgs{}

// ScanDependenciesJobArgs is enqueued when a user requests a GitHub repo scan.
// The worker fetches dependency files and extracts dependencies via LLM.
type ScanDependenciesJobArgs struct {
	ScanID string `json:"scan_id"`
}

func (ScanDependenciesJobArgs) Kind() string { return "scan_dependencies" }

var _ river.JobArgs = ScanDependenciesJobArgs{}

// GateCheckJobArgs is enqueued when a release is ingested for any project.
// The worker checks if a release gate exists and evaluates readiness.
type GateCheckJobArgs struct {
	SourceID  string `json:"source_id"`
	ReleaseID string `json:"release_id"`
	Version   string `json:"version"` // raw version from source
}

func (GateCheckJobArgs) Kind() string { return "gate_check" }

var _ river.JobArgs = GateCheckJobArgs{}

// GateNLEvalJobArgs is enqueued when structured gate rules pass and an NL rule
// needs LLM evaluation.
type GateNLEvalJobArgs struct {
	VersionReadinessID string `json:"version_readiness_id"`
	ProjectID          string `json:"project_id"`
	Version            string `json:"version"`
}

func (GateNLEvalJobArgs) Kind() string { return "gate_nl_eval" }

var _ river.JobArgs = GateNLEvalJobArgs{}

// GateTimeoutJobArgs is a periodic job that sweeps expired pending gates.
type GateTimeoutJobArgs struct{}

func (GateTimeoutJobArgs) Kind() string { return "gate_timeout" }

var _ river.JobArgs = GateTimeoutJobArgs{}
