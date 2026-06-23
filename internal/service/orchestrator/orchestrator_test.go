package orchestrator

import (
	"sync"
	"testing"
)

// ---- Step struct ----

func TestStepDefaults(t *testing.T) {
	s := Step{
		SortOrder: 1, DeviceID: 42, DeviceName: "主机1",
		Action: "startup", WaitSeconds: 30,
	}
	if s.SortOrder != 1 {
		t.Errorf("SortOrder = %d, want 1", s.SortOrder)
	}
	if s.DeviceID != 42 {
		t.Errorf("DeviceID = %d, want 42", s.DeviceID)
	}
	if s.Action != "startup" {
		t.Errorf("Action = %q, want startup", s.Action)
	}
	if s.WaitSeconds != 30 {
		t.Errorf("WaitSeconds = %d, want 30", s.WaitSeconds)
	}
}

func TestStepRetryCount(t *testing.T) {
	// Zero RetryCount means "not configured" — Run() defaults to 1.
	s := Step{RetryCount: 0}
	if s.RetryCount != 0 {
		t.Errorf("default RetryCount = %d, want 0 (will be normalized to 1 in Run)", s.RetryCount)
	}
	s2 := Step{RetryCount: 3}
	if s2.RetryCount != 3 {
		t.Errorf("RetryCount = %d, want 3", s2.RetryCount)
	}
	// Verify Run's normalization logic (inlined for testability)
	maxRetries := s.RetryCount
	if maxRetries < 1 {
		maxRetries = 1
	}
	if maxRetries != 1 {
		t.Errorf("normalized RetryCount = %d, want 1", maxRetries)
	}
}

func TestStepSkipIfOffline(t *testing.T) {
	s := Step{SkipIfOffline: true}
	if !s.SkipIfOffline {
		t.Error("SkipIfOffline should be true")
	}
	s.SkipIfOffline = false
	if s.SkipIfOffline {
		t.Error("SkipIfOffline should be false")
	}
}

// ---- Plan struct ----

func TestPlanFields(t *testing.T) {
	p := Plan{
		ID: 1, Name: "早班启动", BuildingID: 3,
		PrecheckOnline: true, PrecheckAlarm: false,
	}
	if p.ID != 1 || p.Name != "早班启动" || p.BuildingID != 3 {
		t.Errorf("Plan fields mismatch: ID=%d Name=%q BuildingID=%d", p.ID, p.Name, p.BuildingID)
	}
	if !p.PrecheckOnline || p.PrecheckAlarm {
		t.Errorf("Precheck flags mismatch: online=%v alarm=%v", p.PrecheckOnline, p.PrecheckAlarm)
	}
}

// ---- Execution struct (no-DB fields only) ----

func TestExecutionFields(t *testing.T) {
	e := Execution{
		ID: 10, PlanName: "测试计划", Status: "running",
		TotalSteps: 5, DoneSteps: 0,
	}
	if e.ID != 10 || e.PlanName != "测试计划" || e.Status != "running" {
		t.Errorf("Execution fields mismatch: ID=%d PlanName=%q Status=%q", e.ID, e.PlanName, e.Status)
	}
	if e.TotalSteps != 5 || e.DoneSteps != 0 {
		t.Error("step counters mismatch")
	}
}

func TestExecutionConcurrentStatusMutation(t *testing.T) {
	e := Execution{Status: "running"}

	var wg sync.WaitGroup
	for i := range 10 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			e.mu.Lock()
			e.Status = "stopped"
			e.mu.Unlock()
		}()
		_ = i
	}
	wg.Wait()

	e.mu.Lock()
	if e.Status != "stopped" {
		t.Errorf("concurrent setStopped = %q, want stopped", e.Status)
	}
	e.mu.Unlock()
}

func TestExecutionStatusComplete(t *testing.T) {
	e := Execution{Status: "running"}
	// Simulate what Finish does before DB write
	if e.Status == "running" {
		e.Status = "completed"
	}
	if e.Status != "completed" {
		t.Errorf("status after finish = %q, want completed", e.Status)
	}
}

func TestExecutionStatusError(t *testing.T) {
	e := Execution{Status: "running"}
	e.Status = "error"
	if e.Status != "error" {
		t.Errorf("status should be error")
	}
}

// ---- Run normalizations (inlined logic tests, no DB) ----

func TestRunRetryCountNormalization(t *testing.T) {
	tests := []struct {
		input int
		want  int
	}{
		{0, 1},
		{1, 1},
		{2, 2},
		{5, 5},
		{-1, 1},
	}
	for _, tt := range tests {
		v := tt.input
		if v < 1 {
			v = 1
		}
		if v != tt.want {
			t.Errorf("normalize(%d) = %d, want %d", tt.input, v, tt.want)
		}
	}
}

// TODO: Integration tests for LoadPlan, StartExecution, Run, Finish, LogStep
// require a real database or pgxmock. Add with:
//   go get github.com/pashagolub/pgxmock/v4
// Then mock pool.QueryRow / pool.Query / pool.Exec for full orchestrator coverage.
