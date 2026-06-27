package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"xmeco/internal/api/middleware"
	"xmeco/internal/repository/postgres"
	"xmeco/internal/service/orchestrator"

	"github.com/jackc/pgx/v5"
)

// boolDefault 解引用 *bool，nil 时返回默认值。
func boolDefault(p *bool, defaultVal bool) bool {
	if p == nil { return defaultVal }
	return *p
}

// normalizeStep applies defaults for plan step fields (action fallback, retry count min, wait seconds min).
func normalizeStep(action, planType string, retryCount, waitSeconds int) (string, int, int) {
	if action == "" { action = planType }
	if retryCount < 1 { retryCount = 1 }
	if waitSeconds <= 0 { waitSeconds = 30 }
	return action, retryCount, waitSeconds
}

type StartupHandler struct {
	pool      postgres.DBTX
	interlock *orchestrator.Interlock
	dev       *DeviceHandler // enables real hardware dispatch for plan/scheduled control
}

func NewStartupHandler(pool postgres.DBTX) *StartupHandler {
	return &StartupHandler{pool: pool, interlock: orchestrator.NewInterlock(pool)}
}

// SetDeviceHandler injects the device handler so that plan execution and scheduled
// tasks dispatch real Modbus control commands to the gateway (not just audit records).
func (h *StartupHandler) SetDeviceHandler(dh *DeviceHandler) { h.dev = dh }

func (h *StartupHandler) ListPlans(w http.ResponseWriter, r *http.Request) {
	buildingID := queryInt(r, "building_id")
	var rows pgx.Rows
	var err error
	if buildingID > 0 {
		rows, err = h.pool.Query(r.Context(), "SELECT p.id,p.name,p.building_id,p.plan_type,p.precheck_online,p.precheck_alarm,p.enabled,COALESCE((SELECT json_agg(json_build_object('id',s.id,'device_id',s.device_id,'device_name',d.name,'sort_order',s.sort_order,'wait_seconds',s.wait_seconds,'retry_count',COALESCE(s.retry_count,1),'action',s.action) ORDER BY s.sort_order) FROM startup_step s JOIN device d ON d.id=s.device_id WHERE s.plan_id=p.id),'[]'::json) FROM startup_plan p WHERE ($1=0 OR p.building_id=$1) ORDER BY p.id", buildingID)
	} else {
		rows, err = h.pool.Query(r.Context(), "SELECT p.id,p.name,p.building_id,p.plan_type,p.precheck_online,p.precheck_alarm,p.enabled,COALESCE((SELECT json_agg(json_build_object('id',s.id,'device_id',s.device_id,'device_name',d.name,'sort_order',s.sort_order,'wait_seconds',s.wait_seconds,'retry_count',COALESCE(s.retry_count,1),'action',s.action) ORDER BY s.sort_order) FROM startup_step s JOIN device d ON d.id=s.device_id WHERE s.plan_id=p.id),'[]'::json) FROM startup_plan p ORDER BY p.id")
	}
	if err != nil { serverErr(w, err); return }
	defer rows.Close()
	type plan struct {
		ID       int              `json:"id"`
		Name     string           `json:"name"`
		BID      int              `json:"building_id"`
		PlanType string           `json:"plan_type"`
		PreOn    bool             `json:"precheck_online"`
		PreAl    bool             `json:"precheck_alarm"`
		En       bool             `json:"enabled"`
		Steps    json.RawMessage  `json:"steps"`
	}
	var list []plan
	for rows.Next() {
		var p plan
		if err := rows.Scan(&p.ID, &p.Name, &p.BID, &p.PlanType, &p.PreOn, &p.PreAl, &p.En, &p.Steps); err != nil {
			slog.Warn("ListPlans scan failed", "err", err)
			continue
		}
		list = append(list, p)
	}
	if list == nil { list = []plan{} }
	if err := rows.Err(); err != nil { serverErr(w, err); return }
	ok(w, list)
}

