package handler

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"

	"xmeco/internal/api/middleware"
	"xmeco/internal/domain"
	"xmeco/internal/repository/postgres"
	"xmeco/internal/service/auth"
)

type AdminHandler struct {
	repo    *postgres.AdminRepo
	authSvc *auth.Service
}

func NewAdminHandler(repo *postgres.AdminRepo, authSvc *auth.Service) *AdminHandler {
	return &AdminHandler{repo: repo, authSvc: authSvc}
}

// validatePasswordLength 校验密码最小长度为 8 位。
func validatePasswordLength(s string) error {
	if len(s) < 8 {
		return fmt.Errorf("密码长度不能少于 8 位")
	}
	return nil
}

// ==================== 用户管理 ====================

func (h *AdminHandler) ListUsers(w http.ResponseWriter, r *http.Request) {
	users, err := h.repo.ListUsers(r.Context())
	if err != nil {
		serverErr(w, err)
		return
	}
	if users == nil {
		users = []domain.AdminUser{}
	}
	ok(w, users)
}

func (h *AdminHandler) CreateUser(w http.ResponseWriter, r *http.Request) {
	var req domain.CreateUserReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, errBadRequest)
		return
	}
	if req.Username == "" || req.Password == "" || req.RoleID == 0 {
		writeJSON(w, http.StatusBadRequest, M{"error": "用户名、密码、角色不能为空"})
		return
	}
	if err := validatePasswordLength(req.Password); err != nil {
		writeJSON(w, http.StatusBadRequest, M{"error": err.Error()})
		return
	}
	hash, err := auth.HashPassword(req.Password)
	if err != nil {
		serverErr(w, err)
		return
	}
	user, err := h.repo.CreateUser(r.Context(), req, hash)
	if err != nil {
		slog.Warn("CreateUser failed", "username", req.Username, "err", err)
		writeJSON(w, http.StatusConflict, M{"error": "用户名已存在或创建失败"})
		return
	}
	created(w, user)
}

