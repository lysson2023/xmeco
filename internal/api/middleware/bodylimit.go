package middleware

import (
	"net/http"
)

// BodyLimit wraps the request body with http.MaxBytesReader to prevent
// large-payload denial-of-service attacks. Oversized reads will fail
// in downstream handlers (json.Decode returns an error).
func BodyLimit(maxBytes int64) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			r.Body = http.MaxBytesReader(w, r.Body, maxBytes)
			next.ServeHTTP(w, r)
		})
	}
}
