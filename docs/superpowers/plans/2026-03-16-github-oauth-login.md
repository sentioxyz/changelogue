# GitHub OAuth Login Implementation Plan

> **For agentic workers:** REQUIRED: Use superpowers:subagent-driven-development (if subagents available) or superpowers:executing-plans to implement this plan. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add GitHub OAuth login with server-side sessions, restricted to an allowlist of GitHub usernames/orgs, while preserving existing API key auth for machine clients.

**Architecture:** New `internal/auth/` package handles OAuth flow, session management, and allowlist validation. The existing `Auth` middleware in `internal/api/auth.go` is extended to accept either Bearer API keys or session cookies. Frontend gets an `AuthProvider` context that redirects unauthenticated users to a login page.

**Tech Stack:** Go stdlib `net/http`, `crypto/hmac`, `crypto/sha256` for session signing, PostgreSQL for user/session storage, React context + fetch for frontend auth.

**Spec:** `docs/superpowers/specs/2026-03-16-github-oauth-login-design.md`

---

## File Structure

### New files (backend)
| File | Responsibility |
|------|----------------|
| `internal/auth/user.go` | `User` struct, context helpers (`UserFromContext`, `WithUser`) |
| `internal/auth/session.go` | `SessionStore` interface, session create/validate/delete/cleanup, HMAC cookie signing |
| `internal/auth/allowlist.go` | Parse env vars, check username/org membership, startup validation |
| `internal/auth/oauth.go` | GitHub OAuth handlers: authorize redirect, callback, `/auth/me`, `/auth/logout` |
| `internal/auth/state.go` | In-memory OAuth state parameter store with TTL and size limit |
| `internal/auth/session_test.go` | Tests for session HMAC signing/validation |
| `internal/auth/allowlist_test.go` | Tests for allowlist parsing and matching |
| `internal/auth/state_test.go` | Tests for state store TTL and size limits |
| `internal/auth/oauth_test.go` | Tests for OAuth handlers (using httptest) |

### New files (frontend)
| File | Responsibility |
|------|----------------|
| `web/lib/auth/context.tsx` | `AuthProvider` component + `useAuth()` hook |
| `web/app/login/page.tsx` | Login page with "Sign in with GitHub" button |

### Modified files
| File | Change |
|------|--------|
| `internal/db/migrations.go` | Add `users` and `sessions` tables to schema |
| `internal/api/auth.go` | Extend `Auth()` to accept session cookies as fallback; add `SessionStore` interface |
| `internal/api/auth_test.go` | Add tests for session cookie auth path |
| `internal/api/server.go` | Add `SessionStore` to `Dependencies`; pass to `Auth()`; register auth routes |
| `cmd/server/main.go` | Read new env vars, validate allowlist, create session store, start cleanup goroutine |
| `web/app/layout.tsx` | Wrap children in `AuthProvider` |
| `web/components/layout/sidebar.tsx` | Add user avatar + sign out at bottom |

---

## Chunk 1: Database Schema & Core Types

### Task 1: Add users and sessions tables to migrations

**Files:**
- Modify: `internal/db/migrations.go:12-189` (add to `schema` const)

- [ ] **Step 1: Add users and sessions tables to the schema constant**

In `internal/db/migrations.go`, append these tables to the end of the `schema` constant (before the closing backtick on line 189):

```go
-- Authenticated users (GitHub OAuth)
CREATE TABLE IF NOT EXISTS users (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    github_id       BIGINT NOT NULL UNIQUE,
    github_login    VARCHAR(100) NOT NULL,
    name            VARCHAR(200),
    avatar_url      TEXT,
    created_at      TIMESTAMPTZ DEFAULT NOW(),
    updated_at      TIMESTAMPTZ DEFAULT NOW()
);

-- User sessions (server-side, referenced by HttpOnly cookie)
CREATE TABLE IF NOT EXISTS sessions (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id     UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    expires_at  TIMESTAMPTZ NOT NULL,
    created_at  TIMESTAMPTZ DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_sessions_user_id ON sessions(user_id);
CREATE INDEX IF NOT EXISTS idx_sessions_expires_at ON sessions(expires_at);
```

- [ ] **Step 2: Verify migrations compile**

Run: `go build ./internal/db/...`
Expected: No errors

- [ ] **Step 3: Commit**

```bash
git add internal/db/migrations.go
git commit -m "feat(auth): add users and sessions tables to schema"
```

### Task 2: Create User type and context helpers

**Files:**
- Create: `internal/auth/user.go`

- [ ] **Step 1: Create the auth package with User type and context helpers**

```go
package auth

import "context"

// User represents an authenticated user.
type User struct {
	ID          string `json:"id"`
	GitHubID    int64  `json:"github_id"`
	GitHubLogin string `json:"github_login"`
	Name        string `json:"name,omitempty"`
	AvatarURL   string `json:"avatar_url,omitempty"`
}

type contextKey string

const userContextKey contextKey = "user"

// WithUser stores a User in the request context.
func WithUser(ctx context.Context, u *User) context.Context {
	return context.WithValue(ctx, userContextKey, u)
}

// UserFromContext retrieves the authenticated User from the context, or nil.
func UserFromContext(ctx context.Context) *User {
	u, _ := ctx.Value(userContextKey).(*User)
	return u
}
```

- [ ] **Step 2: Verify it compiles**

Run: `go build ./internal/auth/...`
Expected: No errors

