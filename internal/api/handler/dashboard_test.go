package handler

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/pashagolub/pgxmock/v4"

	"xmeco/internal/api/middleware"
	"xmeco/internal/service/auth"
)

// =============================================================================
// Tier 4 — H-43~H-46: DashboardHandler ScreenData
// =============================================================================

// screenDataCommonMocks sets up the queries that run after project filtering.
// pid and bid are what the handler computes after iterating projects/buildings.
func screenDataCommonMocks(mock pgxmock.PgxPoolIface, pid, bid int) {
	// Buildings
	mock.ExpectQuery("SELECT id,name FROM building").
		WithArgs(pid).
		WillReturnRows(pgxmock.NewRows([]string{"id", "name"}).
			AddRow(1, "A栋").AddRow(2, "B栋"))
	// Devices — uses (pid, bid) both
	mock.ExpectQuery("SELECT d\\.id, d\\.name, d\\.device_type, d\\.online_status").
		WithArgs(pid, bid).
		WillReturnRows(pgxmock.NewRows([]string{"id", "name", "type", "status", "device_status", "key_info"}).
			AddRow(1, "冷水机组1", "chiller", "在线", "开机", "温度: 25℃ | 功率: 100kW"))
	// City name → empty (skip weather)
	mock.ExpectQuery("SELECT COALESCE\\(c\\.name").
		WithArgs(pid).
		WillReturnRows(pgxmock.NewRows([]string{"name"}).AddRow(""))
	// Scheduled tasks — uses (pid, bid) both
	mock.ExpectQuery("SELECT st\\.name,d\\.name,st\\.action_type").
		WithArgs(pid, bid).
		WillReturnRows(pgxmock.NewRows([]string{"name", "device", "action", "time", "enabled", "result"}).
			AddRow("定时开机", "冷水机组1", "startup", "08:00:00", true, "success"))
	// Alarms
	mock.ExpectQuery("SELECT COALESCE\\(device_name").
		WillReturnRows(pgxmock.NewRows([]string{"device", "type", "level", "msg", "time"}).
			AddRow("冷水机组1", "阈值告警", "严重", "温度过高", "06-27 10:00"))
	// Saving rate — uses bid
	mock.ExpectQuery("SELECT COALESCE\\(save_rate,0\\) FROM building").
		WithArgs(bid).
		WillReturnRows(pgxmock.NewRows([]string{"save_rate"}).AddRow(0.25))
	// Meter power — uses (pid, bid) both
	mock.ExpectQuery("SELECT COALESCE\\(SUM").
		WithArgs(pid, bid).
		WillReturnRows(pgxmock.NewRows([]string{"total"}).AddRow(500.0))
	// Running days — uses pid
	mock.ExpectQuery("SELECT COALESCE\\(EXTRACT\\(DAY FROM").
		WithArgs(pid).
		WillReturnRows(pgxmock.NewRows([]string{"days"}).AddRow(int64(365)))
	// Individual meters — uses (pid, bid) both
	mock.ExpectQuery("SELECT d\\.id,d\\.name,COALESCE\\(d\\.power_sign").
		WithArgs(pid, bid).
		WillReturnRows(pgxmock.NewRows([]string{"id", "name", "sign", "power"}).
			AddRow(1, "电表1", 1, 200.0))
}