func (h *StartupHandler) CreatePlan(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Name string `json:"name"`
		BuildingID int `json:"building_id"`
		PlanType string `json:"plan_type"`
		Steps []struct{ DeviceID int `json:"device_id"`; DeviceName string `json:"device_name"`; SortOrder int `json:"sort_order"`; WaitSeconds int `json:"wait_seconds"`; RetryCount int `json:"retry_count"`; Action string `json:"action"` } `json:"steps"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeJSON(w, http.StatusBadRequest, M{"error": "请求格式错误"})
		return
	}
	if body.PlanType == "" { body.PlanType = "startup" }

	// Wrap both plan creation and step inserts in a transaction to avoid
	// orphaned plan records on partial step insert failures.
	tx, err := h.pool.Begin(r.Context())
	if err != nil { serverErr(w, err); return }
	defer tx.Rollback(r.Context())

	var id int
	err = tx.QueryRow(r.Context(), "INSERT INTO startup_plan (name,building_id,plan_type) VALUES($1,$2,$3) RETURNING id", body.Name, body.BuildingID, body.PlanType).Scan(&id)
	if err != nil { serverErr(w, err); return }
	for _, s := range body.Steps {
		action, rc, ws := normalizeStep(s.Action, body.PlanType, s.RetryCount, s.WaitSeconds)
		if _, err := tx.Exec(r.Context(), "INSERT INTO startup_step (plan_id,sort_order,device_id,property_id,action,target_value,wait_seconds,retry_count) VALUES($1,$2,$3,0,$4,'',$5,$6)", id, s.SortOrder, s.DeviceID, action, ws, rc); err != nil {
			serverErr(w, err)
			return
		}
	}
	if err := tx.Commit(r.Context()); err != nil { serverErr(w, err); return }
	ok(w, M{"id":fmt.Sprint(id)})
}

func (h *StartupHandler) UpdatePlan(w http.ResponseWriter, r *http.Request) {
	id := pathID(r)
	var body struct {
		Name string `json:"name"`
		PlanType string `json:"plan_type"`
		Steps []struct{ DeviceID int `json:"device_id"`; SortOrder int `json:"sort_order"`; WaitSeconds int `json:"wait_seconds"`; RetryCount int `json:"retry_count"`; Action string `json:"action"` } `json:"steps"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeJSON(w, http.StatusBadRequest, M{"error": "请求格式错误"})
		return
	}
	if _, err := h.pool.Exec(r.Context(), "UPDATE startup_plan SET name=$1,plan_type=$2 WHERE id=$3", body.Name, body.PlanType, id); err != nil {
		serverErr(w, err)
		return
	}
	// Wrap DELETE + INSERT in a transaction to avoid data loss on partial failure
	tx, err := h.pool.Begin(r.Context())
	if err != nil {
		serverErr(w, err)
		return
	}
	defer tx.Rollback(r.Context())
	if _, err := tx.Exec(r.Context(), "DELETE FROM startup_step WHERE plan_id=$1", id); err != nil {
		serverErr(w, err)
		return
	}
	for _, s := range body.Steps {
		action, rc, ws := normalizeStep(s.Action, body.PlanType, s.RetryCount, s.WaitSeconds)
		if _, err := tx.Exec(r.Context(), "INSERT INTO startup_step (plan_id,sort_order,device_id,property_id,action,target_value,wait_seconds,retry_count) VALUES($1,$2,$3,0,$4,'',$5,$6)", id, s.SortOrder, s.DeviceID, action, ws, rc); err != nil {
			serverErr(w, err)
			return
		}
	}
	if err := tx.Commit(r.Context()); err != nil {
		serverErr(w, err)
		return
	}
	ok(w, M{"status":"updated"})
}

func (h *StartupHandler) DeletePlan(w http.ResponseWriter, r *http.Request) {
	id := pathID(r)
	tx, err := h.pool.Begin(r.Context())
	if err != nil { serverErr(w, err); return }
	defer tx.Rollback(r.Context())
	if _, err := tx.Exec(r.Context(), "DELETE FROM startup_step WHERE plan_id=$1", id); err != nil {
		serverErr(w, err)
		return
	}
	if _, err := tx.Exec(r.Context(), "DELETE FROM startup_plan WHERE id=$1", id); err != nil {
		serverErr(w, err)
		return
	}
	if err := tx.Commit(r.Context()); err != nil { serverErr(w, err); return }
	ok(w, M{"status":"deleted"})
}

