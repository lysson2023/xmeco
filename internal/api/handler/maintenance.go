package handler

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"time"

	"github.com/jackc/pgx/v5"

	"xmeco/internal/repository/postgres"
)

type MaintenanceHandler struct{ pool postgres.DBTX }

func NewMaintenanceHandler(pool postgres.DBTX) *MaintenanceHandler {
	return &MaintenanceHandler{pool}
}

type maintRecord struct {
	ID          int      `json:"id"`
	DeviceID    int      `json:"device_id"`
	DeviceName  string   `json:"device_name"`
	BuildingID  int      `json:"building_id"`
	ProjectID   int      `json:"project_id"`
	Name        string   `json:"name"`
	Company     string   `json:"company"`
	RecordType  string   `json:"record_type"`
	Description string   `json:"description"`
	Operator    string   `json:"operator"`
	RecordDate  string   `json:"record_date"`
	NextDate    *string  `json:"next_date"`
	Cost        float64  `json:"cost"`
	Status      string   `json:"status"`
	Remark      *string  `json:"remark"`
	CreatedAt   string   `json:"created_at"`
}

// List returns maintenance records, filterable by building_id, device_id, and record_type.
func (h *MaintenanceHandler) List(w http.ResponseWriter, r *http.Request) {
	buildingID := queryInt(r, "building_id")
	deviceID := queryInt(r, "device_id")
	projectID := queryInt(r, "project_id")

	baseQ := `SELECT mr.id, mr.device_id, COALESCE(NULLIF(mr.device_name,''), d.name, ''), mr.building_id,
		COALESCE(mr.project_id,0), COALESCE(mr.name,''), COALESCE(mr.company,''),
		mr.record_type, COALESCE(mr.description,''),
		COALESCE(mr.operator,''), mr.record_date::text,
		COALESCE(mr.next_date::text,''), COALESCE(mr.cost,0),
		mr.status, mr.remark, COALESCE(mr.created_at::text,'')
		FROM maintenance_record mr
		LEFT JOIN device d ON d.id = mr.device_id`
	var conditions []string
	var args []any
	argIdx := 1

	if buildingID > 0 {
		conditions = append(conditions, "mr.building_id=$"+itos(argIdx))
		args = append(args, buildingID)
		argIdx++
	}
	if deviceID > 0 {
		conditions = append(conditions, "mr.device_id=$"+itos(argIdx))
		args = append(args, deviceID)
		argIdx++
	}
	if projectID > 0 {
		conditions = append(conditions, "mr.project_id=$"+itos(argIdx))
		args = append(args, projectID)
		argIdx++
	}

	if len(conditions) > 0 {
		baseQ += " WHERE " + conditions[0]
		for i := 1; i < len(conditions); i++ {
			baseQ += " AND " + conditions[i]
		}
	}
	baseQ += " ORDER BY mr.record_date DESC, mr.id DESC LIMIT 200"

	rows, err := h.pool.Query(r.Context(), baseQ, args...)
	if err != nil { serverErr(w, err); return }
	defer rows.Close()

	var list []maintRecord
	for rows.Next() {
		var rec maintRecord
		var nextDate, remark *string
		if err := rows.Scan(&rec.ID, &rec.DeviceID, &rec.DeviceName, &rec.BuildingID,
			&rec.ProjectID, &rec.Name, &rec.Company, &rec.RecordType, &rec.Description, &rec.Operator,
			&rec.RecordDate, &nextDate, &rec.Cost, &rec.Status, &remark, &rec.CreatedAt); err != nil {
			slog.Warn("maintenance List scan failed", "err", err)
			continue
		}
		if nextDate != nil && *nextDate != "" { rec.NextDate = nextDate } else { rec.NextDate = nil }
		if remark != nil && *remark != "" { rec.Remark = remark } else { rec.Remark = nil }
		list = append(list, rec)
	}
	if list == nil { list = []maintRecord{} }
	if err := rows.Err(); err != nil { serverErr(w, err); return }
	ok(w, list)
}

// Create adds a new maintenance record.
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
		writeJSON(w, http.StatusBadRequest, M{"error": "请求格式错误"})
		return
	}
	if body.DeviceID == 0 {
		writeJSON(w, http.StatusBadRequest, M{"error": "设备ID不能为空"})
		return
	}
	if body.RecordType == "" { body.RecordType = "维修" }
	if body.Status == "" { body.Status = "已完成" }
	if body.RecordDate == "" { body.RecordDate = time.Now().Format("2006-01-02") }

	// Resolve device_name, building_id, project_id if not provided
	if body.DeviceName == "" || body.BuildingID == 0 || body.ProjectID == 0 {
		var devName string
		var bldID, projID int
		err := h.pool.QueryRow(r.Context(),
			`SELECT d.name, d.building_id, b.project_id
			 FROM device d JOIN building b ON b.id=d.building_id
			 WHERE d.id=$1`, body.DeviceID).
			Scan(&devName, &bldID, &projID)
		if errors.Is(err, pgx.ErrNoRows) {
			writeJSON(w, http.StatusBadRequest, M{"error": "设备不存在"})
			return
		}
		if err != nil {
			serverErr(w, err)
			return
		}
		if body.DeviceName == "" { body.DeviceName = devName }
		if body.BuildingID == 0 { body.BuildingID = bldID }
		if body.ProjectID == 0 { body.ProjectID = projID }
	}

	var id int
	err := h.pool.QueryRow(r.Context(),
		`INSERT INTO maintenance_record (device_id, device_name, building_id, project_id, name, company, record_type, description, operator, record_date, next_date, cost, status, remark)
		 SELECT $1, COALESCE(NULLIF($2,''), d.name), $3, $4, $5, $6, $7, $8, $9, $10::date, $11::date, $12, $13, $14
		 FROM device d WHERE d.id=$1
		 RETURNING maintenance_record.id`,
		body.DeviceID, body.DeviceName, body.BuildingID, body.ProjectID,
		body.Name, body.Company,
		body.RecordType, body.Description, body.Operator,
		body.RecordDate, body.NextDate, body.Cost, body.Status, body.Remark,
	).Scan(&id)
	if err != nil { serverErr(w, err); return }
	ok(w, M{"id": id, "status": "created"})
}

// Update modifies an existing maintenance record.
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
		writeJSON(w, http.StatusBadRequest, M{"error": "请求格式错误"})
		return
	}
	_, err := h.pool.Exec(r.Context(),
		`UPDATE maintenance_record SET name=$1, company=$2, record_type=$3, description=$4, operator=$5,
		 record_date=$6::date, next_date=$7::date, cost=$8, status=$9, remark=$10, updated_at=NOW()
		 WHERE id=$11`,
		body.Name, body.Company,
		body.RecordType, body.Description, body.Operator,
		body.RecordDate, body.NextDate, body.Cost, body.Status, body.Remark, id)
	if err != nil { serverErr(w, err); return }
	ok(w, M{"status": "updated"})
}

// Delete removes a maintenance record.
func (h *MaintenanceHandler) Delete(w http.ResponseWriter, r *http.Request) {
	id := pathID(r)
	if _, err := h.pool.Exec(r.Context(), `DELETE FROM maintenance_record WHERE id=$1`, id); err != nil {
		serverErr(w, err)
		return
	}
	ok(w, M{"status": "deleted"})
}
