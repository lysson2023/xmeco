package handler

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/pashagolub/pgxmock/v4"

	"xmeco/internal/repository/postgres"
)

func nullTime() time.Time { return time.Time{} }

// =============================================================================
// BuildingHandler CRUD
// =============================================================================

func buildingCols() []string {
	return []string{"id", "project_id", "name", "outdoor_temp", "outdoor_humidity",
		"total_energy", "save_rate", "save_energy", "carbon_rate", "carbon_saving", "created_at"}
}

func TestBuildingHandler_Get(t *testing.T) {
	tests := []struct {
		name       string
		id         int
		mockSetup  func(mock pgxmock.PgxPoolIface)
		wantStatus int
		wantBody   string
	}{
		{
			name: "Get 正常返回楼宇",
			id:   1,
			mockSetup: func(mock pgxmock.PgxPoolIface) {
				mock.ExpectQuery("SELECT .+ FROM building WHERE").
					WithArgs(1).
					WillReturnRows(pgxmock.NewRows(buildingCols()).
						AddRow(1, 1, "A栋", nil, nil, 0.0, 0.0, 0.0, 0.0, 0.0, nullTime()))
			},
			wantStatus: http.StatusOK,
			wantBody:   "A栋",
		},
		{
			name: "Get 楼宇不存在→404",
			id:   999,
			mockSetup: func(mock pgxmock.PgxPoolIface) {
				mock.ExpectQuery("SELECT .+ FROM building WHERE").
					WithArgs(999).
					WillReturnRows(pgxmock.NewRows(buildingCols()))
			},
			wantStatus: http.StatusNotFound,
			wantBody:   "楼宇不存在",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock, err := pgxmock.NewPool()
			if err != nil {
				t.Fatal(err)
			}
			defer mock.Close()
			tt.mockSetup(mock)

			h := NewBuildingHandler(postgres.NewBuildingRepo(mock))
			req := httptest.NewRequest("GET", "/api/v1/buildings/"+itos(tt.id), nil)
			req.SetPathValue("id", itos(tt.id))
			rec := httptest.NewRecorder()
			h.Get(rec, req)

			if rec.Code != tt.wantStatus {
				t.Errorf("status = %d, want %d", rec.Code, tt.wantStatus)
			}
			if tt.wantBody != "" && !strings.Contains(rec.Body.String(), tt.wantBody) {
				t.Errorf("body = %q, want to contain %q", rec.Body.String(), tt.wantBody)
			}
			if err := mock.ExpectationsWereMet(); err != nil {
				t.Errorf("unmet expectations: %v", err)
			}
		})
	}
}

func TestBuildingHandler_List(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	mock.ExpectQuery("SELECT .+ FROM building").
		WithArgs(0).
		WillReturnRows(pgxmock.NewRows(buildingCols()).
			AddRow(1, 1, "A栋", nil, nil, 0.0, 0.0, 0.0, 0.0, 0.0, nullTime()).
			AddRow(2, 1, "B栋", nil, nil, 0.0, 0.0, 0.0, 0.0, 0.0, nullTime()))

	h := NewBuildingHandler(postgres.NewBuildingRepo(mock))
	req := httptest.NewRequest("GET", "/api/v1/buildings", nil)
	rec := httptest.NewRecorder()
	h.List(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "A栋") {
		t.Errorf("body missing expected content: %s", rec.Body.String())
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet expectations: %v", err)
	}
}

func TestBuildingHandler_Create(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	mock.ExpectQuery("INSERT INTO building").
		WithArgs(pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(),
			pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg()).
		WillReturnRows(pgxmock.NewRows([]string{"id", "created_at"}).AddRow(1, nullTime()))

	h := NewBuildingHandler(postgres.NewBuildingRepo(mock))
	body := `{"project_id":1,"name":"新建楼宇"}`
	req := httptest.NewRequest("POST", "/api/v1/buildings", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	h.Create(rec, req)

	if rec.Code != http.StatusCreated {
		t.Errorf("status = %d, want 201", rec.Code)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet expectations: %v", err)
	}
}

func TestBuildingHandler_Update(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	mock.ExpectExec("UPDATE building SET").
		WithArgs(pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(),
			pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg()).
		WillReturnResult(pgxmock.NewResult("UPDATE", 1))

	h := NewBuildingHandler(postgres.NewBuildingRepo(mock))
	body := `{"name":"更新楼宇"}`
	req := httptest.NewRequest("PUT", "/api/v1/buildings/1", strings.NewReader(body))
	req.SetPathValue("id", "1")
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	h.Update(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", rec.Code)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet expectations: %v", err)
	}
}

