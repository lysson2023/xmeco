package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"

	"github.com/jackc/pgx/v5"
)

// StartupRepo 封装 startup_plan/step/execution/scheduled_task 表的操作
type StartupRepo struct{ pool DBTX }

func NewStartupRepo(pool DBTX) *StartupRepo { return &StartupRepo{pool} }

// ===== StartupPlan =====

// StartupPlan 启停计划
type StartupPlan struct {
	ID             int             `json:"id"`
	Name           string          `json:"name"`
	BuildingID     int             `json:"building_id"`
	PlanType       string          `json:"plan_type"`
	PrecheckOnline bool            `json:"precheck_online"`
	PrecheckAlarm  bool            `json:"precheck_alarm"`
	Enabled        bool            `json:"enabled"`
	Steps          json.RawMessage `json:"steps"`
}

// ListPlans 返回启停计划列表
func (r *StartupRepo) ListPlans(ctx context.Context, buildingID int) ([]StartupPlan, error) {
	var rows pgx.Rows
	var err error
	query := `SELECT p.id,p.name,p.building_id,p.plan_type,p.precheck_online,p.precheck_alarm,p.enabled,
		COALESCE((SELECT json_agg(json_build_object('id',s.id,'device_id',s.device_id,'device_name',d.name,'sort_order',s.sort_order,'wait_seconds',s.wait_seconds,'retry_count',COALESCE(s.retry_count,1),'action',s.action) ORDER BY s.sort_order)
		FROM startup_step s JOIN device d ON d.id=s.device_id WHERE s.plan_id=p.id),'[]'::json)
		FROM startup_plan p`
	if buildingID > 0 {
		query += " WHERE ($1=0 OR p.building_id=$1)"
		rows, err = r.pool.Query(ctx, query, buildingID)
	} else {
		query += " ORDER BY p.id"
		rows, err = r.pool.Query(ctx, query)
	}
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var list []StartupPlan
	for rows.Next() {
		var p StartupPlan
		if err := rows.Scan(&p.ID, &p.Name, &p.BuildingID, &p.PlanType, &p.PrecheckOnline, &p.PrecheckAlarm, &p.Enabled, &p.Steps); err != nil {
			slog.Warn("StartupRepo.ListPlans scan failed", "err", err)
			continue
		}
		list = append(list, p)
	}
	return list, rows.Err()
}

// PlanStep 计划步骤
type PlanStep struct {
	DeviceID    int `json:"device_id"`
	DeviceName  string `json:"device_name"`
	SortOrder   int `json:"sort_order"`
	WaitSeconds int `json:"wait_seconds"`
	RetryCount  int `json:"retry_count"`
	Action      string `json:"action"`
}

// CreatePlanReq 创建计划请求
type CreatePlanReq struct {
	Name       string      `json:"name"`
	BuildingID int         `json:"building_id"`
	PlanType   string      `json:"plan_type"`
	Steps      []PlanStep  `json:"steps"`
}

// CreatePlan 创建启停计划（事务）
func (r *StartupRepo) CreatePlan(ctx context.Context, req CreatePlanReq) (int, error) {
	if req.PlanType == "" {
		req.PlanType = "startup"
	}
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback(ctx)

	var id int
	err = tx.QueryRow(ctx, "INSERT INTO startup_plan (name,building_id,plan_type) VALUES($1,$2,$3) RETURNING id",
		req.Name, req.BuildingID, req.PlanType).Scan(&id)
	if err != nil {
		return 0, err
	}
	for _, s := range req.Steps {
		action, rc, ws := normalizeStep(s.Action, req.PlanType, s.RetryCount, s.WaitSeconds)
		if _, err := tx.Exec(ctx,
			"INSERT INTO startup_step (plan_id,sort_order,device_id,property_id,action,target_value,wait_seconds,retry_count) VALUES($1,$2,$3,0,$4,'',$5,$6)",
			id, s.SortOrder, s.DeviceID, action, ws, rc); err != nil {
			return 0, err
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return 0, err
	}
	return id, nil
}

// UpdatePlanReq 更新计划请求
type UpdatePlanReq struct {
	Name     string     `json:"name"`
	PlanType string     `json:"plan_type"`
	Steps    []PlanStep `json:"steps"`
}

// UpdatePlan 更新启停计划（事务）
func (r *StartupRepo) UpdatePlan(ctx context.Context, id int, req UpdatePlanReq) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if _, err := tx.Exec(ctx, "UPDATE startup_plan SET name=$1,plan_type=$2 WHERE id=$3", req.Name, req.PlanType, id); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, "DELETE FROM startup_step WHERE plan_id=$1", id); err != nil {
		return err
	}
	for _, s := range req.Steps {
		action, rc, ws := normalizeStep(s.Action, req.PlanType, s.RetryCount, s.WaitSeconds)
		if _, err := tx.Exec(ctx,
			"INSERT INTO startup_step (plan_id,sort_order,device_id,property_id,action,target_value,wait_seconds,retry_count) VALUES($1,$2,$3,0,$4,'',$5,$6)",
			id, s.SortOrder, s.DeviceID, action, ws, rc); err != nil {
			return err
		}
	}
	return tx.Commit(ctx)
}

