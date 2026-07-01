package intelligence

import (
	"context"
	"fmt"
	"log/slog"
	"math"
)

// 冷机-冷却塔联动调优常量
const (
	// copGainPerDegreeC — 冷却水温每降低 1°C 预计 COP 提升百分比（行业经验值 2~4%/°C）
	copGainPerDegreeC = 3.0

	// saveKWPerDegreeC — 典型冷机冷却水温每降低 1°C 的节电功率（中等负荷下约 1.8 kW/°C）
	saveKWPerDegreeC = 1.8

	// minApproachTemp — 冷却塔最小可达成逼近温度（湿球温度低时）
	minApproachTemp = 3.5

	// maxApproachTemp — 高温高湿工况下的逼近温度
	maxApproachTemp = 6.0

	// defaultApproachTemp — 数据库无冷机数据时的缺省逼近温度
	defaultApproachTemp = 5.0

	// coolWeatherApproachTemp — 湿球温度较低（<20°C）时的推荐逼近温度
	coolWeatherApproachTemp = 4.0
)

// ---- Cooperative Control Types ----

// StrategyResult bundles all cooperative control strategy outputs.
type StrategyResult struct {
	Linkages     []LinkageStrategy     `json:"linkages"`
	PumpOptimize []PumpOptimization    `json:"pump_optimize"`
	PriceTactic  *PriceTactic          `json:"price_tactic"`
	RotationPlan []RotationItem        `json:"rotation_plan"`
	TotalSaveKW  float64               `json:"total_save_kw"`
	TotalSaveDay float64               `json:"total_save_day"`
	Summary      string                `json:"summary"`
}

// LinkageStrategy: chiller + cooling tower cooperative control
type LinkageStrategy struct {
	DeviceID          int     `json:"device_id"`
	DeviceName        string  `json:"device_name"`
	CurrentCoolingTemp float64 `json:"current_cooling_temp"` // current cooling water temp
	TargetCoolingTemp  float64 `json:"target_cooling_temp"`   // recommended
	WetBulbTemp       float64 `json:"wet_bulb_temp"`         // estimated wet bulb
	COPImprovement    float64 `json:"cop_improvement"`       // expected COP gain
	SaveKWPerHour     float64 `json:"save_kw_per_hour"`
	Reason            string  `json:"reason"`
	Active            bool    `json:"active"` // false = already optimal
}

// PumpOptimization: VFD frequency tuning
type PumpOptimization struct {
	DeviceID       int     `json:"device_id"`
	DeviceName     string  `json:"device_name"`
	DeviceType     string  `json:"device_type"`
	CurrentFreq    float64 `json:"current_freq"`
	TargetFreq     float64 `json:"target_freq"`
	DeltaT         float64 `json:"delta_t"`         // current temp differential
	TargetDeltaT   float64 `json:"target_delta_t"`   // design delta-T
	FlowRatio      float64 `json:"flow_ratio"`       // current/design flow
	SaveKWPerHour  float64 `json:"save_kw_per_hour"`
	Reason         string  `json:"reason"`
	Active         bool    `json:"active"`
}

// PriceTactic: time-of-use electricity pricing strategy
type PriceTactic struct {
	CurrentHour    int       `json:"current_hour"`
	CurrentPeriod  string    `json:"current_period"`  // 谷/平/峰/尖
	CurrentPrice   float64   `json:"current_price"`    // 元/kWh
	Recommendation string    `json:"recommendation"`   // 策略建议
	Periods        []PricePeriod `json:"periods"`       // all defined periods
	Active         bool      `json:"active"`
}

// PricePeriod represents a single electricity price period.
type PricePeriod struct {
	Name   string  `json:"name"`   // 谷时/平时/峰时/尖峰
	Start  int     `json:"start"`  // 0-23
	End    int     `json:"end"`
	Price  float64 `json:"price"`  // 元/kWh
}

// RotationItem: equipment rotation plan
type RotationItem struct {
	DeviceID       int     `json:"device_id"`
	DeviceName     string  `json:"device_name"`
	DeviceType     string  `json:"device_type"`
	RunHours       float64 `json:"run_hours"`       // accumulated hours
	Priority       int     `json:"priority"`         // 1=primary, 2=standby, 3=rest
	Recommendation string  `json:"recommendation"`   // 主机/备机/停机
	Reason         string  `json:"reason"`
}

