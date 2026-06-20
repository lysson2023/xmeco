package postgres

import (
	"context"

	"xmeco/internal/domain"

	"github.com/jackc/pgx/v5/pgxpool"
)

type AdminRepo struct {
	pool *pgxpool.Pool
}

func NewAdminRepo(pool *pgxpool.Pool) *AdminRepo {
	return &AdminRepo{pool: pool}
}

// ==================== 用户 ====================

func (r *AdminRepo) ListUsers(ctx context.Context) ([]domain.AdminUser, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT u.id, u.username, u.role_id, r.code, r.name,
		       u.agent_id, a.name, u.default_project_id,
		       u.is_active, u.last_login_at, u.remark, u.created_at
		FROM users u
		JOIN role r ON r.id = u.role_id
		LEFT JOIN agent a ON a.id = u.agent_id
		ORDER BY u.id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var users []domain.AdminUser
	for rows.Next() {
		var u domain.AdminUser
		if err := rows.Scan(&u.ID, &u.Username, &u.RoleID, &u.RoleCode, &u.RoleName,
			&u.AgentID, &u.AgentName, &u.DefaultProjectID,
			&u.IsActive, &u.LastLoginAt, &u.Remark, &u.CreatedAt); err != nil {
			return nil, err
		}
		users = append(users, u)
	}
	return users, rows.Err()
}

func (r *AdminRepo) CreateUser(ctx context.Context, req domain.CreateUserReq, passwordHash string) (*domain.AdminUser, error) {
	var u domain.AdminUser
	err := r.pool.QueryRow(ctx, `
		INSERT INTO users (username, password_hash, role_id, agent_id, default_project_id)
		VALUES ($1,$2,$3,$4,$5)
		RETURNING id, created_at`,
		req.Username, passwordHash, req.RoleID, req.AgentID, req.DefaultProjectID,
	).Scan(&u.ID, &u.CreatedAt)
	if err != nil {
		return nil, err
	}
	u.Username = req.Username
	u.RoleID = req.RoleID
	u.AgentID = req.AgentID
	u.DefaultProjectID = req.DefaultProjectID
	u.IsActive = true
	return &u, nil
}

func (r *AdminRepo) UpdateUser(ctx context.Context, id int, roleID int, agentID *int, isActive bool) error {
	_, err := r.pool.Exec(ctx, `
		UPDATE users SET role_id=$1, agent_id=$2, is_active=$3 WHERE id=$4`,
		roleID, agentID, isActive, id)
	return err
}

func (r *AdminRepo) ResetPassword(ctx context.Context, id int, passwordHash string) error {
	_, err := r.pool.Exec(ctx, `UPDATE users SET password_hash=$1 WHERE id=$2`, passwordHash, id)
	return err
}

func (r *AdminRepo) DeleteUser(ctx context.Context, id int) error {
	_, err := r.pool.Exec(ctx, `DELETE FROM users WHERE id=$1`, id)
	return err
}

// ==================== 代理商 ====================

func (r *AdminRepo) ListAgents(ctx context.Context) ([]domain.Agent, error) {
	rows, err := r.pool.Query(ctx, `SELECT id, name, created_at FROM agent ORDER BY id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var agents []domain.Agent
	for rows.Next() {
		var a domain.Agent
		if err := rows.Scan(&a.ID, &a.Name, &a.CreatedAt); err != nil {
			return nil, err
		}
		agents = append(agents, a)
	}
	return agents, rows.Err()
}

func (r *AdminRepo) CreateAgent(ctx context.Context, name string) (*domain.Agent, error) {
	var a domain.Agent
	err := r.pool.QueryRow(ctx, `INSERT INTO agent (name) VALUES ($1) RETURNING id, created_at`, name).
		Scan(&a.ID, &a.CreatedAt)
	if err != nil {
		return nil, err
	}
	a.Name = name
	return &a, nil
}

func (r *AdminRepo) UpdateAgent(ctx context.Context, id int, name string) error {
	_, err := r.pool.Exec(ctx, `UPDATE agent SET name=$1 WHERE id=$2`, name, id)
	return err
}

func (r *AdminRepo) DeleteAgent(ctx context.Context, id int) error {
	_, err := r.pool.Exec(ctx, `DELETE FROM agent WHERE id=$1`, id)
	return err
}

// ==================== 角色 & 权限 ====================

func (r *AdminRepo) ListRoles(ctx context.Context) ([]domain.Role, error) {
	rows, err := r.pool.Query(ctx, `SELECT id, code, name, level, is_system FROM role ORDER BY level`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var roles []domain.Role
	for rows.Next() {
		var ro domain.Role
		if err := rows.Scan(&ro.ID, &ro.Code, &ro.Name, &ro.Level, &ro.IsSystem); err != nil {
			return nil, err
		}
		roles = append(roles, ro)
	}
	return roles, rows.Err()
}

func (r *AdminRepo) ListPermissions(ctx context.Context) ([]domain.Permission, error) {
	rows, err := r.pool.Query(ctx, `SELECT id, code, name, perm_group FROM permission ORDER BY perm_group, id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var perms []domain.Permission
	for rows.Next() {
		var p domain.Permission
		if err := rows.Scan(&p.ID, &p.Code, &p.Name, &p.PermGroup); err != nil {
			return nil, err
		}
		perms = append(perms, p)
	}
	return perms, rows.Err()
}

func (r *AdminRepo) ListRolePermissions(ctx context.Context, roleID int) ([]int, error) {
	rows, err := r.pool.Query(ctx, `SELECT perm_id FROM role_permission WHERE role_id=$1`, roleID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var ids []int
	for rows.Next() {
		var id int
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

func (r *AdminRepo) SetRolePermissions(ctx context.Context, roleID int, permIDs []int) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if _, err := tx.Exec(ctx, `DELETE FROM role_permission WHERE role_id=$1`, roleID); err != nil {
		return err
	}
	for _, pid := range permIDs {
		if _, err := tx.Exec(ctx, `INSERT INTO role_permission (role_id, perm_id) VALUES ($1,$2) ON CONFLICT DO NOTHING`, roleID, pid); err != nil {
			return err
		}
	}
	return tx.Commit(ctx)
}