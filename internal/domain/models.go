package domain

import "time"

type Building struct {
	ID              int       `json:"id"`
	ProjectID       int       `json:"project_id"`
	Name            string    `json:"name"`
	OutdoorTemp     *float64  `json:"outdoor_temp"`
	OutdoorHumidity *float64  `json:"outdoor_humidity"`
	TotalEnergy     float64   `json:"total_energy"`
	SaveRate        float64   `json:"save_rate"`
	SaveEnergy      float64   `json:"save_energy"`
	CarbonRate      float64   `json:"carbon_rate"`
	CarbonSaving    float64   `json:"carbon_saving"`
	CreatedAt       time.Time `json:"created_at"`
}

type Device struct {
	ID            int        `json:"id"`
	BuildingID    int        `json:"building_id"`
	Name          string     `json:"name"`
	DeviceType    string     `json:"device_type"`
	GatewayImei   *string    `json:"gateway_imei"`
	GatewayType   string     `json:"gateway_type"`
	NodeAddress   int        `json:"node_address"`
	DeviceNo      int        `json:"device_no"`
	CTRatio       int        `json:"ct_ratio"`
	PTRatio       int        `json:"pt_ratio"`
	RatedVoltage  *float64   `json:"rated_voltage"`
	RatedCurrent  *float64   `json:"rated_current"`
	OnlineStatus  string     `json:"online_status"`
	DeviceStatus  string     `json:"device_status"`
	LastOnlineAt  *time.Time `json:"last_online_at"`
	LastRecordAt  *time.Time `json:"last_record_at"`
	CreatedAt     time.Time  `json:"created_at"`
}

type DeviceProperty struct {
	ID            int    `json:"id"`
	DeviceID      int    `json:"device_id"`
	PropName      string `json:"prop_name"`
	PropShort     string `json:"prop_short"`
	PropValue     string `json:"prop_value"`
	Unit          string `json:"unit"`
	OperationType string `json:"operation_type"`
	IsKey         bool   `json:"is_key"`
	PropType      string `json:"prop_type"`
	MinValue      string `json:"min_value"`
	MaxValue      string `json:"max_value"`
	SortOrder     int    `json:"sort_order"`
}

type Register struct {
	ID            int     `json:"id"`
	PropertyID    int     `json:"property_id"`
	Name          string  `json:"name"`
	ReadAddr      int     `json:"read_addr"`
	ReadCode      string  `json:"read_code"`
	WriteAddr     *int    `json:"write_addr"`
	WriteCode     *string `json:"write_code"`
	CommandName   *string `json:"command_name"`
	CommandCode   string  `json:"command_code"`
	StatusCode    *string `json:"status_code"`
	DataType      string  `json:"data_type"`
	DataLength    int     `json:"data_length"`
	DataOrder     string  `json:"data_order"`
	DataMask      *string `json:"data_mask"`
	Magnification float64 `json:"magnification"`
}