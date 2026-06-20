package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"xmeco/internal/api/handler"
	"xmeco/internal/api/middleware"
	"xmeco/internal/config"
	"xmeco/internal/gateway"
	"xmeco/internal/repository/postgres"
	"xmeco/internal/service/auth"
	"xmeco/internal/service/telemetry"
)

func main() {
	cfg := config.Load()
	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo})))
	slog.Info("XMECO starting")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	db, err := postgres.New(ctx, cfg.DSN())
	if err != nil { slog.Error("db connect failed", "error", err); os.Exit(1) }
	defer db.Close()
	slog.Info("db connected")

	pool := db.Pool
	// auto migrate
	pool.Exec(ctx, "ALTER TABLE project ADD COLUMN IF NOT EXISTS admin_code VARCHAR(20)")
	pool.Exec(ctx, "ALTER TABLE device RENAME COLUMN gateway_mac TO gateway_imei")
	pool.Exec(ctx, "ALTER TABLE device_properties ADD COLUMN IF NOT EXISTS min_value VARCHAR(50)")
	pool.Exec(ctx, "ALTER TABLE device_properties ADD COLUMN IF NOT EXISTS max_value VARCHAR(50)")
	pool.Exec(ctx, "ALTER TABLE register ADD COLUMN IF NOT EXISTS name VARCHAR(100)")
	pool.Exec(ctx, "ALTER TABLE register ADD COLUMN IF NOT EXISTS command_code VARCHAR(50)")
	pool.Exec(ctx, "ALTER TABLE alarm_rule ADD COLUMN IF NOT EXISTS device_id INT")
	pool.Exec(ctx, "ALTER TABLE alarm_rule ADD COLUMN IF NOT EXISTS property_id INT")
	pool.Exec(ctx, "ALTER TABLE alarm_rule ADD COLUMN IF NOT EXISTS target_value VARCHAR(100)")
	pool.Exec(ctx, "ALTER TABLE alarm_rule ADD COLUMN IF NOT EXISTS min_value VARCHAR(50)")
	pool.Exec(ctx, "ALTER TABLE alarm_rule ADD COLUMN IF NOT EXISTS max_value VARCHAR(50)")
	pool.Exec(ctx, "ALTER TABLE alarm_rule ADD COLUMN IF NOT EXISTS notify_users JSONB DEFAULT '[]'")
	pool.Exec(ctx, "ALTER TABLE alarm_rule ADD COLUMN IF NOT EXISTS name VARCHAR(100)")
	pool.Exec(ctx, "ALTER TABLE startup_plan ADD COLUMN IF NOT EXISTS plan_type VARCHAR(20) DEFAULT 'startup'")
	pool.Exec(ctx, "CREATE TABLE IF NOT EXISTS dashboard_config (key VARCHAR(50) PRIMARY KEY, value TEXT, updated_at TIMESTAMPTZ DEFAULT NOW())")
	pool.Exec(ctx, "INSERT INTO dashboard_config (key,value) VALUES ('service_projects','156'),('service_area','12.8万㎡'),('service_cities','8'),('power_saved','1,245'),('carbon_saved','986'),('days_start','2021-01-01') ON CONFLICT DO NOTHING")

	authSvc := auth.New(pool, cfg.JWTSecret)
	ah := handler.NewAuthHandler(authSvc)
	ph := handler.NewProjectHandler(postgres.NewProjectRepo(pool))
	bh := handler.NewBuildingHandler(postgres.NewBuildingRepo(pool))
	dh := handler.NewDeviceHandler(postgres.NewDeviceRepo(pool))
	pph := handler.NewPropertyHandler(postgres.NewPropertyRepo(pool))
	rh := handler.NewRegisterHandler(postgres.NewRegisterRepo(pool))
	alh := handler.NewAlarmHandler(pool)
	sh := handler.NewStartupHandler(pool)
	th := handler.NewTelemetryHandler(pool)
	logH := handler.NewLogHandler(pool)
	dashH := handler.NewDashboardHandler(pool)
	admh := handler.NewAdminHandler(postgres.NewAdminRepo(pool), authSvc)

	ctxGW, cancelGW := context.WithCancel(context.Background())
	defer cancelGW()
	poller := telemetry.NewPoller(pool, nil)
	gwMgr := gateway.NewManager(poller.PollDevice)
	poller.GwMgr = gwMgr
	gwMgr.StartCustomListener(ctxGW, ":8081")
	gwMgr.StartDTUListener(ctxGW, ":502")

	mux := http.NewServeMux()
	mux.HandleFunc("/api/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if err := db.Health(r.Context()); err != nil {
			w.WriteHeader(http.StatusServiceUnavailable)
			fmt.Fprintf(w, `{"status":"unhealthy","error":"%s"}`, err.Error())
			return
		}
		fmt.Fprintf(w, `{"status":"ok","service":"XMECO","time":"%s"}`, time.Now().Format(time.RFC3339))
	})
	mux.HandleFunc("POST /api/v1/auth/login", ah.Login)

	protected := http.NewServeMux()
	protected.HandleFunc("GET /api/v1/auth/me", ah.Me)
	protected.HandleFunc("GET /api/v1/projects", ph.List)
	protected.HandleFunc("POST /api/v1/projects", ph.Create)
	protected.HandleFunc("GET /api/v1/projects/{id}", ph.Get)
	protected.HandleFunc("PUT /api/v1/projects/{id}", ph.Update)
	protected.HandleFunc("DELETE /api/v1/projects/{id}", ph.Delete)
	protected.HandleFunc("GET /api/v1/buildings", bh.List)
	protected.HandleFunc("GET /api/v1/buildings/{id}", bh.Get)
	protected.HandleFunc("POST /api/v1/buildings", bh.Create)
	protected.HandleFunc("PUT /api/v1/buildings/{id}", bh.Update)
	protected.HandleFunc("DELETE /api/v1/buildings/{id}", bh.Delete)
	protected.HandleFunc("GET /api/v1/devices", dh.List)
	protected.HandleFunc("GET /api/v1/devices/{id}", dh.Get)
	protected.HandleFunc("POST /api/v1/devices", dh.Create)
	protected.HandleFunc("PUT /api/v1/devices/{id}", dh.Update)
	protected.HandleFunc("DELETE /api/v1/devices/{id}", dh.Delete)
	protected.HandleFunc("GET /api/v1/properties", pph.List)
	protected.HandleFunc("GET /api/v1/properties/{id}", pph.Get)
	protected.HandleFunc("POST /api/v1/properties", pph.Create)
	protected.HandleFunc("PUT /api/v1/properties/{id}", pph.Update)
	protected.HandleFunc("DELETE /api/v1/properties/{id}", pph.Delete)
	protected.HandleFunc("GET /api/v1/registers", rh.List)
	protected.HandleFunc("GET /api/v1/registers/{id}", rh.Get)
	protected.HandleFunc("POST /api/v1/registers", rh.Create)
	protected.HandleFunc("PUT /api/v1/registers/{id}", rh.Update)
	protected.HandleFunc("DELETE /api/v1/registers/{id}", rh.Delete)
	protected.HandleFunc("GET /api/v1/alarm-rules", alh.ListRules)
	protected.HandleFunc("POST /api/v1/alarm-rules", alh.CreateRule)
	protected.HandleFunc("PUT /api/v1/alarm-rules/{id}", alh.UpdateRule)
	protected.HandleFunc("DELETE /api/v1/alarm-rules/{id}", alh.DeleteRule)
	protected.HandleFunc("GET /api/v1/alarm-logs", alh.ListLogs)
	protected.HandleFunc("POST /api/v1/alarm-logs/{id}/ack", alh.AckLog)
	protected.HandleFunc("GET /api/v1/startup-plans", sh.ListPlans)
	protected.HandleFunc("POST /api/v1/startup-plans", sh.CreatePlan)
	protected.HandleFunc("PUT /api/v1/startup-plans/{id}", sh.UpdatePlan)
	protected.HandleFunc("DELETE /api/v1/startup-plans/{id}", sh.DeletePlan)
	protected.HandleFunc("POST /api/v1/startup-plans/{id}/execute", sh.Execute)
	protected.HandleFunc("GET /api/v1/startup-executions/{id}", sh.GetExecution)
	protected.HandleFunc("POST /api/v1/startup-executions/{id}/stop", sh.StopExecution)
	protected.HandleFunc("GET /api/v1/telemetry/realtime", th.Realtime)
	protected.HandleFunc("GET /api/v1/telemetry/history", th.History)
	protected.HandleFunc("GET /api/v1/telemetry/stats", th.Stats)
	protected.HandleFunc("GET /api/v1/gateways", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		b, _ := json.Marshal(gwMgr.ListGateways())
		fmt.Fprintf(w, `{"gateways":%s}`, b)
	})
	protected.HandleFunc("GET /api/v1/logs/telemetry", logH.Telemetry)
	protected.HandleFunc("GET /api/v1/logs/controls", logH.Controls)
	protected.HandleFunc("GET /api/v1/logs/stats", logH.Stats)
	protected.HandleFunc("GET /api/v1/dashboard", dashH.GetConfig)
	protected.HandleFunc("PUT /api/v1/dashboard", dashH.UpdateConfig)
	protected.HandleFunc("GET /api/v1/users", admh.ListUsers)
	protected.HandleFunc("POST /api/v1/users", admh.CreateUser)
	protected.HandleFunc("PUT /api/v1/users/{id}", admh.UpdateUser)
	protected.HandleFunc("POST /api/v1/users/{id}/reset-password", admh.ResetPassword)
	protected.HandleFunc("DELETE /api/v1/users/{id}", admh.DeleteUser)
	protected.HandleFunc("GET /api/v1/agents", admh.ListAgents)
	protected.HandleFunc("POST /api/v1/agents", admh.CreateAgent)
	protected.HandleFunc("PUT /api/v1/agents/{id}", admh.UpdateAgent)
	protected.HandleFunc("DELETE /api/v1/agents/{id}", admh.DeleteAgent)
	protected.HandleFunc("GET /api/v1/roles", admh.ListRoles)
	protected.HandleFunc("GET /api/v1/permissions", admh.ListPermissions)
	protected.HandleFunc("GET /api/v1/roles/{id}/permissions", admh.GetRolePermissions)
	protected.HandleFunc("PUT /api/v1/roles/{id}/permissions", admh.SetRolePermissions)

	mux.Handle("/api/v1/", middleware.AuthMiddleware(authSvc)(protected))

	addr := ":" + cfg.ServerPort
	srv := &http.Server{Addr: addr, Handler: mux}
	go func() {
		quit := make(chan os.Signal, 1)
		signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
		<-quit
		slog.Info("shutting down...")
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		srv.Shutdown(ctx)
		gwMgr.Shutdown()
	}()

	slog.Info("XMECO running", "http", addr, "customGW", ":8081", "dtuGW", ":502")
	if err := srv.ListenAndServe(); err != http.ErrServerClosed {
		slog.Error("server error", "error", err)
		os.Exit(1)
	}
	slog.Info("server stopped")
}