package postgres

import (
	"context"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5"
)

// TelemetryRepo 封装 device_telemetry 表的查询操作
type TelemetryRepo struct{ pool DBTX }

func NewTelemetryRepo(pool DBTX) *TelemetryRepo { return &TelemetryRepo{pool} }

// TelemetryPoint 遥测数据点
type TelemetryPoint struct {
	Ts       time.Time `json:"ts"`
	DeviceID int       `json:"device_id"`
	Metric   string    `json:"metric"`
	Value    float64   `json:"value"`
	Unit     string    `json:"unit"`
}

// Realtime 返回最新遥测数据（按 device_id, metric 唯一）
func (r *TelemetryRepo) Realtime(ctx context.Context, deviceID int) ([]TelemetryPoint, error) {
	var rows pgx.Rows
	var err error
	if deviceID > 0 {
		rows, err = r.pool.Query(ctx,
			`SELECT DISTINCT ON (device_id, metric) ts, device_id, metric, value, unit
			 FROM device_telemetry WHERE device_id=$1 ORDER BY device_id, metric, ts DESC`, deviceID)
	} else {
		rows, err = r.pool.Query(ctx,
			`SELECT DISTINCT ON (device_id, metric) ts, device_id, metric, value, unit
			 FROM device_telemetry ORDER BY device_id, metric, ts DESC`)
	}
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var list []TelemetryPoint
	for rows.Next() {
		var p TelemetryPoint
		if err := rows.Scan(&p.Ts, &p.DeviceID, &p.Metric, &p.Value, &p.Unit); err != nil {
			slog.Warn("TelemetryRepo.Realtime scan failed", "err", err)
			continue
		}
		list = append(list, p)
	}
	return list, rows.Err()
}

// HistoryPoint 历史数据点
type HistoryPoint struct {
	Ts    time.Time `json:"ts"`
	Value float64   `json:"value"`
}

// History 返回指定设备、指标的历史数据（按小时范围）
func (r *TelemetryRepo) History(ctx context.Context, deviceID int, metric string, hours int) ([]HistoryPoint, error) {
	if hours <= 0 {
		hours = 24
	}
	since := time.Now().Add(-time.Duration(hours) * time.Hour)
	rows, err := r.pool.Query(ctx,
		`SELECT ts, value FROM device_telemetry WHERE device_id=$1 AND metric=$2 AND ts>$3 ORDER BY ts`,
		deviceID, metric, since)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var list []HistoryPoint
	for rows.Next() {
		var p HistoryPoint
		if err := rows.Scan(&p.Ts, &p.Value); err != nil {
			slog.Warn("TelemetryRepo.History scan failed", "err", err)
			continue
		}
		list = append(list, p)
	}
	return list, rows.Err()
}

// DeviceStat 单设备统计
type DeviceStat struct {
	Metric string  `json:"metric"`
	Count  int     `json:"count"`
	Avg    float64 `json:"avg"`
	Max    float64 `json:"max"`
	Min    float64 `json:"min"`
}

// DeviceStats 返回单设备的统计数据
func (r *TelemetryRepo) DeviceStats(ctx context.Context, deviceID int) ([]DeviceStat, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT metric, COUNT(*)::int, AVG(value)::numeric(10,2),
		        MAX(value)::numeric(10,2), MIN(value)::numeric(10,2)
		 FROM device_telemetry WHERE device_id=$1 GROUP BY metric ORDER BY metric`, deviceID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var list []DeviceStat
	for rows.Next() {
		var s DeviceStat
		if err := rows.Scan(&s.Metric, &s.Count, &s.Avg, &s.Max, &s.Min); err != nil {
			slog.Warn("TelemetryRepo.DeviceStats scan failed", "err", err)
			continue
		}
		list = append(list, s)
	}
	return list, rows.Err()
}

// SystemStats 系统级在线/离线统计
type SystemStats struct {
	Online  int `json:"online"`
	Offline int `json:"offline"`
	Total   int `json:"total"`
}

// SystemOnlineStats 返回系统级设备在线统计
func (r *TelemetryRepo) SystemOnlineStats(ctx context.Context) (*SystemStats, error) {
	var online, offline int
	if err := r.pool.QueryRow(ctx, `SELECT count(*) FROM device WHERE online_status='在线'`).Scan(&online); err != nil {
		slog.Warn("TelemetryRepo.SystemOnlineStats online query failed", "err", err)
		online = 0
	}
	if err := r.pool.QueryRow(ctx, `SELECT count(*) FROM device WHERE online_status='离线'`).Scan(&offline); err != nil {
		slog.Warn("TelemetryRepo.SystemOnlineStats offline query failed", "err", err)
		offline = 0
	}
	return &SystemStats{Online: online, Offline: offline, Total: online + offline}, nil
}