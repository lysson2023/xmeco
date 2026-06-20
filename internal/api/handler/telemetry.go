package handler

import (
	"net/http"
	"strconv"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

type TelemetryHandler struct{ pool *pgxpool.Pool }

func NewTelemetryHandler(pool *pgxpool.Pool) *TelemetryHandler { return &TelemetryHandler{pool} }

func (h *TelemetryHandler) Realtime(w http.ResponseWriter, r *http.Request) {
	deviceID := queryInt(r, "device_id")
	rows, err := h.pool.Query(r.Context(),
		`SELECT DISTINCT ON (device_id, metric) ts, device_id, metric, value, unit
		 FROM device_telemetry WHERE device_id=$1 ORDER BY device_id, metric, ts DESC`, deviceID)
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
		rows.Scan(&p.Ts, &p.DeviceID, &p.Metric, &p.Value, &p.Unit)
		list = append(list, p)
	}
	if list == nil { list = []pt{} }
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
		rows.Scan(&p.Ts, &p.Value)
		list = append(list, p)
	}
	if list == nil { list = []pt{} }
	ok(w, list)
}

func (h *TelemetryHandler) Stats(w http.ResponseWriter, r *http.Request) {
	deviceID := queryInt(r, "device_id")
	var online, offline int
	h.pool.QueryRow(r.Context(), `SELECT count(*) FROM device WHERE building_id IN (SELECT building_id FROM device WHERE id=$1)`, deviceID).Scan(&online)
	_ = offline
	ok(w, map[string]interface{}{"online": online, "total": online})
}
