package routing

import (
	"context"
	"encoding/json"
	"errors"
	"sync"
	"testing"

	"github.com/riverqueue/river"
	"github.com/sentioxyz/changelogue/internal/models"
	"github.com/sentioxyz/changelogue/internal/queue"
)

// --- mock store ---

type mockNotifyStore struct {
	releases         map[string]*models.Release
	sources          map[string]*models.Source
	subscriptions    map[string][]models.Subscription // keyed by sourceID
	channels         map[string]*models.NotificationChannel
	projects         map[string]*models.Project
	previousReleases map[string]*models.Release // keyed by sourceID
	err              error                      // if set, all methods return this error

	// Track EnqueueAgentRun calls.
	mu              sync.Mutex
	agentRunCalls   []agentRunCall
	enqueueAgentErr error
}

type agentRunCall struct {
	ProjectID string
	Trigger   string
	Version   string
}

func (m *mockNotifyStore) GetRelease(_ context.Context, id string) (*models.Release, error) {
	if m.err != nil {
		return nil, m.err
	}
	rel, ok := m.releases[id]
	if !ok {
		return nil, errors.New("release not found")
	}
	return rel, nil
}

func (m *mockNotifyStore) GetSource(_ context.Context, id string) (*models.Source, error) {
	if m.err != nil {
		return nil, m.err
	}
	src, ok := m.sources[id]
	if !ok {
		return nil, errors.New("source not found")
	}
	return src, nil
}

func (m *mockNotifyStore) ListSourceSubscriptions(_ context.Context, sourceID string) ([]models.Subscription, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.subscriptions[sourceID], nil
}

func (m *mockNotifyStore) GetChannel(_ context.Context, id string) (*models.NotificationChannel, error) {
	if m.err != nil {
		return nil, m.err
	}
	ch, ok := m.channels[id]
	if !ok {
		return nil, errors.New("channel not found")
	}
	return ch, nil
}

func (m *mockNotifyStore) GetProject(_ context.Context, id string) (*models.Project, error) {
	if m.err != nil {
		return nil, m.err
	}
	if m.projects == nil {
		return nil, errors.New("project not found")
	}
	p, ok := m.projects[id]
	if !ok {
		return nil, errors.New("project not found")
	}
	return p, nil
}

func (m *mockNotifyStore) GetPreviousRelease(_ context.Context, sourceID string, _ string) (*models.Release, error) {
	if m.err != nil {
		return nil, m.err
	}
	if m.previousReleases == nil {
		return nil, nil
	}
	return m.previousReleases[sourceID], nil
}

func (m *mockNotifyStore) EnqueueAgentRun(_ context.Context, projectID, trigger, version string) error {
	if m.enqueueAgentErr != nil {
		return m.enqueueAgentErr
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.agentRunCalls = append(m.agentRunCalls, agentRunCall{ProjectID: projectID, Trigger: trigger, Version: version})
	return nil
}

func (m *mockNotifyStore) agentRunCallCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.agentRunCalls)
}

// --- mock sender ---

type mockSender struct {
	mu      sync.Mutex
	sent    []Notification
	sendErr error
}

func (m *mockSender) Send(_ context.Context, _ *models.NotificationChannel, msg Notification) error {
	if m.sendErr != nil {
		return m.sendErr
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.sent = append(m.sent, msg)
	return nil
}

func (m *mockSender) sentCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.sent)
}

// --- notification tests ---