- [ ] **Step 3: Commit**

```bash
git add internal/auth/user.go
git commit -m "feat(auth): add User type and context helpers"
```

---

## Chunk 2: Session Store & HMAC Cookie Signing

### Task 3: Create session store with HMAC cookie signing

**Files:**
- Create: `internal/auth/session.go`
- Create: `internal/auth/session_test.go`

- [ ] **Step 1: Write the failing tests for HMAC signing and session cookie parsing**

Create `internal/auth/session_test.go`:

```go
package auth

import "testing"

func TestSignAndParseSessionCookie(t *testing.T) {
	secret := "test-secret-key-32-bytes-long!!"
	sessionID := "550e8400-e29b-41d4-a716-446655440000"

	cookie := SignSessionCookie(sessionID, secret)
	if cookie == sessionID {
		t.Fatal("cookie should not equal raw session ID")
	}

	parsed, err := ParseSessionCookie(cookie, secret)
	if err != nil {
		t.Fatalf("parse failed: %v", err)
	}
	if parsed != sessionID {
		t.Fatalf("expected %s, got %s", sessionID, parsed)
	}
}

func TestParseSessionCookieInvalidSignature(t *testing.T) {
	secret := "test-secret-key-32-bytes-long!!"
	cookie := "550e8400-e29b-41d4-a716-446655440000.invalidsignature"

	_, err := ParseSessionCookie(cookie, secret)
	if err == nil {
		t.Fatal("expected error for invalid signature")
	}
}

func TestParseSessionCookieMalformed(t *testing.T) {
	secret := "test-secret-key-32-bytes-long!!"

	// No dot separator
	_, err := ParseSessionCookie("noseparator", secret)
	if err == nil {
		t.Fatal("expected error for malformed cookie")
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test -v -run TestSign ./internal/auth/...`
Expected: FAIL — functions not defined

- [ ] **Step 3: Implement session.go**

Create `internal/auth/session.go`:

```go
package auth

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

const sessionDuration = 7 * 24 * time.Hour

// SessionStore manages server-side sessions in PostgreSQL.
type SessionStore struct {
	pool   *pgxpool.Pool
	secret string
}

// NewSessionStore creates a new session store.
func NewSessionStore(pool *pgxpool.Pool, secret string) *SessionStore {
	return &SessionStore{pool: pool, secret: secret}
}

// SignSessionCookie produces "sessionID.hmacHex".
func SignSessionCookie(sessionID, secret string) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(sessionID))
	sig := hex.EncodeToString(mac.Sum(nil))
	return sessionID + "." + sig
}

// ParseSessionCookie validates "sessionID.hmacHex" and returns the sessionID.
func ParseSessionCookie(cookie, secret string) (string, error) {
	idx := strings.LastIndex(cookie, ".")
	if idx < 0 {
		return "", errors.New("malformed session cookie")
	}
	sessionID := cookie[:idx]
	sig := cookie[idx+1:]

	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(sessionID))
	expected := hex.EncodeToString(mac.Sum(nil))

	if !hmac.Equal([]byte(sig), []byte(expected)) {
		return "", errors.New("invalid session signature")
	}
	return sessionID, nil
}

// CreateSession creates a new session for the given user and returns the signed cookie value.
func (s *SessionStore) CreateSession(ctx context.Context, userID string) (string, time.Time, error) {
	var sessionID string
	expiresAt := time.Now().Add(sessionDuration)
	err := s.pool.QueryRow(ctx,
		`INSERT INTO sessions (user_id, expires_at) VALUES ($1, $2) RETURNING id`,
		userID, expiresAt,
	).Scan(&sessionID)
	if err != nil {
		return "", time.Time{}, fmt.Errorf("create session: %w", err)
	}
	return SignSessionCookie(sessionID, s.secret), expiresAt, nil
}

// ValidateSession parses the cookie, verifies the HMAC, looks up the session, and returns the user.
func (s *SessionStore) ValidateSession(ctx context.Context, cookie string) (*User, error) {
	sessionID, err := ParseSessionCookie(cookie, s.secret)
	if err != nil {
		return nil, err
	}

	var u User
	err = s.pool.QueryRow(ctx, `
		SELECT u.id, u.github_id, u.github_login, COALESCE(u.name,''), COALESCE(u.avatar_url,'')
		FROM sessions s JOIN users u ON u.id = s.user_id
		WHERE s.id = $1 AND s.expires_at > NOW()
	`, sessionID).Scan(&u.ID, &u.GitHubID, &u.GitHubLogin, &u.Name, &u.AvatarURL)
	if err != nil {
		return nil, fmt.Errorf("invalid or expired session: %w", err)
	}
	return &u, nil
}

// DeleteSession removes a session by cookie value.
func (s *SessionStore) DeleteSession(ctx context.Context, cookie string) error {
	sessionID, err := ParseSessionCookie(cookie, s.secret)
	if err != nil {
		return err
	}
	_, err = s.pool.Exec(ctx, `DELETE FROM sessions WHERE id = $1`, sessionID)
	return err
}

// CleanupExpired removes all expired sessions. Intended to be called periodically.
func (s *SessionStore) CleanupExpired(ctx context.Context) (int64, error) {
	tag, err := s.pool.Exec(ctx, `DELETE FROM sessions WHERE expires_at < NOW()`)
	if err != nil {
		return 0, err
	}
	return tag.RowsAffected(), nil
}

// RunCleanupLoop deletes expired sessions every hour until ctx is cancelled.
func (s *SessionStore) RunCleanupLoop(ctx context.Context) {
	ticker := time.NewTicker(1 * time.Hour)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if n, err := s.CleanupExpired(ctx); err != nil {
				slog.Error("session cleanup failed", "err", err)
			} else if n > 0 {
				slog.Info("cleaned up expired sessions", "count", n)
			}
		}
	}
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test -v -run TestSign ./internal/auth/...`
Expected: PASS (3 tests)

