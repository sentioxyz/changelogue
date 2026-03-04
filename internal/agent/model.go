package agent

import (
	"context"
	"fmt"

	"google.golang.org/adk/model"
	"google.golang.org/adk/model/gemini"
	"google.golang.org/genai"

	oaimodel "github.com/sentioxyz/changelogue/internal/agent/openai"
)

// LLMConfig holds the configuration for creating an LLM model.
type LLMConfig struct {
	Provider string // "gemini" or "openai"
	Model    string // e.g. "gemini-2.5-flash", "gpt-5.2"

	// Gemini
	GoogleAPIKey string

	// OpenAI
	OpenAIAPIKey  string
	OpenAIBaseURL string // defaults to https://api.openai.com/v1
}

// NewLLMModel creates a model.LLM based on the configured provider.
// wsConfig is optional and only applies to the OpenAI provider; it enables
// the Responses API built-in web_search tool on the created model.
func NewLLMModel(ctx context.Context, cfg LLMConfig, wsConfig *oaimodel.WebSearch) (model.LLM, error) {
	switch cfg.Provider {
	case "gemini", "":
		if cfg.GoogleAPIKey == "" {
			return nil, fmt.Errorf("GOOGLE_API_KEY is required for gemini provider")
		}
		return gemini.NewModel(ctx, cfg.Model, &genai.ClientConfig{
			APIKey: cfg.GoogleAPIKey,
		})

	case "openai":
		return oaimodel.NewModel(ctx, cfg.Model, oaimodel.Config{
			APIKey:  cfg.OpenAIAPIKey,
			BaseURL: cfg.OpenAIBaseURL,
		}, wsConfig)

	default:
		return nil, fmt.Errorf("unknown LLM provider: %q (supported: gemini, openai)", cfg.Provider)
	}
}
