package api

import (
	"context"
	"net/http"
	"strings"
	"sync"

	"golang.org/x/time/rate"
)

type KeyStore interface {
	ValidateKey(ctx context.Context, rawKey string) (bool, error)
	TouchKeyUsage(ctx context.Context, rawKey string)
}

func Auth(store KeyStore) Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			header := r.Header.Get("Authorization")
			if !strings.HasPrefix(header, "Bearer ") {
				RespondError(w, r, http.StatusUnauthorized, "unauthorized", "Missing API key")
				return
			}
			rawKey := strings.TrimPrefix(header, "Bearer ")
			valid, err := store.ValidateKey(r.Context(), rawKey)
			if err != nil || !valid {
				RespondError(w, r, http.StatusUnauthorized, "unauthorized", "Invalid API key")
				return
			}
			go store.TouchKeyUsage(context.Background(), rawKey)
			next.ServeHTTP(w, r)
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
