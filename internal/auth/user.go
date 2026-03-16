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