func TestBuildingHandler_Delete(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	mock.ExpectExec("DELETE FROM building WHERE").
		WithArgs(1).
		WillReturnResult(pgxmock.NewResult("DELETE", 1))

	h := NewBuildingHandler(postgres.NewBuildingRepo(mock))
	req := httptest.NewRequest("DELETE", "/api/v1/buildings/1", nil)
	req.SetPathValue("id", "1")
	rec := httptest.NewRecorder()
	h.Delete(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", rec.Code)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet expectations: %v", err)
	}
}

// =============================================================================
// PropertyHandler CRUD
// =============================================================================

func propertyCols() []string {
	return []string{"id", "device_id", "prop_name", "prop_short", "prop_value",
		"unit", "operation_type", "is_key", "prop_type", "min_value", "max_value", "sort_order"}
}

func TestPropertyHandler_Get(t *testing.T) {
	tests := []struct {
		name       string
		id         int
		mockSetup  func(mock pgxmock.PgxPoolIface)
		wantStatus int
		wantBody   string
	}{
		{
			name: "Get 正常返回属性",
			id:   1,
			mockSetup: func(mock pgxmock.PgxPoolIface) {
				mock.ExpectQuery("SELECT .+ FROM device_properties WHERE").
					WithArgs(1).
					WillReturnRows(pgxmock.NewRows(propertyCols()).
						AddRow(1, 1, "温度", "temp", "25.0", "℃", "监测", true, "float", "0", "100", 1))
			},
			wantStatus: http.StatusOK,
			wantBody:   "温度",
		},
		{
			name: "Get 属性不存在→404",
			id:   999,
			mockSetup: func(mock pgxmock.PgxPoolIface) {
				mock.ExpectQuery("SELECT .+ FROM device_properties WHERE").
					WithArgs(999).
					WillReturnRows(pgxmock.NewRows(propertyCols()))
			},
			wantStatus: http.StatusNotFound,
			wantBody:   "属性不存在",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock, err := pgxmock.NewPool()
			if err != nil {
				t.Fatal(err)
			}
			defer mock.Close()
			tt.mockSetup(mock)

			h := NewPropertyHandler(postgres.NewPropertyRepo(mock))
			req := httptest.NewRequest("GET", "/api/v1/properties/"+itos(tt.id), nil)
			req.SetPathValue("id", itos(tt.id))
			rec := httptest.NewRecorder()
			h.Get(rec, req)

			if rec.Code != tt.wantStatus {
				t.Errorf("status = %d, want %d", rec.Code, tt.wantStatus)
			}
			if tt.wantBody != "" && !strings.Contains(rec.Body.String(), tt.wantBody) {
				t.Errorf("body = %q, want to contain %q", rec.Body.String(), tt.wantBody)
			}
			if err := mock.ExpectationsWereMet(); err != nil {
				t.Errorf("unmet expectations: %v", err)
			}
		})
	}
}

func TestPropertyHandler_List(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	mock.ExpectQuery("SELECT .+ FROM device_properties").
		WithArgs(0).
		WillReturnRows(pgxmock.NewRows(propertyCols()).
			AddRow(1, 1, "温度", "temp", "25.0", "℃", "监测", true, "float", "0", "100", 1).
			AddRow(2, 1, "湿度", "hum", "60.0", "%", "监测", false, "float", "0", "100", 2))

	h := NewPropertyHandler(postgres.NewPropertyRepo(mock))
	req := httptest.NewRequest("GET", "/api/v1/properties", nil)
	rec := httptest.NewRecorder()
	h.List(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "温度") {
		t.Errorf("body missing expected content: %s", rec.Body.String())
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet expectations: %v", err)
	}
}

