# CLI Support (`clog`) Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add a `clog` CLI binary that manages Changelogue resources (projects, sources, releases, channels, subscriptions) via the REST API, with AI-friendly command hints for mistyped commands.

**Architecture:** Separate binary at `cmd/cli/main.go` using Cobra for command routing. Thin HTTP client in `internal/cli/client.go` calls the existing REST API. Reuses `internal/models` types for JSON deserialization. Table output via stdlib `text/tabwriter`, with `--json` flag for machine-readable output.

**Tech Stack:** Go 1.25, Cobra, stdlib `net/http`, `text/tabwriter`, `encoding/json`

**Spec:** `docs/superpowers/specs/2026-03-23-cli-support-design.md`

---

### Task 1: Add Cobra dependency and scaffold root command

**Files:**
- Create: `cmd/cli/main.go`
- Modify: `go.mod` (via `go get`)
- Modify: `Makefile` (add `cli` target)

- [ ] **Step 1: Add Cobra dependency**

Run:
```bash
go get github.com/spf13/cobra@latest
```

- [ ] **Step 2: Create `cmd/cli/main.go` with root command**

```go
package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/sentioxyz/changelogue/internal/cli"
	"github.com/spf13/cobra"
)

var version = "dev"

func main() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

var (
	serverURL string
	apiKey    string
	jsonOut   bool
)

var rootCmd = &cobra.Command{
	Use:   "clog",
	Short: "Changelogue CLI — manage projects, sources, releases, channels, and subscriptions",
	Long: `clog is the command-line interface for Changelogue.

It talks to a running Changelogue server via its REST API.
Configure the server URL and API key via flags or environment variables:

  export CHANGELOGUE_SERVER=http://localhost:8080
  export CHANGELOGUE_API_KEY=rg_live_abc123...

Examples:
  clog projects list
  clog sources create --project <id> --provider dockerhub --repository library/postgres
  clog releases list --project <id>
  clog channels create --name my-slack --type slack --config '{"webhook_url":"https://..."}'`,
	SilenceUsage:  true,
	SilenceErrors: true,
}

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print the CLI version",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("clog version", version)
	},
}

// newClient builds a Client from resolved global flags. Called at command execution
// time (not init time) so that flag values and env vars are available.
func newClient() *cli.Client {
	return cli.NewClient(resolveServerURL(), resolveAPIKey())
}

func init() {
	rootCmd.PersistentFlags().StringVar(&serverURL, "server", "", "Changelogue server URL (env: CHANGELOGUE_SERVER)")
	rootCmd.PersistentFlags().StringVar(&apiKey, "api-key", "", "API key for authentication (env: CHANGELOGUE_API_KEY)")
	rootCmd.PersistentFlags().BoolVar(&jsonOut, "json", false, "Output raw JSON instead of table")

	rootCmd.AddCommand(versionCmd)

	// Resource subcommands — each takes newClient so the client is built lazily.
	// Additional subcommands will be added here in subsequent tasks.

	// AI-friendly hints: suggest commands on typo
	rootCmd.SuggestionsMinimumDistance = 2

	// Custom error formatting for unknown flags
	rootCmd.SetFlagErrorFunc(func(cmd *cobra.Command, err error) error {
		msg := err.Error()
		// Try to suggest a close flag match
		if strings.Contains(msg, "unknown flag") {
			cmd.PrintErrln("Error:", msg)
			cmd.PrintErrln()
			cmd.PrintErrln("Available flags:")
			cmd.Flags().PrintDefaults()
			return err
		}
		return err
	})
}

func resolveServerURL() string {
	if serverURL != "" {
		return serverURL
	}
	if v := os.Getenv("CHANGELOGUE_SERVER"); v != "" {
		return v
	}
	return "http://localhost:8080"
}

func resolveAPIKey() string {
	if apiKey != "" {
		return apiKey
	}
	return os.Getenv("CHANGELOGUE_API_KEY")
}
```

- [ ] **Step 3: Add `cli` target to Makefile**

Add after the existing `build` target in the `# --- Backend ---` section of `Makefile`:

```makefile
VERSION ?= dev
cli:
	go build -ldflags "-X main.version=$(VERSION)" -o clog ./cmd/cli
```

Also add `cli` to the `.PHONY` line.

- [ ] **Step 4: Build and verify**

Run:
```bash
make cli && ./clog version
```
Expected: `clog version dev`

Run:
```bash
./clog --help
```
Expected: Shows root help with description and examples.

Run:
```bash
./clog projet
```
Expected: Shows error with "Did you mean: projects" (once projects subcommand is added — for now just verifies no crash).

- [ ] **Step 5: Commit**

```bash
git add cmd/cli/main.go Makefile go.mod go.sum
git commit -m "feat(cli): scaffold clog binary with Cobra root command and version"
```

---

### Task 2: HTTP client and output helpers

**Files:**
- Create: `internal/cli/client.go`
- Create: `internal/cli/output.go`
- Create: `internal/cli/client_test.go`
- Create: `internal/cli/output_test.go`

- [ ] **Step 1: Write failing test for HTTP client**

Create `internal/cli/client_test.go`:

```go
package cli

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestClientGet(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("expected GET, got %s", r.Method)
		}
		if r.URL.Path != "/api/v1/projects" {
			t.Errorf("expected /api/v1/projects, got %s", r.URL.Path)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer test-key" {
			t.Errorf("expected Bearer test-key, got %s", got)
		}
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]any{"data": []any{}, "meta": map[string]any{"request_id": "r1"}})
	}))
	defer srv.Close()

	c := NewClient(srv.URL, "test-key")
	resp, err := c.Get("/api/v1/projects")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
}

func TestClientPost(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", r.Method)
		}
		if ct := r.Header.Get("Content-Type"); ct != "application/json" {
			t.Errorf("expected application/json, got %s", ct)
		}
		var body map[string]string
		json.NewDecoder(r.Body).Decode(&body)
		if body["name"] != "test-project" {
			t.Errorf("expected name=test-project, got %s", body["name"])
		}
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(map[string]any{"data": body, "meta": map[string]any{"request_id": "r2"}})
	}))
	defer srv.Close()

	c := NewClient(srv.URL, "test-key")
	resp, err := c.Post("/api/v1/projects", map[string]string{"name": "test-project"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("expected 201, got %d", resp.StatusCode)
	}
}

func TestClientHandlesErrorResponse(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		json.NewEncoder(w).Encode(map[string]any{
			"error": map[string]string{"code": "unauthorized", "message": "Invalid API key"},
			"meta":  map[string]any{"request_id": "r3"},
		})
	}))
	defer srv.Close()

	c := NewClient(srv.URL, "bad-key")
	resp, err := c.Get("/api/v1/projects")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer resp.Body.Close()

	apiErr, err := DecodeError(resp)
	if err != nil {
		t.Fatalf("failed to decode error: %v", err)
	}
	if apiErr.Err.Code != "unauthorized" {
		t.Errorf("expected code unauthorized, got %s", apiErr.Err.Code)
	}
}

func TestDecodeResponse(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]any{
			"data": map[string]string{"id": "abc", "name": "proj"},
			"meta": map[string]any{"request_id": "r4"},
		})
	}))
	defer srv.Close()

	c := NewClient(srv.URL, "key")
	resp, _ := c.Get("/test")
	defer resp.Body.Close()

	type item struct {
		ID   string `json:"id"`
		Name string `json:"name"`
	}
	var result APIResponse[item]
	if err := DecodeJSON(resp, &result); err != nil {
		t.Fatalf("decode failed: %v", err)
	}
	if result.Data.ID != "abc" {
		t.Errorf("expected id abc, got %s", result.Data.ID)
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run:
```bash
go test ./internal/cli/... -v
```
Expected: FAIL — package does not exist yet.

- [ ] **Step 3: Implement HTTP client**

Create `internal/cli/client.go`:

```go
package cli

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// Client is a thin HTTP wrapper for the Changelogue REST API.
type Client struct {
	BaseURL    string
	APIKey     string
	HTTPClient *http.Client
}