// Execute loads the plan via orchestrator, writes a proper execution record,
// and launches execution in a background goroutine.
func (h *StartupHandler) Execute(w http.ResponseWriter, r *http.Request) {
	planID := pathID(r)

	plan, steps, err := orchestrator.LoadPlan(r.Context(), h.pool, planID)
	if err != nil {
		notFound(w, "启停计划不存在")
		return
	}

	// Require authentication for plan execution.
	claims := middleware.GetClaims(r)
	if claims == nil {
		writeJSON(w, http.StatusUnauthorized, M{"error": "未认证，无法执行启停操作"})
		return
	}
	username := claims.Username

	exec, err := orchestrator.StartExecution(r.Context(), h.pool, planID, plan.Name, username, len(steps))
	if err != nil {
		serverErr(w, err)
		return
	}

	// Run in background — HTTP response returns immediately with execution ID.
	// WithoutCancel decouples from request lifecycle; WithTimeout prevents runaway
	// goroutines when a step blocks (e.g. gateway transport stall).
	bgCtx, cancel := context.WithTimeout(context.WithoutCancel(r.Context()), 5*time.Minute)
	go func() {
		defer cancel()
		defer func() {
			if rec := recover(); rec != nil {
				slog.Error("startup execution panic", "plan", plan.Name, "panic", rec)
				// Mark execution as failed so it doesn't stay "running" forever.
				if _, e := h.pool.Exec(context.Background(),
					`UPDATE startup_execution SET status='failed',finished_at=NOW() WHERE id=$1`, exec.ID); e != nil {
					slog.Warn("failed to mark execution as failed after panic", "exec", exec.ID, "err", e)
				}
			}
		}()
		exec.Run(bgCtx, steps, h.execDeviceControl)
	}()

	ok(w, map[string]any{
		"status":       "started",
		"execution_id": exec.ID,
		"plan_name":    plan.Name,
		"total_steps":  len(steps),
	})
}

// execDeviceControl is the per-step execution function passed to the orchestrator.
// It checks interlock conditions, writes a control_record and updates the device status.
func (h *StartupHandler) execDeviceControl(ctx context.Context, devID int, action, targetValue string) (string, error) {
	// Check interlock conditions before executing
	if err := h.interlock.Check(ctx, devID, action); err != nil {
		return "", fmt.Errorf("联锁检查失败: %w", err)
	}

	var devName, bldName, projName string
	err := h.pool.QueryRow(ctx, `
		SELECT d.name, b.name, COALESCE(p.name, '')
		FROM device d
		JOIN building b ON b.id = d.building_id
		JOIN project p ON p.id = b.project_id
		WHERE d.id = $1`, devID).Scan(&devName, &bldName, &projName)
	if err != nil {
		return "", fmt.Errorf("设备 %d 不存在", devID)
	}

	controlVal, deviceStatus := controlActionCN(action)

	_, err = h.pool.Exec(ctx, `
		INSERT INTO control_record (project_name, building_name, device_name, device_id, prop_name, control_value, username, user_remark)
		VALUES ($1, $2, $3, $4, '启停编排', $5, 'orchestrator', $6)`,
		projName, bldName, devName, devID, controlVal, action)
	if err != nil {
		return "", fmt.Errorf("写控制记录失败: %w", err)
	}

	// Dispatch to the physical device via the gateway (best-effort).
	h.dispatchControl(ctx, devID, action)

	if deviceStatus != "" {
		if _, err := h.pool.Exec(ctx, `UPDATE device SET device_status=$1 WHERE id=$2`, deviceStatus, devID); err != nil {
			slog.Warn("update device status failed", "dev", devID, "status", deviceStatus, "err", err)
		}
	}

	return controlVal, nil
}

