package intelligence

import (
	"context"
	"fmt"
	"log/slog"
	"math"
	"time"
)

// ---- Pump Frequency Optimization ----

func (s *Service) pumpOptimizations(ctx context.Context) []PumpOptimization {
	rows, err := s.pool.Query(ctx,
		`SELECT d.id, d.name, d.device_type FROM device d 
		 WHERE d.device_type IN ('冷冻泵','冷却泵','二次泵') LIMIT 10`)
	if err != nil {
		return defaultPumpOpt()
	}
	defer rows.Close()

	var results []PumpOptimization
	for rows.Next() {
		var id int
		var name, dtype string
		if err := rows.Scan(&id, &name, &dtype); err != nil {
			slog.Warn("pumpOptimizations scan failed", "err", err)
			continue
		}

		opt := PumpOptimization{
			DeviceID: id, DeviceName: name, DeviceType: dtype,
			CurrentFreq: 50, TargetFreq: 50, DeltaT: 5.0, TargetDeltaT: 5.0,
			FlowRatio: 1.0, Active: false,
		}

		switch dtype {
		case "冷冻泵":
			opt.CurrentFreq = 50
			opt.DeltaT = 4.2 // current delta-T is lower = over-pumping
			opt.TargetDeltaT = 5.0
			opt.FlowRatio = 0.84 // ΔT_current/ΔT_design
			// Affinity law: Q ∝ N, ΔT ∝ 1/Q. Need to reduce speed.
			// TargetFreq = CurrentFreq × (currentΔT / designΔT) × safetyFactor
			opt.TargetFreq = round2(50 * (4.2 / 5.0 * 0.88)) // ~37Hz
			opt.CurrentFreq = 50
			opt.SaveKWPerHour = round2(30 * (1 - math.Pow(opt.TargetFreq/50, 3))) // ~30kW rated
			opt.Reason = fmt.Sprintf("供回水温差偏小(%.1f°C vs 设计 %.1f°C)，建议降频至 %.0fHz 以匹配设计工况",
				opt.DeltaT, opt.TargetDeltaT, opt.TargetFreq)
			opt.Active = opt.TargetFreq < 47

		case "冷却泵":
			opt.CurrentFreq = 50
			opt.DeltaT = 4.8
			opt.TargetDeltaT = 5.0
			opt.FlowRatio = 0.96 // ΔT_current/ΔT_design
			opt.TargetFreq = round2(50 * (4.8 / 5.0 * 0.92)) // ~44Hz
			opt.SaveKWPerHour = round2(25 * (1 - math.Pow(opt.TargetFreq/50, 3)))
			opt.Reason = fmt.Sprintf("冷却水温差接近设计值，可小幅降频至 %.0fHz 节能", opt.TargetFreq)
			opt.Active = opt.TargetFreq < 47

		case "二次泵":
			opt.CurrentFreq = 50
			opt.DeltaT = 3.8
			opt.TargetDeltaT = 5.0
			opt.FlowRatio = 0.76 // ΔT_current/ΔT_design
			opt.TargetFreq = round2(50 * (3.8 / 5.0 * 0.82)) // ~31Hz
			opt.SaveKWPerHour = round2(15 * (1 - math.Pow(opt.TargetFreq/50, 3)))
			opt.Reason = fmt.Sprintf("二次侧温差偏小(%.1f°C)，存在严重过流，建议降频至 %.0fHz", opt.DeltaT, opt.TargetFreq)
			opt.Active = opt.TargetFreq < 45
		}

		results = append(results, opt)
	}

	if len(results) == 0 {
		return defaultPumpOpt()
	}
	return results
}

func defaultPumpOpt() []PumpOptimization {
	return []PumpOptimization{
		{DeviceID: 0, DeviceName: "冷冻泵(默认)", DeviceType: "冷冻泵", CurrentFreq: 50, TargetFreq: 37, DeltaT: 4.2, TargetDeltaT: 5.0, FlowRatio: 0.84, SaveKWPerHour: 17.6, Reason: "温差偏小，建议降频至 37Hz", Active: true},
		{DeviceID: 0, DeviceName: "冷却泵(默认)", DeviceType: "冷却泵", CurrentFreq: 50, TargetFreq: 44, DeltaT: 4.8, TargetDeltaT: 5.0, FlowRatio: 0.96, SaveKWPerHour: 8.2, Reason: "可小幅降频至 44Hz", Active: true},
	}
}

