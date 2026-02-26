package api

import (
	"context"
	"net/http"
	"strings"

	"github.com/sentioxyz/changelogue/internal/models"
)

// ChannelsStore defines the persistence operations for notification channels.
type ChannelsStore interface {
	ListChannels(ctx context.Context, page, perPage int) ([]models.NotificationChannel, int, error)
	CreateChannel(ctx context.Context, ch *models.NotificationChannel) error
	GetChannel(ctx context.Context, id string) (*models.NotificationChannel, error)
	UpdateChannel(ctx context.Context, id string, ch *models.NotificationChannel) error
	DeleteChannel(ctx context.Context, id string) error
}

// ChannelsHandler implements HTTP handlers for the /channels resource.
type ChannelsHandler struct {
	store ChannelsStore
}

// NewChannelsHandler returns a new ChannelsHandler.
func NewChannelsHandler(store ChannelsStore) *ChannelsHandler {
	return &ChannelsHandler{store: store}
}

// List handles GET /channels — returns a paginated list of notification channels.
func (h *ChannelsHandler) List(w http.ResponseWriter, r *http.Request) {
	page, perPage := ParsePagination(r)
	channels, total, err := h.store.ListChannels(r.Context(), page, perPage)
	if err != nil {
		RespondError(w, r, http.StatusInternalServerError, "internal_error", "Failed to list channels")
		return
	}
	if channels == nil {
		channels = []models.NotificationChannel{}
	}
	RespondList(w, r, http.StatusOK, channels, page, perPage, total)
}

// Create handles POST /channels — creates a new notification channel.
func (h *ChannelsHandler) Create(w http.ResponseWriter, r *http.Request) {
	var ch models.NotificationChannel
	if err := DecodeJSON(r, &ch); err != nil {
		RespondError(w, r, http.StatusBadRequest, "bad_request", "Invalid JSON body")
		return
	}
	ch.Type = strings.TrimSpace(ch.Type)
	ch.Name = strings.TrimSpace(ch.Name)
	if ch.Type == "" {
		RespondError(w, r, http.StatusUnprocessableEntity, "validation_error", "type is required")
		return
	}
	if ch.Name == "" {
		RespondError(w, r, http.StatusUnprocessableEntity, "validation_error", "name is required")
		return
	}
	if ch.Config == nil || string(ch.Config) == "null" {
		RespondError(w, r, http.StatusUnprocessableEntity, "validation_error", "config is required")
		return
	}
	if err := h.store.CreateChannel(r.Context(), &ch); err != nil {
		RespondError(w, r, http.StatusInternalServerError, "internal_error", "Failed to create channel")
		return
	}
	RespondJSON(w, r, http.StatusCreated, ch)
}

// Get handles GET /channels/{id} — returns a single notification channel.
func (h *ChannelsHandler) Get(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		RespondError(w, r, http.StatusBadRequest, "bad_request", "Invalid channel ID")
		return
	}
	ch, err := h.store.GetChannel(r.Context(), id)
	if err != nil {
		RespondError(w, r, http.StatusNotFound, "not_found", "Channel not found")
		return
	}
	RespondJSON(w, r, http.StatusOK, ch)
}

// Update handles PUT /channels/{id} — updates an existing notification channel.
func (h *ChannelsHandler) Update(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		RespondError(w, r, http.StatusBadRequest, "bad_request", "Invalid channel ID")
		return
	}
	var ch models.NotificationChannel
	if err := DecodeJSON(r, &ch); err != nil {
		RespondError(w, r, http.StatusBadRequest, "bad_request", "Invalid JSON body")
		return
	}
	ch.Type = strings.TrimSpace(ch.Type)
	ch.Name = strings.TrimSpace(ch.Name)
	if ch.Type == "" {
		RespondError(w, r, http.StatusUnprocessableEntity, "validation_error", "type is required")
		return
	}
	if ch.Name == "" {
		RespondError(w, r, http.StatusUnprocessableEntity, "validation_error", "name is required")
		return
	}
	if ch.Config == nil || string(ch.Config) == "null" {
		RespondError(w, r, http.StatusUnprocessableEntity, "validation_error", "config is required")
		return
	}
	if err := h.store.UpdateChannel(r.Context(), id, &ch); err != nil {
		RespondError(w, r, http.StatusNotFound, "not_found", "Channel not found")
		return
	}
	ch.ID = id
	RespondJSON(w, r, http.StatusOK, ch)
}

// Delete handles DELETE /channels/{id} — deletes a notification channel.
func (h *ChannelsHandler) Delete(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		RespondError(w, r, http.StatusBadRequest, "bad_request", "Invalid channel ID")
		return
	}
	if err := h.store.DeleteChannel(r.Context(), id); err != nil {
		RespondError(w, r, http.StatusNotFound, "not_found", "Channel not found")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
