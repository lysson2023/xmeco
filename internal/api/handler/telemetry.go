package handler

import (
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"github.com/jackc/pgx/v5"

	"xmeco/internal/repository/postgres"
)

type TelemetryHandler struct{ pool postgres.DBTX }

func NewTelemetryHandler(pool postgres.DBTX) *TelemetryHandler { return &TelemetryHandler{pool} }

func (h *TelemetryHandler) Realtime(w http.ResponseWriter, r *http.Request) {
	deviceID := queryInt(r, "device_id")
	var rows pgx.Rows
	var err error
	if deviceID > 0 {
		rows, err = h.pool.Query(r.Context(),
			`SELECT DISTINCT ON (device_id, metric) ts, device_id, metric, value, unit
			 FROM device_telemetry WHERE device_id=$1 ORDER BY device_id, metric, ts DESC`, deviceID)
	} else {
		rows, err = h.pool.Query(r.Context(),
			`SELECT DISTINCT ON (device_id, metric) ts, device_id, metric, value, unit
			 FROM device_telemetry ORDER BY device_id, metric, ts DESC`)
	}
	if err != nil { serverErr(w, err); return }
	defer rows.Close()
	type pt struct {
		Ts       time.Time `json:"ts"`
		DeviceID int       `json:"device_id"`
		Metric   string    `json:"metric"`
		Value    float64   `json:"value"`
		Unit     string    `json:"unit"`
	}
	var list []pt
	for rows.Next() {
		var p pt
		if err := rows.Scan(&p.Ts, &p.DeviceID, &p.Metric, &p.Value, &p.Unit); err != nil {
			slog.Warn("Realtime scan failed", "err", err)
			continue
		}
		list = append(list, p)
	}
	if list == nil { list = []pt{} }
	if err := rows.Err(); err != nil { serverErr(w, err); return }
	ok(w, list)
}

func (h *TelemetryHandler) History(w http.ResponseWriter, r *http.Request) {
	deviceID := queryInt(r, "device_id")
	metric := r.URL.Query().Get("metric")
	hours, _ := strconv.Atoi(r.URL.Query().Get("hours"))
	if hours <= 0 { hours = 24 }
	since := time.Now().Add(-time.Duration(hours) * time.Hour)

	rows, err := h.pool.Query(r.Context(),
		`SELECT ts, value FROM device_telemetry WHERE device_id=$1 AND metric=$2 AND ts>$3 ORDER BY ts`,
		deviceID, metric, since)
	if err != nil { serverErr(w, err); return }
	defer rows.Close()
	type pt struct {
		Ts    time.Time `json:"ts"`
		Value float64   `json:"value"`
	}
	var list []pt
	for rows.Next() {
		var p pt
		if err := rows.Scan(&p.Ts, &p.Value); err != nil {
			slog.Warn("History scan failed", "err", err)
			continue
		}
		list = append(list, p)
	}
	if list == nil { list = []pt{} }
	if err := rows.Err(); err != nil { serverErr(w, err); return }
	ok(w, list)
}

func (h *TelemetryHandler) Stats(w http.ResponseWriter, r *http.Request) {
	deviceID := queryInt(r, "device_id")
	if deviceID > 0 {
		// Single device stats
		rows, err := h.pool.Query(r.Context(),
			`SELECT metric, COUNT(*)::int, AVG(value)::numeric(10,2),
			        MAX(value)::numeric(10,2), MIN(value)::numeric(10,2)
			 FROM device_telemetry WHERE device_id=$1 GROUP BY metric ORDER BY metric`, deviceID)
		if err != nil { serverErr(w, err); return }
		defer rows.Close()
		type stat struct{ Metric string; Count int; Avg, Max, Min float64 }
		var list []stat
		for rows.Next() {
			var s stat
			if err := rows.Scan(&s.Metric, &s.Count, &s.Avg, &s.Max, &s.Min); err != nil {
				slog.Warn("Stats scan failed", "err", err)
				continue
			}
			list = append(list, s)
		}
		if list == nil { list = []stat{} }
		ok(w, list)
		return
	}
	// System-wide online/offline stats
	var online, offline int
	if err := h.pool.QueryRow(r.Context(), `SELECT count(*) FROM device WHERE online_status='在线'`).Scan(&online); err != nil {
		slog.Warn("stats online query failed", "err", err)
		online = 0
	}
	if err := h.pool.QueryRow(r.Context(), `SELECT count(*) FROM device WHERE online_status='离线'`).Scan(&offline); err != nil {
		slog.Warn("stats offline query failed", "err", err)
		offline = 0
	}
	ok(w, map[string]any{"online": online, "offline": offline, "total": online + offline})
}
