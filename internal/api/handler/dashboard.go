package handler

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"sync"

	"xmeco/internal/api/middleware"
	"xmeco/internal/repository/postgres"
	"xmeco/internal/service/external/weather"
)

const (
	baseRunningDays = 1000 // 项目创建前的初始运营天数
	carbonFactor    = 0.58 // 中国电网碳排放系数 kg CO₂/kWh
)

type DashboardHandler struct {
	pool       postgres.DBTX
	weatherSvc *weather.Service
}

func NewDashboardHandler(pool postgres.DBTX, weatherSvc *weather.Service) *DashboardHandler {
	return &DashboardHandler{pool: pool, weatherSvc: weatherSvc}
}

func (h *DashboardHandler) GetConfig(w http.ResponseWriter, r *http.Request) {
	rows, err := h.pool.Query(r.Context(), "SELECT key,value FROM dashboard_config")
	if err != nil { serverErr(w, err); return }
	if rows == nil { serverErr(w, fmt.Errorf("dashboard_config query returned nil rows")); return }
	defer rows.Close()
	cfg := map[string]string{}
	for rows.Next() {
		var k, v string
		if err := rows.Scan(&k, &v); err != nil { slog.Warn("dashboard scan", "err", err); continue }
		cfg[k] = v
	}
	var online, alarms, baseDays int
	if err := h.pool.QueryRow(r.Context(), `SELECT count(*) FROM device WHERE online_status='在线'`).Scan(&online); err != nil {
		slog.Warn("dashboard online count failed", "err", err)
	}
	if err := h.pool.QueryRow(r.Context(), `SELECT count(*) FROM alarm_log WHERE created_at::date=CURRENT_DATE`).Scan(&alarms); err != nil {
		slog.Warn("dashboard alarms count failed", "err", err)
	}
	if err := h.pool.QueryRow(r.Context(), `SELECT COALESCE(EXTRACT(DAY FROM NOW()-date(value))::int,0) FROM dashboard_config WHERE key='days_start'`).Scan(&baseDays); err != nil {
		slog.Warn("dashboard base days failed", "err", err)
	}
	cfg["running_days"] = fmt.Sprint(baseRunningDays + baseDays)
	cfg["online_devices"] = fmt.Sprint(online)
	cfg["today_alarms"] = fmt.Sprint(alarms)
	ok(w, cfg)
}

func (h *DashboardHandler) UpdateConfig(w http.ResponseWriter, r *http.Request) {
	var body map[string]string
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeJSON(w, http.StatusBadRequest, errBadRequest); return
	}
	for k, v := range body {
		if _, err := h.pool.Exec(r.Context(), "INSERT INTO dashboard_config (key,value) VALUES($1,$2) ON CONFLICT(key) DO UPDATE SET value=$2,updated_at=NOW()", k, v); err != nil {
			serverErr(w, err); return
		}
	}
	ok(w, M{"status": "ok"})
}

