// Package pipeline defines the interface and types for the DAG processing pipeline.
package pipeline

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/sentioxyz/releaseguard/internal/models"
)

// ErrEventDropped signals that a node has decided to drop the event.
// The runner marks the pipeline job as "skipped" and stops processing.
var ErrEventDropped = errors.New("event dropped by pipeline node")

// PipelineNode is the interface all pipeline processing nodes implement.
// See DESIGN.md Section 2.2 for the canonical definition.
type PipelineNode interface {
	// Name returns the node identifier (e.g., "regex_normalizer").
	Name() string
	// Execute processes the event and returns a JSON result.
	// config is the node's config from pipeline_config (nil for always-on nodes).
	// prior contains results from previously executed nodes, keyed by node name.
	Execute(ctx context.Context, event *models.ReleaseEvent, config json.RawMessage, prior map[string]json.RawMessage) (json.RawMessage, error)
}
