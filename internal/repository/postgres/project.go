package postgres

import (
	"context"
	"errors"
	"log/slog"

	"github.com/jackc/pgx/v5"

	"xmeco/internal/domain"
)

type ProjectRepo struct {
	pool DBTX
}

func NewProjectRepo(pool DBTX) *ProjectRepo {
	return &ProjectRepo{pool: pool}
}

func (r *ProjectRepo) List(ctx context.Context) ([]domain.Project, error) {
	rows, err := r.pool.Query(ctx, `SELECT id, COALESCE(name,''), agent_id, COALESCE(address,''), COALESCE(admin_code,''), city_id, COALESCE(city_name,''), created_at FROM project ORDER BY id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var projects []domain.Project
	for rows.Next() {
		var p domain.Project
		if err := rows.Scan(&p.ID, &p.Name, &p.AgentID, &p.Address, &p.AdminCode, &p.CityID, &p.CityName, &p.CreatedAt); err != nil {
			slog.Warn("ProjectRepo.List scan failed", "err", err)
			continue
		}
		projects = append(projects, p)
	}
	return projects, rows.Err()
}

func (r *ProjectRepo) GetByID(ctx context.Context, id int) (*domain.Project, error) {
	var p domain.Project
	err := r.pool.QueryRow(ctx, `SELECT id, COALESCE(name,''), agent_id, COALESCE(address,''), COALESCE(admin_code,''), city_id, COALESCE(city_name,''), created_at FROM project WHERE id=$1`, id).
		Scan(&p.ID, &p.Name, &p.AgentID, &p.Address, &p.AdminCode, &p.CityID, &p.CityName, &p.CreatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return &p, nil
}

func (r *ProjectRepo) Create(ctx context.Context, p *domain.Project) error {
	return r.pool.QueryRow(ctx,
		`INSERT INTO project (name, agent_id, address, admin_code, city_id, city_name) VALUES ($1,$2,$3,$4,$5,$6) RETURNING id, created_at`,
		p.Name, p.AgentID, p.Address, p.AdminCode, p.CityID, p.CityName).Scan(&p.ID, &p.CreatedAt)
}

func (r *ProjectRepo) Update(ctx context.Context, p *domain.Project) error {
	_, err := r.pool.Exec(ctx,
		`UPDATE project SET name=$1, agent_id=$2, address=$3, admin_code=$4, city_id=$5, city_name=$6 WHERE id=$7`,
		p.Name, p.AgentID, p.Address, p.AdminCode, p.CityID, p.CityName, p.ID)
	return err
}

func (r *ProjectRepo) Delete(ctx context.Context, id int) error {
	_, err := r.pool.Exec(ctx, `DELETE FROM project WHERE id=$1`, id)
	return err
}

// ListProjectUsers returns user IDs assigned to a project.
func (r *ProjectRepo) ListProjectUsers(ctx context.Context, projectID int) ([]int, error) {
	rows, err := r.pool.Query(ctx, `SELECT user_id FROM project_user WHERE project_id=$1`, projectID)
	if err != nil { return nil, err }
	defer rows.Close()
	var ids []int
	for rows.Next() {
		var uid int
		if err := rows.Scan(&uid); err != nil { slog.Warn("ListProjectUsers scan failed", "err", err); continue }
		ids = append(ids, uid)
	}
	if ids == nil { ids = []int{} }
	return ids, rows.Err()
}

// SetProjectUsers replaces assigned user IDs for a project.
func (r *ProjectRepo) SetProjectUsers(ctx context.Context, projectID int, userIDs []int) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil { return err }
	defer tx.Rollback(ctx)
	if _, err := tx.Exec(ctx, `DELETE FROM project_user WHERE project_id=$1`, projectID); err != nil { return err }
	for _, uid := range userIDs {
		if _, err := tx.Exec(ctx, `INSERT INTO project_user (project_id,user_id) VALUES($1,$2) ON CONFLICT DO NOTHING`, projectID, uid); err != nil { return err }
	}
	return tx.Commit(ctx)
}