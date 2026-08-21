package auth

import (
	"context"
	"net/http"
	"strings"

	"github.com/bruhjeshhh/delivery-tracker/internal/models"
)

type ctxKey string

const claimsKey ctxKey = "claims"

// Middleware validates the Bearer token and injects Claims into the request context.
func Middleware(secret string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			h := r.Header.Get("Authorization")
			if !strings.HasPrefix(h, "Bearer ") {
				http.Error(w, `{"error":"missing bearer token"}`, http.StatusUnauthorized)
				return
			}
			token := strings.TrimPrefix(h, "Bearer ")
			claims, err := ParseToken(secret, token)
			if err != nil {
				http.Error(w, `{"error":"invalid or expired token"}`, http.StatusUnauthorized)
				return
			}
			ctx := context.WithValue(r.Context(), claimsKey, claims)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// RequireRole restricts a handler to one or more roles. Call after Middleware.
func RequireRole(roles ...models.UserRole) func(http.Handler) http.Handler {
	allowed := make(map[models.UserRole]bool, len(roles))
	for _, r := range roles {
		allowed[r] = true
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			claims := FromContext(r.Context())
			if claims == nil || !allowed[claims.Role] {
				http.Error(w, `{"error":"forbidden"}`, http.StatusForbidden)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

func FromContext(ctx context.Context) *Claims {
	c, _ := ctx.Value(claimsKey).(*Claims)
	return c
}
