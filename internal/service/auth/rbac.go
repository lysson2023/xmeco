package auth

import (
	"context"
	"errors"
)

// RoleSuperAdmin is the system role code for the super administrator.
const RoleSuperAdmin = "super_admin"

// RoleAdmin is the platform administrator role code (level 10).
const RoleAdmin = "admin"

// Perm groups all permission code constants. Use auth.Perm.ProjectView etc.
var Perm = struct {
	ProjectView     string
	ProjectCreate   string
	ProjectEdit     string
	ProjectDelete   string
	BuildingView    string
	BuildingCreate  string
	BuildingEdit    string
	BuildingDelete  string
	DeviceView      string
	DeviceCreate    string
	DeviceEdit      string
	DeviceDelete    string
	DeviceControl   string
	DeviceProperty  string
	DeviceRegister  string
	MonitorRealtime string
	MonitorGraph    string
	MonitorAlarmCfg string
	MonitorCtrlLog  string
	ScheduleView    string
	ScheduleCreate  string
	ScheduleEdit    string
	ScheduleDelete  string
	UserView        string
	UserCreate      string
	UserEdit        string
	UserDelete      string
	UserAssignRole  string
	AgentView       string
	AgentCreate     string
	AgentEdit       string
	AgentDelete     string
	ReportExport    string
	ReportExcel     string
	SystemConfig    string
	SystemGateway   string
	SystemDB        string
}{
	ProjectView:     "project.view",
	ProjectCreate:   "project.create",
	ProjectEdit:     "project.edit",
	ProjectDelete:   "project.delete",
	BuildingView:    "building.view",
	BuildingCreate:  "building.create",
	BuildingEdit:    "building.edit",
	BuildingDelete:  "building.delete",
	DeviceView:      "device.view",
	DeviceCreate:    "device.create",
	DeviceEdit:      "device.edit",
	DeviceDelete:    "device.delete",
	DeviceControl:   "device.control",
	DeviceProperty:  "device.property",
	DeviceRegister:  "device.register",
	MonitorRealtime: "monitor.realtime",
	MonitorGraph:    "monitor.graph",
	MonitorAlarmCfg: "monitor.alarm_config",
	MonitorCtrlLog:  "monitor.control_log",
	ScheduleView:    "schedule.view",
	ScheduleCreate:  "schedule.create",
	ScheduleEdit:    "schedule.edit",
	ScheduleDelete:  "schedule.delete",
	UserView:        "user.view",
	UserCreate:      "user.create",
	UserEdit:        "user.edit",
	UserDelete:      "user.delete",
	UserAssignRole:  "user.assign_role",
	AgentView:       "agent.view",
	AgentCreate:     "agent.create",
	AgentEdit:       "agent.edit",
	AgentDelete:     "agent.delete",
	ReportExport:    "report.export",
	ReportExcel:     "report.excel",
	SystemConfig:    "system.config",
	SystemGateway:   "system.gateway",
	SystemDB:        "system.db",
}

var ErrPermissionDenied = errors.New("无此操作权限")

// HasPermission 检查用户是否有指定权限点。
// 返回 (有权限, error)。error != nil 表示数据库异常——调用方应返回 503 而非 403。
func (s *Service) HasPermission(ctx context.Context, userID int, permCode string) (bool, error) {
	var count int
	err := s.pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM permission p
		 JOIN role_permission rp ON rp.perm_id = p.id
		 JOIN users u ON u.role_id = rp.role_id
		 WHERE u.id = $1 AND p.code = $2`, userID, permCode,
	).Scan(&count)
	if err != nil {
		return false, err
	}
	return count > 0, nil
}
