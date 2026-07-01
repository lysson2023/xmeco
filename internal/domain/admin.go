package domain

import "time"

type Agent struct {
	ID        int       `json:"id"`
	Name      string    `json:"name"`
	CreatedAt time.Time `json:"created_at"`
}

type Role struct {
	ID       int    `json:"id"`
	Code     string `json:"code"`
	Name     string `json:"name"`
	Level    int    `json:"level"`
	IsSystem bool   `json:"is_system"`
}

type Permission struct {
	ID        int    `json:"id"`
	Code      string `json:"code"`
	Name      string `json:"name"`
	PermGroup string `json:"perm_group"`
}

type AdminUser struct {
	ID               int        `json:"id"`
	Username         string     `json:"username"`
	RoleID           int        `json:"role_id"`
	RoleCode         string     `json:"role_code"`
	RoleName         string     `json:"role_name"`
	AgentID          *int       `json:"agent_id"`
	AgentName        *string    `json:"agent_name"`
	DefaultProjectID *int       `json:"default_project_id"`
	IsActive         bool       `json:"is_active"`
	LastLoginAt      *time.Time `json:"last_login_at"`
	CreatedAt        time.Time  `json:"created_at"`
	Remark           *string    `json:"remark"`
}

type CreateUserReq struct {
	Username         string  `json:"username"`
	Password         string  `json:"password"`
	RoleID           int     `json:"role_id"`
	AgentID          *int    `json:"agent_id"`
	DefaultProjectID *int    `json:"default_project_id"`
	Remark           *string `json:"remark"`
}
