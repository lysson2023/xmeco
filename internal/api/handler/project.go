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
		writeJSON(w, http.StatusInternalServerError, M{"error": err.Error()})
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
		writeJSON(w, http.StatusInternalServerError, M{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusCreated, p)
}

func (h *ProjectHandler) Get(w http.ResponseWriter, r *http.Request) {
	p, err := h.repo.GetByID(r.Context(), pathLast(r.URL.Path))
	if err != nil {
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
	p.ID = pathLast(r.URL.Path)
	if err := h.repo.Update(r.Context(), &p); err != nil {
		writeJSON(w, http.StatusInternalServerError, M{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, p)
}

func (h *ProjectHandler) Delete(w http.ResponseWriter, r *http.Request) {
	if err := h.repo.Delete(r.Context(), pathLast(r.URL.Path)); err != nil {
		writeJSON(w, http.StatusInternalServerError, M{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, M{"status": "deleted"})
}
