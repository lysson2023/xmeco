package handler

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/pashagolub/pgxmock/v4"
)

// =============================================================================
// Tier 4 — H-38~H-42: TelemetryHandler
// =============================================================================

func TestTelemetryHandler_Realtime(t *testing.T) {
	teleCols := []string{"ts", "device_id", "metric", "value", "unit"}
	now := time.Now()

	tests := []struct {
		name       string
		queryStr   string
		mockSetup  func(mock pgxmock.PgxPoolIface)
		wantStatus int
		wantBody   string
	}{
		{
			name:     "H-38 Realtime 无device_id返回全部设备",
			queryStr: "",
			mockSetup: func(mock pgxmock.PgxPoolIface) {
				mock.ExpectQuery("SELECT DISTINCT ON \\(device_id, metric\\)").
					WillReturnRows(pgxmock.NewRows(teleCols).
						AddRow(now, 1, "温度", 25.5, "℃").
						AddRow(now, 2, "温度", 26.0, "℃"))
			},
			wantStatus: http.StatusOK,
			wantBody:   "25.5",
		},
		{
			name:     "H-39 Realtime 指定device_id",
			queryStr: "?device_id=5",
			mockSetup: func(mock pgxmock.PgxPoolIface) {
				mock.ExpectQuery("SELECT DISTINCT ON \\(device_id, metric\\).*WHERE device_id").
					WithArgs(5).
					WillReturnRows(pgxmock.NewRows(teleCols).
						AddRow(now, 5, "有功功率", 100.0, "kW"))
			},
			wantStatus: http.StatusOK,
			wantBody:   "有功功率",
		},
		{
			name:     "Realtime 空结果返回空数组",
			queryStr: "",
			mockSetup: func(mock pgxmock.PgxPoolIface) {
				mock.ExpectQuery("SELECT DISTINCT ON \\(device_id, metric\\)").
					WillReturnRows(pgxmock.NewRows(teleCols))
			},
			wantStatus: http.StatusOK,
			wantBody:   "[]",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock, err := pgxmock.NewPool()
			if err != nil {
				t.Fatal(err)
			}
			defer mock.Close()
			tt.mockSetup(mock)

			h := NewTelemetryHandler(mock)
			req := httptest.NewRequest("GET", "/api/v1/telemetry/realtime"+tt.queryStr, nil)
			rec := httptest.NewRecorder()
			h.Realtime(rec, req)

			if rec.Code != tt.wantStatus {
				t.Errorf("status = %d, want %d", rec.Code, tt.wantStatus)
			}
			if tt.wantBody != "" && !strings.Contains(rec.Body.String(), tt.wantBody) {
				t.Errorf("body = %q, want to contain %q", rec.Body.String(), tt.wantBody)
			}
			if err := mock.ExpectationsWereMet(); err != nil {
				t.Errorf("unmet expectations: %v", err)
			}
		})
	}
}

func TestTelemetryHandler_History(t *testing.T) {
	now := time.Now()

	tests := []struct {
		name       string
		queryStr   string
		mockSetup  func(mock pgxmock.PgxPoolIface)
		wantStatus int
	}{
		{
			name:     "H-40 History 默认24h",
			queryStr: "?device_id=5&metric=temp",
			mockSetup: func(mock pgxmock.PgxPoolIface) {
				mock.ExpectQuery("SELECT ts, value FROM device_telemetry WHERE").
					WithArgs(5, "temp", pgxmock.AnyArg()).
					WillReturnRows(pgxmock.NewRows([]string{"ts", "value"}).
						AddRow(now, 25.0).AddRow(now.Add(-1*time.Hour), 24.5))
			},
			wantStatus: http.StatusOK,
		},
		{
			name:     "H-41 History 自定义hours=48",
			queryStr: "?device_id=5&metric=temp&hours=48",
			mockSetup: func(mock pgxmock.PgxPoolIface) {
				mock.ExpectQuery("SELECT ts, value FROM device_telemetry WHERE").
					WithArgs(5, "temp", pgxmock.AnyArg()).
					WillReturnRows(pgxmock.NewRows([]string{"ts", "value"}))
			},
			wantStatus: http.StatusOK,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock, err := pgxmock.NewPool()
			if err != nil {
				t.Fatal(err)
			}
			defer mock.Close()
			tt.mockSetup(mock)

			h := NewTelemetryHandler(mock)
			req := httptest.NewRequest("GET", "/api/v1/telemetry/history"+tt.queryStr, nil)
			rec := httptest.NewRecorder()
			h.History(rec, req)

			if rec.Code != tt.wantStatus {
				t.Errorf("status = %d, want %d", rec.Code, tt.wantStatus)
			}
			if err := mock.ExpectationsWereMet(); err != nil {
				t.Errorf("unmet expectations: %v", err)
			}
		})
	}
}

