package handler

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"

	"xmeco/internal/domain"
	"xmeco/internal/repository/postgres"
)

type M map[string]string

func queryInt(r *http.Request, key string) int {
	v, _ := strconv.Atoi(r.URL.Query().Get(key))
	return v
}
func pathLast(path string) int {
	parts := strings.Split(strings.TrimRight(path, "/"), "/")
	id, _ := strconv.Atoi(parts[len(parts)-1])
	return id
}
func writeJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}
func ok(w http.ResponseWriter, v interface{})     { writeJSON(w, http.StatusOK, v) }
func created(w http.ResponseWriter, v interface{}) { writeJSON(w, http.StatusCreated, v) }
func notFound(w http.ResponseWriter, msg string)   { writeJSON(w, http.StatusNotFound, M{"error": msg}) }
func serverErr(w http.ResponseWriter, err error)   { writeJSON(w, http.StatusInternalServerError, M{"error": err.Error()}) }

// ===== BUILDING =====
type BuildingHandler struct{ repo *postgres.BuildingRepo }
func NewBuildingHandler(r *postgres.BuildingRepo) *BuildingHandler { return &BuildingHandler{r} }

func (h *BuildingHandler) List(w http.ResponseWriter, r *http.Request) {
	list, err := h.repo.List(r.Context(), queryInt(r, "project_id"))
	if err != nil { serverErr(w, err); return }
	if list == nil { list = []domain.Building{} }
	ok(w, list)
}
func (h *BuildingHandler) Get(w http.ResponseWriter, r *http.Request) {
	b, err := h.repo.GetByID(r.Context(), pathLast(r.URL.Path))
	if err != nil { notFound(w, "楼宇不存在"); return }
	ok(w, b)
}
func (h *BuildingHandler) Create(w http.ResponseWriter, r *http.Request) {
	var b domain.Building
	if err := json.NewDecoder(r.Body).Decode(&b); err != nil { writeJSON(w, http.StatusBadRequest, M{"error": "请求格式错误"}); return }
	if err := h.repo.Create(r.Context(), &b); err != nil { serverErr(w, err); return }
	created(w, b)
}
func (h *BuildingHandler) Update(w http.ResponseWriter, r *http.Request) {
	var b domain.Building
	if err := json.NewDecoder(r.Body).Decode(&b); err != nil { writeJSON(w, http.StatusBadRequest, M{"error": "请求格式错误"}); return }
	b.ID = pathLast(r.URL.Path)
	if err := h.repo.Update(r.Context(), &b); err != nil { serverErr(w, err); return }
	ok(w, b)
}
func (h *BuildingHandler) Delete(w http.ResponseWriter, r *http.Request) {
	if err := h.repo.Delete(r.Context(), pathLast(r.URL.Path)); err != nil { serverErr(w, err); return }
	ok(w, M{"status": "deleted"})
}

// ===== DEVICE =====
type DeviceHandler struct{ repo *postgres.DeviceRepo }
func NewDeviceHandler(r *postgres.DeviceRepo) *DeviceHandler { return &DeviceHandler{r} }

func (h *DeviceHandler) List(w http.ResponseWriter, r *http.Request) {
	list, err := h.repo.List(r.Context(), queryInt(r, "building_id"))
	if err != nil { serverErr(w, err); return }
	if list == nil { list = []domain.Device{} }
	ok(w, list)
}
func (h *DeviceHandler) Get(w http.ResponseWriter, r *http.Request) {
	d, err := h.repo.GetByID(r.Context(), pathLast(r.URL.Path))
	if err != nil { notFound(w, "设备不存在"); return }
	ok(w, d)
}
func (h *DeviceHandler) Create(w http.ResponseWriter, r *http.Request) {
	var d domain.Device
	if err := json.NewDecoder(r.Body).Decode(&d); err != nil { writeJSON(w, http.StatusBadRequest, M{"error": "请求格式错误"}); return }
	if err := h.repo.Create(r.Context(), &d); err != nil { serverErr(w, err); return }
	created(w, d)
}
func (h *DeviceHandler) Update(w http.ResponseWriter, r *http.Request) {
	var d domain.Device
	if err := json.NewDecoder(r.Body).Decode(&d); err != nil { writeJSON(w, http.StatusBadRequest, M{"error": "请求格式错误"}); return }
	d.ID = pathLast(r.URL.Path)
	if err := h.repo.Update(r.Context(), &d); err != nil { serverErr(w, err); return }
	ok(w, d)
}
func (h *DeviceHandler) Delete(w http.ResponseWriter, r *http.Request) {
	if err := h.repo.Delete(r.Context(), pathLast(r.URL.Path)); err != nil { serverErr(w, err); return }
	ok(w, M{"status": "deleted"})
}