- [ ] **Step 5: Commit**

```bash
git add internal/auth/session.go internal/auth/session_test.go
git commit -m "feat(auth): add session store with HMAC cookie signing"
```

---

## Chunk 3: Allowlist & State Store

### Task 4: Create allowlist validator

**Files:**
- Create: `internal/auth/allowlist.go`
- Create: `internal/auth/allowlist_test.go`

- [ ] **Step 1: Write the failing tests**

Create `internal/auth/allowlist_test.go`:

```go
package auth

import "testing"

func TestAllowlistParseAndCheck(t *testing.T) {
	al := NewAllowlist("alice,bob", "myorg,otherog")

	if !al.IsUserAllowed("alice", nil) {
		t.Fatal("alice should be allowed by username")
	}
	if !al.IsUserAllowed("charlie", []string{"myorg"}) {
		t.Fatal("charlie should be allowed by org membership")
	}
	if al.IsUserAllowed("charlie", []string{"randorg"}) {
		t.Fatal("charlie with no matching org should be denied")
	}
	if al.IsUserAllowed("charlie", nil) {
		t.Fatal("charlie with no orgs should be denied")
	}
}

func TestAllowlistEmpty(t *testing.T) {
	al := NewAllowlist("", "")
	if al.HasEntries() {
		t.Fatal("empty allowlist should report no entries")
	}
}

func TestAllowlistCaseInsensitive(t *testing.T) {
	al := NewAllowlist("Alice", "MyOrg")
	if !al.IsUserAllowed("alice", nil) {
		t.Fatal("username check should be case-insensitive")
	}
	if !al.IsUserAllowed("bob", []string{"myorg"}) {
		t.Fatal("org check should be case-insensitive")
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test -v -run TestAllowlist ./internal/auth/...`
Expected: FAIL

- [ ] **Step 3: Implement allowlist.go**

Create `internal/auth/allowlist.go`:

```go
package auth

import "strings"

// Allowlist restricts login to specific GitHub users and/or org members.
type Allowlist struct {
	users map[string]bool
	orgs  map[string]bool
}

// NewAllowlist parses comma-separated username and org lists.
func NewAllowlist(users, orgs string) *Allowlist {
	al := &Allowlist{
		users: make(map[string]bool),
		orgs:  make(map[string]bool),
	}
	for _, u := range strings.Split(users, ",") {
		u = strings.TrimSpace(strings.ToLower(u))
		if u != "" {
			al.users[u] = true
		}
	}
	for _, o := range strings.Split(orgs, ",") {
		o = strings.TrimSpace(strings.ToLower(o))
		if o != "" {
			al.orgs[o] = true
		}
	}
	return al
}

// HasEntries returns true if at least one user or org is configured.
func (a *Allowlist) HasEntries() bool {
	return len(a.users) > 0 || len(a.orgs) > 0
}

// IsUserAllowed checks if a GitHub login or any of their org memberships match.
func (a *Allowlist) IsUserAllowed(login string, orgLogins []string) bool {
	if a.users[strings.ToLower(login)] {
		return true
	}
	for _, o := range orgLogins {
		if a.orgs[strings.ToLower(o)] {
			return true
		}
	}
	return false
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test -v -run TestAllowlist ./internal/auth/...`
Expected: PASS (3 tests)

- [ ] **Step 5: Commit**

```bash
git add internal/auth/allowlist.go internal/auth/allowlist_test.go
git commit -m "feat(auth): add allowlist validator for GitHub users and orgs"
```

### Task 5: Create OAuth state parameter store

**Files:**
- Create: `internal/auth/state.go`
- Create: `internal/auth/state_test.go`

- [ ] **Step 1: Write the failing tests**

Create `internal/auth/state_test.go`:

```go
package auth

import (
	"testing"
	"time"
)

func TestStateStoreCreateAndConsume(t *testing.T) {
	store := NewStateStore(100, 10*time.Minute)

	state := store.Create()
	if state == "" {
		t.Fatal("state should not be empty")
	}

	if !store.Consume(state) {
		t.Fatal("first consume should succeed")
	}

	if store.Consume(state) {
		t.Fatal("second consume should fail (already consumed)")
	}
}

func TestStateStoreMaxSize(t *testing.T) {
	store := NewStateStore(2, 10*time.Minute)

	store.Create()
	store.Create()
	third := store.Create()

	if third != "" {
		t.Fatal("should return empty string when max size reached")
	}
}

func TestStateStoreExpiry(t *testing.T) {
	store := NewStateStore(100, 1*time.Millisecond)

	state := store.Create()
	time.Sleep(5 * time.Millisecond)

	if store.Consume(state) {
		t.Fatal("expired state should not be consumable")
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test -v -run TestStateStore ./internal/auth/...`
Expected: FAIL

- [ ] **Step 3: Implement state.go**

Create `internal/auth/state.go`:

