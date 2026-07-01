package handler

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"

	"xmeco/internal/api/middleware"
	"xmeco/internal/service/auth"
)

type AuthHandler struct {
	svc *auth.Service
}

func NewAuthHandler(svc *auth.Service) *AuthHandler {
	return &AuthHandler{svc: svc}
}

type loginReq struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

func (h *AuthHandler) Login(w http.ResponseWriter, r *http.Request) {
	var req loginReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, errBadRequest)
		return
	}
	token, user, err := h.svc.Login(r.Context(), req.Username, req.Password)
	if err != nil {
		if errors.Is(err, auth.ErrInternal) {
			slog.Error("Login internal error", "err", err)
			writeJSON(w, http.StatusInternalServerError, M{"error": "服务内部错误"})
			return
		}
		writeJSON(w, http.StatusUnauthorized, M{"error": "用户名或密码错误"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"token": token,
		"user":  user,
	})
}

func (h *AuthHandler) Me(w http.ResponseWriter, r *http.Request) {
	claims := middleware.GetClaims(r)
	if claims == nil {
		writeJSON(w, http.StatusUnauthorized, M{"error": "未认证"})
		return
	}
	// 查询角色中文名，让前端直接显示"超级管理员"而非 role code
	var roleName string
	if err := h.svc.Pool().QueryRow(r.Context(), `SELECT COALESCE(name,'') FROM role WHERE code=$1`, claims.RoleCode).Scan(&roleName); err != nil {
		slog.Warn("auth/me: role name query failed", "role", claims.RoleCode, "err", err)
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"user_id":   claims.UserID,
		"username":  claims.Username,
		"role":      claims.RoleCode,
		"role_name": roleName,
	})
}
