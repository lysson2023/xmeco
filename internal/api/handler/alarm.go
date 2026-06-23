package handler

import (
	"encoding/json"
	"log/slog"
	"net/http"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"xmeco/internal/api/middleware"
)

type AlarmHandler struct{ pool *pgxpool.Pool }
func NewAlarmHandler(pool *pgxpool.Pool) *AlarmHandler { return &AlarmHandler{pool} }

func (h *AlarmHandler) ListRules(w http.ResponseWriter, r *http.Request) {
	deviceID := queryInt(r, "device_id")
	var rows pgx.Rows
	var err error
	if deviceID > 0 {
		rows, err = h.pool.Query(r.Context(), "SELECT id,name,device_id,property_id,device_type,metric,condition,threshold,level,target_value,min_value,max_value,notify_users,enabled FROM alarm_rule WHERE device_id=$1 ORDER BY id", deviceID)
	} else {
		rows, err = h.pool.Query(r.Context(), "SELECT id,name,device_id,property_id,device_type,metric,condition,threshold,level,target_value,min_value,max_value,notify_users,enabled FROM alarm_rule ORDER BY id")
	}
	if err != nil { serverErr(w, err); return }
	defer rows.Close()
	type rule struct {
		ID          int      `json:"id"`
		Name        string   `json:"name"`
		DeviceID    *int     `json:"device_id"`
		PropertyID  *int     `json:"property_id"`
		DeviceType  *string  `json:"device_type"`
		Metric      *string  `json:"metric"`
		Condition   *string  `json:"condition"`
		Threshold   *float64 `json:"threshold"`
		Level       *string  `json:"level"`
		TargetValue *string  `json:"target_value"`
		MinValue    *string  `json:"min_value"`
		MaxValue    *string  `json:"max_value"`
		NotifyUsers []byte   `json:"notify_users"`
		Enabled     bool     `json:"enabled"`
	}
	var list []rule
	for rows.Next() {
		var rr rule
		if err := rows.Scan(&rr.ID, &rr.Name, &rr.DeviceID, &rr.PropertyID, &rr.DeviceType, &rr.Metric, &rr.Condition, &rr.Threshold, &rr.Level, &rr.TargetValue, &rr.MinValue, &rr.MaxValue, &rr.NotifyUsers, &rr.Enabled); err != nil {
			slog.Warn("ListRules scan failed", "err", err)
			continue
		}
		list = append(list, rr)
	}
	if list == nil { list = []rule{} }
	if err := rows.Err(); err != nil { serverErr(w, err); return }
	ok(w, list)
}

type alarmRuleReq struct {
	Name        string   `json:"name"`
	DeviceID    *int     `json:"device_id"`
	PropertyID  *int     `json:"property_id"`
	DeviceType  *string  `json:"device_type"`
	Metric      *string  `json:"metric"`
	Condition   *string  `json:"condition"`
	Threshold   *float64 `json:"threshold"`
	Level       *string  `json:"level"`
	TargetValue *string  `json:"target_value"`
	MinValue    *string  `json:"min_value"`
	MaxValue    *string  `json:"max_value"`
	NotifyUsers []byte   `json:"notify_users"`
	Enabled     *bool    `json:"enabled"`
}

func (h *AlarmHandler) CreateRule(w http.ResponseWriter, r *http.Request) {
	var rr alarmRuleReq
	if err := json.NewDecoder(r.Body).Decode(&rr); err != nil {
		writeJSON(w, http.StatusBadRequest, M{"error": "请求格式错误"})
		return
	}
	if rr.Name == "" {
		writeJSON(w, http.StatusBadRequest, M{"error": "规则名称不能为空"})
		return
	}
	var id int
	err := h.pool.QueryRow(r.Context(),
		"INSERT INTO alarm_rule (name,device_id,property_id,device_type,metric,condition,threshold,level,target_value,min_value,max_value,notify_users,enabled) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13) RETURNING id",
		rr.Name, rr.DeviceID, rr.PropertyID, rr.DeviceType, rr.Metric, rr.Condition, rr.Threshold, rr.Level, rr.TargetValue, rr.MinValue, rr.MaxValue, rr.NotifyUsers, rr.Enabled).Scan(&id)
	if err != nil { serverErr(w, err); return }
	ok(w, map[string]any{"id": id})
}

