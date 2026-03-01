package ingestion

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestECRPublicSourceName(t *testing.T) {
	src := NewECRPublicSource(http.DefaultClient, "i6b2w2n6/op-node", "")
	if got := src.Name(); got != "ecr-public" {
		t.Errorf("Name() = %q, want %q", got, "ecr-public")
	}
}

func TestECRPublicFetchNewReleases(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/describeImageTags":
			w.Write([]byte(`{
				"imageTagDetails": [
					{
						"imageTag": "v1.0.0",
						"imageDetail": {"imagePushedAt": "2025-10-10T14:38:14.287Z"}
					},
					{
						"imageTag": "v1.1.0",
						"imageDetail": {"imagePushedAt": "2025-11-20T08:30:00Z"}
					}
				]
			}`))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer srv.Close()

	src := NewECRPublicSource(srv.Client(), "i6b2w2n6/op-node", "src-123")
	src.galleryURL = srv.URL

	results, err := src.FetchNewReleases(context.Background())
	if err != nil {
		t.Fatalf("FetchNewReleases: %v", err)
	}

	if len(results) != 2 {
		t.Fatalf("got %d results, want 2", len(results))
	}
	if results[0].RawVersion != "v1.0.0" {
		t.Errorf("results[0].RawVersion = %q, want %q", results[0].RawVersion, "v1.0.0")
	}
	if results[0].Repository != "i6b2w2n6/op-node" {
		t.Errorf("results[0].Repository = %q, want %q", results[0].Repository, "i6b2w2n6/op-node")
	}

	// Verify timestamps come from imagePushedAt, not time.Now()
	expected, _ := time.Parse(time.RFC3339Nano, "2025-10-10T14:38:14.287Z")
	if !results[0].Timestamp.Equal(expected) {
		t.Errorf("results[0].Timestamp = %v, want %v", results[0].Timestamp, expected)
	}
	expected2, _ := time.Parse(time.RFC3339, "2025-11-20T08:30:00Z")
	if !results[1].Timestamp.Equal(expected2) {
		t.Errorf("results[1].Timestamp = %v, want %v", results[1].Timestamp, expected2)
	}
}

func TestECRPublicFetchEmpty(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"imageTagDetails": []}`))
	}))
	defer srv.Close()

	src := NewECRPublicSource(srv.Client(), "i6b2w2n6/op-node", "")
	src.galleryURL = srv.URL

	results, err := src.FetchNewReleases(context.Background())
	if err != nil {
		t.Fatalf("FetchNewReleases: %v", err)
	}
	if len(results) != 0 {
		t.Fatalf("got %d results, want 0", len(results))
	}
}

func TestECRPublicPagination(t *testing.T) {
	callCount := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		callCount++
		if callCount == 1 {
			w.Write([]byte(`{
				"imageTagDetails": [
					{"imageTag": "v1.0.0", "imageDetail": {"imagePushedAt": "2025-01-01T00:00:00Z"}}
				],
				"nextToken": "page2"
			}`))
		} else {
			w.Write([]byte(`{
				"imageTagDetails": [
					{"imageTag": "v2.0.0", "imageDetail": {"imagePushedAt": "2025-02-01T00:00:00Z"}}
				]
			}`))
		}
	}))
	defer srv.Close()

	src := NewECRPublicSource(srv.Client(), "i6b2w2n6/op-node", "")
	src.galleryURL = srv.URL

	results, err := src.FetchNewReleases(context.Background())
	if err != nil {
		t.Fatalf("FetchNewReleases: %v", err)
	}
	if len(results) != 2 {
		t.Fatalf("got %d results, want 2", len(results))
	}
	if results[0].RawVersion != "v1.0.0" {
		t.Errorf("results[0].RawVersion = %q, want %q", results[0].RawVersion, "v1.0.0")
	}
	if results[1].RawVersion != "v2.0.0" {
		t.Errorf("results[1].RawVersion = %q, want %q", results[1].RawVersion, "v2.0.0")
	}
}

func TestECRPublicGalleryAPIFailure(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	src := NewECRPublicSource(srv.Client(), "i6b2w2n6/op-node", "")
	src.galleryURL = srv.URL

	_, err := src.FetchNewReleases(context.Background())
	if err == nil {
		t.Fatal("expected error for API failure, got nil")
	}
}

func TestECRPublicInvalidRepository(t *testing.T) {
	src := NewECRPublicSource(http.DefaultClient, "no-slash", "")
	_, err := src.FetchNewReleases(context.Background())
	if err == nil {
		t.Fatal("expected error for invalid repository format, got nil")
	}
}