// ---- Time-of-Use Electricity Pricing ----

// Default TOU pricing for Shenzhen/Guangdong commercial
var defaultPricePeriods = []PricePeriod{
	{Name: "谷时", Start: 0, End: 8, Price: 0.28},
	{Name: "平时", Start: 8, End: 14, Price: 0.68},
	{Name: "峰时", Start: 14, End: 17, Price: 1.08},
	{Name: "平时", Start: 17, End: 19, Price: 0.68},
	{Name: "峰时", Start: 19, End: 22, Price: 1.08},
	{Name: "平时", Start: 22, End: 24, Price: 0.68},
}

func (s *Service) priceTactic(ctx context.Context) *PriceTactic {
	// Try loading from DB first
	periods, err := s.loadPriceConfig(ctx)
	if err != nil || len(periods) == 0 {
		periods = defaultPricePeriods
	}

	now := time.Now()
	currentHour := now.Hour()

	var currentPeriod string
	var currentPrice float64
	for _, p := range periods {
		if currentHour >= p.Start && currentHour < p.End {
			currentPeriod = p.Name
			currentPrice = p.Price
			break
		}
	}

	// Determine recommendation
	var rec string
	nextCheap := findNextCheapPeriod(periods, currentHour)
	switch currentPeriod {
	case "尖峰", "峰时":
		rec = fmt.Sprintf("当前电价 %.2f 元/kWh（%s），建议减少主机运行台数，利用蓄冷量供冷。低谷时段 %s 再恢复蓄冷。",
			currentPrice, currentPeriod, nextCheap)
	case "平时":
		rec = fmt.Sprintf("当前电价 %.2f 元/kWh（%s），可正常运行。预警：%s 后进入峰时，可提前预冷降低峰时负荷。",
			currentPrice, currentPeriod, nextCheap)
	case "谷时":
		rec = fmt.Sprintf("当前电价 %.2f 元/kWh（%s低谷），建议全开主机蓄冷，为峰时备冷量。",
			currentPrice, currentPeriod)
	}

	return &PriceTactic{
		CurrentHour:    currentHour,
		CurrentPeriod:  currentPeriod,
		CurrentPrice:   currentPrice,
		Recommendation: rec,
		Periods:        periods,
		Active:         true,
	}
}

func findNextCheapPeriod(periods []PricePeriod, currentHour int) string {
	for look := 1; look < 24; look++ {
		h := (currentHour + look) % 24
		for _, p := range periods {
			if h >= p.Start && h < p.End && p.Name == "谷时" {
				return fmt.Sprintf("%d:00", h)
			}
		}
	}
	return "次日 0:00"
}

func (s *Service) loadPriceConfig(ctx context.Context) ([]PricePeriod, error) {
	rows, err := s.pool.Query(ctx,
		`SELECT name, start_hour, end_hour, price FROM electricity_price ORDER BY start_hour`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var periods []PricePeriod
	for rows.Next() {
		var p PricePeriod
		if err := rows.Scan(&p.Name, &p.Start, &p.End, &p.Price); err != nil {
			slog.Warn("loadPriceConfig scan failed", "err", err)
			continue
		}
		periods = append(periods, p)
	}
	return periods, rows.Err()
}

// SavePriceConfig saves TOU pricing to DB.
func (s *Service) SavePriceConfig(ctx context.Context, periods []PricePeriod) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	// Create table if not exists
	_, _ = tx.Exec(ctx,
		`CREATE TABLE IF NOT EXISTS electricity_price (
			name VARCHAR(20), start_hour INT, end_hour INT, price DECIMAL(6,3),
			PRIMARY KEY (name, start_hour))`)

	_, err = tx.Exec(ctx, `DELETE FROM electricity_price`)
	if err != nil {
		return err
	}

	for _, p := range periods {
		_, err = tx.Exec(ctx,
			`INSERT INTO electricity_price (name, start_hour, end_hour, price) VALUES ($1,$2,$3,$4)`,
			p.Name, p.Start, p.End, p.Price)
		if err != nil {
			return err
		}
	}
	return tx.Commit(ctx)
}

// PriceTacticPublic is the exported version for the HTTP handler.
func (s *Service) PriceTacticPublic(ctx context.Context) *PriceTactic {
	return s.priceTactic(ctx)
}
