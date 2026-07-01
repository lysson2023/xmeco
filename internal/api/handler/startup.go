package handler

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"xmeco/internal/api/middleware"
	"xmeco/internal/repository/postgres"
	"xmeco/internal/service/orchestrator"

	"github.com/jackc/pgx/v5"
)

type StartupHandler struct {
	repo      *postgres.StartupRepo
	pool      postgres.DBTX // 保留pool用于orchestrator和控制记录写入
	interlock *orchestrator.Interlock
	dev       HardwareDispatcher
	bgCtx     context.Context // 后台 context，shutdown 时取消，用于编排 goroutine 生命周期
}

func NewStartupHandler(repo *postgres.StartupRepo, pool postgres.DBTX) *StartupHandler {
	return &StartupHandler{repo: repo, pool: pool, interlock: orchestrator.NewInterlock(pool), bgCtx: context.Background()}
}

func (h *StartupHandler) SetDeviceHandler(dh HardwareDispatcher) { h.dev = dh }

// SetBgCtx 设置编排 goroutine 的后台 context，shutdown 时该 context 会被取消。
func (h *StartupHandler) SetBgCtx(ctx context.Context) { h.bgCtx = ctx }

func (h *StartupHandler) ListPlans(w http.ResponseWriter, r *http.Request) {
	list, err := h.repo.ListPlans(r.Context(), queryInt(r, "building_id"))
	if err != nil { serverErr(w, err); return }
	if list == nil { list = []postgres.StartupPlan{} }
	ok(w, list)
}

func (h *StartupHandler) CreatePlan(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Name       string `json:"name"`
		BuildingID int    `json:"building_id"`
		PlanType   string `json:"plan_type"`
		Steps      []struct {
			DeviceID    int    `json:"device_id"`
			SortOrder   int    `json:"sort_order"`
			WaitSeconds int    `json:"wait_seconds"`
			RetryCount  int    `json:"retry_count"`
			Action      string `json:"action"`
		} `json:"steps"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeJSON(w, http.StatusBadRequest, errBadRequest)
		return
	}
	steps := make([]postgres.PlanStep, len(body.Steps))
	for i, s := range body.Steps {
		steps[i] = postgres.PlanStep{DeviceID: s.DeviceID, SortOrder: s.SortOrder, WaitSeconds: s.WaitSeconds, RetryCount: s.RetryCount, Action: s.Action}
	}
	id, err := h.repo.CreatePlan(r.Context(), postgres.CreatePlanReq{Name: body.Name, BuildingID: body.BuildingID, PlanType: body.PlanType, Steps: steps})
	if err != nil { serverErr(w, err); return }
	ok(w, M{"id": strconv.Itoa(id)})
}

func (h *StartupHandler) UpdatePlan(w http.ResponseWriter, r *http.Request) {
	id := pathID(r)
	var body struct {
		Name     string `json:"name"`
		PlanType string `json:"plan_type"`
		Steps    []struct {
			DeviceID    int    `json:"device_id"`
			SortOrder   int    `json:"sort_order"`
			WaitSeconds int    `json:"wait_seconds"`
			RetryCount  int    `json:"retry_count"`
			Action      string `json:"action"`
		} `json:"steps"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeJSON(w, http.StatusBadRequest, errBadRequest)
		return
	}
	steps := make([]postgres.PlanStep, len(body.Steps))
	for i, s := range body.Steps {
		steps[i] = postgres.PlanStep{DeviceID: s.DeviceID, SortOrder: s.SortOrder, WaitSeconds: s.WaitSeconds, RetryCount: s.RetryCount, Action: s.Action}
	}
	if err := h.repo.UpdatePlan(r.Context(), id, postgres.UpdatePlanReq{Name: body.Name, PlanType: body.PlanType, Steps: steps}); err != nil {
		serverErr(w, err); return
	}
	ok(w, M{"status": "updated"})
}

func (h *StartupHandler) DeletePlan(w http.ResponseWriter, r *http.Request) {
	if err := h.repo.DeletePlan(r.Context(), pathID(r)); err != nil { serverErr(w, err); return }
	ok(w, M{"status": "deleted"})
}

