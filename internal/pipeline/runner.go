// Package pipeline — runner.go implements the core pipeline execution engine.
package pipeline

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/sentioxyz/releaseguard/internal/models"
)

// Runner orchestrates sequential pipeline execution for a single release event.
// It first runs always-on nodes (e.g., regex normalizer, subscription router),
// then runs configurable nodes that are enabled via pipelineConfig.
type Runner struct {
	store        Store
	alwaysOn     []PipelineNode
	configurable []PipelineNode
}

// NewRunner creates a Runner with the given store, always-on nodes, and
// configurable nodes. Always-on nodes run unconditionally; configurable nodes
// run only when their name appears as a key in the pipelineConfig map.
func NewRunner(store Store, alwaysOn []PipelineNode, configurable []PipelineNode) *Runner {
	return &Runner{store: store, alwaysOn: alwaysOn, configurable: configurable}
}

// Process runs the full pipeline for a release. pipelineConfig controls which
// configurable nodes are enabled — a node runs only if its Name() appears as a
// key. Pass nil to run only always-on nodes.
func (r *Runner) Process(ctx context.Context, releaseID string, pipelineConfig map[string]json.RawMessage) error {
	event, err := r.store.GetReleasePayload(ctx, releaseID)
	if err != nil {
		return fmt.Errorf("get release: %w", err)
	}

	jobID, err := r.store.CreatePipelineJob(ctx, releaseID)
	if err != nil {
		return fmt.Errorf("create pipeline job: %w", err)
	}

	nodeResults := make(map[string]json.RawMessage)

	// Always-on nodes run unconditionally with nil config.
	for _, node := range r.alwaysOn {
		stopped, err := r.runNode(ctx, jobID, node, event, nil, nodeResults)
		if err != nil {
			return err
		}
		if stopped {
			return nil
		}
	}

	// Configurable nodes run only when enabled in pipelineConfig.
	for _, node := range r.configurable {
		config, enabled := pipelineConfig[node.Name()]
		if !enabled {
			continue
		}
		stopped, err := r.runNode(ctx, jobID, node, event, config, nodeResults)
		if err != nil {
			return err
		}
		if stopped {
			return nil
		}
	}

	return r.store.CompletePipelineJob(ctx, jobID, nodeResults)
}

// runNode executes a single pipeline node, handling progress tracking, event
// drops, and failures. It returns (stopped, err): stopped is true when the
// pipeline should halt early (event dropped), and err is non-nil on failure.
// On ErrEventDropped the job is marked skipped and (true, nil) is returned
// since dropping is a normal pipeline outcome.
func (r *Runner) runNode(
	ctx context.Context,
	jobID int64,
	node PipelineNode,
	event *models.ReleaseEvent,
	config json.RawMessage,
	nodeResults map[string]json.RawMessage,
) (stopped bool, err error) {
	// Record which node we are about to execute.
	_ = r.store.UpdateNodeProgress(ctx, jobID, node.Name(), nodeResults)

	result, execErr := node.Execute(ctx, event, config, nodeResults)
	if errors.Is(execErr, ErrEventDropped) {
		_ = r.store.SkipPipelineJob(ctx, jobID, fmt.Sprintf("dropped by %s", node.Name()))
		return true, nil
	}
	if execErr != nil {
		_ = r.store.FailPipelineJob(ctx, jobID, execErr.Error())
		return false, fmt.Errorf("node %s: %w", node.Name(), execErr)
	}

	nodeResults[node.Name()] = result
	return false, nil
}
