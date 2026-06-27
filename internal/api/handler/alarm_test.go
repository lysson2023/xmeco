package handler

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/pashagolub/pgxmock/v4"

	"xmeco/internal/api/middleware"
	"xmeco/internal/service/auth"
)

// =============================================================================
// Tier 4 — H-33~H-37: AlarmHandler
// =============================================================================

func TestAlarmHandler_CreateRule(t *testing.T) {
	tests := []struct {
		name       string
		body       string
		mockSetup  func(mock pgxmock.PgxPoolIface)
		wantStatus int
		wantBody   string
	}{
		{
			name:       "H-33 CreateRule 缺少名称→400",
			body:       `{"name":""}`,
			mockSetup:  nil,
			wantStatus: http.StatusBadRequest,
			wantBody:   "规则名称不能为空",
		},
		{
			name: "H-34 CreateRule 默认 Enabled=true",
			body: `{"name":"测试规则","device_type":"chiller","metric":"temp","condition":">","threshold":30}`,
			mockSetup: func(mock pgxmock.PgxPoolIface) {
				mock.ExpectQuery("INSERT INTO alarm_rule").
					WithArgs("测试规则", pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(),
						pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(),
						pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(), true).
					WillReturnRows(pgxmock.NewRows([]string{"id"}).AddRow(1))
			},
			wantStatus: http.StatusOK,
			wantBody:   `"id":1`,
		},
		{
			name: "CreateRule 显式 Enabled=false",
			body: `{"name":"禁用规则","enabled":false}`,
			mockSetup: func(mock pgxmock.PgxPoolIface) {
				mock.ExpectQuery("INSERT INTO alarm_rule").
					WithArgs("禁用规则", pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(),
						pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(),
						pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(), false).
					WillReturnRows(pgxmock.NewRows([]string{"id"}).AddRow(2))
			},
			wantStatus: http.StatusOK,
			wantBody:   `"id":2`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var h *AlarmHandler
			var mock pgxmock.PgxPoolIface

			if tt.mockSetup != nil {
				var err error
				mock, err = pgxmock.NewPool()
				if err != nil {
					t.Fatal(err)
				}
				defer mock.Close()
				tt.mockSetup(mock)
				h = NewAlarmHandler(mock)
			} else {
				h = NewAlarmHandler(nil)
			}

			req := httptest.NewRequest("POST", "/api/v1/alarm-rules", strings.NewReader(tt.body))
			req.Header.Set("Content-Type", "application/json")
			rec := httptest.NewRecorder()

			if tt.mockSetup == nil {
				func() { defer func() { recover() }(); h.CreateRule(rec, req) }()
			} else {
				h.CreateRule(rec, req)
			}

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

func TestAlarmHandler_ListLogs(t *testing.T) {
	alarmLogCols := []string{"id", "device_id", "device_name", "alarm_type", "level",
		"message", "value", "threshold", "created_at", "ack_at"}

	tests := []struct {
		name       string
		queryStr   string
		mockSetup  func(mock pgxmock.PgxPoolIface)
		wantStatus int
	}{
		{
			name:     "H-35 ListLogs today过滤",
			queryStr: "?today=1",
			mockSetup: func(mock pgxmock.PgxPoolIface) {
				mock.ExpectQuery("SELECT al\\.id").
					WillReturnRows(pgxmock.NewRows(alarmLogCols).
						AddRow(1, 1, "冷水机1", "阈值告警", "严重", "温度过高", "35", "30", "2026-06-27 10:00:00", ""))
			},
			wantStatus: http.StatusOK,
		},
		{
			name:     "H-36 ListLogs date范围过滤",
			queryStr: "?date_from=2026-01-01&date_to=2026-01-31",
			mockSetup: func(mock pgxmock.PgxPoolIface) {
				mock.ExpectQuery("SELECT al\\.id").
					WithArgs("2026-01-01", "2026-01-31").
					WillReturnRows(pgxmock.NewRows(alarmLogCols))
			},
			wantStatus: http.StatusOK,
		},
		{
			name:     "ListLogs 含device_id过滤",
			queryStr: "?device_id=5",
			mockSetup: func(mock pgxmock.PgxPoolIface) {
				mock.ExpectQuery("SELECT al\\.id").
					WithArgs(5).
					WillReturnRows(pgxmock.NewRows(alarmLogCols).
						AddRow(2, 5, "设备A", "离线告警", "警告", "设备离线", "", "", "2026-06-27 09:00:00", ""))
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

			h := NewAlarmHandler(mock)
			req := httptest.NewRequest("GET", "/api/v1/alarm-logs"+tt.queryStr, nil)
			rec := httptest.NewRecorder()
			h.ListLogs(rec, req)

			if rec.Code != tt.wantStatus {
				t.Errorf("status = %d, want %d", rec.Code, tt.wantStatus)
			}
			if err := mock.ExpectationsWereMet(); err != nil {
				t.Errorf("unmet expectations: %v", err)
			}
		})
	}
}

func TestAlarmHandler_AckLog(t *testing.T) {
	tests := []struct {
		name       string
		claims     *auth.Claims
		mockSetup  func(mock pgxmock.PgxPoolIface)
		wantStatus int
		wantBody   string
	}{
		{
			name:   "H-37 AckLog 正常确认告警",
			claims: &auth.Claims{UserID: 1, Username: "operator1", RoleCode: "admin"},
			mockSetup: func(mock pgxmock.PgxPoolIface) {
				mock.ExpectExec("UPDATE alarm_log SET ack_by").
					WithArgs("operator1", 42).
					WillReturnResult(pgxmock.NewResult("UPDATE", 1))
			},
			wantStatus: http.StatusOK,
			wantBody:   `"status":"acked"`,
		},
		{
			name:       "AckLog 未认证→401",
			claims:     nil,
			mockSetup:  nil,
			wantStatus: http.StatusUnauthorized,
			wantBody:   "未认证",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var h *AlarmHandler
			var mock pgxmock.PgxPoolIface

			if tt.mockSetup != nil {
				var err error
				mock, err = pgxmock.NewPool()
				if err != nil {
					t.Fatal(err)
				}
				defer mock.Close()
				tt.mockSetup(mock)
				h = NewAlarmHandler(mock)
			} else {
				h = NewAlarmHandler(nil)
			}

			req := httptest.NewRequest("POST", "/api/v1/alarm-logs/42/ack", nil)
			req.SetPathValue("id", "42")
			if tt.claims != nil {
				ctx := context.WithValue(t.Context(), middleware.CtxClaims, tt.claims)
				req = req.WithContext(ctx)
			}
			rec := httptest.NewRecorder()
			h.AckLog(rec, req)

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