func TestNotifyWorker_Work(t *testing.T) {
	sourceID := "src-1"
	releaseID := "rel-1"
	channelID := "ch-1"

	store := &mockNotifyStore{
		releases: map[string]*models.Release{
			releaseID: {ID: releaseID, SourceID: sourceID, Version: "v1.14.0", RawData: json.RawMessage(`{"tag":"v1.14.0"}`)},
		},
		sources: map[string]*models.Source{
			sourceID: {ID: sourceID, ProjectID: "proj-1", Provider: "github", Repository: "ethereum/go-ethereum"},
		},
		subscriptions: map[string][]models.Subscription{
			sourceID: {
				{ID: "sub-1", ChannelID: channelID, Type: "source_release", SourceID: &sourceID},
			},
		},
		channels: map[string]*models.NotificationChannel{
			channelID: {ID: channelID, Name: "test-webhook", Type: "webhook", Config: json.RawMessage(`{"url":"http://example.com"}`)},
		},
		projects: map[string]*models.Project{
			"proj-1": {ID: "proj-1", Name: "test", AgentRules: json.RawMessage(`{}`)},
		},
	}

	webhookSender := &mockSender{}
	worker := &NotifyWorker{
		store: store,
		senders: map[string]Sender{
			"webhook": webhookSender,
		},
	}

	job := &river.Job[queue.NotifyJobArgs]{
		Args: queue.NotifyJobArgs{
			ReleaseID: releaseID,
			SourceID:  sourceID,
		},
	}

	err := worker.Work(context.Background(), job)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if webhookSender.sentCount() != 1 {
		t.Fatalf("expected 1 notification sent, got %d", webhookSender.sentCount())
	}
	if webhookSender.sent[0].Version != "v1.14.0" {
		t.Errorf("expected version v1.14.0, got %s", webhookSender.sent[0].Version)
	}
}

func TestNotifyWorker_MultipleSubscriptions(t *testing.T) {
	sourceID := "src-1"
	releaseID := "rel-1"

	store := &mockNotifyStore{
		releases: map[string]*models.Release{
			releaseID: {ID: releaseID, SourceID: sourceID, Version: "v2.0.0", RawData: json.RawMessage(`{}`)},
		},
		sources: map[string]*models.Source{
			sourceID: {ID: sourceID, ProjectID: "proj-1"},
		},
		subscriptions: map[string][]models.Subscription{
			sourceID: {
				{ID: "sub-1", ChannelID: "ch-webhook", Type: "source_release", SourceID: &sourceID},
				{ID: "sub-2", ChannelID: "ch-slack", Type: "source_release", SourceID: &sourceID},
			},
		},
		channels: map[string]*models.NotificationChannel{
			"ch-webhook": {ID: "ch-webhook", Name: "webhook", Type: "webhook", Config: json.RawMessage(`{"url":"http://example.com"}`)},
			"ch-slack":   {ID: "ch-slack", Name: "slack", Type: "slack", Config: json.RawMessage(`{"webhook_url":"http://slack.example.com"}`)},
		},
		projects: map[string]*models.Project{
			"proj-1": {ID: "proj-1", Name: "test", AgentRules: json.RawMessage(`{}`)},
		},
	}

	webhookSender := &mockSender{}
	slackSender := &mockSender{}
	worker := &NotifyWorker{
		store: store,
		senders: map[string]Sender{
			"webhook": webhookSender,
			"slack":   slackSender,
		},
	}

	job := &river.Job[queue.NotifyJobArgs]{
		Args: queue.NotifyJobArgs{ReleaseID: releaseID, SourceID: sourceID},
	}

	err := worker.Work(context.Background(), job)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if webhookSender.sentCount() != 1 {
		t.Errorf("expected 1 webhook notification, got %d", webhookSender.sentCount())
	}
	if slackSender.sentCount() != 1 {
		t.Errorf("expected 1 slack notification, got %d", slackSender.sentCount())
	}
}

func TestNotifyWorker_ReleaseNotFound(t *testing.T) {
	store := &mockNotifyStore{
		releases:      map[string]*models.Release{},
		subscriptions: map[string][]models.Subscription{},
		channels:      map[string]*models.NotificationChannel{},
	}

	worker := &NotifyWorker{store: store, senders: map[string]Sender{}}
	job := &river.Job[queue.NotifyJobArgs]{
		Args: queue.NotifyJobArgs{ReleaseID: "nonexistent", SourceID: "src-1"},
	}

	err := worker.Work(context.Background(), job)
	if err == nil {
		t.Fatal("expected error when release not found")
	}
}