// NewClient creates a new API client.
func NewClient(baseURL, apiKey string) *Client {
	return &Client{
		BaseURL: baseURL,
		APIKey:  apiKey,
		HTTPClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

func (c *Client) do(method, path string, body any) (*http.Response, error) {
	var buf *bytes.Buffer
	if body != nil {
		buf = new(bytes.Buffer)
		if err := json.NewEncoder(buf).Encode(body); err != nil {
			return nil, fmt.Errorf("encoding request body: %w", err)
		}
	}

	var req *http.Request
	var err error
	if buf != nil {
		req, err = http.NewRequest(method, c.BaseURL+path, buf)
	} else {
		req, err = http.NewRequest(method, c.BaseURL+path, nil)
	}
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}

	if c.APIKey != "" {
		req.Header.Set("Authorization", "Bearer "+c.APIKey)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	return c.HTTPClient.Do(req)
}

// Get sends a GET request to the given API path.
func (c *Client) Get(path string) (*http.Response, error) {
	return c.do(http.MethodGet, path, nil)
}

// Post sends a POST request with a JSON body.
func (c *Client) Post(path string, body any) (*http.Response, error) {
	return c.do(http.MethodPost, path, body)
}

// Put sends a PUT request with a JSON body.
func (c *Client) Put(path string, body any) (*http.Response, error) {
	return c.do(http.MethodPut, path, body)
}

// Delete sends a DELETE request.
func (c *Client) Delete(path string) (*http.Response, error) {
	return c.do(http.MethodDelete, path, nil)
}

// DeleteWithBody sends a DELETE request with a JSON body (for batch operations).
func (c *Client) DeleteWithBody(path string, body any) (*http.Response, error) {
	return c.do(http.MethodDelete, path, body)
}

// --- Response types ---

// APIResponse is the generic success envelope from the Changelogue API.
type APIResponse[T any] struct {
	Data T    `json:"data"`
	Meta Meta `json:"meta"`
}

// Meta contains response metadata.
type Meta struct {
	RequestID string `json:"request_id"`
	Page      int    `json:"page,omitempty"`
	PerPage   int    `json:"per_page,omitempty"`
	Total     int    `json:"total,omitempty"`
}

// APIErrorResponse is the error envelope from the Changelogue API.
type APIErrorResponse struct {
	Err  struct {
		Code    string `json:"code"`
		Message string `json:"message"`
	} `json:"error"`
	Meta Meta `json:"meta"`
}

// DecodeJSON decodes a successful response into the given APIResponse.
func DecodeJSON[T any](resp *http.Response, dst *APIResponse[T]) error {
	return json.NewDecoder(resp.Body).Decode(dst)
}

// DecodeError decodes an error response envelope.
func DecodeError(resp *http.Response) (*APIErrorResponse, error) {
	var apiErr APIErrorResponse
	if err := json.NewDecoder(resp.Body).Decode(&apiErr); err != nil {
		return nil, err
	}
	return &apiErr, nil
}

// CheckResponse checks for HTTP error status and returns a user-friendly error.
// Returns nil if the status is 2xx.
func CheckResponse(resp *http.Response) error {
	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		return nil
	}

	apiErr, err := DecodeError(resp)
	if err != nil {
		return fmt.Errorf("server returned %d", resp.StatusCode)
	}

	switch resp.StatusCode {
	case http.StatusUnauthorized:
		return fmt.Errorf("authentication failed: %s\nCheck your --api-key flag or CHANGELOGUE_API_KEY environment variable", apiErr.Err.Message)
	case http.StatusNotFound:
		return fmt.Errorf("not found: %s", apiErr.Err.Message)
	case http.StatusTooManyRequests:
		retryAfter := resp.Header.Get("Retry-After")
		return fmt.Errorf("rate limited — retry after %s seconds", retryAfter)
	case http.StatusConflict:
		return fmt.Errorf("conflict: %s", apiErr.Err.Message)
	case http.StatusUnprocessableEntity:
		return fmt.Errorf("validation error: %s", apiErr.Err.Message)
	default:
		return fmt.Errorf("server error (%d): %s", resp.StatusCode, apiErr.Err.Message)
	}
}
```

- [ ] **Step 4: Write failing test for output helpers**

Create `internal/cli/output_test.go`:

```go
package cli

import (
	"bytes"
	"strings"
	"testing"
)

func TestRenderTable(t *testing.T) {
	var buf bytes.Buffer
	headers := []string{"ID", "NAME", "CREATED"}
	rows := [][]string{
		{"abc-123", "my-project", "2026-03-23"},
		{"def-456", "other-proj", "2026-03-22"},
	}
	RenderTableTo(&buf, headers, rows)
	out := buf.String()

	if !strings.Contains(out, "ID") {
		t.Error("expected header ID in output")
	}
	if !strings.Contains(out, "my-project") {
		t.Error("expected my-project in output")
	}
	if !strings.Contains(out, "other-proj") {
		t.Error("expected other-proj in output")
	}
}

func TestRenderTableEmpty(t *testing.T) {
	var buf bytes.Buffer
	RenderTableTo(&buf, []string{"ID", "NAME"}, nil)
	out := buf.String()
	if !strings.Contains(out, "No results") {
		t.Error("expected 'No results' message for empty table")
	}
}

func TestRenderJSON(t *testing.T) {
	var buf bytes.Buffer
	data := map[string]string{"id": "abc", "name": "test"}
	RenderJSONTo(&buf, data)
	out := buf.String()
	if !strings.Contains(out, `"id": "abc"`) {
		t.Error("expected JSON with id field")
	}
}
```

- [ ] **Step 5: Implement output helpers**

Create `internal/cli/output.go`:

```go
package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"text/tabwriter"
)

// RenderTable prints headers and rows as a tab-aligned table to stdout.
func RenderTable(headers []string, rows [][]string) {
	RenderTableTo(os.Stdout, headers, rows)
}

// RenderTableTo prints a tab-aligned table to the given writer.
func RenderTableTo(w io.Writer, headers []string, rows [][]string) {
	if len(rows) == 0 {
		fmt.Fprintln(w, "No results.")
		return
	}

	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	for i, h := range headers {
		if i > 0 {
			fmt.Fprint(tw, "\t")
		}
		fmt.Fprint(tw, h)
	}
	fmt.Fprintln(tw)

	for _, row := range rows {
		for i, col := range row {
			if i > 0 {
				fmt.Fprint(tw, "\t")
			}
			fmt.Fprint(tw, col)
		}
		fmt.Fprintln(tw)
	}
	tw.Flush()
}

// RenderJSON prints data as indented JSON to stdout.
func RenderJSON(data any) {
	RenderJSONTo(os.Stdout, data)
}

// RenderJSONTo prints data as indented JSON to the given writer.
func RenderJSONTo(w io.Writer, data any) {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	enc.Encode(data)
}

// Truncate shortens a string to maxLen, appending "..." if truncated.
func Truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	if maxLen <= 3 {
		return s[:maxLen]
	}
	return s[:maxLen-3] + "..."
}

// FormatTime formats a time string for table display (first 19 chars of ISO format).
func FormatTime(t string) string {
	if len(t) > 19 {
		return t[:19]
	}
	return t
}
```

- [ ] **Step 6: Run tests to verify they pass**

Run:
```bash
go test ./internal/cli/... -v
```
Expected: All tests PASS.

- [ ] **Step 7: Commit**

```bash
git add internal/cli/client.go internal/cli/client_test.go internal/cli/output.go internal/cli/output_test.go
git commit -m "feat(cli): add HTTP client and table/JSON output helpers"
```

---

### Task 3: Projects subcommand

**Files:**
- Create: `internal/cli/projects.go`
- Create: `internal/cli/projects_test.go`
- Modify: `cmd/cli/main.go` (register projects command)

- [ ] **Step 1: Write failing test for projects commands**

Create `internal/cli/projects_test.go`:

```go
package cli

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/sentioxyz/changelogue/internal/models"
)

func TestProjectsList(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/projects" {
			t.Errorf("expected /api/v1/projects, got %s", r.URL.Path)
		}
		if r.URL.Query().Get("page") != "1" {
			t.Errorf("expected page=1, got %s", r.URL.Query().Get("page"))
		}
		json.NewEncoder(w).Encode(map[string]any{
			"data": []models.Project{{ID: "p1", Name: "proj1"}},
			"meta": map[string]any{"request_id": "r1", "page": 1, "per_page": 25, "total": 1},
		})
	}))
	defer srv.Close()

	c := NewClient(srv.URL, "key")
	projects, meta, err := ListProjects(c, 1, 25)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(projects) != 1 {
		t.Fatalf("expected 1 project, got %d", len(projects))
	}
	if projects[0].Name != "proj1" {
		t.Errorf("expected proj1, got %s", projects[0].Name)
	}
	if meta.Total != 1 {
		t.Errorf("expected total=1, got %d", meta.Total)
	}
}

func TestProjectsCreate(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", r.Method)
		}
		var body map[string]string
		json.NewDecoder(r.Body).Decode(&body)
		if body["name"] != "new-project" {
			t.Errorf("expected name=new-project, got %s", body["name"])
		}
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(map[string]any{
			"data": models.Project{ID: "p2", Name: "new-project"},
			"meta": map[string]any{"request_id": "r2"},
		})
	}))
	defer srv.Close()

	c := NewClient(srv.URL, "key")
	proj, err := CreateProject(c, "new-project", "desc", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if proj.ID != "p2" {
		t.Errorf("expected p2, got %s", proj.ID)
	}
}

func TestProjectsDelete(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete {
			t.Errorf("expected DELETE, got %s", r.Method)
		}
		if r.URL.Path != "/api/v1/projects/p1" {
			t.Errorf("expected /api/v1/projects/p1, got %s", r.URL.Path)
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()

	c := NewClient(srv.URL, "key")
	err := DeleteProject(c, "p1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestProjectsGet(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/projects/p1" {
			t.Errorf("expected /api/v1/projects/p1, got %s", r.URL.Path)
		}
		json.NewEncoder(w).Encode(map[string]any{
			"data": models.Project{ID: "p1", Name: "my-project"},
			"meta": map[string]any{"request_id": "r3"},
		})
	}))
	defer srv.Close()

	c := NewClient(srv.URL, "key")
	proj, err := GetProject(c, "p1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if proj.Name != "my-project" {
		t.Errorf("expected my-project, got %s", proj.Name)
	}
}

