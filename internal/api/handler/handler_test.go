package handler

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/pashagolub/pgxmock/v4"

	"xmeco/internal/api/middleware"
	"xmeco/internal/repository/postgres"
	"xmeco/internal/service/auth"
)

func TestMParse(t *testing.T) {
	// M is just map[string]any — verify usage
	m := M{"key": "value", "count": 42}
	if m["key"] != "value" {
		t.Errorf("M key = %v", m["key"])
	}
	if m["count"] != 42 {
		t.Errorf("M count = %v", m["count"])
	}
}

func TestPathID(t *testing.T) {
	tests := []struct {
		name      string
		pathValue string
		want      int
	}{
		{"numeric", "5", 5},
		{"zero", "0", 0},
		{"empty", "", 0},
		{"non-numeric", "abc", 0},
		{"large number", "12345", 12345},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := httptest.NewRequest("GET", "/", nil)
			if tt.pathValue != "" {
				r.SetPathValue("id", tt.pathValue)
			}
			got := pathID(r)
			if got != tt.want {
				t.Errorf("pathID() = %d, want %d", got, tt.want)
			}
		})
	}
}

// ---- ok / created / notFound / serverErr ----

func TestOk(t *testing.T) {
	rec := httptest.NewRecorder()
	ok(rec, M{"status": "ok"})
	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", rec.Code)
	}
}

func TestCreated(t *testing.T) {
	rec := httptest.NewRecorder()
	created(rec, M{"id": 1})
	if rec.Code != http.StatusCreated {
		t.Errorf("status = %d, want 201", rec.Code)
	}
}

func TestNotFound(t *testing.T) {
	rec := httptest.NewRecorder()
	notFound(rec, "楼宇不存在")
	if rec.Code != http.StatusNotFound {
		t.Errorf("status = %d, want 404", rec.Code)
	}
	if s := rec.Body.String(); s == "" {
		t.Error("body should not be empty")
	}
}

func TestServerErr(t *testing.T) {
	rec := httptest.NewRecorder()
	serverErr(rec, errors.New("database connection failed"))
	if rec.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want 500", rec.Code)
	}
	ct := rec.Header().Get("Content-Type")
	if ct != "application/json" {
		t.Errorf("Content-Type = %q, want application/json", ct)
	}
}

// =============================================================================
// Tier 4 — H-12~H-13: controlActionCN 动作中文翻译
// =============================================================================

func TestControlActionCN(t *testing.T) {
	tests := []struct {
		name              string
		action            string
		wantControlVal    string
		wantDeviceStatus  string
	}{
		{
			name:             "H-12 start→开机",
			action:           "start",
			wantControlVal:   "开机",
			wantDeviceStatus: "开机",
		},
		{
			name:             "H-12 startup→开机",
			action:           "startup",
			wantControlVal:   "开机",
			wantDeviceStatus: "开机",
		},
		{
			name:             "H-12 stop→关机",
			action:           "stop",
			wantControlVal:   "关机",
			wantDeviceStatus: "关机",
		},
		{
			name:             "H-12 shutdown→关机",
			action:           "shutdown",
			wantControlVal:   "关机",
			wantDeviceStatus: "关机",
		},
		{
			name:             "H-12 restart→重启/开机",
			action:           "restart",
			wantControlVal:   "重启",
			wantDeviceStatus: "开机",
		},
		{
			name:             "H-12 pause→暂停",
			action:           "pause",
			wantControlVal:   "暂停",
			wantDeviceStatus: "暂停",
		},
		{
			name:             "H-12 resume→恢复/开机",
			action:           "resume",
			wantControlVal:   "恢复",
			wantDeviceStatus: "开机",
		},
		{
			name:             "H-13 未知动作原样返回",
			action:           "unknown_action",
			wantControlVal:   "unknown_action",
			wantDeviceStatus: "",
		},
		{
			name:             "H-13 空字符串原样返回",
			action:           "",
			wantControlVal:   "",
			wantDeviceStatus: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotControlVal, gotDeviceStatus := controlActionCN(tt.action)
			if gotControlVal != tt.wantControlVal {
				t.Errorf("controlActionCN(%q) controlVal = %q, want %q", tt.action, gotControlVal, tt.wantControlVal)
			}
			if gotDeviceStatus != tt.wantDeviceStatus {
				t.Errorf("controlActionCN(%q) deviceStatus = %q, want %q", tt.action, gotDeviceStatus, tt.wantDeviceStatus)
			}
		})
	}
}

