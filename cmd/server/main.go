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
	"xmeco/internal/service/alarm"
	"xmeco/internal/service/auth"
	"xmeco/internal/service/external/weather"
	"xmeco/internal/service/migration"
	"xmeco/internal/service/telemetry"
)

func main() {
	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo})))
	cfg, err := config.Load()
	if err != nil {
		slog.Error("config load failed", "error", err)
		os.Exit(1)
	}
	slog.Info("XMECO starting")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	db, err := postgres.New(ctx, cfg.DSN())
	if err != nil {
		slog.Error("db connect failed", "error", err, "dsn", cfg.MaskedDSN())
		os.Exit(1)
	}
	defer db.Close()
	slog.Info("db connected", "dsn", cfg.MaskedDSN())

	pool := db.Pool

	// ---- migrations ----
	if err := migration.Run(context.Background(), pool); err != nil {
		slog.Error("migration failed", "error", err)
		os.Exit(1)
	}

	// ---- services ----
	authSvc := auth.New(pool, cfg.JWTSecret)

	// ---- handlers ----
	weatherSvc := weather.New(pool)
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
	dashH := handler.NewDashboardHandler(pool, weatherSvc)
	admh := handler.NewAdminHandler(postgres.NewAdminRepo(pool), authSvc)
	wh := handler.NewWeatherHandler(weatherSvc)
	ih := handler.NewIntelligenceHandler(pool)
	mh := handler.NewMaintenanceHandler(pool)

	// ---- gateway & telemetry ----
	ctxGW, cancelGW := context.WithCancel(context.Background())
	defer cancelGW()

	// Background context for retention/offline/scheduler goroutines.
	// Cancelled first during graceful shutdown before gateways are torn down.
	ctxBg, cancelBg := context.WithCancel(context.Background())
	defer cancelBg()

	poller := telemetry.NewPoller(pool)
	gwMgr := gateway.NewManager(poller.PollDevice, loadDevicesForGateway(pool), pool)
	dh.SetGwMgr(gwMgr)    // enable device control via gateway
	sh.SetDeviceHandler(dh) // enable plan/scheduled-task real hardware dispatch
	if cfg.PollIntervalSec > 0 {
		gwMgr.SetPollInterval(time.Duration(cfg.PollIntervalSec) * time.Second)
		slog.Info("poll interval set", "seconds", cfg.PollIntervalSec)
	}
	gwMgr.StartCustomListener(ctxGW, ":8081")
	gwMgr.StartDTUListener(ctxGW, ":502")

	// ---- data retention + offline detection + scheduled tasks (panic recovery + immediate first run) ----
	safeGo := func(name string, fn func(ctx context.Context)) {
		go func() {
			defer func() {
				if r := recover(); r != nil {
					slog.Error("goroutine panic recovered", "name", name, "panic", r)
				}
			}()
			fn(ctxBg)
		}()
	}

	if cfg.RetentionDays > 0 {
		safeGo("retention", func(ctx context.Context) {
			runRetention(ctx, pool, cfg.RetentionDays) // immediate first run
			ticker := time.NewTicker(24 * time.Hour)
			defer ticker.Stop()
			for {
				select {
				case <-ctx.Done():
					return
				case <-ticker.C:
					runRetention(ctx, pool, cfg.RetentionDays)
				}
			}
		})
		slog.Info("data retention enabled", "days", cfg.RetentionDays)
	}

	// ---- offline detection: mark devices offline after 10 min, create alarm log ----
	safeGo("offline", func(ctx context.Context) {
		alarmEngine := alarm.New(pool)
		runOfflineCheck(ctx, pool, alarmEngine) // immediate first run
		ticker := time.NewTicker(1 * time.Minute)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				runOfflineCheck(ctx, pool, alarmEngine)
			}
		}
	})

	// ---- scheduled task runner ----
	safeGo("scheduler", func(ctx context.Context) {
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

	// ---- rate limiter: 10 login attempts per minute per IP ----
	trustedCIDRs := cfg.TrustedProxyCIDRs()
	rateLimiter := middleware.NewRateLimiter(10, 1*time.Minute, trustedCIDRs...)

	// ---- routes ----
	mux := registerRoutes(db, rateLimiter, authSvc,
		ah, ph, bh, dh, pph, rh, alh, sh, th, logH, dashH, admh, wh, ih, mh,
		gwMgr,
	)

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
		// 3. Then gracefully shut down HTTP server
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		srv.Shutdown(ctx)
		rateLimiter.Shutdown()
	}()

	slog.Info("XMECO running", "http", addr, "customGW", ":8081", "dtuGW", ":502")
	if err := srv.ListenAndServe(); err != http.ErrServerClosed {
		slog.Error("server error", "error", err)
		os.Exit(1)
	}
	slog.Info("server stopped")
}

