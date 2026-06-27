package orchestrator

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"xmeco/internal/repository/postgres"
)

type Plan struct {
	ID             int
	Name           string
	BuildingID     int
	PrecheckOnline bool
	PrecheckAlarm  bool
}

type Step struct {
	SortOrder      int
	DeviceID       int
	DeviceName     string
	Action         string
	TargetValue    string
	WaitSeconds    int
	SkipIfOffline  bool
	RetryCount     int
}

type Execution struct {
	ID         int
	PlanName   string
	Status     string
	TotalSteps int
	DoneSteps  int
	mu         sync.Mutex
	pool       postgres.DBTX
}

func LoadPlan(ctx context.Context, pool postgres.DBTX, planID int) (*Plan, []Step, error) {
	var p Plan
	err := pool.QueryRow(ctx, `SELECT id,name,building_id,precheck_online,precheck_alarm FROM startup_plan WHERE id=$1`, planID).
		Scan(&p.ID, &p.Name, &p.BuildingID, &p.PrecheckOnline, &p.PrecheckAlarm)
	if err != nil { return nil, nil, err }

	rows, err := pool.Query(ctx,
		`SELECT ss.sort_order, ss.device_id, d.name, ss.action, ss.target_value, ss.wait_seconds, ss.skip_if_offline, COALESCE(ss.retry_count,1)
		 FROM startup_step ss JOIN device d ON d.id=ss.device_id
		 WHERE ss.plan_id=$1 ORDER BY ss.sort_order`, planID)
	if err != nil { return nil, nil, err }
	defer rows.Close()
	var steps []Step
	for rows.Next() {
		var s Step
		if err := rows.Scan(&s.SortOrder, &s.DeviceID, &s.DeviceName, &s.Action, &s.TargetValue, &s.WaitSeconds, &s.SkipIfOffline, &s.RetryCount); err != nil {
			slog.Warn("LoadPlan scan step failed", "plan", planID, "err", err)
			continue
		}
		steps = append(steps, s)
	}
	return &p, steps, rows.Err()
}

func StartExecution(ctx context.Context, pool postgres.DBTX, planID int, planName, triggeredBy string, totalSteps int) (*Execution, error) {
	var id int
	err := pool.QueryRow(ctx,
		`INSERT INTO startup_execution (plan_id,plan_name,triggered_by,status,total_steps,started_at) VALUES($1,$2,$3,'running',$4,$5) RETURNING id`,
		planID, planName, triggeredBy, totalSteps, time.Now()).Scan(&id)
	if err != nil { return nil, err }
	return &Execution{ID: id, PlanName: planName, Status: "running", TotalSteps: totalSteps, pool: pool}, nil
}

func (e *Execution) LogStep(ctx context.Context, step Step, result, responseValue, errMsg string, durationMs int) {
	e.mu.Lock()
	if result == "success" || result == "skip" { e.DoneSteps++ } else if result == "error" { e.Status = "error" }
	doneSteps := e.DoneSteps
	status := e.Status
	e.mu.Unlock()
	_, err := e.pool.Exec(ctx,
		`INSERT INTO startup_step_log (execution_id,step_order,device_name,action,target_value,result,response_value,duration_ms,error_message) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9)`,
		e.ID, step.SortOrder, step.DeviceName, step.Action, step.TargetValue, result, responseValue, durationMs, errMsg)
	if err != nil { slog.Warn("step log failed", "err", err) }
	// 同步更新 DB 的 done_steps 和 status，让前端能实时看到执行进度
	if _, err := e.pool.Exec(ctx, `UPDATE startup_execution SET done_steps=$1, status=$2 WHERE id=$3`,
		doneSteps, status, e.ID); err != nil {
		slog.Warn("execution progress update failed", "exec", e.ID, "err", err)
	}
}

func (e *Execution) Finish(ctx context.Context) {
	e.mu.Lock()
	if e.Status == "running" { e.Status = "completed" }
	status := e.Status
	doneSteps := e.DoneSteps
	e.mu.Unlock()
	if _, err := e.pool.Exec(ctx, `UPDATE startup_execution SET status=$1,done_steps=$2,finished_at=$3 WHERE id=$4`,
		status, doneSteps, time.Now(), e.ID); err != nil {
		slog.Warn("execution finish update failed", "exec", e.ID, "err", err)
	}
	slog.Info("execution finished", "plan", e.PlanName, "status", status)
}

// isStopped checks if the execution has been externally marked as stopped in the DB.
func (e *Execution) isStopped(ctx context.Context) bool {
	var status string
	err := e.pool.QueryRow(ctx, `SELECT status FROM startup_execution WHERE id=$1`, e.ID).Scan(&status)
	if err != nil {
		return false
	}
	return status == "stopped"
}

func (e *Execution) Run(ctx context.Context, steps []Step, execFn func(ctx context.Context, devID int, action, target string) (string, error)) {
	defer e.Finish(ctx)
	setStopped := func() {
		e.mu.Lock()
		e.Status = "stopped"
		e.mu.Unlock()
	}
	for _, step := range steps {
		select {
		case <-ctx.Done():
			setStopped()
			return
		default:
		}

		// Check if execution was stopped via the API (StopExecution updates DB)
		if e.isStopped(ctx) {
			setStopped()
			return
		}

		maxRetries := step.RetryCount
		if maxRetries < 1 { maxRetries = 1 }
		var lastErr error
		var resp string
		for attempt := 1; attempt <= maxRetries; attempt++ {
			start := time.Now()
			resp, lastErr = execFn(ctx, step.DeviceID, step.Action, step.TargetValue)
			ms := int(time.Since(start).Milliseconds())
			if lastErr == nil {
				e.LogStep(ctx, step, "success", resp, "", ms)
				break
			}
			if attempt < maxRetries {
				slog.Warn("step retry", "plan", e.PlanName, "dev", step.DeviceName, "attempt", attempt, "err", lastErr)
			} else {
				e.LogStep(ctx, step, "error", "", lastErr.Error(), ms)
			}
		}
		if lastErr != nil {
			return
		}
		if step.WaitSeconds > 0 {
			timer := time.NewTimer(time.Duration(step.WaitSeconds) * time.Second)
			select {
			case <-timer.C:
			case <-ctx.Done():
				timer.Stop()
				setStopped()
				return
			}
		}
	}
}
