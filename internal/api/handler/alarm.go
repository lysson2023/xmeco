package handler

import (
	"encoding/json"
	"net/http"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type AlarmHandler struct{ pool *pgxpool.Pool }
func NewAlarmHandler(pool *pgxpool.Pool) *AlarmHandler { return &AlarmHandler{pool} }

func (h *AlarmHandler) ListRules(w http.ResponseWriter, r *http.Request) {
	deviceID := queryInt(r, "device_id")
	var rows pgx.Rows
	var err error
	if deviceID > 0 {
		rows, err = h.pool.Query(r.Context(), "SELECT id,name,device_id,property_id,device_type AS DType,metric AS Metric,condition AS Cond,threshold AS Threshold,level AS Level,target_value,min_value,max_value,notify_users,enabled FROM alarm_rule WHERE device_id=$1 ORDER BY id", deviceID)
	} else {
		rows, err = h.pool.Query(r.Context(), "SELECT id,name,device_id,property_id,device_type AS DType,metric AS Metric,condition AS Cond,threshold AS Threshold,level AS Level,target_value,min_value,max_value,notify_users,enabled FROM alarm_rule ORDER BY id")
	}
	if err != nil { serverErr(w, err); return }
	defer rows.Close()
	type rule struct {
		ID int `json:"ID"`
		Name string `json:"Name"`
		DevID *int `json:"DevID"`
		PropID *int `json:"PropID"`
		DType *string `json:"DType"`
		Metric *string `json:"Metric"`
		Cond *string `json:"Cond"`
		Threshold *float64 `json:"Threshold"`
		Level *string `json:"Level"`
		TargetVal *string `json:"target_value"`
		MinVal *string `json:"min_value"`
		MaxVal *string `json:"max_value"`
		NotifyUsers []byte `json:"notify_users"`
		Enabled bool `json:"Enabled"`
	}
	var list []rule
	for rows.Next() {
		var r rule
		rows.Scan(&r.ID, &r.Name, &r.DevID, &r.PropID, &r.DType, &r.Metric, &r.Cond, &r.Threshold, &r.Level, &r.TargetVal, &r.MinVal, &r.MaxVal, &r.NotifyUsers, &r.Enabled)
		list = append(list, r)
	}
	if list == nil { list = []rule{} }
	ok(w, list)
}

func (h *AlarmHandler) CreateRule(w http.ResponseWriter, r *http.Request) {
	var rr map[string]interface{}
	json.NewDecoder(r.Body).Decode(&rr)
	var id int
	err := h.pool.QueryRow(r.Context(),
		"INSERT INTO alarm_rule (name,device_id,property_id,device_type,metric,condition,threshold,level,target_value,min_value,max_value,notify_users,enabled) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13) RETURNING id",
		rr["name"],rr["device_id"],rr["property_id"],rr["device_type"],rr["metric"],rr["condition"],rr["threshold"],rr["level"],rr["target_value"],rr["min_value"],rr["max_value"],rr["notify_users"],rr["enabled"]).Scan(&id)
	if err != nil { serverErr(w, err); return }
	ok(w, map[string]interface{}{"id":id})
}

func (h *AlarmHandler) UpdateRule(w http.ResponseWriter, r *http.Request) {
	id := pathLast(r.URL.Path)
	var rr map[string]interface{}
	json.NewDecoder(r.Body).Decode(&rr)
	_, err := h.pool.Exec(r.Context(),
		"UPDATE alarm_rule SET name=$1,device_id=$2,property_id=$3,metric=$4,condition=$5,threshold=$6,level=$7,target_value=$8,min_value=$9,max_value=$10,notify_users=$11,enabled=$12 WHERE id=$13",
		rr["name"],rr["device_id"],rr["property_id"],rr["metric"],rr["condition"],rr["threshold"],rr["level"],rr["target_value"],rr["min_value"],rr["max_value"],rr["notify_users"],rr["enabled"],id)
	if err != nil { serverErr(w, err); return }
	ok(w, M{"status":"updated"})
}

func (h *AlarmHandler) DeleteRule(w http.ResponseWriter, r *http.Request) {
	h.pool.Exec(r.Context(), "DELETE FROM alarm_rule WHERE id=$1", pathLast(r.URL.Path))
	ok(w, M{"status":"deleted"})
}

func (h *AlarmHandler) ListLogs(w http.ResponseWriter, r *http.Request) {
	deviceID := queryInt(r, "device_id")
	var rows pgx.Rows
	var err error
	if deviceID > 0 {
		rows, err = h.pool.Query(r.Context(), "SELECT id,device_id,device_name,alarm_type,level,message,value,threshold,created_at,ack_at FROM alarm_log WHERE device_id=$1 ORDER BY created_at DESC LIMIT 100", deviceID)
	} else {
		rows, err = h.pool.Query(r.Context(), "SELECT id,device_id,device_name,alarm_type,level,message,value,threshold,created_at,ack_at FROM alarm_log ORDER BY created_at DESC LIMIT 100")
	}
	if err != nil { serverErr(w, err); return }
	defer rows.Close()
	type log struct{ ID,DevID int; DevName,AType,Level,Msg,Val,Thr string; Ts,Ack *string }
	var list []log
	for rows.Next() {
		var l log; rows.Scan(&l.ID,&l.DevID,&l.DevName,&l.AType,&l.Level,&l.Msg,&l.Val,&l.Thr,&l.Ts,&l.Ack)
		list = append(list, l)
	}
	if list == nil { list = []log{} }
	ok(w, list)
}

func (h *AlarmHandler) AckLog(w http.ResponseWriter, r *http.Request) {
	h.pool.Exec(r.Context(), "UPDATE alarm_log SET ack_by='admin', ack_at=NOW() WHERE id=$1", pathLast(r.URL.Path))
	ok(w, M{"status":"acked"})
}