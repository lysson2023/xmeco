package handler

import (
	"encoding/json"
	"fmt"
	"net/http"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type StartupHandler struct{ pool *pgxpool.Pool }

func NewStartupHandler(pool *pgxpool.Pool) *StartupHandler { return &StartupHandler{pool} }

func (h *StartupHandler) ListPlans(w http.ResponseWriter, r *http.Request) {
	buildingID := queryInt(r, "building_id")
	var rows pgx.Rows
	var err error
	if buildingID > 0 {
		rows, err = h.pool.Query(r.Context(), "SELECT p.id,p.name,p.building_id,p.plan_type,p.precheck_online,p.precheck_alarm,p.enabled,COALESCE((SELECT json_agg(json_build_object('id',s.id,'device_id',s.device_id,'device_name',d.name,'sort_order',s.sort_order,'wait_seconds',s.wait_seconds,'action',s.action) ORDER BY s.sort_order) FROM startup_step s JOIN device d ON d.id=s.device_id WHERE s.plan_id=p.id),'[]'::json) FROM startup_plan p WHERE ($1=0 OR p.building_id=$1) ORDER BY p.id", buildingID)
	} else {
		rows, err = h.pool.Query(r.Context(), "SELECT p.id,p.name,p.building_id,p.plan_type,p.precheck_online,p.precheck_alarm,p.enabled,COALESCE((SELECT json_agg(json_build_object('id',s.id,'device_id',s.device_id,'device_name',d.name,'sort_order',s.sort_order,'wait_seconds',s.wait_seconds,'action',s.action) ORDER BY s.sort_order) FROM startup_step s JOIN device d ON d.id=s.device_id WHERE s.plan_id=p.id),'[]'::json) FROM startup_plan p ORDER BY p.id")
	}
	if err != nil { serverErr(w, err); return }
	defer rows.Close()
	type plan struct {
		ID, BID int; Name, PlanType string; PreOn, PreAl, En bool; Steps []byte
	}
	var list []plan
	for rows.Next() {
		var p plan; rows.Scan(&p.ID,&p.Name,&p.BID,&p.PlanType,&p.PreOn,&p.PreAl,&p.En,&p.Steps)
		list = append(list, p)
	}
	if list == nil { list = []plan{} }
	ok(w, list)
}

func (h *StartupHandler) CreatePlan(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Name string `json:"name"`
		BuildingID int `json:"building_id"`
		PlanType string `json:"plan_type"`
		Steps []struct{ DeviceID int `json:"device_id"`; DeviceName string `json:"device_name"`; SortOrder int `json:"sort_order"`; WaitSeconds int `json:"wait_seconds"`; Action string `json:"action"` } `json:"steps"`
	}
	json.NewDecoder(r.Body).Decode(&body)
	if body.PlanType == "" { body.PlanType = "startup" }
	var id int
	err := h.pool.QueryRow(r.Context(), "INSERT INTO startup_plan (name,building_id,plan_type) VALUES($1,$2,$3) RETURNING id", body.Name, body.BuildingID, body.PlanType).Scan(&id)
	if err != nil { serverErr(w, err); return }
	for _, s := range body.Steps {
		action := s.Action
		if action == "" { action = body.PlanType }
		h.pool.Exec(r.Context(), "INSERT INTO startup_step (plan_id,sort_order,device_id,property_id,action,target_value,wait_seconds) VALUES($1,$2,$3,0,$4,'',COALESCE($5,30))", id, s.SortOrder, s.DeviceID, action, s.WaitSeconds)
	}
	ok(w, M{"id":fmt.Sprint(id)})
}

func (h *StartupHandler) UpdatePlan(w http.ResponseWriter, r *http.Request) {
	id := pathLast(r.URL.Path)
	var body struct {
		Name string `json:"name"`
		PlanType string `json:"plan_type"`
		Steps []struct{ DeviceID int `json:"device_id"`; SortOrder int `json:"sort_order"`; WaitSeconds int `json:"wait_seconds"`; Action string `json:"action"` } `json:"steps"`
	}
	json.NewDecoder(r.Body).Decode(&body)
	h.pool.Exec(r.Context(), "UPDATE startup_plan SET name=$1,plan_type=$2 WHERE id=$3", body.Name, body.PlanType, id)
	h.pool.Exec(r.Context(), "DELETE FROM startup_step WHERE plan_id=$1", id)
	for _, s := range body.Steps {
		action := s.Action
		if action == "" { action = body.PlanType }
		h.pool.Exec(r.Context(), "INSERT INTO startup_step (plan_id,sort_order,device_id,property_id,action,target_value,wait_seconds) VALUES($1,$2,$3,0,$4,'',COALESCE($5,30))", id, s.SortOrder, s.DeviceID, action, s.WaitSeconds)
	}
	ok(w, M{"status":"updated"})
}

func (h *StartupHandler) DeletePlan(w http.ResponseWriter, r *http.Request) {
	id := pathLast(r.URL.Path)
	h.pool.Exec(r.Context(), "DELETE FROM startup_step WHERE plan_id=$1", id)
	h.pool.Exec(r.Context(), "DELETE FROM startup_plan WHERE id=$1", id)
	ok(w, M{"status":"deleted"})
}

func (h *StartupHandler) Execute(w http.ResponseWriter, r *http.Request) {
	id := pathLast(r.URL.Path)
	h.pool.Exec(r.Context(), "INSERT INTO startup_execution (plan_id,plan_name,status,total_steps) SELECT $1,name,'running',0 FROM startup_plan WHERE id=$1", id)
	ok(w, M{"status":"started"})
}

func (h *StartupHandler) GetExecution(w http.ResponseWriter, r *http.Request) {
	ok(w, M{"status":"ok"})
}

func (h *StartupHandler) StopExecution(w http.ResponseWriter, r *http.Request) {
	ok(w, M{"status":"stopped"})
}