package postgres

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5"
)

// MaintenanceRepo 封装 maintenance_record 表的 CRUD 操作
type MaintenanceRepo struct{ pool DBTX }

func NewMaintenanceRepo(pool DBTX) *MaintenanceRepo { return &MaintenanceRepo{pool} }

// MaintenanceRecord 维保记录结构
type MaintenanceRecord struct {
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

// MaintenanceFilter 查询过滤条件
type MaintenanceFilter struct {
	BuildingID int
	DeviceID   int
	ProjectID  int
}

// List 返回维保记录列表
func (r *MaintenanceRepo) List(ctx context.Context, filter MaintenanceFilter) ([]MaintenanceRecord, error) {
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

	if filter.BuildingID > 0 {
		conditions = append(conditions, "mr.building_id=$"+itos(argIdx))
		args = append(args, filter.BuildingID)
		argIdx++
	}
	if filter.DeviceID > 0 {
		conditions = append(conditions, "mr.device_id=$"+itos(argIdx))
		args = append(args, filter.DeviceID)
		argIdx++
	}
	if filter.ProjectID > 0 {
		conditions = append(conditions, "mr.project_id=$"+itos(argIdx))
		args = append(args, filter.ProjectID)
		argIdx++
	}

	if len(conditions) > 0 {
		baseQ += " WHERE " + conditions[0]
		for i := 1; i < len(conditions); i++ {
			baseQ += " AND " + conditions[i]
		}
	}
	baseQ += " ORDER BY mr.record_date DESC, mr.id DESC LIMIT 200"

	rows, err := r.pool.Query(ctx, baseQ, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []MaintenanceRecord
	for rows.Next() {
		var rec MaintenanceRecord
		var nextDate, remark *string
		if err := rows.Scan(&rec.ID, &rec.DeviceID, &rec.DeviceName, &rec.BuildingID,
			&rec.ProjectID, &rec.Name, &rec.Company, &rec.RecordType, &rec.Description, &rec.Operator,
			&rec.RecordDate, &nextDate, &rec.Cost, &rec.Status, &remark, &rec.CreatedAt); err != nil {
			slog.Warn("MaintenanceRepo.List scan failed", "err", err)
			continue
		}
		if nextDate != nil && *nextDate != "" {
			rec.NextDate = nextDate
		} else {
			rec.NextDate = nil
		}
		if remark != nil && *remark != "" {
			rec.Remark = remark
		} else {
			rec.Remark = nil
		}
		list = append(list, rec)
	}
	return list, rows.Err()
}

// GetByID 返回单条维保记录，未找到返回 nil, nil
func (r *MaintenanceRepo) GetByID(ctx context.Context, id int) (*MaintenanceRecord, error) {
	var rec MaintenanceRecord
	var nextDate, remark *string
	err := r.pool.QueryRow(ctx,
		`SELECT mr.id, mr.device_id, COALESCE(NULLIF(mr.device_name,''), d.name, ''), mr.building_id,
			COALESCE(mr.project_id,0), COALESCE(mr.name,''), COALESCE(mr.company,''),
			mr.record_type, COALESCE(mr.description,''),
			COALESCE(mr.operator,''), mr.record_date::text,
			COALESCE(mr.next_date::text,''), COALESCE(mr.cost,0),
			mr.status, mr.remark, COALESCE(mr.created_at::text,'')
			FROM maintenance_record mr
			LEFT JOIN device d ON d.id = mr.device_id
			WHERE mr.id=$1`, id).
		Scan(&rec.ID, &rec.DeviceID, &rec.DeviceName, &rec.BuildingID,
			&rec.ProjectID, &rec.Name, &rec.Company, &rec.RecordType, &rec.Description, &rec.Operator,
			&rec.RecordDate, &nextDate, &rec.Cost, &rec.Status, &remark, &rec.CreatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	if nextDate != nil && *nextDate != "" {
		rec.NextDate = nextDate
	}
	if remark != nil && *remark != "" {
		rec.Remark = remark
	}
	return &rec, nil
}

// CreateReq 创建维保记录请求
type MaintenanceCreateReq struct {
	DeviceID    int
	DeviceName  string
	BuildingID  int
	ProjectID   int
	Name        string
	Company     string
	RecordType  string
	Description string
	Operator    string
	RecordDate  string
	NextDate    *string
	Cost        float64
	Status      string
	Remark      *string
}

// Create 创建维保记录，返回生成的 ID
func (r *MaintenanceRepo) Create(ctx context.Context, req MaintenanceCreateReq) (int, error) {
	// 自动填充默认值
	if req.RecordType == "" {
		req.RecordType = "维修"
	}
	if req.Status == "" {
		req.Status = "已完成"
	}
	if req.RecordDate == "" {
		req.RecordDate = time.Now().Format("2006-01-02")
	}

	// 如果缺少设备信息，从数据库查询
	if req.DeviceName == "" || req.BuildingID == 0 || req.ProjectID == 0 {
		var devName string
		var bldID, projID int
		err := r.pool.QueryRow(ctx,
			`SELECT d.name, d.building_id, b.project_id
			 FROM device d JOIN building b ON b.id=d.building_id
			 WHERE d.id=$1`, req.DeviceID).
			Scan(&devName, &bldID, &projID)
		if err != nil {
			return 0, err
		}
		if req.DeviceName == "" {
			req.DeviceName = devName
		}
		if req.BuildingID == 0 {
			req.BuildingID = bldID
		}
		if req.ProjectID == 0 {
			req.ProjectID = projID
		}
	}

	var id int
	err := r.pool.QueryRow(ctx,
		`INSERT INTO maintenance_record (device_id, device_name, building_id, project_id, name, company, record_type, description, operator, record_date, next_date, cost, status, remark)
		 SELECT $1, COALESCE(NULLIF($2,''), d.name), $3, $4, $5, $6, $7, $8, $9, $10::date, $11::date, $12, $13, $14
		 FROM device d WHERE d.id=$1
		 RETURNING maintenance_record.id`,
		req.DeviceID, req.DeviceName, req.BuildingID, req.ProjectID,
		req.Name, req.Company, req.RecordType, req.Description, req.Operator,
		req.RecordDate, req.NextDate, req.Cost, req.Status, req.Remark).Scan(&id)
	return id, err
}

// UpdateReq 更新维保记录请求
type MaintenanceUpdateReq struct {
	Name        string
	Company     string
	RecordType  string
	Description string
	Operator    string
	RecordDate  string
	NextDate    *string
	Cost        float64
	Status      string
	Remark      *string
}

// Update 更新维保记录
func (r *MaintenanceRepo) Update(ctx context.Context, id int, req MaintenanceUpdateReq) error {
	_, err := r.pool.Exec(ctx,
		`UPDATE maintenance_record SET name=$1, company=$2, record_type=$3, description=$4, operator=$5,
		 record_date=$6::date, next_date=$7::date, cost=$8, status=$9, remark=$10, updated_at=NOW()
		 WHERE id=$11`,
		req.Name, req.Company, req.RecordType, req.Description, req.Operator,
		req.RecordDate, req.NextDate, req.Cost, req.Status, req.Remark, id)
	return err
}

// Delete 删除维保记录
func (r *MaintenanceRepo) Delete(ctx context.Context, id int) error {
	_, err := r.pool.Exec(ctx, "DELETE FROM maintenance_record WHERE id=$1", id)
	return err
}