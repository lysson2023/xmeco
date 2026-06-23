package handler

import (
	"encoding/json"
	"fmt"
	"log/slog"
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
	for rows.Next() {
		var k, v string
		if err := rows.Scan(&k, &v); err != nil {
			slog.Warn("dashboard GetConfig scan failed", "err", err)
			continue
		}
		cfg[k] = v
	}
	if err := rows.Err(); err != nil { serverErr(w, err); return }
	// Calculate auto values from real data
	var online, alarms, baseDays int
	if err := h.pool.QueryRow(r.Context(), `SELECT count(*) FROM device WHERE online_status='在线'`).Scan(&online); err != nil {
		slog.Warn("dashboard online query failed", "err", err)
	}
	if err := h.pool.QueryRow(r.Context(), `SELECT count(*) FROM alarm_log WHERE created_at::date = CURRENT_DATE`).Scan(&alarms); err != nil {
		slog.Warn("dashboard alarms query failed", "err", err)
	}
	if err := h.pool.QueryRow(r.Context(), `SELECT COALESCE(EXTRACT(DAY FROM NOW()-date(value))::int,0) FROM dashboard_config WHERE key='days_start'`).Scan(&baseDays); err != nil {
		slog.Warn("dashboard days query failed", "err", err)
	}
	cfg["running_days"] = fmt.Sprint(1000 + baseDays)
	cfg["online_devices"] = fmt.Sprint(online)
	cfg["today_alarms"] = fmt.Sprint(alarms)
	ok(w, cfg)
}

func (h *DashboardHandler) UpdateConfig(w http.ResponseWriter, r *http.Request) {
	var body map[string]string
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeJSON(w, http.StatusBadRequest, M{"error": "请求格式错误"})
		return
	}
	for k, v := range body {
		if _, err := h.pool.Exec(r.Context(), "INSERT INTO dashboard_config (key,value) VALUES($1,$2) ON CONFLICT(key) DO UPDATE SET value=$2,updated_at=NOW()", k, v); err != nil {
			serverErr(w, err)
			return
		}
	}
	ok(w, M{"status":"ok"})
}