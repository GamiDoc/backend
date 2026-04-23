package middleware

import (
	"fmt"
	"log/slog"
	"net/http"

	"github.com/gamidoc/backend/internal/http/response"
)

func Recovery(logger *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			defer func() {
				if recovered := recover(); recovered != nil {
					logger.Error(
						"http_panic",
						"request_id", GetRequestID(r.Context()),
						"method", r.Method,
						"path", r.URL.Path,
						"panic", fmt.Sprint(recovered),
					)
					response.WriteError(w, http.StatusInternalServerError, "INTERNAL_SERVER_ERROR", "Internal server error", nil)
				}
			}()

			next.ServeHTTP(w, r)
		})
	}
}