// DeletePlan 删除启停计划（事务）
func (r *StartupRepo) DeletePlan(ctx context.Context, id int) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if _, err := tx.Exec(ctx, "DELETE FROM startup_step WHERE plan_id=$1", id); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, "DELETE FROM startup_plan WHERE id=$1", id); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// ===== StartupExecution =====

// Execution 执行记录
type Execution struct {
	ID          int     `json:"id"`
	PlanID      int     `json:"plan_id"`
	TotalSteps  int     `json:"total_steps"`
	DoneSteps   int     `json:"done_steps"`
	ErrorStep   int     `json:"error_step"`
	PlanName    string  `json:"plan_name"`
	TriggeredBy string  `json:"triggered_by"`
	Status      string  `json:"status"`
	ErrorMessage string `json:"error_message"`
	StartedAt   *string `json:"started_at"`
	FinishedAt  *string `json:"finished_at"`
}

// StepLog 步骤日志
type StepLog struct {
	StepOrder    int    `json:"step_order"`
	DeviceName   string `json:"device_name"`
	Action       string `json:"action"`
	TargetValue  string `json:"target_value"`
	Result       string `json:"result"`
	ResponseValue string `json:"response_value"`
	DurationMs   int    `json:"duration_ms"`
	ErrorMessage string `json:"error_message"`
	ExecutedAt   string `json:"executed_at"`
}

// GetExecution 返回执行记录及步骤日志
func (r *StartupRepo) GetExecution(ctx context.Context, id int) (*Execution, []StepLog, error) {
	var e Execution
	err := r.pool.QueryRow(ctx,
		`SELECT id,plan_id,plan_name,triggered_by,status,total_steps,COALESCE(done_steps,0),
		        COALESCE(error_step,0),COALESCE(error_message,''),
		        COALESCE(started_at::text,''),COALESCE(finished_at::text,'')
		 FROM startup_execution WHERE id=$1`, id).
		Scan(&e.ID, &e.PlanID, &e.PlanName, &e.TriggeredBy, &e.Status,
			&e.TotalSteps, &e.DoneSteps, &e.ErrorStep, &e.ErrorMessage,
			&e.StartedAt, &e.FinishedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil, nil
		}
		return nil, nil, err
	}

	rows, err := r.pool.Query(ctx,
		`SELECT step_order,device_name,action,target_value,result,response_value,duration_ms,COALESCE(error_message,''),COALESCE(executed_at::text,'')
		 FROM startup_step_log WHERE execution_id=$1 ORDER BY step_order`, id)
	if err != nil {
		return nil, nil, err
	}
	defer rows.Close()
	var steps []StepLog
	for rows.Next() {
		var s StepLog
		if err := rows.Scan(&s.StepOrder, &s.DeviceName, &s.Action, &s.TargetValue, &s.Result, &s.ResponseValue, &s.DurationMs, &s.ErrorMessage, &s.ExecutedAt); err != nil {
			slog.Warn("StartupRepo.GetExecution scan failed", "err", err)
			continue
		}
		steps = append(steps, s)
	}
	return &e, steps, rows.Err()
}

// StopExecution 停止执行
func (r *StartupRepo) StopExecution(ctx context.Context, id int) (bool, error) {
	tag, err := r.pool.Exec(ctx,
		`UPDATE startup_execution SET status='stopped',finished_at=NOW() WHERE id=$1 AND status='running'`, id)
	if err != nil {
		return false, err
	}
	return tag.RowsAffected() > 0, nil
}