func TestTelemetryHandler_Stats(t *testing.T) {
	tests := []struct {
		name       string
		queryStr   string
		mockSetup  func(mock pgxmock.PgxPoolIface)
		wantStatus int
		wantBody   string
	}{
		{
			name:     "H-42 Stats system模式(device_id=0)",
			queryStr: "?device_id=0",
			mockSetup: func(mock pgxmock.PgxPoolIface) {
				mock.ExpectQuery("SELECT count\\(\\*\\) FROM device WHERE online_status='在线'").
					WillReturnRows(pgxmock.NewRows([]string{"count"}).AddRow(int64(8)))
				mock.ExpectQuery("SELECT count\\(\\*\\) FROM device WHERE online_status='离线'").
					WillReturnRows(pgxmock.NewRows([]string{"count"}).AddRow(int64(2)))
			},
			wantStatus: http.StatusOK,
			wantBody:   `"online":8`,
		},
		{
			name:     "Stats 单设备统计(device_id>0)",
			queryStr: "?device_id=5",
			mockSetup: func(mock pgxmock.PgxPoolIface) {
				mock.ExpectQuery("SELECT metric.*FROM device_telemetry WHERE device_id").
					WithArgs(5).
					WillReturnRows(pgxmock.NewRows([]string{"metric", "count", "avg", "max", "min"}).
						AddRow("温度", 100, 25.5, 30.0, 20.0))
			},
			wantStatus: http.StatusOK,
			wantBody:   "温度",
		},
		{
			name:     "Stats 在线查询失败降级",
			queryStr: "?device_id=0",
			mockSetup: func(mock pgxmock.PgxPoolIface) {
				mock.ExpectQuery("SELECT count\\(\\*\\) FROM device WHERE online_status='在线'").
					WillReturnError(errors.New("connection refused"))
				mock.ExpectQuery("SELECT count\\(\\*\\) FROM device WHERE online_status='离线'").
					WillReturnRows(pgxmock.NewRows([]string{"count"}).AddRow(int64(0)))
			},
			wantStatus: http.StatusOK,
			wantBody:   `"online":0`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock, err := pgxmock.NewPool()
			if err != nil {
				t.Fatal(err)
			}
			defer mock.Close()
			tt.mockSetup(mock)

			h := NewTelemetryHandler(mock)
			req := httptest.NewRequest("GET", "/api/v1/telemetry/stats"+tt.queryStr, nil)
			rec := httptest.NewRecorder()
			h.Stats(rec, req)

			if rec.Code != tt.wantStatus {
				t.Errorf("status = %d, want %d", rec.Code, tt.wantStatus)
			}
			if tt.wantBody != "" && !strings.Contains(rec.Body.String(), tt.wantBody) {
				t.Errorf("body = %q, want to contain %q", rec.Body.String(), tt.wantBody)
			}
			if err := mock.ExpectationsWereMet(); err != nil {
				t.Errorf("unmet expectations: %v", err)
			}
		})
	}
}
