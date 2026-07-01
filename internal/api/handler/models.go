package handler

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"xmeco/internal/api/middleware"
	"xmeco/internal/domain"
	"xmeco/internal/gateway"
	"xmeco/internal/gateway/modbus"
	"xmeco/internal/repository/postgres"

	"github.com/jackc/pgx/v5"
)

type M map[string]any

// 常用错误响应，避免字符串字面量散落各处
var (
	errBadRequest   = M{"error": "请求格式错误"}
	errUnauthorized = M{"error": "未认证"}
	errInternal     = M{"error": "内部服务器错误"}
)

func queryInt(r *http.Request, key string) int {
	v := r.URL.Query().Get(key)
	if v == "" {
		return 0
	}
	i, err := strconv.Atoi(v)
	if err != nil {
		slog.Warn("queryInt: invalid integer", "key", key, "value", v)
		return 0
	}
	if i < 0 {
		slog.Warn("queryInt: negative value", "key", key, "value", v)
		return 0
	}
	return i
}

// pathID extracts a numeric path parameter "id" from the request using ServeMux routing.
// Returns 0 if the parameter is missing, non-numeric, or negative.
func pathID(r *http.Request) int {
	v := r.PathValue("id")
	if v == "" {
		return 0
	}
	id, err := strconv.Atoi(v)
	if err != nil {
		slog.Warn("pathID: invalid integer", "value", v)
		return 0
	}
	if id < 0 {
		slog.Warn("pathID: negative ID", "value", v)
		return 0
	}
	return id
}

// 日期格式常量，用于 isValidDate 校验。
const (
	dateFmt     = "2006-01-02"
	datetimeFmt = "2006-01-02 15:04:05"
	isoFmt      = "2006-01-02T15:04:05"
)

// isValidDate 校验字符串是否为合法的日期或时间格式。
// 支持 YYYY-MM-DD、YYYY-MM-DD HH:MM:SS、ISO 8601（不含时区）。
func isValidDate(s string) bool {
	for _, layout := range []string{dateFmt, datetimeFmt, isoFmt} {
		if _, err := time.Parse(layout, s); err == nil {
			return true
		}
	}
	return false
}

func writeJSON(w http.ResponseWriter, status int, v interface{}) {
	// Marshal first so we don't send partial body with a committed status code.
	body, err := json.Marshal(v)
	if err != nil {
		slog.Error("writeJSON marshal failed", "err", err)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		if _, werr := w.Write([]byte(`{"error":"内部服务器错误"}`)); werr != nil {
			slog.Warn("writeJSON: fallback error response write failed", "err", werr)
		}
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if _, err := w.Write(body); err != nil {
		slog.Warn("writeJSON: response write failed", "err", err)
	}
}
func ok(w http.ResponseWriter, v interface{})     { writeJSON(w, http.StatusOK, v) }
func created(w http.ResponseWriter, v interface{}) { writeJSON(w, http.StatusCreated, v) }
func notFound(w http.ResponseWriter, msg string)   { writeJSON(w, http.StatusNotFound, M{"error": msg}) }
func itos(i int) string { return strconv.Itoa(i) } // 供测试辅助 URL 构造

func serverErr(w http.ResponseWriter, err error) {
	slog.Error("internal server error", "err", err)
	writeJSON(w, http.StatusInternalServerError, M{"error": "内部服务器错误"})
}

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
	b, err := h.repo.GetByID(r.Context(), pathID(r))
	if err != nil { serverErr(w, err); return }
	if b == nil { notFound(w, "楼宇不存在"); return }
	ok(w, b)
}
func (h *BuildingHandler) Create(w http.ResponseWriter, r *http.Request) {
	var b domain.Building
	if err := json.NewDecoder(r.Body).Decode(&b); err != nil { writeJSON(w, http.StatusBadRequest, errBadRequest); return }
	if err := h.repo.Create(r.Context(), &b); err != nil { serverErr(w, err); return }
	created(w, b)
}
func (h *BuildingHandler) Update(w http.ResponseWriter, r *http.Request) {
	var b domain.Building
	if err := json.NewDecoder(r.Body).Decode(&b); err != nil { writeJSON(w, http.StatusBadRequest, errBadRequest); return }
	b.ID = pathID(r)
	if err := h.repo.Update(r.Context(), &b); err != nil { serverErr(w, err); return }
	ok(w, b)
}
func (h *BuildingHandler) Delete(w http.ResponseWriter, r *http.Request) {
	if err := h.repo.Delete(r.Context(), pathID(r)); err != nil { serverErr(w, err); return }
	ok(w, M{"status": "deleted"})
}

