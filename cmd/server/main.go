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

	"xmeco/internal/api/handler"
	"xmeco/internal/api/middleware"
	"xmeco/internal/config"
	"xmeco/internal/gateway"
	"xmeco/internal/repository/postgres"
	"xmeco/internal/safego"
	"xmeco/internal/service/alarm"
	"xmeco/internal/service/auth"
	"xmeco/internal/service/external/weather"
	"xmeco/internal/service/migration"
	"xmeco/internal/service/telemetry"
)

func main() {
	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo})))
	if err := run(); err != nil {
		slog.Error("XMECO fatal", "error", err)
		os.Exit(1)
	}
}

// run is the real main — returning an error lets deferred cleanup (db.Close, etc.)
// execute before exit, which os.Exit would skip.
func run() error {
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("config load: %w", err)
	}
	slog.Info("XMECO starting")

	startupCtx, startupCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer startupCancel()
	db, err := postgres.New(startupCtx, cfg.DSN())
	if err != nil {
		return fmt.Errorf("db connect: %w", err)
	}
	defer db.Close()
	slog.Info("db connected", "dsn", cfg.MaskedDSN())

	pool := db.Pool

	// ---- migrations (use longer timeout for first-run schema creation) ----
	migrateCtx, migrateCancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer migrateCancel()
	if err := migration.Run(migrateCtx, pool); err != nil {
		return fmt.Errorf("migration: %w", err)
	}

	// ---- services ----
	authSvc := auth.New(pool, cfg.JWTSecret)

	// ---- repositories (新增) ----
	alarmRuleRepo := postgres.NewAlarmRuleRepo(pool)
	alarmLogRepo := postgres.NewAlarmLogRepo(pool)
	telemetryRepo := postgres.NewTelemetryRepo(pool)
	logRepo := postgres.NewLogRepo(pool)
	maintenanceRepo := postgres.NewMaintenanceRepo(pool)
	startupRepo := postgres.NewStartupRepo(pool)

	// ---- handlers ----
	weatherSvc := weather.New(pool)
	deviceRepo := postgres.NewDeviceRepo(pool)
	ah := handler.NewAuthHandler(authSvc)
	ph := handler.NewProjectHandler(postgres.NewProjectRepo(pool))
	bh := handler.NewBuildingHandler(postgres.NewBuildingRepo(pool))
	dh := handler.NewDeviceHandler(deviceRepo, pool)
	pph := handler.NewPropertyHandler(postgres.NewPropertyRepo(pool))
	rh := handler.NewRegisterHandler(postgres.NewRegisterRepo(pool))
	alh := handler.NewAlarmHandler(alarmRuleRepo, alarmLogRepo) // 使用 Repository
	sh := handler.NewStartupHandler(startupRepo, pool)         // 使用 Repository
	th := handler.NewTelemetryHandler(telemetryRepo)           // 使用 Repository
	logH := handler.NewLogHandler(logRepo)                      // 使用 Repository
	dashH := handler.NewDashboardHandler(pool, weatherSvc)
	admh := handler.NewAdminHandler(postgres.NewAdminRepo(pool), authSvc)
	wh := handler.NewWeatherHandler(weatherSvc)
	ih := handler.NewIntelligenceHandler(pool)
	mh := handler.NewMaintenanceHandler(maintenanceRepo) // 使用 Repository

	// ---- gateway & telemetry ----
	ctxGW, cancelGW := context.WithCancel(context.Background())
	defer cancelGW()

	// Background context for retention/offline/scheduler goroutines.
	// Cancelled first during graceful shutdown before gateways are torn down.
	ctxBg, cancelBg := context.WithCancel(context.Background())
	defer cancelBg()

	poller := telemetry.NewPoller(pool)
	gwMgr := gateway.NewManager(poller.PollDevice, loadDevicesForGateway(deviceRepo), pool)
	dh.SetGwMgr(gwMgr)    // enable device control via gateway
	sh.SetDeviceHandler(dh) // enable plan/scheduled-task real hardware dispatch
	sh.SetBgCtx(ctxBg)     // bind startup execution goroutines to shutdown signal
	if cfg.PollIntervalSec > 0 {
		gwMgr.SetPollInterval(time.Duration(cfg.PollIntervalSec) * time.Second)
		slog.Info("poll interval set", "seconds", cfg.PollIntervalSec)
	}
	gwMgr.StartCustomListener(ctxGW, ":8081")
	gwMgr.StartDTUListener(ctxGW, ":502")

	// ---- background tasks with panic recovery (safego.Go) ----

	if cfg.RetentionDays > 0 {
		safego.Go("retention", ctxBg, func(ctx context.Context) {
			runRetention(ctx, pool, cfg.RetentionDays, cfg.RetentionBatchSize) // immediate first run
			ticker := time.NewTicker(24 * time.Hour)
			defer ticker.Stop()
			for {
				select {
				case <-ctx.Done():
					return
				case <-ticker.C:
					runRetention(ctx, pool, cfg.RetentionDays, cfg.RetentionBatchSize)
				}
			}
		})
		slog.Info("data retention enabled", "days", cfg.RetentionDays)
	}

	// ---- offline detection: mark devices offline, create alarm log ----
	safego.Go("offline", ctxBg, func(ctx context.Context) {
		alarmEngine := alarm.New(pool)
		runOfflineCheck(ctx, pool, alarmEngine, cfg.OfflineThresholdMinutes) // immediate first run
		ticker := time.NewTicker(1 * time.Minute)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				runOfflineCheck(ctx, pool, alarmEngine, cfg.OfflineThresholdMinutes)
			}
		}
	})

	// ---- scheduled task runner ----
	safego.Go("scheduler", ctxBg, func(ctx context.Context) {
		ticker := time.NewTicker(1 * time.Minute)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				sh.RunDueScheduledTasks(ctx)
			}
		}
	})

	// ---- rate limiter: login attempts per minute per IP ----
	trustedCIDRs := cfg.TrustedProxyCIDRs()
	rateLimiter := middleware.NewRateLimiter(cfg.LoginRateLimit, 1*time.Minute, trustedCIDRs...)

	// ---- routes ----
	h := &Handlers{
		Auth: ah, Project: ph, Building: bh, Device: dh,
		Property: pph, Register: rh, Alarm: alh, Startup: sh,
		Telemetry: th, Log: logH, Dashboard: dashH, Admin: admh,
		Weather: wh, Intelligence: ih, Maintenance: mh,
		GatewayMgr: gwMgr,
	}
	mux := registerRoutes(db, rateLimiter, authSvc, h)

	addr := ":" + cfg.ServerPort
	srv := &http.Server{Addr: addr, Handler: middleware.CORS(cfg.AllowedOrigins, mux), ReadTimeout: 30 * time.Second, WriteTimeout: 60 * time.Second}
	go func() {
		quit := make(chan os.Signal, 1)
		signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
		<-quit
		slog.Info("shutting down...")
		// 1. Cancel background goroutines first (retention, offline, scheduler)
		cancelBg()
		// 2. Stop gateway listeners and disconnect all gateways
		cancelGW()
		gwMgr.Shutdown()
		// 3. Stop auth service background cache cleanup goroutine
		authSvc.Shutdown()
		// 4. Then gracefully shut down HTTP server
		shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer shutdownCancel()
		srv.Shutdown(shutdownCtx)
		rateLimiter.Shutdown()
	}()

	slog.Info("XMECO running", "http", addr, "customGW", ":8081", "dtuGW", ":502")
	if err := srv.ListenAndServe(); err != http.ErrServerClosed {
		return fmt.Errorf("server: %w", err)
	}
	slog.Info("server stopped")
	return nil
}

