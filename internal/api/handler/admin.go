package handler

import (
	"encoding/json"
	"net/http"

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
		writeJSON(w, http.StatusBadRequest, M{"error": "请求格式错误"})
		return
	}
	if req.Username == "" || req.Password == "" || req.RoleID == 0 {
		writeJSON(w, http.StatusBadRequest, M{"error": "用户名、密码、角色不能为空"})
		return
	}
	hash, err := auth.HashPassword(req.Password)
	if err != nil {
		serverErr(w, err)
		return
	}
	user, err := h.repo.CreateUser(r.Context(), req, hash)
	if err != nil {
		writeJSON(w, http.StatusConflict, M{"error": "用户名已存在或创建失败: " + err.Error()})
		return
	}
	created(w, user)
}

func (h *AdminHandler) UpdateUser(w http.ResponseWriter, r *http.Request) {
	var body struct {
		RoleID   int   `json:"role_id"`
		AgentID  *int  `json:"agent_id"`
		IsActive *bool `json:"is_active"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeJSON(w, http.StatusBadRequest, M{"error": "请求格式错误"})
		return
	}
	id := pathLast(r.URL.Path)
	isActive := true
	if body.IsActive != nil {
		isActive = *body.IsActive
	}
	if err := h.repo.UpdateUser(r.Context(), id, body.RoleID, body.AgentID, isActive); err != nil {
		writeJSON(w, http.StatusNotFound, M{"error": "用户不存在"})
		return
	}
	ok(w, M{"status": "updated"})
}

func (h *AdminHandler) ResetPassword(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.Password == "" {
		writeJSON(w, http.StatusBadRequest, M{"error": "请输入新密码"})
		return
	}
	hash, err := auth.HashPassword(body.Password)
	if err != nil {
		serverErr(w, err)
		return
	}
	if err := h.repo.ResetPassword(r.Context(), pathLast(r.URL.Path), hash); err != nil {
		writeJSON(w, http.StatusNotFound, M{"error": "用户不存在"})
		return
	}
	ok(w, M{"status": "password_reset"})
}

func (h *AdminHandler) DeleteUser(w http.ResponseWriter, r *http.Request) {
	if err := h.repo.DeleteUser(r.Context(), pathLast(r.URL.Path)); err != nil {
		serverErr(w, err)
		return
	}
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
	var body struct {
		Name string `json:"name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.Name == "" {
		writeJSON(w, http.StatusBadRequest, M{"error": "名称不能为空"})
		return
	}
	if err := h.repo.UpdateAgent(r.Context(), pathLast(r.URL.Path), body.Name); err != nil {
		writeJSON(w, http.StatusNotFound, M{"error": "代理商不存在"})
		return
	}
	ok(w, M{"status": "updated"})
}

func (h *AdminHandler) DeleteAgent(w http.ResponseWriter, r *http.Request) {
	if err := h.repo.DeleteAgent(r.Context(), pathLast(r.URL.Path)); err != nil {
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
	ids, err := h.repo.ListRolePermissions(r.Context(), pathLast(r.URL.Path))
	if err != nil {
		serverErr(w, err)
		return
	}
	if ids == nil {
		ids = []int{}
	}
	ok(w, map[string]interface{}{"role_id": pathLast(r.URL.Path), "perm_ids": ids})
}

func (h *AdminHandler) SetRolePermissions(w http.ResponseWriter, r *http.Request) {
	var body struct {
		PermIDs []int `json:"perm_ids"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeJSON(w, http.StatusBadRequest, M{"error": "请求格式错误"})
		return
	}
	if err := h.repo.SetRolePermissions(r.Context(), pathLast(r.URL.Path), body.PermIDs); err != nil {
		serverErr(w, err)
		return
	}
	ok(w, M{"status": "updated"})
}