func TestProjectsUpdate(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPut || r.URL.Path != "/api/v1/projects/p1" {
			t.Errorf("unexpected %s %s", r.Method, r.URL.Path)
		}
		var body map[string]any
		json.NewDecoder(r.Body).Decode(&body)
		if body["name"] != "updated-name" {
			t.Errorf("expected name=updated-name, got %v", body["name"])
		}
		json.NewEncoder(w).Encode(map[string]any{
			"data": models.Project{ID: "p1", Name: "updated-name"},
			"meta": map[string]any{"request_id": "r4"},
		})
	}))
	defer srv.Close()

	c := NewClient(srv.URL, "key")
	proj, err := UpdateProject(c, "p1", map[string]any{"name": "updated-name"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if proj.Name != "updated-name" {
		t.Errorf("expected updated-name, got %s", proj.Name)
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run:
```bash
go test ./internal/cli/... -v -run TestProjects
```
Expected: FAIL — functions not defined.

- [ ] **Step 3: Implement projects API functions and Cobra commands**

Create `internal/cli/projects.go`:

```go
package cli

import (
	"fmt"

	"github.com/sentioxyz/changelogue/internal/models"
	"github.com/spf13/cobra"
)

// --- API functions ---

func ListProjects(c *Client, page, perPage int) ([]models.Project, Meta, error) {
	path := fmt.Sprintf("/api/v1/projects?page=%d&per_page=%d", page, perPage)
	resp, err := c.Get(path)
	if err != nil {
		return nil, Meta{}, err
	}
	defer resp.Body.Close()
	if err := CheckResponse(resp); err != nil {
		return nil, Meta{}, err
	}
	var result APIResponse[[]models.Project]
	if err := DecodeJSON(resp, &result); err != nil {
		return nil, Meta{}, err
	}
	return result.Data, result.Meta, nil
}

func GetProject(c *Client, id string) (*models.Project, error) {
	resp, err := c.Get("/api/v1/projects/" + id)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if err := CheckResponse(resp); err != nil {
		return nil, err
	}
	var result APIResponse[models.Project]
	if err := DecodeJSON(resp, &result); err != nil {
		return nil, err
	}
	return &result.Data, nil
}

func CreateProject(c *Client, name, description, agentPrompt string) (*models.Project, error) {
	body := map[string]string{"name": name}
	if description != "" {
		body["description"] = description
	}
	if agentPrompt != "" {
		body["agent_prompt"] = agentPrompt
	}
	resp, err := c.Post("/api/v1/projects", body)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if err := CheckResponse(resp); err != nil {
		return nil, err
	}
	var result APIResponse[models.Project]
	if err := DecodeJSON(resp, &result); err != nil {
		return nil, err
	}
	return &result.Data, nil
}

func UpdateProject(c *Client, id string, fields map[string]any) (*models.Project, error) {
	resp, err := c.Put("/api/v1/projects/"+id, fields)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if err := CheckResponse(resp); err != nil {
		return nil, err
	}
	var result APIResponse[models.Project]
	if err := DecodeJSON(resp, &result); err != nil {
		return nil, err
	}
	return &result.Data, nil
}

func DeleteProject(c *Client, id string) error {
	resp, err := c.Delete("/api/v1/projects/" + id)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	return CheckResponse(resp)
}

// --- Cobra commands ---

// NewProjectsCmd returns the "projects" command group.
// clientFn is called at execution time to build the client from resolved flags.
func NewProjectsCmd(clientFn func() *Client, jsonFlag *bool) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "projects",
		Short: "Manage projects",
		Long:  "Create, list, update, and delete projects.\n\nExamples:\n  clog projects list\n  clog projects get <id>\n  clog projects create --name \"My Project\"",
	}

	// Pagination flags (only apply to list)
	var page, perPage int

	// --- list ---
	listCmd := &cobra.Command{
		Use:   "list",
		Short: "List all projects",
		Example: "  clog projects list\n  clog projects list --page 2 --per-page 10",
		RunE: func(cmd *cobra.Command, args []string) error {
			projects, meta, err := ListProjects(clientFn(), page, perPage)
			if err != nil {
				return err
			}
			if *jsonFlag {
				RenderJSON(map[string]any{"data": projects, "meta": meta})
				return nil
			}
			rows := make([][]string, len(projects))
			for i, p := range projects {
				rows[i] = []string{p.ID, p.Name, Truncate(p.Description, 40), FormatTime(p.CreatedAt.Format("2006-01-02T15:04:05"))}
			}
			RenderTable([]string{"ID", "NAME", "DESCRIPTION", "CREATED"}, rows)
			fmt.Printf("\nShowing page %d (total: %d)\n", meta.Page, meta.Total)
			return nil
		},
	}
	listCmd.Flags().IntVar(&page, "page", 1, "Page number")
	listCmd.Flags().IntVar(&perPage, "per-page", 25, "Items per page")

	// --- get ---
	getCmd := &cobra.Command{
		Use:   "get <id>",
		Short: "Get project details",
		Args:  cobra.ExactArgs(1),
		Example: "  clog projects get abc-123",
		RunE: func(cmd *cobra.Command, args []string) error {
			proj, err := GetProject(clientFn(), args[0])
			if err != nil {
				return err
			}
			if *jsonFlag {
				RenderJSON(proj)
				return nil
			}
			rows := [][]string{{proj.ID, proj.Name, Truncate(proj.Description, 40), FormatTime(proj.CreatedAt.Format("2006-01-02T15:04:05"))}}
			RenderTable([]string{"ID", "NAME", "DESCRIPTION", "CREATED"}, rows)
			return nil
		},
	}

	// --- create ---
	var createName, createDesc, createPrompt string
	createCmd := &cobra.Command{
		Use:   "create",
		Short: "Create a new project",
		Example: "  clog projects create --name \"My Project\" --description \"Tracks postgres releases\"",
		RunE: func(cmd *cobra.Command, args []string) error {
			proj, err := CreateProject(clientFn(), createName, createDesc, createPrompt)
			if err != nil {
				return err
			}
			if *jsonFlag {
				RenderJSON(proj)
				return nil
			}
			fmt.Printf("Created project %s (%s)\n", proj.Name, proj.ID)
			return nil
		},
	}
	createCmd.Flags().StringVar(&createName, "name", "", "Project name (required)")
	createCmd.MarkFlagRequired("name")
	createCmd.Flags().StringVar(&createDesc, "description", "", "Project description")
	createCmd.Flags().StringVar(&createPrompt, "agent-prompt", "", "Agent prompt for AI analysis")

	// --- update ---
	var updateName, updateDesc, updatePrompt string
	updateCmd := &cobra.Command{
		Use:   "update <id>",
		Short: "Update a project",
		Args:  cobra.ExactArgs(1),
		Example: "  clog projects update abc-123 --name \"New Name\"",
		RunE: func(cmd *cobra.Command, args []string) error {
			fields := make(map[string]any)
			if cmd.Flags().Changed("name") {
				fields["name"] = updateName
			}
			if cmd.Flags().Changed("description") {
				fields["description"] = updateDesc
			}
			if cmd.Flags().Changed("agent-prompt") {
				fields["agent_prompt"] = updatePrompt
			}
			if len(fields) == 0 {
				return fmt.Errorf("no fields to update — use --name, --description, or --agent-prompt")
			}
			proj, err := UpdateProject(clientFn(), args[0], fields)
			if err != nil {
				return err
			}
			if *jsonFlag {
				RenderJSON(proj)
				return nil
			}
			fmt.Printf("Updated project %s (%s)\n", proj.Name, proj.ID)
			return nil
		},
	}
	updateCmd.Flags().StringVar(&updateName, "name", "", "New project name")
	updateCmd.Flags().StringVar(&updateDesc, "description", "", "New description")
	updateCmd.Flags().StringVar(&updatePrompt, "agent-prompt", "", "New agent prompt")

	// --- delete ---
	deleteCmd := &cobra.Command{
		Use:   "delete <id>",
		Short: "Delete a project",
		Args:  cobra.ExactArgs(1),
		Example: "  clog projects delete abc-123",
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := DeleteProject(clientFn(), args[0]); err != nil {
				return err
			}
			fmt.Println("Deleted project", args[0])
			return nil
		},
	}

	cmd.AddCommand(listCmd, getCmd, createCmd, updateCmd, deleteCmd)
	return cmd
}
```

- [ ] **Step 4: Register projects command in main.go**

In `cmd/cli/main.go`, in the `init()` function, add after the `// Resource subcommands` comment:

```go
	rootCmd.AddCommand(cli.NewProjectsCmd(newClient, &jsonOut))
```

Note: `newClient` is the package-level function defined above. All subsequent tasks add their `AddCommand` call in the same place.

- [ ] **Step 5: Run tests to verify they pass**

Run:
```bash
go test ./internal/cli/... -v -run TestProjects
```
Expected: All PASS.

- [ ] **Step 6: Build and verify help output**

Run:
```bash
make cli && ./clog projects --help
```
Expected: Shows projects subcommands with descriptions and examples.

Run:
```bash
./clog projects lst
```
Expected: Error with "Did you mean: list"

Run:
```bash
./clog projects create
```
Expected: Error: required flag "--name" not set, with usage hint.

- [ ] **Step 7: Commit**

```bash
git add internal/cli/projects.go internal/cli/projects_test.go cmd/cli/main.go
git commit -m "feat(cli): add projects subcommand (list, get, create, update, delete)"
```

---

### Task 4: Sources subcommand

**Files:**
- Create: `internal/cli/sources.go`
- Create: `internal/cli/sources_test.go`
- Modify: `cmd/cli/main.go` (register sources command)

- [ ] **Step 1: Write failing test for sources commands**

Create `internal/cli/sources_test.go`:

```go
package cli

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/sentioxyz/changelogue/internal/models"
)

func TestSourcesList(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/projects/proj-1/sources" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		json.NewEncoder(w).Encode(map[string]any{
			"data": []models.Source{{ID: "s1", ProjectID: "proj-1", Provider: "dockerhub", Repository: "library/postgres"}},
			"meta": map[string]any{"request_id": "r1", "page": 1, "per_page": 25, "total": 1},
		})
	}))
	defer srv.Close()

	c := NewClient(srv.URL, "key")
	sources, _, err := ListSources(c, "proj-1", 1, 25)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(sources) != 1 || sources[0].Provider != "dockerhub" {
		t.Errorf("unexpected sources: %+v", sources)
	}
}

func TestSourcesCreate(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/api/v1/projects/proj-1/sources" {
			t.Errorf("unexpected %s %s", r.Method, r.URL.Path)
		}
		var body map[string]any
		json.NewDecoder(r.Body).Decode(&body)
		if body["provider"] != "dockerhub" {
			t.Errorf("expected provider=dockerhub, got %v", body["provider"])
		}
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(map[string]any{
			"data": models.Source{ID: "s2", Provider: "dockerhub", Repository: "library/postgres"},
			"meta": map[string]any{"request_id": "r2"},
		})
	}))
	defer srv.Close()

	c := NewClient(srv.URL, "key")
	src, err := CreateSource(c, "proj-1", map[string]any{"provider": "dockerhub", "repository": "library/postgres"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if src.ID != "s2" {
		t.Errorf("expected s2, got %s", src.ID)
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run:
```bash
go test ./internal/cli/... -v -run TestSources
```
Expected: FAIL.

- [ ] **Step 3: Implement sources API functions and Cobra commands**

Create `internal/cli/sources.go`:

```go
package cli

import (
	"fmt"

	"github.com/sentioxyz/changelogue/internal/models"
	"github.com/spf13/cobra"
)

func ListSources(c *Client, projectID string, page, perPage int) ([]models.Source, Meta, error) {
	path := fmt.Sprintf("/api/v1/projects/%s/sources?page=%d&per_page=%d", projectID, page, perPage)
	resp, err := c.Get(path)
	if err != nil {
		return nil, Meta{}, err
	}
	defer resp.Body.Close()
	if err := CheckResponse(resp); err != nil {
		return nil, Meta{}, err
	}
	var result APIResponse[[]models.Source]
	if err := DecodeJSON(resp, &result); err != nil {
		return nil, Meta{}, err
	}
	return result.Data, result.Meta, nil
}

func GetSource(c *Client, id string) (*models.Source, error) {
	resp, err := c.Get("/api/v1/sources/" + id)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if err := CheckResponse(resp); err != nil {
		return nil, err
	}
	var result APIResponse[models.Source]
	if err := DecodeJSON(resp, &result); err != nil {
		return nil, err
	}
	return &result.Data, nil
}

func CreateSource(c *Client, projectID string, body map[string]any) (*models.Source, error) {
	resp, err := c.Post("/api/v1/projects/"+projectID+"/sources", body)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if err := CheckResponse(resp); err != nil {
		return nil, err
	}
	var result APIResponse[models.Source]
	if err := DecodeJSON(resp, &result); err != nil {
		return nil, err
	}
	return &result.Data, nil
}

func UpdateSource(c *Client, id string, fields map[string]any) (*models.Source, error) {
	resp, err := c.Put("/api/v1/sources/"+id, fields)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if err := CheckResponse(resp); err != nil {
		return nil, err
	}
	var result APIResponse[models.Source]
	if err := DecodeJSON(resp, &result); err != nil {
		return nil, err
	}
	return &result.Data, nil
}

func DeleteSource(c *Client, id string) error {
	resp, err := c.Delete("/api/v1/sources/" + id)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	return CheckResponse(resp)
}

func NewSourcesCmd(clientFn func() *Client, jsonFlag *bool) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "sources",
		Short: "Manage ingestion sources",
		Long:  "Add, list, update, and remove sources for a project.\n\nProviders: dockerhub, github, ecr, gitlab, pypi, npm\n\nExamples:\n  clog sources list --project <id>\n  clog sources create --project <id> --provider dockerhub --repository library/postgres",
	}

	var page, perPage int

	// --- list ---
	var listProjectID string
	listCmd := &cobra.Command{
		Use:   "list",
		Short: "List sources for a project",
		Example: "  clog sources list --project abc-123",
		RunE: func(cmd *cobra.Command, args []string) error {
			sources, meta, err := ListSources(clientFn(), listProjectID, page, perPage)
			if err != nil {
				return err
			}
			if *jsonFlag {
				RenderJSON(map[string]any{"data": sources, "meta": meta})
				return nil
			}
			rows := make([][]string, len(sources))
			for i, s := range sources {
				enabled := "yes"
				if !s.Enabled {
					enabled = "no"
				}
				rows[i] = []string{s.ID, s.Provider, s.Repository, enabled, fmt.Sprintf("%ds", s.PollIntervalSeconds)}
			}
			RenderTable([]string{"ID", "PROVIDER", "REPOSITORY", "ENABLED", "POLL INTERVAL"}, rows)
			fmt.Printf("\nShowing page %d (total: %d)\n", meta.Page, meta.Total)
			return nil
		},
	}
	listCmd.Flags().StringVar(&listProjectID, "project", "", "Project ID (required)")
	listCmd.MarkFlagRequired("project")
	listCmd.Flags().IntVar(&page, "page", 1, "Page number")
	listCmd.Flags().IntVar(&perPage, "per-page", 25, "Items per page")

	// --- get ---
	getCmd := &cobra.Command{
		Use:   "get <id>",
		Short: "Get source details",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			src, err := GetSource(clientFn(), args[0])
			if err != nil {
				return err
			}
			if *jsonFlag {
				RenderJSON(src)
				return nil
			}
			enabled := "yes"
			if !src.Enabled {
				enabled = "no"
			}
			rows := [][]string{{src.ID, src.Provider, src.Repository, enabled, fmt.Sprintf("%ds", src.PollIntervalSeconds)}}
			RenderTable([]string{"ID", "PROVIDER", "REPOSITORY", "ENABLED", "POLL INTERVAL"}, rows)
			return nil
		},
	}

	// --- create ---
	var createProjectID, createProvider, createRepo, createFilterInclude, createFilterExclude string
	var createPollInterval int
	var createExcludePrerelease bool
	createCmd := &cobra.Command{
		Use:   "create",
		Short: "Add a new source",
		Example: "  clog sources create --project abc-123 --provider dockerhub --repository library/postgres",
		RunE: func(cmd *cobra.Command, args []string) error {
			body := map[string]any{
				"provider":   createProvider,
				"repository": createRepo,
			}
			if createPollInterval > 0 {
				body["poll_interval_seconds"] = createPollInterval
			}
			if createFilterInclude != "" {
				body["version_filter_include"] = createFilterInclude
			}
			if createFilterExclude != "" {
				body["version_filter_exclude"] = createFilterExclude
			}
			if createExcludePrerelease {
				body["exclude_prereleases"] = true
			}
			src, err := CreateSource(clientFn(), createProjectID, body)
			if err != nil {
				return err
			}
			if *jsonFlag {
				RenderJSON(src)
				return nil
			}
			fmt.Printf("Created source %s (%s/%s)\n", src.ID, src.Provider, src.Repository)
			return nil
		},
	}
	createCmd.Flags().StringVar(&createProjectID, "project", "", "Project ID (required)")
	createCmd.MarkFlagRequired("project")
	createCmd.Flags().StringVar(&createProvider, "provider", "", "Provider: dockerhub, github, ecr, gitlab, pypi, npm (required)")
	createCmd.MarkFlagRequired("provider")
	createCmd.Flags().StringVar(&createRepo, "repository", "", "Repository identifier (required)")
	createCmd.MarkFlagRequired("repository")
	createCmd.Flags().IntVar(&createPollInterval, "poll-interval", 0, "Poll interval in seconds")
	createCmd.Flags().StringVar(&createFilterInclude, "filter-include", "", "Version include regex pattern")
	createCmd.Flags().StringVar(&createFilterExclude, "filter-exclude", "", "Version exclude regex pattern")
	createCmd.Flags().BoolVar(&createExcludePrerelease, "exclude-prereleases", false, "Exclude prerelease versions")

	// --- update ---
	var updateProvider, updateRepo, updateFilterInclude, updateFilterExclude string
	var updatePollInterval int
	var updateEnabled, updateExcludePrerelease string
	updateCmd := &cobra.Command{
		Use:   "update <id>",
		Short: "Update a source",
		Args:  cobra.ExactArgs(1),
		Example: "  clog sources update src-123 --repository library/redis",
		RunE: func(cmd *cobra.Command, args []string) error {
			fields := make(map[string]any)
			if cmd.Flags().Changed("provider") {
				fields["provider"] = updateProvider
			}
			if cmd.Flags().Changed("repository") {
				fields["repository"] = updateRepo
			}
			if cmd.Flags().Changed("poll-interval") {
				fields["poll_interval_seconds"] = updatePollInterval
			}
			if cmd.Flags().Changed("enabled") {
				fields["enabled"] = updateEnabled == "true"
			}
			if cmd.Flags().Changed("filter-include") {
				fields["version_filter_include"] = updateFilterInclude
			}
			if cmd.Flags().Changed("filter-exclude") {
				fields["version_filter_exclude"] = updateFilterExclude
			}
			if cmd.Flags().Changed("exclude-prereleases") {
				fields["exclude_prereleases"] = updateExcludePrerelease == "true"
			}
			if len(fields) == 0 {
				return fmt.Errorf("no fields to update")
			}
			src, err := UpdateSource(clientFn(), args[0], fields)
			if err != nil {
				return err
			}
			if *jsonFlag {
				RenderJSON(src)
				return nil
			}
			fmt.Printf("Updated source %s\n", src.ID)
			return nil
		},
	}
	updateCmd.Flags().StringVar(&updateProvider, "provider", "", "Provider")
	updateCmd.Flags().StringVar(&updateRepo, "repository", "", "Repository")
	updateCmd.Flags().IntVar(&updatePollInterval, "poll-interval", 0, "Poll interval in seconds")
	updateCmd.Flags().StringVar(&updateEnabled, "enabled", "", "Enable/disable (true/false)")
	updateCmd.Flags().StringVar(&updateFilterInclude, "filter-include", "", "Version include regex")
	updateCmd.Flags().StringVar(&updateFilterExclude, "filter-exclude", "", "Version exclude regex")
	updateCmd.Flags().StringVar(&updateExcludePrerelease, "exclude-prereleases", "", "Exclude prereleases (true/false)")

	// --- delete ---
	deleteCmd := &cobra.Command{
		Use:   "delete <id>",
		Short: "Remove a source",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := DeleteSource(clientFn(), args[0]); err != nil {
				return err
			}
			fmt.Println("Deleted source", args[0])
			return nil
		},
	}

	cmd.AddCommand(listCmd, getCmd, createCmd, updateCmd, deleteCmd)
	return cmd
}
```

- [ ] **Step 4: Register sources command in `cmd/cli/main.go`**

In `init()`, add alongside the projects registration:
```go
	rootCmd.AddCommand(cli.NewSourcesCmd(newClient, &jsonOut))
