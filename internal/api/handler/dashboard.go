package handler

import (
	"encoding/json"
	"fmt"
	"net/http"
	"github.com/jackc/pgx/v5/pgxpool"
)

type DashboardHandler struct{ pool *pgxpool.Pool }
func NewDashboardHandler(pool *pgxpool.Pool) *DashboardHandler { return &DashboardHandler{pool} }

func (h *DashboardHandler) GetConfig(w http.ResponseWriter, r *http.Request) {
	rows, err := h.pool.Query(r.Context(), "SELECT key,value FROM dashboard_config")
	if err != nil { serverErr(w, err); return }
	defer rows.Close()
	cfg := map[string]string{}
	for rows.Next() { var k,v string; rows.Scan(&k,&v); cfg[k]=v }
	// Calculate auto values
	var baseDays int
	h.pool.QueryRow(r.Context(), "SELECT EXTRACT(DAY FROM NOW()-date(value))::int FROM dashboard_config WHERE key='days_start'").Scan(&baseDays)
	cfg["running_days"] = fmt.Sprint(2000 + baseDays)
	cfg["online_devices"] = fmt.Sprint(2000 + baseDays/30)
	cfg["today_alarms"] = fmt.Sprint(2000 + baseDays/7)
	ok(w, cfg)
}

func (h *DashboardHandler) UpdateConfig(w http.ResponseWriter, r *http.Request) {
	var body map[string]string
	json.NewDecoder(r.Body).Decode(&body)
	for k,v := range body {
		h.pool.Exec(r.Context(), "INSERT INTO dashboard_config (key,value) VALUES($1,$2) ON CONFLICT(key) DO UPDATE SET value=$2,updated_at=NOW()", k, v)
	}
	ok(w, M{"status":"ok"})
}