func TestQueryIntMultipleKeys(t *testing.T) {
	req := httptest.NewRequest("GET", "/test?id=10&device_id=5&metric=A", nil)
	if queryInt(req, "device_id") != 5 {
		t.Error("queryInt('device_id') should return 5")
	}
	if queryInt(req, "metric") != 0 {
		t.Error("queryInt('metric') should return 0 for non-numeric")
	}
	if queryInt(req, "absent") != 0 {
		t.Error("queryInt('absent') should return 0")
	}
}

// =============================================================================
// Tier 4 — H-29~H-32: ProjectHandler CRUD
// =============================================================================

func projectColumns() []string {
	return []string{"id", "name", "agent_id", "address", "admin_code", "city_id", "city_name", "created_at"}
}

func TestProjectHandler_Get(t *testing.T) {
	now := jsonNullTime()
	tests := []struct {
		name       string
		id         int
		mockSetup  func(mock pgxmock.PgxPoolIface)
		wantStatus int
		wantBody   string
	}{
		{
			name: "H-29 Get 正常返回项目",
			id:   1,
			mockSetup: func(mock pgxmock.PgxPoolIface) {
				mock.ExpectQuery("SELECT .+ FROM project WHERE").
					WithArgs(1).
					WillReturnRows(pgxmock.NewRows(projectColumns()).
						AddRow(1, "测试项目", nil, "北京市朝阳区", "ADM001", nil, "", now))
			},
			wantStatus: http.StatusOK,
			wantBody:   "测试项目",
		},
		{
			name: "H-30 Get 项目不存在",
			id:   999,
			mockSetup: func(mock pgxmock.PgxPoolIface) {
				mock.ExpectQuery("SELECT .+ FROM project WHERE").
					WithArgs(999).
					WillReturnRows(pgxmock.NewRows(projectColumns()))
			},
			wantStatus: http.StatusNotFound,
			wantBody:   "项目不存在",
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

			repo := postgres.NewProjectRepo(mock)
			h := NewProjectHandler(repo)

			req := httptest.NewRequest("GET", "/api/v1/projects/"+itos(tt.id), nil)
			req.SetPathValue("id", itos(tt.id))
			rec := httptest.NewRecorder()
			h.Get(rec, req)

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

func TestProjectHandler_List(t *testing.T) {
	now := jsonNullTime()
	tests := []struct {
		name       string
		mockSetup  func(mock pgxmock.PgxPoolIface)
		wantStatus int
		wantBody   string
	}{
		{
			name: "List 正常返回项目列表",
			mockSetup: func(mock pgxmock.PgxPoolIface) {
				mock.ExpectQuery("SELECT .+ FROM project ORDER BY").
					WillReturnRows(pgxmock.NewRows(projectColumns()).
						AddRow(1, "项目A", nil, "地址A", "C1", nil, "", now).
						AddRow(2, "项目B", nil, "地址B", "C2", nil, "", now))
			},
			wantStatus: http.StatusOK,
			wantBody:   "项目A",
		},
		{
			name: "H-31 List 空结果返回空数组",
			mockSetup: func(mock pgxmock.PgxPoolIface) {
				mock.ExpectQuery("SELECT .+ FROM project ORDER BY").
					WillReturnRows(pgxmock.NewRows(projectColumns()))
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

			repo := postgres.NewProjectRepo(mock)
			h := NewProjectHandler(repo)

			req := httptest.NewRequest("GET", "/api/v1/projects", nil)
			rec := httptest.NewRecorder()
			h.List(rec, req)

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

func TestProjectHandler_Create(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	mock.ExpectQuery("INSERT INTO project").
		WithArgs(pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg()).
		WillReturnRows(pgxmock.NewRows([]string{"id", "created_at"}).AddRow(3, jsonNullTime()))

	repo := postgres.NewProjectRepo(mock)
	h := NewProjectHandler(repo)

	body := `{"name":"新项目","address":"测试地址"}`
	req := httptest.NewRequest("POST", "/api/v1/projects", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	h.Create(rec, req)

	if rec.Code != http.StatusCreated {
		t.Errorf("status = %d, want 201", rec.Code)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet expectations: %v", err)
	}
}

func TestProjectHandler_Update(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	mock.ExpectExec("UPDATE project SET").
		WithArgs(pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg()).
		WillReturnResult(pgxmock.NewResult("UPDATE", 1))

	repo := postgres.NewProjectRepo(mock)
	h := NewProjectHandler(repo)

	body := `{"name":"更新项目","address":"新地址"}`
	req := httptest.NewRequest("PUT", "/api/v1/projects/1", strings.NewReader(body))
	req.SetPathValue("id", "1")
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	h.Update(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", rec.Code)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet expectations: %v", err)
	}
}

func TestProjectHandler_Delete(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	mock.ExpectExec("DELETE FROM project WHERE").
		WithArgs(1).
		WillReturnResult(pgxmock.NewResult("DELETE", 1))

	repo := postgres.NewProjectRepo(mock)
	h := NewProjectHandler(repo)

	req := httptest.NewRequest("DELETE", "/api/v1/projects/1", nil)
	req.SetPathValue("id", "1")
	rec := httptest.NewRecorder()
	h.Delete(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", rec.Code)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet expectations: %v", err)
	}
}

func TestProjectHandler_SetProjectUsers(t *testing.T) {
	tests := []struct {
		name       string
		body       string
		mockSetup  func(mock pgxmock.PgxPoolIface)
		wantStatus int
	}{
		{
			name: "H-32 SetProjectUsers 空列表",
			body: `{"user_ids":[]}`,
			mockSetup: func(mock pgxmock.PgxPoolIface) {
				mock.ExpectBegin()
				mock.ExpectExec("DELETE FROM project_user WHERE project_id").
					WithArgs(1).
					WillReturnResult(pgxmock.NewResult("DELETE", 0))
				mock.ExpectCommit()
			},
			wantStatus: http.StatusOK,
		},
		{
			name: "SetProjectUsers 正常分配用户",
			body: `{"user_ids":[2,3,5]}`,
			mockSetup: func(mock pgxmock.PgxPoolIface) {
				mock.ExpectBegin()
				mock.ExpectExec("DELETE FROM project_user WHERE project_id").
					WithArgs(1).
					WillReturnResult(pgxmock.NewResult("DELETE", 2))
				mock.ExpectExec("INSERT INTO project_user").
					WithArgs(1, 2).
					WillReturnResult(pgxmock.NewResult("INSERT", 1))
				mock.ExpectExec("INSERT INTO project_user").
					WithArgs(1, 3).
					WillReturnResult(pgxmock.NewResult("INSERT", 1))
				mock.ExpectExec("INSERT INTO project_user").
					WithArgs(1, 5).
					WillReturnResult(pgxmock.NewResult("INSERT", 1))
				mock.ExpectCommit()
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

			repo := postgres.NewProjectRepo(mock)
			h := NewProjectHandler(repo)

			req := httptest.NewRequest("PUT", "/api/v1/projects/1/users", strings.NewReader(tt.body))
			req.SetPathValue("id", "1")
			req.Header.Set("Content-Type", "application/json")
			rec := httptest.NewRecorder()
			h.SetProjectUsers(rec, req)

			if rec.Code != tt.wantStatus {
				t.Errorf("status = %d, want %d", rec.Code, tt.wantStatus)
			}
			if err := mock.ExpectationsWereMet(); err != nil {
				t.Errorf("unmet expectations: %v", err)
			}
		})
	}
}

func jsonNullTime() time.Time {
	return time.Time{}
}

// =============================================================================
// Tier 4 — H-14~H-17: AuthHandler Login
// =============================================================================

func TestAuthHandler_Login(t *testing.T) {
	validHash, _ := auth.HashPassword("correct-password")

	tests := []struct {
		name       string
		body       string
		mockSetup  func(mock pgxmock.PgxPoolIface)
		wantStatus int
		wantBody   string
	}{
		{
			name: "H-14 Login 成功",
			body: `{"username":"testuser","password":"correct-password"}`,
			mockSetup: func(mock pgxmock.PgxPoolIface) {
				mock.ExpectQuery("SELECT u\\.id, u\\.username, u\\.password_hash").
					WithArgs("testuser").
					WillReturnRows(pgxmock.NewRows([]string{
						"id", "username", "password_hash", "role_id", "code", "level",
						"agent_id", "default_project_id", "is_active",
					}).AddRow(1, "testuser", validHash, 1, "admin", 10, nil, nil, true))
				mock.ExpectQuery("SELECT p\\.code FROM permission").
					WithArgs(1).
					WillReturnRows(pgxmock.NewRows([]string{"code"}).
						AddRow("device.view").AddRow("device.control"))
				mock.ExpectExec("UPDATE users SET last_login_at").
					WithArgs(1).
					WillReturnResult(pgxmock.NewResult("UPDATE", 1))
			},
			wantStatus: http.StatusOK,
			wantBody:   `"token":"`,
		},
		{
			name: "H-15 Login 凭证错误",
			body: `{"username":"nobody","password":"wrong"}`,
			mockSetup: func(mock pgxmock.PgxPoolIface) {
				mock.ExpectQuery("SELECT u\\.id, u\\.username, u\\.password_hash").
					WithArgs("nobody").
					WillReturnError(pgx.ErrNoRows)
			},
			wantStatus: http.StatusUnauthorized,
			wantBody:   "用户名或密码错误",
		},
		{
			name: "H-16 Login 内部错误",
			body: `{"username":"erruser","password":"any"}`,
			mockSetup: func(mock pgxmock.PgxPoolIface) {
				mock.ExpectQuery("SELECT u\\.id, u\\.username, u\\.password_hash").
					WithArgs("erruser").
					WillReturnError(errors.New("connection refused"))
			},
			wantStatus: http.StatusInternalServerError,
			wantBody:   "服务内部错误",
		},
		{
			name:       "H-17 Login 请求体解析失败",
			body:       `{invalid json`,
			mockSetup:  nil,
			wantStatus: http.StatusBadRequest,
			wantBody:   "请求格式错误",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var h *AuthHandler
			var mock pgxmock.PgxPoolIface

			if tt.mockSetup != nil {
				var err error
				mock, err = pgxmock.NewPool()
				if err != nil {
					t.Fatal(err)
				}
				defer mock.Close()
				tt.mockSetup(mock)
				h = NewAuthHandler(auth.New(mock, "test-jwt-secret"))
			} else {
				h = NewAuthHandler(auth.New(nil, "test-jwt-secret"))
			}

			req := httptest.NewRequest("POST", "/api/v1/auth/login", strings.NewReader(tt.body))
			req.Header.Set("Content-Type", "application/json")
			rec := httptest.NewRecorder()
			h.Login(rec, req)

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

// =============================================================================
// Tier 4 — H-18~H-20: AuthHandler Me
// =============================================================================

func TestAuthHandler_Me(t *testing.T) {
	tests := []struct {
		name       string
		claims     *auth.Claims
		mockSetup  func(mock pgxmock.PgxPoolIface)
		wantStatus int
		wantBody   string
	}{
		{
			name:       "H-18 Me 已认证返回用户信息",
			claims:     &auth.Claims{UserID: 1, Username: "admin", RoleCode: "super_admin"},
			mockSetup: func(mock pgxmock.PgxPoolIface) {
				mock.ExpectQuery("SELECT COALESCE\\(name,''\\).FROM role WHERE").
					WithArgs("super_admin").
					WillReturnRows(pgxmock.NewRows([]string{"name"}).AddRow("超级管理员"))
			},
			wantStatus: http.StatusOK,
			wantBody:   "超级管理员",
		},
		{
			name:       "H-19 Me 未认证返回401",
			claims:     nil,
			mockSetup:  nil,
			wantStatus: http.StatusUnauthorized,
			wantBody:   "未认证",
		},
		{
			name:   "H-20 Me 角色名查询失败仍返回200",
			claims: &auth.Claims{UserID: 1, Username: "admin", RoleCode: "unknown_role"},
			mockSetup: func(mock pgxmock.PgxPoolIface) {
				mock.ExpectQuery("SELECT COALESCE\\(name,''\\).FROM role WHERE").
					WithArgs("unknown_role").
					WillReturnError(errors.New("db error"))
			},
			wantStatus: http.StatusOK,
			wantBody:   `"role":"unknown_role"`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var h *AuthHandler
			var mock pgxmock.PgxPoolIface

			if tt.mockSetup != nil {
				var err error
				mock, err = pgxmock.NewPool()
				if err != nil {
					t.Fatal(err)
				}
				defer mock.Close()
				tt.mockSetup(mock)
				h = NewAuthHandler(auth.New(mock, "secret"))
			} else {
				h = NewAuthHandler(auth.New(nil, "secret"))
			}

			req := httptest.NewRequest("GET", "/api/v1/auth/me", nil)
			if tt.claims != nil {
				ctx := context.WithValue(t.Context(), middleware.CtxClaims, tt.claims)
				req = req.WithContext(ctx)
			}
			rec := httptest.NewRecorder()
			h.Me(rec, req)

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

// =============================================================================
// Tier 4 — H-25~H-28: AdminHandler UpdateUser
// =============================================================================

func TestAdminHandler_UpdateUser(t *testing.T) {
	var nilInt *int
	var nilStr *string

	tests := []struct {
		name       string
		pathID     string
		body       string
		claims     *auth.Claims
		mockSetup  func(mock pgxmock.PgxPoolIface)
		wantStatus int
		wantBody   string
	}{
		{
			name:   "H-25 部分更新 IsActive",
			pathID: "3",
			body:   `{"is_active":false}`,
			claims: &auth.Claims{UserID: 1, Username: "admin", RoleCode: "super_admin"},
			mockSetup: func(mock pgxmock.PgxPoolIface) {
				mock.ExpectExec("UPDATE users SET").
					WithArgs(0, nilInt, false, nilStr, 3).
					WillReturnResult(pgxmock.NewResult("UPDATE", 1))
			},
			wantStatus: http.StatusOK,
			wantBody:   `"status":"updated"`,
		},
		{
			name:       "H-26 全零值字段被拒",
			pathID:     "3",
			body:       `{}`,
			claims:     &auth.Claims{UserID: 1, Username: "admin", RoleCode: "super_admin"},
			mockSetup:  nil,
			wantStatus: http.StatusBadRequest,
			wantBody:   "至少需要提供一个要更新的字段",
		},
		{
			name:   "H-27 非超管修改自己信息通过",
			pathID: "5",
			body:   `{"role_id":2}`,
			claims: &auth.Claims{UserID: 5, Username: "user5", RoleCode: "admin"},
			mockSetup: func(mock pgxmock.PgxPoolIface) {
				mock.ExpectExec("UPDATE users SET").
					WithArgs(2, nilInt, true, nilStr, 5).
					WillReturnResult(pgxmock.NewResult("UPDATE", 1))
			},
			wantStatus: http.StatusOK,
			wantBody:   `"status":"updated"`,
		},
		{
			name:       "H-27 非超管修改他人信息被拒",
			pathID:     "10",
			body:       `{"is_active":false}`,
			claims:     &auth.Claims{UserID: 5, Username: "user5", RoleCode: "admin"},
			mockSetup:  nil,
			wantStatus: http.StatusForbidden,
			wantBody:   "无权限修改该用户",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var h *AdminHandler
			var mock pgxmock.PgxPoolIface

			if tt.mockSetup != nil {
				var err error
				mock, err = pgxmock.NewPool()
				if err != nil {
					t.Fatal(err)
				}
				defer mock.Close()
				tt.mockSetup(mock)
				h = NewAdminHandler(postgres.NewAdminRepo(mock), nil)
			} else {
				h = NewAdminHandler(nil, nil)
			}

			req := httptest.NewRequest("PUT", "/api/v1/admin/users/"+tt.pathID, strings.NewReader(tt.body))
			req.SetPathValue("id", tt.pathID)
			req.Header.Set("Content-Type", "application/json")
			if tt.claims != nil {
				ctx := context.WithValue(t.Context(), middleware.CtxClaims, tt.claims)
				req = req.WithContext(ctx)
			}
			rec := httptest.NewRecorder()
			h.UpdateUser(rec, req)

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

func TestAdminHandler_UpdateUser_DuplicateUsername(t *testing.T) {
	// H-28: 重复用户名 → 409 Conflict
	// Note: UpdateUser handler doesn't update username, but CreateUser returns 409 on duplicate.
	// This case tests CreateUser behavior for duplicate usernames.
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	h := NewAdminHandler(postgres.NewAdminRepo(mock), nil)
	body := `{"username":"duplicate","password":"pass1234","role_id":1}`
	req := httptest.NewRequest("POST", "/api/v1/admin/users", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	// Expect CreateUser call - mock will fail with duplicate key error
	mock.ExpectQuery("INSERT INTO users").
		WithArgs(pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg()).
		WillReturnError(errors.New("duplicate key value violates unique constraint"))

	h.CreateUser(rec, req)

	if rec.Code != http.StatusConflict {
		t.Errorf("H-28: expected 409 for duplicate username, got %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "用户名已存在") {
		t.Errorf("H-28: body = %q, want to contain '用户名已存在'", rec.Body.String())
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet expectations: %v", err)
	}
}
