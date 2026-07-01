package intelligence

import (
	"context"
	"fmt"
	"log/slog"
	"math"
)

// devRow holds a device row from the initial query.
type devRow struct {
	id         int
	name       string
	deviceType string
	ratedKW    float64
}

// AnalyzeEfficiency evaluates each device's efficiency based on type-specific benchmarks.
// When device_telemetry contains actual power readings they are used for load %
// and COP calculation; otherwise the function falls back to rated-power estimates.
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

	// Collect all devices first so we can batch-query telemetry.
	var devs []devRow
	for rows.Next() {
		var d devRow
		if err := rows.Scan(&d.id, &d.name, &d.deviceType, &d.ratedKW); err != nil {
			slog.Warn("AnalyzeEfficiency scan failed", "err", err)
			continue
		}
		devs = append(devs, d)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	if len(devs) == 0 {
		return []EfficiencyItem{}, nil
	}

	// Batch query latest power telemetry for all relevant devices.
	powerMap := s.fetchLatestPowerMap(ctx)

	var items []EfficiencyItem
	for _, d := range devs {
		item := EfficiencyItem{
			DeviceID:   d.id,
			DeviceName: d.name,
			DeviceType: d.deviceType,
		}
		ratedKW := d.ratedKW
		if ratedKW <= 0 {
			ratedKW = estimatePower(d.deviceType)
		}

		actualPower, hasTelemetry := powerMap[d.id]

		switch d.deviceType {
		case "主机":
			designCOP := 5.0
			if hasTelemetry && actualPower > 0 {
				item.PowerKW = round2(actualPower)
				item.LoadPct = round2(actualPower / ratedKW * 100)
				// Cooling capacity from rated power × design COP × part-load factor.
				// Non-linear: chillers peak at 70–80 % load, drop off at low loads.
				coolingKW := ratedKW * designCOP * chillerPartLoadFactor(item.LoadPct)
				if item.PowerKW > 0 {
					item.COP = round2(coolingKW / item.PowerKW)
					item.Efficiency = round2(item.COP / designCOP * 100)
				} else {
					item.COP = 0
					item.Efficiency = 0
				}
			} else {
				item.PowerKW = round2(ratedKW * 0.75)
				item.LoadPct = 75
				item.COP = round2(4.5 + ratedKW*0.001)
				item.Efficiency = round2(item.COP / designCOP * 100)
			}

		case "冷冻泵", "冷却泵", "二次泵":
			if hasTelemetry && actualPower > 0 {
				item.PowerKW = round2(actualPower)
				item.LoadPct = round2(actualPower / ratedKW * 100)
			} else {
				item.PowerKW = round2(ratedKW * 0.70)
				item.LoadPct = 70
			}
			item.COP = 0
			item.Efficiency = pumpEfficiencyScore(item.LoadPct)

		case "冷却塔":
			if hasTelemetry && actualPower > 0 {
				item.PowerKW = round2(actualPower)
				item.LoadPct = round2(actualPower / ratedKW * 100)
			} else {
				item.PowerKW = round2(ratedKW * 0.65)
				item.LoadPct = 65
			}
			item.COP = 0
			item.Efficiency = towerEfficiencyScore(item.LoadPct)

		case "电表":
			if hasTelemetry && actualPower > 0 {
				item.PowerKW = round2(actualPower)
				item.LoadPct = round2(actualPower / ratedKW * 100)
			} else {
				item.PowerKW = round2(ratedKW * 0.85)
				item.LoadPct = 85
			}
			item.COP = 0
			item.Efficiency = 95

		default:
			if hasTelemetry && actualPower > 0 {
				item.PowerKW = round2(actualPower)
				item.LoadPct = round2(actualPower / ratedKW * 100)
			} else {
				item.PowerKW = round2(ratedKW * 0.6)
				item.LoadPct = 60
			}
			item.COP = 0
			item.Efficiency = 85
		}

		// Clamp to valid range and guard against NaN/Inf.
		if math.IsNaN(item.Efficiency) || math.IsInf(item.Efficiency, 0) {
			item.Efficiency = 0
		}
		if item.Efficiency > 100 {
			item.Efficiency = 100
		}
		if item.Efficiency < 0 {
			item.Efficiency = 0
		}

		switch {
		case item.Efficiency >= 85:
			item.Status = "优"
		case item.Efficiency >= 70:
			item.Status = "良"
		default:
			item.Status = "差"
		}

		items = append(items, item)
	}

	return items, nil
}

// fetchLatestPowerMap queries device_telemetry for the latest power reading of
// every device. Uses explicit metric name matching instead of LIKE to leverage
// a B-tree index on the metric column.
func (s *Service) fetchLatestPowerMap(ctx context.Context) map[int]float64 {
	rows, err := s.pool.Query(ctx,
		`SELECT DISTINCT ON (device_id) device_id, value
		 FROM device_telemetry
		 WHERE metric = ANY($1)
		 ORDER BY device_id, ts DESC`,
		[]string{"有功功率", "active_power", "P", "功率", "总功率"})
	if err != nil {
		slog.Warn("fetchLatestPowerMap query failed", "err", err)
		return nil
	}
	defer rows.Close()

	m := make(map[int]float64)
	for rows.Next() {
		var id int
		var v float64
		if err := rows.Scan(&id, &v); err != nil {
			continue
		}
		m[id] = v
	}
	if err := rows.Err(); err != nil {
		slog.Warn("fetchLatestPowerMap rows iteration error", "err", err)
	}
	return m
}

// chillerPartLoadFactor returns the cooling output fraction (0–1) for a given
// electrical load percentage, based on a typical chiller part-load curve.
// Efficiency peaks near 70–80 % load; very low loads see significant penalty.
func chillerPartLoadFactor(loadPct float64) float64 {
	l := loadPct
	if l > 100 {
		l = 100 // cannot exceed rated capacity
	}
	l = l / 100
	switch {
	case loadPct >= 70:
		return l * 0.98
	case loadPct >= 50:
		return l * 0.95
	case loadPct >= 30:
		return l * 0.88
	default:
		return l * 0.75
	}
}

// pumpEfficiencyScore returns an efficiency score (0–100) for a pump based on
// its load percentage. Pumps are most efficient in the 60–85 % load range.
func pumpEfficiencyScore(loadPct float64) float64 {
	switch {
	case loadPct >= 60 && loadPct <= 85:
		return 88 + (loadPct-60)*0.2 // 88–93 in sweet spot
	case loadPct > 85:
		return 93 - (loadPct-85)*0.6 // gradually drops above 85%
	case loadPct > 30:
		return 75 + (loadPct-30)*0.4 // ramps up
	default:
		return 65 // very low load → poor efficiency
	}
}

// towerEfficiencyScore returns an efficiency score (0–100) for a cooling tower.
// Towers are designed for ~65–80 % load; outside that range efficiency drops.
func towerEfficiencyScore(loadPct float64) float64 {
	switch {
	case loadPct >= 60 && loadPct <= 85:
		return 82 + (loadPct-60)*0.3 // 82–89 in design range
	case loadPct > 85:
		return 89 - (loadPct-85)*0.5
	case loadPct > 30:
		return 70 + (loadPct-30)*0.4
	default:
		return 60
	}
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
