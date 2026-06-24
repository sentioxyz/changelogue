package ingestion

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestGHCRSourceName(t *testing.T) {
	src := NewGHCRSource(http.DefaultClient, "sentioxyz/changelogue", "")
	if got := src.Name(); got != "ghcr" {
		t.Errorf("Name() = %q, want %q", got, "ghcr")
	}
}

func TestGHCRFetchNewReleases(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v2/sentioxyz/changelogue/tags/list":
			if got := r.URL.Query().Get("n"); got != "100" {
				t.Fatalf("n = %q, want 100", got)
			}
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"name":"sentioxyz/changelogue","tags":["v1.0.0","latest"]}`))
		case "/v2/sentioxyz/changelogue/manifests/v1.0.0":
			w.Header().Set("Content-Type", "application/vnd.oci.image.manifest.v1+json")
			_, _ = w.Write([]byte(`{"annotations":{"org.opencontainers.image.created":"2026-01-02T03:04:05Z"}}`))
		case "/v2/sentioxyz/changelogue/manifests/latest":
			w.Header().Set("Content-Type", "application/vnd.oci.image.manifest.v1+json")
			_, _ = w.Write([]byte(`{"config":{"digest":"sha256:latest-config"}}`))
		case "/v2/sentioxyz/changelogue/blobs/sha256:latest-config":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"created":"2026-01-03T04:05:06Z"}`))
		default:
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
	}))
	defer srv.Close()

	src := NewGHCRSource(srv.Client(), "sentioxyz/changelogue", "src-123")
	src.baseURL = srv.URL

	results, err := src.FetchNewReleases(context.Background())
	if err != nil {
		t.Fatalf("FetchNewReleases: %v", err)
	}
	if len(results) != 2 {
		t.Fatalf("got %d results, want 2", len(results))
	}
	if results[0].RawVersion != "v1.0.0" {
		t.Errorf("results[0].RawVersion = %q, want v1.0.0", results[0].RawVersion)
	}
	if results[0].Repository != "sentioxyz/changelogue" {
		t.Errorf("results[0].Repository = %q, want sentioxyz/changelogue", results[0].Repository)
	}
	if got := results[0].Metadata["release_url"]; got != "https://ghcr.io/sentioxyz/changelogue:v1.0.0" {
		t.Errorf("release_url = %q", got)
	}
	expected, _ := time.Parse(time.RFC3339, "2026-01-02T03:04:05Z")
	if !results[0].Timestamp.Equal(expected) {
		t.Errorf("results[0].Timestamp = %v, want %v", results[0].Timestamp, expected)
	}
	if got := results[0].Metadata["timestamp_source"]; got != "manifest_annotation" {
		t.Errorf("timestamp_source = %q, want manifest_annotation", got)
	}
	expectedConfig, _ := time.Parse(time.RFC3339, "2026-01-03T04:05:06Z")
	if !results[1].Timestamp.Equal(expectedConfig) {
		t.Errorf("results[1].Timestamp = %v, want %v", results[1].Timestamp, expectedConfig)
	}
	if got := results[1].Metadata["timestamp_source"]; got != "image_config" {
		t.Errorf("latest timestamp_source = %q, want image_config", got)
	}
}

func TestGHCRFetchWithBearerChallenge(t *testing.T) {
	var tokenURL string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v2/sentioxyz/private/tags/list":
			if r.Header.Get("Authorization") == "" {
				w.Header().Set("WWW-Authenticate", `Bearer realm="`+tokenURL+`",service="ghcr.io",scope="repository:sentioxyz/private:pull"`)
				w.WriteHeader(http.StatusUnauthorized)
				return
			}
			if got := r.Header.Get("Authorization"); got != "Bearer test-token" {
				t.Fatalf("Authorization = %q, want Bearer test-token", got)
			}
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"name":"sentioxyz/private","tags":["v2.0.0"]}`))
		case "/v2/sentioxyz/private/manifests/v2.0.0":
			if r.Header.Get("Authorization") == "" {
				w.Header().Set("WWW-Authenticate", `Bearer realm="`+tokenURL+`",service="ghcr.io",scope="repository:sentioxyz/private:pull"`)
				w.WriteHeader(http.StatusUnauthorized)
				return
			}
			if got := r.Header.Get("Authorization"); got != "Bearer test-token" {
				t.Fatalf("manifest Authorization = %q, want Bearer test-token", got)
			}
			w.Header().Set("Content-Type", "application/vnd.oci.image.manifest.v1+json")
			_, _ = w.Write([]byte(`{"annotations":{"org.opencontainers.image.created":"2026-02-03T04:05:06Z"}}`))
		case "/token":
			if got := r.URL.Query().Get("service"); got != "ghcr.io" {
				t.Fatalf("service = %q, want ghcr.io", got)
			}
			if got := r.URL.Query().Get("scope"); got != "repository:sentioxyz/private:pull" {
				t.Fatalf("scope = %q", got)
			}
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"token":"test-token"}`))
		default:
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
	}))
	defer srv.Close()
	tokenURL = srv.URL + "/token"

	src := NewGHCRSource(srv.Client(), "ghcr.io/sentioxyz/private", "")
	src.baseURL = srv.URL

	results, err := src.FetchNewReleases(context.Background())
	if err != nil {
		t.Fatalf("FetchNewReleases: %v", err)
	}
	if len(results) != 1 || results[0].RawVersion != "v2.0.0" {
		t.Fatalf("results = %#v", results)
	}
	if results[0].Repository != "sentioxyz/private" {
		t.Fatalf("repository = %q", results[0].Repository)
	}
}

