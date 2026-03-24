package gate

import (
	"context"
	"encoding/json"

	"github.com/sentioxyz/changelogue/internal/models"
)

// GateStore is the data access interface for gate workers.
type GateStore interface {
	// GetReleaseGateBySource loads the release gate config for the project that
	// owns the given source. Returns nil, nil if no gate exists.
	GetReleaseGateBySource(ctx context.Context, sourceID string) (*models.ReleaseGate, error)

	// GetReleaseGate loads a release gate by project ID. Returns nil, nil if none.
	GetReleaseGate(ctx context.Context, projectID string) (*models.ReleaseGate, error)

	// UpsertVersionReadiness atomically adds a source to sources_met for the
	// given project+version. Returns the updated row and whether the gate just
	// became ready (all sources met). Only updates rows with status='pending'.
	UpsertVersionReadiness(ctx context.Context, projectID, version, sourceID string, requiredSources []string, timeoutHours int) (*models.VersionReadiness, bool, error)

	// OpenGate sets a version_readiness row's status to the given value (ready
	// or timed_out). Only transitions from 'pending'. Returns false if already
	// transitioned.
	OpenGate(ctx context.Context, readinessID, status string) (bool, error)

	// MarkAgentTriggered sets agent_triggered=true on the readiness row.
	MarkAgentTriggered(ctx context.Context, readinessID string) error

	// RecordGateEvent inserts a gate_events row.
	RecordGateEvent(ctx context.Context, readinessID, projectID, version, eventType string, sourceID *string, details json.RawMessage) error

	// ListExpiredGates returns version_readiness rows where status='pending'
	// and timeout_at < now(), locked with FOR UPDATE SKIP LOCKED, up to limit.
	ListExpiredGates(ctx context.Context, limit int) ([]models.VersionReadiness, error)

	// GetSource loads a source by ID.
	GetSource(ctx context.Context, id string) (*models.Source, error)

	// GetProject loads a project by ID.
	GetProject(ctx context.Context, id string) (*models.Project, error)

	// ListSourcesByProject returns all sources for a project.
	ListSourcesByProject(ctx context.Context, projectID string, page, perPage int) ([]models.Source, int, error)

	// EnqueueAgentRun creates an agent_run row and enqueues the River job.
	EnqueueAgentRun(ctx context.Context, projectID, trigger, version string) error

	// GetVersionReadiness loads a version_readiness row by ID.
	GetVersionReadiness(ctx context.Context, id string) (*models.VersionReadiness, error)

	// UpdateNLRulePassed sets nl_rule_passed on a version_readiness row.
	UpdateNLRulePassed(ctx context.Context, readinessID string, passed bool) error
}
