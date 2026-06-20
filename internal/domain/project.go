package domain

import "time"

type Project struct {
	ID        int       `json:"id"`
	Name      string    `json:"name"`
	AgentID   *int      `json:"agent_id"`
	Address   string    `json:"address"`
	AdminCode string    `json:"admin_code"`
	CreatedAt time.Time `json:"created_at"`
}