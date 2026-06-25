package postgres

import (
	"context"
	"database/sql"
	"errors"
	"log/slog"
	"xmeco/internal/domain"
)

// ===== BUILDING =====
type BuildingRepo struct{ pool DBTX }
func NewBuildingRepo(p DBTX) *BuildingRepo { return &BuildingRepo{pool: p} }

func (r *BuildingRepo) List(ctx context.Context, projectID int) ([]domain.Building, error) {
	rows, err := r.pool.Query(ctx, "SELECT id,project_id,name,outdoor_temp,outdoor_humidity,total_energy,save_rate,save_energy,carbon_rate,carbon_saving,created_at FROM building WHERE ($1=0 OR project_id=$1) ORDER BY id", projectID)
	if err != nil { return nil, err }
	defer rows.Close()
	var list []domain.Building
	for rows.Next() {
		var b domain.Building
		if err := rows.Scan(&b.ID,&b.ProjectID,&b.Name,&b.OutdoorTemp,&b.OutdoorHumidity,&b.TotalEnergy,&b.SaveRate,&b.SaveEnergy,&b.CarbonRate,&b.CarbonSaving,&b.CreatedAt); err != nil {
			slog.Warn("BuildingRepo.List scan failed", "err", err)
			continue
		}
		list = append(list, b)
	}
	return list, rows.Err()
}
func (r *BuildingRepo) GetByID(ctx context.Context, id int) (*domain.Building, error) {
	var b domain.Building
	err := r.pool.QueryRow(ctx, "SELECT id,project_id,name,outdoor_temp,outdoor_humidity,total_energy,save_rate,save_energy,carbon_rate,carbon_saving,created_at FROM building WHERE id=$1", id).
		Scan(&b.ID,&b.ProjectID,&b.Name,&b.OutdoorTemp,&b.OutdoorHumidity,&b.TotalEnergy,&b.SaveRate,&b.SaveEnergy,&b.CarbonRate,&b.CarbonSaving,&b.CreatedAt)
	if errors.Is(err, sql.ErrNoRows) { return nil, err }
	return &b, err
}
func (r *BuildingRepo) Create(ctx context.Context, b *domain.Building) error {
	return r.pool.QueryRow(ctx,
		"INSERT INTO building (project_id,name,outdoor_temp,outdoor_humidity,total_energy,save_rate,save_energy,carbon_rate,carbon_saving) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9) RETURNING id,created_at",
		b.ProjectID, b.Name, b.OutdoorTemp, b.OutdoorHumidity, b.TotalEnergy, b.SaveRate, b.SaveEnergy, b.CarbonRate, b.CarbonSaving).Scan(&b.ID, &b.CreatedAt)
}
func (r *BuildingRepo) Update(ctx context.Context, b *domain.Building) error {
	_, err := r.pool.Exec(ctx,
		"UPDATE building SET name=$1, outdoor_temp=$2, outdoor_humidity=$3, total_energy=$4, save_rate=$5, save_energy=$6, carbon_rate=$7, carbon_saving=$8 WHERE id=$9",
		b.Name, b.OutdoorTemp, b.OutdoorHumidity, b.TotalEnergy, b.SaveRate, b.SaveEnergy, b.CarbonRate, b.CarbonSaving, b.ID)
	return err
}
func (r *BuildingRepo) Delete(ctx context.Context, id int) error {
	_, err := r.pool.Exec(ctx, "DELETE FROM building WHERE id=$1", id)
	return err
}

// ===== DEVICE =====
type DeviceRepo struct{ pool DBTX }
func NewDeviceRepo(p DBTX) *DeviceRepo { return &DeviceRepo{pool: p} }

