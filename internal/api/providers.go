package api

import "net/http"

// ProvidersHandler implements HTTP handlers for the /providers resource.
// Returns static metadata about supported ingestion providers — no store needed.
type ProvidersHandler struct{}

// NewProvidersHandler returns a new ProvidersHandler.
func NewProvidersHandler() *ProvidersHandler {
	return &ProvidersHandler{}
}

// List handles GET /providers — returns the list of supported ingestion providers.
func (h *ProvidersHandler) List(w http.ResponseWriter, r *http.Request) {
	providers := []map[string]string{
		{"id": "dockerhub", "name": "Docker Hub", "type": "polling"},
		{"id": "github", "name": "GitHub", "type": "webhook"},
		{"id": "ecr-public", "name": "AWS ECR Public", "type": "polling"},
	}
	RespondJSON(w, r, http.StatusOK, providers)
}
