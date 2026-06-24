package ingestion

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const defaultGHCRURL = "https://ghcr.io"

// GHCRSource polls GitHub Container Registry for image tags.
type GHCRSource struct {
	client     *http.Client
	repository string
	baseURL    string
	sourceID   string
}

func NewGHCRSource(client *http.Client, repository string, sourceID string) *GHCRSource {
	return &GHCRSource{
		client:     client,
		repository: strings.TrimPrefix(strings.TrimSpace(repository), "ghcr.io/"),
		baseURL:    defaultGHCRURL,
		sourceID:   sourceID,
	}
}

func (s *GHCRSource) Name() string     { return "ghcr" }
func (s *GHCRSource) SourceID() string { return s.sourceID }

func (s *GHCRSource) FetchNewReleases(ctx context.Context) ([]IngestionResult, error) {
	if err := validateGHCRRepository(s.repository); err != nil {
		return nil, err
	}

	var allTags []string
	last := ""
	for {
		body, nextLast, err := s.fetchTagsPage(ctx, last)
		if err != nil {
			return nil, err
		}
		allTags = append(allTags, body.Tags...)
		if nextLast == "" {
			break
		}
		last = nextLast
	}

	now := time.Now()
	results := make([]IngestionResult, 0, len(allTags))
	for _, tag := range allTags {
		if tag == "" {
			continue
		}
		results = append(results, IngestionResult{
			Repository: s.repository,
			RawVersion: tag,
			Metadata: map[string]string{
				"release_url": fmt.Sprintf("https://ghcr.io/%s:%s", s.repository, url.PathEscape(tag)),
			},
			Timestamp: now,
		})
	}
	return results, nil
}

type ghcrTagsResponse struct {
	Name string   `json:"name"`
	Tags []string `json:"tags"`
}

func (s *GHCRSource) fetchTagsPage(ctx context.Context, last string) (ghcrTagsResponse, string, error) {
	req, err := s.newTagsRequest(ctx, last, "")
	if err != nil {
		return ghcrTagsResponse{}, "", err
	}

	resp, err := s.client.Do(req)
	if err != nil {
		return ghcrTagsResponse{}, "", fmt.Errorf("fetch tags: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusUnauthorized {
		challenge := resp.Header.Get("WWW-Authenticate")
		token, err := s.fetchBearerToken(ctx, challenge)
		if err != nil {
			return ghcrTagsResponse{}, "", err
		}
		req, err = s.newTagsRequest(ctx, last, token)
		if err != nil {
			return ghcrTagsResponse{}, "", err
		}
		resp, err = s.client.Do(req)
		if err != nil {
			return ghcrTagsResponse{}, "", fmt.Errorf("fetch tags: %w", err)
		}
		defer resp.Body.Close()
	}

	if resp.StatusCode != http.StatusOK {
		return ghcrTagsResponse{}, "", httpStatusError(resp)
	}

	var body ghcrTagsResponse
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return ghcrTagsResponse{}, "", fmt.Errorf("decode response: %w", err)
	}
	return body, parseRegistryNextLast(resp.Header.Get("Link")), nil
}

func (s *GHCRSource) newTagsRequest(ctx context.Context, last string, token string) (*http.Request, error) {
	u, err := url.Parse(strings.TrimRight(s.baseURL, "/") + "/v2/" + s.repository + "/tags/list")
	if err != nil {
		return nil, fmt.Errorf("parse tags URL: %w", err)
	}
	q := u.Query()
	q.Set("n", "100")
	if last != "" {
		q.Set("last", last)
	}
	u.RawQuery = q.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Accept", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	return req, nil
}

func (s *GHCRSource) fetchBearerToken(ctx context.Context, challenge string) (string, error) {
	realm, service, scope := parseBearerChallenge(challenge)
	if realm == "" {
		return "", fmt.Errorf("ghcr auth challenge missing bearer realm")
	}
	if scope == "" {
		scope = "repository:" + s.repository + ":pull"
	}

	u, err := url.Parse(realm)
	if err != nil {
		return "", fmt.Errorf("parse token URL: %w", err)
	}
	q := u.Query()
	if service != "" {
		q.Set("service", service)
	}
	q.Set("scope", scope)
	u.RawQuery = q.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return "", fmt.Errorf("create token request: %w", err)
	}
	resp, err := s.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("fetch auth token: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", httpStatusError(resp)
	}
	var body struct {
		Token       string `json:"token"`
		AccessToken string `json:"access_token"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return "", fmt.Errorf("decode token response: %w", err)
	}
	if body.Token != "" {
		return body.Token, nil
	}
	if body.AccessToken != "" {
		return body.AccessToken, nil
	}
	return "", fmt.Errorf("ghcr token response missing token")
}

func validateGHCRRepository(repository string) error {
	parts := strings.Split(strings.TrimSpace(repository), "/")
	if len(parts) < 2 || parts[0] == "" || parts[1] == "" {
		return fmt.Errorf("ghcr repository must be owner/image, got %q", repository)
	}
	return nil
}

func parseBearerChallenge(challenge string) (realm, service, scope string) {
	challenge = strings.TrimSpace(challenge)
	if !strings.HasPrefix(strings.ToLower(challenge), "bearer ") {
		return "", "", ""
	}
	params := strings.TrimSpace(challenge[len("Bearer "):])
	for _, part := range strings.Split(params, ",") {
		key, value, ok := strings.Cut(strings.TrimSpace(part), "=")
		if !ok {
			continue
		}
		value = strings.Trim(value, `"`)
		switch strings.ToLower(key) {
		case "realm":
			realm = value
		case "service":
			service = value
		case "scope":
			scope = value
		}
	}
	return realm, service, scope
}

func parseRegistryNextLast(link string) string {
	for _, part := range strings.Split(link, ",") {
		if !strings.Contains(part, `rel="next"`) {
			continue
		}
		start := strings.Index(part, "<")
		end := strings.Index(part, ">")
		if start < 0 || end <= start {
			continue
		}
		linkURL, err := url.Parse(strings.TrimSpace(part[start+1 : end]))
		if err != nil {
			continue
		}
		return linkURL.Query().Get("last")
	}
	return ""
}