func (r *DeviceRepo) List(ctx context.Context, buildingID int) ([]domain.Device, error) {
	rows, err := r.pool.Query(ctx, "SELECT id,building_id,name,device_type,gateway_imei,COALESCE(gateway_type,'custom'),node_address,device_no,ct_ratio,pt_ratio,rated_voltage,rated_current,COALESCE(online_status,'在线'),COALESCE(device_status,'开机'),last_online_at,last_record_at,created_at FROM device WHERE ($1=0 OR building_id=$1) ORDER BY id", buildingID)
	if err != nil { return nil, err }
	defer rows.Close()
	var list []domain.Device
	for rows.Next() {
		var d domain.Device
		if err := rows.Scan(&d.ID,&d.BuildingID,&d.Name,&d.DeviceType,&d.GatewayImei,&d.GatewayType,&d.NodeAddress,&d.DeviceNo,&d.CTRatio,&d.PTRatio,&d.RatedVoltage,&d.RatedCurrent,&d.OnlineStatus,&d.DeviceStatus,&d.LastOnlineAt,&d.LastRecordAt,&d.CreatedAt); err != nil {
			slog.Warn("DeviceRepo.List scan failed", "err", err)
			continue
		}
		list = append(list, d)
	}
	return list, rows.Err()
}
func (r *DeviceRepo) GetByID(ctx context.Context, id int) (*domain.Device, error) {
	var d domain.Device
	err := r.pool.QueryRow(ctx, "SELECT id,building_id,name,device_type,gateway_imei,COALESCE(gateway_type,'custom'),node_address,device_no,ct_ratio,pt_ratio,rated_voltage,rated_current,COALESCE(online_status,'在线'),COALESCE(device_status,'开机'),last_online_at,last_record_at,created_at FROM device WHERE id=$1", id).
		Scan(&d.ID,&d.BuildingID,&d.Name,&d.DeviceType,&d.GatewayImei,&d.GatewayType,&d.NodeAddress,&d.DeviceNo,&d.CTRatio,&d.PTRatio,&d.RatedVoltage,&d.RatedCurrent,&d.OnlineStatus,&d.DeviceStatus,&d.LastOnlineAt,&d.LastRecordAt,&d.CreatedAt)
	if errors.Is(err, sql.ErrNoRows) { return nil, err }
	return &d, err
}
func (r *DeviceRepo) Create(ctx context.Context, d *domain.Device) error {
	return r.pool.QueryRow(ctx, "INSERT INTO device (building_id,name,device_type,gateway_imei,gateway_type,node_address,device_no,ct_ratio,pt_ratio,rated_voltage,rated_current) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11) RETURNING id,created_at",
		d.BuildingID,d.Name,d.DeviceType,d.GatewayImei,d.GatewayType,d.NodeAddress,d.DeviceNo,d.CTRatio,d.PTRatio,d.RatedVoltage,d.RatedCurrent).Scan(&d.ID, &d.CreatedAt)
}
func (r *DeviceRepo) Update(ctx context.Context, d *domain.Device) error {
	_, err := r.pool.Exec(ctx, "UPDATE device SET name=$1,device_type=$2,gateway_imei=$3,gateway_type=$4,node_address=$5,device_no=$6,ct_ratio=$7,pt_ratio=$8,rated_voltage=$9,rated_current=$10 WHERE id=$11",
		d.Name,d.DeviceType,d.GatewayImei,d.GatewayType,d.NodeAddress,d.DeviceNo,d.CTRatio,d.PTRatio,d.RatedVoltage,d.RatedCurrent,d.ID)
	return err
}
func (r *DeviceRepo) Delete(ctx context.Context, id int) error {
	_, err := r.pool.Exec(ctx, "DELETE FROM device WHERE id=$1", id)
	return err
}

// ===== PROPERTY =====
type PropertyRepo struct{ pool DBTX }
func NewPropertyRepo(p DBTX) *PropertyRepo { return &PropertyRepo{pool: p} }