```

- [ ] **Step 5: Run tests**

Run:
```bash
go test ./internal/cli/... -v -run TestSources
```
Expected: All PASS.

- [ ] **Step 6: Commit**

```bash
git add internal/cli/sources.go internal/cli/sources_test.go cmd/cli/main.go
git commit -m "feat(cli): add sources subcommand (list, get, create, update, delete)"
```

---

### Task 5: Releases subcommand

**Files:**
- Create: `internal/cli/releases.go`
- Create: `internal/cli/releases_test.go`
- Modify: `cmd/cli/main.go` (register releases command)

- [ ] **Step 1: Write failing test for releases commands**

Create `internal/cli/releases_test.go`:

```go
package cli

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/sentioxyz/changelogue/internal/models"
)

func TestReleasesListAll(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/releases" {
			t.Errorf("expected /api/v1/releases, got %s", r.URL.Path)
		}
		json.NewEncoder(w).Encode(map[string]any{
			"data": []models.Release{{ID: "r1", Version: "1.0.0"}},
			"meta": map[string]any{"request_id": "r1", "page": 1, "per_page": 25, "total": 1},
		})
	}))
	defer srv.Close()

	c := NewClient(srv.URL, "key")
	releases, _, err := ListReleases(c, "", "", false, 1, 25)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(releases) != 1 || releases[0].Version != "1.0.0" {
		t.Errorf("unexpected releases: %+v", releases)
	}
}