// ===== DEVICE =====
// GatewayManager abstracts the gateway manager to allow unit-testing DispatchHardware
// without a real gateway connection. The concrete *gateway.Manager satisfies this interface.
type GatewayManager interface {
	GetGateway(id string) *gateway.Gateway
}

// HardwareDispatcher dispatches control actions to physical devices via gateways.
// The concrete *DeviceHandler satisfies this interface, enabling StartupHandler to
// send real hardware commands without depending on the concrete type.
type HardwareDispatcher interface {
	DispatchHardware(ctx context.Context, devID int, action string) dispatchResult
}

type DeviceHandler struct {
	repo  *postgres.DeviceRepo
	pool  postgres.DBTX
	gwMgr GatewayManager
}

func NewDeviceHandler(r *postgres.DeviceRepo, pool postgres.DBTX) *DeviceHandler {
	return &DeviceHandler{repo: r, pool: pool}
}

// SetGwMgr sets the gateway manager for hardware device control.
func (h *DeviceHandler) SetGwMgr(mgr GatewayManager) { h.gwMgr = mgr }

func (h *DeviceHandler) List(w http.ResponseWriter, r *http.Request) {
	list, err := h.repo.List(r.Context(), queryInt(r, "building_id"))
	if err != nil { serverErr(w, err); return }
	if list == nil { list = []domain.Device{} }
	ok(w, list)
}
func (h *DeviceHandler) Get(w http.ResponseWriter, r *http.Request) {
	d, err := h.repo.GetByID(r.Context(), pathID(r))
	if err != nil { serverErr(w, err); return }
	if d == nil { notFound(w, "设备不存在"); return }
	ok(w, d)
}
func (h *DeviceHandler) Create(w http.ResponseWriter, r *http.Request) {
	var d domain.Device
	if err := json.NewDecoder(r.Body).Decode(&d); err != nil { writeJSON(w, http.StatusBadRequest, errBadRequest); return }
	if d.DeviceType == "" { writeJSON(w, http.StatusBadRequest, M{"error": "设备类型不能为空"}); return }
	if err := h.repo.Create(r.Context(), &d); err != nil { serverErr(w, err); return }
	created(w, d)
}
func (h *DeviceHandler) Update(w http.ResponseWriter, r *http.Request) {
	id := pathID(r)
	if id <= 0 {
		writeJSON(w, http.StatusBadRequest, M{"error": "无效的设备 ID"})
		return
	}
	var d domain.Device
	if err := json.NewDecoder(r.Body).Decode(&d); err != nil { writeJSON(w, http.StatusBadRequest, errBadRequest); return }
	d.ID = id
	if err := h.repo.Update(r.Context(), &d); err != nil { serverErr(w, err); return }
	ok(w, d)
}
func (h *DeviceHandler) Delete(w http.ResponseWriter, r *http.Request) {
	if err := h.repo.Delete(r.Context(), pathID(r)); err != nil { serverErr(w, err); return }
	ok(w, M{"status": "deleted"})
}

