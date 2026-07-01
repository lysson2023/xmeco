package handler

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/pashagolub/pgxmock/v4"

	"xmeco/internal/repository/postgres"
)

// =============================================================================
// Tier 4 — H-47~H-51: LogHandler Interval 注入安全
// =============================================================================

func TestLogHandler_Telemetry_IntervalWhitelist(t *testing.T) {
	now := time.Now()

	tests := []struct {
		name       string
		queryStr   string
		mockSetup  func(mock pgxmock.PgxPoolIface)
		wantStatus int
		wantBody   string
	}{
		{
			name:     "H-47 interval=minute 通过白名单",
			queryStr: "?interval=minute",
			mockSetup: func(mock pgxmock.PgxPoolIface) {
				mock.ExpectQuery("SELECT date_trunc").
					WithArgs("minute", pgxmock.AnyArg(), pgxmock.AnyArg()).
					WillReturnRows(pgxmock.NewRows([]string{"ts", "metric", "avg_val", "max_val", "min_val", "cnt"}))
			},
			wantStatus: http.StatusOK,
		},
		{
			name:     "H-48 interval=hour 通过白名单",
			queryStr: "?interval=hour",
			mockSetup: func(mock pgxmock.PgxPoolIface) {
				mock.ExpectQuery("SELECT date_trunc").
					WithArgs("hour", pgxmock.AnyArg(), pgxmock.AnyArg()).
					WillReturnRows(pgxmock.NewRows([]string{"ts", "metric", "avg_val", "max_val", "min_val", "cnt"}))
			},
			wantStatus: http.StatusOK,
		},
		{
			name:     "H-49 interval=day 通过白名单",
			queryStr: "?interval=day",
			mockSetup: func(mock pgxmock.PgxPoolIface) {
				mock.ExpectQuery("SELECT date_trunc").
					WithArgs("day", pgxmock.AnyArg(), pgxmock.AnyArg()).
					WillReturnRows(pgxmock.NewRows([]string{"ts", "metric", "avg_val", "max_val", "min_val", "cnt"}))
			},
			wantStatus: http.StatusOK,
		},
		{
			name:     "H-50 interval=raw 无聚合查询",
			queryStr: "?interval=raw",
			mockSetup: func(mock pgxmock.PgxPoolIface) {
				mock.ExpectQuery("SELECT ts, metric, value, unit FROM device_telemetry").
					WithArgs(pgxmock.AnyArg(), pgxmock.AnyArg()).
					WillReturnRows(pgxmock.NewRows([]string{"ts", "metric", "value", "unit"}).
						AddRow(now, "温度", 25.0, "℃"))
			},
			wantStatus: http.StatusOK,
			wantBody:   "温度",
		},
		{
			name:       "H-51 interval=DROP TABLE 白名单拦截",
			queryStr:   "?interval=DROP%20TABLE",
			mockSetup:  nil,
			wantStatus: http.StatusBadRequest,
			wantBody:   "无效的时间间隔",
		},
		{
			name:       "H-51 interval=1;DROP TABLE users;-- 白名单拦截",
			queryStr:   "?interval=1%3B+DROP+TABLE+users%3B--",
			mockSetup:  nil,
			wantStatus: http.StatusBadRequest,
			wantBody:   "无效的时间间隔",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var h *LogHandler
			var mock pgxmock.PgxPoolIface

			if tt.mockSetup != nil {
				var err error
				mock, err = pgxmock.NewPool()
				if err != nil {
					t.Fatal(err)
				}
				defer mock.Close()
				tt.mockSetup(mock)
				h = NewLogHandler(postgres.NewLogRepo(mock))
			} else {
				h = NewLogHandler(nil)
			}

			req := httptest.NewRequest("GET", "/api/v1/logs/telemetry"+tt.queryStr, nil)
			rec := httptest.NewRecorder()
			h.Telemetry(rec, req)

			if rec.Code != tt.wantStatus {
				t.Errorf("status = %d, want %d", rec.Code, tt.wantStatus)
			}
			if tt.wantBody != "" && !strings.Contains(rec.Body.String(), tt.wantBody) {
				t.Errorf("body = %q, want to contain %q", rec.Body.String(), tt.wantBody)
			}
			if mock != nil {
				if err := mock.ExpectationsWereMet(); err != nil {
					t.Errorf("unmet expectations: %v", err)
				}
			}
		})
	}
}