```go
package auth

import (
	"crypto/rand"
	"encoding/hex"
	"sync"
	"time"
)

type stateEntry struct {
	expiresAt time.Time
}

// StateStore holds OAuth state parameters in memory with TTL and size limit.
type StateStore struct {
	mu      sync.Mutex
	entries map[string]stateEntry
	maxSize int
	ttl     time.Duration
}

// NewStateStore creates a state store with the given max size and TTL per entry.
func NewStateStore(maxSize int, ttl time.Duration) *StateStore {
	return &StateStore{
		entries: make(map[string]stateEntry),
		maxSize: maxSize,
		ttl:     ttl,
	}
}

// Create generates a random state string and stores it. Returns "" if the store is full.
func (s *StateStore) Create() string {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Sweep expired before checking size
	now := time.Now()
	for k, v := range s.entries {
		if now.After(v.expiresAt) {
			delete(s.entries, k)
		}
	}

	if len(s.entries) >= s.maxSize {
		return ""
	}

	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return ""
	}
	state := hex.EncodeToString(b)
	s.entries[state] = stateEntry{expiresAt: now.Add(s.ttl)}
	return state
}

// Consume validates and removes a state parameter. Returns true if valid.
func (s *StateStore) Consume(state string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	entry, ok := s.entries[state]
	if !ok {
		return false
	}
	delete(s.entries, state)
	return time.Now().Before(entry.expiresAt)
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test -v -run TestStateStore ./internal/auth/...`
Expected: PASS (3 tests)

- [ ] **Step 5: Commit**

```bash
git add internal/auth/state.go internal/auth/state_test.go
git commit -m "feat(auth): add in-memory OAuth state store with TTL and size limit"
```

---

## Chunk 4: OAuth Handlers

### Task 6: Create GitHub OAuth handlers

**Files:**
- Create: `internal/auth/oauth.go`
- Create: `internal/auth/oauth_test.go`

- [ ] **Step 1: Write failing tests for the OAuth handlers**

Create `internal/auth/oauth_test.go`:

```go
package auth

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestHandleGitHubRedirect(t *testing.T) {
	h := &OAuthHandler{
		ClientID: "test-client-id",
		States:   NewStateStore(100, 10*time.Minute),
	}

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/auth/github", nil)
	h.HandleGitHubRedirect(w, r)

	if w.Code != http.StatusFound {
		t.Fatalf("expected 302, got %d", w.Code)
	}
	loc := w.Header().Get("Location")
	if loc == "" {
		t.Fatal("expected Location header")
	}
	if !strings.Contains(loc, "github.com/login/oauth/authorize") {
		t.Fatalf("expected GitHub authorize URL, got %s", loc)
	}
	if !strings.Contains(loc, "client_id=test-client-id") {
		t.Fatalf("expected client_id in URL, got %s", loc)
	}
}

func TestHandleGitHubRedirectWhenFull(t *testing.T) {
	h := &OAuthHandler{
		ClientID: "test-client-id",
		States:   NewStateStore(0, 10*time.Minute), // zero capacity
	}

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/auth/github", nil)
	h.HandleGitHubRedirect(w, r)

	if w.Code != http.StatusTooManyRequests {
		t.Fatalf("expected 429, got %d", w.Code)
	}
}

func TestHandleMeNoSession(t *testing.T) {
	h := &OAuthHandler{}

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/auth/me", nil)
	h.HandleMe(w, r)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", w.Code)
	}
}

func TestHandleMeWithUser(t *testing.T) {
	h := &OAuthHandler{}

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/auth/me", nil)
	ctx := WithUser(r.Context(), &User{
		ID:          "user-1",
		GitHubID:    12345,
		GitHubLogin: "alice",
		Name:        "Alice",
		AvatarURL:   "https://github.com/alice.png",
	})
	r = r.WithContext(ctx)
	h.HandleMe(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
}

```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test -v -run TestHandle ./internal/auth/...`
Expected: FAIL

- [ ] **Step 3: Implement oauth.go**

Create `internal/auth/oauth.go`:

```go
package auth

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"time"

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
		http.Redirect(w, r, "/login?error=invalid_state", http.StatusFound)
		return
	}

	code := r.URL.Query().Get("code")
	if code == "" {
		http.Redirect(w, r, "/login?error=missing_code", http.StatusFound)
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
		http.Redirect(w, r, "/login?error=token_exchange", http.StatusFound)
		return
	}

	// Fetch user info
	ghUser, err := fetchGitHubUser(client, token)
	if err != nil {
		slog.Error("github user fetch failed", "err", err)
		http.Redirect(w, r, "/login?error=user_fetch", http.StatusFound)
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
		http.Redirect(w, r, "/login?error=unauthorized", http.StatusFound)
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
		http.Redirect(w, r, "/login?error=server_error", http.StatusFound)
		return
	}

	// Create session
	cookie, expiresAt, err := h.Sessions.CreateSession(r.Context(), userID)
	if err != nil {
		slog.Error("session creation failed", "err", err)
		http.Redirect(w, r, "/login?error=server_error", http.StatusFound)
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
	http.Redirect(w, r, "/", http.StatusFound)
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
	http.Redirect(w, r, "/login", http.StatusFound)
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
	req, _ := http.NewRequest(http.MethodPost, "https://github.com/login/oauth/access_token", nil)
	req.URL.RawQuery = data.Encode()
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
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test -v -run TestHandle ./internal/auth/...`
Expected: PASS (4 tests)

- [ ] **Step 5: Run all auth package tests**

Run: `go test -v ./internal/auth/...`
Expected: PASS (10 tests total)

- [ ] **Step 6: Commit**

```bash
git add internal/auth/oauth.go internal/auth/oauth_test.go
git commit -m "feat(auth): add GitHub OAuth handlers with redirect, callback, me, logout"
```

---

## Chunk 5: Integrate Auth into API Layer

### Task 7: Extend Auth middleware to accept session cookies

**Files:**
- Modify: `internal/api/auth.go:12-35`
- Modify: `internal/api/auth_test.go`

- [ ] **Step 1: Write failing test for session cookie auth**

Add to `internal/api/auth_test.go`:

```go
type mockSessionValidator struct {
	user *auth.User
}

func (m *mockSessionValidator) ValidateSession(_ context.Context, _ string) (*auth.User, error) {
	if m.user != nil {
		return m.user, nil
	}
	return nil, fmt.Errorf("invalid session")
}

func TestAuthMiddlewareSessionCookie(t *testing.T) {
	keys := &mockKeyStore{valid: false}
	sessions := &mockSessionValidator{user: &auth.User{ID: "u1", GitHubLogin: "alice"}}
	handler := Auth(keys, sessions)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		u := auth.UserFromContext(r.Context())
		if u == nil {
			t.Fatal("expected user in context")
		}
		if u.GitHubLogin != "alice" {
			t.Fatalf("expected alice, got %s", u.GitHubLogin)
		}
		w.WriteHeader(http.StatusOK)
	}))

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r.AddCookie(&http.Cookie{Name: "session", Value: "test-cookie"})
	handler.ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
}

