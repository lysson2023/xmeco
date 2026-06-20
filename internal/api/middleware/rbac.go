package middleware

import (
	"net/http"

	"xmeco/internal/service/auth"
)

func RequirePermission(authSvc *auth.Service, permCode string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			claims := GetClaims(r)
			if claims == nil {
				http.Error(w, `{"error":"未认证"}`, http.StatusUnauthorized)
				return
			}
			if claims.RoleCode == "super_admin" {
				next.ServeHTTP(w, r)
				return
			}
			has, err := authSvc.HasPermission(r.Context(), claims.UserID, permCode)
			if err != nil {
				http.Error(w, `{"error":"权限服务异常"}`, http.StatusServiceUnavailable)
				return
			}
			if !has {
				http.Error(w, `{"error":"无此操作权限"}`, http.StatusForbidden)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}