// ===== ScheduledTask =====

// ScheduledTask 定时任务
type ScheduledTask struct {
	ID           int     `json:"id"`
	Name         string  `json:"name"`
	BuildingID   int     `json:"building_id"`
	DeviceID     int     `json:"device_id"`
	DeviceName   string  `json:"device_name"`
	ActionType   string  `json:"action_type"`
	TargetValue  *string `json:"target_value"`
	ScheduleType string  `json:"schedule_type"`
	ScheduleTime string  `json:"schedule_time"`
	DaysOfWeek   *string `json:"days_of_week"`
	Enabled      *bool   `json:"enabled"`
	LastRunAt    *string `json:"last_run_at"`
	LastResult   *string `json:"last_result"`
	CreatedAt    string  `json:"created_at"`
}

// ListScheduledTasks 返回定时任务列表
func (r *StartupRepo) ListScheduledTasks(ctx context.Context, buildingID int) ([]ScheduledTask, error) {
	q := `SELECT st.id, st.name, st.building_id, st.device_id, COALESCE(d.name,''),
		st.action_type, st.target_value, st.schedule_type,
		st.schedule_time::text, st.days_of_week, st.enabled,
		COALESCE(to_char(st.last_run_at,'YYYY-MM-DD HH24:MI:SS'),''),
		COALESCE(st.last_result,''),
		to_char(st.created_at,'YYYY-MM-DD HH24:MI:SS')
	 FROM scheduled_task st JOIN device d ON d.id=st.device_id`
	var args []any
	if buildingID > 0 {
		q += ` WHERE st.building_id=$1`
		args = append(args, buildingID)
	}
	q += ` ORDER BY st.id`
	rows, err := r.pool.Query(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var list []ScheduledTask
	for rows.Next() {
		var t ScheduledTask
		if err := rows.Scan(&t.ID, &t.Name, &t.BuildingID, &t.DeviceID, &t.DeviceName,
			&t.ActionType, &t.TargetValue, &t.ScheduleType,
			&t.ScheduleTime, &t.DaysOfWeek, &t.Enabled,
			&t.LastRunAt, &t.LastResult, &t.CreatedAt); err != nil {
			slog.Warn("StartupRepo.ListScheduledTasks scan failed", "err", err)
			continue
		}
		if t.LastRunAt != nil && *t.LastRunAt == "" {
			t.LastRunAt = nil
		}
		if t.LastResult != nil && *t.LastResult == "" {
			t.LastResult = nil
		}
		list = append(list, t)
	}
	return list, rows.Err()
}

// CreateScheduledTask 创建定时任务
func (r *StartupRepo) CreateScheduledTask(ctx context.Context, t ScheduledTask) (int, error) {
	enabled := true
	if t.Enabled != nil {
		enabled = *t.Enabled
	}
	if t.ScheduleType == "" {
		t.ScheduleType = "once"
	}
	var id int
	err := r.pool.QueryRow(ctx,
		`INSERT INTO scheduled_task (name,building_id,device_id,action_type,target_value,schedule_type,schedule_time,days_of_week,enabled)
		 VALUES ($1,$2,$3,$4,$5,$6,$7::time,$8,$9) RETURNING id`,
		t.Name, t.BuildingID, t.DeviceID, t.ActionType, t.TargetValue, t.ScheduleType, t.ScheduleTime, t.DaysOfWeek, enabled).Scan(&id)
	return id, err
}

// UpdateScheduledTask 更新定时任务（支持部分更新）
func (r *StartupRepo) UpdateScheduledTask(ctx context.Context, id int, t ScheduledTask) error {
	// 获取现有记录
	var cur ScheduledTask
	err := r.pool.QueryRow(ctx,
		`SELECT name, building_id, device_id, action_type, target_value,
		        schedule_type, schedule_time, days_of_week, enabled
		 FROM scheduled_task WHERE id=$1`, id).Scan(
		&cur.Name, &cur.BuildingID, &cur.DeviceID, &cur.ActionType,
		&cur.TargetValue, &cur.ScheduleType, &cur.ScheduleTime,
		&cur.DaysOfWeek, &cur.Enabled)
	if err != nil {
		return err
	}

	// 合并：使用传入的非零值
	if t.Name != "" {
		cur.Name = t.Name
	}
	if t.DeviceID != 0 {
		cur.DeviceID = t.DeviceID
	}
	if t.ActionType != "" {
		cur.ActionType = t.ActionType
	}
	if t.TargetValue != nil {
		cur.TargetValue = t.TargetValue
	}
	if t.ScheduleType != "" {
		cur.ScheduleType = t.ScheduleType
	}
	if t.ScheduleTime != "" {
		cur.ScheduleTime = t.ScheduleTime
	}
	if t.DaysOfWeek != nil {
		cur.DaysOfWeek = t.DaysOfWeek
	}
	if t.Enabled != nil {
		cur.Enabled = t.Enabled
	}

	enabled := true
	if cur.Enabled != nil {
		enabled = *cur.Enabled
	}
	_, err = r.pool.Exec(ctx,
		`UPDATE scheduled_task SET name=$1,device_id=$2,action_type=$3,target_value=$4,schedule_type=$5,schedule_time=$6::time,days_of_week=$7,enabled=$8 WHERE id=$9`,
		cur.Name, cur.DeviceID, cur.ActionType, cur.TargetValue, cur.ScheduleType, cur.ScheduleTime, cur.DaysOfWeek, enabled, id)
	return err
}

// DeleteScheduledTask 删除定时任务
func (r *StartupRepo) DeleteScheduledTask(ctx context.Context, id int) error {
	_, err := r.pool.Exec(ctx, "DELETE FROM scheduled_task WHERE id=$1", id)
	return err
}

// DueScheduledTask 待执行的定时任务
type DueScheduledTask struct {
	ID          int
	DeviceID    int
	ActionType  string
	TargetValue *string
	DeviceName  string
}

// ListDueScheduledTasks 返回待执行的定时任务（由 RunDueScheduledTasks 调用）
func (r *StartupRepo) ListDueScheduledTasks(ctx context.Context) ([]DueScheduledTask, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT st.id, st.device_id, st.action_type, st.target_value, COALESCE(d.name,'')
		 FROM scheduled_task st JOIN device d ON d.id=st.device_id
		 WHERE st.enabled=true
		 AND st.schedule_time::time <= CURRENT_TIME::time
		 AND (
		   st.last_run_at IS NULL
		   OR (st.last_run_at::date < CURRENT_DATE AND st.schedule_type != 'once')
		 )
		 AND (st.schedule_type='daily'
		   OR (st.schedule_type='weekly' AND (
		     CASE extract(dow from CURRENT_DATE) WHEN 0 THEN '7' ELSE extract(dow from CURRENT_DATE)::text END
		   ) = ANY(regexp_split_to_array(COALESCE(st.days_of_week,''), '\s*,\s*')))
		   OR (st.schedule_type='once' AND st.last_run_at IS NULL))
		 ORDER BY st.id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var list []DueScheduledTask
	for rows.Next() {
		var t DueScheduledTask
		if err := rows.Scan(&t.ID, &t.DeviceID, &t.ActionType, &t.TargetValue, &t.DeviceName); err != nil {
			slog.Warn("StartupRepo.ListDueScheduledTasks scan failed", "err", err)
			continue
		}
		list = append(list, t)
	}
	return list, rows.Err()
}

// UpdateScheduledTaskResult 更新定时任务执行结果
func (r *StartupRepo) UpdateScheduledTaskResult(ctx context.Context, id int, result string) error {
	_, err := r.pool.Exec(ctx, `UPDATE scheduled_task SET last_result=$1, last_run_at=NOW() WHERE id=$2`, result, id)
	return err
}

// ===== 辅助函数 =====

// normalizeStep 应用步骤默认值
func normalizeStep(action, planType string, retryCount, waitSeconds int) (string, int, int) {
	if action == "" {
		action = planType
	}
	if retryCount < 1 {
		retryCount = 1
	}
	if waitSeconds <= 0 {
		waitSeconds = 30
	}
	return action, retryCount, waitSeconds
}

// boolDefault 解引用 *bool，nil 时返回默认值
func boolDefault(p *bool, defaultVal bool) bool {
	if p == nil {
		return defaultVal
	}
	return *p
}

// itos 辅助函数已在 log_repo.go 定义，此处不再重复