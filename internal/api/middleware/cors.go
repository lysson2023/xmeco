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
			// 安全策略：wildcard 模式仅用于开发环境。
			// 携带 Authorization 头的凭据请求不反射 Origin（防止任意恶意网站跨域窃取数据）。
			// 开发环境可通过 XMECO_ALLOWED_ORIGINS 设置具体域名白名单来支持凭据请求。
			if r.Header.Get("Authorization") != "" {
				slog.Warn("CORS: credentialed request blocked in wildcard mode — set XMECO_ALLOWED_ORIGINS to explicit domains", "origin", reqOrigin)
				// 不设置 ACAO 头，浏览器将阻止响应（凭据请求不允许 *）
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
