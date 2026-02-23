package queue

import "testing"

func TestPipelineJobArgsKind(t *testing.T) {
	args := PipelineJobArgs{ReleaseID: "test-id"}
	if got := args.Kind(); got != "pipeline_process" {
		t.Errorf("Kind() = %q, want %q", got, "pipeline_process")
	}
}
