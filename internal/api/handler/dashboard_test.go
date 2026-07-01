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

func intPtr(v int) *int { return &v }

// screenDataCommonMocks sets up the queries that run after project filtering.
func screenDataCommonMocks(mock pgxmock.PgxPoolIface, pid, bid int) {
	mock.ExpectQuery("SELECT id,name FROM building").
		WithArgs(pid).
		WillReturnRows(pgxmock.NewRows([]string{"id", "name"}).AddRow(1, "A栋").AddRow(2, "B栋"))
	mock.ExpectQuery("SELECT d\\.id, d\\.name, d\\.device_type, d\\.online_status").
		WithArgs(pid, bid).
		WillReturnRows(pgxmock.NewRows([]string{"id", "name", "type", "status", "device_status", "key_info"}).
			AddRow(1, "冷水机组1", "chiller", "在线", "开机", "温度: 25℃"))
	mock.ExpectQuery("SELECT COALESCE\\(c\\.name").
		WithArgs(pid).
		WillReturnRows(pgxmock.NewRows([]string{"name"}).AddRow(""))
	mock.ExpectQuery("SELECT st\\.name,d\\.name,st\\.action_type").
		WithArgs(pid, bid).
		WillReturnRows(pgxmock.NewRows([]string{"name", "device", "action", "time", "enabled", "result"}).
			AddRow("定时开机", "冷水机组1", "startup", "08:00:00", true, "success"))
	mock.ExpectQuery("SELECT COALESCE\\(al\\.device_name").
		WithArgs(pid, bid).
		WillReturnRows(pgxmock.NewRows([]string{"device", "type", "level", "msg", "time"}).
			AddRow("冷水机组1", "阈值告警", "严重", "温度过高", "06-27 10:00"))
	mock.ExpectQuery("SELECT COALESCE\\(save_rate,0\\) FROM building").
		WithArgs(bid).
		WillReturnRows(pgxmock.NewRows([]string{"save_rate"}).AddRow(0.25))
	mock.ExpectQuery("SELECT COALESCE\\(SUM").
		WithArgs(pid, bid).
		WillReturnRows(pgxmock.NewRows([]string{"total"}).AddRow(500.0))
	mock.ExpectQuery("SELECT COALESCE\\(EXTRACT\\(DAY FROM").
		WithArgs(pid).
		WillReturnRows(pgxmock.NewRows([]string{"days"}).AddRow(int64(365)))
	mock.ExpectQuery("SELECT d\\.id").
		WithArgs(pid, bid).
		WillReturnRows(pgxmock.NewRows([]string{"id", "name", "sign", "power"}).AddRow(1, "电表1", 1, 200.0))
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
			name:   "H-43 超管无默认项目无授权→空项目+提示",
			claims: &auth.Claims{UserID: 1, Username: "admin", RoleCode: "super_admin", RoleLevel: intPtr(0)},
			mockSetup: func(mock pgxmock.PgxPoolIface) {
				mock.ExpectQuery("SELECT COALESCE\\(a\\.name").
					WithArgs(1).
					WillReturnRows(pgxmock.NewRows([]string{"name"}).AddRow(""))
				mock.ExpectQuery("SELECT COALESCE\\(default_project_id").
					WithArgs(1).
					WillReturnRows(pgxmock.NewRows([]string{"default_project_id"}).AddRow(int64(0)))
				mock.ExpectQuery("SELECT p\\.id,p\\.name FROM project p JOIN project_user pu").
					WithArgs(1).
					WillReturnRows(pgxmock.NewRows([]string{"id", "name"}))
				screenDataCommonMocks(mock, 0, 1)
			},
			wantStatus: http.StatusOK,
			wantBody:   "您的名下无项目",
		},
		{
			name:   "H-44 超管有默认项目→UNION返回默认+授权项目",
			claims: &auth.Claims{UserID: 1, Username: "admin", RoleCode: "super_admin", RoleLevel: intPtr(0)},
			mockSetup: func(mock pgxmock.PgxPoolIface) {
				mock.ExpectQuery("SELECT COALESCE\\(a\\.name").
					WithArgs(1).
					WillReturnRows(pgxmock.NewRows([]string{"name"}).AddRow(""))
				mock.ExpectQuery("SELECT COALESCE\\(default_project_id").
					WithArgs(1).
					WillReturnRows(pgxmock.NewRows([]string{"default_project_id"}).AddRow(int64(1)))
				mock.ExpectQuery("SELECT \\* FROM \\(").
					WithArgs(1, 1).
					WillReturnRows(pgxmock.NewRows([]string{"id", "name"}).AddRow(1, "项目A").AddRow(2, "项目B"))
				screenDataCommonMocks(mock, 1, 1)
			},
			wantStatus: http.StatusOK,
			wantBody:   "项目B",
		},
		{
			name:   "H-44b 平台管理员有默认项目→UNION返回默认+授权",
			claims: &auth.Claims{UserID: 2, Username: "admin2", RoleCode: "admin", RoleLevel: intPtr(10)},
			mockSetup: func(mock pgxmock.PgxPoolIface) {
				mock.ExpectQuery("SELECT COALESCE\\(a\\.name").
					WithArgs(2).
					WillReturnRows(pgxmock.NewRows([]string{"name"}).AddRow(""))
				mock.ExpectQuery("SELECT COALESCE\\(default_project_id").
					WithArgs(2).
					WillReturnRows(pgxmock.NewRows([]string{"default_project_id"}).AddRow(int64(1)))
				mock.ExpectQuery("SELECT \\* FROM \\(").
					WithArgs(1, 2).
					WillReturnRows(pgxmock.NewRows([]string{"id", "name"}).AddRow(1, "前海港湾酒店").AddRow(3, "南山商场"))
				screenDataCommonMocks(mock, 1, 1)
			},
			wantStatus: http.StatusOK,
			wantBody:   "南山商场",
		},
		{
			name:   "H-45 普通用户有默认项目→UNION返回默认+授权项目",
			claims: &auth.Claims{UserID: 3, Username: "user3", RoleCode: "project_admin", RoleLevel: intPtr(50)},
			mockSetup: func(mock pgxmock.PgxPoolIface) {
				mock.ExpectQuery("SELECT COALESCE\\(a\\.name").
					WithArgs(3).
					WillReturnRows(pgxmock.NewRows([]string{"name"}).AddRow(""))
				mock.ExpectQuery("SELECT COALESCE\\(default_project_id").
					WithArgs(3).
					WillReturnRows(pgxmock.NewRows([]string{"default_project_id"}).AddRow(int64(1)))
				mock.ExpectQuery("SELECT \\* FROM \\(").
					WithArgs(1, 3).
					WillReturnRows(pgxmock.NewRows([]string{"id", "name"}).AddRow(1, "项目A").AddRow(2, "项目B"))
				screenDataCommonMocks(mock, 1, 1)
			},
			wantStatus: http.StatusOK,
			wantBody:   "项目B",
		},
		{
			name:   "H-46 普通用户无默认无授权→空项目+提示",
			claims: &auth.Claims{UserID: 5, Username: "user5", RoleCode: "project_viewer", RoleLevel: intPtr(70)},
			mockSetup: func(mock pgxmock.PgxPoolIface) {
				mock.ExpectQuery("SELECT COALESCE\\(a\\.name").
					WithArgs(5).
					WillReturnRows(pgxmock.NewRows([]string{"name"}).AddRow(""))
				mock.ExpectQuery("SELECT COALESCE\\(default_project_id").
					WithArgs(5).
					WillReturnRows(pgxmock.NewRows([]string{"default_project_id"}).AddRow(int64(0)))
				mock.ExpectQuery("SELECT p\\.id,p\\.name FROM project p JOIN project_user pu").
					WithArgs(5).
					WillReturnRows(pgxmock.NewRows([]string{"id", "name"}))
				screenDataCommonMocks(mock, 0, 1)
			},
			wantStatus: http.StatusOK,
			wantBody:   "您的名下无项目",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock, err := pgxmock.NewPool()
			mock.MatchExpectationsInOrder(false)
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
	mock, err := pgxmock.NewPool()
	mock.MatchExpectationsInOrder(false)
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	claims := &auth.Claims{UserID: 1, Username: "admin", RoleCode: "super_admin", RoleLevel: intPtr(0)}
	pid := 1

	mock.ExpectQuery("SELECT COALESCE\\(a\\.name").
		WithArgs(1).
		WillReturnRows(pgxmock.NewRows([]string{"name"}).AddRow(""))
	mock.ExpectQuery("SELECT COALESCE\\(default_project_id").
		WithArgs(1).
		WillReturnRows(pgxmock.NewRows([]string{"default_project_id"}).AddRow(int64(0)))
	mock.ExpectQuery("SELECT p\\.id,p\\.name FROM project p JOIN project_user pu").
		WithArgs(1).
		WillReturnRows(pgxmock.NewRows([]string{"id", "name"}).AddRow(1, "项目A"))
	mock.ExpectQuery("SELECT id,name FROM building").
		WithArgs(pid).
		WillReturnError(errors.New("connection refused"))
	mock.ExpectQuery("SELECT d\\.id, d\\.name, d\\.device_type, d\\.online_status").
		WithArgs(pid, 0).
		WillReturnRows(pgxmock.NewRows([]string{"id", "name", "type", "status", "device_status", "key_info"}))
	mock.ExpectQuery("SELECT COALESCE\\(c\\.name").
		WithArgs(pid).
		WillReturnRows(pgxmock.NewRows([]string{"name"}).AddRow(""))
	mock.ExpectQuery("SELECT st\\.name,d\\.name,st\\.action_type").
		WithArgs(pid, 0).
		WillReturnRows(pgxmock.NewRows([]string{"name", "device", "action", "time", "enabled", "result"}))
	mock.ExpectQuery("SELECT COALESCE\\(al\\.device_name").
		WithArgs(pid, 0).
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
	mock.ExpectQuery("SELECT d\\.id").
		WithArgs(pid, 0).
		WillReturnRows(pgxmock.NewRows([]string{"id", "name", "sign", "power"}))

	h := NewDashboardHandler(mock, nil)
	req := httptest.NewRequest("GET", "/api/v1/dashboard/screen-data", nil)
	ctx := context.WithValue(t.Context(), middleware.CtxClaims, claims)
	req = req.WithContext(ctx)
	rec := httptest.NewRecorder()
	h.ScreenData(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected 200 (degraded), got %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), `"buildings":[]`) {
		t.Errorf("expected empty buildings array: %s", rec.Body.String())
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
		WillReturnRows(pgxmock.NewRows([]string{"key", "value"}).AddRow("title", "XMECO").AddRow("logo", "/logo.png"))
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