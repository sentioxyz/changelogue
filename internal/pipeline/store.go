package pipeline

import (
	"context"
	"encoding/json"

	"github.com/sentioxyz/releaseguard/internal/models"
)

// Store abstracts pipeline-related database operations.
// The Pipeline Runner uses this interface for all persistence during execution.
type Store interface {
	// GetReleasePayload loads the ReleaseEvent IR from the releases table.
	GetReleasePayload(ctx context.Context, releaseID string) (*models.ReleaseEvent, error)

	// CreatePipelineJob inserts a new pipeline_jobs row in "running" state
	// and returns the generated job ID.
	CreatePipelineJob(ctx context.Context, releaseID string) (int64, error)

	// UpdateNodeProgress records the current node and accumulated results
	// while the pipeline is still running.
	UpdateNodeProgress(ctx context.Context, jobID int64, currentNode string, nodeResults map[string]json.RawMessage) error

	// CompletePipelineJob marks the job as "completed" with final node results.
	CompletePipelineJob(ctx context.Context, jobID int64, nodeResults map[string]json.RawMessage) error

	// SkipPipelineJob marks the job as "skipped" (e.g., event dropped by a node).
	SkipPipelineJob(ctx context.Context, jobID int64, reason string) error

	// FailPipelineJob marks the job as "failed" with an error message.
	FailPipelineJob(ctx context.Context, jobID int64, errMsg string) error
}

// SubscriptionChecker determines whether a repository has active subscribers.
// Used by the Subscription Router node to decide if processing should continue.
type SubscriptionChecker interface {
	// HasSubscribers returns true if the repository has at least one enabled
	// subscription, or if no subscriptions exist at all ("default open" behavior).
	HasSubscribers(ctx context.Context, repository string) (bool, error)
}
