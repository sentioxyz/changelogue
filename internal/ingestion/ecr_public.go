package ingestion

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

const defaultECRPublicURL = "https://public.ecr.aws"
const ecrGalleryAPI = "https://api.us-east-1.gallery.ecr.aws"

// ECRPublicSource polls AWS ECR Public Gallery for new image tags
// using the ECR Public Gallery API which returns pushed timestamps directly.
type ECRPublicSource struct {
	client     *http.Client
	repository string // "registry_alias/repo_name", e.g. "i6b2w2n6/op-node"
	baseURL    string
	galleryURL string
	sourceID   string
}

func NewECRPublicSource(client *http.Client, repository string, sourceID string) *ECRPublicSource {
	return &ECRPublicSource{
		client:     client,
		repository: repository,
		baseURL:    defaultECRPublicURL,
		galleryURL: ecrGalleryAPI,
		sourceID:   sourceID,
	}
}

func (s *ECRPublicSource) Name() string    { return "ecr-public" }
func (s *ECRPublicSource) SourceID() string { return s.sourceID }

// ecrImageTagDetail represents a single tag from the ECR Gallery API response.
type ecrImageTagDetail struct {
	ImageTag string `json:"imageTag"`
	Detail   struct {
		ImagePushedAt string `json:"imagePushedAt"`
	} `json:"imageDetail"`
}

func (s *ECRPublicSource) FetchNewReleases(ctx context.Context) ([]IngestionResult, error) {
	// Parse "alias/repo" into separate fields for the Gallery API.
	alias, repo, err := splitRepository(s.repository)
	if err != nil {
		return nil, err
	}

	var allTags []ecrImageTagDetail
	var nextToken string

	for {
		tags, token, err := s.fetchImageTags(ctx, alias, repo, nextToken)
		if err != nil {
			return nil, fmt.Errorf("fetch image tags: %w", err)
		}
		allTags = append(allTags, tags...)
		if token == "" {
			break
		}
		nextToken = token
	}

	results := make([]IngestionResult, 0, len(allTags))
	for _, tag := range allTags {
		ts, _ := time.Parse(time.RFC3339Nano, tag.Detail.ImagePushedAt)
		if ts.IsZero() {
			ts = time.Now()
		}
		results = append(results, IngestionResult{
			Repository: s.repository,
			RawVersion: tag.ImageTag,
			Timestamp:  ts,
		})
	}
	return results, nil
}

// fetchImageTags calls the ECR Public Gallery API to list image tags with pushed timestamps.
func (s *ECRPublicSource) fetchImageTags(ctx context.Context, alias, repo, nextToken string) ([]ecrImageTagDetail, string, error) {
	payload := map[string]interface{}{
		"registryAliasName": alias,
		"repositoryName":    repo,
		"maxResults":        100,
	}
	if nextToken != "" {
		payload["nextToken"] = nextToken
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return nil, "", err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, s.galleryURL+"/describeImageTags", bytes.NewReader(body))
	if err != nil {
		return nil, "", err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, "", fmt.Errorf("gallery API status %d", resp.StatusCode)
	}

	var result struct {
		ImageTagDetails []ecrImageTagDetail `json:"imageTagDetails"`
		NextToken       string              `json:"nextToken"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, "", err
	}
	return result.ImageTagDetails, result.NextToken, nil
}

// splitRepository splits "alias/repo" into its two components.
func splitRepository(repository string) (alias, repo string, err error) {
	for i, c := range repository {
		if c == '/' {
			return repository[:i], repository[i+1:], nil
		}
	}
	return "", "", fmt.Errorf("ecr-public repository must be alias/repo, got %q", repository)
}