func TestPropertyHandler_Create(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	mock.ExpectQuery("INSERT INTO device_properties").
		WithArgs(pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(),
			pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(),
			pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg()).
		WillReturnRows(pgxmock.NewRows([]string{"id"}).AddRow(1))

	h := NewPropertyHandler(postgres.NewPropertyRepo(mock))
	body := `{"device_id":1,"prop_name":"电压","unit":"V","prop_type":"float"}`
	req := httptest.NewRequest("POST", "/api/v1/properties", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	h.Create(rec, req)

	if rec.Code != http.StatusCreated {
		t.Errorf("status = %d, want 201", rec.Code)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet expectations: %v", err)
	}
}

func TestPropertyHandler_Update(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	mock.ExpectExec("UPDATE device_properties SET").
		WithArgs(pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(),
			pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(),
			pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg()).
		WillReturnResult(pgxmock.NewResult("UPDATE", 1))

	h := NewPropertyHandler(postgres.NewPropertyRepo(mock))
	body := `{"prop_name":"更新属性","unit":"V"}`
	req := httptest.NewRequest("PUT", "/api/v1/properties/1", strings.NewReader(body))
	req.SetPathValue("id", "1")
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	h.Update(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", rec.Code)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet expectations: %v", err)
	}
}

func TestPropertyHandler_Delete(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	mock.ExpectExec("DELETE FROM device_properties WHERE").
		WithArgs(1).
		WillReturnResult(pgxmock.NewResult("DELETE", 1))

	h := NewPropertyHandler(postgres.NewPropertyRepo(mock))
	req := httptest.NewRequest("DELETE", "/api/v1/properties/1", nil)
	req.SetPathValue("id", "1")
	rec := httptest.NewRecorder()
	h.Delete(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", rec.Code)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet expectations: %v", err)
	}
}

// =============================================================================
// RegisterHandler CRUD
// =============================================================================

func registerCols() []string {
	return []string{"id", "property_id", "name", "read_addr", "read_code",
		"write_addr", "write_code", "command_name", "command_code", "status_code",
		"data_type", "data_length", "data_order", "data_mask", "magnification"}
}

func TestRegisterHandler_Get(t *testing.T) {
	tests := []struct {
		name       string
		id         int
		mockSetup  func(mock pgxmock.PgxPoolIface)
		wantStatus int
		wantBody   string
	}{
		{
			name: "Get 正常返回寄存器",
			id:   1,
			mockSetup: func(mock pgxmock.PgxPoolIface) {
				mock.ExpectQuery("SELECT .+ FROM register WHERE").
					WithArgs(1).
					WillReturnRows(pgxmock.NewRows(registerCols()).
						AddRow(1, 1, "电流A", 100, "03", nil, nil, nil, "", nil, "u16", 2, "", nil, 1.0))
			},
			wantStatus: http.StatusOK,
			wantBody:   "电流A",
		},
		{
			name: "Get 寄存器不存在→404",
			id:   999,
			mockSetup: func(mock pgxmock.PgxPoolIface) {
				mock.ExpectQuery("SELECT .+ FROM register WHERE").
					WithArgs(999).
					WillReturnRows(pgxmock.NewRows(registerCols()))
			},
			wantStatus: http.StatusNotFound,
			wantBody:   "寄存器不存在",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock, err := pgxmock.NewPool()
			if err != nil {
				t.Fatal(err)
			}
			defer mock.Close()
			tt.mockSetup(mock)

			h := NewRegisterHandler(postgres.NewRegisterRepo(mock))
			req := httptest.NewRequest("GET", "/api/v1/registers/"+itos(tt.id), nil)
			req.SetPathValue("id", itos(tt.id))
			rec := httptest.NewRecorder()
			h.Get(rec, req)

			if rec.Code != tt.wantStatus {
				t.Errorf("status = %d, want %d", rec.Code, tt.wantStatus)
			}
			if tt.wantBody != "" && !strings.Contains(rec.Body.String(), tt.wantBody) {
				t.Errorf("body = %q, want to contain %q", rec.Body.String(), tt.wantBody)
			}
			if err := mock.ExpectationsWereMet(); err != nil {
				t.Errorf("unmet expectations: %v", err)
			}
		})
	}
}

func TestRegisterHandler_List(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	mock.ExpectQuery("SELECT .+ FROM register").
		WithArgs(0).
		WillReturnRows(pgxmock.NewRows(registerCols()).
			AddRow(1, 1, "电流A", 100, "03", nil, nil, nil, "", nil, "u16", 2, "", nil, 1.0).
			AddRow(2, 1, "电压V", 102, "03", nil, nil, nil, "", nil, "u16", 2, "", nil, 0.1))

	h := NewRegisterHandler(postgres.NewRegisterRepo(mock))
	req := httptest.NewRequest("GET", "/api/v1/registers", nil)
	rec := httptest.NewRecorder()
	h.List(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "电流A") {
		t.Errorf("body missing expected content: %s", rec.Body.String())
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet expectations: %v", err)
	}
}