// SensorData returns sensor config + latest telemetry for a temperature/humidity sensor device.
// GET /api/v1/devices/{id}/sensor-data
func (h *DeviceHandler) SensorData(w http.ResponseWriter, r *http.Request) {
	id := pathID(r)
	if id <= 0 {
		writeJSON(w, http.StatusBadRequest, M{"error": "无效的设备 ID"})
		return
	}

	// 获取设备信息（含网关编号）
	dev, err := h.repo.GetByID(r.Context(), id)
	if err != nil {
		serverErr(w, err)
		return
	}
	if dev == nil {
		notFound(w, "设备不存在")
		return
	}

	// 获取传感器配置
	var channelNo int
	var sensorNo string
	var intervalMinutes int
	err = h.pool.QueryRow(r.Context(),
		`SELECT COALESCE(channel_no,1), COALESCE(sensor_no,''), COALESCE(interval_minutes,5)
		 FROM sensor_config WHERE device_id=$1`, id).
		Scan(&channelNo, &sensorNo, &intervalMinutes)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		slog.Warn("SensorData sensor_config query failed", "dev", id, "err", err)
	}

	// 查询最新遥测数据（温度、湿度、电压、信号强度）
	type teleRow struct {
		Metric string
		Value  float64
		Unit   string
		Ts     time.Time
	}
	rows, err := h.pool.Query(r.Context(),
		`SELECT DISTINCT ON (metric) metric, value, COALESCE(unit,''), ts
		 FROM device_telemetry
		 WHERE device_id=$1 AND metric IN ('温度','湿度','电压','信号强度','temperature','humidity','voltage','signal')
		 ORDER BY metric, ts DESC`, id)
	if err != nil {
		slog.Warn("SensorData telemetry query failed", "dev", id, "err", err)
	}
	teleMap := map[string]teleRow{}
	var latestTs time.Time
	if rows != nil {
		defer rows.Close()
		for rows.Next() {
			var t teleRow
			if rows.Scan(&t.Metric, &t.Value, &t.Unit, &t.Ts) == nil {
				teleMap[t.Metric] = t
				if t.Ts.After(latestTs) {
					latestTs = t.Ts
				}
			}
		}
	}

	// 统一指标名
	getVal := func(keys ...string) (float64, string) {
		for _, k := range keys {
			if v, ok := teleMap[k]; ok {
				return v.Value, v.Unit
			}
		}
		return 0, ""
	}
	tempVal, _ := getVal("温度", "temperature")
	humVal, _ := getVal("湿度", "humidity")
	voltVal, _ := getVal("电压", "voltage")
	signalVal, _ := getVal("信号强度", "signal")

	gwImei := ""
	if dev.GatewayImei != nil {
		gwImei = *dev.GatewayImei
	}

	updateTime := ""
	if !latestTs.IsZero() {
		updateTime = latestTs.Format("2006-01-02 15:04:05")
	}

	ok(w, M{
		"device_id":        dev.ID,
		"device_name":      dev.Name,
		"device_type":      dev.DeviceType,
		"gateway_imei":     gwImei,
		"channel_no":       channelNo,
		"sensor_no":        sensorNo,
		"interval_minutes": intervalMinutes,
		"temperature":      tempVal,
		"humidity":         humVal,
		"voltage":          voltVal,
		"signal_strength":  signalVal,
		"update_time":      updateTime,
	})
}

