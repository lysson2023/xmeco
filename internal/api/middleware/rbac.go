package middleware

import (
	"encoding/json"
	"log/slog"
	"net/http"

	"xmeco/internal/service/auth"
)

func RequirePermission(authSvc *auth.Service, permCode string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			claims := GetClaims(r)
			if claims == nil {
				writeJSONError(w, http.StatusUnauthorized, "未认证")
				return
			}
			// 超管绕过权限检查——设计意图：新增权限点时超管自动拥有，无需 DB 配置
			if claims.RoleCode == auth.RoleSuperAdmin {
				next.ServeHTTP(w, r)
				return
			}
			has, err := authSvc.HasPermission(r.Context(), claims.UserID, permCode)
			if err != nil {
				writeJSONError(w, http.StatusInternalServerError, "权限服务异常")
				return
			}
			if !has {
				writeJSONError(w, http.StatusForbidden, "无此操作权限")
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// writeJSONError writes a JSON error response with correct Content-Type.
func writeJSONError(w http.ResponseWriter, status int, msg string) {
	// Marshal first to avoid sending partial body after header commit.
	body, err := json.Marshal(map[string]string{"error": msg})
	if err != nil {
		slog.Error("writeJSONError marshal failed", "err", err)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(`{"error":"内部服务器错误"}`))
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if _, err := w.Write(body); err != nil {
		slog.Warn("writeJSONError write failed", "err", err)
	}
}
