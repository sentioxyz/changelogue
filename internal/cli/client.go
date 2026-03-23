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
	Err struct {
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
