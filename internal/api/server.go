package api

import (
	"net/http"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Dependencies holds all external dependencies required for registering API routes.
type Dependencies struct {
	DB                 *pgxpool.Pool
	ProjectsStore      ProjectsStore
	ReleasesStore      ReleasesStore
	SubscriptionsStore SubscriptionsStore
	SourcesStore       SourcesStore
	ChannelsStore      ChannelsStore
	KeyStore           KeyStore
	HealthChecker      HealthChecker
	Broadcaster        *Broadcaster
}

// RegisterRoutes registers all API v1 routes on the given ServeMux.
func RegisterRoutes(mux *http.ServeMux, deps Dependencies) {
	// Authenticated chain: all middleware including auth and rate limiting.
	chain := Chain(RequestID, Logger, Recovery, RateLimit(10, 20), Auth(deps.KeyStore), CORS)
	// Public chain: no auth required.
	publicChain := Chain(RequestID, Logger, Recovery, CORS)

	// Projects (CRUD)
	projects := NewProjectsHandler(deps.ProjectsStore)
	mux.Handle("GET /api/v1/projects", chain(http.HandlerFunc(projects.List)))
	mux.Handle("POST /api/v1/projects", chain(http.HandlerFunc(projects.Create)))
	mux.Handle("GET /api/v1/projects/{id}", chain(http.HandlerFunc(projects.Get)))
	mux.Handle("PUT /api/v1/projects/{id}", chain(http.HandlerFunc(projects.Update)))
	mux.Handle("DELETE /api/v1/projects/{id}", chain(http.HandlerFunc(projects.Delete)))

	// Releases (read-only)
	releases := NewReleasesHandler(deps.ReleasesStore)
	mux.Handle("GET /api/v1/releases", chain(http.HandlerFunc(releases.List)))
	mux.Handle("GET /api/v1/releases/{id}", chain(http.HandlerFunc(releases.Get)))
	mux.Handle("GET /api/v1/releases/{id}/pipeline", chain(http.HandlerFunc(releases.Pipeline)))
	mux.Handle("GET /api/v1/releases/{id}/notes", chain(http.HandlerFunc(releases.Notes)))

	// Subscriptions (CRUD)
	subscriptions := NewSubscriptionsHandler(deps.SubscriptionsStore)
	mux.Handle("GET /api/v1/subscriptions", chain(http.HandlerFunc(subscriptions.List)))
	mux.Handle("POST /api/v1/subscriptions", chain(http.HandlerFunc(subscriptions.Create)))
	mux.Handle("GET /api/v1/subscriptions/{id}", chain(http.HandlerFunc(subscriptions.Get)))
	mux.Handle("PUT /api/v1/subscriptions/{id}", chain(http.HandlerFunc(subscriptions.Update)))
	mux.Handle("DELETE /api/v1/subscriptions/{id}", chain(http.HandlerFunc(subscriptions.Delete)))

	// Sources (CRUD + release lookups)
	sources := NewSourcesHandler(deps.SourcesStore)
	mux.Handle("GET /api/v1/sources", chain(http.HandlerFunc(sources.List)))
	mux.Handle("POST /api/v1/sources", chain(http.HandlerFunc(sources.Create)))
	mux.Handle("GET /api/v1/sources/{id}", chain(http.HandlerFunc(sources.Get)))
	mux.Handle("PUT /api/v1/sources/{id}", chain(http.HandlerFunc(sources.Update)))
	mux.Handle("DELETE /api/v1/sources/{id}", chain(http.HandlerFunc(sources.Delete)))
	mux.Handle("GET /api/v1/sources/{id}/latest-release", chain(http.HandlerFunc(sources.LatestRelease)))
	mux.Handle("GET /api/v1/sources/{id}/releases/{version}", chain(http.HandlerFunc(sources.ReleaseByVersion)))

	// Channels (CRUD)
	channels := NewChannelsHandler(deps.ChannelsStore)
	mux.Handle("GET /api/v1/channels", chain(http.HandlerFunc(channels.List)))
	mux.Handle("POST /api/v1/channels", chain(http.HandlerFunc(channels.Create)))
	mux.Handle("GET /api/v1/channels/{id}", chain(http.HandlerFunc(channels.Get)))
	mux.Handle("PUT /api/v1/channels/{id}", chain(http.HandlerFunc(channels.Update)))
	mux.Handle("DELETE /api/v1/channels/{id}", chain(http.HandlerFunc(channels.Delete)))

	// Providers (metadata — static, no store needed)
	providers := NewProvidersHandler()
	mux.Handle("GET /api/v1/providers", chain(http.HandlerFunc(providers.List)))

	// SSE events
	events := NewEventsHandler(deps.Broadcaster)
	mux.Handle("GET /api/v1/events", chain(http.HandlerFunc(events.Stream)))

	// Health (public — no auth middleware)
	health := NewHealthHandler(deps.HealthChecker)
	mux.Handle("GET /api/v1/health", publicChain(http.HandlerFunc(health.Check)))
	mux.Handle("GET /api/v1/stats", chain(http.HandlerFunc(health.Stats)))
}
