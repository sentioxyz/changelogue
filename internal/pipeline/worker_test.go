package pipeline

import (
	"testing"

	"github.com/riverqueue/river"
	"github.com/sentioxyz/releaseguard/internal/queue"
)

func TestPipelineWorkerRegistration(t *testing.T) {
	// Verify Worker can be added to River workers without panic
	workers := river.NewWorkers()
	river.AddWorker(workers, NewWorker(nil))
}

// Compile-time check that Worker implements river.Worker.
var _ river.Worker[queue.PipelineJobArgs] = (*Worker)(nil)
