package postgres

import (
	"context"
	"xmeco/internal/domain"
	"github.com/jackc/pgx/v5/pgxpool"
)

type ProjectRepo struct {
	pool *pgxpool.Pool
}

func NewProjectRepo(pool *pgxpool.Pool) *ProjectRepo {
	return &ProjectRepo{pool: pool}
}

func (r *ProjectRepo) List(ctx context.Context) ([]domain.Project, error) {
	rows, err := r.pool.Query(ctx, `SELECT id, name, agent_id, address, COALESCE(admin_code,''), created_at FROM project ORDER BY id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var projects []domain.Project
	for rows.Next() {
		var p domain.Project
		if err := rows.Scan(&p.ID, &p.Name, &p.AgentID, &p.Address, &p.AdminCode, &p.CreatedAt); err != nil {
			return nil, err
		}
		projects = append(projects, p)
	}
	return projects, rows.Err()
}

func (r *ProjectRepo) GetByID(ctx context.Context, id int) (*domain.Project, error) {
	var p domain.Project
	err := r.pool.QueryRow(ctx, `SELECT id, name, agent_id, address, COALESCE(admin_code,''), created_at FROM project WHERE id=$1`, id).
		Scan(&p.ID, &p.Name, &p.AgentID, &p.Address, &p.AdminCode, &p.CreatedAt)
	if err != nil {
		return nil, err
	}
	return &p, nil
}

func (r *ProjectRepo) Create(ctx context.Context, p *domain.Project) error {
	return r.pool.QueryRow(ctx,
		`INSERT INTO project (name, agent_id, address, admin_code) VALUES ($1,$2,$3,$4) RETURNING id, created_at`,
		p.Name, p.AgentID, p.Address, p.AdminCode).Scan(&p.ID, &p.CreatedAt)
}

func (r *ProjectRepo) Update(ctx context.Context, p *domain.Project) error {
	_, err := r.pool.Exec(ctx,
		`UPDATE project SET name=$1, agent_id=$2, address=$3, admin_code=$4 WHERE id=$5`,
		p.Name, p.AgentID, p.Address, p.AdminCode, p.ID)
	return err
}

func (r *ProjectRepo) Delete(ctx context.Context, id int) error {
	_, err := r.pool.Exec(ctx, `DELETE FROM project WHERE id=$1`, id)
	return err
}