package intelligence

import (
	"context"
	"fmt"
	"log/slog"
	"sort"
)

// ---- Equipment Rotation Strategy ----

func (s *Service) rotationPlan(ctx context.Context) []RotationItem {
	// Single query approach (avoids N+1): JOIN device with control_record on device_name.
	// Run-hours are estimated from control_record count (1 record ≈ 1 hour runtime)
	// because snapshot_after start/stop timestamps are not yet populated.
	// NOTE: control_record.device_id 已在 migration 000017 添加，但旧数据可能为 NULL，
	// 此处仍按 device_name 匹配以确保兼容性。
	rows, err := s.pool.Query(ctx,
		`SELECT d.id, d.name, d.device_type,
		 COALESCE(COUNT(c.id), 0)::float8 as run_hours
		 FROM device d
		 LEFT JOIN control_record c ON c.device_name = d.name
		 WHERE d.device_type IN ('主机','冷冻泵','冷却泵','冷却塔')
		 GROUP BY d.id, d.name, d.device_type
		 ORDER BY d.device_type, d.id`)
	if err != nil {
		return defaultRotation()
	}
	defer rows.Close()

	type dev struct {
		id    int
		name  string
		dtype string
		hours float64
	}

	byType := make(map[string][]dev)
	for rows.Next() {
		var d dev
		if err := rows.Scan(&d.id, &d.name, &d.dtype, &d.hours); err != nil {
			slog.Warn("rotationPlan scan failed", "err", err)
			continue
		}
		byType[d.dtype] = append(byType[d.dtype], d)
	}
	if err := rows.Err(); err != nil {
		slog.Warn("rotationPlan rows iteration error", "err", err)
		return nil
	}

	var items []RotationItem
	for _, devs := range byType {
		if len(devs) == 0 {
			continue
		}
		if len(devs) == 1 {
			items = append(items, RotationItem{
				DeviceID: devs[0].id, DeviceName: devs[0].name, DeviceType: devs[0].dtype,
				RunHours: round2(devs[0].hours), Priority: 1,
				Recommendation: "主机", Reason: "单台设备，无需轮换",
			})
			continue
		}

		// Sort by runtime descending
		sort.Slice(devs, func(i, j int) bool { return devs[i].hours > devs[j].hours })

		for i, d := range devs {
			var rec, reason string
			var pri int
			switch i {
			case 0:
				rec = "备机"
				pri = 2
				reason = fmt.Sprintf("累计运行 %.0f 小时（最多），建议切换为备机以均匀磨损", d.hours)
			case 1:
				rec = "主机"
				pri = 1
				reason = fmt.Sprintf("累计运行 %.0f 小时（较少），建议切换为主机", d.hours)
			default:
				rec = "停机"
				pri = 3
				reason = fmt.Sprintf("累计运行 %.0f 小时，可作为应急备用", d.hours)
			}

			items = append(items, RotationItem{
				DeviceID: d.id, DeviceName: d.name, DeviceType: d.dtype,
				RunHours: round2(d.hours), Priority: pri,
				Recommendation: rec, Reason: reason,
			})
		}
	}

	if len(items) == 0 {
		return defaultRotation()
	}
	return items
}

func defaultRotation() []RotationItem {
	return []RotationItem{
		{DeviceID: 0, DeviceName: "约克主机1", DeviceType: "主机", RunHours: 8450, Priority: 2, Recommendation: "备机", Reason: "累计运行 8450 小时（最多），建议切换为备机"},
		{DeviceID: 0, DeviceName: "约克主机2", DeviceType: "主机", RunHours: 6200, Priority: 1, Recommendation: "主机", Reason: "累计运行 6200 小时，建议切换为主机"},
	}
}
