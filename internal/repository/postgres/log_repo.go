package postgres

import (
	"context"
	"encoding/csv"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"time"
)

// LogRepo 封装日志查询和导出操作
type LogRepo struct{ pool DBTX }

func NewLogRepo(pool DBTX) *LogRepo { return &LogRepo{pool} }

// TelemetryFilter 遥测日志查询过滤条件
type TelemetryFilter struct {
	DeviceID   int
	BuildingID int
	Metric     string
	Interval   string // minute, hour, day, week, month, year, raw
	Start      string
	End        string
}

// ValidIntervals 允许的interval参数白名单
var ValidIntervals = map[string]bool{
	"minute": true, "hour": true, "day": true, "week": true,
	"month": true, "year": true, "raw": true,
}

// TelemetryRow 遥测日志行
type TelemetryRow struct {
	Ts     time.Time
	Metric string
	Value  float64
	Unit   string
}

// TelemetryAggRow 聚合遥测日志行
type TelemetryAggRow struct {
	Ts   time.Time
	Metric string
	Avg   float64
	Max   float64
	Min   float64
	Count int
}

// Telemetry 返回遥测日志数据
func (r *LogRepo) Telemetry(ctx context.Context, filter TelemetryFilter) ([]map[string]any, error) {
	query := ""
	args := []any{}

	if filter.Interval != "" && filter.Interval != "raw" {
		query = "SELECT date_trunc($1, ts) as ts, metric, AVG(value)::numeric(10,2) as avg_val, MAX(value)::numeric(10,2) as max_val, MIN(value)::numeric(10,2) as min_val, COUNT(*)::int as cnt FROM device_telemetry WHERE ts>=$2 AND ts<=$3"
		args = append(args, filter.Interval, filter.Start, filter.End)
	} else {
		query = "SELECT ts, metric, value, unit FROM device_telemetry WHERE ts>=$1 AND ts<=$2"
		args = append(args, filter.Start, filter.End)
	}
	if filter.DeviceID > 0 {
		query += " AND device_id=$" + itos(len(args)+1)
		args = append(args, filter.DeviceID)
	}
	if filter.BuildingID > 0 {
		query += " AND EXISTS (SELECT 1 FROM device d WHERE d.id=device_telemetry.device_id AND d.building_id=$" + itos(len(args)+1) + ")"
		args = append(args, filter.BuildingID)
	}
	if filter.Metric != "" {
		query += " AND metric=$" + itos(len(args)+1)
		args = append(args, filter.Metric)
	}
	if filter.Interval != "" && filter.Interval != "raw" {
		query += " GROUP BY date_trunc($1, ts), metric ORDER BY date_trunc($1, ts) DESC, metric"
	} else {
		query += " ORDER BY ts DESC LIMIT 5000"
	}

	rows, err := r.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []map[string]any
	for rows.Next() {
		if filter.Interval != "" && filter.Interval != "raw" {
			var ts time.Time
			var m string
			var av, mx, mn float64
			var c int
			if err := rows.Scan(&ts, &m, &av, &mx, &mn, &c); err != nil {
				slog.Warn("LogRepo.Telemetry agg scan failed", "err", err)
				continue
			}
			list = append(list, map[string]any{"ts": ts, "metric": m, "avg": av, "max": mx, "min": mn, "count": c})
		} else {
			var ts time.Time
			var m, u string
			var v float64
			if err := rows.Scan(&ts, &m, &v, &u); err != nil {
				slog.Warn("LogRepo.Telemetry raw scan failed", "err", err)
				continue
			}
			list = append(list, map[string]any{"ts": ts, "metric": m, "value": v, "unit": u})
		}
	}
	return list, rows.Err()
}

// ExportTelemetryCSV 导出遥测日志为CSV
func (r *LogRepo) ExportTelemetryCSV(ctx context.Context, w http.ResponseWriter, filter TelemetryFilter) error {
	list, err := r.Telemetry(ctx, filter)
	if err != nil {
		return err
	}

	w.Header().Set("Content-Type", "text/csv; charset=utf-8")
	w.Header().Set("Content-Disposition", "attachment; filename=telemetry_"+time.Now().Format("20060102")+".csv")
	wr := csv.NewWriter(w)

	if filter.Interval != "" && filter.Interval != "raw" {
		if err := wr.Write([]string{"时间", "指标", "平均值", "最大值", "最小值", "记录数"}); err != nil {
			return err
		}
		for _, row := range list {
			ts, ok1 := row["ts"].(time.Time)
			metric, ok2 := row["metric"].(string)
			avg, ok3 := row["avg"].(float64)
			max, ok4 := row["max"].(float64)
			min, ok5 := row["min"].(float64)
			count, ok6 := row["count"].(int)
			if !ok1 || !ok2 || !ok3 || !ok4 || !ok5 || !ok6 {
				slog.Warn("ExportTelemetryCSV: unexpected row type in agg mode")
				continue
			}
			if err := wr.Write([]string{
				ts.Format("2006-01-02 15:04:05"),
				metric,
				ftoa(avg),
				ftoa(max),
				ftoa(min),
				itos(count),
			}); err != nil {
				return err
			}
		}
	} else {
		if err := wr.Write([]string{"时间", "指标", "值", "单位"}); err != nil {
			return err
		}
		for _, row := range list {
			ts, ok1 := row["ts"].(time.Time)
			metric, ok2 := row["metric"].(string)
			value, ok3 := row["value"].(float64)
			unit, ok4 := row["unit"].(string)
			if !ok1 || !ok2 || !ok3 || !ok4 {
				slog.Warn("ExportTelemetryCSV: unexpected row type in raw mode, got ts=%T metric=%T value=%T unit=%T",
					row["ts"], row["metric"], row["value"], row["unit"])
				continue
			}
			if err := wr.Write([]string{
				ts.Format("2006-01-02 15:04:05"),
				metric,
				ftoa(value),
				unit,
			}); err != nil {
				return err
			}
		}
	}
	wr.Flush()
	return wr.Error()
}

