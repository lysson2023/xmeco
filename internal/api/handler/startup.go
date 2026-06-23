package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"

	"xmeco/internal/api/middleware"
	"xmeco/internal/service/orchestrator"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type StartupHandler struct {
	pool      *pgxpool.Pool
	interlock *orchestrator.Interlock
}

func NewStartupHandler(pool *pgxpool.Pool) *StartupHandler {
	return &StartupHandler{pool: pool, interlock: orchestrator.NewInterlock(pool)}
}

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
	var id int
	err := h.pool.QueryRow(r.Context(), "INSERT INTO startup_plan (name,building_id,plan_type) VALUES($1,$2,$3) RETURNING id", body.Name, body.BuildingID, body.PlanType).Scan(&id)
	if err != nil { serverErr(w, err); return }
	for _, s := range body.Steps {
		action := s.Action
		if action == "" { action = body.PlanType }
		rc := s.RetryCount
		if rc < 1 { rc = 1 }
		if _, err := h.pool.Exec(r.Context(), "INSERT INTO startup_step (plan_id,sort_order,device_id,property_id,action,target_value,wait_seconds,retry_count) VALUES($1,$2,$3,0,$4,'',COALESCE($5,30),$6)", id, s.SortOrder, s.DeviceID, action, s.WaitSeconds, rc); err != nil {
			slog.Warn("create startup step failed", "plan", id, "err", err)
		}
	}
	ok(w, M{"id":fmt.Sprint(id)})
}

func (h *StartupHandler) UpdatePlan(w http.ResponseWriter, r *http.Request) {
	id := pathLast(r.URL.Path)
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
		action := s.Action
		if action == "" { action = body.PlanType }
		rc := s.RetryCount
		if rc < 1 { rc = 1 }
		if _, err := tx.Exec(r.Context(), "INSERT INTO startup_step (plan_id,sort_order,device_id,property_id,action,target_value,wait_seconds,retry_count) VALUES($1,$2,$3,0,$4,'',COALESCE($5,30),$6)", id, s.SortOrder, s.DeviceID, action, s.WaitSeconds, rc); err != nil {
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
	id := pathLast(r.URL.Path)
	if _, err := h.pool.Exec(r.Context(), "DELETE FROM startup_step WHERE plan_id=$1", id); err != nil {
		serverErr(w, err)
		return
	}
	if _, err := h.pool.Exec(r.Context(), "DELETE FROM startup_plan WHERE id=$1", id); err != nil {
		serverErr(w, err)
		return
	}
	ok(w, M{"status":"deleted"})
}

// Execute loads the plan via orchestrator, writes a proper execution record,
// and launches execution in a background goroutine.
func (h *StartupHandler) Execute(w http.ResponseWriter, r *http.Request) {
	planID := pathID(r.URL.Path)

	plan, steps, err := orchestrator.LoadPlan(r.Context(), h.pool, planID)
	if err != nil {
		notFound(w, "启停计划不存在")
		return
	}

	username := "admin"
	if claims := middleware.GetClaims(r); claims != nil {
		username = claims.Username
	}

	exec, err := orchestrator.StartExecution(r.Context(), h.pool, planID, plan.Name, username, len(steps))
	if err != nil {
		serverErr(w, err)
		return
	}

	// Run in background — HTTP response returns immediately with execution ID.
	// Use WithoutCancel to retain tracing values but not cancel on HTTP disconnect.
	bgCtx := context.WithoutCancel(r.Context())
	go func() {
		defer func() {
			if rec := recover(); rec != nil {
				slog.Error("startup execution panic", "plan", plan.Name, "panic", rec)
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

	controlVal := action
	deviceStatus := ""
	switch action {
	case "start", "startup":
		controlVal = "开机"
		deviceStatus = "开机"
	case "stop", "shutdown":
		controlVal = "关机"
		deviceStatus = "关机"
	}

	_, err = h.pool.Exec(ctx, `
		INSERT INTO control_record (project_name, building_name, device_name, prop_name, control_value, username, user_remark)
		VALUES ($1, $2, $3, '启停编排', $4, 'orchestrator', $5)`,
		projName, bldName, devName, controlVal, action)
	if err != nil {
		return "", fmt.Errorf("写控制记录失败: %w", err)
	}

	if deviceStatus != "" {
		if _, err := h.pool.Exec(ctx, `UPDATE device SET device_status=$1 WHERE id=$2`, deviceStatus, devID); err != nil {
			slog.Warn("update device status failed", "dev", devID, "status", deviceStatus, "err", err)
		}
	}

	return controlVal, nil
}

func (h *StartupHandler) GetExecution(w http.ResponseWriter, r *http.Request) {
	id := pathID(r.URL.Path)
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
	id := pathID(r.URL.Path)
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