func TestAuthMiddlewareBearerTakesPrecedence(t *testing.T) {
	keys := &mockKeyStore{valid: true}
	sessions := &mockSessionValidator{user: nil} // would fail if reached
	handler := Auth(keys, sessions)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r.Header.Set("Authorization", "Bearer valid-key")
	r.AddCookie(&http.Cookie{Name: "session", Value: "test-cookie"})
	handler.ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
}
```

Add import for `auth` package and `fmt` at top of the test file.

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test -v -run TestAuthMiddlewareSession ./internal/api/...`
Expected: FAIL (function signature mismatch)

- [ ] **Step 3: Modify Auth middleware to accept sessions**

Update `internal/api/auth.go`:

```go
package api

import (
	"context"
	"net/http"
	"strings"
	"sync"

	"github.com/sentioxyz/changelogue/internal/auth"
	"golang.org/x/time/rate"
)

type KeyStore interface {
	ValidateKey(ctx context.Context, rawKey string) (bool, error)
	TouchKeyUsage(ctx context.Context, rawKey string)
}

// SessionValidator validates a session cookie and returns the user.
type SessionValidator interface {
	ValidateSession(ctx context.Context, cookie string) (*auth.User, error)
}

func Auth(store KeyStore, sessions SessionValidator) Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Try Bearer token first
			if header := r.Header.Get("Authorization"); strings.HasPrefix(header, "Bearer ") {
				rawKey := strings.TrimPrefix(header, "Bearer ")
				valid, err := store.ValidateKey(r.Context(), rawKey)
				if err != nil || !valid {
					RespondError(w, r, http.StatusUnauthorized, "unauthorized", "Invalid API key")
					return
				}
				go store.TouchKeyUsage(context.Background(), rawKey)
				next.ServeHTTP(w, r)
				return
			}

			// Try session cookie
			if sessions != nil {
				if c, err := r.Cookie("session"); err == nil {
					u, err := sessions.ValidateSession(r.Context(), c.Value)
					if err == nil {
						next.ServeHTTP(w, r.WithContext(auth.WithUser(r.Context(), u)))
						return
					}
				}
			}

			RespondError(w, r, http.StatusUnauthorized, "unauthorized", "Missing API key")
		})
	}
}
```

Keep the `RateLimit` function unchanged.

- [ ] **Step 4: Update existing tests for new Auth signature**

Update all existing `Auth(store)` calls to `Auth(store, nil)` in `auth_test.go` (lines 18, 34, 66). The `nil` session validator means session cookie path is skipped (backwards compatible).

- [ ] **Step 5: Run all auth tests**

Run: `go test -v ./internal/api/... -run TestAuth`
Expected: PASS (5 tests — 3 existing + 2 new)

- [ ] **Step 6: Commit**

```bash
git add internal/api/auth.go internal/api/auth_test.go
git commit -m "feat(auth): extend Auth middleware to accept session cookies as fallback"
```

### Task 8: Wire auth into server.go and main.go

**Files:**
- Modify: `internal/api/server.go:12-42`
- Modify: `cmd/server/main.go`

- [ ] **Step 1: Add SessionValidator to Dependencies and update Auth call in server.go**

In `internal/api/server.go`, add `SessionValidator` field to `Dependencies` struct (after `KeyStore` on line 25):

```go
SessionValidator  SessionValidator
```

Update line 41 to pass sessions:

```go
chain = Chain(RequestID, Logger, Recovery, RateLimit(10, 20), Auth(deps.KeyStore, deps.SessionValidator))
```

- [ ] **Step 2: Register auth routes in server.go**

Add after the `RegisterRoutes` function or within it (after the health routes at line 143), a new `RegisterAuthRoutes` function. Add it as a separate exported function in `server.go`:

