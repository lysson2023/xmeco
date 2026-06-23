package intelligence

import (
	"context"
	"testing"
)

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
	s := &Service{} // nil pool, forecast doesn't use DB

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
