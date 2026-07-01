package middleware

import (
	"context"
	"log/slog"
	"net/http"
	"strings"

	"xmeco/internal/service/auth"
)

type contextKey struct{ name string }

var CtxClaims = contextKey{name: "claims"}

func AuthMiddleware(authSvc *auth.Service) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			authHeader := r.Header.Get("Authorization")
		if authHeader == "" || !strings.HasPrefix(authHeader, "Bearer ") {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusUnauthorized)
			if _, err := w.Write([]byte(`{"error":"未提供认证令牌"}`)); err != nil {
				slog.Warn("auth middleware write failed", "err", err)
			}
			return
		}
		tokenStr := strings.TrimPrefix(authHeader, "Bearer ")
		claims, err := authSvc.ValidateToken(r.Context(), tokenStr)
		if err != nil {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusUnauthorized)
			if _, err := w.Write([]byte(`{"error":"认证令牌无效或已过期"}`)); err != nil {
				slog.Warn("auth middleware write failed", "err", err)
			}
			return
		}
			ctx := context.WithValue(r.Context(), CtxClaims, claims)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func GetClaims(r *http.Request) *auth.Claims {
	if claims, ok := r.Context().Value(CtxClaims).(*auth.Claims); ok {
		return claims
	}
	return nil
}
