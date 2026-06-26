package orchestrator

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/jackc/pgx/v5/pgxpool"
)

type Interlock struct{ pool *pgxpool.Pool }

func NewInterlock(pool *pgxpool.Pool) *Interlock { return &Interlock{pool} }

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
		if err != nil { actual = "" }

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
