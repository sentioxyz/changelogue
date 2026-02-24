// internal/pipeline/subscription_router.go
package pipeline

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/sentioxyz/releaseguard/internal/models"
)

// SubscriptionRouterResult is the output of the Subscription Router node.
type SubscriptionRouterResult struct {
	Routed bool `json:"routed"`
}

// SubscriptionRouter checks if any subscriptions exist for the release's repository.
// Always-on node — drops events with no subscribers via ErrEventDropped.
type SubscriptionRouter struct {
	checker SubscriptionChecker
}

func NewSubscriptionRouter(checker SubscriptionChecker) *SubscriptionRouter {
	return &SubscriptionRouter{checker: checker}
}

func (n *SubscriptionRouter) Name() string { return "subscription_router" }

func (n *SubscriptionRouter) Execute(ctx context.Context, event *models.ReleaseEvent, _ json.RawMessage, _ map[string]json.RawMessage) (json.RawMessage, error) {
	hasSubscribers, err := n.checker.HasSubscribers(ctx, event.Repository)
	if err != nil {
		return nil, fmt.Errorf("check subscribers: %w", err)
	}

	if !hasSubscribers {
		return nil, ErrEventDropped
	}

	return json.Marshal(SubscriptionRouterResult{Routed: true})
}

var _ PipelineNode = (*SubscriptionRouter)(nil)
