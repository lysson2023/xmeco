package handler

import (
	"encoding/json"
	"net/http"

	"xmeco/internal/domain"
	"xmeco/internal/repository/postgres"
)

type ProjectHandler struct {
	repo *postgres.ProjectRepo
}

func NewProjectHandler(repo *postgres.ProjectRepo) *ProjectHandler {
	return &ProjectHandler{repo: repo}
}

func (h *ProjectHandler) List(w http.ResponseWriter, r *http.Request) {
	projects, err := h.repo.List(r.Context())
	if err != nil {
		serverErr(w, err)
		return
	}
	if projects == nil {
		projects = []domain.Project{}
	}
	writeJSON(w, http.StatusOK, projects)
}

func (h *ProjectHandler) Create(w http.ResponseWriter, r *http.Request) {
	var p domain.Project
	if err := json.NewDecoder(r.Body).Decode(&p); err != nil {
		writeJSON(w, http.StatusBadRequest, M{"error": "请求格式错误"})
		return
	}
	if err := h.repo.Create(r.Context(), &p); err != nil {
		writeJSON(w, http.StatusConflict, M{"error": "项目创建失败"})
		return
	}
	writeJSON(w, http.StatusCreated, p)
}

func (h *ProjectHandler) Get(w http.ResponseWriter, r *http.Request) {
	p, err := h.repo.GetByID(r.Context(), pathID(r))
	if err != nil {
		serverErr(w, err)
		return
	}
	if p == nil {
		writeJSON(w, http.StatusNotFound, M{"error": "项目不存在"})
		return
	}
	writeJSON(w, http.StatusOK, p)
}

func (h *ProjectHandler) Update(w http.ResponseWriter, r *http.Request) {
	var p domain.Project
	if err := json.NewDecoder(r.Body).Decode(&p); err != nil {
		writeJSON(w, http.StatusBadRequest, M{"error": "请求格式错误"})
		return
	}
	p.ID = pathID(r)
	if err := h.repo.Update(r.Context(), &p); err != nil {
		writeJSON(w, http.StatusInternalServerError, M{"error": "项目更新失败"})
		return
	}
	writeJSON(w, http.StatusOK, p)
}

func (h *ProjectHandler) Delete(w http.ResponseWriter, r *http.Request) {
	if err := h.repo.Delete(r.Context(), pathID(r)); err != nil {
		writeJSON(w, http.StatusInternalServerError, M{"error": "项目删除失败"})
		return
	}
	writeJSON(w, http.StatusOK, M{"status": "deleted"})
}

// GetProjectUsers returns user IDs assigned to this project.
func (h *ProjectHandler) GetProjectUsers(w http.ResponseWriter, r *http.Request) {
	ids, err := h.repo.ListProjectUsers(r.Context(), pathID(r))
	if err != nil {
		serverErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, M{"user_ids": ids})
}

// SetProjectUsers replaces user assignments for a project.
func (h *ProjectHandler) SetProjectUsers(w http.ResponseWriter, r *http.Request) {
	var body struct {
		UserIDs []int `json:"user_ids"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeJSON(w, http.StatusBadRequest, M{"error": "请求格式错误"})
		return
	}
	if body.UserIDs == nil { body.UserIDs = []int{} }
	if err := h.repo.SetProjectUsers(r.Context(), pathID(r), body.UserIDs); err != nil {
		serverErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, M{"status": "updated"})
}
