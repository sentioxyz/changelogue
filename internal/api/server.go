package api

import (
	"net/http"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Dependencies holds all external dependencies required for registering API routes.
type Dependencies struct {
	DB                    *pgxpool.Pool
	ProjectsStore         ProjectsStore
	ReleasesStore         ReleasesStore
	SubscriptionsStore    SubscriptionsStore
	SourcesStore          SourcesStore
	ChannelsStore         ChannelsStore
	ContextSourcesStore   ContextSourcesStore
	SemanticReleasesStore SemanticReleasesStore
	AgentStore            AgentStore
	KeyStore              KeyStore
	HealthChecker         HealthChecker
	Broadcaster           *Broadcaster
	NoAuth                bool
}

// RegisterRoutes registers all API v1 routes on the given ServeMux.
func RegisterRoutes(mux *http.ServeMux, deps Dependencies) {
	// Public chain: no auth required.
	publicChain := Chain(RequestID, Logger, Recovery)

	// Authenticated chain: includes auth unless NoAuth is set.
	chain := publicChain
	if !deps.NoAuth {
		chain = Chain(RequestID, Logger, Recovery, RateLimit(10, 20), Auth(deps.KeyStore))
	}

	// Projects (CRUD)
	projects := NewProjectsHandler(deps.ProjectsStore)
	mux.Handle("GET /api/v1/projects", chain(http.HandlerFunc(projects.List)))
	mux.Handle("POST /api/v1/projects", chain(http.HandlerFunc(projects.Create)))
	mux.Handle("GET /api/v1/projects/{id}", chain(http.HandlerFunc(projects.Get)))
	mux.Handle("PUT /api/v1/projects/{id}", chain(http.HandlerFunc(projects.Update)))
	mux.Handle("DELETE /api/v1/projects/{id}", chain(http.HandlerFunc(projects.Delete)))

	// Sources (nested under projects)
	sources := NewSourcesHandler(deps.SourcesStore)
	mux.Handle("GET /api/v1/projects/{projectId}/sources", chain(http.HandlerFunc(sources.List)))
	mux.Handle("POST /api/v1/projects/{projectId}/sources", chain(http.HandlerFunc(sources.Create)))
	mux.Handle("GET /api/v1/sources/{id}", chain(http.HandlerFunc(sources.Get)))
	mux.Handle("PUT /api/v1/sources/{id}", chain(http.HandlerFunc(sources.Update)))
	mux.Handle("DELETE /api/v1/sources/{id}", chain(http.HandlerFunc(sources.Delete)))

	// Context Sources (nested under projects)
	contextSources := NewContextSourcesHandler(deps.ContextSourcesStore)
	mux.Handle("GET /api/v1/projects/{projectId}/context-sources", chain(http.HandlerFunc(contextSources.List)))
	mux.Handle("POST /api/v1/projects/{projectId}/context-sources", chain(http.HandlerFunc(contextSources.Create)))
	mux.Handle("GET /api/v1/context-sources/{id}", chain(http.HandlerFunc(contextSources.Get)))
	mux.Handle("PUT /api/v1/context-sources/{id}", chain(http.HandlerFunc(contextSources.Update)))
	mux.Handle("DELETE /api/v1/context-sources/{id}", chain(http.HandlerFunc(contextSources.Delete)))

	// Releases (read-only, nested under sources and projects)
	releases := NewReleasesHandler(deps.ReleasesStore)
	mux.Handle("GET /api/v1/sources/{id}/releases", chain(http.HandlerFunc(releases.ListBySource)))
	mux.Handle("GET /api/v1/projects/{projectId}/releases", chain(http.HandlerFunc(releases.ListByProject)))
	mux.Handle("GET /api/v1/releases/{id}", chain(http.HandlerFunc(releases.Get)))

	// Semantic Releases (read-only, nested under projects)
	semanticReleases := NewSemanticReleasesHandler(deps.SemanticReleasesStore)
	mux.Handle("GET /api/v1/projects/{projectId}/semantic-releases", chain(http.HandlerFunc(semanticReleases.List)))
	mux.Handle("GET /api/v1/semantic-releases/{id}", chain(http.HandlerFunc(semanticReleases.Get)))

	// Subscriptions (CRUD)
	subscriptions := NewSubscriptionsHandler(deps.SubscriptionsStore)
	mux.Handle("GET /api/v1/subscriptions", chain(http.HandlerFunc(subscriptions.List)))
	mux.Handle("POST /api/v1/subscriptions", chain(http.HandlerFunc(subscriptions.Create)))
	mux.Handle("GET /api/v1/subscriptions/{id}", chain(http.HandlerFunc(subscriptions.Get)))
	mux.Handle("PUT /api/v1/subscriptions/{id}", chain(http.HandlerFunc(subscriptions.Update)))
	mux.Handle("DELETE /api/v1/subscriptions/{id}", chain(http.HandlerFunc(subscriptions.Delete)))

	// Channels (CRUD)
	channels := NewChannelsHandler(deps.ChannelsStore)
	mux.Handle("GET /api/v1/channels", chain(http.HandlerFunc(channels.List)))
	mux.Handle("POST /api/v1/channels", chain(http.HandlerFunc(channels.Create)))
	mux.Handle("GET /api/v1/channels/{id}", chain(http.HandlerFunc(channels.Get)))
	mux.Handle("PUT /api/v1/channels/{id}", chain(http.HandlerFunc(channels.Update)))
	mux.Handle("DELETE /api/v1/channels/{id}", chain(http.HandlerFunc(channels.Delete)))

	// Agent
	agent := NewAgentHandler(deps.AgentStore)
	mux.Handle("POST /api/v1/projects/{projectId}/agent/run", chain(http.HandlerFunc(agent.TriggerRun)))
	mux.Handle("GET /api/v1/projects/{projectId}/agent/runs", chain(http.HandlerFunc(agent.ListRuns)))
	mux.Handle("GET /api/v1/agent-runs/{id}", chain(http.HandlerFunc(agent.GetRun)))

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
