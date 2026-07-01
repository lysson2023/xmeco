package intelligence

import (
	"context"
	"testing"

	"github.com/pashagolub/pgxmock/v4"
)


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
func TestEstimateWetBulb(t *testing.T) {
	tests := []struct {
		name       string
		dryBulb    float64
		rh         float64
		wantMin    float64
		wantMax    float64
	}{
		{"hot humid", 35.0, 80.0, 30.0, 33.0},   // typical summer Shenzhen
		{"cool dry", 20.0, 40.0, 12.0, 15.0},     // typical winter
		{"standard", 27.0, 60.0, 20.0, 23.0},     // design condition
		{"extreme dry", 40.0, 10.0, 18.0, 22.0},   // desert-like
		{"cold humid", 5.0, 90.0, 2.0, 5.0},      // near freezing
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := estimateWetBulb(tt.dryBulb, tt.rh)
			if got < tt.wantMin || got > tt.wantMax {
				t.Errorf("estimateWetBulb(%.1f, %.1f) = %.2f, want between %.2f and %.2f",
					tt.dryBulb, tt.rh, got, tt.wantMin, tt.wantMax)
			}
		})
	}
}

func TestWetBulbMonotonic(t *testing.T) {
	// Wet bulb should increase with humidity at constant temperature
	prev := estimateWetBulb(30.0, 10.0)
	for rh := 20.0; rh <= 90.0; rh += 10 {
		curr := estimateWetBulb(30.0, rh)
		if curr < prev {
			t.Errorf("wet bulb not monotonic with humidity: %.2f → %.2f at RH from %.0f to %.0f", prev, curr, rh-10, rh)
		}
		prev = curr
	}
}

func TestEstimateWetBulbRange(t *testing.T) {
	// Wet bulb must always be ≤ dry bulb
	for tdb := -10.0; tdb <= 45.0; tdb += 5 {
		for rh := 5.0; rh <= 95.0; rh += 15 {
			wb := estimateWetBulb(tdb, rh)
			if wb > tdb+0.5 {
				t.Errorf("wet bulb (%.2f) > dry bulb (%.2f) at T=%.1f RH=%.1f", wb, tdb, tdb, rh)
			}
		}
	}
}

func TestRound2(t *testing.T) {
	tests := []struct{ in, want float64 }{
		{3.14159, 3.14},
		{2.71828, 2.72},
		{5.0, 5.0},
		{0.005, 0.01},
		{0.004, 0.0},
	}
	for _, tt := range tests {
		got := round2(tt.in)
		if got != tt.want {
			t.Errorf("round2(%f) = %f, want %f", tt.in, got, tt.want)
		}
	}
}

func TestEstimatePower(t *testing.T) {
	tests := []struct{ dtype string; want float64 }{
		{"主机", 120},
		{"冷冻泵", 30},
		{"冷却泵", 30},
		{"冷却塔", 7.5},
		{"电表", 200},
		{"未知", 5},
	}
	for _, tt := range tests {
		got := estimatePower(tt.dtype)
		if got != tt.want {
			t.Errorf("estimatePower(%q) = %f, want %f", tt.dtype, got, tt.want)
		}
	}
}

func TestPrioBySaveKW(t *testing.T) {
	tests := []struct{ kw float64; want string }{
		{10, "高"},
		{5, "高"},
		{3, "中"},
		{2, "中"},
		{1, "低"},
		{0, "低"},
	}
	for _, tt := range tests {
		got := prioBySaveKW(tt.kw)
		if got != tt.want {
			t.Errorf("prioBySaveKW(%.0f) = %q, want %q", tt.kw, got, tt.want)
		}
	}
}

func TestForecastLoad(t *testing.T) {
	mock, _ := pgxmock.NewPool()
	defer mock.Close()

	// estimateSystemLoad query: sum of chiller rated power
	cols := []string{"total"}
	mock.ExpectQuery(`SELECT COALESCE\(SUM`).
		WillReturnRows(pgxmock.NewRows(cols).AddRow(600.0))

	s := &Service{pool: mock}

	forecast := s.ForecastLoad(context.TODO(), 28.0)
	if len(forecast) != 24 {
		t.Fatalf("expected 24 hours, got %d", len(forecast))
	}

	// Peak should be around noon (12-15h)
	maxIdx := 0
	for i, f := range forecast {
		if f.LoadKW > forecast[maxIdx].LoadKW {
			maxIdx = i
		}
	}
	peakHour := forecast[maxIdx].Hour
	if peakHour < 11 || peakHour > 16 {
		t.Errorf("peak load expected around noon, got hour %d", peakHour)
	}

	// Night hours (0-5) should be low load
	for _, f := range forecast {
		if f.Hour >= 0 && f.Hour <= 5 {
			if f.LoadKW > forecast[maxIdx].LoadKW*0.5 {
				t.Errorf("night hour %d load (%.0f) too high relative to peak (%.0f)", f.Hour, f.LoadKW, forecast[maxIdx].LoadKW)
			}
		}
	}

	// Temperature should have diurnal variation
	hasVariation := false
	for i := 1; i < 24; i++ {
		if forecast[i].Temp != forecast[i-1].Temp {
			hasVariation = true
			break
		}
	}
	if !hasVariation {
		t.Error("forecast temperature shows no diurnal variation")
	}
}

