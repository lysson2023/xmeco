package alarm

import (
	"context"
	"fmt"
	"log/slog"
	"strconv"
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

		msg := buildAlarmMsg(deviceName, metric, value, cond, threshold, minVal, maxVal)
		thr := fmt.Sprintf("%.1f", threshold)
		if cond == "range" {
			thr = minVal + "~" + maxVal
		}
		// Atomic dedup via partial unique index — eliminates TOCTOU race
		// between SELECT and INSERT.
		tag, err := e.pool.Exec(ctx,
			`INSERT INTO alarm_log (device_id,device_name,alarm_type,level,message,value,threshold,created_at)
			 VALUES($1,$2,$3,$4,$5,$6,$7,$8)
			 ON CONFLICT (device_id, alarm_type) WHERE ack_at IS NULL DO NOTHING`,
			deviceID, deviceName, metric, level, msg, fmt.Sprintf("%.1f", value), thr, time.Now())
		if err != nil {
			slog.Error("alarm insert failed", "dev", deviceName, "err", err)
		} else if tag.RowsAffected() > 0 {
			slog.Warn("alarm", "dev", deviceName, "metric", metric, "level", level)
		}
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
		lo, loOK := parseFloatOK(minVal)
		hi, hiOK := parseFloatOK(maxVal)
		// If a non-empty bound failed to parse, skip the rule to avoid
		// false positives (0 is a valid number, not a parse-failure sentinel).
		if (minVal != "" && !loOK) || (maxVal != "" && !hiOK) {
			return false
		}
		below := loOK && val < lo
		above := hiOK && val > hi
		return below || above
	}
	return false
}

// parseFloatOK parses s as a float64.
// The second return value is false when s is empty or not a valid number.
func parseFloatOK(s string) (float64, bool) {
	if s == "" {
		return 0, false
	}
	f, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return 0, false
	}
	return f, true
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
// Uses alarm_type='离线' (separate namespace from metric-based alarm_type
// used by Evaluate). Deduplicates atomically via INSERT ... ON CONFLICT
// DO NOTHING on the partial unique index (device_id, alarm_type)
// WHERE ack_at IS NULL.
func (e *Engine) AlertOffline(ctx context.Context, deviceID int, deviceName string) error {
	msg := fmt.Sprintf("%s 设备离线，网络连接已断开超过 10 分钟", deviceName)
	tag, err := e.pool.Exec(ctx,
		`INSERT INTO alarm_log (device_id,device_name,alarm_type,level,message,value,threshold,created_at)
		 VALUES($1,$2,'离线','warning',$3,'','',NOW())
		 ON CONFLICT (device_id, alarm_type) WHERE ack_at IS NULL DO NOTHING`,
		deviceID, deviceName, msg)
	if err != nil {
		return err
	}
	if tag.RowsAffected() > 0 {
		slog.Warn("alarm: device offline", "dev", deviceName)
	}
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