```go
// RegisterAuthRoutes registers OAuth and session endpoints on the mux.
func RegisterAuthRoutes(mux *http.ServeMux, oauthHandler *auth.OAuthHandler, noAuth bool) {
	if noAuth {
		// Dev mode: /auth/me returns fake user, other auth routes are no-ops
		mux.HandleFunc("GET /auth/me", auth.HandleMeDev)
		mux.HandleFunc("POST /auth/logout", func(w http.ResponseWriter, r *http.Request) {
			http.Redirect(w, r, "/login", http.StatusFound)
		})
		return
	}

	sessionMw := oauthHandler.RequireSession
	mux.HandleFunc("GET /auth/github", oauthHandler.HandleGitHubRedirect)
	mux.HandleFunc("GET /auth/github/callback", oauthHandler.HandleGitHubCallback)
	mux.Handle("GET /auth/me", sessionMw(http.HandlerFunc(oauthHandler.HandleMe)))
	mux.Handle("POST /auth/logout", sessionMw(http.HandlerFunc(oauthHandler.HandleLogout)))
}
```

Add import for `"github.com/sentioxyz/changelogue/internal/auth"`.

- [ ] **Step 3: Wire everything in main.go**

In `cmd/server/main.go`, add these changes:

After `noAuth` declaration (line 39), add env var reads:

```go
githubClientID := os.Getenv("GITHUB_CLIENT_ID")
githubClientSecret := os.Getenv("GITHUB_CLIENT_SECRET")
allowedUsers := os.Getenv("ALLOWED_GITHUB_USERS")
allowedOrgs := os.Getenv("ALLOWED_GITHUB_ORGS")
sessionSecret := os.Getenv("SESSION_SECRET")
secureCookies := os.Getenv("SECURE_COOKIES") != "false" // secure by default
```

After the `pgStore` creation (line 57), add allowlist validation and auth setup:

```go
// Auth setup
allowlist := auth.NewAllowlist(allowedUsers, allowedOrgs)
if !noAuth && !allowlist.HasEntries() {
	slog.Error("ALLOWED_GITHUB_USERS or ALLOWED_GITHUB_ORGS must be set when auth is enabled")
	os.Exit(1)
}

sessionStore := auth.NewSessionStore(pool, sessionSecret)
stateStore := auth.NewStateStore(1000, 10*time.Minute)

oauthHandler := &auth.OAuthHandler{
	ClientID:     githubClientID,
	ClientSecret: githubClientSecret,
	Pool:         pool,
	Sessions:     sessionStore,
	States:       stateStore,
	Allowlist:    allowlist,
	SecureCookie: secureCookies,
	HTTPClient:   http.DefaultClient,
}
```

In the `Dependencies` struct initialization (around line 141), add:

```go
SessionValidator: sessionStore,
```

After `api.RegisterRoutes(mux, ...)` call (around line 160), add:

```go
api.RegisterAuthRoutes(mux, oauthHandler, noAuth)
```

Before `<-ctx.Done()` (around line 178), start the session cleanup goroutine:

```go
go sessionStore.RunCleanupLoop(ctx)
```

Add import for `"github.com/sentioxyz/changelogue/internal/auth"`.

- [ ] **Step 4: Verify it compiles**

Run: `go build ./cmd/server/...`
Expected: No errors

- [ ] **Step 5: Run all tests**

Run: `go test ./internal/api/... ./internal/auth/...`
Expected: PASS

- [ ] **Step 6: Commit**

```bash
git add internal/api/server.go cmd/server/main.go
git commit -m "feat(auth): wire OAuth handlers and session store into server"
```

---

## Chunk 6: Frontend Auth

### Task 9: Create AuthProvider context

**Files:**
- Create: `web/lib/auth/context.tsx`

- [ ] **Step 1: Create the auth context provider**

Create `web/lib/auth/context.tsx`:

```tsx
"use client";

import {
  createContext,
  useContext,
  useEffect,
  useState,
  type ReactNode,
} from "react";
import { usePathname, useRouter } from "next/navigation";

interface User {
  id: string;
  github_id: number;
  github_login: string;
  name?: string;
  avatar_url?: string;
}

interface AuthContextValue {
  user: User | null;
  loading: boolean;
  logout: () => Promise<void>;
}

const AuthContext = createContext<AuthContextValue>({
  user: null,
  loading: true,
  logout: async () => {},
});

export function useAuth() {
  return useContext(AuthContext);
}

export function AuthProvider({ children }: { children: ReactNode }) {
  const [user, setUser] = useState<User | null>(null);
  const [loading, setLoading] = useState(true);
  const pathname = usePathname();
  const router = useRouter();

  useEffect(() => {
    fetch("/auth/me")
      .then((res) => {
        if (res.ok) return res.json();
        throw new Error("unauthorized");
      })
      .then((data) => {
        setUser(data);
        setLoading(false);
      })
      .catch(() => {
        setUser(null);
        setLoading(false);
        if (pathname !== "/login") {
          router.replace("/login");
        }
      });
  }, [pathname, router]);

  const logout = async () => {
    await fetch("/auth/logout", { method: "POST" });
    setUser(null);
    router.replace("/login");
  };

  if (loading) {
    return (
      <div className="flex h-screen items-center justify-center bg-[#f8f8f6]">
        <div className="h-6 w-6 animate-spin rounded-full border-2 border-[#e8601a] border-t-transparent" />
      </div>
    );
  }

  // On login page, render without redirect guard
  if (pathname === "/login") {
    return (
      <AuthContext.Provider value={{ user, loading, logout }}>
        {children}
      </AuthContext.Provider>
    );
  }

  // Not logged in and not on login page — handled by useEffect redirect above
  if (!user) {
    return null;
  }

  return (
    <AuthContext.Provider value={{ user, loading, logout }}>
      {children}
    </AuthContext.Provider>
  );
}
```

