package handler

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"

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
		writeJSON(w, http.StatusBadRequest, M{"error": "请求格式错误"}); return
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

	// Projects — filter by assigned users; if no assignments, return empty (not all projects)
	var userID int
	if claims != nil { userID = claims.UserID }
	var projQ string
	var projArgs []any
	if userID > 0 {
		// 检查该用户是否被分配了项目
		var assigned int
		if err := h.pool.QueryRow(ctx, `SELECT count(*) FROM project_user WHERE user_id=$1`, userID).Scan(&assigned); err != nil {
			slog.Warn("ScreenData project_user count failed", "err", err)
		}
		if assigned > 0 {
			// 只返回被分配的项目
			projQ = `SELECT p.id,p.name FROM project p JOIN project_user pu ON pu.project_id=p.id WHERE pu.user_id=$1 ORDER BY p.id`
			projArgs = []any{userID}
		} else {
			// 检查是否为超级管理员（super_admin 角色），超管可看所有项目
			var roleCode string
			if err := h.pool.QueryRow(ctx, `SELECT r.code FROM users u JOIN role r ON r.id=u.role_id WHERE u.id=$1`, userID).Scan(&roleCode); err != nil {
				slog.Warn("ScreenData role code query failed", "err", err)
			}
			if roleCode == "super_admin" {
				projQ = `SELECT id,name FROM project ORDER BY id`
			} else {
				// 普通用户未分配项目 → 返回空列表
				projQ = `SELECT id,name FROM project WHERE 1=0`
			}
		}
	} else {
		// 未认证用户 → 返回空列表
		projQ = `SELECT id,name FROM project WHERE 1=0`
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
			if err := pr.Scan(&id, &n); err != nil {
				slog.Warn("ScreenData project scan failed", "err", err)
				continue
			}
			projs = append(projs, map[string]any{"id": id, "name": n})
		}
		pr.Close()
	}
	if projs == nil { projs = []map[string]any{} }
	out["projects"] = projs
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

	// Devices for topology (with key properties for big-screen display)
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
	var devs []map[string]any
	if dr != nil {
		for dr.Next() {
			var id int
			var n, tp, st, ds, kv string
			if scanErr := dr.Scan(&id, &n, &tp, &st, &ds, &kv); scanErr != nil {
				slog.Warn("ScreenData device scan failed", "err", scanErr)
				continue
			}
			devs = append(devs, map[string]any{"id": id, "name": n, "type": tp, "status": st, "device_status": ds, "key_info": kv})
		}
		dr.Close()
	}
	if err != nil {
		slog.Warn("ScreenData devices query failed", "err", err)
	}
	if devs == nil { devs = []map[string]any{} }
	out["devices"] = devs

	// Weather — use the weather service (cache + live wttr.in fetch with stale fallback)
	var cityName string
	if err := h.pool.QueryRow(ctx, `SELECT COALESCE(c.name,'') FROM project p LEFT JOIN city c ON c.id=p.city_id WHERE p.id=$1`, pid).Scan(&cityName); err != nil {
		slog.Warn("ScreenData city name query failed", "err", err)
	}
	if cityName != "" {
		if wd, err := h.weatherSvc.GetNowByCityName(ctx, cityName); err == nil && wd != nil {
			out["weather"] = map[string]string{
				"city":       wd.CityName,
				"temp":       wd.Temp,
				"text":       wd.WeatherText,
				"wind_dir":   wd.WindDir,
				"wind_scale": wd.WindScale,
				"humidity":   wd.Humidity,
			}
		}
	}

	// Scheduled tasks
	tr, err := h.pool.Query(ctx, `SELECT st.name,d.name,st.action_type,st.schedule_time::text,st.enabled,COALESCE(st.last_result,'') FROM scheduled_task st JOIN device d ON d.id=st.device_id WHERE ($1=0 OR st.building_id IN (SELECT id FROM building WHERE project_id=$1)) AND ($2=0 OR st.building_id=$2) ORDER BY st.id LIMIT 10`, pid, bid)
	if err != nil {
		slog.Warn("ScreenData tasks query failed", "err", err)
	}
	var tasks []map[string]any
	if tr != nil {
		for tr.Next() {
			var n, dn, at, ti, res string
			var en bool
			if err := tr.Scan(&n, &dn, &at, &ti, &en, &res); err != nil {
				slog.Warn("ScreenData task scan failed", "err", err)
				continue
			}
			tasks = append(tasks, map[string]any{"name": n, "device": dn, "action": at, "time": ti, "enabled": en, "result": res})
		}
		tr.Close()
	}
	if tasks == nil { tasks = []map[string]any{} }
	out["tasks"] = tasks

	// Alarms
	ar, err := h.pool.Query(ctx, `SELECT COALESCE(device_name,''),alarm_type,level,message,to_char(created_at,'MM-DD HH24:MI') FROM alarm_log ORDER BY created_at DESC LIMIT 20`)
	if err != nil {
		slog.Warn("ScreenData alarms query failed", "err", err)
	}
	var alarms []map[string]any
	if ar != nil {
		for ar.Next() {
			var dev, at, lv, msg, ti string
			if err := ar.Scan(&dev, &at, &lv, &msg, &ti); err != nil {
				slog.Warn("ScreenData alarm scan failed", "err", err)
				continue
			}
			alarms = append(alarms, map[string]any{"device": dev, "type": at, "level": lv, "msg": msg, "time": ti})
		}
		ar.Close()
	}
	if alarms == nil { alarms = []map[string]any{} }
	out["alarms"] = alarms

	// Energy
	var savingRate, meterPower float64
	if err := h.pool.QueryRow(ctx, `SELECT COALESCE(save_rate,0) FROM building WHERE ($1=0 OR id=$1) ORDER BY id LIMIT 1`, bid).Scan(&savingRate); err != nil {
		slog.Warn("ScreenData save_rate query failed", "err", err)
	}
	if err := h.pool.QueryRow(ctx, `SELECT COALESCE(SUM(CASE WHEN COALESCE(d.power_sign,1)=-1 THEN -v ELSE v END),0) FROM (SELECT DISTINCT ON(device_id) device_id, value v FROM device_telemetry WHERE metric IN ('有功功率','active_power','P') AND device_id IN (SELECT id FROM device WHERE device_type='电表' AND ($1=0 OR building_id IN (SELECT id FROM building WHERE project_id=$1)) AND ($2=0 OR building_id=$2)) ORDER BY device_id,ts DESC) _ JOIN device d ON d.id=_.device_id`, pid, bid).Scan(&meterPower); err != nil {
		slog.Warn("ScreenData meter_power query failed", "err", err)
	}
	out["saving_rate"] = savingRate
	out["meter_power"] = meterPower
	// savingRate stored as decimal (e.g., 0.25 = 25%), so multiply directly — no /100 needed.
	out["power_saved"] = meterPower * savingRate
	out["carbon_saved"] = meterPower * savingRate * carbonFactor

	// Running days
	var days int
	if err := h.pool.QueryRow(ctx, `SELECT COALESCE(EXTRACT(DAY FROM NOW()-MIN(created_at))::int,0) FROM project WHERE ($1=0 OR id=$1)`, pid).Scan(&days); err != nil {
		slog.Warn("ScreenData running days query failed", "err", err)
	}
	out["running_days"] = days

	// Individual meters (返回 power_sign 供前端显示加减标记)
	mr, err := h.pool.Query(ctx, `SELECT d.id,d.name,COALESCE(d.power_sign,1),COALESCE((SELECT value FROM device_telemetry WHERE device_id=d.id AND metric IN ('有功功率','active_power','P') ORDER BY ts DESC LIMIT 1),0) FROM device d WHERE d.device_type='电表' AND ($1=0 OR d.building_id IN (SELECT id FROM building WHERE project_id=$1)) AND ($2=0 OR d.building_id=$2) ORDER BY d.id`, pid, bid)
	if err != nil {
		slog.Warn("ScreenData meters query failed", "err", err)
	}
	var meters []map[string]any
	if mr != nil {
		for mr.Next() {
			var id int
			var n string
			var sign int
			var p float64
			if err := mr.Scan(&id, &n, &sign, &p); err != nil {
				slog.Warn("ScreenData meter scan failed", "err", err)
				continue
			}
			meters = append(meters, map[string]any{"id": id, "name": n, "sign": sign, "power": p})
		}
		mr.Close()
	}
	if meters == nil { meters = []map[string]any{} }
	out["meters"] = meters

	ok(w, out)
}
