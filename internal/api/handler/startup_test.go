package handler

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/pashagolub/pgxmock/v4"

	"xmeco/internal/api/middleware"
	"xmeco/internal/service/auth"
)

func TestStartupHandler_Execute(t *testing.T) {
	// H-52: Execute plan — success path, goroutine runs in background
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	// 1. LoadPlan: QueryRow for plan metadata
	mock.ExpectQuery("SELECT id,name,building_id,precheck_online,precheck_alarm FROM startup_plan").
		WithArgs(1).
		WillReturnRows(pgxmock.NewRows([]string{"id", "name", "building_id", "precheck_online", "precheck_alarm"}).
			AddRow(1, "夏季开机计划", 1, true, false))
	// 2. LoadPlan: Query for steps
	mock.ExpectQuery("SELECT ss\\.sort_order.*FROM startup_step").
		WithArgs(1).
		WillReturnRows(pgxmock.NewRows([]string{"sort_order", "device_id", "name", "action", "target_value", "wait_seconds", "skip_if_offline", "retry_count"}).
			AddRow(1, 5, "冷水机组1", "startup", nil, 0, false, 1))
	// 3. StartExecution: INSERT INTO startup_execution
	mock.ExpectQuery("INSERT INTO startup_execution").
		WithArgs(1, "夏季开机计划", "admin", 1, pgxmock.AnyArg()).
		WillReturnRows(pgxmock.NewRows([]string{"id"}).AddRow(100))

	h := NewStartupHandler(mock)
	claims := &auth.Claims{UserID: 1, Username: "admin", RoleCode: "super_admin"}
	ctx := context.WithValue(t.Context(), middleware.CtxClaims, claims)
	req := httptest.NewRequest("POST", "/api/v1/startup-plans/1/execute", nil)
	req = req.WithContext(ctx)
	req.SetPathValue("id", "1")
	rec := httptest.NewRecorder()
	h.Execute(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("H-52: status = %d, want 200", rec.Code)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("H-52: unmet expectations: %v", err)
	}
}

func TestStartupHandler_Execute_Unauthenticated(t *testing.T) {
	// H-52 boundary: no claims in context → 401
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	// LoadPlan succeeds
	mock.ExpectQuery("SELECT id,name,building_id,precheck_online,precheck_alarm FROM startup_plan").
		WithArgs(1).
		WillReturnRows(pgxmock.NewRows([]string{"id", "name", "building_id", "precheck_online", "precheck_alarm"}).
			AddRow(1, "计划", 1, true, false))
	mock.ExpectQuery("SELECT ss\\.sort_order.*FROM startup_step").
		WithArgs(1).
		WillReturnRows(pgxmock.NewRows([]string{"sort_order", "device_id", "name", "action", "target_value", "wait_seconds", "skip_if_offline", "retry_count"}))

	h := NewStartupHandler(mock)
	req := httptest.NewRequest("POST", "/api/v1/startup-plans/1/execute", nil)
	req.SetPathValue("id", "1")
	rec := httptest.NewRecorder()
	h.Execute(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401", rec.Code)
	}
}

func TestStartupHandler_Execute_PlanNotFound(t *testing.T) {
	// H-52 boundary: plan doesn't exist → 404
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	mock.ExpectQuery("SELECT id,name,building_id,precheck_online,precheck_alarm FROM startup_plan").
		WithArgs(999).
		WillReturnError(errors.New("no rows"))

	h := NewStartupHandler(mock)
	req := httptest.NewRequest("POST", "/api/v1/startup-plans/999/execute", nil)
	req.SetPathValue("id", "999")
	rec := httptest.NewRecorder()
	h.Execute(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Errorf("status = %d, want 404", rec.Code)
	}
}