func TestRegisterHandler_Create(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	mock.ExpectQuery("INSERT INTO register").
		WithArgs(pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(),
			pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(),
			pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(),
			pgxmock.AnyArg(), pgxmock.AnyArg()).
		WillReturnRows(pgxmock.NewRows([]string{"id"}).AddRow(1))

	h := NewRegisterHandler(postgres.NewRegisterRepo(mock))
	body := `{"property_id":1,"name":"功率","read_addr":104,"read_code":"03","data_type":"u16","data_length":2,"magnification":1}`
	req := httptest.NewRequest("POST", "/api/v1/registers", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	h.Create(rec, req)

	if rec.Code != http.StatusCreated {
		t.Errorf("status = %d, want 201", rec.Code)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet expectations: %v", err)
	}
}

func TestRegisterHandler_Update(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	mock.ExpectExec("UPDATE register SET").
		WithArgs(pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(),
			pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(),
			pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(),
			pgxmock.AnyArg(), pgxmock.AnyArg()).
		WillReturnResult(pgxmock.NewResult("UPDATE", 1))

	h := NewRegisterHandler(postgres.NewRegisterRepo(mock))
	body := `{"name":"更新寄存器","magnification":10}`
	req := httptest.NewRequest("PUT", "/api/v1/registers/1", strings.NewReader(body))
	req.SetPathValue("id", "1")
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	h.Update(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", rec.Code)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet expectations: %v", err)
	}
}

func TestRegisterHandler_Delete(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	mock.ExpectExec("DELETE FROM register WHERE").
		WithArgs(1).
		WillReturnResult(pgxmock.NewResult("DELETE", 1))

	h := NewRegisterHandler(postgres.NewRegisterRepo(mock))
	req := httptest.NewRequest("DELETE", "/api/v1/registers/1", nil)
	req.SetPathValue("id", "1")
	rec := httptest.NewRecorder()
	h.Delete(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", rec.Code)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet expectations: %v", err)
	}
}

// =============================================================================
// DeviceHandler CRUD
// =============================================================================

func deviceCols() []string {
	return []string{"id", "building_id", "name", "device_type", "gateway_imei",
		"gateway_type", "node_address", "device_no", "ct_ratio", "pt_ratio",
		"rated_voltage", "rated_current", "power_sign", "online_status", "device_status",
		"last_online_at", "last_record_at", "created_at"}
}

func TestDeviceHandler_Get(t *testing.T) {
	tests := []struct {
		name       string
		id         int
		mockSetup  func(mock pgxmock.PgxPoolIface)
		wantStatus int
		wantBody   string
	}{
		{
			name: "Get 正常返回设备",
			id:   1,
			mockSetup: func(mock pgxmock.PgxPoolIface) {
				mock.ExpectQuery("SELECT .+ FROM device WHERE").
					WithArgs(1).
					WillReturnRows(pgxmock.NewRows(deviceCols()).
						AddRow(1, 1, "冷水机组1", "chiller", nil, "custom", 0, 1, 100, 100,
							nil, nil, 1, "在线", "开机", nil, nil, nullTime()))
			},
			wantStatus: http.StatusOK,
			wantBody:   "冷水机组1",
		},
		{
			name: "Get 设备不存在→404",
			id:   999,
			mockSetup: func(mock pgxmock.PgxPoolIface) {
				mock.ExpectQuery("SELECT .+ FROM device WHERE").
					WithArgs(999).
					WillReturnRows(pgxmock.NewRows(deviceCols()))
			},
			wantStatus: http.StatusNotFound,
			wantBody:   "设备不存在",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock, err := pgxmock.NewPool()
			if err != nil {
				t.Fatal(err)
			}
			defer mock.Close()
			tt.mockSetup(mock)

			h := NewDeviceHandler(postgres.NewDeviceRepo(mock), nil)
			req := httptest.NewRequest("GET", "/api/v1/devices/"+itos(tt.id), nil)
			req.SetPathValue("id", itos(tt.id))
			rec := httptest.NewRecorder()
			h.Get(rec, req)

			if rec.Code != tt.wantStatus {
				t.Errorf("status = %d, want %d", rec.Code, tt.wantStatus)
			}
			if tt.wantBody != "" && !strings.Contains(rec.Body.String(), tt.wantBody) {
				t.Errorf("body = %q, want to contain %q", rec.Body.String(), tt.wantBody)
			}
			if err := mock.ExpectationsWereMet(); err != nil {
				t.Errorf("unmet expectations: %v", err)
			}
		})
	}
}

