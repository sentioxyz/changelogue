package onboard

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/sentioxyz/changelogue/internal/models"
	"google.golang.org/genai"
)

const extractionSystemPrompt = `You are a dependency extraction agent. Given the contents of dependency/manifest
files from a software project, extract all dependencies.

For each dependency, return:
- name: the package/library name
- version: the version constraint or pinned version
- ecosystem: one of "go", "npm", "pypi", "cargo", "rubygems", "maven", "gradle", "docker", "other"
- upstream_repo: your best guess at the canonical GitHub repository URL (e.g., "github.com/gorilla/mux")
- provider: the Changelogue provider to use for release tracking. Use "github" for repos
  with GitHub releases, "dockerhub" for Docker images, "gitlab" for GitLab repos,
  "ecr_public" for AWS ECR images. When unsure, default to "github".

Return ONLY a JSON array. No explanations.`

// DependencyExtractor uses an LLM to extract dependencies from file contents.
// Supports both Gemini and OpenAI providers.
type DependencyExtractor struct {
	provider string // "gemini" or "openai"
	model    string

	// Gemini
	geminiClient *genai.Client

	// OpenAI
	openaiAPIKey  string
	openaiBaseURL string
	httpClient    *http.Client
}

// ExtractorConfig holds the configuration for creating a DependencyExtractor.
type ExtractorConfig struct {
	Provider      string // "gemini" or "openai"
	Model         string
	GoogleAPIKey  string
	OpenAIAPIKey  string
	OpenAIBaseURL string // defaults to https://api.openai.com/v1
}

// NewDependencyExtractor creates a DependencyExtractor for the configured provider.
func NewDependencyExtractor(ctx context.Context, cfg ExtractorConfig) (*DependencyExtractor, error) {
	switch cfg.Provider {
	case "gemini", "":
		if cfg.GoogleAPIKey == "" {
			return nil, fmt.Errorf("GOOGLE_API_KEY is required for gemini provider")
		}
		client, err := genai.NewClient(ctx, &genai.ClientConfig{
			APIKey:  cfg.GoogleAPIKey,
			Backend: genai.BackendGeminiAPI,
		})
		if err != nil {
			return nil, fmt.Errorf("create genai client: %w", err)
		}
		model := cfg.Model
		if model == "" {
			model = "gemini-2.0-flash"
		}
		return &DependencyExtractor{
			provider:     "gemini",
			model:        model,
			geminiClient: client,
		}, nil

	case "openai":
		if cfg.OpenAIAPIKey == "" {
			return nil, fmt.Errorf("OPENAI_API_KEY is required for openai provider")
		}
		baseURL := cfg.OpenAIBaseURL
		if baseURL == "" {
			baseURL = "https://api.openai.com/v1"
		}
		model := cfg.Model
		if model == "" {
			model = "gpt-4o-mini"
		}
		return &DependencyExtractor{
			provider:      "openai",
			model:         model,
			openaiAPIKey:  cfg.OpenAIAPIKey,
			openaiBaseURL: baseURL,
			httpClient:    &http.Client{},
		}, nil

	default:
		return nil, fmt.Errorf("unknown LLM provider: %q (supported: gemini, openai)", cfg.Provider)
	}
}

// Extract sends file contents to the configured LLM and returns parsed dependencies.
func (e *DependencyExtractor) Extract(ctx context.Context, files []DependencyFile) ([]models.ScannedDependency, error) {
	prompt := BuildExtractionPrompt(files)

	var text string
	var err error

	switch e.provider {
	case "gemini":
		text, err = e.extractGemini(ctx, prompt)
	case "openai":
		text, err = e.extractOpenAI(ctx, prompt)
	default:
		return nil, fmt.Errorf("unknown provider: %s", e.provider)
	}

	if err != nil {
		return nil, err
	}

	return ParseDependencies([]byte(text))
}

func (e *DependencyExtractor) extractGemini(ctx context.Context, prompt string) (string, error) {
	resp, err := e.geminiClient.Models.GenerateContent(ctx, e.model, []*genai.Content{genai.NewContentFromText(prompt, "user")}, &genai.GenerateContentConfig{
		SystemInstruction: genai.NewContentFromText(extractionSystemPrompt, "user"),
		Temperature:       genai.Ptr(float32(0.1)),
	})
	if err != nil {
		return "", fmt.Errorf("generate content: %w", err)
	}

	if len(resp.Candidates) == 0 || len(resp.Candidates[0].Content.Parts) == 0 {
		return "", fmt.Errorf("empty response from Gemini")
	}

	return resp.Candidates[0].Content.Parts[0].Text, nil
}