func (h *AdminHandler) UpdateUser(w http.ResponseWriter, r *http.Request) {
	var body struct {
		RoleID           int     `json:"role_id"`
		AgentID          *int    `json:"agent_id"`
		IsActive         *bool   `json:"is_active"`
		Remark           *string `json:"remark"`
		DefaultProjectID *int    `json:"default_project_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeJSON(w, http.StatusBadRequest, errBadRequest)
		return
	}
	id := pathID(r)
	if id <= 0 {
		writeJSON(w, http.StatusBadRequest, M{"error": "无效的用户 ID"})
		return
	}
	claims := middleware.GetClaims(r)
	if claims == nil {
		writeJSON(w, http.StatusUnauthorized, M{"error": "未认证"})
		return
	}
	// Only super_admin or the user themselves can update user info.
	if claims.RoleCode != auth.RoleSuperAdmin && claims.UserID != id {
		writeJSON(w, http.StatusForbidden, M{"error": "无权限修改该用户"})
		return
	}

	// Validate default_project_id
	if body.DefaultProjectID != nil && *body.DefaultProjectID < 0 {
		writeJSON(w, http.StatusBadRequest, M{"error": "无效的项目 ID"})
		return
	}
	// roleID=0 skips the update (SQL uses CASE WHEN $1=0 THEN role_id ELSE $1 END).
	roleID := body.RoleID
	if body.IsActive == nil && body.RoleID == 0 && body.AgentID == nil && body.Remark == nil && body.DefaultProjectID == nil {
		writeJSON(w, http.StatusBadRequest, M{"error": "至少需要提供一个要更新的字段"})
		return
	}
	if err := h.repo.UpdateUser(r.Context(), id, roleID, body.AgentID, body.IsActive, body.Remark, body.DefaultProjectID); err != nil {
		slog.Warn("UpdateUser failed", "id", id, "err", err)
		writeJSON(w, http.StatusInternalServerError, M{"error": "更新失败"})
		return
	}
	// 禁用用户时递增 token_version，使已签发的 token 立即失效
	if body.IsActive != nil && !*body.IsActive {
		if err := h.repo.IncrementTokenVersion(r.Context(), id); err != nil {
			slog.Warn("increment token_version on disable failed", "user", id, "err", err)
		}
		h.authSvc.InvalidateTokenVersion(id)
	}
	ok(w, M{"status": "updated"})
}

func (h *AdminHandler) ResetPassword(w http.ResponseWriter, r *http.Request) {
	var body struct {
		OldPassword string `json:"old_password"`
		NewPassword string `json:"new_password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.NewPassword == "" {
		writeJSON(w, http.StatusBadRequest, M{"error": "请输入新密码"})
		return
	}
	if err := validatePasswordLength(body.NewPassword); err != nil {
		writeJSON(w, http.StatusBadRequest, M{"error": err.Error()})
		return
	}
	targetID := pathID(r)
	if targetID == 0 {
		writeJSON(w, http.StatusBadRequest, M{"error": "无效的用户 ID"})
		return
	}
	claims := middleware.GetClaims(r)
	if claims == nil {
		writeJSON(w, http.StatusUnauthorized, M{"error": "未认证"})
		return
	}
	// Only super_admin can reset any user's password;
	// regular users can only reset their own password.
	if claims.RoleCode != auth.RoleSuperAdmin && claims.UserID != targetID {
		writeJSON(w, http.StatusForbidden, M{"error": "无权限重置该用户密码"})
		return
	}

	// Verify old password:
	// - super_admin can skip this check by leaving old_password empty
	// - regular users MUST provide and pass old_password verification
	isSuperAdmin := claims.RoleCode == auth.RoleSuperAdmin
	if !isSuperAdmin && body.OldPassword == "" {
		writeJSON(w, http.StatusForbidden, M{"error": "请提供原密码"})
		return
	}
	if !isSuperAdmin || body.OldPassword != "" {
		currentHash, err := h.repo.GetPasswordHash(r.Context(), targetID)
		if err != nil {
			writeJSON(w, http.StatusNotFound, M{"error": "用户不存在"})
			return
		}
		if err := auth.CheckPassword(currentHash, body.OldPassword); err != nil {
			writeJSON(w, http.StatusForbidden, M{"error": "原密码错误"})
			return
		}
	}
	hash, err := auth.HashPassword(body.NewPassword)
	if err != nil {
		serverErr(w, err)
		return
	}
	if err := h.repo.ResetPassword(r.Context(), targetID, hash); err != nil {
		writeJSON(w, http.StatusNotFound, M{"error": "用户不存在"})
		return
	}
	// Invalidate all existing tokens for this user by incrementing token_version
	if err := h.repo.IncrementTokenVersion(r.Context(), targetID); err != nil {
		slog.Warn("increment token_version failed", "user", targetID, "err", err)
	}
	h.authSvc.InvalidateTokenVersion(targetID)
	ok(w, M{"status": "password_reset"})
}

func (h *AdminHandler) DeleteUser(w http.ResponseWriter, r *http.Request) {
	claims := middleware.GetClaims(r)
	if claims == nil {
		writeJSON(w, http.StatusUnauthorized, M{"error": "未认证"})
		return
	}
	// Only super_admin can delete users.
	if claims.RoleCode != auth.RoleSuperAdmin {
		writeJSON(w, http.StatusForbidden, M{"error": "无权限删除用户"})
		return
	}
	id := pathID(r)
	if err := h.repo.DeleteUser(r.Context(), id); err != nil {
		serverErr(w, err)
		return
	}
	// Invalidate all existing tokens for the deleted user
	if err := h.repo.IncrementTokenVersion(r.Context(), id); err != nil {
		slog.Warn("increment token_version on delete failed", "user", id, "err", err)
	}
	h.authSvc.InvalidateTokenVersion(id)
	ok(w, M{"status": "deleted"})
}

// ==================== 代理商管理 ====================

func (h *AdminHandler) ListAgents(w http.ResponseWriter, r *http.Request) {
	agents, err := h.repo.ListAgents(r.Context())
	if err != nil {
		serverErr(w, err)
		return
	}
	if agents == nil {
		agents = []domain.Agent{}
	}
	ok(w, agents)
}