func TestDashboardHandler_ScreenData(t *testing.T) {
	tests := []struct {
		name       string
		claims     *auth.Claims
		mockSetup  func(mock pgxmock.PgxPoolIface)
		wantStatus int
		wantBody   string
	}{
		{
			name:   "H-43 ScreenData 超级管理员看到所有项目",
			claims: &auth.Claims{UserID: 1, Username: "admin", RoleCode: "super_admin"},
			mockSetup: func(mock pgxmock.PgxPoolIface) {
				// 1. Agent name query (runs first, after claims)
				mock.ExpectQuery("SELECT COALESCE\\(a\\.name").
					WithArgs(1).
					WillReturnRows(pgxmock.NewRows([]string{"name"}).AddRow(""))
				// 2. project_user count → 0
				mock.ExpectQuery("SELECT count\\(\\*\\) FROM project_user").
					WithArgs(1).
					WillReturnRows(pgxmock.NewRows([]string{"count"}).AddRow(int64(0)))
				// 3. role check → super_admin
				mock.ExpectQuery("SELECT r\\.code FROM users u JOIN role r").
					WithArgs(1).
					WillReturnRows(pgxmock.NewRows([]string{"code"}).AddRow("super_admin"))
				// 4. All projects
				mock.ExpectQuery("SELECT id,name FROM project ORDER BY").
					WillReturnRows(pgxmock.NewRows([]string{"id", "name"}).
						AddRow(1, "项目A").AddRow(2, "项目B"))
				screenDataCommonMocks(mock, 1, 1)
			},
			wantStatus: http.StatusOK,
			wantBody:   "项目A",
		},
		{
			name:   "H-44 ScreenData 普通用户看到已分配项目",
			claims: &auth.Claims{UserID: 5, Username: "user5", RoleCode: "admin"},
			mockSetup: func(mock pgxmock.PgxPoolIface) {
				// 1. Agent name query
				mock.ExpectQuery("SELECT COALESCE\\(a\\.name").
					WithArgs(5).
					WillReturnRows(pgxmock.NewRows([]string{"name"}).AddRow("代理商Y"))
				// 2. project_user count → 2 (has assignments)
				mock.ExpectQuery("SELECT count\\(\\*\\) FROM project_user").
					WithArgs(5).
					WillReturnRows(pgxmock.NewRows([]string{"count"}).AddRow(int64(2)))
				// 3. Only assigned projects
				mock.ExpectQuery("SELECT p\\.id,p\\.name FROM project p JOIN project_user").
					WithArgs(5).
					WillReturnRows(pgxmock.NewRows([]string{"id", "name"}).
						AddRow(3, "分配项目X"))
				screenDataCommonMocks(mock, 3, 1)
			},
			wantStatus: http.StatusOK,
			wantBody:   "分配项目X",
		},
		{
			name:   "H-45 ScreenData 普通用户未分配→空项目列表",
			claims: &auth.Claims{UserID: 5, Username: "user5", RoleCode: "admin"},
			mockSetup: func(mock pgxmock.PgxPoolIface) {
				// 1. Agent name query
				mock.ExpectQuery("SELECT COALESCE\\(a\\.name").
					WithArgs(5).
					WillReturnRows(pgxmock.NewRows([]string{"name"}).AddRow(""))
				// 2. project_user count → 0
				mock.ExpectQuery("SELECT count\\(\\*\\) FROM project_user").
					WithArgs(5).
					WillReturnRows(pgxmock.NewRows([]string{"count"}).AddRow(int64(0)))
				// 3. role check → admin (not super_admin)
				mock.ExpectQuery("SELECT r\\.code FROM users u JOIN role r").
					WithArgs(5).
					WillReturnRows(pgxmock.NewRows([]string{"code"}).AddRow("admin"))
				// 4. Empty projects (WHERE 1=0)
				mock.ExpectQuery("SELECT id,name FROM project WHERE 1=0").
					WillReturnRows(pgxmock.NewRows([]string{"id", "name"}))
				// No pid set → pid stays 0
				screenDataCommonMocks(mock, 0, 1)
			},
			wantStatus: http.StatusOK,
			wantBody:   `"projects":[]`,
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

			h := NewDashboardHandler(mock, nil)
			req := httptest.NewRequest("GET", "/api/v1/dashboard/screen-data", nil)
			ctx := context.WithValue(t.Context(), middleware.CtxClaims, tt.claims)
			req = req.WithContext(ctx)
			rec := httptest.NewRecorder()
			h.ScreenData(rec, req)

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

func TestDashboardHandler_ScreenData_BuildingQueryFail(t *testing.T) {
	// H-46: building 查询失败 → Warn + 空 buildings，不 500
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	claims := &auth.Claims{UserID: 1, Username: "admin", RoleCode: "super_admin"}
	pid := 1

	// 1. Agent name
	mock.ExpectQuery("SELECT COALESCE\\(a\\.name").
		WithArgs(1).
		WillReturnRows(pgxmock.NewRows([]string{"name"}).AddRow(""))
	// 2. project_user count
	mock.ExpectQuery("SELECT count\\(\\*\\) FROM project_user").
		WithArgs(1).
		WillReturnRows(pgxmock.NewRows([]string{"count"}).AddRow(int64(0)))
	// 3. role check → super_admin
	mock.ExpectQuery("SELECT r\\.code FROM users u JOIN role r").
		WithArgs(1).
		WillReturnRows(pgxmock.NewRows([]string{"code"}).AddRow("super_admin"))
	// 4. All projects
	mock.ExpectQuery("SELECT id,name FROM project ORDER BY").
		WillReturnRows(pgxmock.NewRows([]string{"id", "name"}).
			AddRow(1, "项目A"))
	// 5. Buildings query FAILS
	mock.ExpectQuery("SELECT id,name FROM building").
		WithArgs(pid).
		WillReturnError(errors.New("connection refused"))
	// Remaining queries still fire
	mock.ExpectQuery("SELECT d\\.id, d\\.name, d\\.device_type, d\\.online_status").
		WithArgs(pid, 0).
		WillReturnRows(pgxmock.NewRows([]string{"id", "name", "type", "status", "device_status", "key_info"}))
	mock.ExpectQuery("SELECT COALESCE\\(c\\.name").
		WithArgs(pid).
		WillReturnRows(pgxmock.NewRows([]string{"name"}).AddRow(""))
	mock.ExpectQuery("SELECT st\\.name,d\\.name,st\\.action_type").
		WithArgs(pid, 0).
		WillReturnRows(pgxmock.NewRows([]string{"name", "device", "action", "time", "enabled", "result"}))
	mock.ExpectQuery("SELECT COALESCE\\(device_name").
		WillReturnRows(pgxmock.NewRows([]string{"device", "type", "level", "msg", "time"}))
	mock.ExpectQuery("SELECT COALESCE\\(save_rate,0\\) FROM building").
		WithArgs(0).
		WillReturnRows(pgxmock.NewRows([]string{"save_rate"}).AddRow(0.0))
	mock.ExpectQuery("SELECT COALESCE\\(SUM").
		WithArgs(pid, 0).
		WillReturnRows(pgxmock.NewRows([]string{"total"}).AddRow(0.0))
	mock.ExpectQuery("SELECT COALESCE\\(EXTRACT\\(DAY FROM").
		WithArgs(pid).
		WillReturnRows(pgxmock.NewRows([]string{"days"}).AddRow(int64(0)))
	mock.ExpectQuery("SELECT d\\.id,d\\.name,COALESCE\\(d\\.power_sign").
		WithArgs(pid, 0).
		WillReturnRows(pgxmock.NewRows([]string{"id", "name", "sign", "power"}))

	h := NewDashboardHandler(mock, nil)
	req := httptest.NewRequest("GET", "/api/v1/dashboard/screen-data", nil)
	ctx := context.WithValue(t.Context(), middleware.CtxClaims, claims)
	req = req.WithContext(ctx)
	rec := httptest.NewRecorder()
	h.ScreenData(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("H-46: expected 200 (degraded), got %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), `"buildings":[]`) {
		t.Errorf("H-46: expected empty buildings array: %s", rec.Body.String())
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet expectations: %v", err)
	}
}

func TestDashboardHandler_GetConfig(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	mock.ExpectQuery("SELECT key,value FROM dashboard_config").
		WillReturnRows(pgxmock.NewRows([]string{"key", "value"}).
			AddRow("title", "XMECO").AddRow("logo", "/logo.png"))
	mock.ExpectQuery("SELECT count\\(\\*\\) FROM device WHERE online_status='在线'").
		WillReturnRows(pgxmock.NewRows([]string{"count"}).AddRow(int64(10)))
	mock.ExpectQuery("SELECT count\\(\\*\\) FROM alarm_log WHERE created_at::date=CURRENT_DATE").
		WillReturnRows(pgxmock.NewRows([]string{"count"}).AddRow(int64(3)))
	mock.ExpectQuery("SELECT COALESCE\\(EXTRACT\\(DAY FROM").
		WillReturnRows(pgxmock.NewRows([]string{"days"}).AddRow(int64(100)))

	h := NewDashboardHandler(mock, nil)
	req := httptest.NewRequest("GET", "/api/v1/dashboard/config", nil)
	rec := httptest.NewRecorder()
	h.GetConfig(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), `"title":"XMECO"`) {
		t.Errorf("body missing title: %s", rec.Body.String())
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet expectations: %v", err)
	}
}
