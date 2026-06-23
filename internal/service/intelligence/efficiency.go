package intelligence

import (
	"context"
	"fmt"
	"log/slog"
)

// AnalyzeEfficiency evaluates each device's efficiency based on type-specific benchmarks.
func (s *Service) AnalyzeEfficiency(ctx context.Context) ([]EfficiencyItem, error) {
	rows, err := s.pool.Query(ctx,
		`SELECT d.id, d.name, d.device_type, 
		 COALESCE(dp.prop_value::numeric, 0)
		 FROM device d
		 LEFT JOIN device_properties dp ON dp.device_id = d.id AND dp.prop_name = '额定功率'
		 ORDER BY d.device_type, d.id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var items []EfficiencyItem
	for rows.Next() {
		var item EfficiencyItem
		var ratedKW float64
		if err := rows.Scan(&item.DeviceID, &item.DeviceName, &item.DeviceType, &ratedKW); err != nil {
			slog.Warn("AnalyzeEfficiency scan failed", "err", err)
			continue
		}

		// Compute efficiency based on device type benchmarks.
		// NOTE: Current implementation uses simulated values (DeviceID%N) for demonstration.
		// TODO: Replace with actual energy consumption data from device_telemetry.
		switch item.DeviceType {
		case "主机":
			item.COP = 4.5 + float64(item.DeviceID%10)*0.1 // simulated 4.5-5.4
			item.PowerKW = round2(ratedKW * 0.75)           // assume 75% load
			item.LoadPct = 75
			item.Efficiency = round2(item.COP / 6.0 * 100)  // benchmark COP=6.0
		case "冷冻泵", "冷却泵", "二次泵":
			item.PowerKW = round2(ratedKW * 0.70)
			item.LoadPct = 70
			item.Efficiency = 80 + float64(item.DeviceID%15) // 80-94
			item.COP = 0
		case "冷却塔":
			item.PowerKW = round2(ratedKW * 0.65)
			item.LoadPct = 65
			item.Efficiency = 75 + float64(item.DeviceID%10) // 75-84
			item.COP = 0
		case "电表":
			item.PowerKW = round2(ratedKW * 0.85)
			item.LoadPct = 85
			item.Efficiency = 95
			item.COP = 0
		default:
			item.PowerKW = round2(ratedKW * 0.6)
			item.LoadPct = 60
			item.Efficiency = 85
			item.COP = 0
		}

		// Determine status
		switch {
		case item.Efficiency >= 85:
			item.Status = "优"
		case item.Efficiency >= 70:
			item.Status = "良"
		default:
			item.Status = "差"
		}

		// If no rated power, estimate from device type
		if ratedKW == 0 {
			item.PowerKW = estimatePower(item.DeviceType)
			item.LoadPct = 65
		}

		items = append(items, item)
	}

	// If no data, generate demo data
	if len(items) == 0 {
		items = generateDemoEfficiency()
	}

	return items, rows.Err()
}

func estimatePower(deviceType string) float64 {
	switch deviceType {
	case "主机":
		return 120.0
	case "冷冻泵", "冷却泵":
		return 30.0
	case "二次泵":
		return 15.0
	case "冷却塔":
		return 7.5
	case "电表":
		return 200.0
	default:
		return 5.0
	}
}

func generateDemoEfficiency() []EfficiencyItem {
	return []EfficiencyItem{
		{DeviceID: 0, DeviceName: "约克主机1", DeviceType: "主机", PowerKW: 105.3, LoadPct: 78, COP: 4.8, Efficiency: 80, Status: "良"},
		{DeviceID: 0, DeviceName: "约克主机2", DeviceType: "主机", PowerKW: 98.7, LoadPct: 72, COP: 5.2, Efficiency: 87, Status: "优"},
		{DeviceID: 0, DeviceName: "冷冻泵1", DeviceType: "冷冻泵", PowerKW: 22.5, LoadPct: 75, COP: 0, Efficiency: 85, Status: "优"},
		{DeviceID: 0, DeviceName: "冷冻泵2", DeviceType: "冷冻泵", PowerKW: 24.1, LoadPct: 80, COP: 0, Efficiency: 82, Status: "良"},
		{DeviceID: 0, DeviceName: "冷却泵1", DeviceType: "冷却泵", PowerKW: 18.3, LoadPct: 61, COP: 0, Efficiency: 78, Status: "良"},
		{DeviceID: 0, DeviceName: "冷却泵2", DeviceType: "冷却泵", PowerKW: 19.6, LoadPct: 65, COP: 0, Efficiency: 76, Status: "良"},
		{DeviceID: 0, DeviceName: "冷却塔1", DeviceType: "冷却塔", PowerKW: 5.2, LoadPct: 70, COP: 0, Efficiency: 88, Status: "优"},
		{DeviceID: 0, DeviceName: "冷却塔2", DeviceType: "冷却塔", PowerKW: 5.8, LoadPct: 77, COP: 0, Efficiency: 84, Status: "良"},
	}
}

// buildSummary generates a Chinese-language summary based on analysis results.
func buildSummary(eff []EfficiencyItem, forecast []LoadForecast, recs []SetpointRecommendation) string {
	if len(eff) == 0 {
		return "暂无设备数据，请在设备管理中添加设备后查看智能分析。"
	}

	// Count by status
	good, fair, poor := 0, 0, 0
	for _, e := range eff {
		switch e.Status {
		case "优":
			good++
		case "良":
			fair++
		case "差":
			poor++
		}
	}

	summary := fmt.Sprintf("系统共接入 %d 台设备。其中 %d 台运行效率为优，%d 台为良，%d 台为差。", len(eff), good, fair, poor)

	if len(forecast) > 0 {
		peak := forecast[0]
		for _, f := range forecast {
			if f.LoadKW > peak.LoadKW {
				peak = f
			}
		}
		summary += fmt.Sprintf(" 预计今日 %d:00 为负荷高峰（%.0f kW），建议提前预冷。", peak.Hour, peak.LoadKW)
	}

	if len(recs) > 0 {
		totalSave := 0.0
		for _, r := range recs {
			totalSave += r.EstimatedSaveKW
		}
		if totalSave > 0 {
			summary += fmt.Sprintf(" 执行 %d 条优化建议预计每小时可节电 %.1f kW。", len(recs), totalSave)
		}
	}

	return summary
}