func (h *AdminHandler) CreateAgent(w http.ResponseWriter, r *http.Request) {
	claims := middleware.GetClaims(r)
	if claims == nil {
		writeJSON(w, http.StatusUnauthorized, M{"error": "未认证"})
		return
	}
	// Only super_admin can create agents.
	if claims.RoleCode != auth.RoleSuperAdmin {
		writeJSON(w, http.StatusForbidden, M{"error": "无权限创建代理商"})
		return
	}
	var body struct {
		Name string `json:"name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.Name == "" {
		writeJSON(w, http.StatusBadRequest, M{"error": "名称不能为空"})
		return
	}
	agent, err := h.repo.CreateAgent(r.Context(), body.Name)
	if err != nil {
		serverErr(w, err)
		return
	}
	created(w, agent)
}

func (h *AdminHandler) UpdateAgent(w http.ResponseWriter, r *http.Request) {
	claims := middleware.GetClaims(r)
	if claims == nil {
		writeJSON(w, http.StatusUnauthorized, M{"error": "未认证"})
		return
	}
	if claims.RoleCode != auth.RoleSuperAdmin {
		writeJSON(w, http.StatusForbidden, M{"error": "无权限修改代理商"})
		return
	}
	id := pathID(r)
	var body struct {
		Name string `json:"name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.Name == "" {
		writeJSON(w, http.StatusBadRequest, M{"error": "名称不能为空"})
		return
	}
	if err := h.repo.UpdateAgent(r.Context(), id, body.Name); err != nil {
		writeJSON(w, http.StatusNotFound, M{"error": "代理商不存在"})
		return
	}
	ok(w, M{"status": "updated"})
}

func (h *AdminHandler) DeleteAgent(w http.ResponseWriter, r *http.Request) {
	claims := middleware.GetClaims(r)
	if claims == nil {
		writeJSON(w, http.StatusUnauthorized, M{"error": "未认证"})
		return
	}
	if claims.RoleCode != auth.RoleSuperAdmin {
		writeJSON(w, http.StatusForbidden, M{"error": "无权限删除代理商"})
		return
	}
	id := pathID(r)
	if err := h.repo.DeleteAgent(r.Context(), id); err != nil {
		serverErr(w, err)
		return
	}
	ok(w, M{"status": "deleted"})
}

// ==================== 角色 & 权限管理 ====================

func (h *AdminHandler) ListRoles(w http.ResponseWriter, r *http.Request) {
	roles, err := h.repo.ListRoles(r.Context())
	if err != nil {
		serverErr(w, err)
		return
	}
	if roles == nil {
		roles = []domain.Role{}
	}
	ok(w, roles)
}

func (h *AdminHandler) ListPermissions(w http.ResponseWriter, r *http.Request) {
	perms, err := h.repo.ListPermissions(r.Context())
	if err != nil {
		serverErr(w, err)
		return
	}
	if perms == nil {
		perms = []domain.Permission{}
	}
	ok(w, perms)
}

func (h *AdminHandler) GetRolePermissions(w http.ResponseWriter, r *http.Request) {
	roleID := pathID(r)
	ids, err := h.repo.ListRolePermissions(r.Context(), roleID)
	if err != nil {
		serverErr(w, err)
		return
	}
	if ids == nil {
		ids = []int{}
	}
	ok(w, map[string]any{"role_id": roleID, "perm_ids": ids})
}

func (h *AdminHandler) SetRolePermissions(w http.ResponseWriter, r *http.Request) {
	claims := middleware.GetClaims(r)
	if claims == nil {
		writeJSON(w, http.StatusUnauthorized, M{"error": "未认证"})
		return
	}
	if claims.RoleCode != auth.RoleSuperAdmin {
		writeJSON(w, http.StatusForbidden, M{"error": "无权限设置角色权限"})
		return
	}
	var body struct {
		PermIDs []int `json:"perm_ids"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeJSON(w, http.StatusBadRequest, errBadRequest)
		return
	}
	roleID := pathID(r)
	if err := h.repo.SetRolePermissions(r.Context(), roleID, body.PermIDs); err != nil {
		serverErr(w, err)
		return
	}
	ok(w, M{"status": "updated"})
}

// SystemInfo returns basic system information. Requires system.config.
func (h *AdminHandler) SystemInfo(w http.ResponseWriter, r *http.Request) {
	info, err := h.repo.SystemInfo(r.Context())
	if err != nil {
		serverErr(w, err)
		return
	}
	ok(w, info)
}

// DBStats returns database statistics. Requires system.db.
func (h *AdminHandler) DBStats(w http.ResponseWriter, r *http.Request) {
	stats, err := h.repo.DBStats(r.Context())
	if err != nil {
		serverErr(w, err)
		return
	}
	ok(w, stats)
}
