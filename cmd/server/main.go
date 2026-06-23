package main

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"xmeco/internal/api/handler"
	"xmeco/internal/api/middleware"
	"xmeco/internal/config"
	"xmeco/internal/gateway"
	"xmeco/internal/repository/postgres"
	"xmeco/internal/service/alarm"
	"xmeco/internal/service/auth"
	"xmeco/internal/service/migration"
	"xmeco/internal/service/telemetry"
)

// withPerm wraps a handler with RBAC permission check.
func withPerm(authSvc *auth.Service, permCode string, next http.HandlerFunc) http.HandlerFunc {
	return middleware.RequirePermission(authSvc, permCode)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		next(w, r)
	})).ServeHTTP
}

func main() {
	cfg := config.Load()
	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo})))
	slog.Info("XMECO starting")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	db, err := postgres.New(ctx, cfg.DSN())
	if err != nil {
		slog.Error("db connect failed", "error", err)
		os.Exit(1)
	}
	defer db.Close()
	slog.Info("db connected")

	pool := db.Pool

	// ---- migrations ----
	if err := migration.Run(context.Background(), pool); err != nil {
		slog.Error("migration failed", "error", err)
		os.Exit(1)
	}

	// ---- services ----
	authSvc := auth.New(pool, cfg.JWTSecret)

	// ---- handlers ----
	ah := handler.NewAuthHandler(authSvc)
	ph := handler.NewProjectHandler(postgres.NewProjectRepo(pool))
	bh := handler.NewBuildingHandler(postgres.NewBuildingRepo(pool))
	dh := handler.NewDeviceHandler(postgres.NewDeviceRepo(pool), pool)
	pph := handler.NewPropertyHandler(postgres.NewPropertyRepo(pool))
	rh := handler.NewRegisterHandler(postgres.NewRegisterRepo(pool))
	alh := handler.NewAlarmHandler(pool)
	sh := handler.NewStartupHandler(pool)
	th := handler.NewTelemetryHandler(pool)
	logH := handler.NewLogHandler(pool)
	dashH := handler.NewDashboardHandler(pool)
	admh := handler.NewAdminHandler(postgres.NewAdminRepo(pool), authSvc)
	wh := handler.NewWeatherHandler(pool)
	ih := handler.NewIntelligenceHandler(pool)

	// ---- gateway & telemetry ----
	ctxGW, cancelGW := context.WithCancel(context.Background())
	defer cancelGW()
	poller := telemetry.NewPoller(pool)
	gwMgr := gateway.NewManager(poller.PollDevice, loadDevicesForGateway(pool), pool)
	dh.SetGwMgr(gwMgr) // enable device control via gateway
	gwMgr.StartCustomListener(ctxGW, ":8081")
	gwMgr.StartDTUListener(ctxGW, ":502")

	// ---- offline detection: mark devices offline after 10 min, create alarm log ----
	go func() {
		ticker := time.NewTicker(1 * time.Minute)
		defer ticker.Stop()
		alarmEngine := alarm.New(pool)
		for range ticker.C {
			// Find devices that should be marked offline
			rows, err := pool.Query(ctx,
				`SELECT id, COALESCE(name,'') FROM device
				 WHERE online_status='在线' AND last_online_at < NOW() - INTERVAL '10 minutes'`)
			if err != nil {
				slog.Warn("offline detection query failed", "err", err)
				continue
			}
			for rows.Next() {
				var id int
				var name string
				if err := rows.Scan(&id, &name); err != nil {
					continue
				}
				// Create alarm log (deduplicated: only one un-acked offline alarm per device)
				if err := alarmEngine.AlertOffline(ctx, id, name); err != nil {
					slog.Warn("offline alarm create failed", "dev", id, "err", err)
				}
			}
			rows.Close()
			// Update status
			if _, err := pool.Exec(ctx,
				`UPDATE device SET online_status='离线'
				 WHERE online_status='在线' AND last_online_at < NOW() - INTERVAL '10 minutes'`); err != nil {
				slog.Warn("offline status update failed", "err", err)
			}
		}
	}()

	// ---- rate limiter: 10 login attempts per minute per IP ----
	rateLimiter := middleware.NewRateLimiter(10, 1*time.Minute)

	// ---- routes ----
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
	mux.HandleFunc("POST /api/v1/auth/login", rateLimiter.LimitLogin(ah.Login))

	protected := http.NewServeMux()
	protected.HandleFunc("GET /api/v1/auth/me", ah.Me)
	protected.HandleFunc("GET /api/v1/projects", withPerm(authSvc, "project.view", ph.List))
	protected.HandleFunc("POST /api/v1/projects", withPerm(authSvc, "project.create", ph.Create))
	protected.HandleFunc("GET /api/v1/projects/{id}", withPerm(authSvc, "project.view", ph.Get))
	protected.HandleFunc("PUT /api/v1/projects/{id}", withPerm(authSvc, "project.edit", ph.Update))
	protected.HandleFunc("DELETE /api/v1/projects/{id}", withPerm(authSvc, "project.delete", ph.Delete))
	protected.HandleFunc("GET /api/v1/buildings", withPerm(authSvc, "building.view", bh.List))
	protected.HandleFunc("GET /api/v1/buildings/{id}", withPerm(authSvc, "building.view", bh.Get))
	protected.HandleFunc("POST /api/v1/buildings", withPerm(authSvc, "building.create", bh.Create))
	protected.HandleFunc("PUT /api/v1/buildings/{id}", withPerm(authSvc, "building.edit", bh.Update))
	protected.HandleFunc("DELETE /api/v1/buildings/{id}", withPerm(authSvc, "building.delete", bh.Delete))
	protected.HandleFunc("GET /api/v1/devices", withPerm(authSvc, "device.view", dh.List))
	protected.HandleFunc("GET /api/v1/devices/{id}", withPerm(authSvc, "device.view", dh.Get))
	protected.HandleFunc("POST /api/v1/devices", withPerm(authSvc, "device.create", dh.Create))
	protected.HandleFunc("PUT /api/v1/devices/{id}", withPerm(authSvc, "device.edit", dh.Update))
	protected.HandleFunc("DELETE /api/v1/devices/{id}", withPerm(authSvc, "device.delete", dh.Delete))
	protected.HandleFunc("POST /api/v1/devices/{id}/control", withPerm(authSvc, "device.control", dh.Control))
	protected.HandleFunc("GET /api/v1/properties", withPerm(authSvc, "device.view", pph.List))
	protected.HandleFunc("GET /api/v1/properties/{id}", withPerm(authSvc, "device.view", pph.Get))
	protected.HandleFunc("POST /api/v1/properties", withPerm(authSvc, "device.property", pph.Create))
	protected.HandleFunc("PUT /api/v1/properties/{id}", withPerm(authSvc, "device.property", pph.Update))
	protected.HandleFunc("DELETE /api/v1/properties/{id}", withPerm(authSvc, "device.property", pph.Delete))
	protected.HandleFunc("GET /api/v1/registers", withPerm(authSvc, "device.view", rh.List))
	protected.HandleFunc("GET /api/v1/registers/{id}", withPerm(authSvc, "device.view", rh.Get))
	protected.HandleFunc("POST /api/v1/registers", withPerm(authSvc, "device.register", rh.Create))
	protected.HandleFunc("PUT /api/v1/registers/{id}", withPerm(authSvc, "device.register", rh.Update))
	protected.HandleFunc("DELETE /api/v1/registers/{id}", withPerm(authSvc, "device.register", rh.Delete))
	protected.HandleFunc("GET /api/v1/alarm-rules", withPerm(authSvc, "monitor.alarm_config", alh.ListRules))
	protected.HandleFunc("POST /api/v1/alarm-rules", withPerm(authSvc, "monitor.alarm_config", alh.CreateRule))
	protected.HandleFunc("PUT /api/v1/alarm-rules/{id}", withPerm(authSvc, "monitor.alarm_config", alh.UpdateRule))
	protected.HandleFunc("DELETE /api/v1/alarm-rules/{id}", withPerm(authSvc, "monitor.alarm_config", alh.DeleteRule))
	protected.HandleFunc("GET /api/v1/alarm-logs", withPerm(authSvc, "monitor.realtime", alh.ListLogs))
	protected.HandleFunc("POST /api/v1/alarm-logs/{id}/ack", withPerm(authSvc, "monitor.alarm_config", alh.AckLog))
	protected.HandleFunc("GET /api/v1/startup-plans", withPerm(authSvc, "schedule.view", sh.ListPlans))
	protected.HandleFunc("POST /api/v1/startup-plans", withPerm(authSvc, "schedule.create", sh.CreatePlan))
	protected.HandleFunc("PUT /api/v1/startup-plans/{id}", withPerm(authSvc, "schedule.edit", sh.UpdatePlan))
	protected.HandleFunc("DELETE /api/v1/startup-plans/{id}", withPerm(authSvc, "schedule.delete", sh.DeletePlan))
	protected.HandleFunc("POST /api/v1/startup-plans/{id}/execute", withPerm(authSvc, "schedule.create", sh.Execute))
	protected.HandleFunc("GET /api/v1/startup-executions/{id}", withPerm(authSvc, "schedule.view", sh.GetExecution))
	protected.HandleFunc("POST /api/v1/startup-executions/{id}/stop", withPerm(authSvc, "schedule.edit", sh.StopExecution))
	protected.HandleFunc("GET /api/v1/telemetry/realtime", withPerm(authSvc, "monitor.realtime", th.Realtime))
	protected.HandleFunc("GET /api/v1/telemetry/history", withPerm(authSvc, "monitor.graph", th.History))
	protected.HandleFunc("GET /api/v1/telemetry/stats", withPerm(authSvc, "monitor.graph", th.Stats))
	protected.HandleFunc("GET /api/v1/gateways", withPerm(authSvc, "system.gateway", gwMgr.HandleListGateways))
	protected.HandleFunc("GET /api/v1/export/telemetry", withPerm(authSvc, "report.export", logH.ExportTelemetry))
	protected.HandleFunc("GET /api/v1/export/controls", withPerm(authSvc, "report.excel", logH.ExportControls))
	protected.HandleFunc("GET /api/v1/system/info", withPerm(authSvc, "system.config", admh.SystemInfo))
	protected.HandleFunc("GET /api/v1/system/db-stats", withPerm(authSvc, "system.db", admh.DBStats))
	protected.HandleFunc("GET /api/v1/logs/telemetry", withPerm(authSvc, "monitor.realtime", logH.Telemetry))
	protected.HandleFunc("GET /api/v1/logs/controls", withPerm(authSvc, "monitor.control_log", logH.Controls))
	protected.HandleFunc("GET /api/v1/logs/stats", withPerm(authSvc, "monitor.realtime", logH.Stats))
	protected.HandleFunc("GET /api/v1/dashboard", withPerm(authSvc, "monitor.realtime", dashH.GetConfig))
	protected.HandleFunc("PUT /api/v1/dashboard", withPerm(authSvc, "monitor.realtime", dashH.UpdateConfig))
	// ---- 权限管理 ----
	protected.HandleFunc("GET /api/v1/users", withPerm(authSvc, "user.view", admh.ListUsers))
	protected.HandleFunc("POST /api/v1/users", withPerm(authSvc, "user.create", admh.CreateUser))
	protected.HandleFunc("PUT /api/v1/users/{id}", withPerm(authSvc, "user.edit", admh.UpdateUser))
	protected.HandleFunc("POST /api/v1/users/{id}/reset-password", withPerm(authSvc, "user.edit", admh.ResetPassword))
	protected.HandleFunc("DELETE /api/v1/users/{id}", withPerm(authSvc, "user.delete", admh.DeleteUser))
	protected.HandleFunc("GET /api/v1/agents", withPerm(authSvc, "agent.view", admh.ListAgents))
	protected.HandleFunc("POST /api/v1/agents", withPerm(authSvc, "agent.create", admh.CreateAgent))
	protected.HandleFunc("PUT /api/v1/agents/{id}", withPerm(authSvc, "agent.edit", admh.UpdateAgent))
	protected.HandleFunc("DELETE /api/v1/agents/{id}", withPerm(authSvc, "agent.delete", admh.DeleteAgent))
	protected.HandleFunc("GET /api/v1/roles", withPerm(authSvc, "user.view", admh.ListRoles))
	protected.HandleFunc("GET /api/v1/permissions", withPerm(authSvc, "user.view", admh.ListPermissions))
	protected.HandleFunc("GET /api/v1/roles/{id}/permissions", withPerm(authSvc, "user.assign_role", admh.GetRolePermissions))
	protected.HandleFunc("PUT /api/v1/roles/{id}/permissions", withPerm(authSvc, "user.assign_role", admh.SetRolePermissions))
	// ---- 天气 ----
	protected.HandleFunc("GET /api/v1/weather/cities", withPerm(authSvc, "project.view", wh.ListCities))
	protected.HandleFunc("GET /api/v1/weather/provinces", withPerm(authSvc, "project.view", wh.ListProvinceCities))
	protected.HandleFunc("GET /api/v1/weather/cities/{id}", withPerm(authSvc, "project.view", wh.GetCity))
	protected.HandleFunc("GET /api/v1/weather/now", withPerm(authSvc, "monitor.realtime", wh.Now))
	protected.HandleFunc("GET /api/v1/weather/project", withPerm(authSvc, "monitor.realtime", wh.ProjectWeather))
	// ---- 智能分析 ----
	protected.HandleFunc("GET /api/v1/intelligence/full", withPerm(authSvc, "monitor.graph", ih.FullAnalysis))
	protected.HandleFunc("GET /api/v1/intelligence/efficiency", withPerm(authSvc, "monitor.graph", ih.Efficiency))
	protected.HandleFunc("GET /api/v1/intelligence/forecast", withPerm(authSvc, "monitor.graph", ih.Forecast))
	protected.HandleFunc("GET /api/v1/intelligence/recommendations", withPerm(authSvc, "monitor.graph", ih.Recommendations))
	protected.HandleFunc("GET /api/v1/intelligence/strategies", withPerm(authSvc, "monitor.graph", ih.Strategies))
	protected.HandleFunc("GET /api/v1/intelligence/price-config", withPerm(authSvc, "monitor.graph", ih.PriceConfig))
	protected.HandleFunc("PUT /api/v1/intelligence/price-config", withPerm(authSvc, "monitor.graph", ih.SavePriceConfig))

	mux.Handle("/api/v1/", middleware.AuthMiddleware(authSvc)(protected))

	addr := ":" + cfg.ServerPort
	srv := &http.Server{Addr: addr, Handler: middleware.CORS(mux)}
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

// loadDevicesForGateway returns a DeviceLoaderFn that queries the DB for
// devices whose gateway_imei matches the gateway ID.
func loadDevicesForGateway(pool *pgxpool.Pool) gateway.DeviceLoaderFn {
	return func(ctx context.Context, gwID string) ([]gateway.DeviceRef, error) {
		rows, err := pool.Query(ctx,
			`SELECT id, device_no, device_type, COALESCE(name,''), node_address
			 FROM device WHERE gateway_imei=$1 AND gateway_imei IS NOT NULL AND gateway_imei!=''`, gwID)
		if err != nil {
			return nil, err
		}
		defer rows.Close()
		var devs []gateway.DeviceRef
		for rows.Next() {
			var d gateway.DeviceRef
			if err := rows.Scan(&d.DeviceID, &d.DeviceNo, &d.DeviceType, &d.DeviceName, &d.NodeAddr); err != nil {
				return nil, err
			}
			devs = append(devs, d)
		}
		return devs, rows.Err()
	}
}

