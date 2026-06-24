package ingestion

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestGHCRSourceName(t *testing.T) {
	src := NewGHCRSource(http.DefaultClient, "sentioxyz/changelogue", "")
	if got := src.Name(); got != "ghcr" {
		t.Errorf("Name() = %q, want %q", got, "ghcr")
	}
}

func TestGHCRFetchNewReleases(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v2/sentioxyz/changelogue/tags/list" {
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
		if got := r.URL.Query().Get("n"); got != "100" {
			t.Fatalf("n = %q, want 100", got)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"name":"sentioxyz/changelogue","tags":["v1.0.0","latest"]}`))
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

func TestGHCRPagination(t *testing.T) {
	callCount := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		w.Header().Set("Content-Type", "application/json")
		if callCount == 1 {
			w.Header().Set("Link", `</v2/sentioxyz/changelogue/tags/list?n=100&last=v1.0.0>; rel="next"`)
			_, _ = w.Write([]byte(`{"name":"sentioxyz/changelogue","tags":["v1.0.0"]}`))
			return
		}
		if got := r.URL.Query().Get("last"); got != "v1.0.0" {
			t.Fatalf("last = %q, want v1.0.0", got)
		}
		_, _ = w.Write([]byte(`{"name":"sentioxyz/changelogue","tags":["v2.0.0"]}`))
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
