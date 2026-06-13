package githubauth

import (
	"bytes"
	"context"
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"
)

const defaultBaseURL = "https://api.github.com"

type HTTPDoer interface {
	Do(*http.Request) (*http.Response, error)
}

type AppTokenProvider struct {
	client                HTTPDoer
	baseURL               string
	appID                 string
	privateKey            *rsa.PrivateKey
	defaultInstallationID int64
	now                   func() time.Time

	mu            sync.Mutex
	installations map[string]int64
	tokens        map[int64]cachedToken
}

type Installation struct {
	ID                  int64
	AccountLogin        string
	AccountType         string
	RepositorySelection string
	Permissions         json.RawMessage
}

type Repository struct {
	FullName string
	Private  bool
	HTMLURL  string
}

type cachedToken struct {
	token     string
	expiresAt time.Time
}

type AppConfig struct {
	AppID                 string
	PrivateKeyPEM         string
	PrivateKeyFile        string
	DefaultInstallationID int64
	BaseURL               string
}

func NewAppTokenProviderFromEnv(client HTTPDoer, baseURL string) *AppTokenProvider {
	installationID, _ := strconv.ParseInt(strings.TrimSpace(os.Getenv("GITHUB_APP_INSTALLATION_ID")), 10, 64)
	return NewAppTokenProvider(client, AppConfig{
		AppID:                 os.Getenv("GITHUB_APP_ID"),
		PrivateKeyPEM:         os.Getenv("GITHUB_APP_PRIVATE_KEY"),
		PrivateKeyFile:        os.Getenv("GITHUB_APP_PRIVATE_KEY_FILE"),
		DefaultInstallationID: installationID,
		BaseURL:               baseURL,
	})
}

func NewAppTokenProvider(client HTTPDoer, cfg AppConfig) *AppTokenProvider {
	if client == nil {
		client = http.DefaultClient
	}
	baseURL := strings.TrimRight(cfg.BaseURL, "/")
	if baseURL == "" {
		baseURL = defaultBaseURL
	}
	key, _ := loadPrivateKey(cfg.PrivateKeyPEM, cfg.PrivateKeyFile)
	return &AppTokenProvider{
		client:                client,
		baseURL:               baseURL,
		appID:                 strings.TrimSpace(cfg.AppID),
		privateKey:            key,
		defaultInstallationID: cfg.DefaultInstallationID,
		now:                   time.Now,
		installations:         map[string]int64{},
		tokens:                map[int64]cachedToken{},
	}
}

func (p *AppTokenProvider) TokenForRepo(ctx context.Context, owner, repo string) (string, error) {
	if p == nil || p.appID == "" || p.privateKey == nil {
		return "", ErrNotConfigured
	}
	installationID, err := p.installationIDForRepo(ctx, owner, repo)
	if err != nil {
		return "", err
	}
	return p.installationToken(ctx, installationID)
}

func (p *AppTokenProvider) Configured() bool {
	return p != nil && p.appID != "" && p.privateKey != nil
}

func (p *AppTokenProvider) AppID() string {
	if p == nil {
		return ""
	}
	return p.appID
}

func (p *AppTokenProvider) ListInstallations(ctx context.Context) ([]Installation, error) {
	if !p.Configured() {
		return nil, ErrNotConfigured
	}
	var installations []Installation
	path := "/app/installations?per_page=100"
	for path != "" {
		var page []struct {
			ID                  int64           `json:"id"`
			Account             githubAccount   `json:"account"`
			RepositorySelection string          `json:"repository_selection"`
			Permissions         json.RawMessage `json:"permissions"`
		}
		next, err := p.appGet(ctx, path, &page)
		if err != nil {
			return nil, err
		}
		for _, item := range page {
			installations = append(installations, Installation{
				ID:                  item.ID,
				AccountLogin:        item.Account.Login,
				AccountType:         item.Account.Type,
				RepositorySelection: item.RepositorySelection,
				Permissions:         item.Permissions,
			})
		}
		path = next
	}
	return installations, nil
}

func (p *AppTokenProvider) ListInstallationRepositories(ctx context.Context, installationID int64) ([]Repository, error) {
	if !p.Configured() {
		return nil, ErrNotConfigured
	}
	token, err := p.installationToken(ctx, installationID)
	if err != nil {
		return nil, err
	}
	var repositories []Repository
	path := "/installation/repositories?per_page=100"
	for path != "" {
		var page struct {
			Repositories []struct {
				FullName string `json:"full_name"`
				Private  bool   `json:"private"`
				HTMLURL  string `json:"html_url"`
			} `json:"repositories"`
		}
		next, err := p.bearerGet(ctx, token, path, &page)
		if err != nil {
			return nil, err
		}
		for _, item := range page.Repositories {
			repositories = append(repositories, Repository{FullName: item.FullName, Private: item.Private, HTMLURL: item.HTMLURL})
		}
		path = next
	}
	return repositories, nil
}

type githubAccount struct {
	Login string `json:"login"`
	Type  string `json:"type"`
}

