package alarm

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

type Engine struct{ pool *pgxpool.Pool }

func New(pool *pgxpool.Pool) *Engine { return &Engine{pool} }

func (e *Engine) Evaluate(ctx context.Context, deviceID int, deviceName, deviceType, metric string, value float64) error {
	rows, err := e.pool.Query(ctx,
		`SELECT id, condition, COALESCE(threshold,0), level,
		        COALESCE(min_value,''), COALESCE(max_value,'')
		 FROM alarm_rule
		 WHERE enabled=true AND metric=$1 AND (device_type=$2 OR device_type IS NULL OR device_type='')`,
		metric, deviceType)
	if err != nil { return err }
	defer rows.Close()

	for rows.Next() {
		var id int
		var cond string
		var threshold float64
		var level, minVal, maxVal string
		if err := rows.Scan(&id, &cond, &threshold, &level, &minVal, &maxVal); err != nil {
			slog.Warn("alarm Evaluate scan failed", "err", err)
			continue
		}
		if !triggered(cond, value, threshold, minVal, maxVal) { continue }

		var recentID int
		if err := e.pool.QueryRow(ctx,
			`SELECT id FROM alarm_log WHERE device_id=$1 AND alarm_type=$2 AND ack_at IS NULL ORDER BY created_at DESC LIMIT 1`,
			deviceID, metric).Scan(&recentID); err != nil {
			recentID = 0
		}
		if recentID > 0 { continue }

		msg := buildAlarmMsg(deviceName, metric, value, cond, threshold, minVal, maxVal)
		thr := fmt.Sprintf("%.1f", threshold)
		if cond == "range" {
			thr = minVal + "~" + maxVal
		}
		if _, err := e.pool.Exec(ctx,
			`INSERT INTO alarm_log (device_id,device_name,alarm_type,level,message,value,threshold,created_at)
			 VALUES($1,$2,$3,$4,$5,$6,$7,$8)`,
			deviceID, deviceName, metric, level, msg, fmt.Sprintf("%.1f", value), thr, time.Now()); err != nil {
			slog.Error("alarm insert failed", "dev", deviceName, "err", err)
		}
		slog.Warn("alarm", "dev", deviceName, "metric", metric, "level", level)
	}
	return rows.Err()
}

func triggered(cond string, val, threshold float64, minVal, maxVal string) bool {
	switch cond {
	case "gt": return val > threshold
	case "ge": return val >= threshold
	case "lt": return val < threshold
	case "le": return val <= threshold
	case "eq": return val == threshold
	case "range":
		lo := parseFloat(minVal)
		hi := parseFloat(maxVal)
		return val < lo || val > hi
	}
	return false
}

func parseFloat(s string) float64 {
	if s == "" { return 0 }
	var f float64
	if _, err := fmt.Sscanf(s, "%f", &f); err == nil {
		return f
	}
	return 0
}

func buildAlarmMsg(deviceName, metric string, value float64, cond string, threshold float64, minV, maxV string) string {
	switch cond {
	case "range":
		return fmt.Sprintf("%s %s %.1f 超出范围 [%s, %s]", deviceName, metric, value, minV, maxV)
	case "eq":
		return fmt.Sprintf("%s %s %.1f %s %.1f", deviceName, metric, value, condCN(cond), threshold)
	default:
		return fmt.Sprintf("%s %s %.1f %s 阈值 %.1f", deviceName, metric, value, condCN(cond), threshold)
	}
}

// AlertOffline creates an alarm log entry when a device goes offline.
// Deduplicates: if an un-acked offline alarm already exists for this device, it is skipped.
func (e *Engine) AlertOffline(ctx context.Context, deviceID int, deviceName string) error {
	var existing int
	if err := e.pool.QueryRow(ctx,
		`SELECT id FROM alarm_log WHERE device_id=$1 AND alarm_type='离线' AND ack_at IS NULL LIMIT 1`,
		deviceID).Scan(&existing); err == nil && existing > 0 {
		return nil // already alerted
	}
	msg := fmt.Sprintf("%s 设备离线，网络连接已断开超过 10 分钟", deviceName)
	_, err := e.pool.Exec(ctx,
		`INSERT INTO alarm_log (device_id,device_name,alarm_type,level,message,value,threshold,created_at)
		 VALUES($1,$2,'离线','warning',$3,'','',NOW())`,
		deviceID, deviceName, msg)
	if err != nil {
		return err
	}
	slog.Warn("alarm: device offline", "dev", deviceName)
	return nil
}

func condCN(c string) string {
	switch c {
	case "gt": return "超过"
	case "ge": return "达到或超过"
	case "lt": return "低于"
	case "le": return "达到或低于"
	case "eq": return "等于"
	case "range": return "超出范围"
	}
	return c
}
