package api

import (
	"net/http"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/sentioxyz/changelogue/internal/auth"
	"github.com/sentioxyz/changelogue/internal/ingestion"
	"github.com/sentioxyz/changelogue/internal/routing"
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
	TodosStore            TodosStore
	OnboardStore          OnboardStore
	GatesStore            GatesStore
	PublicURL             string
	KeyStore              KeyStore
	SessionValidator      SessionValidator
	HealthChecker         HealthChecker
	Broadcaster           *Broadcaster
	NoAuth                bool
	IngestionService      *ingestion.Service
	HTTPClient            *http.Client
}

// RegisterRoutes registers all API v1 routes on the given ServeMux.
func RegisterRoutes(mux *http.ServeMux, deps Dependencies) {
	// Public chain: no auth required.
	publicChain := Chain(RequestID, Logger, Recovery)

	// Authenticated chain: includes auth unless NoAuth is set.
	chain := publicChain
	if !deps.NoAuth {
		chain = Chain(RequestID, Logger, Recovery, RateLimit(10, 20), Auth(deps.KeyStore, deps.SessionValidator))
	}

	// Projects (CRUD)
	projects := NewProjectsHandler(deps.ProjectsStore)
	mux.Handle("GET /api/v1/projects", chain(http.HandlerFunc(projects.List)))
	mux.Handle("POST /api/v1/projects", chain(http.HandlerFunc(projects.Create)))
	mux.Handle("GET /api/v1/projects/{id}", chain(http.HandlerFunc(projects.Get)))
	mux.Handle("PUT /api/v1/projects/{id}", chain(http.HandlerFunc(projects.Update)))
	mux.Handle("DELETE /api/v1/projects/{id}", chain(http.HandlerFunc(projects.Delete)))

	// Sources (nested under projects)
	sources := NewSourcesHandler(deps.SourcesStore, deps.IngestionService, deps.HTTPClient)
	mux.Handle("GET /api/v1/projects/{projectId}/sources", chain(http.HandlerFunc(sources.List)))
	mux.Handle("POST /api/v1/projects/{projectId}/sources", chain(http.HandlerFunc(sources.Create)))
	mux.Handle("GET /api/v1/sources/{id}", chain(http.HandlerFunc(sources.Get)))
	mux.Handle("PUT /api/v1/sources/{id}", chain(http.HandlerFunc(sources.Update)))
	mux.Handle("DELETE /api/v1/sources/{id}", chain(http.HandlerFunc(sources.Delete)))
	mux.Handle("POST /api/v1/sources/{id}/poll", chain(http.HandlerFunc(sources.FetchReleases)))

	// Context Sources (nested under projects)
	contextSources := NewContextSourcesHandler(deps.ContextSourcesStore)
	mux.Handle("GET /api/v1/projects/{projectId}/context-sources", chain(http.HandlerFunc(contextSources.List)))
	mux.Handle("POST /api/v1/projects/{projectId}/context-sources", chain(http.HandlerFunc(contextSources.Create)))
	mux.Handle("GET /api/v1/context-sources/{id}", chain(http.HandlerFunc(contextSources.Get)))
	mux.Handle("PUT /api/v1/context-sources/{id}", chain(http.HandlerFunc(contextSources.Update)))
	mux.Handle("DELETE /api/v1/context-sources/{id}", chain(http.HandlerFunc(contextSources.Delete)))

	// Releases (read-only, nested under sources and projects)
	releases := NewReleasesHandler(deps.ReleasesStore)
	mux.Handle("GET /api/v1/releases", chain(http.HandlerFunc(releases.List)))
	mux.Handle("GET /api/v1/sources/{id}/releases", chain(http.HandlerFunc(releases.ListBySource)))
	mux.Handle("GET /api/v1/projects/{projectId}/releases", chain(http.HandlerFunc(releases.ListByProject)))
	mux.Handle("GET /api/v1/releases/{id}", chain(http.HandlerFunc(releases.Get)))

	// Semantic Releases (nested under projects + top-level list)
	semanticReleases := NewSemanticReleasesHandler(deps.SemanticReleasesStore)
	mux.Handle("GET /api/v1/semantic-releases", chain(http.HandlerFunc(semanticReleases.ListAll)))
	mux.Handle("GET /api/v1/projects/{projectId}/semantic-releases", chain(http.HandlerFunc(semanticReleases.List)))
	mux.Handle("GET /api/v1/semantic-releases/{id}", chain(http.HandlerFunc(semanticReleases.Get)))
	mux.Handle("GET /api/v1/semantic-releases/{id}/sources", chain(http.HandlerFunc(semanticReleases.ListSources)))
	mux.Handle("DELETE /api/v1/semantic-releases/{id}", chain(http.HandlerFunc(semanticReleases.Delete)))

	// Subscriptions (CRUD)
	subscriptions := NewSubscriptionsHandler(deps.SubscriptionsStore)
	mux.Handle("GET /api/v1/subscriptions", chain(http.HandlerFunc(subscriptions.List)))
	mux.Handle("POST /api/v1/subscriptions", chain(http.HandlerFunc(subscriptions.Create)))
	mux.Handle("POST /api/v1/subscriptions/batch", chain(http.HandlerFunc(subscriptions.BatchCreate)))
	mux.Handle("DELETE /api/v1/subscriptions/batch", chain(http.HandlerFunc(subscriptions.BatchDelete)))
	mux.Handle("GET /api/v1/subscriptions/{id}", chain(http.HandlerFunc(subscriptions.Get)))
	mux.Handle("PUT /api/v1/subscriptions/{id}", chain(http.HandlerFunc(subscriptions.Update)))
	mux.Handle("DELETE /api/v1/subscriptions/{id}", chain(http.HandlerFunc(subscriptions.Delete)))

	// Channels (CRUD + test)
	channels := NewChannelsHandler(deps.ChannelsStore, routing.NewSenders(), deps.PublicURL)
	mux.Handle("GET /api/v1/channels", chain(http.HandlerFunc(channels.List)))
	mux.Handle("POST /api/v1/channels", chain(http.HandlerFunc(channels.Create)))
	mux.Handle("GET /api/v1/channels/{id}", chain(http.HandlerFunc(channels.Get)))
	mux.Handle("PUT /api/v1/channels/{id}", chain(http.HandlerFunc(channels.Update)))
	mux.Handle("DELETE /api/v1/channels/{id}", chain(http.HandlerFunc(channels.Delete)))
	mux.Handle("POST /api/v1/channels/{id}/test", chain(http.HandlerFunc(channels.Test)))

	// Agent
	agent := NewAgentHandler(deps.AgentStore)
	mux.Handle("POST /api/v1/projects/{projectId}/agent/run", chain(http.HandlerFunc(agent.TriggerRun)))
	mux.Handle("GET /api/v1/projects/{projectId}/agent/runs", chain(http.HandlerFunc(agent.ListRuns)))
	mux.Handle("GET /api/v1/agent-runs/{id}", chain(http.HandlerFunc(agent.GetRun)))

	// Todos
	todos := NewTodosHandler(deps.TodosStore, deps.PublicURL)
	mux.Handle("GET /api/v1/todos", chain(http.HandlerFunc(todos.List)))
	mux.Handle("GET /api/v1/todos/{id}", chain(http.HandlerFunc(todos.Get)))
	mux.Handle("PATCH /api/v1/todos/{id}/acknowledge", chain(http.HandlerFunc(todos.Acknowledge)))
	mux.Handle("PATCH /api/v1/todos/{id}/resolve", chain(http.HandlerFunc(todos.Resolve)))
	mux.Handle("PATCH /api/v1/todos/{id}/reopen", chain(http.HandlerFunc(todos.Reopen)))
	// One-click endpoints for notification links (GET so they work as <a href>)
	mux.Handle("GET /api/v1/todos/{id}/acknowledge", chain(http.HandlerFunc(todos.Acknowledge)))
	mux.Handle("GET /api/v1/todos/{id}/resolve", chain(http.HandlerFunc(todos.Resolve)))

	// Onboard (repo dependency scanning)
	ob := NewOnboardHandler(deps.OnboardStore)
	mux.Handle("POST /api/v1/onboard/scan", chain(http.HandlerFunc(ob.Scan)))
	mux.Handle("GET /api/v1/onboard/scans/{id}", chain(http.HandlerFunc(ob.GetScan)))
	mux.Handle("POST /api/v1/onboard/scans/{id}/apply", chain(http.HandlerFunc(ob.Apply)))

	// Release Gates
	gates := NewGatesHandler(deps.GatesStore)
	mux.Handle("GET /api/v1/projects/{id}/release-gate", chain(http.HandlerFunc(gates.GetGate)))
	mux.Handle("PUT /api/v1/projects/{id}/release-gate", chain(http.HandlerFunc(gates.UpsertGate)))
	mux.Handle("DELETE /api/v1/projects/{id}/release-gate", chain(http.HandlerFunc(gates.DeleteGate)))
	mux.Handle("GET /api/v1/projects/{id}/version-readiness", chain(http.HandlerFunc(gates.ListReadiness)))
	mux.Handle("GET /api/v1/projects/{id}/version-readiness/{version}", chain(http.HandlerFunc(gates.GetReadiness)))
	mux.Handle("GET /api/v1/projects/{id}/version-readiness/{version}/events", chain(http.HandlerFunc(gates.ListEventsByVersion)))
	mux.Handle("GET /api/v1/projects/{id}/gate-events", chain(http.HandlerFunc(gates.ListEvents)))

	// Providers (metadata — static, no store needed)
	providers := NewProvidersHandler()
	mux.Handle("GET /api/v1/providers", chain(http.HandlerFunc(providers.List)))

	// Discovery (public — no auth, proxies external APIs)
	discover := NewDiscoverHandler(deps.HTTPClient, "", "")
	mux.Handle("GET /api/v1/discover/github", publicChain(http.HandlerFunc(discover.GitHub)))
	mux.Handle("GET /api/v1/discover/dockerhub", publicChain(http.HandlerFunc(discover.DockerHub)))

	// Personalized suggestions (auth required — needs github_login from session)
	suggestions := NewSuggestionsHandler(deps.HTTPClient, deps.SourcesStore, "", "")
	mux.Handle("GET /api/v1/suggestions/stars", chain(http.HandlerFunc(suggestions.Stars)))
	mux.Handle("GET /api/v1/suggestions/repos", chain(http.HandlerFunc(suggestions.Repos)))

	// SSE events
	events := NewEventsHandler(deps.Broadcaster)
	mux.Handle("GET /api/v1/events", chain(http.HandlerFunc(events.Stream)))

	// Health (public — no auth middleware)
	health := NewHealthHandler(deps.HealthChecker)
	mux.Handle("GET /api/v1/health", publicChain(http.HandlerFunc(health.Check)))
	mux.Handle("GET /api/v1/stats", chain(http.HandlerFunc(health.Stats)))
	mux.Handle("GET /api/v1/stats/trend", chain(http.HandlerFunc(health.Trend)))
}

