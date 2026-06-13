package githubauth

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestAppTokenProviderResolvesAndCachesInstallationToken(t *testing.T) {
	privateKey := testPrivateKeyPEM(t)
	installationLookups := 0
	tokenRequests := 0

	mux := http.NewServeMux()
	mux.HandleFunc("GET /repos/acme/private/installation", func(w http.ResponseWriter, r *http.Request) {
		installationLookups++
		if got := r.Header.Get("Authorization"); !strings.HasPrefix(got, "Bearer ") {
			t.Fatalf("installation lookup Authorization = %q, want bearer jwt", got)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":123}`))
	})
	mux.HandleFunc("POST /app/installations/123/access_tokens", func(w http.ResponseWriter, r *http.Request) {
		tokenRequests++
		if got := r.Header.Get("Authorization"); !strings.HasPrefix(got, "Bearer ") {
			t.Fatalf("token request Authorization = %q, want bearer jwt", got)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"token":"installation-token","expires_at":"2026-06-13T12:00:00Z"}`))
	})
	server := httptest.NewServer(mux)
	defer server.Close()

	provider := NewAppTokenProvider(server.Client(), AppConfig{
		AppID:         "42",
		PrivateKeyPEM: privateKey,
		BaseURL:       server.URL,
	})
	provider.now = func() time.Time { return time.Date(2026, 6, 13, 10, 0, 0, 0, time.UTC) }

	for i := 0; i < 2; i++ {
		token, err := provider.TokenForRepo(context.Background(), "acme", "private")
		if err != nil {
			t.Fatalf("TokenForRepo attempt %d: %v", i+1, err)
		}
		if token != "installation-token" {
			t.Fatalf("token = %q, want installation-token", token)
		}
	}
	if installationLookups != 1 {
		t.Fatalf("installation lookups = %d, want 1", installationLookups)
	}
	if tokenRequests != 1 {
		t.Fatalf("token requests = %d, want 1", tokenRequests)
	}
}

func TestAppTokenProviderListsInstallationsAndRepositories(t *testing.T) {
	privateKey := testPrivateKeyPEM(t)
	mux := http.NewServeMux()
	mux.HandleFunc("GET /app/installations", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`[{"id":123,"account":{"login":"acme","type":"Organization"},"repository_selection":"selected","permissions":{"contents":"read"}}]`))
	})
	mux.HandleFunc("POST /app/installations/123/access_tokens", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"token":"installation-token","expires_at":"2026-06-13T12:00:00Z"}`))
	})
	mux.HandleFunc("GET /installation/repositories", func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Authorization"); got != "Bearer installation-token" {
			t.Fatalf("Authorization = %q, want Bearer installation-token", got)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"repositories":[{"full_name":"acme/private","private":true,"html_url":"https://github.com/acme/private"}]}`))
	})
	server := httptest.NewServer(mux)
	defer server.Close()

	provider := NewAppTokenProvider(server.Client(), AppConfig{AppID: "42", PrivateKeyPEM: privateKey, BaseURL: server.URL})
	provider.now = func() time.Time { return time.Date(2026, 6, 13, 10, 0, 0, 0, time.UTC) }

	installations, err := provider.ListInstallations(context.Background())
	if err != nil {
		t.Fatalf("ListInstallations: %v", err)
	}
	if len(installations) != 1 || installations[0].AccountLogin != "acme" {
		t.Fatalf("installations = %+v", installations)
	}
	repos, err := provider.ListInstallationRepositories(context.Background(), 123)
	if err != nil {
		t.Fatalf("ListInstallationRepositories: %v", err)
	}
	if len(repos) != 1 || repos[0].FullName != "acme/private" || !repos[0].Private {
		t.Fatalf("repos = %+v", repos)
	}
}

func testPrivateKeyPEM(t *testing.T) string {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}
	return string(pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(key)}))
}