// ControlFilter 控制日志查询过滤条件
type ControlFilter struct {
	DeviceID   int
	BuildingID int
	ProjectID  int
	Start      string
	End        string
}

// ControlRow 控制日志行
type ControlRow struct {
	Ts       time.Time `json:"created_at"`
	Proj     string    `json:"project_name"`
	Bld      string    `json:"building_name"`
	Dev      string    `json:"device_name"`
	DeviceID int       `json:"device_id"`
	Prop     string    `json:"prop_name"`
	Val      string    `json:"control_value"`
	User     string    `json:"username"`
	Remark   string    `json:"user_remark"`
}

// Controls 返回控制日志数据
func (r *LogRepo) Controls(ctx context.Context, filter ControlFilter) ([]ControlRow, error) {
	query := "SELECT cr.created_at,cr.project_name,cr.building_name,cr.device_name,cr.prop_name,cr.control_value,cr.username,cr.user_remark,cr.device_id FROM control_record cr WHERE cr.created_at>=$1 AND cr.created_at<=$2"
	args := []any{filter.Start, filter.End}
	if filter.DeviceID > 0 {
		query += " AND cr.device_id=$" + itos(len(args)+1)
		args = append(args, filter.DeviceID)
	}
	if filter.BuildingID > 0 {
		query += " AND EXISTS (SELECT 1 FROM device d WHERE d.id=cr.device_id AND d.building_id=$" + itos(len(args)+1) + ")"
		args = append(args, filter.BuildingID)
	}
	if filter.ProjectID > 0 {
		query += " AND EXISTS (SELECT 1 FROM device d JOIN building b ON b.id=d.building_id WHERE d.id=cr.device_id AND b.project_id=$" + itos(len(args)+1) + ")"
		args = append(args, filter.ProjectID)
	}
	query += " ORDER BY cr.created_at DESC LIMIT 1000"

	rows, err := r.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []ControlRow
	for rows.Next() {
		var c ControlRow
		if err := rows.Scan(&c.Ts, &c.Proj, &c.Bld, &c.Dev, &c.Prop, &c.Val, &c.User, &c.Remark, &c.DeviceID); err != nil {
			slog.Warn("LogRepo.Controls scan failed", "err", err)
			continue
		}
		list = append(list, c)
	}
	return list, rows.Err()
}

// ExportControlsCSV 导出控制日志为CSV
func (r *LogRepo) ExportControlsCSV(ctx context.Context, w http.ResponseWriter, filter ControlFilter) error {
	list, err := r.Controls(ctx, filter)
	if err != nil {
		return err
	}

	w.Header().Set("Content-Type", "text/csv; charset=utf-8")
	w.Header().Set("Content-Disposition", "attachment; filename=controls_"+time.Now().Format("20060102")+".csv")
	wr := csv.NewWriter(w)

	if err := wr.Write([]string{"时间", "项目", "楼宇", "设备", "属性", "操作值", "操作人", "备注"}); err != nil {
		return err
	}
	for _, c := range list {
		if err := wr.Write([]string{
			c.Ts.Format("2006-01-02 15:04:05"),
			c.Proj, c.Bld, c.Dev, c.Prop, c.Val, c.User, c.Remark,
		}); err != nil {
			return err
		}
	}
	wr.Flush()
	return wr.Error()
}

// LogStat 日志统计行
type LogStat struct {
	Metric string  `json:"metric"`
	Count  int     `json:"count"`
	Avg    float64 `json:"avg"`
	Sum    float64 `json:"sum"`
	Max    float64 `json:"max"`
	Min    float64 `json:"min"`
}

// Stats 返回遥测统计数据
func (r *LogRepo) Stats(ctx context.Context, deviceID int, start, end string) ([]LogStat, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT metric,COUNT(*)::int as cnt,AVG(value)::numeric(10,2),SUM(value)::numeric(10,2),MAX(value)::numeric(10,2),MIN(value)::numeric(10,2)
		 FROM device_telemetry WHERE ts>=$1 AND ts<=$2 AND ($3=0 OR device_id=$3) GROUP BY metric ORDER BY metric`,
		start, end, deviceID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var list []LogStat
	for rows.Next() {
		var s LogStat
		if err := rows.Scan(&s.Metric, &s.Count, &s.Avg, &s.Sum, &s.Max, &s.Min); err != nil {
			slog.Warn("LogRepo.Stats scan error", "err", err)
			continue
		}
		list = append(list, s)
	}
	return list, rows.Err()
}

// itos 辅助函数（避免导入handler包）
func itos(i int) string { return strconv.Itoa(i) }

// ftoa 辅助函数
func ftoa(f float64) string { return fmt.Sprintf("%.2f", f) }