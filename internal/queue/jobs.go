package queue

import "github.com/riverqueue/river"

// PipelineJobArgs defines the payload for a pipeline processing job.
type PipelineJobArgs struct {
	ReleaseID string `json:"release_id"`
}

func (PipelineJobArgs) Kind() string { return "pipeline_process" }

// Compile-time check that PipelineJobArgs satisfies river.JobArgs.
var _ river.JobArgs = PipelineJobArgs{}