func (p *AppTokenProvider) installationIDForRepo(ctx context.Context, owner, repo string) (int64, error) {
	key := owner + "/" + repo
	p.mu.Lock()
	if id, ok := p.installations[key]; ok {
		p.mu.Unlock()
		return id, nil
	}
	p.mu.Unlock()

	if p.defaultInstallationID > 0 {
		return p.defaultInstallationID, nil
	}

	jwt, err := p.jwt()
	if err != nil {
		return 0, err
	}
	url := fmt.Sprintf("%s/repos/%s/%s/installation", p.baseURL, owner, repo)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return 0, err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("Authorization", "Bearer "+jwt)

	resp, err := p.client.Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return 0, ErrRepoInstallationNone
	}
	if resp.StatusCode == http.StatusForbidden || resp.StatusCode == http.StatusUnauthorized {
		return 0, ErrRepoUnauthorized
	}
	if resp.StatusCode == http.StatusTooManyRequests {
		return 0, ErrRateLimited
	}
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return 0, fmt.Errorf("github installation lookup returned %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}

	var result struct {
		ID int64 `json:"id"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return 0, fmt.Errorf("decode github installation: %w", err)
	}
	if result.ID == 0 {
		return 0, ErrRepoInstallationNone
	}

	p.mu.Lock()
	p.installations[key] = result.ID
	p.mu.Unlock()
	return result.ID, nil
}

func (p *AppTokenProvider) appGet(ctx context.Context, path string, out any) (string, error) {
	jwt, err := p.jwt()
	if err != nil {
		return "", err
	}
	return p.bearerGet(ctx, jwt, path, out)
}

func (p *AppTokenProvider) bearerGet(ctx context.Context, token, path string, out any) (string, error) {
	url := p.baseURL + path
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := p.client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusForbidden || resp.StatusCode == http.StatusUnauthorized {
		return "", ErrRepoUnauthorized
	}
	if resp.StatusCode == http.StatusTooManyRequests {
		return "", ErrRateLimited
	}
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return "", fmt.Errorf("github api returned %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}
	if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
		return "", fmt.Errorf("decode github response: %w", err)
	}
	return nextPath(resp.Header.Get("Link")), nil
}

func (p *AppTokenProvider) installationToken(ctx context.Context, installationID int64) (string, error) {
	p.mu.Lock()
	if cached, ok := p.tokens[installationID]; ok && p.now().Before(cached.expiresAt.Add(-time.Minute)) {
		p.mu.Unlock()
		return cached.token, nil
	}
	p.mu.Unlock()

	jwt, err := p.jwt()
	if err != nil {
		return "", err
	}
	url := fmt.Sprintf("%s/app/installations/%d/access_tokens", p.baseURL, installationID)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader([]byte("{}")))
	if err != nil {
		return "", err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("Authorization", "Bearer "+jwt)
	req.Header.Set("Content-Type", "application/json")

	resp, err := p.client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusForbidden || resp.StatusCode == http.StatusUnauthorized {
		return "", ErrRepoUnauthorized
	}
	if resp.StatusCode == http.StatusTooManyRequests {
		return "", ErrRateLimited
	}
	if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return "", fmt.Errorf("github installation token returned %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}

	var result struct {
		Token     string    `json:"token"`
		ExpiresAt time.Time `json:"expires_at"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", fmt.Errorf("decode github installation token: %w", err)
	}
	if result.Token == "" {
		return "", fmt.Errorf("github installation token response missing token")
	}

	p.mu.Lock()
	p.tokens[installationID] = cachedToken{token: result.Token, expiresAt: result.ExpiresAt}
	p.mu.Unlock()
	return result.Token, nil
}

func (p *AppTokenProvider) jwt() (string, error) {
	issuedAt := p.now().Add(-time.Minute).Unix()
	expiresAt := p.now().Add(9 * time.Minute).Unix()
	header := map[string]string{"alg": "RS256", "typ": "JWT"}
	payload := map[string]any{"iat": issuedAt, "exp": expiresAt, "iss": p.appID}

	headerJSON, _ := json.Marshal(header)
	payloadJSON, _ := json.Marshal(payload)
	signingInput := base64.RawURLEncoding.EncodeToString(headerJSON) + "." + base64.RawURLEncoding.EncodeToString(payloadJSON)
	digest := sha256.Sum256([]byte(signingInput))
	sig, err := rsa.SignPKCS1v15(rand.Reader, p.privateKey, crypto.SHA256, digest[:])
	if err != nil {
		return "", fmt.Errorf("sign github app jwt: %w", err)
	}
	return signingInput + "." + base64.RawURLEncoding.EncodeToString(sig), nil
}

func loadPrivateKey(raw, file string) (*rsa.PrivateKey, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" && strings.TrimSpace(file) != "" {
		b, err := os.ReadFile(strings.TrimSpace(file))
		if err != nil {
			return nil, err
		}
		raw = string(b)
	}
	if raw == "" {
		return nil, ErrNotConfigured
	}
	raw = strings.ReplaceAll(raw, `\n`, "\n")
	block, _ := pem.Decode([]byte(raw))
	if block == nil {
		return nil, errors.New("decode github app private key PEM")
	}
	if key, err := x509.ParsePKCS1PrivateKey(block.Bytes); err == nil {
		return key, nil
	}
	parsed, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("parse github app private key: %w", err)
	}
	key, ok := parsed.(*rsa.PrivateKey)
	if !ok {
		return nil, errors.New("github app private key must be RSA")
	}
	return key, nil
}

func nextPath(linkHeader string) string {
	for _, part := range strings.Split(linkHeader, ",") {
		part = strings.TrimSpace(part)
		if !strings.Contains(part, `rel="next"`) {
			continue
		}
		start := strings.Index(part, "<")
		end := strings.Index(part, ">")
		if start < 0 || end <= start {
			continue
		}
		raw := part[start+1 : end]
		if idx := strings.Index(raw, "/app/"); idx >= 0 {
			return raw[idx:]
		}
		if idx := strings.Index(raw, "/installation/"); idx >= 0 {
			return raw[idx:]
		}
	}
	return ""
}
