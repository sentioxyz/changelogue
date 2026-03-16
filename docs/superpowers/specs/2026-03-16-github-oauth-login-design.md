# GitHub OAuth Login Design

## Overview

Add GitHub OAuth login to Changelogue with server-side sessions, restricted to an allowlist of GitHub usernames and/or org memberships. Existing API key auth for machine clients is preserved — both auth methods coexist.

## OAuth Flow

### Routes

| Route | Auth | Purpose |
|-------|------|---------|
| `GET /auth/github` | None | Redirect to GitHub OAuth authorize URL |
| `GET /auth/github/callback` | None | Receive code, exchange for token, validate user, create session |
| `GET /auth/me` | Session | Return current user info |
| `POST /auth/logout` | Session | Delete session, clear cookie |

### Flow

1. Frontend redirects to `/auth/github`
2. Backend generates random `state`, stores in memory (10-min TTL, max 1000 entries), redirects to `https://github.com/login/oauth/authorize?client_id=...&state=...&scope=read:org`
3. GitHub redirects to `/auth/github/callback?code=...&state=...`
4. Backend validates `state`, exchanges `code` for access token via `https://github.com/login/oauth/access_token`
5. Fetches `https://api.github.com/user` and `https://api.github.com/user/orgs` using the access token
6. **Access token is discarded** after fetching user info — it is not stored. Org membership is checked at login time only.
7. Checks username against `ALLOWED_GITHUB_USERS` and org logins against `ALLOWED_GITHUB_ORGS`
8. If allowed: upsert user, create session, set HttpOnly cookie, redirect to `/`
9. If denied: redirect to `/login?error=unauthorized`

**Logging**: Log successful logins (`slog.Info`) and denied logins (`slog.Warn`) with username and reason.

### Environment Variables

| Variable | Required | Purpose |
|----------|----------|---------|
| `GITHUB_CLIENT_ID` | Prod | GitHub OAuth App client ID |
| `GITHUB_CLIENT_SECRET` | Prod | GitHub OAuth App client secret |
| `ALLOWED_GITHUB_USERS` | No | Comma-separated GitHub usernames |
| `ALLOWED_GITHUB_ORGS` | No | Comma-separated GitHub org logins |
| `SESSION_SECRET` | Prod | Secret for HMAC-signing session cookie values |

**Startup validation**: If `NO_AUTH` is not `true`, the server must refuse to start if both `ALLOWED_GITHUB_USERS` and `ALLOWED_GITHUB_ORGS` are empty. Log error and `os.Exit(1)`, consistent with existing startup failure patterns.

**Note on allowlists**: If a user changes their GitHub username after being added to `ALLOWED_GITHUB_USERS`, they lose access. This is expected — update the allowlist accordingly.

## Cookie Format

The session cookie value is `sessionID.hmacSignature`:
- `sessionID` is the UUID from the `sessions` table
- `hmacSignature` is `HMAC-SHA256(sessionID, SESSION_SECRET)`, hex-encoded
- On validation: split cookie value at `.`, verify HMAC, then look up session in DB
- This prevents session ID forgery even if an attacker can guess UUIDs

Cookie settings:
| Setting | Value |
|---------|-------|
| Name | `session` |
| HttpOnly | `true` |
| Secure | `true` by default; set `SECURE_COOKIES=false` for local dev |
| SameSite | `Lax` |
| Path | `/` |

**CSRF note**: `SameSite=Lax` prevents cookies from being sent on cross-origin POST requests, providing CSRF protection for the logout endpoint without needing a separate CSRF token.

## Database Schema

Two new tables added to `internal/db/migrations.go`:

```sql
CREATE TABLE IF NOT EXISTS users (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    github_id       BIGINT NOT NULL UNIQUE,
    github_login    VARCHAR(100) NOT NULL,
    name            VARCHAR(200),
    avatar_url      TEXT,
    created_at      TIMESTAMPTZ DEFAULT NOW(),
    updated_at      TIMESTAMPTZ DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS sessions (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id     UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    expires_at  TIMESTAMPTZ NOT NULL,
    created_at  TIMESTAMPTZ DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_sessions_user_id ON sessions(user_id);
CREATE INDEX IF NOT EXISTS idx_sessions_expires_at ON sessions(expires_at);
```