func (h *StartupHandler) GetExecution(w http.ResponseWriter, r *http.Request) {
	id := pathID(r)
	var e struct {
		ID                    int     `json:"id"`
		PlanID                int     `json:"plan_id"`
		TotalSteps            int     `json:"total_steps"`
		DoneSteps             int     `json:"done_steps"`
		ErrorStep             int     `json:"error_step"`
		PlanName              string  `json:"plan_name"`
		TriggeredBy           string  `json:"triggered_by"`
		Status                string  `json:"status"`
		ErrorMessage          string  `json:"error_message"`
		StartedAt             *string `json:"started_at"`
		FinishedAt            *string `json:"finished_at"`
	}
	err := h.pool.QueryRow(r.Context(),
		`SELECT id,plan_id,plan_name,triggered_by,status,total_steps,COALESCE(done_steps,0),
		        COALESCE(error_step,0),COALESCE(error_message,''),
		        COALESCE(started_at::text,''),COALESCE(finished_at::text,'')
		 FROM startup_execution WHERE id=$1`, id).
		Scan(&e.ID, &e.PlanID, &e.PlanName, &e.TriggeredBy, &e.Status,
			&e.TotalSteps, &e.DoneSteps, &e.ErrorStep, &e.ErrorMessage,
			&e.StartedAt, &e.FinishedAt)
	if err != nil {
		notFound(w, "执行记录不存在")
		return
	}

	// load step logs
	rows, err := h.pool.Query(r.Context(),
		`SELECT step_order,device_name,action,target_value,result,response_value,duration_ms,COALESCE(error_message,''),COALESCE(executed_at::text,'')
		 FROM startup_step_log WHERE execution_id=$1 ORDER BY step_order`, id)
	if err != nil {
		serverErr(w, err)
		return
	}
	defer rows.Close()
	type stepLog struct {
		StepOrder int `json:"step_order"`
		DeviceName string `json:"device_name"`
		Action string `json:"action"`
		TargetValue string `json:"target_value"`
		Result string `json:"result"`
		ResponseValue string `json:"response_value"`
		DurationMs int `json:"duration_ms"`
		ErrorMessage string `json:"error_message"`
		ExecutedAt string `json:"executed_at"`
	}
	var steps []stepLog
	for rows.Next() {
		var s stepLog
		if err := rows.Scan(&s.StepOrder, &s.DeviceName, &s.Action, &s.TargetValue, &s.Result, &s.ResponseValue, &s.DurationMs, &s.ErrorMessage, &s.ExecutedAt); err != nil {
			slog.Warn("GetExecution scan failed", "err", err)
			continue
		}
		steps = append(steps, s)
	}
	if steps == nil { steps = []stepLog{} }
	if err := rows.Err(); err != nil { serverErr(w, err); return }

	ok(w, map[string]any{
		"execution":    e,
		"step_logs":    steps,
	})
}

func (h *StartupHandler) StopExecution(w http.ResponseWriter, r *http.Request) {
	id := pathID(r)
	tag, err := h.pool.Exec(r.Context(),
		`UPDATE startup_execution SET status='stopped',finished_at=NOW() WHERE id=$1 AND status='running'`, id)
	if err != nil {
		serverErr(w, err)
		return
	}
	if tag.RowsAffected() == 0 {
		notFound(w, "执行记录不存在或已结束")
		return
	}
	ok(w, M{"status":"stopped"})
}

// ---- Scheduled Tasks ----

