package cli

// Client is a thin HTTP wrapper for the Changelogue REST API.
type Client struct {
	BaseURL string
	APIKey  string
}

// NewClient creates a new API client.
func NewClient(baseURL, apiKey string) *Client {
	return &Client{BaseURL: baseURL, APIKey: apiKey}
}
