package routing

import (
	"context"
	"encoding/json"
	"errors"
	"sync"
	"testing"

	"github.com/riverqueue/river"
	"github.com/sentioxyz/releaseguard/internal/models"
	"github.com/sentioxyz/releaseguard/internal/queue"
)

// --- mock store ---

type mockNotifyStore struct {
	releases      map[string]*models.Release
	sources       map[string]*models.Source
	subscriptions map[string][]models.Subscription // keyed by sourceID
	channels      map[string]*models.NotificationChannel
	err           error // if set, all methods return this error
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

// --- mock sender ---

type mockSender struct {
	mu       sync.Mutex
	sent     []Notification
	sendErr  error
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

// --- tests ---

func TestNotifyWorker_Work(t *testing.T) {
	sourceID := "src-1"
	releaseID := "rel-1"
	channelID := "ch-1"

	store := &mockNotifyStore{
		releases: map[string]*models.Release{
			releaseID: {ID: releaseID, SourceID: sourceID, Version: "v1.14.0", RawData: json.RawMessage(`{"tag":"v1.14.0"}`)},
		},
		sources: map[string]*models.Source{
			sourceID: {ID: sourceID, Provider: "github", Repository: "ethereum/go-ethereum"},
		},
		subscriptions: map[string][]models.Subscription{
			sourceID: {
				{ID: "sub-1", ChannelID: channelID, Type: "source", SourceID: &sourceID},
			},
		},
		channels: map[string]*models.NotificationChannel{
			channelID: {ID: channelID, Name: "test-webhook", Type: "webhook", Config: json.RawMessage(`{"url":"http://example.com"}`)},
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
			sourceID: {ID: sourceID},
		},
		subscriptions: map[string][]models.Subscription{
			sourceID: {
				{ID: "sub-1", ChannelID: "ch-webhook", Type: "source", SourceID: &sourceID},
				{ID: "sub-2", ChannelID: "ch-slack", Type: "source", SourceID: &sourceID},
			},
		},
		channels: map[string]*models.NotificationChannel{
			"ch-webhook": {ID: "ch-webhook", Name: "webhook", Type: "webhook", Config: json.RawMessage(`{"url":"http://example.com"}`)},
			"ch-slack":   {ID: "ch-slack", Name: "slack", Type: "slack", Config: json.RawMessage(`{"webhook_url":"http://slack.example.com"}`)},
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
		subscriptions: map[string][]models.Subscription{},
		channels:      map[string]*models.NotificationChannel{},
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
		subscriptions: map[string][]models.Subscription{
			sourceID: {{ID: "sub-1", ChannelID: "ch-1", Type: "source", SourceID: &sourceID}},
		},
		channels: map[string]*models.NotificationChannel{
			"ch-1": {ID: "ch-1", Name: "pagerduty", Type: "pagerduty", Config: json.RawMessage(`{}`)},
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
		subscriptions: map[string][]models.Subscription{
			sourceID: {{ID: "sub-1", ChannelID: "ch-1", Type: "source", SourceID: &sourceID}},
		},
		channels: map[string]*models.NotificationChannel{
			"ch-1": {ID: "ch-1", Name: "test", Type: "webhook", Config: json.RawMessage(`{"url":"http://example.com"}`)},
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