func TestStartupHandler_RunDueScheduledTasks_Weekly(t *testing.T) {
	// H-55: RunDueScheduledTasks — weekly schedule with startup action
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	// 1. Main due-tasks query returns a weekly startup task
	mock.ExpectQuery("SELECT st\\.id, st\\.device_id, st\\.action_type").
		WillReturnRows(pgxmock.NewRows([]string{"id", "device_id", "action_type", "target_value", "device_name"}).
			AddRow(10, 5, "startup", nil, "冷水机组1"))
	// 2. executeAction: INSERT control_record (devID, controlVal, remark)
	mock.ExpectExec("INSERT INTO control_record.*SELECT.*WHERE d\\.id").
		WithArgs(5, "开机", "startup").
		WillReturnResult(pgxmock.NewResult("INSERT", 1))
	// 3. UPDATE device SET device_status (now parameterized: $1, $2)
	mock.ExpectExec("UPDATE device SET device_status").
		WithArgs("开机", 5).
		WillReturnResult(pgxmock.NewResult("UPDATE", 1))
	// 4. UPDATE scheduled_task last_result=$1
	mock.ExpectExec("UPDATE scheduled_task SET last_result").
		WithArgs("success", 10).
		WillReturnResult(pgxmock.NewResult("UPDATE", 1))

	h := NewStartupHandler(mock)
	h.RunDueScheduledTasks(context.Background())

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("H-55: unmet expectations: %v", err)
	}
}

func TestStartupHandler_RunDueScheduledTasks_Once(t *testing.T) {
	// H-56: RunDueScheduledTasks — once schedule with shutdown action
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	// 1. Main due-tasks query returns a one-time shutdown task
	mock.ExpectQuery("SELECT st\\.id, st\\.device_id, st\\.action_type").
		WillReturnRows(pgxmock.NewRows([]string{"id", "device_id", "action_type", "target_value", "device_name"}).
			AddRow(11, 3, "shutdown", nil, "冷却泵1"))
	// 2. executeAction: INSERT control_record (devID, controlVal, remark)
	mock.ExpectExec("INSERT INTO control_record.*SELECT.*WHERE d\\.id").
		WithArgs(3, "关机", "shutdown").
		WillReturnResult(pgxmock.NewResult("INSERT", 1))
	// 3. UPDATE device SET device_status (now parameterized: $1, $2)
	mock.ExpectExec("UPDATE device SET device_status").
		WithArgs("关机", 3).
		WillReturnResult(pgxmock.NewResult("UPDATE", 1))
	// 4. UPDATE scheduled_task last_result=$1
	mock.ExpectExec("UPDATE scheduled_task SET last_result").
		WithArgs("success", 11).
		WillReturnResult(pgxmock.NewResult("UPDATE", 1))

	h := NewStartupHandler(mock)
	h.RunDueScheduledTasks(context.Background())

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("H-56: unmet expectations: %v", err)
	}
}

func TestStartupHandler_RunDueScheduledTasks_ExecuteFailed(t *testing.T) {
	// RunDueScheduledTasks: executeAction fails → last_result=$1
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	// Main query returns a task
	mock.ExpectQuery("SELECT st\\.id, st\\.device_id, st\\.action_type").
		WillReturnRows(pgxmock.NewRows([]string{"id", "device_id", "action_type", "target_value", "device_name"}).
			AddRow(12, 7, "startup", nil, "设备X"))
	// executeAction: INSERT fails (devID, controlVal, remark)
	mock.ExpectExec("INSERT INTO control_record.*SELECT.*WHERE d\\.id").
		WithArgs(7, "开机", "startup").
		WillReturnError(errors.New("connection refused"))
	// Failure path: UPDATE last_result=$1
	mock.ExpectExec("UPDATE scheduled_task SET last_result").
		WithArgs("failed", 12).
		WillReturnResult(pgxmock.NewResult("UPDATE", 1))

	h := NewStartupHandler(mock)
	h.RunDueScheduledTasks(context.Background())

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet expectations: %v", err)
	}
}

func TestStartupHandler_RunDueScheduledTasks_Empty(t *testing.T) {
	// RunDueScheduledTasks: no due tasks → no downstream calls
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	// Main query returns empty result
	mock.ExpectQuery("SELECT st\\.id, st\\.device_id, st\\.action_type").
		WillReturnRows(pgxmock.NewRows([]string{"id", "device_id", "action_type", "target_value", "device_name"}))

	h := NewStartupHandler(mock)
	h.RunDueScheduledTasks(context.Background())

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet expectations: %v", err)
	}
}
