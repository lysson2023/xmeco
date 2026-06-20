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
		`SELECT id, condition, threshold, level FROM alarm_rule
		 WHERE enabled=true AND metric=$1 AND (device_type=$2 OR device_type IS NULL OR device_type='')`,
		metric, deviceType)
	if err != nil { return err }
	defer rows.Close()

	for rows.Next() {
		var id int; var cond string; var threshold float64; var level string
		rows.Scan(&id, &cond, &threshold, &level)
		if !triggered(cond, value, threshold) { continue }

		var recentID int
		e.pool.QueryRow(ctx,
			`SELECT id FROM alarm_log WHERE device_id=$1 AND alarm_type=$2 AND ack_at IS NULL ORDER BY created_at DESC LIMIT 1`,
			deviceID, metric).Scan(&recentID)
		if recentID > 0 { continue }

		msg := fmt.Sprintf("%s %s %.1f %s 阈值 %.1f", deviceName, metric, value, condCN(cond), threshold)
		e.pool.Exec(ctx,
			`INSERT INTO alarm_log (device_id,device_name,alarm_type,level,message,value,threshold,created_at)
			 VALUES($1,$2,$3,$4,$5,$6,$7,$8)`,
			deviceID, deviceName, metric, level, msg, fmt.Sprintf("%.1f", value), fmt.Sprintf("%.1f", threshold), time.Now())
		slog.Warn("alarm", "dev", deviceName, "metric", metric, "level", level)
	}
	return rows.Err()
}

func triggered(cond string, val, threshold float64) bool {
	switch cond { case "gt": return val > threshold; case "lt": return val < threshold; case "eq": return val == threshold }
	return false
}

func condCN(c string) string {
	switch c { case "gt": return "超过"; case "lt": return "低于"; case "eq": return "等于" }
	return c
}