func (h *AlarmHandler) UpdateRule(w http.ResponseWriter, r *http.Request) {
	id := pathLast(r.URL.Path)
	var rr alarmRuleReq
	if err := json.NewDecoder(r.Body).Decode(&rr); err != nil {
		writeJSON(w, http.StatusBadRequest, M{"error": "请求格式错误"})
		return
	}
	_, err := h.pool.Exec(r.Context(),
		"UPDATE alarm_rule SET name=$1,device_id=$2,property_id=$3,device_type=$4,metric=$5,condition=$6,threshold=$7,level=$8,target_value=$9,min_value=$10,max_value=$11,notify_users=$12,enabled=$13 WHERE id=$14",
		rr.Name, rr.DeviceID, rr.PropertyID, rr.DeviceType, rr.Metric, rr.Condition, rr.Threshold, rr.Level, rr.TargetValue, rr.MinValue, rr.MaxValue, rr.NotifyUsers, rr.Enabled, id)
	if err != nil { serverErr(w, err); return }
	ok(w, M{"status": "updated"})
}

func (h *AlarmHandler) DeleteRule(w http.ResponseWriter, r *http.Request) {
	if _, err := h.pool.Exec(r.Context(), "DELETE FROM alarm_rule WHERE id=$1", pathLast(r.URL.Path)); err != nil {
		serverErr(w, err)
		return
	}
	ok(w, M{"status":"deleted"})
}

func (h *AlarmHandler) ListLogs(w http.ResponseWriter, r *http.Request) {
	deviceID := queryInt(r, "device_id")
	var rows pgx.Rows
	var err error
	if deviceID > 0 {
		rows, err = h.pool.Query(r.Context(), "SELECT id,device_id,device_name,alarm_type,level,message,value,threshold,COALESCE(created_at::text,''),COALESCE(ack_at::text,'') FROM alarm_log WHERE device_id=$1 ORDER BY created_at DESC LIMIT 100", deviceID)
	} else {
		rows, err = h.pool.Query(r.Context(), "SELECT id,device_id,device_name,alarm_type,level,message,value,threshold,COALESCE(created_at::text,''),COALESCE(ack_at::text,'') FROM alarm_log ORDER BY created_at DESC LIMIT 100")
	}
	if err != nil { serverErr(w, err); return }
	defer rows.Close()
	type alarmLog struct {
		ID         int     `json:"id"`
		DeviceID   int     `json:"device_id"`
		DeviceName string  `json:"device_name"`
		AlarmType  string  `json:"alarm_type"`
		Level      string  `json:"level"`
		Message    string  `json:"message"`
		Value      string  `json:"value"`
		Threshold  string  `json:"threshold"`
		CreatedAt  *string `json:"created_at"`
		AckAt      *string `json:"ack_at"`
	}
	var list []alarmLog
	for rows.Next() {
		var l alarmLog
		if err := rows.Scan(&l.ID, &l.DeviceID, &l.DeviceName, &l.AlarmType, &l.Level, &l.Message, &l.Value, &l.Threshold, &l.CreatedAt, &l.AckAt); err != nil {
			slog.Warn("ListLogs scan failed", "err", err)
			continue
		}
		list = append(list, l)
	}
	if list == nil { list = []alarmLog{} }
	if err := rows.Err(); err != nil { serverErr(w, err); return }
	ok(w, list)
}

func (h *AlarmHandler) AckLog(w http.ResponseWriter, r *http.Request) {
	id := pathID(r.URL.Path)
	username := "admin"
	if claims := middleware.GetClaims(r); claims != nil {
		username = claims.Username
	}
	if _, err := h.pool.Exec(r.Context(), "UPDATE alarm_log SET ack_by=$1, ack_at=NOW() WHERE id=$2", username, id); err != nil {
		serverErr(w, err)
		return
	}
	ok(w, M{"status":"acked"})
}