// registerRoutes wires up all HTTP routes onto a new ServeMux and returns it.
func registerRoutes(
	db *postgres.DB,
	rateLimiter *middleware.RateLimiter,
	authSvc *auth.Service,
	ah *handler.AuthHandler,
	ph *handler.ProjectHandler,
	bh *handler.BuildingHandler,
	dh *handler.DeviceHandler,
	pph *handler.PropertyHandler,
	rh *handler.RegisterHandler,
	alh *handler.AlarmHandler,
	sh *handler.StartupHandler,
	th *handler.TelemetryHandler,
	logH *handler.LogHandler,
	dashH *handler.DashboardHandler,
	admh *handler.AdminHandler,
	wh *handler.WeatherHandler,
	ih *handler.IntelligenceHandler,
	mh *handler.MaintenanceHandler,
	gwMgr *gateway.Manager,
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
	mux.Handle("POST /api/v1/auth/login", middleware.BodyLimit(1<<20)(rateLimiter.LimitLogin(ah.Login)))

	protected := http.NewServeMux()
	protected.HandleFunc("GET /api/v1/auth/me", ah.Me)
	protected.HandleFunc("GET /api/v1/projects", wp(p.ProjectView, ph.List))
	protected.HandleFunc("POST /api/v1/projects", wp(p.ProjectCreate, ph.Create))
	protected.HandleFunc("GET /api/v1/projects/{id}", wp(p.ProjectView, ph.Get))
	protected.HandleFunc("PUT /api/v1/projects/{id}", wp(p.ProjectEdit, ph.Update))
	protected.HandleFunc("DELETE /api/v1/projects/{id}", wp(p.ProjectDelete, ph.Delete))
	protected.HandleFunc("GET /api/v1/projects/{id}/users", wp(p.ProjectEdit, ph.GetProjectUsers))
	protected.HandleFunc("PUT /api/v1/projects/{id}/users", wp(p.ProjectEdit, ph.SetProjectUsers))
	protected.HandleFunc("GET /api/v1/buildings", wp(p.BuildingView, bh.List))
	protected.HandleFunc("GET /api/v1/buildings/{id}", wp(p.BuildingView, bh.Get))
	protected.HandleFunc("POST /api/v1/buildings", wp(p.BuildingCreate, bh.Create))
	protected.HandleFunc("PUT /api/v1/buildings/{id}", wp(p.BuildingEdit, bh.Update))
	protected.HandleFunc("DELETE /api/v1/buildings/{id}", wp(p.BuildingDelete, bh.Delete))
	protected.HandleFunc("GET /api/v1/devices", wp(p.DeviceView, dh.List))
	protected.HandleFunc("GET /api/v1/devices/{id}", wp(p.DeviceView, dh.Get))
	protected.HandleFunc("POST /api/v1/devices", wp(p.DeviceCreate, dh.Create))
	protected.HandleFunc("PUT /api/v1/devices/{id}", wp(p.DeviceEdit, dh.Update))
	protected.HandleFunc("DELETE /api/v1/devices/{id}", wp(p.DeviceDelete, dh.Delete))
	protected.HandleFunc("POST /api/v1/devices/{id}/control", wp(p.DeviceControl, dh.Control))
	protected.HandleFunc("GET /api/v1/devices/{id}/sensor-data", wp(p.DeviceView, dh.SensorData))
	protected.HandleFunc("PUT /api/v1/devices/{id}/sensor-config", wp(p.DeviceEdit, dh.SaveSensorConfig))
	protected.HandleFunc("GET /api/v1/properties", wp(p.DeviceView, pph.List))
	protected.HandleFunc("GET /api/v1/properties/{id}", wp(p.DeviceView, pph.Get))
	protected.HandleFunc("POST /api/v1/properties", wp(p.DeviceProperty, pph.Create))
	protected.HandleFunc("PUT /api/v1/properties/{id}", wp(p.DeviceProperty, pph.Update))
	protected.HandleFunc("DELETE /api/v1/properties/{id}", wp(p.DeviceProperty, pph.Delete))
	protected.HandleFunc("GET /api/v1/registers", wp(p.DeviceView, rh.List))
	protected.HandleFunc("GET /api/v1/registers/{id}", wp(p.DeviceView, rh.Get))
	protected.HandleFunc("POST /api/v1/registers", wp(p.DeviceRegister, rh.Create))
	protected.HandleFunc("PUT /api/v1/registers/{id}", wp(p.DeviceRegister, rh.Update))
	protected.HandleFunc("DELETE /api/v1/registers/{id}", wp(p.DeviceRegister, rh.Delete))
	protected.HandleFunc("GET /api/v1/alarm-rules", wp(p.MonitorAlarmCfg, alh.ListRules))
	protected.HandleFunc("POST /api/v1/alarm-rules", wp(p.MonitorAlarmCfg, alh.CreateRule))
	protected.HandleFunc("PUT /api/v1/alarm-rules/{id}", wp(p.MonitorAlarmCfg, alh.UpdateRule))
	protected.HandleFunc("DELETE /api/v1/alarm-rules/{id}", wp(p.MonitorAlarmCfg, alh.DeleteRule))
	protected.HandleFunc("GET /api/v1/alarm-logs", wp(p.MonitorRealtime, alh.ListLogs))
	protected.HandleFunc("POST /api/v1/alarm-logs/{id}/ack", wp(p.MonitorAlarmCfg, alh.AckLog))
	protected.HandleFunc("GET /api/v1/maintenance-records", wp(p.DeviceView, mh.List))
	protected.HandleFunc("POST /api/v1/maintenance-records", wp(p.DeviceEdit, mh.Create))
	protected.HandleFunc("PUT /api/v1/maintenance-records/{id}", wp(p.DeviceEdit, mh.Update))
	protected.HandleFunc("DELETE /api/v1/maintenance-records/{id}", wp(p.DeviceEdit, mh.Delete))
	protected.HandleFunc("GET /api/v1/startup-plans", wp(p.ScheduleView, sh.ListPlans))
	protected.HandleFunc("POST /api/v1/startup-plans", wp(p.ScheduleCreate, sh.CreatePlan))
	protected.HandleFunc("PUT /api/v1/startup-plans/{id}", wp(p.ScheduleEdit, sh.UpdatePlan))
	protected.HandleFunc("DELETE /api/v1/startup-plans/{id}", wp(p.ScheduleDelete, sh.DeletePlan))
	protected.HandleFunc("POST /api/v1/startup-plans/{id}/execute", wp(p.ScheduleCreate, sh.Execute))
	protected.HandleFunc("GET /api/v1/startup-executions/{id}", wp(p.ScheduleView, sh.GetExecution))
	protected.HandleFunc("POST /api/v1/startup-executions/{id}/stop", wp(p.ScheduleEdit, sh.StopExecution))
	protected.HandleFunc("GET /api/v1/scheduled-tasks", wp(p.ScheduleView, sh.ListScheduledTasks))
	protected.HandleFunc("POST /api/v1/scheduled-tasks", wp(p.ScheduleCreate, sh.CreateScheduledTask))
	protected.HandleFunc("PUT /api/v1/scheduled-tasks/{id}", wp(p.ScheduleEdit, sh.UpdateScheduledTask))
	protected.HandleFunc("DELETE /api/v1/scheduled-tasks/{id}", wp(p.ScheduleDelete, sh.DeleteScheduledTask))
	protected.HandleFunc("GET /api/v1/telemetry/realtime", wp(p.MonitorRealtime, th.Realtime))
	protected.HandleFunc("GET /api/v1/telemetry/history", wp(p.MonitorGraph, th.History))
	protected.HandleFunc("GET /api/v1/telemetry/stats", wp(p.MonitorGraph, th.Stats))
	protected.HandleFunc("GET /api/v1/gateways", wp(p.SystemGateway, gwMgr.HandleListGateways))
	protected.HandleFunc("GET /api/v1/export/telemetry", wp(p.ReportExport, logH.ExportTelemetry))
	protected.HandleFunc("GET /api/v1/export/controls", wp(p.ReportExcel, logH.ExportControls))
	protected.HandleFunc("GET /api/v1/system/info", wp(p.SystemConfig, admh.SystemInfo))
	protected.HandleFunc("GET /api/v1/system/db-stats", wp(p.SystemDB, admh.DBStats))
	protected.HandleFunc("GET /api/v1/logs/telemetry", wp(p.MonitorRealtime, logH.Telemetry))
	protected.HandleFunc("GET /api/v1/logs/controls", wp(p.MonitorCtrlLog, logH.Controls))
	protected.HandleFunc("GET /api/v1/logs/stats", wp(p.MonitorRealtime, logH.Stats))
	protected.HandleFunc("GET /api/v1/dashboard", wp(p.MonitorRealtime, dashH.GetConfig))
	protected.HandleFunc("PUT /api/v1/dashboard", wp(p.MonitorRealtime, dashH.UpdateConfig))
	protected.HandleFunc("GET /api/v1/screen/data", wp(p.MonitorRealtime, dashH.ScreenData))
	protected.HandleFunc("GET /api/v1/users", wp(p.UserView, admh.ListUsers))
	protected.HandleFunc("POST /api/v1/users", wp(p.UserCreate, admh.CreateUser))
	protected.HandleFunc("PUT /api/v1/users/{id}", wp(p.UserEdit, admh.UpdateUser))
	protected.HandleFunc("POST /api/v1/users/{id}/reset-password", wp(p.UserEdit, admh.ResetPassword))
	protected.HandleFunc("DELETE /api/v1/users/{id}", wp(p.UserDelete, admh.DeleteUser))
	protected.HandleFunc("GET /api/v1/agents", wp(p.AgentView, admh.ListAgents))
	protected.HandleFunc("POST /api/v1/agents", wp(p.AgentCreate, admh.CreateAgent))
	protected.HandleFunc("PUT /api/v1/agents/{id}", wp(p.AgentEdit, admh.UpdateAgent))
	protected.HandleFunc("DELETE /api/v1/agents/{id}", wp(p.AgentDelete, admh.DeleteAgent))
	protected.HandleFunc("GET /api/v1/roles", wp(p.UserView, admh.ListRoles))
	protected.HandleFunc("GET /api/v1/permissions", wp(p.UserView, admh.ListPermissions))
	protected.HandleFunc("GET /api/v1/roles/{id}/permissions", wp(p.UserAssignRole, admh.GetRolePermissions))
	protected.HandleFunc("PUT /api/v1/roles/{id}/permissions", wp(p.UserAssignRole, admh.SetRolePermissions))
	protected.HandleFunc("GET /api/v1/weather/cities", wp(p.ProjectView, wh.ListCities))
	protected.HandleFunc("GET /api/v1/weather/provinces", wp(p.ProjectView, wh.ListProvinceCities))
	protected.HandleFunc("GET /api/v1/weather/cities/{id}", wp(p.ProjectView, wh.GetCity))
	protected.HandleFunc("GET /api/v1/weather/now", wp(p.MonitorRealtime, wh.Now))
	protected.HandleFunc("GET /api/v1/weather/project", wp(p.MonitorRealtime, wh.ProjectWeather))
	protected.HandleFunc("GET /api/v1/intelligence/full", wp(p.MonitorGraph, ih.FullAnalysis))
	protected.HandleFunc("GET /api/v1/intelligence/efficiency", wp(p.MonitorGraph, ih.Efficiency))
	protected.HandleFunc("GET /api/v1/intelligence/forecast", wp(p.MonitorGraph, ih.Forecast))
	protected.HandleFunc("GET /api/v1/intelligence/recommendations", wp(p.MonitorGraph, ih.Recommendations))
	protected.HandleFunc("GET /api/v1/intelligence/strategies", wp(p.MonitorGraph, ih.Strategies))
	protected.HandleFunc("GET /api/v1/intelligence/price-config", wp(p.MonitorGraph, ih.PriceConfig))
	protected.HandleFunc("PUT /api/v1/intelligence/price-config", wp(p.MonitorGraph, ih.SavePriceConfig))
	protected.HandleFunc("GET /api/v1/intelligence/power-quality", wp(p.MonitorGraph, ih.PowerQuality))
	protected.HandleFunc("GET /api/v1/intelligence/meter-devices", wp(p.MonitorGraph, ih.MeterDevices))

	mux.Handle("/api/v1/", middleware.BodyLimit(1<<20)(middleware.AuthMiddleware(authSvc)(protected)))
	return mux
}

