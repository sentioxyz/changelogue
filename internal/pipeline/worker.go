// Package pipeline — worker.go connects River's job processing to the pipeline Runner.
package pipeline

import (
	"context"
	"encoding/json"

	"github.com/riverqueue/river"
	"github.com/sentioxyz/releaseguard/internal/queue"
)

// Worker is a River worker that processes pipeline jobs.
type Worker struct {
	river.WorkerDefaults[queue.PipelineJobArgs]
	runner *Runner
}

// NewWorker creates a Worker that delegates to the given Runner.
func NewWorker(runner *Runner) *Worker {
	return &Worker{runner: runner}
}

// Work processes a pipeline job by running the release through the pipeline.
func (w *Worker) Work(ctx context.Context, job *river.Job[queue.PipelineJobArgs]) error {
	// TODO: Load pipeline_config from project when projects table is implemented.
	// For now, only always-on nodes run (nil config = no configurable nodes enabled).
	var pipelineConfig map[string]json.RawMessage
	return w.runner.Process(ctx, job.Args.ReleaseID, pipelineConfig)
}
