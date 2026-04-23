package middleware

import (
	"context"
	"net/http"
	"strings"

	"github.com/gamidoc/backend/internal/http/response"
	"github.com/gamidoc/backend/internal/token"
)

type authUserIDKey struct{}
type authEmailKey struct{}

func RequireAuth(manager *token.Manager) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if manager == nil {
				response.WriteError(w, http.StatusInternalServerError, "INTERNAL_SERVER_ERROR", "Internal server error", nil)
				return
			}

			header := r.Header.Get("Authorization")
			if header == "" {
				response.WriteError(w, http.StatusUnauthorized, "UNAUTHORIZED", "Missing bearer token", nil)
				return
			}

			parts := strings.SplitN(header, " ", 2)
			if len(parts) != 2 || parts[0] != "Bearer" || strings.TrimSpace(parts[1]) == "" {
				response.WriteError(w, http.StatusUnauthorized, "UNAUTHORIZED", "Invalid bearer token", nil)
				return
			}

			claims, err := manager.Parse(parts[1])
			if err != nil {
				response.WriteError(w, http.StatusUnauthorized, "UNAUTHORIZED", "Invalid bearer token", nil)
				return
			}

			ctx := context.WithValue(r.Context(), authUserIDKey{}, claims.UserID)
			ctx = context.WithValue(ctx, authEmailKey{}, claims.Email)

			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func GetAuthUserID(ctx context.Context) string {
	value, ok := ctx.Value(authUserIDKey{}).(string)
	if !ok {
		return ""
	}
	return value
}

func GetAuthEmail(ctx context.Context) string {
	value, ok := ctx.Value(authEmailKey{}).(string)
	if !ok {
		return ""
	}
	return value
}
