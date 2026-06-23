package handler

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"strconv"
	"strings"

	"xmeco/internal/api/middleware"
	"xmeco/internal/domain"
	"xmeco/internal/gateway"
	"xmeco/internal/gateway/modbus"
	"xmeco/internal/repository/postgres"

	"github.com/jackc/pgx/v5/pgxpool"
)

type M map[string]any

func queryInt(r *http.Request, key string) int {
	v, _ := strconv.Atoi(r.URL.Query().Get(key))
	return v
}
func pathLast(path string) int {
	parts := strings.Split(strings.TrimRight(path, "/"), "/")
	id, _ := strconv.Atoi(parts[len(parts)-1])
	return id
}

// pathID returns the last numeric segment (skips suffixes like control, execute, ack).
func pathID(path string) int {
	parts := strings.Split(strings.TrimRight(path, "/"), "/")
	for i := len(parts) - 1; i >= 0; i-- {
		if id, err := strconv.Atoi(parts[i]); err == nil && id > 0 {
			return id
		}
	}
	return 0
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
type DeviceHandler struct {
	repo  *postgres.DeviceRepo
	pool  *pgxpool.Pool
	gwMgr *gateway.Manager
}

func NewDeviceHandler(r *postgres.DeviceRepo, pool *pgxpool.Pool) *DeviceHandler {
	return &DeviceHandler{repo: r, pool: pool}
}

// SetGwMgr sets the gateway manager for hardware device control.
func (h *DeviceHandler) SetGwMgr(mgr *gateway.Manager) { h.gwMgr = mgr }

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

func (h *DeviceHandler) Control(w http.ResponseWriter, r *http.Request) {
	id := pathID(r.URL.Path)
	if id == 0 {
		writeJSON(w, http.StatusBadRequest, M{"error": "无效的设备 ID"})
		return
	}
	var body struct {
		Action string `json:"action"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.Action == "" {
		writeJSON(w, http.StatusBadRequest, M{"error": "action 不能为空"})
		return
	}
	username := "unknown"
	if claims := middleware.GetClaims(r); claims != nil {
		username = claims.Username
	}
	controlVal := body.Action
	deviceStatus := ""
	switch body.Action {
	case "start":
		controlVal = "开机"
		deviceStatus = "开机"
	case "stop":
		controlVal = "关机"
		deviceStatus = "关机"
	}
	var devName, bldName, projName, gwImei, gwType string
	var nodeAddr, deviceNo int
	err := h.pool.QueryRow(r.Context(), `
		SELECT d.name, b.name, COALESCE(p.name, ''),
		       COALESCE(d.gateway_imei,''), COALESCE(d.gateway_type,'custom'),
		       d.node_address, d.device_no
		FROM device d
		JOIN building b ON b.id = d.building_id
		JOIN project p ON p.id = b.project_id
		WHERE d.id = $1`, id).Scan(&devName, &bldName, &projName, &gwImei, &gwType, &nodeAddr, &deviceNo)
	if err != nil {
		notFound(w, "设备不存在")
		return
	}
	_, err = h.pool.Exec(r.Context(), `
		INSERT INTO control_record (project_name, building_name, device_name, prop_name, control_value, username, user_remark)
		VALUES ($1, $2, $3, '设备控制', $4, $5, $6)`,
		projName, bldName, devName, controlVal, username, body.Action)
	if err != nil {
		serverErr(w, err)
		return
	}

	// Try hardware dispatch via gateway
	if h.gwMgr != nil && gwImei != "" {
		gw := h.gwMgr.GetGateway(gwImei)
		if gw != nil && gw.Transport != nil && gw.Transport.IsConnected() {
			var writeAddr *int
			var writeCode *string
			err := h.pool.QueryRow(r.Context(),
				`SELECT r.write_addr, r.write_code FROM register r
				 JOIN device_properties dp ON dp.id=r.property_id
				 WHERE dp.device_id=$1 AND r.write_addr>0 AND r.command_code=$2
				 ORDER BY r.id LIMIT 1`, id, body.Action).Scan(&writeAddr, &writeCode)
			if err == nil && writeAddr != nil && writeCode != nil {
				dn := deviceNo & 0xFF
				cmd := modbus.BuildWriteCommand(byte(dn), modbus.CodeFromStr(*writeCode), uint16(*writeAddr), 1)
				if gwType == "custom" {
					lora := []byte{byte(nodeAddr >> 8), byte(nodeAddr)}
					cmd = append(lora, cmd...)
				}
				if _, err := gw.Transport.SendAndReceive(cmd); err != nil {
					slog.Warn("gateway dispatch failed", "dev", id, "gw", gwImei, "err", err)
				} else {
					slog.Info("gateway dispatch ok", "dev", id, "gw", gwImei, "action", body.Action)
				}
			}
		}
	}

	if deviceStatus != "" {
		if _, err := h.pool.Exec(r.Context(), `UPDATE device SET device_status=$1 WHERE id=$2`, deviceStatus, id); err != nil {
			slog.Warn("Control update device_status failed", "dev", id, "err", err)
		}
	}
	ok(w, map[string]any{"status": "ok", "action": body.Action})
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
