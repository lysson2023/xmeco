package handler

import (
	"encoding/csv"
	"log/slog"
	"net/http"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

type LogHandler struct{ pool *pgxpool.Pool }

func NewLogHandler(pool *pgxpool.Pool) *LogHandler { return &LogHandler{pool} }

func (h *LogHandler) Telemetry(w http.ResponseWriter, r *http.Request) {
	deviceID := queryInt(r, "device_id")
	metric := r.URL.Query().Get("metric")     // empty = all
	interval := r.URL.Query().Get("interval") // hour, day, month, year
	start := r.URL.Query().Get("start")
	end := r.URL.Query().Get("end")
	if end == "" {
		end = time.Now().Format("2006-01-02 15:04:05")
	} else if len(end) == 10 {
		end += " 23:59:59"
	}
	if start == "" {
		start = time.Now().AddDate(0, -1, 0).Format("2006-01-02")
	}
	export := r.URL.Query().Get("export") == "csv"

	// Whitelist the interval parameter to prevent unexpected SQL function injection.
	validIntervals := map[string]bool{"minute": true, "hour": true, "day": true, "week": true, "month": true, "year": true, "raw": true}
	if interval != "" && !validIntervals[interval] {
		writeJSON(w, http.StatusBadRequest, M{"error": "无效的时间间隔，可选值：minute/hour/day/week/month/year/raw"})
		return
	}

	query := ""
	args := []any{}
	argIdx := 1

	if interval != "" && interval != "raw" {
		trunc := truncMap[interval]
		if trunc == "" {
			trunc = "day"
		}
		query = "SELECT date_trunc($1, ts) as ts, metric, AVG(value)::numeric(10,2) as avg_val, MAX(value)::numeric(10,2) as max_val, MIN(value)::numeric(10,2) as min_val, COUNT(*)::int as cnt FROM device_telemetry WHERE ts>=$2 AND ts<=$3"
		args = append(args, trunc, start, end)
		argIdx = 4
	} else {
		query = "SELECT ts, metric, value, unit FROM device_telemetry WHERE ts>=$1 AND ts<=$2"
		args = append(args, start, end)
		argIdx = 3
	}
	if deviceID > 0 {
		query += " AND device_id=$" + itos(argIdx)
		args = append(args, deviceID)
		argIdx++
	}
	if metric != "" {
		query += " AND metric=$" + itos(argIdx)
		args = append(args, metric)
		argIdx++
	}
	if interval != "" && interval != "raw" {
		query += " GROUP BY date_trunc($1, ts), metric ORDER BY date_trunc($1, ts) DESC, metric"
	} else {
		query += " ORDER BY ts DESC LIMIT 5000"
	}

	rows, err := h.pool.Query(r.Context(), query, args...)
	if err != nil {
		serverErr(w, err)
		return
	}
	defer rows.Close()

	if export {
		w.Header().Set("Content-Type", "text/csv; charset=utf-8")
		w.Header().Set("Content-Disposition", "attachment; filename=telemetry_"+time.Now().Format("20060102")+".csv")
		wr := csv.NewWriter(w)
		if interval != "" && interval != "raw" {
			if err := wr.Write([]string{"时间", "指标", "平均值", "最大值", "最小值", "记录数"}); err != nil {
				slog.Warn("CSV write header failed", "err", err)
				return
			}
			for rows.Next() {
				var ts time.Time
				var m string
				var av, mx, mn float64
				var c int
				if err := rows.Scan(&ts, &m, &av, &mx, &mn, &c); err != nil {
					slog.Warn("Telemetry export scan failed", "err", err)
					continue
				}
				if err := wr.Write([]string{ts.Format("2006-01-02 15:04:05"), m, ftoa(av), ftoa(mx), ftoa(mn), itos(c)}); err != nil {
					slog.Warn("CSV write row failed", "err", err)
					return
				}
			}
		} else {
			if err := wr.Write([]string{"时间", "指标", "值", "单位"}); err != nil {
				slog.Warn("CSV write header failed", "err", err)
				return
			}
			for rows.Next() {
				var ts time.Time
				var m, u string
				var v float64
				if err := rows.Scan(&ts, &m, &v, &u); err != nil {
					slog.Warn("Telemetry export scan failed", "err", err)
					continue
				}
				if err := wr.Write([]string{ts.Format("2006-01-02 15:04:05"), m, ftoa(v), u}); err != nil {
					slog.Warn("CSV write row failed", "err", err)
					return
				}
			}
		}
		wr.Flush()
		if err := wr.Error(); err != nil {
			slog.Warn("CSV flush failed", "err", err)
		}
		return
	}

	var list []map[string]any
	for rows.Next() {
		if interval != "" && interval != "raw" {
			var ts time.Time
			var m string
			var av, mx, mn float64
			var c int
			if err := rows.Scan(&ts, &m, &av, &mx, &mn, &c); err != nil {
				slog.Warn("Telemetry scan failed", "err", err)
				continue
			}
			list = append(list, map[string]any{"ts": ts, "metric": m, "avg": av, "max": mx, "min": mn, "count": c})
		} else {
			var ts time.Time
			var m, u string
			var v float64
			if err := rows.Scan(&ts, &m, &v, &u); err != nil {
				slog.Warn("Telemetry scan failed", "err", err)
				continue
			}
			list = append(list, map[string]any{"ts": ts, "metric": m, "value": v, "unit": u})
		}
	}
	if list == nil {
		list = []map[string]any{}
	}
	if err := rows.Err(); err != nil {
		serverErr(w, err)
		return
	}
	ok(w, list)
}

func (h *LogHandler) Controls(w http.ResponseWriter, r *http.Request) {
	start := r.URL.Query().Get("start")
	end := r.URL.Query().Get("end")
	deviceID := queryInt(r, "device_id")
	if end == "" {
		end = time.Now().Format("2006-01-02 15:04:05")
	} else if len(end) == 10 {
		end += " 23:59:59"
	}
	if start == "" {
		start = time.Now().AddDate(0, -1, 0).Format("2006-01-02")
	}
	export := r.URL.Query().Get("export") == "csv"

	query := "SELECT created_at,project_name,building_name,device_name,prop_name,control_value,username,user_remark FROM control_record WHERE created_at>=$1 AND created_at<=$2"
	args := []any{start, end}
	argIdx := 3
	if deviceID > 0 {
		query += " AND EXISTS (SELECT 1 FROM device d WHERE d.name = control_record.device_name AND d.id=$" + itos(argIdx) + ")"
		args = append(args, deviceID)
		argIdx++
	}
	query += " ORDER BY created_at DESC LIMIT 1000"

	rows, err := h.pool.Query(r.Context(), query, args...)
	if err != nil {
		serverErr(w, err)
		return
	}
	defer rows.Close()

	if export {
		w.Header().Set("Content-Type", "text/csv; charset=utf-8")
		w.Header().Set("Content-Disposition", "attachment; filename=controls_"+time.Now().Format("20060102")+".csv")
		wr := csv.NewWriter(w)
		if err := wr.Write([]string{"时间", "项目", "楼宇", "设备", "属性", "操作值", "操作人", "备注"}); err != nil {
			slog.Warn("CSV write header failed", "err", err)
			return
		}
		for rows.Next() {
			var ts time.Time
			var pn, bn, dn, pn2, cv, un, rm string
			if err := rows.Scan(&ts, &pn, &bn, &dn, &pn2, &cv, &un, &rm); err != nil {
				slog.Warn("Controls export scan failed", "err", err)
				continue
			}
			if err := wr.Write([]string{ts.Format("2006-01-02 15:04:05"), pn, bn, dn, pn2, cv, un, rm}); err != nil {
				slog.Warn("CSV write row failed", "err", err)
				return
			}
		}
		wr.Flush()
		if err := wr.Error(); err != nil {
			slog.Warn("CSV flush failed", "err", err)
		}
		return
	}

	type ctrl struct {
		Ts     time.Time `json:"created_at"`
		Proj   string    `json:"project_name"`
		Bld    string    `json:"building_name"`
		Dev    string    `json:"device_name"`
		Prop   string    `json:"prop_name"`
		Val    string    `json:"control_value"`
		User   string    `json:"username"`
		Remark string    `json:"user_remark"`
	}
	var list []ctrl
	for rows.Next() {
		var c ctrl
		if err := rows.Scan(&c.Ts, &c.Proj, &c.Bld, &c.Dev, &c.Prop, &c.Val, &c.User, &c.Remark); err != nil {
			slog.Warn("Controls scan failed", "err", err)
			continue
		}
		list = append(list, c)
	}
	if list == nil {
		list = []ctrl{}
	}
	if err := rows.Err(); err != nil {
		serverErr(w, err)
		return
	}
	ok(w, list)
}

func (h *LogHandler) Stats(w http.ResponseWriter, r *http.Request) {
	deviceID := queryInt(r, "device_id")
	start := r.URL.Query().Get("start")
	end := r.URL.Query().Get("end")
	if end == "" {
		end = time.Now().Format("2006-01-02 15:04:05")
	} else if len(end) == 10 {
		end += " 23:59:59"
	}
	if start == "" {
		start = time.Now().AddDate(0, -1, 0).Format("2006-01-02")
	}

	rows, err := h.pool.Query(r.Context(), "SELECT metric,COUNT(*)::int as cnt,AVG(value)::numeric(10,2),SUM(value)::numeric(10,2),MAX(value)::numeric(10,2),MIN(value)::numeric(10,2) FROM device_telemetry WHERE ts>=$1 AND ts<=$2 AND ($3=0 OR device_id=$3) GROUP BY metric ORDER BY metric", start, end, deviceID)
	if err != nil {
		serverErr(w, err)
		return
	}
	defer rows.Close()
	type stat struct {
		Metric string  `json:"metric"`
		Count  int     `json:"count"`
		Avg    float64 `json:"avg"`
		Sum    float64 `json:"sum"`
		Max    float64 `json:"max"`
		Min    float64 `json:"min"`
	}
	var list []stat
	for rows.Next() {
		var s stat
		if err := rows.Scan(&s.Metric, &s.Count, &s.Avg, &s.Sum, &s.Max, &s.Min); err != nil {
			slog.Warn("log stats scan error", "err", err)
			continue
		}
		list = append(list, s)
	}
	if list == nil {
		list = []stat{}
	}
	if err := rows.Err(); err != nil {
		serverErr(w, err)
		return
	}
	ok(w, list)
}

// ExportTelemetry exports telemetry logs as CSV. Requires report.export.
func (h *LogHandler) ExportTelemetry(w http.ResponseWriter, r *http.Request) {
	r2 := r.Clone(r.Context())
	q := r2.URL.Query()
	q.Set("export", "csv")
	r2.URL.RawQuery = q.Encode()
	h.Telemetry(w, r2)
}

// ExportControls exports control records as CSV. Requires report.excel.
func (h *LogHandler) ExportControls(w http.ResponseWriter, r *http.Request) {
	r2 := r.Clone(r.Context())
	q := r2.URL.Query()
	q.Set("export", "csv")
	r2.URL.RawQuery = q.Encode()
	h.Controls(w, r2)
}

// helpers
var truncMap = map[string]string{
	"minute": "minute", "hour": "hour", "day": "day",
	"week": "week", "month": "month", "year": "year",
}
