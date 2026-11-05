package middleware

import "net/http"

// BodyLimit caps the request body at maxBytes. Requests matched by one of skip
// are left alone: their handler applies its own limit (e.g. image uploads).
func BodyLimit(maxBytes int64, skip ...func(*http.Request) bool) func(http.Handler) http.Handler {
	if maxBytes <= 0 {
		return func(next http.Handler) http.Handler {
			return next
		}
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			for _, s := range skip {
				if s(r) {
					next.ServeHTTP(w, r)
					return
				}
			}
			if r.ContentLength > maxBytes {
				http.Error(w, http.StatusText(http.StatusRequestEntityTooLarge), http.StatusRequestEntityTooLarge)
				return
			}

			r.Body = http.MaxBytesReader(w, r.Body, maxBytes)
			next.ServeHTTP(w, r)
		})
	}
}