- [ ] **Step 2: Commit**

```bash
git add web/lib/auth/context.tsx
git commit -m "feat(auth): add AuthProvider context and useAuth hook"
```

### Task 10: Create login page

**Files:**
- Create: `web/app/login/page.tsx`

- [ ] **Step 1: Create the login page**

Create `web/app/login/page.tsx`:

```tsx
"use client";

import { useSearchParams } from "next/navigation";
import { Suspense } from "react";

function LoginContent() {
  const params = useSearchParams();
  const error = params.get("error");

  const errorMessages: Record<string, string> = {
    unauthorized: "Your GitHub account is not authorized to access this application.",
    invalid_state: "Login session expired. Please try again.",
    missing_code: "GitHub did not return an authorization code.",
    token_exchange: "Failed to authenticate with GitHub. Please try again.",
    user_fetch: "Failed to fetch your GitHub profile. Please try again.",
    server_error: "An unexpected error occurred. Please try again.",
  };

  return (
    <div
      className="flex min-h-screen items-center justify-center"
      style={{ backgroundColor: "#f8f8f6" }}
    >
      <div className="w-full max-w-sm space-y-6 text-center">
        <div className="flex items-center justify-center gap-2">
          <img src="/logo.svg" alt="" className="h-8 w-8" />
          <span
            className="text-xl italic text-[#16181c]"
            style={{ fontFamily: "var(--font-fraunces)" }}
          >
            Changelogue
          </span>
        </div>

        <p className="text-sm text-[#6b7280]">
          Sign in to access your release intelligence dashboard.
        </p>

        {error && (
          <div className="rounded-md border border-red-200 bg-red-50 px-4 py-3 text-sm text-red-700">
            {errorMessages[error] || "An error occurred. Please try again."}
          </div>
        )}

        <a
          href="/auth/github"
          className="inline-flex w-full items-center justify-center gap-2 rounded-md px-4 py-2 text-sm font-medium text-white transition-colors hover:opacity-90"
          style={{ backgroundColor: "#16181c" }}
        >
          <svg className="h-5 w-5" fill="currentColor" viewBox="0 0 24 24">
            <path d="M12 0C5.37 0 0 5.37 0 12c0 5.31 3.435 9.795 8.205 11.385.6.105.825-.255.825-.57 0-.285-.015-1.23-.015-2.235-3.015.555-3.795-.735-4.035-1.41-.135-.345-.72-1.41-1.23-1.695-.42-.225-1.02-.78-.015-.795.945-.015 1.62.87 1.845 1.23 1.08 1.815 2.805 1.305 3.495.99.105-.78.42-1.305.765-1.605-2.67-.3-5.46-1.335-5.46-5.925 0-1.305.465-2.385 1.23-3.225-.12-.3-.54-1.53.12-3.18 0 0 1.005-.315 3.3 1.23.96-.27 1.98-.405 3-.405s2.04.135 3 .405c2.295-1.56 3.3-1.23 3.3-1.23.66 1.65.24 2.88.12 3.18.765.84 1.23 1.905 1.23 3.225 0 4.605-2.805 5.625-5.475 5.925.435.375.81 1.095.81 2.22 0 1.605-.015 2.895-.015 3.3 0 .315.225.69.825.57A12.02 12.02 0 0024 12c0-6.63-5.37-12-12-12z" />
          </svg>
          Sign in with GitHub
        </a>
      </div>
    </div>
  );
}

export default function LoginPage() {
  return (
    <Suspense>
      <LoginContent />
    </Suspense>
  );
}
```

- [ ] **Step 2: Commit**

```bash
git add web/app/login/page.tsx
git commit -m "feat(auth): add login page with GitHub sign-in button"
```

### Task 11: Integrate AuthProvider into layout and sidebar

**Files:**
- Modify: `web/app/layout.tsx`
- Modify: `web/components/layout/sidebar.tsx`

- [ ] **Step 1: Wrap layout in AuthProvider**

In `web/app/layout.tsx`, add import:

```tsx
import { AuthProvider } from "@/lib/auth/context";
```

Wrap the body contents in `AuthProvider`. The current `<body>` content (lines 34-40) is:

```tsx
<body className={`${fraunces.variable} ${dmSans.variable} antialiased`}>
  <div className="flex h-screen">
    <Sidebar />
    <div className="flex flex-1 flex-col overflow-hidden">
      <main className="flex-1 overflow-y-auto p-6 fade-in">{children}</main>
    </div>
  </div>
</body>
```

Change it to:

```tsx
<body className={`${fraunces.variable} ${dmSans.variable} antialiased`}>
  <AuthProvider>
    <LayoutShell>{children}</LayoutShell>
  </AuthProvider>
</body>
```

Since `AuthProvider` is a client component and `layout.tsx` is a server component (for metadata), create `LayoutShell` as a separate client component.

Create `web/components/layout/layout-shell.tsx`:

