package middleware

import (
	"context"
	"net/http"
	"strings"

	"xmeco/internal/service/auth"
)

type contextKey string

const (
	CtxClaims contextKey = "claims"
)

func AuthMiddleware(authSvc *auth.Service) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			authHeader := r.Header.Get("Authorization")
			if authHeader == "" || !strings.HasPrefix(authHeader, "Bearer ") {
				http.Error(w, `{"error":"未提供认证令牌"}`, http.StatusUnauthorized)
				return
			}
			tokenStr := strings.TrimPrefix(authHeader, "Bearer ")
			claims, err := authSvc.ValidateToken(tokenStr)
			if err != nil {
				http.Error(w, `{"error":"认证令牌无效或已过期"}`, http.StatusUnauthorized)
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
