package ingestion

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const defaultGHCRURL = "https://ghcr.io"

const ghcrManifestAccept = "application/vnd.oci.image.index.v1+json, application/vnd.docker.distribution.manifest.list.v2+json, application/vnd.oci.image.manifest.v1+json, application/vnd.docker.distribution.manifest.v2+json"

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

	results := make([]IngestionResult, 0, len(allTags))
	for _, tag := range allTags {
		if tag == "" {
			continue
		}
		ts, metadata := s.fetchTagMetadata(ctx, tag)
		metadata["release_url"] = fmt.Sprintf("https://ghcr.io/%s:%s", s.repository, url.PathEscape(tag))
		results = append(results, IngestionResult{
			Repository: s.repository,
			RawVersion: tag,
			Metadata:   metadata,
			Timestamp:  ts,
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

func (s *GHCRSource) fetchTagMetadata(ctx context.Context, tag string) (time.Time, map[string]string) {
	metadata := map[string]string{}
	manifest, err := s.fetchManifest(ctx, tag)
	if err != nil {
		metadata["timestamp_source"] = "unavailable"
		metadata["timestamp_error"] = err.Error()
		return time.Time{}, metadata
	}

	if ts, ok := timestampFromAnnotations(manifest.Annotations); ok {
		metadata["timestamp_source"] = "manifest_annotation"
		return ts, metadata
	}

	if manifest.Config != nil && manifest.Config.Digest != "" {
		if config, err := s.fetchConfig(ctx, manifest.Config.Digest); err == nil {
			if ts, ok := timestampFromConfig(config); ok {
				metadata["timestamp_source"] = "image_config"
				return ts, metadata
			}
		} else {
			metadata["timestamp_error"] = err.Error()
		}
	}

	var latest time.Time
	for _, child := range manifest.Manifests {
		if ts, ok := timestampFromAnnotations(child.Annotations); ok && ts.After(latest) {
			latest = ts
		}
	}
	if !latest.IsZero() {
		metadata["timestamp_source"] = "manifest_list_annotation"
		return latest, metadata
	}

	metadata["timestamp_source"] = "unavailable"
	return time.Time{}, metadata
}

type ghcrManifest struct {
	Config *struct {
		Digest string `json:"digest"`
	} `json:"config"`
	Manifests []struct {
		Annotations map[string]string `json:"annotations"`
	} `json:"manifests"`
	Annotations map[string]string `json:"annotations"`
}

type ghcrConfig struct {
	Created string `json:"created"`
	Config  struct {
		Labels map[string]string `json:"Labels"`
	} `json:"config"`
	History []struct {
		Created string `json:"created"`
	} `json:"history"`
}

func (s *GHCRSource) fetchManifest(ctx context.Context, reference string) (ghcrManifest, error) {
	resp, err := s.doRegistryRequest(ctx, func(token string) (*http.Request, error) {
		u := strings.TrimRight(s.baseURL, "/") + "/v2/" + s.repository + "/manifests/" + url.PathEscape(reference)
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
		if err != nil {
			return nil, fmt.Errorf("create manifest request: %w", err)
		}
		req.Header.Set("Accept", ghcrManifestAccept)
		if token != "" {
			req.Header.Set("Authorization", "Bearer "+token)
		}
		return req, nil
	})
	if err != nil {
		return ghcrManifest{}, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return ghcrManifest{}, httpStatusError(resp)
	}
	var manifest ghcrManifest
	if err := json.NewDecoder(resp.Body).Decode(&manifest); err != nil {
		return ghcrManifest{}, fmt.Errorf("decode manifest: %w", err)
	}
	return manifest, nil
}

func (s *GHCRSource) fetchConfig(ctx context.Context, digest string) (ghcrConfig, error) {
	resp, err := s.doRegistryRequest(ctx, func(token string) (*http.Request, error) {
		u := strings.TrimRight(s.baseURL, "/") + "/v2/" + s.repository + "/blobs/" + digest
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
		if err != nil {
			return nil, fmt.Errorf("create config request: %w", err)
		}
		if token != "" {
			req.Header.Set("Authorization", "Bearer "+token)
		}
		return req, nil
	})
	if err != nil {
		return ghcrConfig{}, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return ghcrConfig{}, httpStatusError(resp)
	}
	var config ghcrConfig
	if err := json.NewDecoder(resp.Body).Decode(&config); err != nil {
		return ghcrConfig{}, fmt.Errorf("decode config: %w", err)
	}
	return config, nil
}

func (s *GHCRSource) doRegistryRequest(ctx context.Context, newRequest func(token string) (*http.Request, error)) (*http.Response, error) {
	resp, err := doRequestWithToken(newRequest, "", s.client)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusUnauthorized {
		return resp, nil
	}

	challenge := resp.Header.Get("WWW-Authenticate")
	_, _ = io.Copy(io.Discard, resp.Body)
	resp.Body.Close()
	token, err := s.fetchBearerToken(ctx, challenge)
	if err != nil {
		return nil, err
	}
	return doRequestWithToken(newRequest, token, s.client)
}

func doRequestWithToken(newRequest func(token string) (*http.Request, error), token string, client *http.Client) (*http.Response, error) {
	req, err := newRequest(token)
	if err != nil {
		return nil, err
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("registry request: %w", err)
	}
	return resp, nil
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

func timestampFromAnnotations(annotations map[string]string) (time.Time, bool) {
	for _, key := range []string{
		"org.opencontainers.image.created",
		"org.label-schema.build-date",
		"org.label-schema.created",
		"build-date",
	} {
		if ts, ok := parseGHCROCIUTCTime(annotations[key]); ok {
			return ts, true
		}
	}
	return time.Time{}, false
}

func timestampFromConfig(config ghcrConfig) (time.Time, bool) {
	if ts, ok := parseGHCROCIUTCTime(config.Created); ok {
		return ts, true
	}
	if ts, ok := timestampFromAnnotations(config.Config.Labels); ok {
		return ts, true
	}
	var latest time.Time
	for _, h := range config.History {
		if ts, ok := parseGHCROCIUTCTime(h.Created); ok && ts.After(latest) {
			latest = ts
		}
	}
	if !latest.IsZero() {
		return latest, true
	}
	return time.Time{}, false
}

func parseGHCROCIUTCTime(value string) (time.Time, bool) {
	value = strings.TrimSpace(value)
	if value == "" {
		return time.Time{}, false
	}
	for _, layout := range []string{time.RFC3339Nano, time.RFC3339, "2006-01-02"} {
		if ts, err := time.Parse(layout, value); err == nil {
			return ts, true
		}
	}
	return time.Time{}, false
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
