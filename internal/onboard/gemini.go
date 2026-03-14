package onboard

import (
	"context"
	"encoding/json"
	"fmt"
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

// DependencyExtractor uses Gemini to extract dependencies from file contents.
type DependencyExtractor struct {
	client *genai.Client
	model  string
}

// NewDependencyExtractor creates a DependencyExtractor.
func NewDependencyExtractor(ctx context.Context, apiKey, model string) (*DependencyExtractor, error) {
	client, err := genai.NewClient(ctx, &genai.ClientConfig{
		APIKey:  apiKey,
		Backend: genai.BackendGeminiAPI,
	})
	if err != nil {
		return nil, fmt.Errorf("create genai client: %w", err)
	}
	if model == "" {
		model = "gemini-2.0-flash"
	}
	return &DependencyExtractor{client: client, model: model}, nil
}

// Extract sends file contents to Gemini and returns parsed dependencies.
func (e *DependencyExtractor) Extract(ctx context.Context, files []DependencyFile) ([]models.ScannedDependency, error) {
	prompt := BuildExtractionPrompt(files)

	resp, err := e.client.Models.GenerateContent(ctx, e.model, []*genai.Content{genai.NewContentFromText(prompt, "user")}, &genai.GenerateContentConfig{
		SystemInstruction: genai.NewContentFromText(extractionSystemPrompt, "user"),
		Temperature:       genai.Ptr(float32(0.1)),
	})
	if err != nil {
		return nil, fmt.Errorf("generate content: %w", err)
	}

	if len(resp.Candidates) == 0 || len(resp.Candidates[0].Content.Parts) == 0 {
		return nil, fmt.Errorf("empty response from Gemini")
	}

	text := resp.Candidates[0].Content.Parts[0].Text
	return ParseDependencies([]byte(text))
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