// 后台任务常量
const (
	retentionBatchSize = 10000             // 数据清理每批次最大行数
	offlineIntervalSQL = "'10 minutes'"    // PostgreSQL interval 格式，设备离线判定阈值
)

// withPerm wraps a handler with RBAC permission check.
func withPerm(authSvc *auth.Service, permCode string, next http.HandlerFunc) http.HandlerFunc {
	handler := middleware.RequirePermission(authSvc, permCode)(next)
	return func(w http.ResponseWriter, r *http.Request) { handler.ServeHTTP(w, r) }
}

// runRetention deletes telemetry data older than retentionDays in batches.
func runRetention(ctx context.Context, pool postgres.DBTX, retentionDays int) {
	cutoff := time.Now().AddDate(0, 0, -retentionDays)
	for {
		ct, err := pool.Exec(ctx,
			`DELETE FROM device_telemetry WHERE ts < $1 AND ctid IN (SELECT ctid FROM device_telemetry WHERE ts < $1 LIMIT $2)`,
			cutoff, retentionBatchSize)
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

// runOfflineCheck marks devices offline and creates alarm logs after 10 min of silence.
func runOfflineCheck(ctx context.Context, pool postgres.DBTX, alarmEngine *alarm.Engine) {
	rows, err := pool.Query(ctx,
		`SELECT id, COALESCE(name,'') FROM device
		 WHERE online_status='在线' AND last_online_at < NOW() - `+offlineIntervalSQL)
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
		if err := alarmEngine.AlertOffline(ctx, id, name); err != nil {
			slog.Warn("offline alarm create failed", "dev", id, "err", err)
		}
	}
	if _, err := pool.Exec(ctx,
		`UPDATE device SET online_status='离线'
		 WHERE online_status='在线' AND last_online_at < NOW() - `+offlineIntervalSQL); err != nil {
		slog.Warn("offline status update failed", "err", err)
	}
}

// loadDevicesForGateway returns a DeviceLoaderFn that queries the DB for
// devices whose gateway_imei matches the gateway ID.
func loadDevicesForGateway(pool postgres.DBTX) gateway.DeviceLoaderFn {
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
