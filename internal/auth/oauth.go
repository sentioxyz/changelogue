package auth

import (
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"
)

// OAuthHandler handles GitHub OAuth flow and session endpoints.
type OAuthHandler struct {
	ClientID     string
	ClientSecret string
	Pool         *pgxpool.Pool
	Sessions     *SessionStore
	States       *StateStore
	Allowlist    *Allowlist
	SecureCookie bool
	FrontendURL  string // optional: redirect here after login/logout (for dev with separate frontend port)
	HTTPClient   *http.Client
}

// HandleGitHubRedirect redirects to GitHub's OAuth authorize URL.
func (h *OAuthHandler) HandleGitHubRedirect(w http.ResponseWriter, r *http.Request) {
	state := h.States.Create()
	if state == "" {
		http.Error(w, "too many pending login requests", http.StatusTooManyRequests)
		return
	}

	params := url.Values{
		"client_id":    {h.ClientID},
		"state":        {state},
		"scope":        {"read:org"},
		"redirect_uri": {callbackURL(r)},
	}
	http.Redirect(w, r, "https://github.com/login/oauth/authorize?"+params.Encode(), http.StatusFound)
}

// HandleGitHubCallback handles the OAuth callback from GitHub.
func (h *OAuthHandler) HandleGitHubCallback(w http.ResponseWriter, r *http.Request) {
	// Validate state
	state := r.URL.Query().Get("state")
	if !h.States.Consume(state) {
		http.Redirect(w, r, h.frontendURL("/login?error=invalid_state"), http.StatusFound)
		return
	}

	code := r.URL.Query().Get("code")
	if code == "" {
		http.Redirect(w, r, h.frontendURL("/login?error=missing_code"), http.StatusFound)
		return
	}

	client := h.HTTPClient
	if client == nil {
		client = http.DefaultClient
	}

	// Exchange code for access token
	token, err := exchangeCode(client, h.ClientID, h.ClientSecret, code)
	if err != nil {
		slog.Error("github token exchange failed", "err", err)
		http.Redirect(w, r, h.frontendURL("/login?error=token_exchange"), http.StatusFound)
		return
	}

	// Fetch user info
	ghUser, err := fetchGitHubUser(client, token)
	if err != nil {
		slog.Error("github user fetch failed", "err", err)
		http.Redirect(w, r, h.frontendURL("/login?error=user_fetch"), http.StatusFound)
		return
	}

	// Fetch user orgs
	orgs, err := fetchGitHubOrgs(client, token)
	if err != nil {
		slog.Warn("github orgs fetch failed, proceeding without orgs", "err", err)
	}

	// Check allowlist
	orgLogins := make([]string, len(orgs))
	for i, o := range orgs {
		orgLogins[i] = o.Login
	}
	if !h.Allowlist.IsUserAllowed(ghUser.Login, orgLogins) {
		slog.Warn("login denied: user not in allowlist", "login", ghUser.Login, "orgs", orgLogins)
		http.Redirect(w, r, h.frontendURL("/login?error=unauthorized"), http.StatusFound)
		return
	}

	// Upsert user
	var userID string
	err = h.Pool.QueryRow(r.Context(), `
		INSERT INTO users (github_id, github_login, name, avatar_url)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (github_id) DO UPDATE SET
			github_login = EXCLUDED.github_login,
			name = EXCLUDED.name,
			avatar_url = EXCLUDED.avatar_url,
			updated_at = NOW()
		RETURNING id
	`, ghUser.ID, ghUser.Login, ghUser.Name, ghUser.AvatarURL).Scan(&userID)
	if err != nil {
		slog.Error("user upsert failed", "err", err)
		http.Redirect(w, r, h.frontendURL("/login?error=server_error"), http.StatusFound)
		return
	}

	// Create session
	cookie, expiresAt, err := h.Sessions.CreateSession(r.Context(), userID)
	if err != nil {
		slog.Error("session creation failed", "err", err)
		http.Redirect(w, r, h.frontendURL("/login?error=server_error"), http.StatusFound)
		return
	}

	slog.Info("user logged in", "login", ghUser.Login, "user_id", userID)

	http.SetCookie(w, &http.Cookie{
		Name:     "session",
		Value:    cookie,
		Path:     "/",
		Expires:  expiresAt,
		HttpOnly: true,
		Secure:   h.SecureCookie,
		SameSite: http.SameSiteLaxMode,
	})
	http.Redirect(w, r, h.frontendURL("/"), http.StatusFound)
}