// SaveSensorConfig saves or updates sensor_config for a device.
// PUT /api/v1/devices/{id}/sensor-config
func (h *DeviceHandler) SaveSensorConfig(w http.ResponseWriter, r *http.Request) {
	id := pathID(r)
	if id <= 0 {
		writeJSON(w, http.StatusBadRequest, M{"error": "无效的设备 ID"})
		return
	}
	var body struct {
		ChannelNo       int     `json:"channel_no"`
		SensorNo        string  `json:"sensor_no"`
		IntervalMinutes int     `json:"interval_minutes"`
		Temperature     *float64 `json:"temperature"`
		Humidity        *float64 `json:"humidity"`
		Voltage         *float64 `json:"voltage"`
		SignalStrength  *float64 `json:"signal_strength"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeJSON(w, http.StatusBadRequest, errBadRequest)
		return
	}
	if body.IntervalMinutes <= 0 {
		body.IntervalMinutes = 5
	}
	_, err := h.pool.Exec(r.Context(),
		`INSERT INTO sensor_config (device_id, channel_no, sensor_no, interval_minutes)
		 VALUES ($1,$2,$3,$4)
		 ON CONFLICT (device_id) DO UPDATE SET channel_no=$2, sensor_no=$3, interval_minutes=$4`,
		id, body.ChannelNo, body.SensorNo, body.IntervalMinutes)
	if err != nil {
		serverErr(w, err)
		return
	}
	// 将温度/湿度/电压/信号强度写入 device_telemetry
	now := time.Now()
	type teleVal struct {
		metric string
		value  float64
		unit   string
	}
	var teleVals []teleVal
	if body.Temperature != nil {
		teleVals = append(teleVals, teleVal{"温度", *body.Temperature, "℃"})
	}
	if body.Humidity != nil {
		teleVals = append(teleVals, teleVal{"湿度", *body.Humidity, "%"})
	}
	if body.Voltage != nil {
		teleVals = append(teleVals, teleVal{"电压", *body.Voltage, "V"})
	}
	if body.SignalStrength != nil {
		teleVals = append(teleVals, teleVal{"信号强度", *body.SignalStrength, "dbm"})
	}
	for _, tv := range teleVals {
		if _, err := h.pool.Exec(r.Context(),
			`INSERT INTO device_telemetry (ts, device_id, metric, value, unit) VALUES ($1,$2,$3,$4,$5)`,
			now, id, tv.metric, tv.value, tv.unit); err != nil {
			slog.Warn("sensor telemetry insert failed", "dev", id, "metric", tv.metric, "err", err)
		}
	}
	ok(w, M{"status": "saved"})
}

// dispatchResult captures the outcome of a hardware dispatch attempt.
type dispatchResult struct {
	Dispatched bool   `json:"dispatched"`
	Message    string `json:"dispatch_msg,omitempty"`
}

// controlActionCN maps an action code to its Chinese display value and target device status.
func controlActionCN(action string) (controlVal, deviceStatus string) {
	switch action {
	case "start", "startup":
		return "开机", "开机"
	case "stop", "shutdown":
		return "关机", "关机"
	case "restart":
		return "重启", "开机"
	case "pause":
		return "暂停", "暂停"
	case "resume":
		return "恢复", "开机"
	case "set_value":
		return "设值", ""
	case "mode_change":
		return "切换模式", ""
	default:
		return action, ""
	}
}

// DispatchHardware sends the control action to the physical device via the gateway.
// It looks up the register's write_addr/write_code by command_code, builds a Modbus
// write command, and transmits it through the device's gateway transport.
// Returns a dispatchResult indicating whether the command reached the hardware.
func (h *DeviceHandler) DispatchHardware(ctx context.Context, devID int, action string) dispatchResult {
	if h.gwMgr == nil {
		return dispatchResult{Dispatched: false, Message: "gateway manager not available"}
	}
	var gwImei, gwType string
	var nodeAddr, deviceNo int
	err := h.pool.QueryRow(ctx,
		`SELECT COALESCE(d.gateway_imei,''), COALESCE(d.gateway_type,'custom'),
		        d.node_address, d.device_no
		 FROM device d WHERE d.id = $1`, devID).
		Scan(&gwImei, &gwType, &nodeAddr, &deviceNo)
	if err != nil || gwImei == "" {
		return dispatchResult{Dispatched: false, Message: "device not configured for gateway"}
	}
	gw := h.gwMgr.GetGateway(gwImei)
	if gw == nil || gw.Transport == nil || !gw.Transport.IsConnected() {
		return dispatchResult{Dispatched: false, Message: "gateway offline or not connected"}
	}
	var writeAddr int
	var writeCode string
	err = h.pool.QueryRow(ctx,
		`SELECT r.write_addr, r.write_code FROM register r
		 JOIN device_properties dp ON dp.id=r.property_id
		 WHERE dp.device_id=$1 AND r.write_addr>0 AND r.command_code=$2
		 ORDER BY r.id LIMIT 1`, devID, action).Scan(&writeAddr, &writeCode)
	if err != nil {
		slog.Warn("DispatchHardware: no register config", "dev", devID, "action", action)
		return dispatchResult{Dispatched: false, Message: "no register write config for action: " + action}
	}
	if deviceNo > 255 {
		return dispatchResult{Dispatched: false, Message: fmt.Sprintf("deviceNo %d exceeds 255", deviceNo)}
	}
	if nodeAddr > 65535 || nodeAddr < 0 {
		return dispatchResult{Dispatched: false, Message: fmt.Sprintf("nodeAddr %d exceeds uint16 range", nodeAddr)}
	}
	dn := deviceNo & 0xFF
	writeVal := uint16(0xFF00) // ON by default
	switch action {
	case "stop", "pause":
		writeVal = 0x0000
	}
	cmd := modbus.BuildWriteCommand(byte(dn), modbus.CodeFromStr(writeCode), uint16(writeAddr), 1, writeVal)
	if gwType == "custom" {
		lora := []byte{byte(nodeAddr >> 8), byte(nodeAddr)}
		cmd = append(lora, cmd...)
	}
	if _, err := gw.Transport.SendAndReceive(cmd); err != nil {
		slog.Warn("gateway dispatch failed", "dev", devID, "gw", gwImei, "err", err)
		return dispatchResult{Dispatched: false, Message: "gateway dispatch failed"}
	}
	slog.Info("gateway dispatch ok", "dev", devID, "gw", gwImei, "action", action)
	return dispatchResult{Dispatched: true, Message: "ok"}
}

func (h *DeviceHandler) Control(w http.ResponseWriter, r *http.Request) {
	id := pathID(r)
	if id <= 0 {
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
	controlVal, deviceStatus := controlActionCN(body.Action)
	var devName, bldName, projName string
	err := h.pool.QueryRow(r.Context(), `
		SELECT d.name, b.name, COALESCE(p.name, '')
		FROM device d
		JOIN building b ON b.id = d.building_id
		JOIN project p ON p.id = b.project_id
		WHERE d.id = $1`, id).Scan(&devName, &bldName, &projName)
	if err != nil {
		notFound(w, "设备不存在")
		return
	}
	_, err = h.pool.Exec(r.Context(), `
		INSERT INTO control_record (project_name, building_name, device_name, device_id, prop_name, control_value, username, user_remark)
		VALUES ($1, $2, $3, $4, '设备控制', $5, $6, $7)`,
		projName, bldName, devName, id, controlVal, username, body.Action)
	if err != nil {
		serverErr(w, err)
		return
	}

	// Dispatch to physical device via gateway.
	dr := h.DispatchHardware(r.Context(), id, body.Action)

	if deviceStatus != "" {
		if _, err := h.pool.Exec(r.Context(), `UPDATE device SET device_status=$1 WHERE id=$2`, deviceStatus, id); err != nil {
			slog.Warn("Control update device_status failed", "dev", id, "err", err)
		}
	}
	ok(w, map[string]any{"status": "ok", "action": body.Action, "dispatched": dr.Dispatched, "dispatch_msg": dr.Message})
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
	p, err := h.repo.GetByID(r.Context(), pathID(r))
	if err != nil { serverErr(w, err); return }
	if p == nil { notFound(w, "属性不存在"); return }
	ok(w, p)
}
func (h *PropertyHandler) Create(w http.ResponseWriter, r *http.Request) {
	var p domain.DeviceProperty
	if err := json.NewDecoder(r.Body).Decode(&p); err != nil { writeJSON(w, http.StatusBadRequest, errBadRequest); return }
	if err := h.repo.Create(r.Context(), &p); err != nil { serverErr(w, err); return }
	created(w, p)
}
func (h *PropertyHandler) Update(w http.ResponseWriter, r *http.Request) {
	var p domain.DeviceProperty
	if err := json.NewDecoder(r.Body).Decode(&p); err != nil { writeJSON(w, http.StatusBadRequest, errBadRequest); return }
	p.ID = pathID(r)
	if err := h.repo.Update(r.Context(), &p); err != nil { serverErr(w, err); return }
	ok(w, p)
}
func (h *PropertyHandler) Delete(w http.ResponseWriter, r *http.Request) {
	if err := h.repo.Delete(r.Context(), pathID(r)); err != nil { serverErr(w, err); return }
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
	reg, err := h.repo.GetByID(r.Context(), pathID(r))
	if err != nil { serverErr(w, err); return }
	if reg == nil { notFound(w, "寄存器不存在"); return }
	ok(w, reg)
}
func (h *RegisterHandler) Create(w http.ResponseWriter, r *http.Request) {
	var reg domain.Register
	if err := json.NewDecoder(r.Body).Decode(&reg); err != nil { writeJSON(w, http.StatusBadRequest, errBadRequest); return }
	if err := h.repo.Create(r.Context(), &reg); err != nil { serverErr(w, err); return }
	created(w, reg)
}
func (h *RegisterHandler) Update(w http.ResponseWriter, r *http.Request) {
	var reg domain.Register
	if err := json.NewDecoder(r.Body).Decode(&reg); err != nil { writeJSON(w, http.StatusBadRequest, errBadRequest); return }
	reg.ID = pathID(r)
	if err := h.repo.Update(r.Context(), &reg); err != nil { serverErr(w, err); return }
	ok(w, reg)
}
func (h *RegisterHandler) Delete(w http.ResponseWriter, r *http.Request) {
	if err := h.repo.Delete(r.Context(), pathID(r)); err != nil { serverErr(w, err); return }
	ok(w, M{"status": "deleted"})
}