- Users are upserted on each login (updates login, name, avatar_url, updated_at)
- Sessions expire after 7 days, no sliding window — user re-authenticates after expiry
- When a session expires mid-use, the next API call returns 401 and the frontend redirects to `/login`
- Deleting a user cascades to their sessions

### Future extensibility

- Email/password: add `email`, `password_hash` columns to `users`
- Google OAuth: add `google_id` column, new `/auth/google` routes
- Multi-provider linking: add `user_identities(user_id, provider, provider_id, email)` table
- Org-based access control: add `organizations` and `org_members` tables
- None of these require changing the session or cookie mechanism

## Backend Package: `internal/auth/`

| File | Responsibility |
|------|----------------|
| `oauth.go` | GitHub OAuth: authorize redirect, callback, token exchange, user info fetch |
| `session.go` | Session CRUD: create, validate (verify HMAC + DB lookup), delete, cleanup expired |
| `middleware.go` | `RequireSession` middleware: check cookie, load user, inject into context |
| `allowlist.go` | Parse env vars, check username/org membership, startup validation |

### Auth middleware integration

The existing `Auth` middleware in `internal/api/auth.go` is modified to check two auth methods in order:

1. **Bearer token** (existing): If `Authorization: Bearer ...` header is present, validate via `KeyStore.ValidateKey()`. If invalid, return 401.
2. **Session cookie** (new): If no Bearer header, check for `session` cookie. Parse and verify HMAC signature, look up session in DB via a new `SessionStore` interface, load user. If invalid/expired, return 401.
3. If neither is present, return 401.

**`SessionStore` interface** (implemented by `internal/auth/session.go`):
```go
type SessionStore interface {
    ValidateSession(ctx context.Context, sessionID string) (*User, error)
}
```

**Context key**: Authenticated user is stored in request context using `type contextKey string` with key `"user"`. API handlers access it via `auth.UserFromContext(ctx)` which returns `*auth.User` or nil.

**`Auth` function signature** changes from `Auth(store KeyStore)` to `Auth(keys KeyStore, sessions SessionStore)`.

Route auth mapping:

```
/auth/github          → no auth (public)
/auth/github/callback → no auth (public)
/auth/me              → session cookie only (uses RequireSession)
/auth/logout          → session cookie only (uses RequireSession)
/api/v1/health        → no auth (existing)
/api/v1/*             → API key OR session cookie (uses modified Auth)
```

## Frontend Changes

### New files

| File | Purpose |
|------|---------|
| `web/app/login/page.tsx` | Login page: app logo, "Sign in with GitHub" button, error display |
| `web/lib/auth/context.tsx` | `AuthProvider` + `useAuth()` hook |

### Auth context

```
AuthProvider
  → on mount: fetch /auth/me
  → if 200: set user state, render children
  → if 401 AND pathname is not /login: redirect to /login
  → if on /login: render login page without redirect
  → shows loading state while checking
```

The `/login` page is excluded from the auth redirect loop — `AuthProvider` checks `window.location.pathname` before redirecting.

### Layout changes (`web/app/layout.tsx`)

- Wrap app in `AuthProvider`
- Unauthenticated users are redirected to `/login` (except when already on `/login`)

### Sidebar changes

- Add user avatar + dropdown with "Sign out" option
- Sign out POSTs to `/auth/logout`, redirects to `/login`

### No changes to API client

`web/lib/api/client.ts` uses `fetch()` which automatically sends `HttpOnly` cookies with same-origin requests. No Authorization header needed for session-based auth.

## Development & NO_AUTH Mode

When `NO_AUTH=true`:
- API key auth bypass continues (existing)
- Session auth middleware bypasses — injects a fake dev user
- `/auth/me` returns a hardcoded dev user
- No `GITHUB_CLIENT_ID` / `GITHUB_CLIENT_SECRET` needed
- Login page accessible but not required
- Startup validation for allowlists is skipped

## Session Cleanup

A background goroutine runs hourly to delete expired sessions (`expires_at < NOW()`). Starts alongside the existing polling engine in `cmd/server/main.go`. Respects context cancellation for clean shutdown, using `time.Ticker`.

## State Parameter Storage

OAuth `state` values stored in an in-memory `sync.Map` with:
- 10-minute TTL per entry
- Maximum 1000 pending states (rejects new `/auth/github` requests with 429 if full)
- Cleanup goroutine sweeps expired entries every 5 minutes
- Sufficient for single-instance deployment; can be moved to PostgreSQL if multi-instance is needed later