// HandleMe returns the current user from the session context.
func (h *OAuthHandler) HandleMe(w http.ResponseWriter, r *http.Request) {
	u := UserFromContext(r.Context())
	if u == nil {
		http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(u)
}

// HandleLogout deletes the session and clears the cookie.
func (h *OAuthHandler) HandleLogout(w http.ResponseWriter, r *http.Request) {
	if c, err := r.Cookie("session"); err == nil {
		h.Sessions.DeleteSession(r.Context(), c.Value)
	}
	http.SetCookie(w, &http.Cookie{
		Name:     "session",
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   h.SecureCookie,
		SameSite: http.SameSiteLaxMode,
	})
	w.WriteHeader(http.StatusOK)
}

// frontendURL returns a path prefixed with FrontendURL if set (for dev with separate frontend port).
func (h *OAuthHandler) frontendURL(path string) string {
	if h.FrontendURL != "" {
		return strings.TrimRight(h.FrontendURL, "/") + path
	}
	return path
}

// RequireSession is middleware that loads the user from the session cookie into context.
func (h *OAuthHandler) RequireSession(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c, err := r.Cookie("session")
		if err != nil {
			http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
			return
		}
		u, err := h.Sessions.ValidateSession(r.Context(), c.Value)
		if err != nil {
			http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
			return
		}
		next.ServeHTTP(w, r.WithContext(WithUser(r.Context(), u)))
	})
}

// --- GitHub API helpers ---

type githubUser struct {
	ID        int64  `json:"id"`
	Login     string `json:"login"`
	Name      string `json:"name"`
	AvatarURL string `json:"avatar_url"`
}

type githubOrg struct {
	Login string `json:"login"`
}

func exchangeCode(client *http.Client, clientID, clientSecret, code string) (string, error) {
	data := url.Values{
		"client_id":     {clientID},
		"client_secret": {clientSecret},
		"code":          {code},
	}
	req, _ := http.NewRequest(http.MethodPost, "https://github.com/login/oauth/access_token", strings.NewReader(data.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("token request: %w", err)
	}
	defer resp.Body.Close()

	var result struct {
		AccessToken string `json:"access_token"`
		Error       string `json:"error"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", fmt.Errorf("decode token response: %w", err)
	}
	if result.Error != "" {
		return "", fmt.Errorf("github error: %s", result.Error)
	}
	if result.AccessToken == "" {
		return "", fmt.Errorf("empty access token")
	}
	return result.AccessToken, nil
}

func fetchGitHubUser(client *http.Client, token string) (*githubUser, error) {
	req, _ := http.NewRequest(http.MethodGet, "https://api.github.com/user", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("github /user returned %d: %s", resp.StatusCode, body)
	}

	var u githubUser
	if err := json.NewDecoder(resp.Body).Decode(&u); err != nil {
		return nil, err
	}
	return &u, nil
}

func fetchGitHubOrgs(client *http.Client, token string) ([]githubOrg, error) {
	req, _ := http.NewRequest(http.MethodGet, "https://api.github.com/user/orgs", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("github /user/orgs returned %d", resp.StatusCode)
	}

	var orgs []githubOrg
	if err := json.NewDecoder(resp.Body).Decode(&orgs); err != nil {
		return nil, err
	}
	return orgs, nil
}

func callbackURL(r *http.Request) string {
	scheme := "https"
	if r.TLS == nil {
		scheme = "http"
	}
	if fwd := r.Header.Get("X-Forwarded-Proto"); fwd != "" {
		scheme = fwd
	}
	return scheme + "://" + r.Host + "/auth/github/callback"
}

// DevUser returns a fake user for NO_AUTH development mode.
func DevUser() *User {
	return &User{
		ID:          "00000000-0000-0000-0000-000000000000",
		GitHubID:    0,
		GitHubLogin: "dev",
		Name:        "Dev User",
		AvatarURL:   "",
	}
}

// HandleMeDev returns a hardcoded dev user (for NO_AUTH mode).
func HandleMeDev(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(DevUser())
}

// DevSession is middleware that injects a fake dev user (for NO_AUTH mode).
func DevSession(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := WithUser(r.Context(), DevUser())
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}