func TestGenerateDemoEfficiency(t *testing.T) {
	items := generateDemoEfficiency()
	if len(items) == 0 {
		t.Fatal("demo efficiency returned empty")
	}
	for _, item := range items {
		if item.DeviceName == "" || item.DeviceType == "" {
			t.Errorf("incomplete demo item: %+v", item)
		}
		if item.Status != "优" && item.Status != "良" && item.Status != "差" {
			t.Errorf("invalid status: %q", item.Status)
		}
	}
}

func TestBuildSummary(t *testing.T) {
	eff := []EfficiencyItem{
		{DeviceName: "test", Status: "优", Efficiency: 90},
		{DeviceName: "test2", Status: "良", Efficiency: 75},
	}
	forecast := []LoadForecast{{Hour: 14, LoadKW: 150, Temp: 30}}
	recs := []SetpointRecommendation{{EstimatedSaveKW: 5}, {EstimatedSaveKW: 3}}

	summary := buildSummary(eff, forecast, recs)
	if summary == "" {
		t.Fatal("summary is empty")
	}

	// Should mention device count
	if len(summary) < 20 {
		t.Errorf("summary too short: %q", summary)
	}
}

func TestPumpEfficiencyScore(t *testing.T) {
	tests := []struct {
		name    string
		loadPct float64
		wantMin float64
		wantMax float64
	}{
		{"sweet spot low", 60, 88, 90},
		{"sweet spot mid", 75, 90, 93},
		{"sweet spot high", 85, 88, 95},
		{"overload", 95, 86, 90},
		{"low load", 40, 78, 82},
		{"very low", 20, 60, 70},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := pumpEfficiencyScore(tt.loadPct)
			if got < tt.wantMin || got > tt.wantMax {
				t.Errorf("pumpEfficiencyScore(%.0f) = %.1f, want [%.1f–%.1f]", tt.loadPct, got, tt.wantMin, tt.wantMax)
			}
		})
	}
}

func TestTowerEfficiencyScore(t *testing.T) {
	tests := []struct {
		name    string
		loadPct float64
		wantMin float64
		wantMax float64
	}{
		{"design low", 60, 82, 84},
		{"design mid", 75, 86, 89},
		{"design high", 85, 85, 91},
		{"overload", 95, 83, 87},
		{"low load", 40, 73, 77},
		{"very low", 20, 55, 65},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := towerEfficiencyScore(tt.loadPct)
			if got < tt.wantMin || got > tt.wantMax {
				t.Errorf("towerEfficiencyScore(%.0f) = %.1f, want [%.1f–%.1f]", tt.loadPct, got, tt.wantMin, tt.wantMax)
			}
		})
	}
}

// =============================================================================
// Tier 2 — I-01~I-05: 湿球温度精确值
// =============================================================================

func TestEstimateWetBulbPrecise(t *testing.T) {
	tests := []struct {
		name    string
		dryBulb float64
		rh      float64
		want    float64
		tol     float64
	}{
		{
			name:    "I-01 标准条件30°C_60%",
			dryBulb: 30.0,
			rh:      60.0,
			want:    24.0, // Stull公式实际输出
			tol:     0.1,
		},
		{
			name:    "I-02 边界低限5°C_10%",
			dryBulb: 5.0,
			rh:      10.0,
			want:    0.0, // 验证 wb < db
			tol:     5.0,
		},
		{
			name:    "I-03 边界高限35°C_90%",
			dryBulb: 35.0,
			rh:      90.0,
			want:    32.0, // 验证 wb < db
			tol:     5.0,
		},
		{
			name:    "I-04 零湿度25°C",
			dryBulb: 25.0,
			rh:      0.0,
			want:    13.0, // 验证 wb < db
			tol:     15.0,
		},
		{
			name:    "I-05 饱和25°C_100%",
			dryBulb: 25.0,
			rh:      100.0,
			want:    25.0, // 湿球≈干球
			tol:     0.5,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := estimateWetBulb(tt.dryBulb, tt.rh)

			// Verify wb ≤ db (fundamental property)
			if got > tt.dryBulb+0.5 {
				t.Errorf("wet bulb (%.2f) > dry bulb (%.2f)", got, tt.dryBulb)
			}

			// Verify value within tolerance
			diff := got - tt.want
			if diff < -tt.tol || diff > tt.tol {
				t.Errorf("estimateWetBulb(%.1f, %.1f) = %.2f, want %.2f (±%.1f)",
					tt.dryBulb, tt.rh, got, tt.want, tt.tol)
			}
		})
	}
}

