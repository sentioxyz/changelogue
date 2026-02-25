package api

import (
	"context"
	"net/http"
	"strings"

	"github.com/sentioxyz/releaseguard/internal/models"
)

// SubscriptionsStore defines the persistence operations for subscriptions.
type SubscriptionsStore interface {
	ListSubscriptions(ctx context.Context, page, perPage int) ([]models.Subscription, int, error)
	CreateSubscription(ctx context.Context, sub *models.Subscription) error
	GetSubscription(ctx context.Context, id string) (*models.Subscription, error)
	UpdateSubscription(ctx context.Context, id string, sub *models.Subscription) error
	DeleteSubscription(ctx context.Context, id string) error
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
	sub.Type = strings.TrimSpace(sub.Type)
	sub.ChannelID = strings.TrimSpace(sub.ChannelID)
	// Validate type is source or project.
	if sub.Type != "source" && sub.Type != "project" {
		RespondError(w, r, http.StatusUnprocessableEntity, "validation_error", "type must be 'source' or 'project'")
		return
	}
	if sub.ChannelID == "" {
		RespondError(w, r, http.StatusUnprocessableEntity, "validation_error", "channel_id is required")
		return
	}
	// Validate that the corresponding ID is set for the type.
	if sub.Type == "source" && (sub.SourceID == nil || *sub.SourceID == "") {
		RespondError(w, r, http.StatusUnprocessableEntity, "validation_error", "source_id is required when type is 'source'")
		return
	}
	if sub.Type == "project" && (sub.ProjectID == nil || *sub.ProjectID == "") {
		RespondError(w, r, http.StatusUnprocessableEntity, "validation_error", "project_id is required when type is 'project'")
		return
	}
	if err := h.store.CreateSubscription(r.Context(), &sub); err != nil {
		RespondError(w, r, http.StatusInternalServerError, "internal_error", "Failed to create subscription")
		return
	}
	RespondJSON(w, r, http.StatusCreated, sub)
}

// Get handles GET /subscriptions/{id} — returns a single subscription.
func (h *SubscriptionsHandler) Get(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
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
	id := r.PathValue("id")
	if id == "" {
		RespondError(w, r, http.StatusBadRequest, "bad_request", "Invalid subscription ID")
		return
	}
	var sub models.Subscription
	if err := DecodeJSON(r, &sub); err != nil {
		RespondError(w, r, http.StatusBadRequest, "bad_request", "Invalid JSON body")
		return
	}
	sub.Type = strings.TrimSpace(sub.Type)
	sub.ChannelID = strings.TrimSpace(sub.ChannelID)
	if sub.Type != "source" && sub.Type != "project" {
		RespondError(w, r, http.StatusUnprocessableEntity, "validation_error", "type must be 'source' or 'project'")
		return
	}
	if sub.ChannelID == "" {
		RespondError(w, r, http.StatusUnprocessableEntity, "validation_error", "channel_id is required")
		return
	}
	if sub.Type == "source" && (sub.SourceID == nil || *sub.SourceID == "") {
		RespondError(w, r, http.StatusUnprocessableEntity, "validation_error", "source_id is required when type is 'source'")
		return
	}
	if sub.Type == "project" && (sub.ProjectID == nil || *sub.ProjectID == "") {
		RespondError(w, r, http.StatusUnprocessableEntity, "validation_error", "project_id is required when type is 'project'")
		return
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
	id := r.PathValue("id")
	if id == "" {
		RespondError(w, r, http.StatusBadRequest, "bad_request", "Invalid subscription ID")
		return
	}
	if err := h.store.DeleteSubscription(r.Context(), id); err != nil {
		RespondError(w, r, http.StatusNotFound, "not_found", "Subscription not found")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