func TestReleasesListBySource(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/sources/s1/releases" {
			t.Errorf("expected /api/v1/sources/s1/releases, got %s", r.URL.Path)
		}
		json.NewEncoder(w).Encode(map[string]any{
			"data": []models.Release{{ID: "r2", Version: "2.0.0", SourceID: "s1"}},
			"meta": map[string]any{"request_id": "r2", "page": 1, "per_page": 25, "total": 1},
		})
	}))
	defer srv.Close()

	c := NewClient(srv.URL, "key")
	releases, _, err := ListReleases(c, "s1", "", false, 1, 25)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(releases) != 1 || releases[0].SourceID != "s1" {
		t.Errorf("unexpected releases: %+v", releases)
	}
}

func TestReleasesListByProject(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/projects/p1/releases" {
			t.Errorf("expected /api/v1/projects/p1/releases, got %s", r.URL.Path)
		}
		json.NewEncoder(w).Encode(map[string]any{
			"data": []models.Release{},
			"meta": map[string]any{"request_id": "r3", "page": 1, "per_page": 25, "total": 0},
		})
	}))
	defer srv.Close()

	c := NewClient(srv.URL, "key")
	releases, _, err := ListReleases(c, "", "p1", false, 1, 25)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(releases) != 0 {
		t.Errorf("expected 0 releases, got %d", len(releases))
	}
}