func (h *StartupHandler) Execute(w http.ResponseWriter, r *http.Request) {
	planID := pathID(r)
	plan, steps, err := orchestrator.LoadPlan(r.Context(), h.pool, planID)
	if err != nil { notFound(w, "启停计划不存在"); return }
	claims := middleware.GetClaims(r)
	if claims == nil { writeJSON(w, http.StatusUnauthorized, M{"error": "未认证"}); return }
	exec, err := orchestrator.StartExecution(r.Context(), h.pool, planID, plan.Name, claims.Username, len(steps))
	if err != nil { serverErr(w, err); return }
	// 从 h.bgCtx 派生超时 context：shutdown 时 h.bgCtx 被取消，编排 goroutine 随之终止。
	// 同时设置 30 分钟超时防止单个编排无限挂起。
	bgCtx, cancel := context.WithTimeout(h.bgCtx, 30*time.Minute)
	go func() {
		defer cancel()
		defer func() {
			if rec := recover(); rec != nil {
				slog.Error("startup execution panic", "plan", plan.Name, "panic", rec)
				c, cancel2 := context.WithTimeout(context.Background(), 10*time.Second)
				defer cancel2()
				if _, err := h.pool.Exec(c, `UPDATE startup_execution SET status='failed',finished_at=NOW() WHERE id=$1`, exec.ID); err != nil {
					slog.Warn("startup execution panic: update status failed", "err", err)
				}
			}
		}()
		exec.Run(bgCtx, steps, h.execDeviceControl)
	}()
	ok(w, map[string]any{"status": "started", "execution_id": exec.ID, "plan_name": plan.Name, "total_steps": len(steps)})
}

func (h *StartupHandler) execDeviceControl(ctx context.Context, devID int, action, targetValue string) (string, error) {
	if err := h.interlock.Check(ctx, devID, action); err != nil { return "", fmt.Errorf("联锁检查失败: %w", err) }
	var devName, bldName, projName string
	err := h.pool.QueryRow(ctx, `SELECT d.name, b.name, COALESCE(p.name,'') FROM device d JOIN building b ON b.id=d.building_id JOIN project p ON p.id=b.project_id WHERE d.id=$1`, devID).Scan(&devName, &bldName, &projName)
	if err != nil { return "", fmt.Errorf("设备 %d 不存在", devID) }
	controlVal, deviceStatus := controlActionCN(action)
	if _, err := h.pool.Exec(ctx,
		`INSERT INTO control_record (project_name,building_name,device_name,device_id,prop_name,control_value,username,user_remark)
		 VALUES ($1,$2,$3,$4,'启停编排',$5,'orchestrator',$6)`,
		projName, bldName, devName, devID, controlVal, action); err != nil {
		slog.Warn("execDeviceControl: control_record insert failed", "dev", devID, "err", err)
	}
	if err := h.dispatchControl(ctx, devID, action); err != nil {
		slog.Error("hardware dispatch failed", "dev", devID, "action", action, "err", err)
		if _, statErr := h.pool.Exec(ctx,
			`UPDATE device SET device_status='通讯故障' WHERE id=$1`, devID); statErr != nil {
			slog.Warn("execDeviceControl: device_status update failed", "dev", devID, "err", statErr)
		}
		return controlVal, fmt.Errorf("硬件调度失败: %w", err)
	}
	if deviceStatus != "" {
		if _, err := h.pool.Exec(ctx,
			`UPDATE device SET device_status=$1 WHERE id=$2`, deviceStatus, devID); err != nil {
			slog.Warn("execDeviceControl: device_status update failed", "dev", devID, "err", err)
		}
	}
	return controlVal, nil
}

func (h *StartupHandler) GetExecution(w http.ResponseWriter, r *http.Request) {
	e, steps, err := h.repo.GetExecution(r.Context(), pathID(r))
	if err != nil || e == nil { notFound(w, "执行记录不存在"); return }
	if steps == nil { steps = []postgres.StepLog{} }
	ok(w, map[string]any{"execution": e, "step_logs": steps})
}

func (h *StartupHandler) StopExecution(w http.ResponseWriter, r *http.Request) {
	affected, err := h.repo.StopExecution(r.Context(), pathID(r))
	if err != nil { serverErr(w, err); return }
	if !affected { notFound(w, "执行记录不存在或已结束"); return }
	ok(w, M{"status": "stopped"})
}