func (r *PropertyRepo) List(ctx context.Context, deviceID int) ([]domain.DeviceProperty, error) {
	rows, err := r.pool.Query(ctx, "SELECT id,device_id,prop_name,COALESCE(prop_short,''),COALESCE(prop_value,''),unit,operation_type,is_key,prop_type,COALESCE(min_value,''),COALESCE(max_value,''),sort_order FROM device_properties WHERE ($1=0 OR device_id=$1) ORDER BY CASE WHEN operation_type='开关机' THEN 0 ELSE 1 END, sort_order", deviceID)
	if err != nil { return nil, err }
	defer rows.Close()
	var list []domain.DeviceProperty
	for rows.Next() {
		var p domain.DeviceProperty
		if err := rows.Scan(&p.ID,&p.DeviceID,&p.PropName,&p.PropShort,&p.PropValue,&p.Unit,&p.OperationType,&p.IsKey,&p.PropType,&p.MinValue,&p.MaxValue,&p.SortOrder); err != nil {
			slog.Warn("PropertyRepo.List scan failed", "err", err)
			continue
		}
		list = append(list, p)
	}
	return list, rows.Err()
}
func (r *PropertyRepo) GetByID(ctx context.Context, id int) (*domain.DeviceProperty, error) {
	var p domain.DeviceProperty
	err := r.pool.QueryRow(ctx, "SELECT id,device_id,prop_name,COALESCE(prop_short,''),COALESCE(prop_value,''),unit,operation_type,is_key,prop_type,COALESCE(min_value,''),COALESCE(max_value,''),sort_order FROM device_properties WHERE id=$1", id).
		Scan(&p.ID,&p.DeviceID,&p.PropName,&p.PropShort,&p.PropValue,&p.Unit,&p.OperationType,&p.IsKey,&p.PropType,&p.MinValue,&p.MaxValue,&p.SortOrder)
	if errors.Is(err, sql.ErrNoRows) { return nil, err }
	return &p, err
}
func (r *PropertyRepo) Create(ctx context.Context, p *domain.DeviceProperty) error {
	return r.pool.QueryRow(ctx, "INSERT INTO device_properties (device_id,prop_name,prop_short,prop_value,unit,operation_type,is_key,prop_type,min_value,max_value,sort_order) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11) RETURNING id",
		p.DeviceID,p.PropName,p.PropShort,p.PropValue,p.Unit,p.OperationType,p.IsKey,p.PropType,p.MinValue,p.MaxValue,p.SortOrder).Scan(&p.ID)
}
func (r *PropertyRepo) Update(ctx context.Context, p *domain.DeviceProperty) error {
	_, err := r.pool.Exec(ctx, "UPDATE device_properties SET prop_name=$1,prop_short=$2,prop_value=$3,unit=$4,operation_type=$5,is_key=$6,prop_type=$7,min_value=$8,max_value=$9,sort_order=$10 WHERE id=$11",
		p.PropName,p.PropShort,p.PropValue,p.Unit,p.OperationType,p.IsKey,p.PropType,p.MinValue,p.MaxValue,p.SortOrder,p.ID)
	return err
}
func (r *PropertyRepo) Delete(ctx context.Context, id int) error {
	_, err := r.pool.Exec(ctx, "DELETE FROM device_properties WHERE id=$1", id)
	return err
}

// ===== REGISTER =====
type RegisterRepo struct{ pool DBTX }
func NewRegisterRepo(p DBTX) *RegisterRepo { return &RegisterRepo{pool: p} }

