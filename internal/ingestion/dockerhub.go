package ingestion

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

const defaultDockerHubURL = "https://hub.docker.com"

// DockerHubSource polls Docker Hub for new image tags.
type DockerHubSource struct {
	client     *http.Client
	repository string
	baseURL    string
	sourceID   string
}

func NewDockerHubSource(client *http.Client, repository string, sourceID string) *DockerHubSource {
	return &DockerHubSource{
		client:     client,
		repository: repository,
		baseURL:    defaultDockerHubURL,
		sourceID:   sourceID,
	}
}

func (s *DockerHubSource) Name() string      { return "dockerhub" }
func (s *DockerHubSource) SourceID() string   { return s.sourceID }

func (s *DockerHubSource) FetchNewReleases(ctx context.Context) ([]IngestionResult, error) {
	url := fmt.Sprintf("%s/v2/repositories/%s/tags/?page_size=25&ordering=last_updated",
		s.baseURL, s.repository)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetch tags: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, httpStatusError(resp)
	}

	var body struct {
		Results []struct {
			Name        string `json:"name"`
			LastUpdated string `json:"last_updated"`
		} `json:"results"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	results := make([]IngestionResult, 0, len(body.Results))
	for _, tag := range body.Results {
		ts, _ := time.Parse(time.RFC3339Nano, tag.LastUpdated)
		results = append(results, IngestionResult{
			Repository: s.repository,
			RawVersion: tag.Name,
			Timestamp:  ts,
		})
	}
	return results, nil
}
