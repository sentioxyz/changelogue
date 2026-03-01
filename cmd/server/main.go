package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"time"

	"github.com/riverqueue/river"
	agentpkg "github.com/sentioxyz/changelogue/internal/agent"
	"github.com/sentioxyz/changelogue/internal/api"
	"github.com/sentioxyz/changelogue/internal/db"
	"github.com/sentioxyz/changelogue/internal/ingestion"
	"github.com/sentioxyz/changelogue/internal/queue"
	"github.com/sentioxyz/changelogue/internal/routing"
)

func main() {
	// Configure log level from LOG_LEVEL env (debug, info, warn, error).
	logLevel := new(slog.LevelVar) // defaults to Info
	switch os.Getenv("LOG_LEVEL") {
	case "debug":
		logLevel.Set(slog.LevelDebug)
	case "warn":
		logLevel.Set(slog.LevelWarn)
	case "error":
		logLevel.Set(slog.LevelError)
	}
	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: logLevel})))

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	dbURL := envOr("DATABASE_URL", "postgres://localhost:5432/changelogue?sslmode=disable")
	addr := envOr("LISTEN_ADDR", ":8080")
	noAuth := os.Getenv("NO_AUTH") == "true"

	// Database
	pool, err := db.NewPool(ctx, dbURL)
	if err != nil {
		slog.Error("database connection failed", "err", err)
		os.Exit(1)
	}
	defer pool.Close()

	if err := db.RunMigrations(ctx, pool); err != nil {
		slog.Error("migrations failed", "err", err)
		os.Exit(1)
	}

	// API & routing store (created before River so we can register workers)
	// The river client is set to nil initially; TriggerAgentRun (which needs it)
	// won't be called until the server is up and the client is set below.
	pgStore := api.NewPgStore(pool, nil)

	// Register River workers
	workers := river.NewWorkers()
	notifyWorker := routing.NewNotifyWorker(pgStore, os.Getenv("PUBLIC_URL"))
	river.AddWorker(workers, notifyWorker)

	// Agent worker: requires an LLM API key. If unavailable, agent jobs
	// will remain in the queue until the key is configured and the server
	// is restarted.
	llmProvider := envOr("LLM_PROVIDER", "gemini")
	llmModelDefault := "gemini-2.5-flash"
	if llmProvider == "openai" {
		llmModelDefault = "gpt-5.2"
	}
	llmConfig := agentpkg.LLMConfig{
		Provider:          llmProvider,
		Model:             envOr("LLM_MODEL", llmModelDefault),
		GoogleAPIKey:      os.Getenv("GOOGLE_API_KEY"),
		OpenAIAPIKey:      os.Getenv("OPENAI_API_KEY"),
		OpenAIBaseURL:     os.Getenv("OPENAI_BASE_URL"),
		OpenAISearchModel: envOr("OPENAI_SEARCH_MODEL", "gpt-5-search-api"),
	}
	agentOrchestrator, err := agentpkg.NewOrchestrator(pgStore, llmConfig)
	if err != nil {
		slog.Warn("agent orchestrator not available — agent jobs will not be processed", "err", err)
	} else {
		agentWorker := agentpkg.NewAgentWorker(agentOrchestrator, pgStore)
		river.AddWorker(workers, agentWorker)
		slog.Info("agent worker registered")
	}

	riverClient, err := queue.NewRiverClient(pool, workers)
	if err != nil {
		slog.Error("river client failed", "err", err)
		os.Exit(1)
	}

	// Now that the river client exists, set it on the existing pgStore so that
	// all holders (including NotifyWorker) can enqueue agent jobs.
	pgStore.SetRiverClient(riverClient)

	if err := riverClient.Start(ctx); err != nil {
		slog.Error("river start failed", "err", err)
		os.Exit(1)
	}

	// Ingestion layer
	ingestionStore := ingestion.NewPgStore(pool, riverClient)
	svc := ingestion.NewService(ingestionStore)

	loader := ingestion.NewSourceLoader(pool, http.DefaultClient)
	orch := ingestion.NewOrchestrator(svc, loader, pool, 5*time.Minute)

	broadcaster := api.NewBroadcaster()

	mux := http.NewServeMux()

	// Serve frontend static export if available
	if info, err := os.Stat("web/out"); err == nil && info.IsDir() {
		api.RegisterFrontend(mux, "web/out")
		slog.Info("serving frontend from web/out")
	}

	// Register all API v1 routes
	api.RegisterRoutes(mux, api.Dependencies{
		DB:                    pool,
		ProjectsStore:         pgStore,
		ReleasesStore:         pgStore,
		SubscriptionsStore:    pgStore,
		SourcesStore:          pgStore,
		ChannelsStore:         pgStore,
		ContextSourcesStore:   pgStore,
		SemanticReleasesStore: pgStore,
		AgentStore:            pgStore,
		KeyStore:              pgStore,
		HealthChecker:         pgStore,
		Broadcaster:           broadcaster,
		NoAuth:                noAuth,
		IngestionService:      svc,
		HTTPClient:            http.DefaultClient,
	})

	srv := &http.Server{Addr: addr, Handler: api.CORS(mux)}

	// Start polling in background
	go orch.Run(ctx)

	// Start PostgreSQL LISTEN/NOTIFY → SSE bridge
	go api.ListenForNotifications(ctx, pool, broadcaster)

	// Start HTTP server
	go func() {
		slog.Info("server starting", "addr", srv.Addr)
		if err := srv.ListenAndServe(); err != http.ErrServerClosed {
			slog.Error("server error", "err", err)
		}
	}()

	<-ctx.Done()
	slog.Info("shutting down")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	riverClient.Stop(shutdownCtx)
	srv.Shutdown(shutdownCtx)
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
