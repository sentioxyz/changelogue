package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"time"

	"github.com/riverqueue/river"
	"github.com/sentioxyz/releaseguard/internal/db"
	"github.com/sentioxyz/releaseguard/internal/ingestion"
	"github.com/sentioxyz/releaseguard/internal/pipeline"
	"github.com/sentioxyz/releaseguard/internal/queue"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	dbURL := envOr("DATABASE_URL", "postgres://localhost:5432/releaseguard?sslmode=disable")
	ghSecret := envOr("GITHUB_WEBHOOK_SECRET", "")
	addr := envOr("LISTEN_ADDR", ":8080")

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

	// Pipeline
	pipelineStore := pipeline.NewPgStore(pool)
	pipelineRunner := pipeline.NewRunner(
		pipelineStore,
		[]pipeline.PipelineNode{
			pipeline.NewRegexNormalizer(),
			pipeline.NewSubscriptionRouter(pipelineStore),
		},
		[]pipeline.PipelineNode{
			pipeline.NewUrgencyScorer(),
		},
	)

	// River queue with pipeline worker
	workers := river.NewWorkers()
	river.AddWorker(workers, pipeline.NewWorker(pipelineRunner))

	riverClient, err := queue.NewRiverClient(pool, workers)
	if err != nil {
		slog.Error("river client failed", "err", err)
		os.Exit(1)
	}

	if err := riverClient.Start(ctx); err != nil {
		slog.Error("river start failed", "err", err)
		os.Exit(1)
	}

	// Ingestion layer
	store := ingestion.NewPgStore(pool, riverClient)
	svc := ingestion.NewService(store)

	sources := []ingestion.IIngestionSource{
		ingestion.NewDockerHubSource(http.DefaultClient, "library/golang"),
	}

	orch := ingestion.NewOrchestrator(svc, sources, 5*time.Minute)

	// GitHub webhook handler
	webhookHandler := ingestion.NewGitHubWebhookHandler(ghSecret, func(results []ingestion.IngestionResult) {
		if err := svc.ProcessResults(ctx, "github", results); err != nil {
			slog.Error("github webhook processing failed", "err", err)
		}
	})

	mux := http.NewServeMux()
	mux.Handle("POST /webhook/github", webhookHandler)

	srv := &http.Server{Addr: addr, Handler: mux}

	// Start polling in background
	go orch.Run(ctx)

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
