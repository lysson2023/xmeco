package postgres

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/pashagolub/pgxmock/v4"

	"xmeco/internal/domain"
)

// ---- BuildingRepo ----

func TestBuildingRepoList(t *testing.T) {
	mock, _ := pgxmock.NewPool()
	defer mock.Close()
	repo := NewBuildingRepo(mock)
	now := time.Now()

	cols := []string{"id", "project_id", "name", "outdoor_temp", "outdoor_humidity", "total_energy", "save_rate", "save_energy", "carbon_rate", "carbon_saving", "created_at"}
	mock.ExpectQuery("SELECT (.+) FROM building").WithArgs(0).
		WillReturnRows(pgxmock.NewRows(cols).AddRow(1, 1, "A栋", nil, nil, 10000.0, 0.15, 1500.0, 0.50, 750.0, now))

	list, err := repo.List(context.TODO(), 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 1 || list[0].Name != "A栋" {
		t.Errorf("expected 1 building named A栋, got %d", len(list))
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet expectations: %v", err)
	}
}

func TestBuildingRepoListFilter(t *testing.T) {
	mock, _ := pgxmock.NewPool()
	defer mock.Close()
	repo := NewBuildingRepo(mock)

	cols := []string{"id", "project_id", "name", "outdoor_temp", "outdoor_humidity", "total_energy", "save_rate", "save_energy", "carbon_rate", "carbon_saving", "created_at"}
	mock.ExpectQuery("SELECT (.+) FROM building").WithArgs(3).
		WillReturnRows(pgxmock.NewRows(cols))

	list, err := repo.List(context.TODO(), 3)
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 0 {
		t.Errorf("expected empty, got %d", len(list))
	}
}

func TestBuildingRepoGetByID(t *testing.T) {
	mock, _ := pgxmock.NewPool()
	defer mock.Close()
	repo := NewBuildingRepo(mock)
	now := time.Now()

	cols := []string{"id", "project_id", "name", "outdoor_temp", "outdoor_humidity", "total_energy", "save_rate", "save_energy", "carbon_rate", "carbon_saving", "created_at"}
	mock.ExpectQuery("SELECT (.+) FROM building WHERE id=\\$1").WithArgs(1).
		WillReturnRows(pgxmock.NewRows(cols).AddRow(1, 2, "B栋", nil, nil, 0.0, 0.0, 0.0, 0.0, 0.0, now))

	b, err := repo.GetByID(context.TODO(), 1)
	if err != nil {
		t.Fatal(err)
	}
	if b.Name != "B栋" || b.ProjectID != 2 {
		t.Errorf("unexpected: %+v", b)
	}
}

func TestBuildingRepoGetByIDNotFound(t *testing.T) {
	mock, _ := pgxmock.NewPool()
	defer mock.Close()
	repo := NewBuildingRepo(mock)

	mock.ExpectQuery("SELECT (.+) FROM building WHERE id=\\$1").WithArgs(999).
		WillReturnError(pgx.ErrNoRows)

	_, err := repo.GetByID(context.TODO(), 999)
	if err == nil {
		t.Fatal("expected error for non-existent building")
	}
}

func TestBuildingRepoCreate(t *testing.T) {
	mock, _ := pgxmock.NewPool()
	defer mock.Close()
	repo := NewBuildingRepo(mock)
	now := time.Now()

	cols := []string{"id", "created_at"}
	b := &domain.Building{ProjectID: 1, Name: "C栋", SaveRate: 0.12}
	// Use AnyArg for nullable float pointers since pgxmock typed-nil matching is brittle
	mock.ExpectQuery("INSERT INTO building").
		WithArgs(1, "C栋", pgxmock.AnyArg(), pgxmock.AnyArg(), 0.0, 0.12, 0.0, 0.0, 0.0).
		WillReturnRows(pgxmock.NewRows(cols).AddRow(10, now))

	if err := repo.Create(context.TODO(), b); err != nil {
		t.Fatal(err)
	}
	if b.ID != 10 {
		t.Errorf("ID = %d, want 10", b.ID)
	}
}

func TestBuildingRepoUpdate(t *testing.T) {
	mock, _ := pgxmock.NewPool()
	defer mock.Close()
	repo := NewBuildingRepo(mock)

	b := &domain.Building{ID: 1, Name: "D栋", SaveRate: 0.2}
	mock.ExpectExec("UPDATE building SET").
		WithArgs("D栋", pgxmock.AnyArg(), pgxmock.AnyArg(), 0.0, 0.2, 0.0, 0.0, 0.0, 1).
		WillReturnResult(pgxmock.NewResult("UPDATE", 1))

	if err := repo.Update(context.TODO(), b); err != nil {
		t.Fatal(err)
	}
}

func TestBuildingRepoDelete(t *testing.T) {
	mock, _ := pgxmock.NewPool()
	defer mock.Close()
	repo := NewBuildingRepo(mock)

	mock.ExpectExec("DELETE FROM building WHERE id=\\$1").WithArgs(1).
		WillReturnResult(pgxmock.NewResult("DELETE", 1))

	if err := repo.Delete(context.TODO(), 1); err != nil {
		t.Fatal(err)
	}
}

// ---- DeviceRepo ----

func TestDeviceRepoList(t *testing.T) {
	mock, _ := pgxmock.NewPool()
	defer mock.Close()
	repo := NewDeviceRepo(mock)
	now := time.Now()

	dCols := []string{"id", "building_id", "name", "device_type", "gateway_imei", "gateway_type", "node_address", "device_no", "ct_ratio", "pt_ratio", "rated_voltage", "rated_current", "power_sign", "online_status", "device_status", "last_online_at", "last_record_at", "created_at"}
	mock.ExpectQuery("SELECT (.+) FROM device").WithArgs(0).
		WillReturnRows(pgxmock.NewRows(dCols).AddRow(1, 1, "主机1", "主机", nil, "custom", 1, 1, 100, 200, nil, nil, 1, "在线", "运行", nil, nil, now))

	list, err := repo.List(context.TODO(), 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 1 || list[0].DeviceType != "主机" {
		t.Errorf("expected 1 device type 主机, got %d", len(list))
	}
}

func TestDeviceRepoGetByID(t *testing.T) {
	mock, _ := pgxmock.NewPool()
	defer mock.Close()
	repo := NewDeviceRepo(mock)
	now := time.Now()

	dCols2 := []string{"id", "building_id", "name", "device_type", "gateway_imei", "gateway_type", "node_address", "device_no", "ct_ratio", "pt_ratio", "rated_voltage", "rated_current", "power_sign", "online_status", "device_status", "last_online_at", "last_record_at", "created_at"}
	mock.ExpectQuery("SELECT (.+) FROM device WHERE id=\\$1").WithArgs(1).
		WillReturnRows(pgxmock.NewRows(dCols2).AddRow(1, 1, "主机1", "主机", nil, "custom", 1, 1, 0, 0, nil, nil, 1, "在线", "运行中", nil, nil, now))

	d, err := repo.GetByID(context.TODO(), 1)
	if err != nil {
		t.Fatal(err)
	}
	if d.Name != "主机1" || d.DeviceStatus != "运行中" {
		t.Errorf("unexpected: Name=%q Status=%q", d.Name, d.DeviceStatus)
	}
}

// ---- PropertyRepo ----

func TestPropertyRepoList(t *testing.T) {
	mock, _ := pgxmock.NewPool()
	defer mock.Close()
	repo := NewPropertyRepo(mock)

	cols := []string{"id", "device_id", "prop_name", "prop_short", "prop_value", "unit", "operation_type", "is_key", "prop_type", "min_value", "max_value", "sort_order"}
	mock.ExpectQuery("SELECT (.+) FROM device_properties").WithArgs(0).
		WillReturnRows(pgxmock.NewRows(cols).AddRow(1, 1, "电压", "U", "220", "V", "数值", true, "float", "", "", 1))

	list, err := repo.List(context.TODO(), 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 1 || list[0].PropName != "电压" {
		t.Errorf("expected 1 property named 电压, got %d", len(list))
	}
}

func TestPropertyRepoCreate(t *testing.T) {
	mock, _ := pgxmock.NewPool()
	defer mock.Close()
	repo := NewPropertyRepo(mock)

	cols := []string{"id"}
	p := &domain.DeviceProperty{DeviceID: 1, PropName: "电流", Unit: "A", SortOrder: 2}
	mock.ExpectQuery("INSERT INTO device_properties").
		WithArgs(1, "电流", "", "", "A", "", false, "", "", "", 2).
		WillReturnRows(pgxmock.NewRows(cols).AddRow(5))

	if err := repo.Create(context.TODO(), p); err != nil {
		t.Fatal(err)
	}
	if p.ID != 5 {
		t.Errorf("ID = %d, want 5", p.ID)
	}
}

// ---- RegisterRepo ----

func TestRegisterRepoList(t *testing.T) {
	mock, _ := pgxmock.NewPool()
	defer mock.Close()
	repo := NewRegisterRepo(mock)

	rCols := []string{"id", "property_id", "name", "read_addr", "read_code", "write_addr", "write_code", "command_name", "command_code", "status_code", "data_type", "data_length", "data_order", "data_mask", "magnification"}
	mock.ExpectQuery("SELECT (.+) FROM register").WithArgs(0).
		WillReturnRows(pgxmock.NewRows(rCols).AddRow(1, 1, "电压寄存器", 0, "03", nil, nil, nil, "03", nil, "float", 2, "AB", nil, 1.0))

	list, err := repo.List(context.TODO(), 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 1 || list[0].Name != "电压寄存器" {
		t.Errorf("expected 1 register named 电压寄存器, got %d", len(list))
	}
}

func TestRegisterRepoListByDeviceID(t *testing.T) {
	mock, _ := pgxmock.NewPool()
	defer mock.Close()
	repo := NewRegisterRepo(mock)

	rCols := []string{"id", "property_id", "name", "read_addr", "read_code", "write_addr", "write_code", "command_name", "command_code", "status_code", "data_type", "data_length", "data_order", "data_mask", "magnification"}
	mock.ExpectQuery("SELECT r\\.(.+) FROM register r JOIN").WithArgs(1).
		WillReturnRows(pgxmock.NewRows(rCols))

	list, err := repo.ListByDeviceID(context.TODO(), 1)
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 0 {
		t.Errorf("expected empty, got %d", len(list))
	}
}

func TestRegisterRepoUpdate(t *testing.T) {
	mock, _ := pgxmock.NewPool()
	defer mock.Close()
	repo := NewRegisterRepo(mock)

	reg := &domain.Register{ID: 1, Name: "更新后", ReadAddr: 100, ReadCode: "03", DataType: "float", DataLength: 4, Magnification: 1.0}
	wAddr := pgxmock.AnyArg()
	mock.ExpectExec("UPDATE register SET").
		WithArgs("更新后", 100, "03", wAddr, pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(), "float", 4, "", pgxmock.AnyArg(), 1.0, 1).
		WillReturnResult(pgxmock.NewResult("UPDATE", 1))

	if err := repo.Update(context.TODO(), reg); err != nil {
		t.Fatal(err)
	}
}

// ---- ProjectRepo ----

func TestProjectRepoList(t *testing.T) {
	mock, _ := pgxmock.NewPool()
	defer mock.Close()
	repo := NewProjectRepo(mock)
	now := time.Now()

	cols := []string{"id", "name", "agent_id", "address", "admin_code", "city_id", "city_name", "created_at"}
	mock.ExpectQuery("SELECT (.+) FROM project").
		WillReturnRows(pgxmock.NewRows(cols).AddRow(1, "项目A", nil, "addr", "NJ", nil, nil, now))

	list, err := repo.List(context.TODO())
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 1 || list[0].Name != "项目A" {
		t.Errorf("expected 1 project named 项目A, got %d", len(list))
	}
}

func TestProjectRepoListEmpty(t *testing.T) {
	mock, _ := pgxmock.NewPool()
	defer mock.Close()
	repo := NewProjectRepo(mock)

	cols := []string{"id", "name", "agent_id", "address", "admin_code", "city_id", "city_name", "created_at"}
	mock.ExpectQuery("SELECT (.+) FROM project").
		WillReturnRows(pgxmock.NewRows(cols))

	list, err := repo.List(context.TODO())
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 0 {
		t.Errorf("expected empty, got %d", len(list))
	}
}

func TestProjectRepoGetByID(t *testing.T) {
	mock, _ := pgxmock.NewPool()
	defer mock.Close()
	repo := NewProjectRepo(mock)
	now := time.Now()

	cols := []string{"id", "name", "agent_id", "address", "admin_code", "city_id", "city_name", "created_at"}
	mock.ExpectQuery("SELECT (.+) FROM project WHERE id=\\$1").WithArgs(1).
		WillReturnRows(pgxmock.NewRows(cols).AddRow(1, "项目A", nil, "", "", nil, nil, now))

	p, err := repo.GetByID(context.TODO(), 1)
	if err != nil {
		t.Fatal(err)
	}
	if p.Name != "项目A" {
		t.Errorf("Name = %q, want 项目A", p.Name)
	}
}

func TestProjectRepoGetByIDNotFound(t *testing.T) {
	mock, _ := pgxmock.NewPool()
	defer mock.Close()
	repo := NewProjectRepo(mock)

	mock.ExpectQuery("SELECT (.+) FROM project WHERE id=\\$1").WithArgs(999).
		WillReturnError(pgx.ErrNoRows)

	_, err := repo.GetByID(context.TODO(), 999)
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestProjectRepoCreate(t *testing.T) {
	mock, _ := pgxmock.NewPool()
	defer mock.Close()
	repo := NewProjectRepo(mock)
	now := time.Now()

	cols := []string{"id", "created_at"}
	p := &domain.Project{Name: "新项目", Address: "上海", AdminCode: "NJ"}
	mock.ExpectQuery("INSERT INTO project").
		WithArgs("新项目", pgxmock.AnyArg(), "上海", "NJ", pgxmock.AnyArg(), pgxmock.AnyArg()).
		WillReturnRows(pgxmock.NewRows(cols).AddRow(3, now))

	if err := repo.Create(context.TODO(), p); err != nil {
		t.Fatal(err)
	}
	if p.ID != 3 {
		t.Errorf("ID = %d, want 3", p.ID)
	}
}

func TestProjectRepoUpdate(t *testing.T) {
	mock, _ := pgxmock.NewPool()
	defer mock.Close()
	repo := NewProjectRepo(mock)

	p := &domain.Project{ID: 1, Name: "已更新", AdminCode: "SH"}
	mock.ExpectExec("UPDATE project SET").
		WithArgs("已更新", pgxmock.AnyArg(), "", "SH", pgxmock.AnyArg(), pgxmock.AnyArg(), 1).
		WillReturnResult(pgxmock.NewResult("UPDATE", 1))

	if err := repo.Update(context.TODO(), p); err != nil {
		t.Fatal(err)
	}
}

func TestProjectRepoDelete(t *testing.T) {
	mock, _ := pgxmock.NewPool()
	defer mock.Close()
	repo := NewProjectRepo(mock)

	mock.ExpectExec("DELETE FROM project WHERE id=\\$1").WithArgs(1).
		WillReturnResult(pgxmock.NewResult("DELETE", 1))

	if err := repo.Delete(context.TODO(), 1); err != nil {
		t.Fatal(err)
	}
}

// ---- AdminRepo ----

func TestAdminRepoListUsers(t *testing.T) {
	mock, _ := pgxmock.NewPool()
	defer mock.Close()
	repo := NewAdminRepo(mock)
	now := time.Now()

	cols := []string{"id", "username", "role_id", "code", "name", "agent_id", "agent_name", "default_project_id", "is_active", "last_login_at", "remark", "created_at"}
	mock.ExpectQuery("SELECT (.+) FROM users u JOIN role r").
		WillReturnRows(pgxmock.NewRows(cols).AddRow(1, "admin", 1, "super_admin", "超级管理员", nil, nil, nil, true, nil, nil, now))

	users, err := repo.ListUsers(context.TODO())
	if err != nil {
		t.Fatal(err)
	}
	if len(users) != 1 || users[0].Username != "admin" {
		t.Errorf("expected 1 user admin, got %d", len(users))
	}
}

func TestAdminRepoListUsersEmpty(t *testing.T) {
	mock, _ := pgxmock.NewPool()
	defer mock.Close()
	repo := NewAdminRepo(mock)

	cols := []string{"id", "username", "role_id", "code", "name", "agent_id", "agent_name", "default_project_id", "is_active", "last_login_at", "remark", "created_at"}
	mock.ExpectQuery("SELECT (.+) FROM users u JOIN role r").
		WillReturnRows(pgxmock.NewRows(cols))

	users, err := repo.ListUsers(context.TODO())
	if err != nil {
		t.Fatal(err)
	}
	if len(users) != 0 {
		t.Errorf("expected empty, got %d", len(users))
	}
}

func TestAdminRepoCreateUser(t *testing.T) {
	mock, _ := pgxmock.NewPool()
	defer mock.Close()
	repo := NewAdminRepo(mock)
	now := time.Now()

	cols := []string{"id", "created_at"}
	req := domain.CreateUserReq{Username: "newuser", RoleID: 2}
	mock.ExpectQuery("INSERT INTO users").
		WithArgs("newuser", "hashed", 2, pgxmock.AnyArg(), pgxmock.AnyArg()).
		WillReturnRows(pgxmock.NewRows(cols).AddRow(5, now))

	u, err := repo.CreateUser(context.TODO(), req, "hashed")
	if err != nil {
		t.Fatal(err)
	}
	if u.ID != 5 || u.Username != "newuser" {
		t.Errorf("unexpected: ID=%d Username=%q", u.ID, u.Username)
	}
}

func TestAdminRepoUpdateUser(t *testing.T) {
	mock, _ := pgxmock.NewPool()
	defer mock.Close()
	repo := NewAdminRepo(mock)

	agentID := 2
	remark := "备注"
	mock.ExpectExec("UPDATE users SET").
		WithArgs(3, &agentID, true, &remark, 1).
		WillReturnResult(pgxmock.NewResult("UPDATE", 1))

	if err := repo.UpdateUser(context.TODO(), 1, 3, &agentID, true, &remark); err != nil {
		t.Fatal(err)
	}
}

func TestAdminRepoUpdateUserMinimal(t *testing.T) {
	mock, _ := pgxmock.NewPool()
	defer mock.Close()
	repo := NewAdminRepo(mock)

	// roleID=0 means "keep current" (CASE WHEN $1=0)
	mock.ExpectExec("UPDATE users SET").
		WithArgs(0, pgxmock.AnyArg(), true, pgxmock.AnyArg(), 1).
		WillReturnResult(pgxmock.NewResult("UPDATE", 1))

	if err := repo.UpdateUser(context.TODO(), 1, 0, nil, true, nil); err != nil {
		t.Fatal(err)
	}
}

func TestAdminRepoResetPassword(t *testing.T) {
	mock, _ := pgxmock.NewPool()
	defer mock.Close()
	repo := NewAdminRepo(mock)

	mock.ExpectExec("UPDATE users SET password_hash=\\$1").WithArgs("newhash", 1).
		WillReturnResult(pgxmock.NewResult("UPDATE", 1))

	if err := repo.ResetPassword(context.TODO(), 1, "newhash"); err != nil {
		t.Fatal(err)
	}
}

func TestAdminRepoDeleteUser(t *testing.T) {
	mock, _ := pgxmock.NewPool()
	defer mock.Close()
	repo := NewAdminRepo(mock)

	mock.ExpectExec("DELETE FROM users WHERE id=\\$1").WithArgs(1).
		WillReturnResult(pgxmock.NewResult("DELETE", 1))

	if err := repo.DeleteUser(context.TODO(), 1); err != nil {
		t.Fatal(err)
	}
}

func TestAdminRepoListAgents(t *testing.T) {
	mock, _ := pgxmock.NewPool()
	defer mock.Close()
	repo := NewAdminRepo(mock)
	now := time.Now()

	cols := []string{"id", "name", "created_at"}
	mock.ExpectQuery("SELECT (.+) FROM agent").
		WillReturnRows(pgxmock.NewRows(cols).AddRow(1, "代理商A", now))

	list, err := repo.ListAgents(context.TODO())
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 1 || list[0].Name != "代理商A" {
		t.Errorf("expected 1 agent 代理商A, got %d", len(list))
	}
}

func TestAdminRepoCreateAgent(t *testing.T) {
	mock, _ := pgxmock.NewPool()
	defer mock.Close()
	repo := NewAdminRepo(mock)
	now := time.Now()

	cols := []string{"id", "created_at"}
	mock.ExpectQuery("INSERT INTO agent").WithArgs("新代理商").
		WillReturnRows(pgxmock.NewRows(cols).AddRow(3, now))

	a, err := repo.CreateAgent(context.TODO(), "新代理商")
	if err != nil {
		t.Fatal(err)
	}
	if a.ID != 3 || a.Name != "新代理商" {
		t.Errorf("unexpected: ID=%d Name=%q", a.ID, a.Name)
	}
}

func TestAdminRepoUpdateAgent(t *testing.T) {
	mock, _ := pgxmock.NewPool()
	defer mock.Close()
	repo := NewAdminRepo(mock)

	mock.ExpectExec("UPDATE agent SET").WithArgs("已更名", 1).
		WillReturnResult(pgxmock.NewResult("UPDATE", 1))

	if err := repo.UpdateAgent(context.TODO(), 1, "已更名"); err != nil {
		t.Fatal(err)
	}
}

func TestAdminRepoDeleteAgent(t *testing.T) {
	mock, _ := pgxmock.NewPool()
	defer mock.Close()
	repo := NewAdminRepo(mock)

	mock.ExpectExec("DELETE FROM agent WHERE id=\\$1").WithArgs(1).
		WillReturnResult(pgxmock.NewResult("DELETE", 1))

	if err := repo.DeleteAgent(context.TODO(), 1); err != nil {
		t.Fatal(err)
	}
}

func TestAdminRepoListRoles(t *testing.T) {
	mock, _ := pgxmock.NewPool()
	defer mock.Close()
	repo := NewAdminRepo(mock)

	cols := []string{"id", "code", "name", "level", "is_system"}
	mock.ExpectQuery("SELECT (.+) FROM role").
		WillReturnRows(pgxmock.NewRows(cols).AddRow(1, "super_admin", "超级管理员", 1, true))

	list, err := repo.ListRoles(context.TODO())
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 1 || list[0].Code != "super_admin" {
		t.Errorf("expected 1 role super_admin, got %d", len(list))
	}
}

func TestAdminRepoListPermissions(t *testing.T) {
	mock, _ := pgxmock.NewPool()
	defer mock.Close()
	repo := NewAdminRepo(mock)

	cols := []string{"id", "code", "name", "perm_group"}
	mock.ExpectQuery("SELECT (.+) FROM permission").
		WillReturnRows(pgxmock.NewRows(cols).AddRow(1, "monitor.realtime", "实时监测", "monitor"))

	list, err := repo.ListPermissions(context.TODO())
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 1 || list[0].Code != "monitor.realtime" {
		t.Errorf("expected 1 permission monitor.realtime, got %d", len(list))
	}
}

func TestAdminRepoListRolePermissions(t *testing.T) {
	mock, _ := pgxmock.NewPool()
	defer mock.Close()
	repo := NewAdminRepo(mock)

	cols := []string{"perm_id"}
	mock.ExpectQuery("SELECT perm_id FROM role_permission").WithArgs(1).
		WillReturnRows(pgxmock.NewRows(cols).AddRow(1).AddRow(2).AddRow(3))

	ids, err := repo.ListRolePermissions(context.TODO(), 1)
	if err != nil {
		t.Fatal(err)
	}
	if len(ids) != 3 {
		t.Errorf("expected 3 perm IDs, got %d", len(ids))
	}
}

func TestAdminRepoSetRolePermissions(t *testing.T) {
	mock, _ := pgxmock.NewPool()
	defer mock.Close()
	repo := NewAdminRepo(mock)

	mock.ExpectBegin()
	mock.ExpectExec("DELETE FROM role_permission").WithArgs(1).
		WillReturnResult(pgxmock.NewResult("DELETE", 3))
	mock.ExpectExec("INSERT INTO role_permission").WithArgs(1, 10).
		WillReturnResult(pgxmock.NewResult("INSERT", 1))
	mock.ExpectExec("INSERT INTO role_permission").WithArgs(1, 20).
		WillReturnResult(pgxmock.NewResult("INSERT", 1))
	mock.ExpectCommit()

	if err := repo.SetRolePermissions(context.TODO(), 1, []int{10, 20}); err != nil {
		t.Fatal(err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet: %v", err)
	}
}

func TestAdminRepoSetRolePermissionsTxError(t *testing.T) {
	mock, _ := pgxmock.NewPool()
	defer mock.Close()
	repo := NewAdminRepo(mock)

	mock.ExpectBegin()
	mock.ExpectExec("DELETE FROM role_permission").WithArgs(1).
		WillReturnError(errors.New("tx failure"))
	mock.ExpectRollback()

	err := repo.SetRolePermissions(context.TODO(), 1, []int{10})
	if err == nil {
		t.Fatal("expected error from DB failure")
	}
}

func TestAdminRepoSystemInfo(t *testing.T) {
	mock, _ := pgxmock.NewPool()
	defer mock.Close()
	repo := NewAdminRepo(mock)

	mock.ExpectQuery("SELECT version\\(\\)").
		WillReturnRows(pgxmock.NewRows([]string{"version"}).AddRow("PostgreSQL 16.0"))
	mock.ExpectQuery("SELECT count\\(\\*\\) FROM schema_migrations").
		WillReturnRows(pgxmock.NewRows([]string{"count"}).AddRow(12))
	mock.ExpectQuery("SELECT to_char\\(NOW\\(\\).*").
		WillReturnRows(pgxmock.NewRows([]string{"to_char"}).AddRow("2026-06-26 12:00:00"))

	info, err := repo.SystemInfo(context.TODO())
	if err != nil {
		t.Fatal(err)
	}
	if info["service"] != "XMECO" {
		t.Errorf("service = %v", info["service"])
	}
	if info["db_version"] != "PostgreSQL 16.0" {
		t.Errorf("db_version = %v", info["db_version"])
	}
	if info["status"] != "running" {
		t.Errorf("status = %v", info["status"])
	}
	if info["migrations"] != 12 {
		t.Errorf("migrations = %v", info["migrations"])
	}
}