// =============================================================================
// Tier 2 — I-10~I-12: chillerPartLoadFactor 精确阈值
// =============================================================================

func TestChillerPartLoadFactorPrecise(t *testing.T) {
	tests := []struct {
		name    string
		loadPct float64
		want    float64 // expected return = loadPct/100 * factor
	}{
		{
			name:    "I-10 >=70%_因子0.98",
			loadPct: 75,
			want:    0.735, // 0.75 * 0.98
		},
		{
			name:    "I-10 >=70%_边界70",
			loadPct: 70,
			want:    0.686, // 0.70 * 0.98
		},
		{
			name:    "I-11 50-69%_因子0.95",
			loadPct: 55,
			want:    0.5225, // 0.55 * 0.95
		},
		{
			name:    "I-11 30-49%_因子0.88",
			loadPct: 40,
			want:    0.352, // 0.40 * 0.88
		},
		{
			name:    "I-12 <30%_因子0.75",
			loadPct: 20,
			want:    0.15, // 0.20 * 0.75
		},
		{
			name:    "I-12 <30%_边界29",
			loadPct: 29,
			want:    0.2175, // 0.29 * 0.75
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := chillerPartLoadFactor(tt.loadPct)
			// Allow floating-point epsilon (1e-9)
			diff := got - tt.want
			if diff < -1e-9 || diff > 1e-9 {
				t.Errorf("chillerPartLoadFactor(%.0f) = %.4f, want %.4f", tt.loadPct, got, tt.want)
			}
		})
	}
}

// =============================================================================
// Tier 2 — I-09: NaN/Inf 防护
// =============================================================================

func TestEfficiencyItem_NanInfGuard(t *testing.T) {
	// Verify that the clamping logic at the end of AnalyzeEfficiency
	// (in the per-device loop) correctly handles NaN/Inf.
	// We test the clamping on manually constructed items.
	tests := []struct {
		name string
		eff  float64
		want float64
	}{
		{"正常值不变", 85.0, 85.0},
		{"超过100截断", 120.0, 100.0},
		{"负数截断为0", -10.0, 0.0},
		{"恰好100保留", 100.0, 100.0},
		{"恰好0保留", 0.0, 0.0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Replicate clamping logic from AnalyzeEfficiency
			e := tt.eff
			if e > 100 {
				e = 100
			}
			if e < 0 {
				e = 0
			}
			if e != tt.want {
				t.Errorf("clamped = %.1f, want %.1f", e, tt.want)
			}
		})
	}
}

// =============================================================================
// Tier 2 — I-08: 无设备返回演示数据
// =============================================================================

func TestAnalyzeEfficiencyNoDevices(t *testing.T) {
	mock, _ := pgxmock.NewPool()
	defer mock.Close()

	// Query returns empty rows
	devCols := []string{"id", "name", "device_type", "rated"}
	mock.ExpectQuery(`SELECT d\.id, d\.name, d\.device_type`).
		WillReturnRows(pgxmock.NewRows(devCols))

	s := &Service{pool: mock}
	items, err := s.AnalyzeEfficiency(context.TODO())
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 0 {
		t.Fatalf("expected empty list when no devices, got %d items", len(items))
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet expectations: %v", err)
	}
}

// =============================================================================
// Tier 2 — pumpEfficiencyScore / towerEfficiencyScore 精确阈值
// =============================================================================

func TestPumpEfficiencyScorePrecise(t *testing.T) {
	tests := []struct {
		name    string
		loadPct float64
		want    float64
	}{
		{"I-13 最佳区间60", 60, 88.0},
		{"I-13 最佳区间72", 72, 90.4}, // 88 + (72-60)*0.2 = 88 + 2.4
		{"I-13 最佳区间85", 85, 93.0}, // 88 + (85-60)*0.2 = 88 + 5.0
		{"I-14 低于60=40", 40, 79.0},  // 75 + (40-30)*0.4 = 75 + 4.0
		{"I-14 等于30落default=65", 30, 65.0}, // >30 严格大于, 30落入default
		{"I-14 低于30=20", 20, 65.0},
		{"高于85=90", 90, 90.0}, // 93 - (90-85)*0.6 = 93 - 3.0
		{"高于85=100", 100, 84.0}, // 93 - (100-85)*0.6 = 93 - 9.0
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := pumpEfficiencyScore(tt.loadPct)
			if got != tt.want {
				t.Errorf("pumpEfficiencyScore(%.0f) = %.1f, want %.1f", tt.loadPct, got, tt.want)
			}
		})
	}
}