func (r *RegisterRepo) List(ctx context.Context, propertyID int) ([]domain.Register, error) {
	rows, err := r.pool.Query(ctx, "SELECT id,property_id,COALESCE(name,''),read_addr,read_code,write_addr,write_code,COALESCE(command_name,''),COALESCE(command_code,''),COALESCE(status_code,''),data_type,data_length,data_order,data_mask,magnification FROM register WHERE ($1=0 OR property_id=$1) ORDER BY id", propertyID)
	if err != nil { return nil, err }
	defer rows.Close()
	var list []domain.Register
	for rows.Next() {
		var reg domain.Register
		if err := rows.Scan(&reg.ID,&reg.PropertyID,&reg.Name,&reg.ReadAddr,&reg.ReadCode,&reg.WriteAddr,&reg.WriteCode,&reg.CommandName,&reg.CommandCode,&reg.StatusCode,&reg.DataType,&reg.DataLength,&reg.DataOrder,&reg.DataMask,&reg.Magnification); err != nil {
			slog.Warn("RegisterRepo.List scan failed", "err", err)
			continue
		}
		list = append(list, reg)
	}
	return list, rows.Err()
}
func (r *RegisterRepo) ListByDeviceID(ctx context.Context, deviceID int) ([]domain.Register, error) {
	rows, err := r.pool.Query(ctx, "SELECT r.id,r.property_id,COALESCE(r.name,''),r.read_addr,r.read_code,r.write_addr,r.write_code,COALESCE(r.command_name,''),COALESCE(r.command_code,''),COALESCE(r.status_code,''),r.data_type,r.data_length,r.data_order,r.data_mask,r.magnification FROM register r JOIN device_properties dp ON dp.id=r.property_id WHERE dp.device_id=$1 ORDER BY r.id", deviceID)
	if err != nil { return nil, err }
	defer rows.Close()
	var list []domain.Register
	for rows.Next() {
		var reg domain.Register
		if err := rows.Scan(&reg.ID,&reg.PropertyID,&reg.Name,&reg.ReadAddr,&reg.ReadCode,&reg.WriteAddr,&reg.WriteCode,&reg.CommandName,&reg.CommandCode,&reg.StatusCode,&reg.DataType,&reg.DataLength,&reg.DataOrder,&reg.DataMask,&reg.Magnification); err != nil {
			slog.Warn("RegisterRepo.ListByDeviceID scan failed", "err", err)
			continue
		}
		list = append(list, reg)
	}
	return list, rows.Err()
}
func (r *RegisterRepo) GetByID(ctx context.Context, id int) (*domain.Register, error) {
	var reg domain.Register
	err := r.pool.QueryRow(ctx, "SELECT id,property_id,COALESCE(name,''),read_addr,read_code,write_addr,write_code,COALESCE(command_name,''),COALESCE(command_code,''),COALESCE(status_code,''),data_type,data_length,data_order,data_mask,magnification FROM register WHERE id=$1", id).
		Scan(&reg.ID,&reg.PropertyID,&reg.Name,&reg.ReadAddr,&reg.ReadCode,&reg.WriteAddr,&reg.WriteCode,&reg.CommandName,&reg.CommandCode,&reg.StatusCode,&reg.DataType,&reg.DataLength,&reg.DataOrder,&reg.DataMask,&reg.Magnification)
	if errors.Is(err, sql.ErrNoRows) { return nil, err }
	return &reg, err
}
func (r *RegisterRepo) Create(ctx context.Context, reg *domain.Register) error {
	return r.pool.QueryRow(ctx, "INSERT INTO register (property_id,name,read_addr,read_code,write_addr,write_code,command_name,command_code,status_code,data_type,data_length,data_order,data_mask,magnification) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14) RETURNING id",
		reg.PropertyID,reg.Name,reg.ReadAddr,reg.ReadCode,reg.WriteAddr,reg.WriteCode,reg.CommandName,reg.CommandCode,reg.StatusCode,reg.DataType,reg.DataLength,reg.DataOrder,reg.DataMask,reg.Magnification).Scan(&reg.ID)
}
func (r *RegisterRepo) Update(ctx context.Context, reg *domain.Register) error {
	_, err := r.pool.Exec(ctx, "UPDATE register SET name=$1,read_addr=$2,read_code=$3,write_addr=$4,write_code=$5,command_name=$6,command_code=$7,status_code=$8,data_type=$9,data_length=$10,data_order=$11,data_mask=$12,magnification=$13 WHERE id=$14",
		reg.Name,reg.ReadAddr,reg.ReadCode,reg.WriteAddr,reg.WriteCode,reg.CommandName,reg.CommandCode,reg.StatusCode,reg.DataType,reg.DataLength,reg.DataOrder,reg.DataMask,reg.Magnification,reg.ID)
	return err
}
func (r *RegisterRepo) Delete(ctx context.Context, id int) error {
	_, err := r.pool.Exec(ctx, "DELETE FROM register WHERE id=$1", id)
	return err
}