// ---- Wet bulb calculation ----

// estimateWetBulb approximates wet bulb temperature from dry bulb and humidity.
// Formula: Tw = T * atan(0.151977 * sqrt(RH + 8.313659)) + atan(T + RH) - atan(RH - 1.676331) + 0.00391838 * RH^(3/2) * atan(0.023101 * RH) - 4.686035
// Simplified for engineering use.
func estimateWetBulb(dryBulbC, rhPct float64) float64 {
	T := dryBulbC
	RH := rhPct
	// Stull formula (simplified, accurate for 5-35°C, 10-90% RH)
	Tw := T*math.Atan(0.151977*math.Sqrt(RH+8.313659)) +
		math.Atan(T+RH) -
		math.Atan(RH-1.676331) +
		0.00391838*math.Pow(RH, 1.5)*math.Atan(0.023101*RH) -
		4.686035
	return round2(Tw)
}

// ---- Chiller-Tower Linkage Strategy ----

func (s *Service) linkageStrategies(ctx context.Context, outdoorTemp, outdoorHumidity float64) []LinkageStrategy {
	wetBulb := estimateWetBulb(outdoorTemp, outdoorHumidity)

	// Query chillers
	rows, err := s.pool.Query(ctx,
		`SELECT d.id, d.name FROM device d WHERE d.device_type = '主机' LIMIT 5`)
	if err != nil {
		return defaultLinkages(wetBulb)
	}
	defer rows.Close()

	var results []LinkageStrategy
	for rows.Next() {
		var id int
		var name string
		if err := rows.Scan(&id, &name); err != nil {
			slog.Warn("linkageStrategies scan failed", "err", err)
			continue
		}

		// Cooling water approach: typically tw + 4~6°C
		currentApproach := maxApproachTemp
		targetApproach := currentApproach

		// Optimal approach depends on wet bulb
		if wetBulb <= 15 {
			targetApproach = minApproachTemp // lower approach when cool
		} else if wetBulb <= 20 {
			targetApproach = minApproachTemp + 1.0
		} else if wetBulb <= 25 {
			targetApproach = minApproachTemp + 2.0
		} else {
			targetApproach = maxApproachTemp
		}

		currentCoolingTemp := wetBulb + currentApproach
		targetCoolingTemp := wetBulb + targetApproach

		diff := currentCoolingTemp - targetCoolingTemp
		active := diff > 0.5

		copImprove := diff * copGainPerDegreeC
		saveKW := diff * saveKWPerDegreeC

		var reason string
		if active {
			reason = fmt.Sprintf("当前湿球温度 %.1f°C，冷却水温可降低 %.1f°C 至 %.1f°C，预计 COP 提升 %.0f%%",
				wetBulb, diff, targetCoolingTemp, copImprove)
		} else {
			reason = fmt.Sprintf("当前冷却水温已处于最优区间（湿球 %.1f°C + 逼近 %.1f°C）", wetBulb, currentApproach)
		}

		results = append(results, LinkageStrategy{
			DeviceID:            id,
			DeviceName:          name,
			CurrentCoolingTemp:  round2(currentCoolingTemp),
			TargetCoolingTemp:   round2(targetCoolingTemp),
			WetBulbTemp:         wetBulb,
			COPImprovement:      round2(copImprove),
			SaveKWPerHour:       round2(saveKW),
			Reason:              reason,
			Active:              active,
		})
	}

	if len(results) == 0 {
		return defaultLinkages(wetBulb)
	}
	return results
}

func defaultLinkages(wetBulb float64) []LinkageStrategy {
	approach := defaultApproachTemp
	if wetBulb < 20 {
		approach = coolWeatherApproachTemp
	}
	current := wetBulb + maxApproachTemp
	target := wetBulb + approach
	diff := current - target
	return []LinkageStrategy{{
		DeviceID: 0, DeviceName: "主机(默认)", CurrentCoolingTemp: round2(current),
		TargetCoolingTemp: round2(target), WetBulbTemp: wetBulb,
		COPImprovement: round2(diff * copGainPerDegreeC), SaveKWPerHour: round2(diff * saveKWPerDegreeC),
		Reason: fmt.Sprintf("湿球温度 %.1f°C，建议降低冷却水温 %.1f°C", wetBulb, diff), Active: diff > 0.5,
	}}
}