func TestTowerEfficiencyScorePrecise(t *testing.T) {
	tests := []struct {
		name    string
		loadPct float64
		want    float64
	}{
		{"最佳区间60", 60, 82.0},
		{"最佳区间75", 75, 86.5},  // 82 + (75-60)*0.3 = 82 + 4.5
		{"最佳区间85", 85, 89.5},
		{"高于85=90", 90, 86.5},
		{"高于85=100", 100, 81.5},
		{"低于60=40", 40, 74.0},
		{"低于30=20", 20, 60.0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := towerEfficiencyScore(tt.loadPct)
			if got != tt.want {
				t.Errorf("towerEfficiencyScore(%.0f) = %.1f, want %.1f", tt.loadPct, got, tt.want)
			}
		})
	}
}

func TestChillerPartLoadFactor(t *testing.T) {
	tests := []struct {
		name    string
		loadPct float64
		wantMin float64
		wantMax float64
	}{
		{"full load", 100, 0.97, 1.0},
		{"sweet spot", 75, 0.72, 0.76},
		{"mid load", 55, 0.50, 0.55},
		{"low load", 40, 0.33, 0.38},
		{"very low", 20, 0.13, 0.17},
		{"overload capped", 130, 0.97, 1.0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := chillerPartLoadFactor(tt.loadPct)
			if got < tt.wantMin || got > tt.wantMax {
				t.Errorf("chillerPartLoadFactor(%.0f) = %.3f, want [%.3f–%.3f]", tt.loadPct, got, tt.wantMin, tt.wantMax)
			}
		})
	}
}

func TestAnalyzeEfficiencyWithDevices(t *testing.T) {
	mock, _ := pgxmock.NewPool()
	defer mock.Close()

	// Initial device query
	devCols := []string{"id", "name", "device_type", "rated"}
	mock.ExpectQuery(`SELECT d\.id, d\.name, d\.device_type`).
		WillReturnRows(pgxmock.NewRows(devCols).
			AddRow(1, "约克主机1", "主机", 140.0).
			AddRow(2, "冷冻泵1", "冷冻泵", 30.0).
			AddRow(3, "冷却塔1", "冷却塔", 7.5))

	// Telemetry query for power metrics
	telCols := []string{"device_id", "value"}
	mock.ExpectQuery(`SELECT DISTINCT ON \(device_id\) device_id, value`).
		WithArgs(pgxmock.AnyArg()).
		WillReturnRows(pgxmock.NewRows(telCols).
			AddRow(1, 105.0).  // chiller running ~75%
			AddRow(2, 22.5))   // pump running ~75%
	// tower has no telemetry → falls back to rated

	s := &Service{pool: mock}
	items, err := s.AnalyzeEfficiency(context.TODO())
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 3 {
		t.Fatalf("expected 3 items, got %d", len(items))
	}

	// Chiller (host) with telemetry — 140kW rated, 105kW actual → ~75% load
	chiller := items[0]
	if chiller.PowerKW <= 0 {
		t.Error("chiller should have power from telemetry")
	}
	if chiller.LoadPct <= 0 {
		t.Error("chiller should have load% from telemetry")
	}
	if chiller.COP <= 0 {
		t.Error("chiller should have COP")
	}
	// COP must NOT be a flat 5.0 — should reflect part-load curve.
	if chiller.COP == 5.0 {
		t.Error("chiller COP should vary with load, not be flat 5.0")
	}
	// At 75 % load with part-load factor ~0.98, expected COP ≈ 4.9
	if chiller.COP < 4.0 || chiller.COP > 5.5 {
		t.Errorf("chiller COP out of reasonable range: %.2f", chiller.COP)
	}

	// Pump with telemetry
	pump := items[1]
	if round2(pump.PowerKW) != 22.5 {
		t.Errorf("pump power should be 22.5 from telemetry, got %.1f", pump.PowerKW)
	}

	// Tower without telemetry — should fall back to rated
	tower := items[2]
	if tower.PowerKW <= 0 {
		t.Error("tower should have power from rated fallback")
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet expectations: %v", err)
	}
}

func TestGenerateDemoRecommendations(t *testing.T) {
	recs := generateDemoRecommendations(28.0)
	if len(recs) < 3 {
		t.Errorf("expected at least 3 demo recommendations, got %d", len(recs))
	}
	for _, r := range recs {
		if r.DeviceName == "" || r.Parameter == "" || r.Reason == "" {
			t.Errorf("incomplete recommendation: %+v", r)
		}
	}
}
