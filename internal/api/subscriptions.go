package api

import (
	"context"
	"net/http"
	"strconv"
	"strings"

	"github.com/sentioxyz/releaseguard/internal/models"
)

// SubscriptionsStore defines the persistence operations for subscriptions.
type SubscriptionsStore interface {
	ListSubscriptions(ctx context.Context, page, perPage int) ([]models.Subscription, int, error)
	CreateSubscription(ctx context.Context, sub *models.Subscription) error
	GetSubscription(ctx context.Context, id int) (*models.Subscription, error)
	UpdateSubscription(ctx context.Context, id int, sub *models.Subscription) error
	DeleteSubscription(ctx context.Context, id int) error
}

// SubscriptionsHandler implements HTTP handlers for the /subscriptions resource.
type SubscriptionsHandler struct {
	store SubscriptionsStore
}

// NewSubscriptionsHandler returns a new SubscriptionsHandler.
func NewSubscriptionsHandler(store SubscriptionsStore) *SubscriptionsHandler {
	return &SubscriptionsHandler{store: store}
}

// List handles GET /subscriptions — returns a paginated list of subscriptions.
func (h *SubscriptionsHandler) List(w http.ResponseWriter, r *http.Request) {
	page, perPage := ParsePagination(r)
	subs, total, err := h.store.ListSubscriptions(r.Context(), page, perPage)
	if err != nil {
		RespondError(w, r, http.StatusInternalServerError, "internal_error", "Failed to list subscriptions")
		return
	}
	if subs == nil {
		subs = []models.Subscription{}
	}
	RespondList(w, r, http.StatusOK, subs, page, perPage, total)
}

// Create handles POST /subscriptions — creates a new subscription.
func (h *SubscriptionsHandler) Create(w http.ResponseWriter, r *http.Request) {
	var sub models.Subscription
	if err := DecodeJSON(r, &sub); err != nil {
		RespondError(w, r, http.StatusBadRequest, "bad_request", "Invalid JSON body")
		return
	}
	sub.ChannelType = strings.TrimSpace(sub.ChannelType)
	if sub.ProjectID == 0 {
		RespondError(w, r, http.StatusUnprocessableEntity, "validation_error", "project_id is required")
		return
	}
	if sub.ChannelType == "" {
		RespondError(w, r, http.StatusUnprocessableEntity, "validation_error", "channel_type is required")
		return
	}
	// Default frequency to "instant" if not provided.
	if strings.TrimSpace(sub.Frequency) == "" {
		sub.Frequency = "instant"
	}
	if err := h.store.CreateSubscription(r.Context(), &sub); err != nil {
		RespondError(w, r, http.StatusInternalServerError, "internal_error", "Failed to create subscription")
		return
	}
	RespondJSON(w, r, http.StatusCreated, sub)
}

// Get handles GET /subscriptions/{id} — returns a single subscription.
func (h *SubscriptionsHandler) Get(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(r.PathValue("id"))
	if err != nil {
		RespondError(w, r, http.StatusBadRequest, "bad_request", "Invalid subscription ID")
		return
	}
	sub, err := h.store.GetSubscription(r.Context(), id)
	if err != nil {
		RespondError(w, r, http.StatusNotFound, "not_found", "Subscription not found")
		return
	}
	RespondJSON(w, r, http.StatusOK, sub)
}

// Update handles PUT /subscriptions/{id} — updates an existing subscription.
func (h *SubscriptionsHandler) Update(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(r.PathValue("id"))
	if err != nil {
		RespondError(w, r, http.StatusBadRequest, "bad_request", "Invalid subscription ID")
		return
	}
	var sub models.Subscription
	if err := DecodeJSON(r, &sub); err != nil {
		RespondError(w, r, http.StatusBadRequest, "bad_request", "Invalid JSON body")
		return
	}
	sub.ChannelType = strings.TrimSpace(sub.ChannelType)
	if sub.ChannelType == "" {
		RespondError(w, r, http.StatusUnprocessableEntity, "validation_error", "channel_type is required")
		return
	}
	if strings.TrimSpace(sub.Frequency) == "" {
		sub.Frequency = "instant"
	}
	if err := h.store.UpdateSubscription(r.Context(), id, &sub); err != nil {
		RespondError(w, r, http.StatusNotFound, "not_found", "Subscription not found")
		return
	}
	sub.ID = id
	RespondJSON(w, r, http.StatusOK, sub)
}

// Delete handles DELETE /subscriptions/{id} — deletes a subscription.
func (h *SubscriptionsHandler) Delete(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(r.PathValue("id"))
	if err != nil {
		RespondError(w, r, http.StatusBadRequest, "bad_request", "Invalid subscription ID")
		return
	}
	if err := h.store.DeleteSubscription(r.Context(), id); err != nil {
		RespondError(w, r, http.StatusNotFound, "not_found", "Subscription not found")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
