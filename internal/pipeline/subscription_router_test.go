// internal/pipeline/subscription_router_test.go
package pipeline

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/sentioxyz/releaseguard/internal/models"
)

type mockSubscriptionChecker struct {
	hasSubscribers bool
	err            error
}

func (m *mockSubscriptionChecker) HasSubscribers(_ context.Context, _ string) (bool, error) {
	return m.hasSubscribers, m.err
}

func TestSubscriptionRouterName(t *testing.T) {
	n := NewSubscriptionRouter(&mockSubscriptionChecker{})
	if got := n.Name(); got != "subscription_router" {
		t.Errorf("Name() = %q, want %q", got, "subscription_router")
	}
}

func TestSubscriptionRouterWithSubscribers(t *testing.T) {
	checker := &mockSubscriptionChecker{hasSubscribers: true}
	n := NewSubscriptionRouter(checker)

	event := &models.ReleaseEvent{Repository: "library/golang", Timestamp: time.Now()}

	result, err := n.Execute(context.Background(), event, nil, nil)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}

	var res SubscriptionRouterResult
	if err := json.Unmarshal(result, &res); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if !res.Routed {
		t.Error("expected Routed=true")
	}
}

func TestSubscriptionRouterNoSubscribers(t *testing.T) {
	checker := &mockSubscriptionChecker{hasSubscribers: false}
	n := NewSubscriptionRouter(checker)

	event := &models.ReleaseEvent{Repository: "library/golang", Timestamp: time.Now()}

	_, err := n.Execute(context.Background(), event, nil, nil)
	if !errors.Is(err, ErrEventDropped) {
		t.Errorf("expected ErrEventDropped, got: %v", err)
	}
}

func TestSubscriptionRouterCheckerError(t *testing.T) {
	checker := &mockSubscriptionChecker{err: errors.New("db error")}
	n := NewSubscriptionRouter(checker)

	event := &models.ReleaseEvent{Repository: "library/golang", Timestamp: time.Now()}

	_, err := n.Execute(context.Background(), event, nil, nil)
	if err == nil {
		t.Fatal("expected error")
	}
	if errors.Is(err, ErrEventDropped) {
		t.Error("db error should not be ErrEventDropped")
	}
}
