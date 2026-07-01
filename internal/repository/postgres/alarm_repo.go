package postgres

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"github.com/jackc/pgx/v5"
)

// AlarmRuleRepo 封装 alarm_rule 表的 CRUD 操作
type AlarmRuleRepo struct{ pool DBTX }

func NewAlarmRuleRepo(pool DBTX) *AlarmRuleRepo { return &AlarmRuleRepo{pool} }

// AlarmRule 数据结构（从Handler提取）
type AlarmRule struct {
	ID          int      `json:"id"`
	Name        string   `json:"name"`
	DeviceID    *int     `json:"device_id"`
	PropertyID  *int     `json:"property_id"`
	DeviceType  *string  `json:"device_type"`
	Metric      *string  `json:"metric"`
	Condition   *string  `json:"condition"`
	Threshold   *float64 `json:"threshold"`
	Level       *string  `json:"level"`
	TargetValue *string  `json:"target_value"`
	MinValue    *string  `json:"min_value"`
	MaxValue    *string  `json:"max_value"`
	NotifyUsers []int    `json:"notify_users"`
	Enabled     bool     `json:"enabled"`
}

// List 返回告警规则列表，可选按 device_id 过滤
func (r *AlarmRuleRepo) List(ctx context.Context, deviceID int) ([]AlarmRule, error) {
	var rows pgx.Rows
	var err error
	if deviceID > 0 {
		rows, err = r.pool.Query(ctx,
			`SELECT id,name,device_id,property_id,device_type,metric,condition,threshold,level,target_value,min_value,max_value,notify_users,enabled
			 FROM alarm_rule WHERE device_id=$1 ORDER BY id`, deviceID)
	} else {
		rows, err = r.pool.Query(ctx,
			`SELECT id,name,device_id,property_id,device_type,metric,condition,threshold,level,target_value,min_value,max_value,notify_users,enabled
			 FROM alarm_rule ORDER BY id`)
	}
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var list []AlarmRule
	for rows.Next() {
		var rr AlarmRule
		if err := rows.Scan(&rr.ID, &rr.Name, &rr.DeviceID, &rr.PropertyID, &rr.DeviceType,
			&rr.Metric, &rr.Condition, &rr.Threshold, &rr.Level, &rr.TargetValue,
			&rr.MinValue, &rr.MaxValue, &rr.NotifyUsers, &rr.Enabled); err != nil {
			slog.Warn("AlarmRuleRepo.List scan failed", "err", err)
			continue
		}
		list = append(list, rr)
	}
	return list, rows.Err()
}

// GetByID 返回单个告警规则，未找到返回 nil, nil
func (r *AlarmRuleRepo) GetByID(ctx context.Context, id int) (*AlarmRule, error) {
	var rr AlarmRule
	err := r.pool.QueryRow(ctx,
		`SELECT id,name,device_id,property_id,device_type,metric,condition,threshold,level,target_value,min_value,max_value,notify_users,enabled
		 FROM alarm_rule WHERE id=$1`, id).
		Scan(&rr.ID, &rr.Name, &rr.DeviceID, &rr.PropertyID, &rr.DeviceType,
			&rr.Metric, &rr.Condition, &rr.Threshold, &rr.Level, &rr.TargetValue,
			&rr.MinValue, &rr.MaxValue, &rr.NotifyUsers, &rr.Enabled)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return &rr, nil
}

// Create 创建告警规则（使用传入的enabled值）
func (r *AlarmRuleRepo) Create(ctx context.Context, rr *AlarmRule) error {
	return r.CreateWithEnabled(ctx, rr, rr.Enabled)
}

// CreateWithEnabled 创建告警规则，显式传入enabled值
func (r *AlarmRuleRepo) CreateWithEnabled(ctx context.Context, rr *AlarmRule, enabled bool) error {
	return r.pool.QueryRow(ctx,
		`INSERT INTO alarm_rule (name,device_id,property_id,device_type,metric,condition,threshold,level,target_value,min_value,max_value,notify_users,enabled)
		 VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13) RETURNING id`,
		rr.Name, rr.DeviceID, rr.PropertyID, rr.DeviceType, rr.Metric, rr.Condition,
		rr.Threshold, rr.Level, rr.TargetValue, rr.MinValue, rr.MaxValue, rr.NotifyUsers, enabled).Scan(&rr.ID)
}

// Update 更新告警规则
func (r *AlarmRuleRepo) Update(ctx context.Context, rr *AlarmRule) error {
	enabled := rr.Enabled // 直接使用传入的值
	_, err := r.pool.Exec(ctx,
		`UPDATE alarm_rule SET name=$1,device_id=$2,property_id=$3,device_type=$4,metric=$5,condition=$6,threshold=$7,level=$8,target_value=$9,min_value=$10,max_value=$11,notify_users=$12,enabled=$13 WHERE id=$14`,
		rr.Name, rr.DeviceID, rr.PropertyID, rr.DeviceType, rr.Metric, rr.Condition,
		rr.Threshold, rr.Level, rr.TargetValue, rr.MinValue, rr.MaxValue, rr.NotifyUsers, enabled, rr.ID)
	return err
}

