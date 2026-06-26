package middleware

import (
	"log/slog"
	"net/http"
	"strings"
)

// CORS adds CORS headers. Pass allowed origins as a comma-separated list or "*" for development.
// In production, restrict origin to the actual frontend domain.
func CORS(allowedOrigins string, next http.Handler) http.Handler {
	origins := strings.FieldsFunc(allowedOrigins, func(r rune) bool { return r == ',' })
	originMap := make(map[string]bool, len(origins))
	wildcard := false
	for _, o := range origins {
		o = strings.TrimSpace(o)
		if o == "*" {
			wildcard = true
			break
		}
		originMap[o] = true
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		reqOrigin := r.Header.Get("Origin")
		if wildcard {
			// Per Fetch spec, Access-Control-Allow-Origin:* cannot be used with credentials.
			// If the request carries an Authorization header, echo the request origin instead
			// of using "*", otherwise the browser will block the response.
			if r.Header.Get("Authorization") != "" && reqOrigin != "" {
				w.Header().Set("Access-Control-Allow-Origin", reqOrigin)
				w.Header().Set("Vary", "Origin")
				slog.Warn("CORS: wildcard origin replaced due to credentialed request", "origin", reqOrigin)
			} else {
				w.Header().Set("Access-Control-Allow-Origin", "*")
			}
		} else if originMap[reqOrigin] {
			w.Header().Set("Access-Control-Allow-Origin", reqOrigin)
			w.Header().Set("Access-Control-Allow-Credentials", "true")
		} else if reqOrigin != "" {
			// Origin not in allowed list: do not set CORS headers (browser will block).
			// Fall through to serve the request anyway (middleware logs are the caller's responsibility).
		} else {
			// No Origin header (same-origin request): use first allowed origin as fallback.
			if len(origins) > 0 && origins[0] != "*" {
				w.Header().Set("Access-Control-Allow-Origin", origins[0])
			}
		}
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
		w.Header().Set("Access-Control-Max-Age", "86400")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}
