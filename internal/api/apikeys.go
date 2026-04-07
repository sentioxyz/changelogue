package api

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"strings"

	"github.com/sentioxyz/changelogue/internal/models"
)

// ApiKeysStore defines the persistence operations for API keys.
type ApiKeysStore interface {
	ListApiKeys(ctx context.Context, page, perPage int) ([]models.ApiKey, int, error)
	CreateApiKey(ctx context.Context, key *models.ApiKey) error
	DeleteApiKey(ctx context.Context, id string) error
}

// ApiKeysHandler implements HTTP handlers for the /api-keys resource.
type ApiKeysHandler struct {
	store ApiKeysStore
}

// NewApiKeysHandler returns a new ApiKeysHandler.
func NewApiKeysHandler(store ApiKeysStore) *ApiKeysHandler {
	return &ApiKeysHandler{store: store}
}

// List handles GET /api-keys — returns a paginated list of API keys.
func (h *ApiKeysHandler) List(w http.ResponseWriter, r *http.Request) {
	page, perPage := ParsePagination(r)
	keys, total, err := h.store.ListApiKeys(r.Context(), page, perPage)
	if err != nil {
		RespondError(w, r, http.StatusInternalServerError, "internal_error", "Failed to list API keys")
		return
	}
	if keys == nil {
		keys = []models.ApiKey{}
	}
	RespondList(w, r, http.StatusOK, keys, page, perPage, total)
}

// Create handles POST /api-keys — creates a new API key and returns the raw key once.
func (h *ApiKeysHandler) Create(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Name string `json:"name"`
	}
	if err := DecodeJSON(r, &input); err != nil {
		RespondError(w, r, http.StatusBadRequest, "bad_request", "Invalid JSON body")
		return
	}
	input.Name = strings.TrimSpace(input.Name)
	if input.Name == "" {
		RespondError(w, r, http.StatusUnprocessableEntity, "validation_error", "name is required")
		return
	}

	// Generate raw key: cl_ prefix + 32 random hex bytes.
	rawBytes := make([]byte, 32)
	if _, err := rand.Read(rawBytes); err != nil {
		RespondError(w, r, http.StatusInternalServerError, "internal_error", "Failed to generate key")
		return
	}
	rawKey := fmt.Sprintf("cl_%s", hex.EncodeToString(rawBytes))

	hash := sha256.Sum256([]byte(rawKey))
	keyHash := hex.EncodeToString(hash[:])
	keyPrefix := rawKey[:12]

	key := models.ApiKey{
		Name:      input.Name,
		KeyPrefix: keyPrefix,
		Key:       rawKey,
	}

	if err := h.store.CreateApiKey(r.Context(), &models.ApiKey{
		Name:      input.Name,
		KeyPrefix: keyPrefix,
		Key:       keyHash, // store passes this as the hash
	}); err != nil {
		RespondError(w, r, http.StatusInternalServerError, "internal_error", "Failed to create API key")
		return
	}

	RespondJSON(w, r, http.StatusCreated, key)
}

// Delete handles DELETE /api-keys/{id} — revokes an API key.
func (h *ApiKeysHandler) Delete(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		RespondError(w, r, http.StatusBadRequest, "bad_request", "Invalid API key ID")
		return
	}
	if err := h.store.DeleteApiKey(r.Context(), id); err != nil {
		RespondError(w, r, http.StatusInternalServerError, "internal_error", "Failed to delete API key")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
