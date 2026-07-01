package handler

import (
	"encoding/json"
	"net/http"

	"xmeco/internal/repository/postgres"
)

type MaintenanceHandler struct{ repo *postgres.MaintenanceRepo }

func NewMaintenanceHandler(repo *postgres.MaintenanceRepo) *MaintenanceHandler {
	return &MaintenanceHandler{repo: repo}
}

func (h *MaintenanceHandler) List(w http.ResponseWriter, r *http.Request) {
	filter := postgres.MaintenanceFilter{
		BuildingID: queryInt(r, "building_id"),
		DeviceID:   queryInt(r, "device_id"),
		ProjectID:  queryInt(r, "project_id"),
	}
	list, err := h.repo.List(r.Context(), filter)
	if err != nil {
		serverErr(w, err)
		return
	}
	if list == nil {
		list = []postgres.MaintenanceRecord{}
	}
	ok(w, list)
}

func (h *MaintenanceHandler) Create(w http.ResponseWriter, r *http.Request) {
	var body struct {
		DeviceID    int     `json:"device_id"`
		DeviceName  string  `json:"device_name"`
		BuildingID  int     `json:"building_id"`
		ProjectID   int     `json:"project_id"`
		Name        string  `json:"name"`
		Company     string  `json:"company"`
		RecordType  string  `json:"record_type"`
		Description string  `json:"description"`
		Operator    string  `json:"operator"`
		RecordDate  string  `json:"record_date"`
		NextDate    *string `json:"next_date"`
		Cost        float64 `json:"cost"`
		Status      string  `json:"status"`
		Remark      *string `json:"remark"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeJSON(w, http.StatusBadRequest, errBadRequest)
		return
	}
	if body.DeviceID == 0 {
		writeJSON(w, http.StatusBadRequest, M{"error": "设备ID不能为空"})
		return
	}

	req := postgres.MaintenanceCreateReq{
		DeviceID:    body.DeviceID,
		DeviceName:  body.DeviceName,
		BuildingID:  body.BuildingID,
		ProjectID:   body.ProjectID,
		Name:        body.Name,
		Company:     body.Company,
		RecordType:  body.RecordType,
		Description: body.Description,
		Operator:    body.Operator,
		RecordDate:  body.RecordDate,
		NextDate:    body.NextDate,
		Cost:        body.Cost,
		Status:      body.Status,
		Remark:      body.Remark,
	}

	id, err := h.repo.Create(r.Context(), req)
	if err != nil {
		serverErr(w, err)
		return
	}
	ok(w, M{"id": id, "status": "created"})
}

func (h *MaintenanceHandler) Update(w http.ResponseWriter, r *http.Request) {
	id := pathID(r)
	var body struct {
		Name        string  `json:"name"`
		Company     string  `json:"company"`
		RecordType  string  `json:"record_type"`
		Description string  `json:"description"`
		Operator    string  `json:"operator"`
		RecordDate  string  `json:"record_date"`
		NextDate    *string `json:"next_date"`
		Cost        float64 `json:"cost"`
		Status      string  `json:"status"`
		Remark      *string `json:"remark"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeJSON(w, http.StatusBadRequest, errBadRequest)
		return
	}

	req := postgres.MaintenanceUpdateReq{
		Name:        body.Name,
		Company:     body.Company,
		RecordType:  body.RecordType,
		Description: body.Description,
		Operator:    body.Operator,
		RecordDate:  body.RecordDate,
		NextDate:    body.NextDate,
		Cost:        body.Cost,
		Status:      body.Status,
		Remark:      body.Remark,
	}

	if err := h.repo.Update(r.Context(), id, req); err != nil {
		serverErr(w, err)
		return
	}
	ok(w, M{"status": "updated"})
}

func (h *MaintenanceHandler) Delete(w http.ResponseWriter, r *http.Request) {
	if err := h.repo.Delete(r.Context(), pathID(r)); err != nil {
		serverErr(w, err)
		return
	}
	ok(w, M{"status": "deleted"})
}