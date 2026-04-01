package stealth

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/sentioxyz/changelogue/internal/api"
	"github.com/sentioxyz/changelogue/internal/auth"
	"github.com/sentioxyz/changelogue/internal/models"
)

var errNotImplemented = fmt.Errorf("not implemented in stealth mode")

// ---------------------------------------------------------------------------
// SemanticReleasesStore stub
// ---------------------------------------------------------------------------

// SemanticReleasesStub is a no-op implementation of api.SemanticReleasesStore.
type SemanticReleasesStub struct{}

var _ api.SemanticReleasesStore = (*SemanticReleasesStub)(nil)

func (SemanticReleasesStub) ListAllSemanticReleases(_ context.Context, _, _ int) ([]models.SemanticRelease, int, error) {
	return nil, 0, errNotImplemented
}

func (SemanticReleasesStub) ListSemanticReleases(_ context.Context, _ string, _, _ int) ([]models.SemanticRelease, int, error) {
	return nil, 0, errNotImplemented
}

func (SemanticReleasesStub) GetSemanticRelease(_ context.Context, _ string) (*models.SemanticRelease, error) {
	return nil, errNotImplemented
}

func (SemanticReleasesStub) GetSemanticReleaseSources(_ context.Context, _ string) ([]models.Release, error) {
	return nil, errNotImplemented
}

func (SemanticReleasesStub) DeleteSemanticRelease(_ context.Context, _ string) error {
	return errNotImplemented
}

// ---------------------------------------------------------------------------
// AgentStore stub
// ---------------------------------------------------------------------------

// AgentStub is a no-op implementation of api.AgentStore.
type AgentStub struct{}

var _ api.AgentStore = (*AgentStub)(nil)

func (AgentStub) TriggerAgentRun(_ context.Context, _, _, _ string) (*models.AgentRun, error) {
	return nil, errNotImplemented
}

func (AgentStub) ListAgentRuns(_ context.Context, _ string, _, _ int) ([]models.AgentRun, int, error) {
	return nil, 0, errNotImplemented
}

func (AgentStub) GetAgentRun(_ context.Context, _ string) (*models.AgentRun, error) {
	return nil, errNotImplemented
}

// ---------------------------------------------------------------------------
// TodosStore stub
// ---------------------------------------------------------------------------

// TodosStub is a no-op implementation of api.TodosStore.
type TodosStub struct{}

var _ api.TodosStore = (*TodosStub)(nil)

func (TodosStub) ListTodos(_ context.Context, _ string, _, _ int, _ bool, _ api.TodoFilter) ([]models.Todo, int, error) {
	return nil, 0, errNotImplemented
}

func (TodosStub) GetTodo(_ context.Context, _ string) (*models.Todo, error) {
	return nil, errNotImplemented
}

func (TodosStub) AcknowledgeTodo(_ context.Context, _ string, _ bool) error {
	return errNotImplemented
}

func (TodosStub) ResolveTodo(_ context.Context, _ string, _ bool) error {
	return errNotImplemented
}

func (TodosStub) ReopenTodo(_ context.Context, _ string) error {
	return errNotImplemented
}

// ---------------------------------------------------------------------------
// OnboardStore stub
// ---------------------------------------------------------------------------

// OnboardStub is a no-op implementation of api.OnboardStore.
type OnboardStub struct{}

var _ api.OnboardStore = (*OnboardStub)(nil)

func (OnboardStub) CreateOnboardScan(_ context.Context, _ string) (*models.OnboardScan, error) {
	return nil, errNotImplemented
}

func (OnboardStub) GetOnboardScan(_ context.Context, _ string) (*models.OnboardScan, error) {
	return nil, errNotImplemented
}

func (OnboardStub) UpdateOnboardScanStatus(_ context.Context, _, _ string, _ json.RawMessage, _ string) error {
	return errNotImplemented
}

func (OnboardStub) ActiveScanForRepo(_ context.Context, _ string) (*models.OnboardScan, error) {
	return nil, errNotImplemented
}

func (OnboardStub) ApplyOnboardScan(_ context.Context, _ string, _ []api.OnboardSelection) (*api.OnboardApplyResult, error) {
	return nil, errNotImplemented
}

// ---------------------------------------------------------------------------
// GatesStore stub
// ---------------------------------------------------------------------------

// GatesStub is a no-op implementation of api.GatesStore.
type GatesStub struct{}

var _ api.GatesStore = (*GatesStub)(nil)

func (GatesStub) GetReleaseGate(_ context.Context, _ string) (*models.ReleaseGate, error) {
	return nil, errNotImplemented
}

func (GatesStub) CreateReleaseGate(_ context.Context, _ *models.ReleaseGate) error {
	return errNotImplemented
}

func (GatesStub) UpdateReleaseGate(_ context.Context, _ *models.ReleaseGate) error {
	return errNotImplemented
}

func (GatesStub) DeleteReleaseGate(_ context.Context, _ string) error {
	return errNotImplemented
}

func (GatesStub) ListVersionReadiness(_ context.Context, _ string, _, _ int) ([]models.VersionReadiness, int, error) {
	return nil, 0, errNotImplemented
}

func (GatesStub) GetVersionReadinessByVersion(_ context.Context, _, _ string) (*models.VersionReadiness, error) {
	return nil, errNotImplemented
}

func (GatesStub) ListGateEvents(_ context.Context, _ string, _, _ int) ([]models.GateEvent, int, error) {
	return nil, 0, errNotImplemented
}

func (GatesStub) ListGateEventsByVersion(_ context.Context, _, _ string, _, _ int) ([]models.GateEvent, int, error) {
	return nil, 0, errNotImplemented
}

// ---------------------------------------------------------------------------
// ContextSourcesStore stub
// ---------------------------------------------------------------------------

// ContextSourcesStub is a no-op implementation of api.ContextSourcesStore.
type ContextSourcesStub struct{}

var _ api.ContextSourcesStore = (*ContextSourcesStub)(nil)

func (ContextSourcesStub) ListContextSources(_ context.Context, _ string, _, _ int) ([]models.ContextSource, int, error) {
	return nil, 0, errNotImplemented
}

func (ContextSourcesStub) CreateContextSource(_ context.Context, _ *models.ContextSource) error {
	return errNotImplemented
}

func (ContextSourcesStub) GetContextSource(_ context.Context, _ string) (*models.ContextSource, error) {
	return nil, errNotImplemented
}

func (ContextSourcesStub) UpdateContextSource(_ context.Context, _ string, _ *models.ContextSource) error {
	return errNotImplemented
}

func (ContextSourcesStub) DeleteContextSource(_ context.Context, _ string) error {
	return errNotImplemented
}

// ---------------------------------------------------------------------------
// SessionValidator stub
// ---------------------------------------------------------------------------

// SessionValidatorStub is a no-op implementation of api.SessionValidator.
type SessionValidatorStub struct{}

var _ api.SessionValidator = (*SessionValidatorStub)(nil)

func (SessionValidatorStub) ValidateSession(_ context.Context, _ string) (*auth.User, error) {
	return nil, errNotImplemented
}