// ScreenData returns all data needed for the big-screen dashboard.
func (h *DashboardHandler) ScreenData(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	pid := queryInt(r, "project_id")
	bid := queryInt(r, "building_id")
	out := make(map[string]any)

	// User + agent
	claims := middleware.GetClaims(r)
	if claims != nil {
		out["username"] = claims.Username
		var an string
		if err := h.pool.QueryRow(ctx, `SELECT COALESCE(a.name,'') FROM users u LEFT JOIN agent a ON a.id=u.agent_id WHERE u.id=$1`, claims.UserID).Scan(&an); err != nil {
			slog.Warn("ScreenData agent name query failed", "err", err)
		}
		out["agent_name"] = an
	}

	// Projects — honour project_user assignments for all roles
	var userID int
	if claims != nil { userID = claims.UserID }
	var projQ string
	var projArgs []any
	var noProjectMsg string
	var defaultPID int
	if userID > 0 {
		if err := h.pool.QueryRow(ctx,
			`SELECT COALESCE(default_project_id, 0) FROM users WHERE id=$1`,
			userID,
		).Scan(&defaultPID); err != nil {
			slog.Warn("ScreenData user default_project query failed", "err", err)
		}
		if defaultPID > 0 {
			// 默认项目 + 所有 project_user 授权的项目（UNION 去重）
			// 子查询包裹 UNION 以支持 ORDER BY 表达式（PG 不允许 UNION 直接 ORDER BY 表达式）
			projQ = `SELECT * FROM (
				 SELECT id,name FROM project WHERE id=$1
				 UNION
				 SELECT p.id,p.name FROM project p JOIN project_user pu ON pu.project_id=p.id WHERE pu.user_id=$2
			) sub ORDER BY CASE WHEN id=$1 THEN 0 ELSE 1 END, id`
			projArgs = []any{defaultPID, userID}
			out["default_project_id"] = defaultPID
		} else {
			// 检查 project_user 表作为 fallback（无默认项目但被分配了项目）
			noProjectMsg = "您的名下无项目"
			projQ = `SELECT p.id,p.name FROM project p JOIN project_user pu ON pu.project_id=p.id WHERE pu.user_id=$1 ORDER BY p.id`
			projArgs = []any{userID}
		}
	} else {
		// 未认证用户 → 返回空列表
		projQ = `SELECT id,name FROM project WHERE 1=0`
	}
	if noProjectMsg != "" {
		out["message"] = noProjectMsg
	}
	pr, err := h.pool.Query(ctx, projQ, projArgs...)
	if err != nil {
		slog.Warn("ScreenData projects query failed", "err", err)
	}
	var projs []map[string]any
	if pr != nil {
		for pr.Next() {
			var id int
			var n string
			if scanErr := pr.Scan(&id, &n); scanErr != nil {
				slog.Warn("ScreenData project scan failed", "err", scanErr)
				continue
			}
			projs = append(projs, map[string]any{"id": id, "name": n})
		}
		pr.Close()
	}
	if projs == nil { projs = []map[string]any{} }
	out["projects"] = projs
	{
		rc := ""; rl := -1
		if claims != nil { rc = claims.RoleCode; if claims.RoleLevel != nil { rl = *claims.RoleLevel } }
		slog.Info("ScreenData projects", "user", userID, "defaultPID", defaultPID, "count", len(projs), "roleCode", rc, "roleLevel", rl)
	}
	if pid == 0 && len(projs) > 0 {
		if id, ok := projs[0]["id"].(int); ok {
			pid = id
		}
	}

	// Buildings
	br, err := h.pool.Query(ctx, `SELECT id,name FROM building WHERE ($1=0 OR project_id=$1) ORDER BY id`, pid)
	if err != nil {
		slog.Warn("ScreenData buildings query failed", "err", err)
	}
	var blds []map[string]any
	if br != nil {
		for br.Next() {
			var id int
			var n string
			if err := br.Scan(&id, &n); err != nil {
				slog.Warn("ScreenData building scan failed", "err", err)
				continue
			}
			blds = append(blds, map[string]any{"id": id, "name": n})
		}
		br.Close()
	}
	if blds == nil { blds = []map[string]any{} }
	out["buildings"] = blds
	if bid == 0 && len(blds) > 0 {
		if id, ok := blds[0]["id"].(int); ok {
			bid = id
		}
	}

	// ---- parallel: all queries below depend only on pid/bid and are independent ----
	var wg sync.WaitGroup
	var (
		devs        []map[string]any
		tasks       []map[string]any
		alarms      []map[string]any
		meters      []map[string]any
		weatherData map[string]string
		savingRate  float64
		meterPower  float64
		days        int
	)
	// 4 parallel groups — peak 4 DB connections + 1 external HTTP call
	errs := make(chan error, 4)

	// Group 1: Devices (heaviest SQL with LATERAL joins)
	wg.Add(1)
	go func() {
		defer func() {
			if r := recover(); r != nil {
				slog.Error("ScreenData goroutine panic recovered", "panic", r)
				errs <- fmt.Errorf("internal error")
			}
			wg.Done()
		}()
		dr, err := h.pool.Query(ctx,
			`SELECT d.id, d.name, d.device_type, d.online_status,
			        CASE WHEN fp.device_id IS NOT NULL THEN '故障' ELSE COALESCE(d.device_status, '开机') END,
			        COALESCE(kp.key_values, '')
			 FROM device d
			 JOIN building b ON b.id=d.building_id
			 LEFT JOIN LATERAL (
			   SELECT string_agg(dp.prop_name || ': ' || COALESCE(dp.prop_value,'') || COALESCE(' '||dp.unit,''), '  |  ' ORDER BY CASE WHEN dp.operation_type='开关机' THEN 0 ELSE 1 END, dp.sort_order)
			   FROM device_properties dp WHERE dp.device_id=d.id AND dp.is_key=true
			 ) kp(key_values) ON true
			 LEFT JOIN LATERAL (
			   SELECT 1 FROM device_properties dp2 WHERE dp2.device_id=d.id AND dp2.prop_name='故障报警' AND dp2.prop_value='故障' LIMIT 1
			 ) fp(device_id) ON true
			 WHERE ($1=0 OR b.project_id=$1) AND ($2=0 OR d.building_id=$2)
			 ORDER BY d.id`, pid, bid)
		if err != nil {
			slog.Warn("ScreenData devices query failed", "err", err)
			errs <- err
			return
		}
		defer dr.Close()
		for dr.Next() {
			var id int
			var n, tp, st, ds, kv string
			if scanErr := dr.Scan(&id, &n, &tp, &st, &ds, &kv); scanErr != nil {
				slog.Warn("ScreenData device scan failed", "err", scanErr)
				continue
			}
			devs = append(devs, map[string]any{"id": id, "name": n, "type": tp, "status": st, "device_status": ds, "key_info": kv})
		}
	}()

	// Group 2: Weather (external HTTP call — slowest, run in parallel)
	wg.Add(1)
	go func() {
		defer func() {
			if r := recover(); r != nil {
				slog.Error("ScreenData goroutine panic recovered", "panic", r)
				errs <- fmt.Errorf("internal error")
			}
			wg.Done()
		}()
		var cityName string
		if err := h.pool.QueryRow(ctx, `SELECT COALESCE(c.name,'') FROM project p LEFT JOIN city c ON c.id=p.city_id WHERE p.id=$1`, pid).Scan(&cityName); err != nil {
			slog.Warn("ScreenData city name query failed", "err", err)
			return
		}
		if cityName == "" {
			return
		}
		if wd, wErr := h.weatherSvc.GetNowByCityName(ctx, cityName); wErr == nil && wd != nil {
			weatherData = map[string]string{
				"city": wd.CityName, "temp": wd.Temp, "text": wd.WeatherText,
				"wind_dir": wd.WindDir, "wind_scale": wd.WindScale, "humidity": wd.Humidity,
			}
		}
	}()

	// Group 3: Tasks + Alarms (two small queries, sequential within the goroutine)
	wg.Add(1)
	go func() {
		defer func() {
			if r := recover(); r != nil {
				slog.Error("ScreenData goroutine panic recovered", "panic", r)
				errs <- fmt.Errorf("internal error")
			}
			wg.Done()
		}()
		// tasks
		tr, err := h.pool.Query(ctx, `SELECT st.name,d.name,st.action_type,st.schedule_time::text,st.enabled,COALESCE(st.last_result,'') FROM scheduled_task st JOIN device d ON d.id=st.device_id WHERE ($1=0 OR st.building_id IN (SELECT id FROM building WHERE project_id=$1)) AND ($2=0 OR st.building_id=$2) ORDER BY st.id LIMIT 10`, pid, bid)
		if err != nil {
			slog.Warn("ScreenData tasks query failed", "err", err)
		} else {
			defer tr.Close()
			for tr.Next() {
				var n, dn, at, ti, res string
				var en bool
				if tr.Scan(&n, &dn, &at, &ti, &en, &res) == nil {
					tasks = append(tasks, map[string]any{"name": n, "device": dn, "action": at, "time": ti, "enabled": en, "result": res})
				}
			}
		}
		// alarms — filtered by project/building via device ownership
		ar, err := h.pool.Query(ctx, `SELECT COALESCE(al.device_name,''),al.alarm_type,al.level,al.message,to_char(al.created_at,'MM-DD HH24:MI')
			FROM alarm_log al
			WHERE ($1=0 OR al.device_id IN (
				SELECT id FROM device WHERE building_id IN (SELECT id FROM building WHERE project_id=$1)
			))
			AND ($2=0 OR al.device_id IN (
				SELECT id FROM device WHERE building_id=$2
			))
			ORDER BY al.created_at DESC LIMIT 20`, pid, bid)
		if err != nil {
			slog.Warn("ScreenData alarms query failed", "err", err)
		} else {
			defer ar.Close()
			for ar.Next() {
				var dev, at, lv, msg, ti string
				if ar.Scan(&dev, &at, &lv, &msg, &ti) == nil {
					alarms = append(alarms, map[string]any{"device": dev, "type": at, "level": lv, "msg": msg, "time": ti})
				}
			}
		}
	}()

	// Group 4: Energy (savingRate + meterPower + runningDays + meters)
	wg.Add(1)
	go func() {
		defer func() {
			if r := recover(); r != nil {
				slog.Error("ScreenData goroutine panic recovered", "panic", r)
				errs <- fmt.Errorf("internal error")
			}
			wg.Done()
		}()
		// saving rate — prefer exact building match, fall back to first building
		if err := h.pool.QueryRow(ctx, `SELECT COALESCE(save_rate,0) FROM building WHERE ($1=0 OR id=$1) ORDER BY id LIMIT 1`, bid).Scan(&savingRate); err != nil {
			slog.Warn("ScreenData save_rate query failed", "err", err)
		}
		// meter power aggregate
		if err := h.pool.QueryRow(ctx, `SELECT COALESCE(SUM(CASE WHEN COALESCE(d.power_sign,1)=-1 THEN -v ELSE v END),0) FROM (SELECT DISTINCT ON(device_id) device_id, value v FROM device_telemetry WHERE metric IN ('有功功率','active_power','P') AND device_id IN (SELECT id FROM device WHERE device_type='电表' AND ($1=0 OR building_id IN (SELECT id FROM building WHERE project_id=$1)) AND ($2=0 OR building_id=$2)) ORDER BY device_id,ts DESC) _ JOIN device d ON d.id=_.device_id`, pid, bid).Scan(&meterPower); err != nil {
			slog.Warn("ScreenData meter_power query failed", "err", err)
		}
		// running days
		if err := h.pool.QueryRow(ctx, `SELECT COALESCE(EXTRACT(DAY FROM NOW()-MIN(created_at))::int,0) FROM project WHERE ($1=0 OR id=$1)`, pid).Scan(&days); err != nil {
			slog.Warn("ScreenData running days query failed", "err", err)
		}
		// individual meters — use LATERAL join instead of correlated subquery
		mr, err := h.pool.Query(ctx, `SELECT d.id, d.name, COALESCE(d.power_sign,1), COALESCE(t.value, 0)
			FROM device d
			LEFT JOIN LATERAL (
				SELECT value FROM device_telemetry
				WHERE device_id = d.id AND metric IN ('有功功率','active_power','P')
				ORDER BY ts DESC LIMIT 1
			) t ON true
			WHERE d.device_type='电表' AND ($1=0 OR d.building_id IN (SELECT id FROM building WHERE project_id=$1)) AND ($2=0 OR d.building_id=$2)
			ORDER BY d.id`, pid, bid)
		if err != nil {
			slog.Warn("ScreenData meters query failed", "err", err)
		} else {
			defer mr.Close()
			for mr.Next() {
				var id int
				var n string
				var sign int
				var p float64
				if mr.Scan(&id, &n, &sign, &p) == nil {
					meters = append(meters, map[string]any{"id": id, "name": n, "sign": sign, "power": p})
				}
			}
		}
	}()

	wg.Wait()
	close(errs)
	// Log all errors from goroutines (informational — partial data is still served)
	for err := range errs {
		slog.Warn("ScreenData parallel query failed", "err", err)
	}

	// ---- assemble response ----
	if devs == nil { devs = []map[string]any{} }
	out["devices"] = devs
	if weatherData != nil { out["weather"] = weatherData }
	if tasks == nil { tasks = []map[string]any{} }
	out["tasks"] = tasks
	if alarms == nil { alarms = []map[string]any{} }
	out["alarms"] = alarms
	out["saving_rate"] = savingRate
	out["meter_power"] = meterPower
	out["power_saved"] = meterPower * savingRate
	out["carbon_saved"] = meterPower * savingRate * carbonFactor
	out["running_days"] = days
	if meters == nil { meters = []map[string]any{} }
	out["meters"] = meters

	ok(w, out)
}