// RegisterAuthRoutes registers OAuth and session endpoints on the mux.
// frontendURL is used in noAuth mode to redirect back to the frontend (may be empty).
func RegisterAuthRoutes(mux *http.ServeMux, oauthHandler *auth.OAuthHandler, noAuth bool, frontendURL string) {
	if noAuth {
		mux.HandleFunc("GET /auth/me", func(w http.ResponseWriter, r *http.Request) {
			if c, err := r.Cookie("dev_logged_out"); err == nil && c.Value == "1" {
				http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
				return
			}
			auth.HandleMeDev(w, r)
		})
		mux.HandleFunc("POST /auth/logout", func(w http.ResponseWriter, r *http.Request) {
			http.SetCookie(w, &http.Cookie{
				Name:  "dev_logged_out",
				Value: "1",
				Path:  "/",
			})
			w.WriteHeader(http.StatusOK)
		})
		mux.HandleFunc("GET /auth/github", func(w http.ResponseWriter, r *http.Request) {
			// Clear the logout cookie so dev user is "logged in" again
			http.SetCookie(w, &http.Cookie{
				Name:   "dev_logged_out",
				Value:  "",
				Path:   "/",
				MaxAge: -1,
			})
			target := "/"
			if frontendURL != "" {
				target = strings.TrimRight(frontendURL, "/") + "/"
			}
			http.Redirect(w, r, target, http.StatusFound)
		})
		return
	}

	sessionMw := oauthHandler.RequireSession
	mux.HandleFunc("GET /auth/github", oauthHandler.HandleGitHubRedirect)
	mux.HandleFunc("GET /auth/github/callback", oauthHandler.HandleGitHubCallback)
	mux.Handle("GET /auth/me", sessionMw(http.HandlerFunc(oauthHandler.HandleMe)))
	mux.HandleFunc("POST /auth/logout", oauthHandler.HandleLogout)
}
