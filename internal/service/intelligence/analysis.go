package intelligence

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"math"

	"github.com/jackc/pgx/v5"

	"xmeco/internal/repository/postgres"
)

// Service bundles all intelligence analysis capabilities.
type Service struct {
	pool postgres.DBTX
}

// New creates a new intelligence service.
func New(pool postgres.DBTX) *Service {
	return &Service{pool: pool}
}

// ---- Shared DTOs ----

// EfficiencyItem represents a single device's efficiency analysis.
type EfficiencyItem struct {
	DeviceID    int     `json:"device_id"`
	DeviceName  string  `json:"device_name"`
	DeviceType  string  `json:"device_type"`
	PowerKW     float64 `json:"power_kw"`      // 当前功率 kW
	LoadPct     float64 `json:"load_pct"`       // 负荷率 %
	COP         float64 `json:"cop"`            // 能效比（仅主机）
	Efficiency  float64 `json:"efficiency"`     // 综合效率评分 0-100
	Status      string  `json:"status"`         // 优/良/差
}

// LoadForecast represents hourly load predictions.
type LoadForecast struct {
	Hour        int     `json:"hour"`          // 0-23
	Temp        float64 `json:"temp"`           // 预测室外温度
	LoadKW      float64 `json:"load_kw"`        // 预测冷负荷 kW
	LoadPct     float64 `json:"load_pct"`       // 负荷率 %
}

// SetpointRecommendation is a suggested setpoint change.
type SetpointRecommendation struct {
	DeviceID        int     `json:"device_id"`
	DeviceName      string  `json:"device_name"`
	Parameter       string  `json:"parameter"`        // 设定参数名
	CurrentValue    float64 `json:"current_value"`     // 当前设定值
	RecommendedValue float64 `json:"recommended_value"` // 推荐设定值
	Unit            string  `json:"unit"`
	Reason          string  `json:"reason"`            // 推荐理由
	EstimatedSaveKW float64 `json:"estimated_save_kw"` // 预估节电 kW
	Priority        string  `json:"priority"`          // 高/中/低
}

// IntelligenceResult bundles all analyses for the frontend.
type IntelligenceResult struct {
	Efficiencies    []EfficiencyItem        `json:"efficiencies"`
	Forecast        []LoadForecast          `json:"forecast"`
	Recommendations []SetpointRecommendation `json:"recommendations"`
	Summary         string                  `json:"summary"` // AI 总结语
}

// ---- Helpers ----

// round2 rounds a float64 to 2 decimal places.
func round2(v float64) float64 {
	return math.Round(v*100) / 100
}

// GetWeatherTemp fetches current outdoor temperature from weather_cache or returns estimate.
func (s *Service) GetWeatherTemp(ctx context.Context) (float64, error) {
	var temp float64
	err := s.pool.QueryRow(ctx,
		`SELECT COALESCE(temp::numeric, 25.0) FROM weather_cache ORDER BY fetched_at DESC LIMIT 1`).
		Scan(&temp)
	if err != nil {
		if !errors.Is(err, pgx.ErrNoRows) {
			slog.Warn("GetWeatherTemp query failed, using default", "err", err)
		}
		return 25.0, nil // default 25°C
	}
	return temp, nil
}

// RunFullAnalysis executes all three intelligence analyses and returns bundled results.
func (s *Service) RunFullAnalysis(ctx context.Context) (*IntelligenceResult, error) {
	outdoorTemp, _ := s.GetWeatherTemp(ctx)

	eff, err := s.AnalyzeEfficiency(ctx)
	if err != nil {
		slog.Warn("RunFullAnalysis efficiency failed, using demo data", "err", err)
	}
	forecast := s.ForecastLoad(ctx, outdoorTemp)
	recs := s.RecommendSetpoints(ctx, outdoorTemp)

	return &IntelligenceResult{
		Efficiencies:    eff,
		Forecast:        forecast,
		Recommendations: recs,
		Summary:         buildSummary(eff, forecast, recs),
	}, nil
}

// RunStrategies executes all cooperative control strategies and returns bundled results.
func (s *Service) RunStrategies(ctx context.Context) (*StrategyResult, error) {
	outdoorTemp, _ := s.GetWeatherTemp(ctx)
	outdoorHumidity := 60.0 // default; could read from weather_cache

	// Try to get actual humidity from weather cache
	var hum float64
	err := s.pool.QueryRow(ctx,
		`SELECT COALESCE(humidity::numeric, 60.0) FROM weather_cache ORDER BY fetched_at DESC LIMIT 1`).Scan(&hum)
	if err == nil {
		outdoorHumidity = hum
	}

	linkages := s.linkageStrategies(ctx, outdoorTemp, outdoorHumidity)
	pumps := s.pumpOptimizations(ctx)
	price := s.priceTactic(ctx)
	rotation := s.rotationPlan(ctx)

	// Calculate total savings
	var totalKW float64
	for _, l := range linkages {
		if l.Active {
			totalKW += l.SaveKWPerHour
		}
	}
	for _, p := range pumps {
		if p.Active {
			totalKW += p.SaveKWPerHour
		}
	}
	totalDay := totalKW * 24

	// Build summary
	activeCount := 0
	for _, l := range linkages {
		if l.Active {
			activeCount++
		}
	}
	pumpActive := 0
	for _, p := range pumps {
		if p.Active {
			pumpActive++
		}
	}
	summary := fmt.Sprintf("协同控制分析完成：%d 条联动策略、%d 条泵优化策略生效。", activeCount, pumpActive)
	summary += fmt.Sprintf(" 当前电价时段：%s (%.2f 元/kWh)。", price.CurrentPeriod, price.CurrentPrice)
	if totalKW > 0 {
		summary += fmt.Sprintf(" 执行所有策略预计每小时节电 %.1f kW。", totalKW)
	}

	return &StrategyResult{
		Linkages:     linkages,
		PumpOptimize: pumps,
		PriceTactic:  price,
		RotationPlan: rotation,
		TotalSaveKW:  round2(totalKW),
		TotalSaveDay: round2(totalDay),
		Summary:      summary,
	}, nil
}
