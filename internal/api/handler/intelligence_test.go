package handler

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/pashagolub/pgxmock/v4"

	"xmeco/internal/service/intelligence"
)

// =============================================================================
// Tier 4 — H-57~H-62: IntelligenceHandler
// =============================================================================

func TestParseTimeParam(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    time.Time
		wantErr bool
	}{
		{
			name:    "H-57 parseTimeParam RFC3339",
			input:   "2026-06-26T12:00:00Z",
			want:    time.Date(2026, 6, 26, 12, 0, 0, 0, time.UTC),
			wantErr: false,
		},
		{
			name:    "H-58 parseTimeParam 无时区",
			input:   "2026-06-26T12:00:00",
			want:    time.Date(2026, 6, 26, 12, 0, 0, 0, time.UTC),
			wantErr: false,
		},
		{
			name:    "H-59 parseTimeParam 日期格式",
			input:   "2026-06-26",
			want:    time.Date(2026, 6, 26, 0, 0, 0, 0, time.UTC),
			wantErr: false,
		},
		{
			name:    "H-60 parseTimeParam 空字符串",
			input:   "",
			want:    time.Time{},
			wantErr: false,
		},
		{
			name:    "parseTimeParam 非法格式",
			input:   "not-a-date",
			want:    time.Time{},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseTimeParam(tt.input)
			if tt.wantErr && err == nil {
				t.Error("expected error, got nil")
			}
			if !tt.wantErr && err != nil {
				t.Errorf("unexpected error: %v", err)
			}
			if !got.Equal(tt.want) {
				t.Errorf("parseTimeParam(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}

func TestIntelligenceHandler_PowerQuality_MissingDeviceID(t *testing.T) {
	// H-61: PowerQuality 缺少 device_id → 400
	h := NewIntelligenceHandler(nil)
	req := httptest.NewRequest("GET", "/api/v1/intelligence/power-quality", nil)
	rec := httptest.NewRecorder()
	h.PowerQuality(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("H-61: status = %d, want 400", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "缺少 device_id") {
		t.Errorf("H-61: body = %q, want to contain '缺少 device_id'", rec.Body.String())
	}
}

func TestIntelligenceHandler_Forecast(t *testing.T) {
	// H-62: Forecast 正常路径 — weather_cache 查询失败时 GetWeatherTemp
	// 返回默认 25°C (nil error)，所以 handler 的 Warn 分支不会被触发。
	// ForecastLoad 的 DB 查询已通过 intelligence.Service 层测试覆盖。
	// 此处验证 handler 薄层正确代理。
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	// GetWeatherTemp: weather_cache query
	mock.ExpectQuery("SELECT COALESCE\\(temp::numeric").
		WillReturnRows(pgxmock.NewRows([]string{"temp"}).AddRow(28.0))
	// estimateSystemLoad (called by ForecastLoad)
	mock.ExpectQuery("SELECT COALESCE\\(SUM").
		WillReturnRows(pgxmock.NewRows([]string{"total"}).AddRow(500.0))

	svc := intelligence.New(mock)
	h := &IntelligenceHandler{svc: svc}
	req := httptest.NewRequest("GET", "/api/v1/intelligence/forecast", nil)
	rec := httptest.NewRecorder()
	h.Forecast(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("H-62: status = %d, want 200", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), `"load_kw"`) {
		t.Errorf("H-62: body missing forecast data: %s", rec.Body.String())
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet expectations: %v", err)
	}
}

func TestIntelligenceHandler_FullAnalysis(t *testing.T) {
	// FullAnalysis calls RunFullAnalysis which orchestrates 4+ analyses.
	// Each analysis makes independent DB queries. The complex orchestration
	// is tested at the service layer; handler is a thin passthrough.
	// We test that the handler succeeds when given empty DB (demo path).
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	// RunFullAnalysis order: GetWeatherTemp → AnalyzeEfficiency → ForecastLoad → RecommendSetpoints
	// GetWeatherTemp: QueryRow on weather_cache
	mock.ExpectQuery("SELECT COALESCE\\(temp::numeric").
		WillReturnRows(pgxmock.NewRows([]string{"temp"}).AddRow(28.0))
	// AnalyzeEfficiency: device query
	mock.ExpectQuery("SELECT d\\.id, d\\.name, d\\.device_type").
		WillReturnRows(pgxmock.NewRows([]string{"id", "name", "device_type", "prop_value"}))
	// ForecastLoad → estimateSystemLoad
	mock.ExpectQuery("SELECT COALESCE\\(SUM").
		WillReturnRows(pgxmock.NewRows([]string{"total"}).AddRow(500.0))
	// RecommendSetpoints: chiller device query
	mock.ExpectQuery("WHERE d\\.device_type IN").
		WillReturnRows(pgxmock.NewRows([]string{"id", "name", "device_type", "rated_power"}))

	svc := intelligence.New(mock)
	h := &IntelligenceHandler{svc: svc}
	req := httptest.NewRequest("GET", "/api/v1/intelligence/full-analysis", nil)
	rec := httptest.NewRecorder()
	h.FullAnalysis(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), `"efficiencies"`) {
		t.Errorf("body missing expected fields: %s", rec.Body.String())
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet expectations: %v", err)
	}
}
