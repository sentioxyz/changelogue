package api

import (
	"context"
	"net/http"

	"github.com/sentioxyz/changelogue/internal/models"
)

// TodosStore defines the data access interface for TODO operations.
type TodosStore interface {
	ListTodos(ctx context.Context, status string, page, perPage int, aggregated bool) ([]models.Todo, int, error)
	GetTodo(ctx context.Context, id string) (*models.Todo, error)
	AcknowledgeTodo(ctx context.Context, id string) error
	ResolveTodo(ctx context.Context, id string) error
}

// TodosHandler handles HTTP requests for release TODOs.
type TodosHandler struct {
	store     TodosStore
	publicURL string
}

// NewTodosHandler creates a new TodosHandler.
func NewTodosHandler(store TodosStore, publicURL string) *TodosHandler {
	return &TodosHandler{store: store, publicURL: publicURL}
}

// List returns a paginated list of TODOs, optionally filtered by status.
func (h *TodosHandler) List(w http.ResponseWriter, r *http.Request) {
	page, perPage := ParsePagination(r)
	status := r.URL.Query().Get("status")
	aggregated := r.URL.Query().Get("aggregated") == "true"

	todos, total, err := h.store.ListTodos(r.Context(), status, page, perPage, aggregated)
	if err != nil {
		RespondError(w, r, http.StatusInternalServerError, "internal_error", err.Error())
		return
	}
	if todos == nil {
		todos = []models.Todo{}
	}
	RespondList(w, r, http.StatusOK, todos, page, perPage, total)
}

// Get returns a single TODO by ID.
func (h *TodosHandler) Get(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	todo, err := h.store.GetTodo(r.Context(), id)
	if err != nil {
		RespondError(w, r, http.StatusNotFound, "not_found", "Todo not found")
		return
	}
	RespondJSON(w, r, http.StatusOK, todo)
}

// Acknowledge marks a TODO as acknowledged. Supports both PATCH (API) and GET with redirect (notification links).
func (h *TodosHandler) Acknowledge(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if err := h.store.AcknowledgeTodo(r.Context(), id); err != nil {
		RespondError(w, r, http.StatusNotFound, "not_found", "Todo not found")
		return
	}

	// If redirect=true, send 302 to the frontend todo page.
	if r.URL.Query().Get("redirect") == "true" && h.publicURL != "" {
		http.Redirect(w, r, h.publicURL+"/todo", http.StatusFound)
		return
	}

	RespondJSON(w, r, http.StatusOK, map[string]string{"status": "acknowledged"})
}

// Resolve marks a TODO as resolved. Supports both PATCH (API) and GET with redirect (notification links).
func (h *TodosHandler) Resolve(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if err := h.store.ResolveTodo(r.Context(), id); err != nil {
		RespondError(w, r, http.StatusNotFound, "not_found", "Todo not found")
		return
	}

	// If redirect=true, send 302 to the frontend todo page.
	if r.URL.Query().Get("redirect") == "true" && h.publicURL != "" {
		http.Redirect(w, r, h.publicURL+"/todo", http.StatusFound)
		return
	}

	RespondJSON(w, r, http.StatusOK, map[string]string{"status": "resolved"})
}
