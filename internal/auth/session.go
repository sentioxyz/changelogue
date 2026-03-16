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
