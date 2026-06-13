package githubauth

import (
	"context"
	"errors"
	"os"
	"strings"
)

var (
	ErrNotConfigured        = errors.New("github auth is not configured")
	ErrRepoInstallationNone = errors.New("github app is not installed for repository")
	ErrRepoUnauthorized     = errors.New("github repository is not authorized")
	ErrRateLimited          = errors.New("github api rate limit exceeded")
)

// TokenProvider returns a GitHub bearer token for a specific repository.
type TokenProvider interface {
	TokenForRepo(ctx context.Context, owner, repo string) (string, error)
}

// StaticTokenProvider returns the same token for every repository.
type StaticTokenProvider struct {
	token string
}

func NewStaticTokenProvider(token string) *StaticTokenProvider {
	return &StaticTokenProvider{token: strings.TrimSpace(token)}
}

func NewEnvTokenProvider() *StaticTokenProvider {
	return NewStaticTokenProvider(os.Getenv("GITHUB_TOKEN"))
}

func (p *StaticTokenProvider) TokenForRepo(context.Context, string, string) (string, error) {
	if p == nil || p.token == "" {
		return "", ErrNotConfigured
	}
	return p.token, nil
}

// ChainTokenProvider tries providers in order. Auth-not-configured and
// installation-missing errors allow fallback; authorization/rate-limit errors do not.
type ChainTokenProvider struct {
	providers []TokenProvider
}

func NewChainTokenProvider(providers ...TokenProvider) *ChainTokenProvider {
	return &ChainTokenProvider{providers: providers}
}

func (p *ChainTokenProvider) TokenForRepo(ctx context.Context, owner, repo string) (string, error) {
	if p == nil {
		return "", ErrNotConfigured
	}
	var lastErr error = ErrNotConfigured
	for _, provider := range p.providers {
		if provider == nil {
			continue
		}
		token, err := provider.TokenForRepo(ctx, owner, repo)
		if err == nil {
			return token, nil
		}
		lastErr = err
		if errors.Is(err, ErrNotConfigured) || errors.Is(err, ErrRepoInstallationNone) {
			continue
		}
		return "", err
	}
	return "", lastErr
}

// NewDefaultTokenProvider builds the server's default GitHub auth chain.
func NewDefaultTokenProvider(client HTTPDoer, baseURL string) TokenProvider {
	return NewChainTokenProvider(NewAppTokenProviderFromEnv(client, baseURL), NewEnvTokenProvider())
}