// Delete 删除告警规则
func (r *AlarmRuleRepo) Delete(ctx context.Context, id int) error {
	_, err := r.pool.Exec(ctx, "DELETE FROM alarm_rule WHERE id=$1", id)
	return err
}

// ===== AlarmLogRepo =====

// AlarmLogRepo 封装 alarm_log 表的查询和更新操作
type AlarmLogRepo struct{ pool DBTX }

func NewAlarmLogRepo(pool DBTX) *AlarmLogRepo { return &AlarmLogRepo{pool} }

// AlarmLog 数据结构
type AlarmLog struct {
	ID         int     `json:"id"`
	DeviceID   int     `json:"device_id"`
	DeviceName string  `json:"device_name"`
	AlarmType  string  `json:"alarm_type"`
	Level      string  `json:"level"`
	Message    string  `json:"message"`
	Value      string  `json:"value"`
	Threshold  string  `json:"threshold"`
	CreatedAt  *string `json:"created_at"`
	AckAt      *string `json:"ack_at"`
}

// ListFilter 查询过滤条件
type AlarmLogFilter struct {
	DeviceID   int
	BuildingID int
	ProjectID  int
	DateFrom   string
	DateTo     string
	Today      bool // "1" = today only
}

// List 返回告警日志列表，支持多条件过滤
func (r *AlarmLogRepo) List(ctx context.Context, filter AlarmLogFilter) ([]AlarmLog, error) {
	baseQ := `SELECT al.id, al.device_id, al.device_name, al.alarm_type, al.level,
		al.message, al.value, al.threshold,
		COALESCE(al.created_at::text,''), COALESCE(al.ack_at::text,'')
		FROM alarm_log al`
	var conditions []string
	var args []any
	argIdx := 1

	if filter.BuildingID > 0 || filter.ProjectID > 0 {
		baseQ += ` JOIN device d ON d.id = al.device_id`
		if filter.ProjectID > 0 {
			baseQ += ` JOIN building b ON b.id = d.building_id`
		}
	}

	if filter.DeviceID > 0 {
		conditions = append(conditions, fmt.Sprintf("al.device_id=$%d", argIdx))
		args = append(args, filter.DeviceID)
		argIdx++
	}
	if filter.BuildingID > 0 {
		conditions = append(conditions, fmt.Sprintf("d.building_id=$%d", argIdx))
		args = append(args, filter.BuildingID)
		argIdx++
	}
	if filter.ProjectID > 0 {
		conditions = append(conditions, fmt.Sprintf("b.project_id=$%d", argIdx))
		args = append(args, filter.ProjectID)
		argIdx++
	}
	if filter.Today {
		conditions = append(conditions, "al.created_at::date = CURRENT_DATE")
	}
	if filter.DateFrom != "" {
		conditions = append(conditions, fmt.Sprintf("al.created_at >= $%d::timestamp", argIdx))
		args = append(args, filter.DateFrom)
		argIdx++
	}
	if filter.DateTo != "" {
		conditions = append(conditions, fmt.Sprintf("al.created_at <= $%d::timestamp", argIdx))
		args = append(args, filter.DateTo)
		argIdx++
	}

	if len(conditions) > 0 {
		baseQ += " WHERE " + conditions[0]
		for i := 1; i < len(conditions); i++ {
			baseQ += " AND " + conditions[i]
		}
	}
	baseQ += " ORDER BY al.created_at DESC LIMIT 200"

	rows, err := r.pool.Query(ctx, baseQ, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var list []AlarmLog
	for rows.Next() {
		var l AlarmLog
		if err := rows.Scan(&l.ID, &l.DeviceID, &l.DeviceName, &l.AlarmType, &l.Level,
			&l.Message, &l.Value, &l.Threshold, &l.CreatedAt, &l.AckAt); err != nil {
			slog.Warn("AlarmLogRepo.List scan failed", "err", err)
			continue
		}
		if l.CreatedAt != nil && *l.CreatedAt == "" {
			l.CreatedAt = nil
		}
		if l.AckAt != nil && *l.AckAt == "" {
			l.AckAt = nil
		}
		list = append(list, l)
	}
	return list, rows.Err()
}

// Ack 确认告警日志
func (r *AlarmLogRepo) Ack(ctx context.Context, id int, ackBy string) error {
	_, err := r.pool.Exec(ctx, "UPDATE alarm_log SET ack_by=$1, ack_at=NOW() WHERE id=$2", ackBy, id)
	return err
}