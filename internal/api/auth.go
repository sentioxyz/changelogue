package api

import (
	"context"
	"net/http"
	"strings"
	"sync"

	"golang.org/x/time/rate"

	"github.com/sentioxyz/changelogue/internal/auth"
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

func RateLimit(rps float64, burst int) Middleware {
	var mu sync.Mutex
	limiters := make(map[string]*rate.Limiter)
	getLimiter := func(key string) *rate.Limiter {
		mu.Lock()
		defer mu.Unlock()
		if lim, ok := limiters[key]; ok {
			return lim
		}
		lim := rate.NewLimiter(rate.Limit(rps), burst)
		limiters[key] = lim
		return lim
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			key := r.RemoteAddr
			if auth := r.Header.Get("Authorization"); strings.HasPrefix(auth, "Bearer ") {
				key = strings.TrimPrefix(auth, "Bearer ")
			}
			lim := getLimiter(key)
			if !lim.Allow() {
				w.Header().Set("Retry-After", "1")
				RespondError(w, r, http.StatusTooManyRequests, "rate_limited", "Too many requests")
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}