func TestReleasesGet(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/releases/r1" {
			t.Errorf("expected /api/v1/releases/r1, got %s", r.URL.Path)
		}
		json.NewEncoder(w).Encode(map[string]any{
			"data": models.Release{ID: "r1", Version: "3.0.0"},
			"meta": map[string]any{"request_id": "r4"},
		})
	}))
	defer srv.Close()

	c := NewClient(srv.URL, "key")
	rel, err := GetRelease(c, "r1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rel.Version != "3.0.0" {
		t.Errorf("expected 3.0.0, got %s", rel.Version)
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run:
```bash
go test ./internal/cli/... -v -run TestReleases
```
Expected: FAIL.

- [ ] **Step 3: Implement releases API functions and Cobra commands**

Create `internal/cli/releases.go`:

```go
package cli

import (
	"fmt"

	"github.com/sentioxyz/changelogue/internal/models"
	"github.com/spf13/cobra"
)

func ListReleases(c *Client, sourceID, projectID string, includeExcluded bool, page, perPage int) ([]models.Release, Meta, error) {
	var path string
	switch {
	case sourceID != "":
		path = fmt.Sprintf("/api/v1/sources/%s/releases", sourceID)
	case projectID != "":
		path = fmt.Sprintf("/api/v1/projects/%s/releases", projectID)
	default:
		path = "/api/v1/releases"
	}
	path += fmt.Sprintf("?page=%d&per_page=%d", page, perPage)
	if includeExcluded {
		path += "&include_excluded=true"
	}
	resp, err := c.Get(path)
	if err != nil {
		return nil, Meta{}, err
	}
	defer resp.Body.Close()
	if err := CheckResponse(resp); err != nil {
		return nil, Meta{}, err
	}
	var result APIResponse[[]models.Release]
	if err := DecodeJSON(resp, &result); err != nil {
		return nil, Meta{}, err
	}
	return result.Data, result.Meta, nil
}

func GetRelease(c *Client, id string) (*models.Release, error) {
	resp, err := c.Get("/api/v1/releases/" + id)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if err := CheckResponse(resp); err != nil {
		return nil, err
	}
	var result APIResponse[models.Release]
	if err := DecodeJSON(resp, &result); err != nil {
		return nil, err
	}
	return &result.Data, nil
}

func NewReleasesCmd(clientFn func() *Client, jsonFlag *bool) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "releases",
		Short: "Browse releases",
		Long:  "List and view releases.\n\nExamples:\n  clog releases list\n  clog releases list --project <id>\n  clog releases list --source <id>\n  clog releases get <id>",
	}

	var page, perPage int
	var sourceID, projectID string
	var includeExcluded bool

	listCmd := &cobra.Command{
		Use:   "list",
		Short: "List releases",
		Example: "  clog releases list\n  clog releases list --project abc-123\n  clog releases list --source src-123 --include-excluded",
		RunE: func(cmd *cobra.Command, args []string) error {
			releases, meta, err := ListReleases(clientFn(), sourceID, projectID, includeExcluded, page, perPage)
			if err != nil {
				return err
			}
			if *jsonFlag {
				RenderJSON(map[string]any{"data": releases, "meta": meta})
				return nil
			}
			rows := make([][]string, len(releases))
			for i, r := range releases {
				releasedAt := ""
				if r.ReleasedAt != nil {
					releasedAt = FormatTime(r.ReleasedAt.Format("2006-01-02T15:04:05"))
				}
				rows[i] = []string{r.ID, r.Version, r.Provider, r.Repository, releasedAt, r.SemanticReleaseStatus}
			}
			RenderTable([]string{"ID", "VERSION", "PROVIDER", "REPOSITORY", "RELEASED", "SEMANTIC STATUS"}, rows)
			fmt.Printf("\nShowing page %d (total: %d)\n", meta.Page, meta.Total)
			return nil
		},
	}
	listCmd.Flags().StringVar(&sourceID, "source", "", "Filter by source ID")
	listCmd.Flags().StringVar(&projectID, "project", "", "Filter by project ID")
	listCmd.Flags().BoolVar(&includeExcluded, "include-excluded", false, "Include releases filtered out by version patterns")
	listCmd.Flags().IntVar(&page, "page", 1, "Page number")
	listCmd.Flags().IntVar(&perPage, "per-page", 25, "Items per page")

	getCmd := &cobra.Command{
		Use:   "get <id>",
		Short: "Get release details",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			rel, err := GetRelease(clientFn(), args[0])
			if err != nil {
				return err
			}
			if *jsonFlag {
				RenderJSON(rel)
				return nil
			}
			releasedAt := ""
			if rel.ReleasedAt != nil {
				releasedAt = FormatTime(rel.ReleasedAt.Format("2006-01-02T15:04:05"))
			}
			rows := [][]string{{rel.ID, rel.Version, rel.Provider, rel.Repository, releasedAt}}
			RenderTable([]string{"ID", "VERSION", "PROVIDER", "REPOSITORY", "RELEASED"}, rows)
			return nil
		},
	}

	cmd.AddCommand(listCmd, getCmd)
	return cmd
}
```

- [ ] **Step 4: Register in `cmd/cli/main.go`**

```go
	rootCmd.AddCommand(cli.NewReleasesCmd(newClient, &jsonOut))
```

- [ ] **Step 5: Run tests**

Run:
```bash
go test ./internal/cli/... -v -run TestReleases
```
Expected: All PASS.

- [ ] **Step 6: Commit**

```bash
git add internal/cli/releases.go internal/cli/releases_test.go cmd/cli/main.go
git commit -m "feat(cli): add releases subcommand (list, get) with path-based routing"
```

---

### Task 6: Channels subcommand

**Files:**
- Create: `internal/cli/channels.go`
- Create: `internal/cli/channels_test.go`
- Modify: `cmd/cli/main.go` (register channels command)

- [ ] **Step 1: Write failing test for channels commands**

Create `internal/cli/channels_test.go`:

```go
package cli

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/sentioxyz/changelogue/internal/models"
)

func TestChannelsList(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/channels" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		json.NewEncoder(w).Encode(map[string]any{
			"data": []models.NotificationChannel{{ID: "ch1", Name: "my-slack", Type: "slack"}},
			"meta": map[string]any{"request_id": "r1", "page": 1, "per_page": 25, "total": 1},
		})
	}))
	defer srv.Close()

	c := NewClient(srv.URL, "key")
	channels, _, err := ListChannels(c, 1, 25)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(channels) != 1 || channels[0].Type != "slack" {
		t.Errorf("unexpected channels: %+v", channels)
	}
}

func TestChannelsTest(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/api/v1/channels/ch1/test" {
			t.Errorf("unexpected %s %s", r.Method, r.URL.Path)
		}
		json.NewEncoder(w).Encode(map[string]any{
			"data": map[string]string{"status": "ok"},
			"meta": map[string]any{"request_id": "r2"},
		})
	}))
	defer srv.Close()

	c := NewClient(srv.URL, "key")
	err := TestChannel(c, "ch1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run:
```bash
go test ./internal/cli/... -v -run TestChannels
```
Expected: FAIL.

- [ ] **Step 3: Implement channels API functions and Cobra commands**

Create `internal/cli/channels.go`:

```go
package cli

import (
	"encoding/json"
	"fmt"

	"github.com/sentioxyz/changelogue/internal/models"
	"github.com/spf13/cobra"
)

func ListChannels(c *Client, page, perPage int) ([]models.NotificationChannel, Meta, error) {
	path := fmt.Sprintf("/api/v1/channels?page=%d&per_page=%d", page, perPage)
	resp, err := c.Get(path)
	if err != nil {
		return nil, Meta{}, err
	}
	defer resp.Body.Close()
	if err := CheckResponse(resp); err != nil {
		return nil, Meta{}, err
	}
	var result APIResponse[[]models.NotificationChannel]
	if err := DecodeJSON(resp, &result); err != nil {
		return nil, Meta{}, err
	}
	return result.Data, result.Meta, nil
}

func GetChannel(c *Client, id string) (*models.NotificationChannel, error) {
	resp, err := c.Get("/api/v1/channels/" + id)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if err := CheckResponse(resp); err != nil {
		return nil, err
	}
	var result APIResponse[models.NotificationChannel]
	if err := DecodeJSON(resp, &result); err != nil {
		return nil, err
	}
	return &result.Data, nil
}

func CreateChannel(c *Client, name, chType string, config json.RawMessage) (*models.NotificationChannel, error) {
	body := map[string]any{"name": name, "type": chType, "config": config}
	resp, err := c.Post("/api/v1/channels", body)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if err := CheckResponse(resp); err != nil {
		return nil, err
	}
	var result APIResponse[models.NotificationChannel]
	if err := DecodeJSON(resp, &result); err != nil {
		return nil, err
	}
	return &result.Data, nil
}

func UpdateChannel(c *Client, id string, fields map[string]any) (*models.NotificationChannel, error) {
	resp, err := c.Put("/api/v1/channels/"+id, fields)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if err := CheckResponse(resp); err != nil {
		return nil, err
	}
	var result APIResponse[models.NotificationChannel]
	if err := DecodeJSON(resp, &result); err != nil {
		return nil, err
	}
	return &result.Data, nil
}

func DeleteChannel(c *Client, id string) error {
	resp, err := c.Delete("/api/v1/channels/" + id)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	return CheckResponse(resp)
}

func TestChannel(c *Client, id string) error {
	resp, err := c.Post("/api/v1/channels/"+id+"/test", nil)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	return CheckResponse(resp)
}

func NewChannelsCmd(clientFn func() *Client, jsonFlag *bool) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "channels",
		Short: "Manage notification channels",
		Long:  "Create, list, update, delete, and test notification channels.\n\nTypes: slack, discord, email, webhook\n\nExamples:\n  clog channels list\n  clog channels create --name my-slack --type slack --config '{\"webhook_url\":\"https://...\"}'\n  clog channels test <id>",
	}

	var page, perPage int

	listCmd := &cobra.Command{
		Use:     "list",
		Short:   "List all notification channels",
		Example: "  clog channels list",
		RunE: func(cmd *cobra.Command, args []string) error {
			channels, meta, err := ListChannels(clientFn(), page, perPage)
			if err != nil {
				return err
			}
			if *jsonFlag {
				RenderJSON(map[string]any{"data": channels, "meta": meta})
				return nil
			}
			rows := make([][]string, len(channels))
			for i, ch := range channels {
				rows[i] = []string{ch.ID, ch.Name, ch.Type, FormatTime(ch.CreatedAt.Format("2006-01-02T15:04:05"))}
			}
			RenderTable([]string{"ID", "NAME", "TYPE", "CREATED"}, rows)
			fmt.Printf("\nShowing page %d (total: %d)\n", meta.Page, meta.Total)
			return nil
		},
	}
	listCmd.Flags().IntVar(&page, "page", 1, "Page number")
	listCmd.Flags().IntVar(&perPage, "per-page", 25, "Items per page")

	getCmd := &cobra.Command{
		Use:   "get <id>",
		Short: "Get channel details",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ch, err := GetChannel(clientFn(), args[0])
			if err != nil {
				return err
			}
			if *jsonFlag {
				RenderJSON(ch)
				return nil
			}
			rows := [][]string{{ch.ID, ch.Name, ch.Type, string(ch.Config)}}
			RenderTable([]string{"ID", "NAME", "TYPE", "CONFIG"}, rows)
			return nil
		},
	}

	var createName, createType, createConfig string
	createCmd := &cobra.Command{
		Use:   "create",
		Short: "Create a notification channel",
		Example: "  clog channels create --name my-slack --type slack --config '{\"webhook_url\":\"https://hooks.slack.com/...\"}'",
		RunE: func(cmd *cobra.Command, args []string) error {
			if !json.Valid([]byte(createConfig)) {
				return fmt.Errorf("--config must be valid JSON")
			}
			ch, err := CreateChannel(clientFn(), createName, createType, json.RawMessage(createConfig))
			if err != nil {
				return err
			}
			if *jsonFlag {
				RenderJSON(ch)
				return nil
			}
			fmt.Printf("Created channel %s (%s, type=%s)\n", ch.Name, ch.ID, ch.Type)
			return nil
		},
	}
	createCmd.Flags().StringVar(&createName, "name", "", "Channel name (required)")
	createCmd.MarkFlagRequired("name")
	createCmd.Flags().StringVar(&createType, "type", "", "Channel type: slack, discord, email, webhook (required)")
	createCmd.MarkFlagRequired("type")
	createCmd.Flags().StringVar(&createConfig, "config", "", "Channel config as JSON (required)")
	createCmd.MarkFlagRequired("config")

	var updateName, updateType, updateConfig string
	updateCmd := &cobra.Command{
		Use:   "update <id>",
		Short: "Update a channel",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			fields := make(map[string]any)
			if cmd.Flags().Changed("name") {
				fields["name"] = updateName
			}
			if cmd.Flags().Changed("type") {
				fields["type"] = updateType
			}
			if cmd.Flags().Changed("config") {
				if !json.Valid([]byte(updateConfig)) {
					return fmt.Errorf("--config must be valid JSON")
				}
				fields["config"] = json.RawMessage(updateConfig)
			}
			if len(fields) == 0 {
				return fmt.Errorf("no fields to update — use --name, --type, or --config")
			}
			ch, err := UpdateChannel(clientFn(), args[0], fields)
			if err != nil {
				return err
			}
			if *jsonFlag {
				RenderJSON(ch)
				return nil
			}
			fmt.Printf("Updated channel %s\n", ch.ID)
			return nil
		},
	}
	updateCmd.Flags().StringVar(&updateName, "name", "", "Channel name")
	updateCmd.Flags().StringVar(&updateType, "type", "", "Channel type")
	updateCmd.Flags().StringVar(&updateConfig, "config", "", "Channel config as JSON")

	deleteCmd := &cobra.Command{
		Use:   "delete <id>",
		Short: "Delete a channel",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := DeleteChannel(clientFn(), args[0]); err != nil {
				return err
			}
			fmt.Println("Deleted channel", args[0])
			return nil
		},
	}

	testCmd := &cobra.Command{
		Use:   "test <id>",
		Short: "Send a test notification",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := TestChannel(clientFn(), args[0]); err != nil {
				return err
			}
			fmt.Println("Test notification sent successfully")
			return nil
		},
	}

	cmd.AddCommand(listCmd, getCmd, createCmd, updateCmd, deleteCmd, testCmd)
	return cmd
}
```

- [ ] **Step 4: Register in `cmd/cli/main.go`**

```go
	rootCmd.AddCommand(cli.NewChannelsCmd(newClient, &jsonOut))