// ===== PROPERTY =====
type PropertyHandler struct{ repo *postgres.PropertyRepo }
func NewPropertyHandler(r *postgres.PropertyRepo) *PropertyHandler { return &PropertyHandler{r} }

func (h *PropertyHandler) List(w http.ResponseWriter, r *http.Request) {
	list, err := h.repo.List(r.Context(), queryInt(r, "device_id"))
	if err != nil { serverErr(w, err); return }
	if list == nil { list = []domain.DeviceProperty{} }
	ok(w, list)
}
func (h *PropertyHandler) Get(w http.ResponseWriter, r *http.Request) {
	p, err := h.repo.GetByID(r.Context(), pathLast(r.URL.Path))
	if err != nil { notFound(w, "属性不存在"); return }
	ok(w, p)
}
func (h *PropertyHandler) Create(w http.ResponseWriter, r *http.Request) {
	var p domain.DeviceProperty
	if err := json.NewDecoder(r.Body).Decode(&p); err != nil { writeJSON(w, http.StatusBadRequest, M{"error": "请求格式错误"}); return }
	if err := h.repo.Create(r.Context(), &p); err != nil { serverErr(w, err); return }
	created(w, p)
}
func (h *PropertyHandler) Update(w http.ResponseWriter, r *http.Request) {
	var p domain.DeviceProperty
	if err := json.NewDecoder(r.Body).Decode(&p); err != nil { writeJSON(w, http.StatusBadRequest, M{"error": "请求格式错误"}); return }
	p.ID = pathLast(r.URL.Path)
	if err := h.repo.Update(r.Context(), &p); err != nil { serverErr(w, err); return }
	ok(w, p)
}
func (h *PropertyHandler) Delete(w http.ResponseWriter, r *http.Request) {
	if err := h.repo.Delete(r.Context(), pathLast(r.URL.Path)); err != nil { serverErr(w, err); return }
	ok(w, M{"status": "deleted"})
}

// ===== REGISTER =====
type RegisterHandler struct{ repo *postgres.RegisterRepo }
func NewRegisterHandler(r *postgres.RegisterRepo) *RegisterHandler { return &RegisterHandler{r} }

func (h *RegisterHandler) List(w http.ResponseWriter, r *http.Request) {
	list, err := h.repo.List(r.Context(), queryInt(r, "property_id"))
	if err != nil { serverErr(w, err); return }
	if list == nil { list = []domain.Register{} }
	ok(w, list)
}
func (h *RegisterHandler) Get(w http.ResponseWriter, r *http.Request) {
	reg, err := h.repo.GetByID(r.Context(), pathLast(r.URL.Path))
	if err != nil { notFound(w, "寄存器不存在"); return }
	ok(w, reg)
}
func (h *RegisterHandler) Create(w http.ResponseWriter, r *http.Request) {
	var reg domain.Register
	if err := json.NewDecoder(r.Body).Decode(&reg); err != nil { writeJSON(w, http.StatusBadRequest, M{"error": "请求格式错误"}); return }
	if err := h.repo.Create(r.Context(), &reg); err != nil { serverErr(w, err); return }
	created(w, reg)
}
func (h *RegisterHandler) Update(w http.ResponseWriter, r *http.Request) {
	var reg domain.Register
	if err := json.NewDecoder(r.Body).Decode(&reg); err != nil { writeJSON(w, http.StatusBadRequest, M{"error": "请求格式错误"}); return }
	reg.ID = pathLast(r.URL.Path)
	if err := h.repo.Update(r.Context(), &reg); err != nil { serverErr(w, err); return }
	ok(w, reg)
}
func (h *RegisterHandler) Delete(w http.ResponseWriter, r *http.Request) {
	if err := h.repo.Delete(r.Context(), pathLast(r.URL.Path)); err != nil { serverErr(w, err); return }
	ok(w, M{"status": "deleted"})
}
