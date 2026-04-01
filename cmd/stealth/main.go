package main

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"time"

	"github.com/sentioxyz/changelogue/internal/api"
	"github.com/sentioxyz/changelogue/internal/ingestion"
	"github.com/sentioxyz/changelogue/internal/routing"
	"github.com/sentioxyz/changelogue/internal/stealth"
)

func main() {
	if len(os.Args) > 1 {
		switch os.Args[1] {
		case "install":
			install()
			return
		case "uninstall":
			uninstall()
			return
		case "status":
			status()
			return
		case "serve":
			// fall through to server startup
		default:
			fmt.Fprintf(os.Stderr, "Usage: clog-stealth [serve|install|uninstall|status]\n")
			os.Exit(1)
		}
	}

	// Configure logging
	logLevel := new(slog.LevelVar)
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

	// Database path
	home, err := os.UserHomeDir()
	if err != nil {
		slog.Error("cannot determine home directory", "err", err)
		os.Exit(1)
	}
	dbPath := envOr("CHANGELOGUE_STEALTH_DB", filepath.Join(home, ".changelogue", "stealth.db"))
	port := envOr("CHANGELOGUE_STEALTH_PORT", "9876")
	addr := "localhost:" + port
	noAuth := os.Getenv("NO_AUTH") != "false" // default to no auth in stealth mode

	// Open SQLite store
	store, err := stealth.NewStore(dbPath)
	if err != nil {
		slog.Error("failed to open database", "path", dbPath, "err", err)
		os.Exit(1)
	}
	defer store.Close()

	// Ingestion layer
	svc := ingestion.NewService(store)
	loader := ingestion.NewSourceLoader(store, http.DefaultClient)
	pollInterval := 5 * time.Minute
	if v := os.Getenv("POLL_INTERVAL"); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			pollInterval = d
		}
	}
	orch := ingestion.NewOrchestrator(svc, loader, store, pollInterval)

	// Senders
	senders := routing.NewSenders()

	// Wire up post-ingest notification hook
	store.NotifyHook = func(ctx context.Context, releaseID, sourceID string) {
		store.NotifyRelease(ctx, releaseID, sourceID, senders)
	}

	// API dependencies
	broadcaster := api.NewBroadcaster()
	mux := http.NewServeMux()
	api.RegisterRoutes(mux, api.Dependencies{
		ProjectsStore:         store,
		ReleasesStore:         store,
		SubscriptionsStore:    store,
		SourcesStore:          store,
		ChannelsStore:         store,
		ContextSourcesStore:   stealth.ContextSourcesStub{},
		SemanticReleasesStore: stealth.SemanticReleasesStub{},
		AgentStore:            stealth.AgentStub{},
		TodosStore:            stealth.TodosStub{},
		OnboardStore:          stealth.OnboardStub{},
		GatesStore:            stealth.GatesStub{},
		KeyStore:              store,
		SessionValidator:      stealth.SessionValidatorStub{},
		HealthChecker:         store,
		Broadcaster:           broadcaster,
		NoAuth:                noAuth,
		IngestionService:      svc,
		HTTPClient:            http.DefaultClient,
	})

	srv := &http.Server{Addr: addr, Handler: api.CORS(mux)}

	// Write PID file
	pidPath := filepath.Join(home, ".changelogue", "stealth.pid")
	os.WriteFile(pidPath, []byte(fmt.Sprintf("%d", os.Getpid())), 0o644)
	defer os.Remove(pidPath)

	// Start polling in background
	go orch.Run(ctx)

	// Start HTTP server
	go func() {
		slog.Info("stealth server starting", "addr", addr, "db", dbPath)
		if err := srv.ListenAndServe(); err != http.ErrServerClosed {
			slog.Error("server error", "err", err)
		}
	}()

	<-ctx.Done()
	slog.Info("shutting down")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	srv.Shutdown(shutdownCtx)
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func install() {
	fmt.Println("TODO: install system service (launchd/systemd)")
}

func uninstall() {
	fmt.Println("TODO: uninstall system service")
}

func status() {
	home, _ := os.UserHomeDir()
	pidPath := filepath.Join(home, ".changelogue", "stealth.pid")
	data, err := os.ReadFile(pidPath)
	if err != nil {
		fmt.Println("stealth: not running (no PID file)")
		return
	}
	fmt.Printf("stealth: running (PID %s)\n", string(data))
}