```

- [ ] **Step 5: Run tests**

Run:
```bash
go test ./internal/cli/... -v -run TestChannels
```
Expected: All PASS.

- [ ] **Step 6: Commit**

```bash
git add internal/cli/channels.go internal/cli/channels_test.go cmd/cli/main.go
git commit -m "feat(cli): add channels subcommand (list, get, create, update, delete, test)"
```

---

### Task 7: Subscriptions subcommand

**Files:**
- Create: `internal/cli/subscriptions.go`
- Create: `internal/cli/subscriptions_test.go`
- Modify: `cmd/cli/main.go` (register subscriptions command)

- [ ] **Step 1: Write failing test for subscriptions commands**

Create `internal/cli/subscriptions_test.go`:

```go
package cli

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/sentioxyz/changelogue/internal/models"
)

func TestSubscriptionsList(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/subscriptions" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		json.NewEncoder(w).Encode(map[string]any{
			"data": []models.Subscription{{ID: "sub1", ChannelID: "ch1", Type: "source_release"}},
			"meta": map[string]any{"request_id": "r1", "page": 1, "per_page": 25, "total": 1},
		})
	}))
	defer srv.Close()

	c := NewClient(srv.URL, "key")
	subs, _, err := ListSubscriptions(c, 1, 25)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(subs) != 1 || subs[0].Type != "source_release" {
		t.Errorf("unexpected subscriptions: %+v", subs)
	}
}

