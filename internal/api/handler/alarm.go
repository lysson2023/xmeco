package handler

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"

	"github.com/jackc/pgx/v5"

	"xmeco/internal/api/middleware"
	"xmeco/internal/repository/postgres"
)

type AlarmHandler struct{ pool postgres.DBTX }
func NewAlarmHandler(pool postgres.DBTX) *AlarmHandler { return &AlarmHandler{pool} }

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
		NotifyUsers []int    `json:"notify_users"`
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
	NotifyUsers []int    `json:"notify_users"`
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
	enabled := true
	if rr.Enabled != nil {
		enabled = *rr.Enabled
	}
	err := h.pool.QueryRow(r.Context(),
		"INSERT INTO alarm_rule (name,device_id,property_id,device_type,metric,condition,threshold,level,target_value,min_value,max_value,notify_users,enabled) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13) RETURNING id",
		rr.Name, rr.DeviceID, rr.PropertyID, rr.DeviceType, rr.Metric, rr.Condition, rr.Threshold, rr.Level, rr.TargetValue, rr.MinValue, rr.MaxValue, rr.NotifyUsers, enabled).Scan(&id)
	if err != nil { serverErr(w, err); return }
	ok(w, map[string]any{"id": id})
}

func (h *AlarmHandler) UpdateRule(w http.ResponseWriter, r *http.Request) {
	id := pathID(r)
	var rr alarmRuleReq
	if err := json.NewDecoder(r.Body).Decode(&rr); err != nil {
		writeJSON(w, http.StatusBadRequest, M{"error": "请求格式错误"})
		return
	}
	enabled := true
	if rr.Enabled != nil {
		enabled = *rr.Enabled
	}
	_, err := h.pool.Exec(r.Context(),
		"UPDATE alarm_rule SET name=$1,device_id=$2,property_id=$3,device_type=$4,metric=$5,condition=$6,threshold=$7,level=$8,target_value=$9,min_value=$10,max_value=$11,notify_users=$12,enabled=$13 WHERE id=$14",
		rr.Name, rr.DeviceID, rr.PropertyID, rr.DeviceType, rr.Metric, rr.Condition, rr.Threshold, rr.Level, rr.TargetValue, rr.MinValue, rr.MaxValue, rr.NotifyUsers, enabled, id)
	if err != nil { serverErr(w, err); return }
	ok(w, M{"status": "updated"})
}

func (h *AlarmHandler) DeleteRule(w http.ResponseWriter, r *http.Request) {
	if _, err := h.pool.Exec(r.Context(), "DELETE FROM alarm_rule WHERE id=$1", pathID(r)); err != nil {
		serverErr(w, err)
		return
	}
	ok(w, M{"status":"deleted"})
}

func (h *AlarmHandler) ListLogs(w http.ResponseWriter, r *http.Request) {
	deviceID := queryInt(r, "device_id")
	buildingID := queryInt(r, "building_id")
	projectID := queryInt(r, "project_id")
	dateFrom := r.URL.Query().Get("date_from")
	dateTo := r.URL.Query().Get("date_to")
	today := r.URL.Query().Get("today") // "1" = today only

	var rows pgx.Rows
	var err error

	// Build dynamic query for flexible filtering
	baseQ := `SELECT al.id, al.device_id, al.device_name, al.alarm_type, al.level,
		al.message, al.value, al.threshold,
		COALESCE(al.created_at::text,''), COALESCE(al.ack_at::text,'')
		FROM alarm_log al`
	var conditions []string
	var args []any
	argIdx := 1

	if buildingID > 0 || projectID > 0 {
		baseQ += ` JOIN device d ON d.id = al.device_id`
		if projectID > 0 {
			baseQ += ` JOIN building b ON b.id = d.building_id`
		}
	}

	if deviceID > 0 {
		conditions = append(conditions, fmt.Sprintf("al.device_id=$%d", argIdx))
		args = append(args, deviceID)
		argIdx++
	}
	if buildingID > 0 {
		conditions = append(conditions, fmt.Sprintf("d.building_id=$%d", argIdx))
		args = append(args, buildingID)
		argIdx++
	}
	if projectID > 0 {
		conditions = append(conditions, fmt.Sprintf("b.project_id=$%d", argIdx))
		args = append(args, projectID)
		argIdx++
	}
	if today == "1" {
		conditions = append(conditions, "al.created_at::date = CURRENT_DATE")
	}
	if dateFrom != "" {
		conditions = append(conditions, fmt.Sprintf("al.created_at >= $%d::timestamp", argIdx))
		args = append(args, dateFrom)
		argIdx++
	}
	if dateTo != "" {
		conditions = append(conditions, fmt.Sprintf("al.created_at <= $%d::timestamp", argIdx))
		args = append(args, dateTo)
		argIdx++
	}

	if len(conditions) > 0 {
		baseQ += " WHERE " + conditions[0]
		for i := 1; i < len(conditions); i++ {
			baseQ += " AND " + conditions[i]
		}
	}
	baseQ += " ORDER BY al.created_at DESC LIMIT 200"

	rows, err = h.pool.Query(r.Context(), baseQ, args...)
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
		if l.CreatedAt != nil && *l.CreatedAt == "" { l.CreatedAt = nil }
		if l.AckAt != nil && *l.AckAt == "" { l.AckAt = nil }
		list = append(list, l)
	}
	if list == nil { list = []alarmLog{} }
	if err := rows.Err(); err != nil { serverErr(w, err); return }
	ok(w, list)
}

func (h *AlarmHandler) AckLog(w http.ResponseWriter, r *http.Request) {
	id := pathID(r)
	claims := middleware.GetClaims(r)
	if claims == nil {
		writeJSON(w, http.StatusUnauthorized, M{"error": "未认证"})
		return
	}
	if _, err := h.pool.Exec(r.Context(), "UPDATE alarm_log SET ack_by=$1, ack_at=NOW() WHERE id=$2", claims.Username, id); err != nil {
		serverErr(w, err)
		return
	}
	ok(w, M{"status":"acked"})
}