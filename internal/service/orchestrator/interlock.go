package orchestrator

import (
	"context"
	"fmt"
	"log/slog"

	"xmeco/internal/repository/postgres"
)

type Interlock struct{ pool postgres.DBTX }

func NewInterlock(pool postgres.DBTX) *Interlock { return &Interlock{pool} }

// Check verifies that all interlock conditions for a target device+action are met
func (il *Interlock) Check(ctx context.Context, targetDeviceID int, action string) error {
	rows, err := il.pool.Query(ctx,
		`SELECT ic.check_device_id, ic.check_prop_name, ic.check_expected_value, ic.message, d.name
		 FROM interlock_config ic JOIN device d ON d.id=ic.check_device_id
		 WHERE ic.target_device_id=$1 AND ic.target_action=$2`, targetDeviceID, action)
	if err != nil { return err }
	defer rows.Close()

	hasRules := false
	for rows.Next() {
		hasRules = true
		var checkDevID int; var propName, expected, msg, devName string
		if err := rows.Scan(&checkDevID, &propName, &expected, &msg, &devName); err != nil {
			slog.Warn("interlock check scan failed", "target_dev", targetDeviceID, "action", action, "err", err)
			continue
		}

		var actual string
		err := il.pool.QueryRow(ctx,
			`SELECT prop_value FROM device_properties WHERE device_id=$1 AND prop_name=$2 ORDER BY id LIMIT 1`,
			checkDevID, propName).Scan(&actual)
		if err != nil {
			// DB 查询失败时采用 fail-safe 策略：阻止操作并返回错误，而非静默放行
			slog.Warn("interlock: property query failed, blocking for safety", "dev", checkDevID, "prop", propName, "err", err)
			return fmt.Errorf("联锁检查异常：无法查询 %s 的 %s 属性", devName, propName)
		}

		if actual != expected {
			if msg == "" { msg = devName + " " + propName + " 应为 " + expected + "，实际为 " + actual }
			return fmt.Errorf("%s", msg)
		}
	}
	if !hasRules {
		slog.Debug("interlock: no rules configured, allowing", "target_dev", targetDeviceID, "action", action)
	}
	return rows.Err()
}
