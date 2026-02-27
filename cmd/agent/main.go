package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"log/slog"
	"os"

	"google.golang.org/adk/agent"
	"google.golang.org/adk/cmd/launcher"
	"google.golang.org/adk/cmd/launcher/full"

	agentpkg "github.com/sentioxyz/changelogue/internal/agent"
	"github.com/sentioxyz/changelogue/internal/api"
	"github.com/sentioxyz/changelogue/internal/db"
)

func main() {
	projectID := flag.String("project-id", "", "Project ID to scope the agent to (required)")
	flag.Parse()

	if *projectID == "" {
		fmt.Fprintln(os.Stderr, "error: --project-id is required")
		fmt.Fprintf(os.Stderr, "usage: go run ./cmd/agent --project-id=<uuid> [web api webui]\n")
		os.Exit(1)
	}

	ctx := context.Background()

	// Database
	dbURL := envOr("DATABASE_URL", "postgres://localhost:5432/changelogue?sslmode=disable")
	pool, err := db.NewPool(ctx, dbURL)
	if err != nil {
		log.Fatalf("database connection failed: %v", err)
	}
	defer pool.Close()

	store := api.NewPgStore(pool, nil)

	// Validate project exists
	project, err := store.GetProject(ctx, *projectID)
	if err != nil {
		log.Fatalf("project %q not found: %v", *projectID, err)
	}
	slog.Info("loaded project", "id", project.ID, "name", project.Name)

	// LLM config (same env vars as server)
	llmProvider := envOr("LLM_PROVIDER", "gemini")
	llmModelDefault := "gemini-2.5-flash"
	if llmProvider == "openai" {
		llmModelDefault = "gpt-5.2"
	}
	llmConfig := agentpkg.LLMConfig{
		Provider:      llmProvider,
		Model:         envOr("LLM_MODEL", llmModelDefault),
		GoogleAPIKey:  os.Getenv("GOOGLE_API_KEY"),
		OpenAIAPIKey:  os.Getenv("OPENAI_API_KEY"),
		OpenAIBaseURL: os.Getenv("OPENAI_BASE_URL"),
	}

	agentInstance, err := agentpkg.BuildAgent(ctx, store, project, llmConfig, "")
	if err != nil {
		log.Fatalf("build agent: %v", err)
	}

	// Launch with ADK-Web UI
	config := &launcher.Config{
		AgentLoader: agent.NewSingleLoader(agentInstance),
	}

	l := full.NewLauncher()
	if err := l.Execute(ctx, config, flag.Args()); err != nil {
		log.Fatalf("launcher failed: %v\n\n%s", err, l.CommandLineSyntax())
	}
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