```tsx
"use client";

import { usePathname } from "next/navigation";
import { Sidebar } from "@/components/layout/sidebar";

export function LayoutShell({ children }: { children: React.ReactNode }) {
  const pathname = usePathname();
  if (pathname === "/login") {
    return <>{children}</>;
  }
  return (
    <div className="flex h-screen">
      <Sidebar />
      <div className="flex flex-1 flex-col overflow-hidden">
        <main className="flex-1 overflow-y-auto p-6 fade-in">{children}</main>
      </div>
    </div>
  );
}
```

Update `web/app/layout.tsx` to use it:

```tsx
import { AuthProvider } from "@/lib/auth/context";
import { LayoutShell } from "@/components/layout/layout-shell";
```

And remove the direct `Sidebar` import.

- [ ] **Step 2: Add user avatar and sign-out to sidebar**

In `web/components/layout/sidebar.tsx`, add import at top:

```tsx
import { LogOut } from "lucide-react";
import { useAuth } from "@/lib/auth/context";
```

Inside the `Sidebar` component function, after `const [expanded, setExpanded] = useState(false);`, add:

```tsx
const { user, logout } = useAuth();
```

Before the closing `</aside>`, after the `</nav>` (line 108), add:

```tsx
{/* User section */}
{user && (
  <div className="border-t border-[rgba(255,255,255,0.1)] p-2">
    <div className={cn("flex items-center gap-2", expanded ? "px-2" : "justify-center")}>
      {user.avatar_url ? (
        <img
          src={user.avatar_url}
          alt={user.github_login}
          className="h-6 w-6 shrink-0 rounded-full"
        />
      ) : (
        <div className="flex h-6 w-6 shrink-0 items-center justify-center rounded-full bg-[#e8601a] text-xs text-white">
          {user.github_login[0].toUpperCase()}
        </div>
      )}
      {expanded && (
        <div className="flex flex-1 items-center justify-between">
          <span className="truncate text-xs text-[#9ca3af]">{user.github_login}</span>
          <button
            onClick={logout}
            title="Sign out"
            className="text-[#9ca3af] transition-colors hover:text-white"
          >
            <LogOut className="h-3.5 w-3.5" />
          </button>
        </div>
      )}
    </div>
  </div>
)}
```

- [ ] **Step 3: Verify frontend builds**

Run: `cd web && npm run build`
Expected: Build succeeds

- [ ] **Step 4: Commit**

```bash
git add web/app/layout.tsx web/components/layout/layout-shell.tsx web/components/layout/sidebar.tsx
git commit -m "feat(auth): integrate AuthProvider into layout, add user menu to sidebar"
```

---

## Chunk 7: Frontend Route & Final Verification

### Task 12: Add login to frontend dynamic routes

**Files:**
- Modify: `internal/api/frontend.go:13-23` (if needed)

- [ ] **Step 1: Check if /login route works with static export**

The Next.js static export will produce `web/out/login.html`. The existing `RegisterFrontend` in `internal/api/frontend.go` has a fallback that tries `path + ".html"` (line 59-64). This means `/login` will resolve to `login.html` automatically.

Verify by checking the static export output:

Run: `ls web/out/login.html` (after `npm run build` in web/)

If the file exists, no changes needed to `frontend.go`.

- [ ] **Step 2: Commit (if changes were needed)**

Only commit if `frontend.go` was modified.

### Task 13: Update README with new environment variables

**Files:**
- Modify: `README.md`

- [ ] **Step 1: Add auth environment variables to README**

Add a new section or update the existing environment variables table with:

| Variable | Default | Purpose |
|----------|---------|---------|
| `GITHUB_CLIENT_ID` | _(required in prod)_ | GitHub OAuth App client ID |
| `GITHUB_CLIENT_SECRET` | _(required in prod)_ | GitHub OAuth App client secret |
| `ALLOWED_GITHUB_USERS` | _(empty)_ | Comma-separated GitHub usernames allowed to log in |
| `ALLOWED_GITHUB_ORGS` | _(empty)_ | Comma-separated GitHub org logins allowed to log in |
| `SESSION_SECRET` | _(required in prod)_ | Secret key for HMAC-signing session cookies |
| `SECURE_COOKIES` | `true` | Set `false` for local HTTP dev |

Add a note: "At least one of `ALLOWED_GITHUB_USERS` or `ALLOWED_GITHUB_ORGS` must be set when `NO_AUTH` is not `true`."

- [ ] **Step 2: Commit**

```bash
git add README.md
git commit -m "docs: add GitHub OAuth environment variables to README"
```

### Task 14: End-to-end verification

- [ ] **Step 1: Run all Go tests**

Run: `go test ./...`
Expected: PASS

- [ ] **Step 2: Run Go vet**

Run: `go vet ./...`
Expected: No issues

- [ ] **Step 3: Build the binary**

Run: `go build -o changelogue ./cmd/server`
Expected: Builds successfully

- [ ] **Step 4: Build the frontend**

Run: `cd web && npm run build`
Expected: Builds successfully, `web/out/login.html` exists

- [ ] **Step 5: Manual smoke test (dev mode)**

Run: `make dev` (with `NO_AUTH=true`)
- Navigate to `http://localhost:8080` — should load dashboard normally
- Navigate to `http://localhost:8080/auth/me` — should return dev user JSON
- Navigate to `http://localhost:8080/login` — should show login page

- [ ] **Step 6: Final commit (if any remaining changes)**

```bash
git add -A
git commit -m "feat(auth): GitHub OAuth login with session-based auth"
```