func TestSubscriptionsCreate(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/api/v1/subscriptions" {
			t.Errorf("unexpected %s %s", r.Method, r.URL.Path)
		}
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(map[string]any{
			"data": models.Subscription{ID: "sub2", ChannelID: "ch1", Type: "semantic_release"},
			"meta": map[string]any{"request_id": "r2"},
		})
	}))
	defer srv.Close()

	c := NewClient(srv.URL, "key")
	sub, err := CreateSubscription(c, map[string]any{"channel_id": "ch1", "type": "semantic_release", "project_id": "p1"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if sub.ID != "sub2" {
		t.Errorf("expected sub2, got %s", sub.ID)
	}
}

func TestSubscriptionsBatchDelete(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete || r.URL.Path != "/api/v1/subscriptions/batch" {
			t.Errorf("unexpected %s %s", r.Method, r.URL.Path)
		}
		var body map[string][]string
		json.NewDecoder(r.Body).Decode(&body)
		if len(body["ids"]) != 2 {
			t.Errorf("expected 2 ids, got %d", len(body["ids"]))
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()

	c := NewClient(srv.URL, "key")
	err := BatchDeleteSubscriptions(c, []string{"sub1", "sub2"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run:
```bash
go test ./internal/cli/... -v -run TestSubscriptions
```
Expected: FAIL.

- [ ] **Step 3: Implement subscriptions API functions and Cobra commands**

Create `internal/cli/subscriptions.go`:

```go
package cli

import (
	"fmt"
	"strings"

	"github.com/sentioxyz/changelogue/internal/models"
	"github.com/spf13/cobra"
)

func ListSubscriptions(c *Client, page, perPage int) ([]models.Subscription, Meta, error) {
	path := fmt.Sprintf("/api/v1/subscriptions?page=%d&per_page=%d", page, perPage)
	resp, err := c.Get(path)
	if err != nil {
		return nil, Meta{}, err
	}
	defer resp.Body.Close()
	if err := CheckResponse(resp); err != nil {
		return nil, Meta{}, err
	}
	var result APIResponse[[]models.Subscription]
	if err := DecodeJSON(resp, &result); err != nil {
		return nil, Meta{}, err
	}
	return result.Data, result.Meta, nil
}

func GetSubscription(c *Client, id string) (*models.Subscription, error) {
	resp, err := c.Get("/api/v1/subscriptions/" + id)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if err := CheckResponse(resp); err != nil {
		return nil, err
	}
	var result APIResponse[models.Subscription]
	if err := DecodeJSON(resp, &result); err != nil {
		return nil, err
	}
	return &result.Data, nil
}

func CreateSubscription(c *Client, body map[string]any) (*models.Subscription, error) {
	resp, err := c.Post("/api/v1/subscriptions", body)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if err := CheckResponse(resp); err != nil {
		return nil, err
	}
	var result APIResponse[models.Subscription]
	if err := DecodeJSON(resp, &result); err != nil {
		return nil, err
	}
	return &result.Data, nil
}

func UpdateSubscription(c *Client, id string, fields map[string]any) (*models.Subscription, error) {
	resp, err := c.Put("/api/v1/subscriptions/"+id, fields)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if err := CheckResponse(resp); err != nil {
		return nil, err
	}
	var result APIResponse[models.Subscription]
	if err := DecodeJSON(resp, &result); err != nil {
		return nil, err
	}
	return &result.Data, nil
}

func DeleteSubscription(c *Client, id string) error {
	resp, err := c.Delete("/api/v1/subscriptions/" + id)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	return CheckResponse(resp)
}

func BatchCreateSubscriptions(c *Client, body map[string]any) ([]models.Subscription, error) {
	resp, err := c.Post("/api/v1/subscriptions/batch", body)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if err := CheckResponse(resp); err != nil {
		return nil, err
	}
	var result APIResponse[[]models.Subscription]
	if err := DecodeJSON(resp, &result); err != nil {
		return nil, err
	}
	return result.Data, nil
}

func BatchDeleteSubscriptions(c *Client, ids []string) error {
	body := map[string]any{"ids": ids}
	resp, err := c.DeleteWithBody("/api/v1/subscriptions/batch", body)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	return CheckResponse(resp)
}

func NewSubscriptionsCmd(clientFn func() *Client, jsonFlag *bool) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "subscriptions",
		Short: "Manage subscriptions",
		Long:  "Create, list, update, and delete notification subscriptions.\n\nTypes:\n  source_release    — raw release notifications from a specific source\n  semantic_release  — AI-analyzed release notifications for a project\n\nExamples:\n  clog subscriptions list\n  clog subscriptions create --channel <id> --type source_release --source <id>\n  clog subscriptions create --channel <id> --type semantic_release --project <id>",
	}

	var page, perPage int

	// --- list ---
	listCmd := &cobra.Command{
		Use:     "list",
		Short:   "List all subscriptions",
		Example: "  clog subscriptions list",
		RunE: func(cmd *cobra.Command, args []string) error {
			subs, meta, err := ListSubscriptions(clientFn(), page, perPage)
			if err != nil {
				return err
			}
			if *jsonFlag {
				RenderJSON(map[string]any{"data": subs, "meta": meta})
				return nil
			}
			rows := make([][]string, len(subs))
			for i, s := range subs {
				target := ""
				if s.SourceID != nil {
					target = "source:" + *s.SourceID
				} else if s.ProjectID != nil {
					target = "project:" + *s.ProjectID
				}
				rows[i] = []string{s.ID, s.ChannelID, s.Type, target, s.VersionFilter}
			}
			RenderTable([]string{"ID", "CHANNEL", "TYPE", "TARGET", "VERSION FILTER"}, rows)
			fmt.Printf("\nShowing page %d (total: %d)\n", meta.Page, meta.Total)
			return nil
		},
	}
	listCmd.Flags().IntVar(&page, "page", 1, "Page number")
	listCmd.Flags().IntVar(&perPage, "per-page", 25, "Items per page")

	// --- get ---
	getCmd := &cobra.Command{
		Use:   "get <id>",
		Short: "Get subscription details",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			sub, err := GetSubscription(clientFn(), args[0])
			if err != nil {
				return err
			}
			if *jsonFlag {
				RenderJSON(sub)
				return nil
			}
			target := ""
			if sub.SourceID != nil {
				target = "source:" + *sub.SourceID
			} else if sub.ProjectID != nil {
				target = "project:" + *sub.ProjectID
			}
			rows := [][]string{{sub.ID, sub.ChannelID, sub.Type, target, sub.VersionFilter}}
			RenderTable([]string{"ID", "CHANNEL", "TYPE", "TARGET", "VERSION FILTER"}, rows)
			return nil
		},
	}

	// --- create ---
	var createChannelID, createType, createSourceID, createProjectID, createVersionFilter string
	createCmd := &cobra.Command{
		Use:   "create",
		Short: "Create a subscription",
		Example: "  clog subscriptions create --channel ch1 --type source_release --source src1\n  clog subscriptions create --channel ch1 --type semantic_release --project p1",
		RunE: func(cmd *cobra.Command, args []string) error {
			body := map[string]any{
				"channel_id": createChannelID,
				"type":       createType,
			}
			if createSourceID != "" {
				body["source_id"] = createSourceID
			}
			if createProjectID != "" {
				body["project_id"] = createProjectID
			}
			if createVersionFilter != "" {
				body["version_filter"] = createVersionFilter
			}
			sub, err := CreateSubscription(clientFn(), body)
			if err != nil {
				return err
			}
			if *jsonFlag {
				RenderJSON(sub)
				return nil
			}
			fmt.Printf("Created subscription %s\n", sub.ID)
			return nil
		},
	}
	createCmd.Flags().StringVar(&createChannelID, "channel", "", "Channel ID (required)")
	createCmd.MarkFlagRequired("channel")
	createCmd.Flags().StringVar(&createType, "type", "", "Type: source_release or semantic_release (required)")
	createCmd.MarkFlagRequired("type")
	createCmd.Flags().StringVar(&createSourceID, "source", "", "Source ID (for source_release type)")
	createCmd.Flags().StringVar(&createProjectID, "project", "", "Project ID (for semantic_release type)")
	createCmd.Flags().StringVar(&createVersionFilter, "version-filter", "", "Version regex filter")

	// --- update ---
	var updateVersionFilter, updateChannelID string
	updateCmd := &cobra.Command{
		Use:   "update <id>",
		Short: "Update a subscription",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			fields := make(map[string]any)
			if cmd.Flags().Changed("channel") {
				fields["channel_id"] = updateChannelID
			}
			if cmd.Flags().Changed("version-filter") {
				fields["version_filter"] = updateVersionFilter
			}
			if len(fields) == 0 {
				return fmt.Errorf("no fields to update — use --channel or --version-filter")
			}
			sub, err := UpdateSubscription(clientFn(), args[0], fields)
			if err != nil {
				return err
			}
			if *jsonFlag {
				RenderJSON(sub)
				return nil
			}
			fmt.Printf("Updated subscription %s\n", sub.ID)
			return nil
		},
	}
	updateCmd.Flags().StringVar(&updateChannelID, "channel", "", "New channel ID")
	updateCmd.Flags().StringVar(&updateVersionFilter, "version-filter", "", "New version filter")

	// --- delete ---
	deleteCmd := &cobra.Command{
		Use:   "delete <id>",
		Short: "Delete a subscription",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := DeleteSubscription(clientFn(), args[0]); err != nil {
				return err
			}
			fmt.Println("Deleted subscription", args[0])
			return nil
		},
	}

	// --- batch-create ---
	var batchChannelID, batchType, batchVersionFilter string
	var batchProjectIDs, batchSourceIDs string
	batchCreateCmd := &cobra.Command{
		Use:   "batch-create",
		Short: "Batch create subscriptions",
		Example: "  clog subscriptions batch-create --channel ch1 --type semantic_release --project-ids p1,p2,p3",
		RunE: func(cmd *cobra.Command, args []string) error {
			body := map[string]any{
				"channel_id": batchChannelID,
				"type":       batchType,
			}
			if batchProjectIDs != "" {
				body["project_ids"] = strings.Split(batchProjectIDs, ",")
			}
			if batchSourceIDs != "" {
				body["source_ids"] = strings.Split(batchSourceIDs, ",")
			}
			if batchVersionFilter != "" {
				body["version_filter"] = batchVersionFilter
			}
			subs, err := BatchCreateSubscriptions(clientFn(), body)
			if err != nil {
				return err
			}
			if *jsonFlag {
				RenderJSON(subs)
				return nil
			}
			fmt.Printf("Created %d subscriptions\n", len(subs))
			return nil
		},
	}
	batchCreateCmd.Flags().StringVar(&batchChannelID, "channel", "", "Channel ID (required)")
	batchCreateCmd.MarkFlagRequired("channel")
	batchCreateCmd.Flags().StringVar(&batchType, "type", "", "Type: source_release or semantic_release (required)")
	batchCreateCmd.MarkFlagRequired("type")
	batchCreateCmd.Flags().StringVar(&batchProjectIDs, "project-ids", "", "Comma-separated project IDs")
	batchCreateCmd.Flags().StringVar(&batchSourceIDs, "source-ids", "", "Comma-separated source IDs")
	batchCreateCmd.Flags().StringVar(&batchVersionFilter, "version-filter", "", "Version regex filter")

	// --- batch-delete ---
	var batchDeleteIDs string
	batchDeleteCmd := &cobra.Command{
		Use:   "batch-delete",
		Short: "Batch delete subscriptions",
		Example: "  clog subscriptions batch-delete --ids sub1,sub2,sub3",
		RunE: func(cmd *cobra.Command, args []string) error {
			ids := strings.Split(batchDeleteIDs, ",")
			if err := BatchDeleteSubscriptions(clientFn(), ids); err != nil {
				return err
			}
			fmt.Printf("Deleted %d subscriptions\n", len(ids))
			return nil
		},
	}
	batchDeleteCmd.Flags().StringVar(&batchDeleteIDs, "ids", "", "Comma-separated subscription IDs (required)")
	batchDeleteCmd.MarkFlagRequired("ids")

	cmd.AddCommand(listCmd, getCmd, createCmd, updateCmd, deleteCmd, batchCreateCmd, batchDeleteCmd)
	return cmd
}
```

- [ ] **Step 4: Register in `cmd/cli/main.go`**

```go
	rootCmd.AddCommand(cli.NewSubscriptionsCmd(newClient, &jsonOut))
```

- [ ] **Step 5: Run tests**

Run:
```bash
go test ./internal/cli/... -v -run TestSubscriptions
```
Expected: All PASS.

- [ ] **Step 6: Commit**

```bash
git add internal/cli/subscriptions.go internal/cli/subscriptions_test.go cmd/cli/main.go
git commit -m "feat(cli): add subscriptions subcommand (list, get, create, update, delete, batch-create, batch-delete)"
```

---

### Task 8: Update Makefile, README, and run full build

**Files:**
- Modify: `Makefile` (should already have `cli` target from Task 1; update `clean` target)
- Modify: `README.md` (add CLI section)

- [ ] **Step 1: Update Makefile clean target**

In `Makefile`, update the `clean` target to also remove the `clog` binary:

```makefile
clean:
	rm -f $(BINARY) clog
	docker compose down -v
```

Also ensure `VERSION ?= dev` is placed with the other configuration variables (after `BINARY := changelogue`), and `cli` is added to the `.PHONY` line:

```makefile
.PHONY: up down db-reset build run run-auth dev test vet lint coverage \
        frontend-install frontend-dev frontend-build \
        integration-test agent-dev cli clean
```

- [ ] **Step 2: Update README.md with CLI section**

Add a new `## CLI` section after the `## Quick start` section in `README.md`:

```markdown
## CLI

The `clog` CLI manages Changelogue resources from the command line.

### Install

```bash
make cli    # builds ./clog binary
```

### Configuration

```bash
export CHANGELOGUE_SERVER=http://localhost:8080    # server URL
export CHANGELOGUE_API_KEY=rg_live_abc123...       # API key
```

Or pass per-command:

```bash
clog --server http://myserver:8080 --api-key rg_live_... projects list
```

### Commands

```
clog projects list|get|create|update|delete      Manage projects
clog sources list|get|create|update|delete        Manage ingestion sources
clog releases list|get                            Browse releases
clog channels list|get|create|update|delete|test  Manage notification channels
clog subscriptions list|get|create|update|delete  Manage subscriptions
clog subscriptions batch-create|batch-delete      Batch operations
clog version                                      Print CLI version
```

Use `--json` on any command for machine-readable output. Use `--help` on any command for detailed usage and examples.
```

Also add to the `## Project structure` section:

```
cmd/
  cli/                 CLI binary (clog) — REST API client
```

Also add to the `## Useful commands` section:

```bash
make cli                # build clog CLI binary
```

- [ ] **Step 3: Run full test suite**

Run:
```bash
go test ./internal/cli/... -v
```
Expected: All tests PASS.

- [ ] **Step 4: Build both binaries**

Run:
```bash
make build && make cli
```
Expected: Both `changelogue` and `clog` binaries created.

- [ ] **Step 5: Verify CLI help and suggestions**

Run:
```bash
./clog --help
./clog projet
./clog projects lst
./clog projects create
./clog channels
```
Expected: Each shows appropriate help, suggestions, or required-flag errors.

- [ ] **Step 6: Commit**

```bash
git add README.md Makefile
git commit -m "docs: add CLI section to README and update Makefile clean target"
```

---

### Task 9: Final verification and cleanup

- [ ] **Step 1: Run `go vet`**

Run:
```bash
go vet ./...
```
Expected: No errors.

- [ ] **Step 2: Run full test suite**

Run:
```bash
go test ./...
```
Expected: All tests PASS (including existing tests — no regressions).

- [ ] **Step 3: Verify binary runs end-to-end**

Run:
```bash
./clog version
./clog --help
./clog projects --help
./clog sources --help
./clog releases --help
./clog channels --help
./clog subscriptions --help
```
Expected: All show correct help output with examples and command suggestions.

- [ ] **Step 4: Verify AI-friendly hints work**

Run:
```bash
./clog projet
./clog projects lst
./clog sources create --project abc --repo test
```
Expected:
- "projet" suggests "projects"
- "lst" suggests "list"
- "--repo" suggests "--repository"

- [ ] **Step 5: Final commit if any cleanup needed**

```bash
git add -A && git commit -m "chore(cli): final cleanup and verification"
```