// Handlers groups all HTTP handlers to keep registerRoutes signature manageable.
type Handlers struct {
	Auth         *handler.AuthHandler
	Project      *handler.ProjectHandler
	Building     *handler.BuildingHandler
	Device       *handler.DeviceHandler
	Property     *handler.PropertyHandler
	Register     *handler.RegisterHandler
	Alarm        *handler.AlarmHandler
	Startup      *handler.StartupHandler
	Telemetry    *handler.TelemetryHandler
	Log          *handler.LogHandler
	Dashboard    *handler.DashboardHandler
	Admin        *handler.AdminHandler
	Weather      *handler.WeatherHandler
	Intelligence *handler.IntelligenceHandler
	Maintenance  *handler.MaintenanceHandler
	GatewayMgr   *gateway.Manager
}

// registerRoutes wires up all HTTP routes onto a new ServeMux and returns it.
func registerRoutes(
	db *postgres.DB,
	rateLimiter *middleware.RateLimiter,
	authSvc *auth.Service,
	h *Handlers,
) *http.ServeMux {
	p := auth.Perm // shorthand for permission constants
	wp := func(code string, next http.HandlerFunc) http.HandlerFunc {
		return withPerm(authSvc, code, next)
	}

	mux := http.NewServeMux()

	// public
	mux.HandleFunc("/api/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if err := db.Health(r.Context()); err != nil {
			slog.Error("db health check failed", "error", err)
			w.WriteHeader(http.StatusServiceUnavailable)
			fmt.Fprintf(w, `{"status":"unhealthy","error":"database connection failed"}`)
			return
		}
		fmt.Fprintf(w, `{"status":"ok","service":"XMECO","time":"%s"}`, time.Now().Format(time.RFC3339))
	})
	mux.Handle("POST /api/v1/auth/login", middleware.BodyLimit(1<<20)(rateLimiter.LimitLogin(h.Auth.Login)))

	protected := http.NewServeMux()
	protected.HandleFunc("GET /api/v1/auth/me", h.Auth.Me)
	protected.HandleFunc("GET /api/v1/projects", wp(p.ProjectView, h.Project.List))
	protected.HandleFunc("POST /api/v1/projects", wp(p.ProjectCreate, h.Project.Create))
	protected.HandleFunc("GET /api/v1/projects/{id}", wp(p.ProjectView, h.Project.Get))
	protected.HandleFunc("PUT /api/v1/projects/{id}", wp(p.ProjectEdit, h.Project.Update))
	protected.HandleFunc("DELETE /api/v1/projects/{id}", wp(p.ProjectDelete, h.Project.Delete))
	protected.HandleFunc("GET /api/v1/projects/{id}/users", wp(p.ProjectEdit, h.Project.GetProjectUsers))
	protected.HandleFunc("PUT /api/v1/projects/{id}/users", wp(p.ProjectEdit, h.Project.SetProjectUsers))
	protected.HandleFunc("GET /api/v1/buildings", wp(p.BuildingView, h.Building.List))
	protected.HandleFunc("GET /api/v1/buildings/{id}", wp(p.BuildingView, h.Building.Get))
	protected.HandleFunc("POST /api/v1/buildings", wp(p.BuildingCreate, h.Building.Create))
	protected.HandleFunc("PUT /api/v1/buildings/{id}", wp(p.BuildingEdit, h.Building.Update))
	protected.HandleFunc("DELETE /api/v1/buildings/{id}", wp(p.BuildingDelete, h.Building.Delete))
	protected.HandleFunc("GET /api/v1/devices", wp(p.DeviceView, h.Device.List))
	protected.HandleFunc("GET /api/v1/devices/{id}", wp(p.DeviceView, h.Device.Get))
	protected.HandleFunc("POST /api/v1/devices", wp(p.DeviceCreate, h.Device.Create))
	protected.HandleFunc("PUT /api/v1/devices/{id}", wp(p.DeviceEdit, h.Device.Update))
	protected.HandleFunc("DELETE /api/v1/devices/{id}", wp(p.DeviceDelete, h.Device.Delete))
	protected.HandleFunc("POST /api/v1/devices/{id}/control", wp(p.DeviceControl, h.Device.Control))
	protected.HandleFunc("GET /api/v1/devices/{id}/sensor-data", wp(p.DeviceView, h.Device.SensorData))
	protected.HandleFunc("PUT /api/v1/devices/{id}/sensor-config", wp(p.DeviceEdit, h.Device.SaveSensorConfig))
	protected.HandleFunc("GET /api/v1/properties", wp(p.DeviceView, h.Property.List))
	protected.HandleFunc("GET /api/v1/properties/{id}", wp(p.DeviceView, h.Property.Get))
	protected.HandleFunc("POST /api/v1/properties", wp(p.DeviceProperty, h.Property.Create))
	protected.HandleFunc("PUT /api/v1/properties/{id}", wp(p.DeviceProperty, h.Property.Update))
	protected.HandleFunc("DELETE /api/v1/properties/{id}", wp(p.DeviceProperty, h.Property.Delete))
	protected.HandleFunc("GET /api/v1/registers", wp(p.DeviceView, h.Register.List))
	protected.HandleFunc("GET /api/v1/registers/{id}", wp(p.DeviceView, h.Register.Get))
	protected.HandleFunc("POST /api/v1/registers", wp(p.DeviceRegister, h.Register.Create))
	protected.HandleFunc("PUT /api/v1/registers/{id}", wp(p.DeviceRegister, h.Register.Update))
	protected.HandleFunc("DELETE /api/v1/registers/{id}", wp(p.DeviceRegister, h.Register.Delete))
	protected.HandleFunc("GET /api/v1/alarm-rules", wp(p.MonitorAlarmCfg, h.Alarm.ListRules))
	protected.HandleFunc("POST /api/v1/alarm-rules", wp(p.MonitorAlarmCfg, h.Alarm.CreateRule))
	protected.HandleFunc("PUT /api/v1/alarm-rules/{id}", wp(p.MonitorAlarmCfg, h.Alarm.UpdateRule))
	protected.HandleFunc("DELETE /api/v1/alarm-rules/{id}", wp(p.MonitorAlarmCfg, h.Alarm.DeleteRule))
	protected.HandleFunc("GET /api/v1/alarm-logs", wp(p.MonitorRealtime, h.Alarm.ListLogs))
	protected.HandleFunc("POST /api/v1/alarm-logs/{id}/ack", wp(p.MonitorAlarmCfg, h.Alarm.AckLog))
	protected.HandleFunc("GET /api/v1/maintenance-records", wp(p.DeviceView, h.Maintenance.List))
	protected.HandleFunc("POST /api/v1/maintenance-records", wp(p.DeviceEdit, h.Maintenance.Create))
	protected.HandleFunc("PUT /api/v1/maintenance-records/{id}", wp(p.DeviceEdit, h.Maintenance.Update))
	protected.HandleFunc("DELETE /api/v1/maintenance-records/{id}", wp(p.DeviceEdit, h.Maintenance.Delete))
	protected.HandleFunc("GET /api/v1/startup-plans", wp(p.ScheduleView, h.Startup.ListPlans))
	protected.HandleFunc("POST /api/v1/startup-plans", wp(p.ScheduleCreate, h.Startup.CreatePlan))
	protected.HandleFunc("PUT /api/v1/startup-plans/{id}", wp(p.ScheduleEdit, h.Startup.UpdatePlan))
	protected.HandleFunc("DELETE /api/v1/startup-plans/{id}", wp(p.ScheduleDelete, h.Startup.DeletePlan))
	protected.HandleFunc("POST /api/v1/startup-plans/{id}/execute", wp(p.ScheduleCreate, h.Startup.Execute))
	protected.HandleFunc("GET /api/v1/startup-executions/{id}", wp(p.ScheduleView, h.Startup.GetExecution))
	protected.HandleFunc("POST /api/v1/startup-executions/{id}/stop", wp(p.ScheduleEdit, h.Startup.StopExecution))
	protected.HandleFunc("GET /api/v1/scheduled-tasks", wp(p.ScheduleView, h.Startup.ListScheduledTasks))
	protected.HandleFunc("POST /api/v1/scheduled-tasks", wp(p.ScheduleCreate, h.Startup.CreateScheduledTask))
	protected.HandleFunc("PUT /api/v1/scheduled-tasks/{id}", wp(p.ScheduleEdit, h.Startup.UpdateScheduledTask))
	protected.HandleFunc("DELETE /api/v1/scheduled-tasks/{id}", wp(p.ScheduleDelete, h.Startup.DeleteScheduledTask))
	protected.HandleFunc("GET /api/v1/telemetry/realtime", wp(p.MonitorRealtime, h.Telemetry.Realtime))
	protected.HandleFunc("GET /api/v1/telemetry/history", wp(p.MonitorGraph, h.Telemetry.History))
	protected.HandleFunc("GET /api/v1/telemetry/stats", wp(p.MonitorGraph, h.Telemetry.Stats))
	protected.HandleFunc("GET /api/v1/gateways", wp(p.SystemGateway, h.GatewayMgr.HandleListGateways))
	protected.HandleFunc("GET /api/v1/export/telemetry", wp(p.ReportExport, h.Log.ExportTelemetry))
	protected.HandleFunc("GET /api/v1/export/controls", wp(p.ReportExcel, h.Log.ExportControls))
	protected.HandleFunc("GET /api/v1/system/info", wp(p.SystemConfig, h.Admin.SystemInfo))
	protected.HandleFunc("GET /api/v1/system/db-stats", wp(p.SystemDB, h.Admin.DBStats))
	protected.HandleFunc("GET /api/v1/logs/telemetry", wp(p.MonitorRealtime, h.Log.Telemetry))
	protected.HandleFunc("GET /api/v1/logs/controls", wp(p.MonitorCtrlLog, h.Log.Controls))
	protected.HandleFunc("GET /api/v1/logs/stats", wp(p.MonitorRealtime, h.Log.Stats))
	protected.HandleFunc("GET /api/v1/dashboard", wp(p.MonitorRealtime, h.Dashboard.GetConfig))
	protected.HandleFunc("PUT /api/v1/dashboard", wp(p.MonitorRealtime, h.Dashboard.UpdateConfig))
	protected.HandleFunc("GET /api/v1/screen/data", wp(p.MonitorRealtime, h.Dashboard.ScreenData))
	protected.HandleFunc("GET /api/v1/users", wp(p.UserView, h.Admin.ListUsers))
	protected.HandleFunc("POST /api/v1/users", wp(p.UserCreate, h.Admin.CreateUser))
	protected.HandleFunc("PUT /api/v1/users/{id}", wp(p.UserEdit, h.Admin.UpdateUser))
	protected.HandleFunc("POST /api/v1/users/{id}/reset-password", wp(p.UserEdit, h.Admin.ResetPassword))
	protected.HandleFunc("DELETE /api/v1/users/{id}", wp(p.UserDelete, h.Admin.DeleteUser))
	protected.HandleFunc("GET /api/v1/agents", wp(p.AgentView, h.Admin.ListAgents))
	protected.HandleFunc("POST /api/v1/agents", wp(p.AgentCreate, h.Admin.CreateAgent))
	protected.HandleFunc("PUT /api/v1/agents/{id}", wp(p.AgentEdit, h.Admin.UpdateAgent))
	protected.HandleFunc("DELETE /api/v1/agents/{id}", wp(p.AgentDelete, h.Admin.DeleteAgent))
	protected.HandleFunc("GET /api/v1/roles", wp(p.UserView, h.Admin.ListRoles))
	protected.HandleFunc("GET /api/v1/permissions", wp(p.UserView, h.Admin.ListPermissions))
	protected.HandleFunc("GET /api/v1/roles/{id}/permissions", wp(p.UserAssignRole, h.Admin.GetRolePermissions))
	protected.HandleFunc("PUT /api/v1/roles/{id}/permissions", wp(p.UserAssignRole, h.Admin.SetRolePermissions))
	protected.HandleFunc("GET /api/v1/weather/cities", wp(p.ProjectView, h.Weather.ListCities))
	protected.HandleFunc("GET /api/v1/weather/provinces", wp(p.ProjectView, h.Weather.ListProvinceCities))
	protected.HandleFunc("GET /api/v1/weather/cities/{id}", wp(p.ProjectView, h.Weather.GetCity))
	protected.HandleFunc("GET /api/v1/weather/now", wp(p.MonitorRealtime, h.Weather.Now))
	protected.HandleFunc("GET /api/v1/weather/project", wp(p.MonitorRealtime, h.Weather.ProjectWeather))
	protected.HandleFunc("GET /api/v1/intelligence/full", wp(p.MonitorGraph, h.Intelligence.FullAnalysis))
	protected.HandleFunc("GET /api/v1/intelligence/efficiency", wp(p.MonitorGraph, h.Intelligence.Efficiency))
	protected.HandleFunc("GET /api/v1/intelligence/forecast", wp(p.MonitorGraph, h.Intelligence.Forecast))
	protected.HandleFunc("GET /api/v1/intelligence/recommendations", wp(p.MonitorGraph, h.Intelligence.Recommendations))
	protected.HandleFunc("GET /api/v1/intelligence/strategies", wp(p.MonitorGraph, h.Intelligence.Strategies))
	protected.HandleFunc("GET /api/v1/intelligence/price-config", wp(p.MonitorGraph, h.Intelligence.PriceConfig))
	protected.HandleFunc("PUT /api/v1/intelligence/price-config", wp(p.MonitorGraph, h.Intelligence.SavePriceConfig))
	protected.HandleFunc("GET /api/v1/intelligence/power-quality", wp(p.MonitorGraph, h.Intelligence.PowerQuality))
	protected.HandleFunc("GET /api/v1/intelligence/meter-devices", wp(p.MonitorGraph, h.Intelligence.MeterDevices))

	mux.Handle("/api/v1/", middleware.BodyLimit(1<<20)(middleware.AuthMiddleware(authSvc)(protected)))
	return mux
}

