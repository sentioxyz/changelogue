package onboard

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

// DependencyFile holds the path and decoded content of a dependency manifest.
type DependencyFile struct {
	Path    string `json:"path"`
	Content string `json:"content"`
}

// depFileNames lists known dependency/manifest file names.
var depFileNames = map[string]bool{
	"go.mod":               true,
	"go.sum":               false, // skip, too large and low value
	"package.json":         true,
	"package-lock.json":    false,
	"yarn.lock":            false,
	"requirements.txt":     true,
	"Pipfile":              true,
	"pyproject.toml":       true,
	"Cargo.toml":           true,
	"Gemfile":              true,
	"pom.xml":              true,
	"build.gradle":         true,
	"build.gradle.kts":     true,
	"Dockerfile":           true,
	"docker-compose.yml":   true,
	"docker-compose.yaml":  true,
}

// Scanner fetches dependency files from a GitHub repo via the API.
type Scanner struct {
	client  *http.Client
	baseURL string
	token   string
}

// NewScanner creates a Scanner. If token is empty, it reads GITHUB_TOKEN from env.
func NewScanner(client *http.Client, baseURL, token string) *Scanner {
	if token == "" {
		token = os.Getenv("GITHUB_TOKEN")
	}
	if baseURL == "" {
		baseURL = "https://api.github.com"
	}
	return &Scanner{client: client, baseURL: baseURL, token: token}
}

// ParseRepoURL extracts owner and repo from various GitHub URL formats.
func ParseRepoURL(raw string) (owner, repo string, err error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", "", fmt.Errorf("empty repo URL")
	}
	// Strip common prefixes
	raw = strings.TrimPrefix(raw, "https://")
	raw = strings.TrimPrefix(raw, "http://")
	raw = strings.TrimPrefix(raw, "github.com/")
	raw = strings.TrimSuffix(raw, ".git")
	raw = strings.TrimSuffix(raw, "/")

	parts := strings.SplitN(raw, "/", 3)
	if len(parts) < 2 || parts[0] == "" || parts[1] == "" {
		return "", "", fmt.Errorf("invalid repo URL: expected 'owner/repo', got %q", raw)
	}
	return parts[0], parts[1], nil
}

// FetchDependencyFiles fetches the repo tree, identifies dependency files, and returns their contents.
func (s *Scanner) FetchDependencyFiles(ctx context.Context, owner, repo string) ([]DependencyFile, error) {
	branch, err := s.getDefaultBranch(ctx, owner, repo)
	if err != nil {
		return nil, fmt.Errorf("get default branch: %w", err)
	}

	tree, err := s.getTree(ctx, owner, repo, branch)
	if err != nil {
		return nil, fmt.Errorf("get tree: %w", err)
	}

	// Filter to dependency files
	var depPaths []string
	for _, entry := range tree {
		if entry.Type != "blob" {
			continue
		}
		base := filepath.Base(entry.Path)
		if depFileNames[base] {
			depPaths = append(depPaths, entry.Path)
		}
	}

	// Fetch contents
	var files []DependencyFile
	for _, path := range depPaths {
		content, err := s.getFileContent(ctx, owner, repo, path)
		if err != nil {
			return nil, fmt.Errorf("fetch %s: %w", path, err)
		}
		files = append(files, DependencyFile{Path: path, Content: content})
	}
	return files, nil
}

type treeEntry struct {
	Path string `json:"path"`
	Type string `json:"type"`
}

func (s *Scanner) getDefaultBranch(ctx context.Context, owner, repo string) (string, error) {
	url := fmt.Sprintf("%s/repos/%s/%s", s.baseURL, owner, repo)
	var result struct {
		DefaultBranch string `json:"default_branch"`
	}
	if err := s.doGet(ctx, url, &result); err != nil {
		return "", err
	}
	return result.DefaultBranch, nil
}

func (s *Scanner) getTree(ctx context.Context, owner, repo, branch string) ([]treeEntry, error) {
	url := fmt.Sprintf("%s/repos/%s/%s/git/trees/%s?recursive=1", s.baseURL, owner, repo, branch)
	var result struct {
		Tree []treeEntry `json:"tree"`
	}
	if err := s.doGet(ctx, url, &result); err != nil {
		return nil, err
	}
	return result.Tree, nil
}

func (s *Scanner) getFileContent(ctx context.Context, owner, repo, path string) (string, error) {
	url := fmt.Sprintf("%s/repos/%s/%s/contents/%s", s.baseURL, owner, repo, path)
	var result struct {
		Content  string `json:"content"`
		Encoding string `json:"encoding"`
	}
	if err := s.doGet(ctx, url, &result); err != nil {
		return "", err
	}
	if result.Encoding == "base64" {
		decoded, err := base64.StdEncoding.DecodeString(strings.ReplaceAll(result.Content, "\n", ""))
		if err != nil {
			return "", fmt.Errorf("decode base64: %w", err)
		}
		return string(decoded), nil
	}
	return result.Content, nil
}

func (s *Scanner) doGet(ctx context.Context, url string, out any) error {
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return err
	}
	if s.token != "" {
		req.Header.Set("Authorization", "Bearer "+s.token)
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	resp, err := s.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("GitHub API returned %d for %s", resp.StatusCode, url)
	}
	return json.NewDecoder(resp.Body).Decode(out)
}