func TestDeviceHandler_List(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	mock.ExpectQuery("SELECT .+ FROM device").
		WithArgs(0).
		WillReturnRows(pgxmock.NewRows(deviceCols()).
			AddRow(1, 1, "冷水机组1", "chiller", nil, "custom", 0, 1, 100, 100,
				nil, nil, 1, "在线", "开机", nil, nil, nullTime()).
			AddRow(2, 1, "冷却泵1", "cooling_pump", nil, "custom", 0, 2, 50, 50,
				nil, nil, 1, "在线", "开机", nil, nil, nullTime()))

	h := NewDeviceHandler(postgres.NewDeviceRepo(mock), nil)
	req := httptest.NewRequest("GET", "/api/v1/devices", nil)
	rec := httptest.NewRecorder()
	h.List(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "冷水机组1") {
		t.Errorf("body missing expected content: %s", rec.Body.String())
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet expectations: %v", err)
	}
}

func TestDeviceHandler_Create(t *testing.T) {
	tests := []struct {
		name       string
		body       string
		mockSetup  func(mock pgxmock.PgxPoolIface)
		wantStatus int
	}{
		{
			name: "Create 正常创建设备",
			body: `{"building_id":1,"name":"新设备","device_type":"chiller"}`,
			mockSetup: func(mock pgxmock.PgxPoolIface) {
				mock.ExpectQuery("INSERT INTO device").
					WithArgs(pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(),
						pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(),
						pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg()).
					WillReturnRows(pgxmock.NewRows([]string{"id", "created_at"}).AddRow(3, nullTime()))
			},
			wantStatus: http.StatusCreated,
		},
		{
			name:       "Create 缺少device_type→400",
			body:       `{"building_id":1,"name":"无效设备"}`,
			mockSetup:  nil,
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "Create 非法JSON→400",
			body:       `{invalid`,
			mockSetup:  nil,
			wantStatus: http.StatusBadRequest,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var h *DeviceHandler
			var mock pgxmock.PgxPoolIface

			if tt.mockSetup != nil {
				var err error
				mock, err = pgxmock.NewPool()
				if err != nil {
					t.Fatal(err)
				}
				defer mock.Close()
				tt.mockSetup(mock)
				h = NewDeviceHandler(postgres.NewDeviceRepo(mock), nil)
			} else {
				h = NewDeviceHandler(nil, nil)
			}

			req := httptest.NewRequest("POST", "/api/v1/devices", strings.NewReader(tt.body))
			req.Header.Set("Content-Type", "application/json")
			rec := httptest.NewRecorder()

			if tt.mockSetup == nil {
				func() {
					defer func() { recover() }()
					h.Create(rec, req)
				}()
			} else {
				h.Create(rec, req)
			}

			if rec.Code != tt.wantStatus {
				t.Errorf("status = %d, want %d", rec.Code, tt.wantStatus)
			}
			if mock != nil {
				if err := mock.ExpectationsWereMet(); err != nil {
					t.Errorf("unmet expectations: %v", err)
				}
			}
		})
	}
}

func TestDeviceHandler_Update(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	mock.ExpectExec("UPDATE device SET").
		WithArgs(pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(),
			pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(),
			pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg()).
		WillReturnResult(pgxmock.NewResult("UPDATE", 1))

	h := NewDeviceHandler(postgres.NewDeviceRepo(mock), nil)
	body := `{"name":"更新设备","device_type":"pump"}`
	req := httptest.NewRequest("PUT", "/api/v1/devices/1", strings.NewReader(body))
	req.SetPathValue("id", "1")
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	h.Update(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", rec.Code)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet expectations: %v", err)
	}
}

func TestDeviceHandler_Delete(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	mock.ExpectExec("DELETE FROM device WHERE").
		WithArgs(1).
		WillReturnResult(pgxmock.NewResult("DELETE", 1))

	h := NewDeviceHandler(postgres.NewDeviceRepo(mock), nil)
	req := httptest.NewRequest("DELETE", "/api/v1/devices/1", nil)
	req.SetPathValue("id", "1")
	rec := httptest.NewRecorder()
	h.Delete(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", rec.Code)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet expectations: %v", err)
	}
}