// withPerm wraps a handler with RBAC permission check.
func withPerm(authSvc *auth.Service, permCode string, next http.HandlerFunc) http.HandlerFunc {
	handler := middleware.RequirePermission(authSvc, permCode)(next)
	return func(w http.ResponseWriter, r *http.Request) { handler.ServeHTTP(w, r) }
}

// runRetention deletes telemetry data older than retentionDays in batches.
func runRetention(ctx context.Context, pool postgres.DBTX, retentionDays, batchSize int) {
	if batchSize <= 0 {
		return
	}
	cutoff := time.Now().AddDate(0, 0, -retentionDays)
	for {
		ct, err := pool.Exec(ctx,
			`DELETE FROM device_telemetry WHERE ts < $1 AND ctid IN (SELECT ctid FROM device_telemetry WHERE ts < $1 LIMIT $2)`,
			cutoff, batchSize)
		if err != nil {
			slog.Warn("data retention cleanup failed", "err", err)
			return
		}
		if ct.RowsAffected() == 0 {
			return
		}
		slog.Info("data retention: deleted rows", "count", ct.RowsAffected())
	}
}

// runOfflineCheck marks devices offline and creates alarm logs after the configured
// threshold minutes of silence. The SELECT and UPDATE run in a single transaction
// to prevent a TOCTOU race where a device could come back online between read and write.
func runOfflineCheck(ctx context.Context, pool postgres.DBTX, alarmEngine *alarm.Engine, offlineThresholdMinutes int) {
	threshold := fmt.Sprintf("%d minutes", offlineThresholdMinutes)
	tx, err := pool.Begin(ctx)
	if err != nil {
		slog.Warn("offline detection begin tx failed", "err", err)
		return
	}
	defer tx.Rollback(ctx) // no-op after Commit

	rows, err := tx.Query(ctx,
		`SELECT id, COALESCE(name,'') FROM device
		 WHERE online_status='在线' AND last_online_at < NOW() - $1::interval`, threshold)
	if err != nil {
		slog.Warn("offline detection query failed", "err", err)
		return
	}
	defer rows.Close()
	for rows.Next() {
		var id int
		var name string
		if err := rows.Scan(&id, &name); err != nil {
			continue
		}
		// 告警写入使用事务连接 tx，确保与后续 UPDATE 原子一致
		if err := alarmEngine.AlertOfflineTx(ctx, tx, id, name, offlineThresholdMinutes); err != nil {
			slog.Warn("offline alarm create failed", "dev", id, "err", err)
		}
	}
	if rows.Err() != nil {
		slog.Warn("offline detection rows iteration failed", "err", rows.Err())
		return
	}

	if _, err := tx.Exec(ctx,
		`UPDATE device SET online_status='离线'
		 WHERE online_status='在线' AND last_online_at < NOW() - $1::interval`, threshold); err != nil {
		slog.Warn("offline status update failed", "err", err)
		return
	}

	if err := tx.Commit(ctx); err != nil {
		slog.Warn("offline detection commit failed", "err", err)
	}
}

// loadDevicesForGateway returns a DeviceLoaderFn that queries the DB via DeviceRepo
// for devices whose gateway_imei matches the gateway ID.
func loadDevicesForGateway(repo *postgres.DeviceRepo) gateway.DeviceLoaderFn {
	return func(ctx context.Context, gwID string) ([]gateway.DeviceRef, error) {
		devs, err := repo.ListByGatewayIMEI(ctx, gwID)
		if err != nil {
			return nil, err
		}
		result := make([]gateway.DeviceRef, len(devs))
		for i, d := range devs {
			result[i] = gateway.DeviceRef{
				DeviceID:   d.DeviceID,
				DeviceNo:   byte(d.DeviceNo),
				DeviceType: d.DeviceType,
				DeviceName: d.DeviceName,
				NodeAddr:   uint16(d.NodeAddr),
			}
		}
		return result, nil
	}
}