func TestGHCRFetchTimestampFromManifestListAnnotations(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/v2/sentioxyz/changelogue/tags/list":
			_, _ = w.Write([]byte(`{"name":"sentioxyz/changelogue","tags":["multi"]}`))
		case "/v2/sentioxyz/changelogue/manifests/multi":
			_, _ = w.Write([]byte(`{
				"manifests": [
					{"annotations":{"org.opencontainers.image.created":"2026-03-01T00:00:00Z"}},
					{"annotations":{"org.opencontainers.image.created":"2026-03-02T00:00:00Z"}}
				]
			}`))
		default:
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
	}))
	defer srv.Close()

	src := NewGHCRSource(srv.Client(), "sentioxyz/changelogue", "")
	src.baseURL = srv.URL

	results, err := src.FetchNewReleases(context.Background())
	if err != nil {
		t.Fatalf("FetchNewReleases: %v", err)
	}
	expected, _ := time.Parse(time.RFC3339, "2026-03-02T00:00:00Z")
	if !results[0].Timestamp.Equal(expected) {
		t.Fatalf("timestamp = %v, want %v", results[0].Timestamp, expected)
	}
	if got := results[0].Metadata["timestamp_source"]; got != "manifest_list_annotation" {
		t.Fatalf("timestamp_source = %q", got)
	}
}

func TestGHCRFetchTimestampUnavailableDoesNotUseFetchTime(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/v2/sentioxyz/changelogue/tags/list":
			_, _ = w.Write([]byte(`{"name":"sentioxyz/changelogue","tags":["unknown"]}`))
		case "/v2/sentioxyz/changelogue/manifests/unknown":
			_, _ = w.Write([]byte(`{"schemaVersion":2}`))
		default:
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
	}))
	defer srv.Close()

	src := NewGHCRSource(srv.Client(), "sentioxyz/changelogue", "")
	src.baseURL = srv.URL

	results, err := src.FetchNewReleases(context.Background())
	if err != nil {
		t.Fatalf("FetchNewReleases: %v", err)
	}
	if !results[0].Timestamp.IsZero() {
		t.Fatalf("timestamp = %v, want zero when metadata is unavailable", results[0].Timestamp)
	}
	if got := results[0].Metadata["timestamp_source"]; got != "unavailable" {
		t.Fatalf("timestamp_source = %q", got)
	}
}

func TestGHCRPagination(t *testing.T) {
	tagsCallCount := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/v2/sentioxyz/changelogue/tags/list":
			tagsCallCount++
			if tagsCallCount == 1 {
				w.Header().Set("Link", `</v2/sentioxyz/changelogue/tags/list?n=100&last=v1.0.0>; rel="next"`)
				_, _ = w.Write([]byte(`{"name":"sentioxyz/changelogue","tags":["v1.0.0"]}`))
				return
			}
			if got := r.URL.Query().Get("last"); got != "v1.0.0" {
				t.Fatalf("last = %q, want v1.0.0", got)
			}
			_, _ = w.Write([]byte(`{"name":"sentioxyz/changelogue","tags":["v2.0.0"]}`))
		case "/v2/sentioxyz/changelogue/manifests/v1.0.0":
			_, _ = w.Write([]byte(`{"annotations":{"org.opencontainers.image.created":"2026-01-01T00:00:00Z"}}`))
		case "/v2/sentioxyz/changelogue/manifests/v2.0.0":
			_, _ = w.Write([]byte(`{"annotations":{"org.opencontainers.image.created":"2026-02-01T00:00:00Z"}}`))
		default:
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
	}))
	defer srv.Close()

	src := NewGHCRSource(srv.Client(), "sentioxyz/changelog", "")
	src.repository = "sentioxyz/changelogue"
	src.baseURL = srv.URL

	results, err := src.FetchNewReleases(context.Background())
	if err != nil {
		t.Fatalf("FetchNewReleases: %v", err)
	}
	if len(results) != 2 {
		t.Fatalf("got %d results, want 2", len(results))
	}
	if results[1].RawVersion != "v2.0.0" {
		t.Fatalf("second version = %q", results[1].RawVersion)
	}
}

func TestGHCRInvalidRepository(t *testing.T) {
	src := NewGHCRSource(http.DefaultClient, "no-slash", "")
	_, err := src.FetchNewReleases(context.Background())
	if err == nil {
		t.Fatal("expected error for invalid repository")
	}
	if !strings.Contains(err.Error(), "owner/image") {
		t.Fatalf("error = %v", err)
	}
}

func TestParseBearerChallenge(t *testing.T) {
	realm, service, scope := parseBearerChallenge(`Bearer realm="https://ghcr.io/token",service="ghcr.io",scope="repository:owner/image:pull"`)
	if realm != "https://ghcr.io/token" || service != "ghcr.io" || scope != "repository:owner/image:pull" {
		t.Fatalf("realm=%q service=%q scope=%q", realm, service, scope)
	}
}