type scheduledTaskRow struct {
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

func (h *StartupHandler) ListScheduledTasks(w http.ResponseWriter, r *http.Request) {
	bid := queryInt(r, "building_id")
	q := `SELECT st.id, st.name, st.building_id, st.device_id, COALESCE(d.name,''),
		st.action_type, st.target_value, st.schedule_type,
		st.schedule_time::text, st.days_of_week, st.enabled,
		COALESCE(to_char(st.last_run_at,'YYYY-MM-DD HH24:MI:SS'),''),
		COALESCE(st.last_result,''),
		to_char(st.created_at,'YYYY-MM-DD HH24:MI:SS')
	 FROM scheduled_task st JOIN device d ON d.id=st.device_id`
	var args []any
	if bid > 0 {
		q += ` WHERE st.building_id=$1`
		args = append(args, bid)
	}
	q += ` ORDER BY st.id`
	rows, err := h.pool.Query(r.Context(), q, args...)
	if err != nil { serverErr(w, err); return }
	defer rows.Close()
	var list []scheduledTaskRow
	for rows.Next() {
		var t scheduledTaskRow
		if err := rows.Scan(&t.ID, &t.Name, &t.BuildingID, &t.DeviceID, &t.DeviceName,
			&t.ActionType, &t.TargetValue, &t.ScheduleType,
			&t.ScheduleTime, &t.DaysOfWeek, &t.Enabled,
			&t.LastRunAt, &t.LastResult, &t.CreatedAt); err != nil {
			slog.Warn("ListScheduledTasks scan failed", "err", err)
			continue
		}
		if t.LastRunAt != nil && *t.LastRunAt == "" { t.LastRunAt = nil }
		if t.LastResult != nil && *t.LastResult == "" { t.LastResult = nil }
		list = append(list, t)
	}
	if list == nil { list = []scheduledTaskRow{} }
	ok(w, list)
}

func (h *StartupHandler) CreateScheduledTask(w http.ResponseWriter, r *http.Request) {
	var t scheduledTaskRow
	if err := json.NewDecoder(r.Body).Decode(&t); err != nil { writeJSON(w, 400, M{"error":"格式错误"}); return }
	if t.Name == "" || t.DeviceID == 0 || t.ScheduleTime == "" { writeJSON(w, 400, M{"error":"名称/设备/时间不能为空"}); return }
	if t.ScheduleType == "" { t.ScheduleType = "once" }
	err := h.pool.QueryRow(r.Context(),
		`INSERT INTO scheduled_task (name,building_id,device_id,action_type,target_value,schedule_type,schedule_time,days_of_week,enabled)
		 VALUES ($1,$2,$3,$4,$5,$6,$7::time,$8,$9) RETURNING id`,
		t.Name, t.BuildingID, t.DeviceID, t.ActionType, t.TargetValue, t.ScheduleType, t.ScheduleTime, t.DaysOfWeek, boolDefault(t.Enabled, true),
	).Scan(&t.ID)
	if err != nil { serverErr(w, err); return }
	created(w, t)
}

func (h *StartupHandler) UpdateScheduledTask(w http.ResponseWriter, r *http.Request) {
	var t scheduledTaskRow
	if err := json.NewDecoder(r.Body).Decode(&t); err != nil { writeJSON(w, 400, M{"error":"格式错误"}); return }
	t.ID = pathID(r)

	// Fetch existing record to support partial updates (e.g. toggling just "enabled")
	var cur scheduledTaskRow
	err := h.pool.QueryRow(r.Context(),
		`SELECT name, building_id, device_id, action_type, target_value,
		        schedule_type, schedule_time, days_of_week, enabled
		 FROM scheduled_task WHERE id=$1`, t.ID).Scan(
		&cur.Name, &cur.BuildingID, &cur.DeviceID, &cur.ActionType,
		&cur.TargetValue, &cur.ScheduleType, &cur.ScheduleTime,
		&cur.DaysOfWeek, &cur.Enabled,
	)
	if err != nil { notFound(w, "定时任务不存在"); return }

	// Merge: use incoming non-zero values, fall back to current
	if t.Name != "" { cur.Name = t.Name }
	if t.DeviceID != 0 { cur.DeviceID = t.DeviceID }
	if t.ActionType != "" { cur.ActionType = t.ActionType }
	if t.TargetValue != nil { cur.TargetValue = t.TargetValue }
	if t.ScheduleType != "" { cur.ScheduleType = t.ScheduleType }
	if t.ScheduleTime != "" { cur.ScheduleTime = t.ScheduleTime }
	if t.DaysOfWeek != nil { cur.DaysOfWeek = t.DaysOfWeek }
	if t.Enabled != nil { cur.Enabled = t.Enabled }

	_, err = h.pool.Exec(r.Context(),
		`UPDATE scheduled_task SET name=$1,device_id=$2,action_type=$3,target_value=$4,schedule_type=$5,schedule_time=$6::time,days_of_week=$7,enabled=$8 WHERE id=$9`,
		cur.Name, cur.DeviceID, cur.ActionType, cur.TargetValue, cur.ScheduleType, cur.ScheduleTime, cur.DaysOfWeek, boolDefault(cur.Enabled, true), t.ID)
	if err != nil { serverErr(w, err); return }
	ok(w, M{"status":"updated"})
}

func (h *StartupHandler) DeleteScheduledTask(w http.ResponseWriter, r *http.Request) {
	_, err := h.pool.Exec(r.Context(), `DELETE FROM scheduled_task WHERE id=$1`, pathID(r))
	if err != nil { serverErr(w, err); return }
	ok(w, M{"status":"deleted"})
}

// Helper: run due scheduled tasks (called by scheduler goroutine)
func (h *StartupHandler) RunDueScheduledTasks(ctx context.Context) {
	// NOTE: extract(dow) returns 0=Sun..6=Sat. days_of_week uses 1=Mon..7=Sun.
	// Map Sunday (dow=0) to 7 for matching. Use regexp_split_to_array to avoid
	// substring false matches (e.g. "1" matching "10").
	rows, err := h.pool.Query(ctx,
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
	if err != nil { slog.Error("RunDueScheduledTasks query failed", "err", err); return }
	defer rows.Close()
	for rows.Next() {
		var id, devID int
		var action, devName string
		var targetVal *string
		if err := rows.Scan(&id, &devID, &action, &targetVal, &devName); err != nil {
			slog.Warn("RunDueScheduledTasks scan failed", "err", err)
			continue
		}
		// Execute the action via gateway (with per-task timeout to avoid
		// a hung gateway from blocking all subsequent tasks).
		taskCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
		err := h.executeAction(taskCtx, devID, action, targetVal)
		cancel()
		result := "success"
		if err != nil {
			result = "failed"
			slog.Warn("scheduled task failed", "id", id, "err", err)
		} else {
			slog.Info("scheduled task completed", "id", id, "device", devID, "action", action)
		}
		if _, e := h.pool.Exec(ctx, `UPDATE scheduled_task SET last_result=$1, last_run_at=NOW() WHERE id=$2`, result, id); e != nil {
			slog.Warn("scheduled task result update failed", "id", id, "err", e)
		}
	}
}

func (h *StartupHandler) executeAction(ctx context.Context, devID int, action string, targetVal *string) error {
	controlVal, deviceStatus := controlActionCN(action)
	remark := action
	if targetVal != nil && *targetVal != "" {
		remark = action + ":" + *targetVal
	}
	_, err := h.pool.Exec(ctx,
		`INSERT INTO control_record (project_name,building_name,device_name,device_id,prop_name,control_value,username,user_remark,created_at)
		 SELECT COALESCE(p.name,''), COALESCE(b.name,''), COALESCE(d.name,''), d.id, '定时任务', $2, '定时任务', $3, NOW()
		 FROM device d LEFT JOIN building b ON b.id=d.building_id LEFT JOIN project p ON p.id=b.project_id
		 WHERE d.id=$1`, devID, controlVal, remark)
	if err == nil {
		h.dispatchControl(ctx, devID, action)
		if deviceStatus != "" {
			if _, e := h.pool.Exec(ctx, `UPDATE device SET device_status=$1 WHERE id=$2`, deviceStatus, devID); e != nil {
				slog.Warn("scheduled task update status failed", "dev", devID, "err", e)
			}
		}
	}
	return err
}

// dispatchControl forwards a control action to the physical device via the gateway.
// Logs dispatch result but does not fail the caller (the control_record written by the
// caller remains as the permanent audit trail).
func (h *StartupHandler) dispatchControl(ctx context.Context, devID int, action string) {
	if h.dev == nil {
		slog.Warn("startup/scheduled dispatch skipped: device handler not wired", "dev", devID, "action", action)
		return
	}
	dr := h.dev.dispatchHardware(ctx, devID, action)
	if !dr.Dispatched {
		slog.Warn("startup/scheduled dispatch failed", "dev", devID, "action", action, "msg", dr.Message)
	}
}
