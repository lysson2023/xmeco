package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/pashagolub/pgxmock/v4"

	"xmeco/internal/api/middleware"
	"xmeco/internal/repository/postgres"
	"xmeco/internal/service/auth"
)

// =============================================================================
// Tier 1 — H-21~H-24: AdminHandler ResetPassword 安全路径
// =============================================================================

func TestAdminHandler_ResetPassword(t *testing.T) {
	// 预备 bcrypt 哈希用于 CheckPassword
	validHash, _ := auth.HashPassword("correct-old-password")

	tests := []struct {
		name        string
		claims      *auth.Claims
		targetID    int
		oldPassword string
		newPassword string
		// pgxmock setup
		mockSetup func(mock pgxmock.PgxPoolIface)
		wantStatus int
		wantBody   string
	}{
		{
			name:        "H-21 超级管理员无需旧密码",
			claims:      &auth.Claims{UserID: 1, Username: "admin", RoleCode: auth.RoleSuperAdmin},
			targetID:    5,
			oldPassword: "",
			newPassword: "new-pass-123",
			mockSetup: func(mock pgxmock.PgxPoolIface) {
				// 超级管理员 old_password 为空 → 跳过 GetPasswordHash 和 CheckPassword
				// 直接到 HashPassword + ResetPassword
				mock.ExpectExec("UPDATE users SET password_hash=\\$1 WHERE id=\\$2").
					WithArgs(pgxmock.AnyArg(), 5).
					WillReturnResult(pgxmock.NewResult("UPDATE", 1))
			},
			wantStatus: http.StatusOK,
		},
		{
			name:        "H-21 超级管理员提供旧密码也允许(验证通过)",
			claims:      &auth.Claims{UserID: 1, Username: "admin", RoleCode: auth.RoleSuperAdmin},
			targetID:    3,
			oldPassword: "correct-old-password",
			newPassword: "new-pass-456",
			mockSetup: func(mock pgxmock.PgxPoolIface) {
				// 超级管理员提供了 old_password → 走验证路径
				mock.ExpectQuery("SELECT password_hash FROM users WHERE id=\\$1").
					WithArgs(3).
					WillReturnRows(pgxmock.NewRows([]string{"password_hash"}).AddRow(validHash))
				mock.ExpectExec("UPDATE users SET password_hash=\\$1 WHERE id=\\$2").
					WithArgs(pgxmock.AnyArg(), 3).
					WillReturnResult(pgxmock.NewResult("UPDATE", 1))
			},
			wantStatus: http.StatusOK,
		},
		{
			name:        "H-22 普通用户提供正确旧密码",
			claims:      &auth.Claims{UserID: 10, Username: "user1", RoleCode: "operator"},
			targetID:    10, // 自己
			oldPassword: "correct-old-password",
			newPassword: "new-pass-789",
			mockSetup: func(mock pgxmock.PgxPoolIface) {
				mock.ExpectQuery("SELECT password_hash FROM users WHERE id=\\$1").
					WithArgs(10).
					WillReturnRows(pgxmock.NewRows([]string{"password_hash"}).AddRow(validHash))
				mock.ExpectExec("UPDATE users SET password_hash=\\$1 WHERE id=\\$2").
					WithArgs(pgxmock.AnyArg(), 10).
					WillReturnResult(pgxmock.NewResult("UPDATE", 1))
			},
			wantStatus: http.StatusOK,
		},
		{
			name:        "H-23 普通用户提供错误旧密码",
			claims:      &auth.Claims{UserID: 10, Username: "user1", RoleCode: "operator"},
			targetID:    10,
			oldPassword: "wrong-old-password",
			newPassword: "new-pass-000",
			mockSetup: func(mock pgxmock.PgxPoolIface) {
				mock.ExpectQuery("SELECT password_hash FROM users WHERE id=\\$1").
					WithArgs(10).
					WillReturnRows(pgxmock.NewRows([]string{"password_hash"}).AddRow(validHash))
				// CheckPassword 失败 → 直接返回 403，不调 ResetPassword
			},
			wantStatus: http.StatusForbidden,
			wantBody:   "原密码错误",
		},
		{
			name:        "H-24 普通用户不提供旧密码",
			claims:      &auth.Claims{UserID: 10, Username: "user1", RoleCode: "operator"},
			targetID:    10,
			oldPassword: "",
			newPassword: "new-pass-111",
			mockSetup:   nil, // 不访问 DB
			wantStatus:  http.StatusForbidden,
			wantBody:    "请提供原密码",
		},
		{
			name:        "普通用户无法重置他人密码",
			claims:      &auth.Claims{UserID: 10, Username: "user1", RoleCode: "operator"},
			targetID:    99, // 别人的 ID
			oldPassword: "",
			newPassword: "new-pass-222",
			mockSetup:   nil, // RBAC 阶段就拒绝
			wantStatus:  http.StatusForbidden,
			wantBody:    "无权限",
		},
		{
			name:        "缺少新密码",
			claims:      &auth.Claims{UserID: 10, Username: "user1", RoleCode: "operator"},
			targetID:    10,
			oldPassword: "",
			newPassword: "",
			mockSetup:   nil,
			wantStatus:  http.StatusBadRequest,
			wantBody:    "请输入新密码",
		},
		{
			name:        "无效的用户ID",
			claims:      &auth.Claims{UserID: 10, Username: "user1", RoleCode: "operator"},
			targetID:    0,
			oldPassword: "",
			newPassword: "some-pass",
			mockSetup:   nil,
			wantStatus:  http.StatusBadRequest,
			wantBody:    "无效的用户 ID",
		},
		{
			name:        "缺少Claims上下文",
			claims:      nil,
			targetID:    1,
			oldPassword: "",
			newPassword: "some-pass",
			mockSetup:   nil,
			wantStatus:  http.StatusUnauthorized,
			wantBody:    "未认证",
		},
		{
			name:        "用户不存在-GetPasswordHash返回404",
			claims:      &auth.Claims{UserID: 10, Username: "user1", RoleCode: "operator"},
			targetID:    10,
			oldPassword: "any-old-password",
			newPassword: "new-pass",
			mockSetup: func(mock pgxmock.PgxPoolIface) {
				mock.ExpectQuery("SELECT password_hash FROM users WHERE id=\\$1").
					WithArgs(10).
					WillReturnError(pgx.ErrNoRows)
			},
			wantStatus: http.StatusNotFound,
			wantBody:   "用户不存在",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var repo *postgres.AdminRepo
			var mock pgxmock.PgxPoolIface

			if tt.mockSetup != nil {
				var err error
				mock, err = pgxmock.NewPool()
				if err != nil {
					t.Fatal(err)
				}
				defer mock.Close()
				tt.mockSetup(mock)
				repo = postgres.NewAdminRepo(mock)
			} else {
				// 无 mock setup 时用一个不会用到 DB 的空 AdminHandler
				repo = postgres.NewAdminRepo(nil) // nil DB will panic if accessed
			}

			// authSvc 用于 ResetPassword 中的 token_version 递增（需 Pool()）
			var authSvc *auth.Service
			if mock != nil {
				authSvc = auth.New(mock, "test-secret")
			}
			h := NewAdminHandler(repo, authSvc)

			// 构造包含 claims 的 context
			ctx := context.Background()
			if tt.claims != nil {
				ctx = context.WithValue(ctx, middleware.CtxClaims, tt.claims)
			}

			// 构造 JSON body + 设置 path value (Go 1.22+ 路由参数)
			bodyJSON, _ := json.Marshal(map[string]string{"old_password": tt.oldPassword, "new_password": tt.newPassword})
			req := httptest.NewRequest("POST", "/api/v1/users/"+itos(tt.targetID)+"/reset-password", bytes.NewReader(bodyJSON))
			req = req.WithContext(ctx)
			req.SetPathValue("id", itos(tt.targetID))
			req.Header.Set("Content-Type", "application/json")
			if tt.claims != nil {
				req.Header.Set("Authorization", "Bearer test-token")
			}

			rec := httptest.NewRecorder()

			// 对于 nil repo 的 case，recover 防止 panic
			if tt.mockSetup == nil {
				func() {
					defer func() { recover() }()
					h.ResetPassword(rec, req)
				}()
				// 验证响应码
				if rec.Code != tt.wantStatus {
					t.Errorf("status = %d, want %d", rec.Code, tt.wantStatus)
				}
			} else {
				h.ResetPassword(rec, req)
				if rec.Code != tt.wantStatus {
					t.Errorf("status = %d, want %d", rec.Code, tt.wantStatus)
				}
			}

			if tt.wantBody != "" {
				if !strings.Contains(rec.Body.String(), tt.wantBody) {
					t.Errorf("body = %q, want to contain %q", rec.Body.String(), tt.wantBody)
				}
			}

			if mock != nil {
				if err := mock.ExpectationsWereMet(); err != nil {
					t.Errorf("unmet expectations: %v", err)
				}
			}
		})
	}
}