func (h *StartupHandler) ListScheduledTasks(w http.ResponseWriter, r *http.Request) {
	list, err := h.repo.ListScheduledTasks(r.Context(), queryInt(r, "building_id"))
	if err != nil { serverErr(w, err); return }
	if list == nil { list = []postgres.ScheduledTask{} }
	ok(w, list)
}

func (h *StartupHandler) CreateScheduledTask(w http.ResponseWriter, r *http.Request) {
	var t postgres.ScheduledTask
	if err := json.NewDecoder(r.Body).Decode(&t); err != nil { writeJSON(w, 400, M{"error": "格式错误"}); return }
	if t.Name == "" || t.DeviceID == 0 || t.ScheduleTime == "" { writeJSON(w, 400, M{"error": "名称/设备/时间不能为空"}); return }
	id, err := h.repo.CreateScheduledTask(r.Context(), t)
	if err != nil { serverErr(w, err); return }
	t.ID = id
	created(w, t)
}

func (h *StartupHandler) UpdateScheduledTask(w http.ResponseWriter, r *http.Request) {
	var t postgres.ScheduledTask
	if err := json.NewDecoder(r.Body).Decode(&t); err != nil { writeJSON(w, 400, M{"error": "格式错误"}); return }
	t.ID = pathID(r)
	if err := h.repo.UpdateScheduledTask(r.Context(), t.ID, t); err != nil {
		if errors.Is(err, pgx.ErrNoRows) { notFound(w, "定时任务不存在"); return }
		serverErr(w, err); return
	}
	ok(w, M{"status": "updated"})
}

func (h *StartupHandler) DeleteScheduledTask(w http.ResponseWriter, r *http.Request) {
	if err := h.repo.DeleteScheduledTask(r.Context(), pathID(r)); err != nil { serverErr(w, err); return }
	ok(w, M{"status": "deleted"})
}

func (h *StartupHandler) RunDueScheduledTasks(ctx context.Context) {
	tasks, err := h.repo.ListDueScheduledTasks(ctx)
	if err != nil { slog.Error("RunDueScheduledTasks query failed", "err", err); return }
	for _, t := range tasks {
		taskCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
		err := h.executeAction(taskCtx, t.DeviceID, t.ActionType, t.TargetValue)
		cancel()
		result := "success"
		if err != nil {
			result = "failed"
			slog.Warn("scheduled task failed", "id", t.ID, "err", err)
		} else {
			slog.Info("scheduled task completed", "id", t.ID)
		}
		if _, err := h.pool.Exec(ctx,
			`UPDATE scheduled_task SET last_result=$1, last_run_at=NOW() WHERE id=$2`, result, t.ID); err != nil {
			slog.Warn("RunDueScheduledTasks: update result failed", "id", t.ID, "err", err)
		}
	}
}

func (h *StartupHandler) executeAction(ctx context.Context, devID int, action string, targetVal *string) error {
	controlVal, deviceStatus := controlActionCN(action)
	remark := action
	if targetVal != nil && *targetVal != "" {
		remark = action + ":" + *targetVal
	}
	_, err := h.pool.Exec(ctx, `INSERT INTO control_record (project_name,building_name,device_name,device_id,prop_name,control_value,username,user_remark,created_at) SELECT COALESCE(p.name,''), COALESCE(b.name,''), COALESCE(d.name,''), d.id, '定时任务', $2, '定时任务', $3, NOW() FROM device d LEFT JOIN building b ON b.id=d.building_id LEFT JOIN project p ON p.id=b.project_id WHERE d.id=$1`, devID, controlVal, remark)
	if err != nil {
		return err
	}
	if derr := h.dispatchControl(ctx, devID, action); derr != nil {
		slog.Error("scheduled task dispatch failed", "dev", devID, "err", derr)
	}
	if deviceStatus != "" {
		if _, err := h.pool.Exec(ctx,
			`UPDATE device SET device_status=$1 WHERE id=$2`, deviceStatus, devID); err != nil {
			slog.Warn("executeAction: device_status update failed", "dev", devID, "err", err)
		}
	}
	return nil
}

func (h *StartupHandler) dispatchControl(ctx context.Context, devID int, action string) error {
	if h.dev == nil { return errors.New("device handler not wired") }
	dr := h.dev.DispatchHardware(ctx, devID, action)
	if !dr.Dispatched { return fmt.Errorf("hardware dispatch failed: %s", dr.Message) }
	return nil
}