func TestNotifyWorker_NoSubscriptions(t *testing.T) {
	sourceID := "src-1"
	releaseID := "rel-1"

	store := &mockNotifyStore{
		releases: map[string]*models.Release{
			releaseID: {ID: releaseID, SourceID: sourceID, Version: "v1.0.0", RawData: json.RawMessage(`{}`)},
		},
		sources: map[string]*models.Source{
			sourceID: {ID: sourceID, ProjectID: "proj-1"},
		},
		subscriptions: map[string][]models.Subscription{},
		channels:      map[string]*models.NotificationChannel{},
		projects: map[string]*models.Project{
			"proj-1": {ID: "proj-1", Name: "test", AgentRules: json.RawMessage(`{}`)},
		},
	}

	webhookSender := &mockSender{}
	worker := &NotifyWorker{
		store:   store,
		senders: map[string]Sender{"webhook": webhookSender},
	}

	job := &river.Job[queue.NotifyJobArgs]{
		Args: queue.NotifyJobArgs{ReleaseID: releaseID, SourceID: sourceID},
	}

	err := worker.Work(context.Background(), job)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if webhookSender.sentCount() != 0 {
		t.Errorf("expected 0 notifications, got %d", webhookSender.sentCount())
	}
}

func TestNotifyWorker_UnknownChannelType(t *testing.T) {
	sourceID := "src-1"
	releaseID := "rel-1"

	store := &mockNotifyStore{
		releases: map[string]*models.Release{
			releaseID: {ID: releaseID, SourceID: sourceID, Version: "v1.0.0", RawData: json.RawMessage(`{}`)},
		},
		sources: map[string]*models.Source{
			sourceID: {ID: sourceID, ProjectID: "proj-1"},
		},
		subscriptions: map[string][]models.Subscription{
			sourceID: {{ID: "sub-1", ChannelID: "ch-1", Type: "source_release", SourceID: &sourceID}},
		},
		channels: map[string]*models.NotificationChannel{
			"ch-1": {ID: "ch-1", Name: "pagerduty", Type: "pagerduty", Config: json.RawMessage(`{}`)},
		},
		projects: map[string]*models.Project{
			"proj-1": {ID: "proj-1", Name: "test", AgentRules: json.RawMessage(`{}`)},
		},
	}

	worker := &NotifyWorker{
		store:   store,
		senders: map[string]Sender{"webhook": &mockSender{}},
	}

	job := &river.Job[queue.NotifyJobArgs]{
		Args: queue.NotifyJobArgs{ReleaseID: releaseID, SourceID: sourceID},
	}

	// Should not error — unknown channel types are logged and skipped.
	err := worker.Work(context.Background(), job)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestNotifyWorker_SenderError(t *testing.T) {
	sourceID := "src-1"
	releaseID := "rel-1"

	store := &mockNotifyStore{
		releases: map[string]*models.Release{
			releaseID: {ID: releaseID, SourceID: sourceID, Version: "v1.0.0", RawData: json.RawMessage(`{}`)},
		},
		sources: map[string]*models.Source{
			sourceID: {ID: sourceID, ProjectID: "proj-1"},
		},
		subscriptions: map[string][]models.Subscription{
			sourceID: {{ID: "sub-1", ChannelID: "ch-1", Type: "source_release", SourceID: &sourceID}},
		},
		channels: map[string]*models.NotificationChannel{
			"ch-1": {ID: "ch-1", Name: "test", Type: "webhook", Config: json.RawMessage(`{"url":"http://example.com"}`)},
		},
		projects: map[string]*models.Project{
			"proj-1": {ID: "proj-1", Name: "test", AgentRules: json.RawMessage(`{}`)},
		},
	}

	failingSender := &mockSender{sendErr: errors.New("network timeout")}
	worker := &NotifyWorker{
		store:   store,
		senders: map[string]Sender{"webhook": failingSender},
	}

	job := &river.Job[queue.NotifyJobArgs]{
		Args: queue.NotifyJobArgs{ReleaseID: releaseID, SourceID: sourceID},
	}

	// Sender errors are logged but should not fail the worker — partial delivery is OK.
	err := worker.Work(context.Background(), job)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// --- agent rule triggering tests ---

func TestNotifyWorker_AgentRulesTriggered_MajorBump(t *testing.T) {
	sourceID := "src-1"
	releaseID := "rel-1"
	projectID := "proj-1"

	store := &mockNotifyStore{
		releases: map[string]*models.Release{
			releaseID: {ID: releaseID, SourceID: sourceID, Version: "v2.0.0", RawData: json.RawMessage(`{}`)},
		},
		sources: map[string]*models.Source{
			sourceID: {ID: sourceID, ProjectID: projectID},
		},
		subscriptions: map[string][]models.Subscription{},
		channels:      map[string]*models.NotificationChannel{},
		projects: map[string]*models.Project{
			projectID: {
				ID:         projectID,
				Name:       "test-project",
				AgentRules: json.RawMessage(`{"on_major_release":true}`),
			},
		},
		previousReleases: map[string]*models.Release{
			sourceID: {ID: "rel-0", SourceID: sourceID, Version: "v1.5.3"},
		},
	}

	worker := &NotifyWorker{store: store, senders: map[string]Sender{}}

	job := &river.Job[queue.NotifyJobArgs]{
		Args: queue.NotifyJobArgs{ReleaseID: releaseID, SourceID: sourceID},
	}

	err := worker.Work(context.Background(), job)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if store.agentRunCallCount() != 1 {
		t.Fatalf("expected 1 EnqueueAgentRun call, got %d", store.agentRunCallCount())
	}
	if store.agentRunCalls[0].ProjectID != projectID {
		t.Errorf("expected project ID %s, got %s", projectID, store.agentRunCalls[0].ProjectID)
	}
	expectedTrigger := "auto:version:v2.0.0"
	if store.agentRunCalls[0].Trigger != expectedTrigger {
		t.Errorf("expected trigger %q, got %q", expectedTrigger, store.agentRunCalls[0].Trigger)
	}
}

func TestNotifyWorker_AgentRulesNotTriggered_PatchBump(t *testing.T) {
	sourceID := "src-1"
	releaseID := "rel-1"
	projectID := "proj-1"

	store := &mockNotifyStore{
		releases: map[string]*models.Release{
			releaseID: {ID: releaseID, SourceID: sourceID, Version: "v1.5.4", RawData: json.RawMessage(`{}`)},
		},
		sources: map[string]*models.Source{
			sourceID: {ID: sourceID, ProjectID: projectID},
		},
		subscriptions: map[string][]models.Subscription{},
		channels:      map[string]*models.NotificationChannel{},
		projects: map[string]*models.Project{
			projectID: {
				ID:         projectID,
				Name:       "test-project",
				AgentRules: json.RawMessage(`{"on_major_release":true}`),
			},
		},
		previousReleases: map[string]*models.Release{
			sourceID: {ID: "rel-0", SourceID: sourceID, Version: "v1.5.3"},
		},
	}

	worker := &NotifyWorker{store: store, senders: map[string]Sender{}}

	job := &river.Job[queue.NotifyJobArgs]{
		Args: queue.NotifyJobArgs{ReleaseID: releaseID, SourceID: sourceID},
	}

	err := worker.Work(context.Background(), job)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if store.agentRunCallCount() != 0 {
		t.Fatalf("expected 0 EnqueueAgentRun calls, got %d", store.agentRunCallCount())
	}
}

func TestNotifyWorker_AgentRulesTriggered_NoPreviousRelease(t *testing.T) {
	sourceID := "src-1"
	releaseID := "rel-1"
	projectID := "proj-1"

	store := &mockNotifyStore{
		releases: map[string]*models.Release{
			releaseID: {ID: releaseID, SourceID: sourceID, Version: "v1.0.0", RawData: json.RawMessage(`{}`)},
		},
		sources: map[string]*models.Source{
			sourceID: {ID: sourceID, ProjectID: projectID},
		},
		subscriptions: map[string][]models.Subscription{},
		channels:      map[string]*models.NotificationChannel{},
		projects: map[string]*models.Project{
			projectID: {
				ID:         projectID,
				Name:       "test-project",
				AgentRules: json.RawMessage(`{"on_major_release":true}`),
			},
		},
		// No previous releases — first release.
	}

	worker := &NotifyWorker{store: store, senders: map[string]Sender{}}

	job := &river.Job[queue.NotifyJobArgs]{
		Args: queue.NotifyJobArgs{ReleaseID: releaseID, SourceID: sourceID},
	}

	err := worker.Work(context.Background(), job)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// v1.0.0 vs "" — major > 0, so OnMajorRelease should trigger.
	if store.agentRunCallCount() != 1 {
		t.Fatalf("expected 1 EnqueueAgentRun call for first release, got %d", store.agentRunCallCount())
	}
}

func TestNotifyWorker_AgentRulesEmptyRules(t *testing.T) {
	sourceID := "src-1"
	releaseID := "rel-1"
	projectID := "proj-1"

	store := &mockNotifyStore{
		releases: map[string]*models.Release{
			releaseID: {ID: releaseID, SourceID: sourceID, Version: "v2.0.0", RawData: json.RawMessage(`{}`)},
		},
		sources: map[string]*models.Source{
			sourceID: {ID: sourceID, ProjectID: projectID},
		},
		subscriptions: map[string][]models.Subscription{},
		channels:      map[string]*models.NotificationChannel{},
		projects: map[string]*models.Project{
			projectID: {
				ID:         projectID,
				Name:       "test-project",
				AgentRules: json.RawMessage(`{}`),
			},
		},
		previousReleases: map[string]*models.Release{
			sourceID: {ID: "rel-0", SourceID: sourceID, Version: "v1.0.0"},
		},
	}

	worker := &NotifyWorker{store: store, senders: map[string]Sender{}}

	job := &river.Job[queue.NotifyJobArgs]{
		Args: queue.NotifyJobArgs{ReleaseID: releaseID, SourceID: sourceID},
	}

	err := worker.Work(context.Background(), job)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// All rules are false — should not trigger.
	if store.agentRunCallCount() != 0 {
		t.Fatalf("expected 0 EnqueueAgentRun calls for empty rules, got %d", store.agentRunCallCount())
	}
}

func TestNotifyWorker_AgentRulesTriggered_SecurityPatch(t *testing.T) {
	sourceID := "src-1"
	releaseID := "rel-1"
	projectID := "proj-1"

	store := &mockNotifyStore{
		releases: map[string]*models.Release{
			releaseID: {ID: releaseID, SourceID: sourceID, Version: "v1.5.4-security", RawData: json.RawMessage(`{}`)},
		},
		sources: map[string]*models.Source{
			sourceID: {ID: sourceID, ProjectID: projectID},
		},
		subscriptions: map[string][]models.Subscription{},
		channels:      map[string]*models.NotificationChannel{},
		projects: map[string]*models.Project{
			projectID: {
				ID:         projectID,
				Name:       "test-project",
				AgentRules: json.RawMessage(`{"on_security_patch":true}`),
			},
		},
		previousReleases: map[string]*models.Release{
			sourceID: {ID: "rel-0", SourceID: sourceID, Version: "v1.5.3"},
		},
	}

	worker := &NotifyWorker{store: store, senders: map[string]Sender{}}

	job := &river.Job[queue.NotifyJobArgs]{
		Args: queue.NotifyJobArgs{ReleaseID: releaseID, SourceID: sourceID},
	}

	err := worker.Work(context.Background(), job)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if store.agentRunCallCount() != 1 {
		t.Fatalf("expected 1 EnqueueAgentRun call for security patch, got %d", store.agentRunCallCount())
	}
}

func TestNotifyWorker_VersionFilterExclude(t *testing.T) {
	sourceID := "src-1"
	releaseID := "rel-1"
	channelID := "ch-1"
	exclude := "-(alpha|beta|rc)"

	store := &mockNotifyStore{
		releases: map[string]*models.Release{
			releaseID: {ID: releaseID, SourceID: sourceID, Version: "v1.0.0-beta", RawData: json.RawMessage(`{}`)},
		},
		sources: map[string]*models.Source{
			sourceID: {ID: sourceID, ProjectID: "proj-1", VersionFilterExclude: &exclude},
		},
		subscriptions: map[string][]models.Subscription{
			sourceID: {{ID: "sub-1", ChannelID: channelID, Type: "source_release", SourceID: &sourceID}},
		},
		channels: map[string]*models.NotificationChannel{
			channelID: {ID: channelID, Name: "test", Type: "webhook", Config: json.RawMessage(`{"url":"http://example.com"}`)},
		},
		projects: map[string]*models.Project{
			"proj-1": {ID: "proj-1", Name: "test", AgentRules: json.RawMessage(`{}`)},
		},
	}

	webhookSender := &mockSender{}
	worker := &NotifyWorker{store: store, senders: map[string]Sender{"webhook": webhookSender}}
	job := &river.Job[queue.NotifyJobArgs]{Args: queue.NotifyJobArgs{ReleaseID: releaseID, SourceID: sourceID}}

	err := worker.Work(context.Background(), job)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if webhookSender.sentCount() != 0 {
		t.Fatalf("expected 0 notifications (excluded by filter), got %d", webhookSender.sentCount())
	}
	if store.agentRunCallCount() != 0 {
		t.Fatalf("expected 0 agent runs (excluded by filter), got %d", store.agentRunCallCount())
	}
}

func TestNotifyWorker_VersionFilterInclude(t *testing.T) {
	sourceID := "src-1"
	releaseID := "rel-1"
	channelID := "ch-1"
	include := `^v\d+\.\d+\.\d+$`

	store := &mockNotifyStore{
		releases: map[string]*models.Release{
			releaseID: {ID: releaseID, SourceID: sourceID, Version: "nightly-20260301", RawData: json.RawMessage(`{}`)},
		},
		sources: map[string]*models.Source{
			sourceID: {ID: sourceID, ProjectID: "proj-1", VersionFilterInclude: &include},
		},
		subscriptions: map[string][]models.Subscription{
			sourceID: {{ID: "sub-1", ChannelID: channelID, Type: "source_release", SourceID: &sourceID}},
		},
		channels: map[string]*models.NotificationChannel{
			channelID: {ID: channelID, Name: "test", Type: "webhook", Config: json.RawMessage(`{"url":"http://example.com"}`)},
		},
		projects: map[string]*models.Project{
			"proj-1": {ID: "proj-1", Name: "test", AgentRules: json.RawMessage(`{}`)},
		},
	}

	webhookSender := &mockSender{}
	worker := &NotifyWorker{store: store, senders: map[string]Sender{"webhook": webhookSender}}
	job := &river.Job[queue.NotifyJobArgs]{Args: queue.NotifyJobArgs{ReleaseID: releaseID, SourceID: sourceID}}

	err := worker.Work(context.Background(), job)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if webhookSender.sentCount() != 0 {
		t.Fatalf("expected 0 notifications (not matching include), got %d", webhookSender.sentCount())
	}
}

func TestNotifyWorker_VersionFilterPassesThrough(t *testing.T) {
	sourceID := "src-1"
	releaseID := "rel-1"
	channelID := "ch-1"
	include := `^v\d+\.\d+\.\d+$`
	exclude := "-beta"

	store := &mockNotifyStore{
		releases: map[string]*models.Release{
			releaseID: {ID: releaseID, SourceID: sourceID, Version: "v2.0.0", RawData: json.RawMessage(`{}`)},
		},
		sources: map[string]*models.Source{
			sourceID: {ID: sourceID, ProjectID: "proj-1", VersionFilterInclude: &include, VersionFilterExclude: &exclude},
		},
		subscriptions: map[string][]models.Subscription{
			sourceID: {{ID: "sub-1", ChannelID: channelID, Type: "source_release", SourceID: &sourceID}},
		},
		channels: map[string]*models.NotificationChannel{
			channelID: {ID: channelID, Name: "test", Type: "webhook", Config: json.RawMessage(`{"url":"http://example.com"}`)},
		},
		projects: map[string]*models.Project{
			"proj-1": {ID: "proj-1", Name: "test", AgentRules: json.RawMessage(`{}`)},
		},
	}

	webhookSender := &mockSender{}
	worker := &NotifyWorker{store: store, senders: map[string]Sender{"webhook": webhookSender}}
	job := &river.Job[queue.NotifyJobArgs]{Args: queue.NotifyJobArgs{ReleaseID: releaseID, SourceID: sourceID}}

	err := worker.Work(context.Background(), job)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if webhookSender.sentCount() != 1 {
		t.Fatalf("expected 1 notification (passes all filters), got %d", webhookSender.sentCount())
	}
}