// OpenAI Responses API types (minimal, for dependency extraction only)
type openaiResponsesRequest struct {
	Model        string           `json:"model"`
	Input        []openaiInput    `json:"input"`
	Instructions string           `json:"instructions,omitempty"`
	Temperature  *float32         `json:"temperature,omitempty"`
	Store        bool             `json:"store"`
}

type openaiInput struct {
	Type    string `json:"type"`
	Role    string `json:"role,omitempty"`
	Content string `json:"content,omitempty"`
}

type openaiResponsesResponse struct {
	Output []openaiOutputItem `json:"output"`
	Error  *openaiError       `json:"error,omitempty"`
}

type openaiOutputItem struct {
	Type    string                `json:"type"`
	Content []openaiOutputContent `json:"content,omitempty"`
}

type openaiOutputContent struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

type openaiError struct {
	Message string `json:"message"`
	Type    string `json:"type"`
}

func (e *DependencyExtractor) extractOpenAI(ctx context.Context, prompt string) (string, error) {
	temp := float32(0.1)
	reqBody := openaiResponsesRequest{
		Model: e.model,
		Input: []openaiInput{
			{Type: "message", Role: "user", Content: prompt},
		},
		Instructions: extractionSystemPrompt,
		Temperature:  &temp,
		Store:        false,
	}

	body, err := json.Marshal(reqBody)
	if err != nil {
		return "", fmt.Errorf("marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, e.openaiBaseURL+"/responses", bytes.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("create http request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+e.openaiAPIKey)

	httpResp, err := e.httpClient.Do(httpReq)
	if err != nil {
		return "", fmt.Errorf("http request: %w", err)
	}
	defer httpResp.Body.Close()

	respBody, err := io.ReadAll(httpResp.Body)
	if err != nil {
		return "", fmt.Errorf("read response: %w", err)
	}

	if httpResp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("OpenAI API returned status %d: %s", httpResp.StatusCode, string(respBody))
	}

	var resp openaiResponsesResponse
	if err := json.Unmarshal(respBody, &resp); err != nil {
		return "", fmt.Errorf("unmarshal response: %w", err)
	}

	if resp.Error != nil {
		return "", fmt.Errorf("OpenAI API error: %s (type=%s)", resp.Error.Message, resp.Error.Type)
	}

	// Extract text from message output
	for _, item := range resp.Output {
		if item.Type == "message" {
			for _, c := range item.Content {
				if c.Type == "output_text" && c.Text != "" {
					return c.Text, nil
				}
			}
		}
	}

	return "", fmt.Errorf("empty response from OpenAI")
}

// BuildExtractionPrompt constructs the user prompt from dependency files.
func BuildExtractionPrompt(files []DependencyFile) string {
	var b strings.Builder
	b.WriteString("Extract all dependencies from these files:\n\n")
	for _, f := range files {
		b.WriteString(fmt.Sprintf("--- %s ---\n%s\n\n", f.Path, f.Content))
	}
	return b.String()
}

// ParseDependencies parses the LLM response into ScannedDependency structs.
// Handles raw JSON arrays and markdown-fenced JSON.
func ParseDependencies(raw []byte) ([]models.ScannedDependency, error) {
	text := strings.TrimSpace(string(raw))

	// Strip markdown code fences if present
	if strings.HasPrefix(text, "```") {
		lines := strings.Split(text, "\n")
		if len(lines) > 2 {
			// Remove first and last lines (``` markers)
			lines = lines[1 : len(lines)-1]
			// If last line is also ```, it was already removed
			if len(lines) > 0 && strings.TrimSpace(lines[len(lines)-1]) == "```" {
				lines = lines[:len(lines)-1]
			}
			text = strings.Join(lines, "\n")
		}
	}

	var deps []models.ScannedDependency
	if err := json.Unmarshal([]byte(text), &deps); err != nil {
		return nil, fmt.Errorf("parse dependencies JSON: %w\nraw: %s", err, text)
	}
	return deps, nil
}
