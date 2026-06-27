package handler

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/pashagolub/pgxmock/v4"

	"xmeco/internal/gateway"
	"xmeco/internal/gateway/transport"
)

// =============================================================================
// Tier 4 — H-63~H-68: DeviceHandler Control & dispatchHardware
// =============================================================================

func TestDeviceHandler_Control(t *testing.T) {
	tests := []struct {
		name       string
		pathID     string
		body       string
		mockSetup  func(mock pgxmock.PgxPoolIface)
		wantStatus int
		wantBody   string
	}{
		{
			name:   "H-63 Control 正常写入控制记录",
			pathID: "5",
			body:   `{"action":"start"}`,
			mockSetup: func(mock pgxmock.PgxPoolIface) {
				// device/building/project name lookup
				mock.ExpectQuery("SELECT d\\.name, b\\.name, COALESCE\\(p\\.name").
					WithArgs(5).
					WillReturnRows(pgxmock.NewRows([]string{"name", "bname", "pname"}).
						AddRow("冷水机组1", "A栋", "项目一"))
				// INSERT control_record
				mock.ExpectExec("INSERT INTO control_record").
					WithArgs("项目一", "A栋", "冷水机组1", 5, "开机", "unknown", "start").
					WillReturnResult(pgxmock.NewResult("INSERT", 1))
				// UPDATE device_status
				mock.ExpectExec("UPDATE device SET device_status").
					WithArgs("开机", 5).
					WillReturnResult(pgxmock.NewResult("UPDATE", 1))
			},
			wantStatus: http.StatusOK,
			wantBody:   `"status":"ok"`,
		},
		{
			name:   "Control stop动作",
			pathID: "3",
			body:   `{"action":"stop"}`,
			mockSetup: func(mock pgxmock.PgxPoolIface) {
				mock.ExpectQuery("SELECT d\\.name, b\\.name, COALESCE\\(p\\.name").
					WithArgs(3).
					WillReturnRows(pgxmock.NewRows([]string{"name", "bname", "pname"}).
						AddRow("冷却泵1", "B栋", "项目二"))
				mock.ExpectExec("INSERT INTO control_record").
					WithArgs("项目二", "B栋", "冷却泵1", 3, "关机", "unknown", "stop").
					WillReturnResult(pgxmock.NewResult("INSERT", 1))
				mock.ExpectExec("UPDATE device SET device_status").
					WithArgs("关机", 3).
					WillReturnResult(pgxmock.NewResult("UPDATE", 1))
			},
			wantStatus: http.StatusOK,
			wantBody:   `"action":"stop"`,
		},
		{
			name:       "Control 缺少action→400",
			pathID:     "1",
			body:       `{"action":""}`,
			mockSetup:  nil,
			wantStatus: http.StatusBadRequest,
			wantBody:   "action 不能为空",
		},
		{
			name:       "Control 无效device_id→400",
			pathID:     "0",
			body:       `{"action":"start"}`,
			mockSetup:  nil,
			wantStatus: http.StatusBadRequest,
			wantBody:   "无效的设备",
		},
		{
			name:       "Control 非法JSON→400",
			pathID:     "1",
			body:       `{invalid`,
			mockSetup:  nil,
			wantStatus: http.StatusBadRequest,
			wantBody:   "action 不能为空",
		},
		{
			name:   "H-64 Control 无gwMgr跳过硬件分发",
			pathID: "5",
			body:   `{"action":"start"}`,
			mockSetup: func(mock pgxmock.PgxPoolIface) {
				mock.ExpectQuery("SELECT d\\.name, b\\.name, COALESCE\\(p\\.name").
					WithArgs(5).
					WillReturnRows(pgxmock.NewRows([]string{"name", "bname", "pname"}).
						AddRow("设备A", "楼宇A", "项目A"))
				mock.ExpectExec("INSERT INTO control_record").
					WithArgs("项目A", "楼宇A", "设备A", 5, "开机", "unknown", "start").
					WillReturnResult(pgxmock.NewResult("INSERT", 1))
				mock.ExpectExec("UPDATE device SET device_status").
					WithArgs("开机", 5).
					WillReturnResult(pgxmock.NewResult("UPDATE", 1))
			},
			wantStatus: http.StatusOK,
			wantBody:   `"dispatched":false`,
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
				h = NewDeviceHandler(nil, mock)
				// No gwMgr set → dispatchHardware returns "gateway manager not available"
			} else {
				h = NewDeviceHandler(nil, nil)
			}

			req := httptest.NewRequest("POST", "/api/v1/devices/"+tt.pathID+"/control", strings.NewReader(tt.body))
			req.SetPathValue("id", tt.pathID)
			req.Header.Set("Content-Type", "application/json")
			rec := httptest.NewRecorder()

			if tt.mockSetup == nil {
				func() { defer func() { recover() }(); h.Control(rec, req) }()
			} else {
				h.Control(rec, req)
			}

			if rec.Code != tt.wantStatus {
				t.Errorf("status = %d, want %d", rec.Code, tt.wantStatus)
			}
			if tt.wantBody != "" && !strings.Contains(rec.Body.String(), tt.wantBody) {
				t.Errorf("body = %q, want to contain %q", rec.Body.String(), tt.wantBody)
			}
			if mock != nil {
				if err := mock.ExpectationsWereMet(); err != nil {
					t.Errorf("unmet expectations: %v", err)
				}
			}
		})
	}
}

func TestDispatchHardware_NoGateway(t *testing.T) {
	// H-64 / H-65: dispatchHardware with nil gwMgr
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	// gwMgr is nil → returns immediately without DB query
	h := NewDeviceHandler(nil, mock)
	dr := h.dispatchHardware(context.Background(), 1, "start")

	if dr.Dispatched {
		t.Error("should not dispatch when gwMgr is nil")
	}
	if !strings.Contains(dr.Message, "gateway manager not available") {
		t.Errorf("message = %q, want 'gateway manager not available'", dr.Message)
	}
}

// mockGatewayManager implements GatewayManager for testing dispatchHardware.
type mockGatewayManager struct {
	gw *gateway.Gateway
}

func (m *mockGatewayManager) GetGateway(id string) *gateway.Gateway { return m.gw }

// mockTransport implements transport.Transport for testing.
type mockTransport struct {
	connected bool
	sendResp  []byte
	sendErr   error
}

func (m *mockTransport) Type() transport.GatewayType              { return transport.TypeCustom }
func (m *mockTransport) GatewayID() string                        { return "test-gw" }
func (m *mockTransport) IsConnected() bool                        { return m.connected }
func (m *mockTransport) SendAndReceive(data []byte) ([]byte, error) { return m.sendResp, m.sendErr }
func (m *mockTransport) Close() error                             { return nil }

func TestDispatchHardware_NoGatewayConfig(t *testing.T) {
	// H-65: device not configured for gateway (gwImei empty) — now with non-nil gwMgr
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	mock.ExpectQuery("SELECT COALESCE\\(d\\.gateway_imei").
		WithArgs(1).
		WillReturnRows(pgxmock.NewRows([]string{"gw_imei", "gw_type", "node_addr", "device_no"}).
			AddRow("", "custom", 0, 1))

	h := NewDeviceHandler(nil, mock)
	h.SetGwMgr(&mockGatewayManager{}) // non-nil gwMgr but gwImei is empty
	dr := h.dispatchHardware(context.Background(), 1, "start")

	if dr.Dispatched {
		t.Error("H-65: should not dispatch when gwImei is empty")
	}
	if !strings.Contains(dr.Message, "not configured for gateway") {
		t.Errorf("H-65: message = %q, want 'not configured for gateway'", dr.Message)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet expectations: %v", err)
	}
}

func TestDispatchHardware_GatewayOffline(t *testing.T) {
	// H-66: device has gwImei but gateway Transport is not connected
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	// Device query: valid gwImei
	mock.ExpectQuery("SELECT COALESCE\\(d\\.gateway_imei").
		WithArgs(1).
		WillReturnRows(pgxmock.NewRows([]string{"gw_imei", "gw_type", "node_addr", "device_no"}).
			AddRow("AA:BB:CC:DD:EE:FF", "custom", 0x0102, 1))

	gw := &gateway.Gateway{
		ID:        "AA:BB:CC:DD:EE:FF",
		Transport: &mockTransport{connected: false},
	}
	h := NewDeviceHandler(nil, mock)
	h.SetGwMgr(&mockGatewayManager{gw: gw})
	dr := h.dispatchHardware(context.Background(), 1, "start")

	if dr.Dispatched {
		t.Error("H-66: should not dispatch when gateway is offline")
	}
	if !strings.Contains(dr.Message, "offline") {
		t.Errorf("H-66: message = %q, want 'offline'", dr.Message)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet expectations: %v", err)
	}
}

func TestDispatchHardware_DeviceNoExceeds255(t *testing.T) {
	// H-67: deviceNo > 255 should be rejected (after register config lookup)
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	// Device query: valid gwImei, deviceNo=300 (>255)
	mock.ExpectQuery("SELECT COALESCE\\(d\\.gateway_imei").
		WithArgs(1).
		WillReturnRows(pgxmock.NewRows([]string{"gw_imei", "gw_type", "node_addr", "device_no"}).
			AddRow("AA:BB:CC:DD:EE:FF", "custom", 0x0102, 300))
	// Register config query: valid write config
	mock.ExpectQuery("SELECT r\\.write_addr, r\\.write_code FROM register").
		WithArgs(1, "start").
		WillReturnRows(pgxmock.NewRows([]string{"write_addr", "write_code"}).
			AddRow(100, "06"))

	gw := &gateway.Gateway{
		ID:        "AA:BB:CC:DD:EE:FF",
		Transport: &mockTransport{connected: true},
	}
	h := NewDeviceHandler(nil, mock)
	h.SetGwMgr(&mockGatewayManager{gw: gw})
	dr := h.dispatchHardware(context.Background(), 1, "start")

	if dr.Dispatched {
		t.Error("H-67: should not dispatch when deviceNo > 255")
	}
	if !strings.Contains(dr.Message, "exceeds 255") {
		t.Errorf("H-67: message = %q, want 'exceeds 255'", dr.Message)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet expectations: %v", err)
	}
}

func TestDispatchHardware_Success(t *testing.T) {
	// H-68: full dispatchHardware success path
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	// Device query: valid gwImei, nodeAddr=0x0102, deviceNo=10
	mock.ExpectQuery("SELECT COALESCE\\(d\\.gateway_imei").
		WithArgs(5).
		WillReturnRows(pgxmock.NewRows([]string{"gw_imei", "gw_type", "node_addr", "device_no"}).
			AddRow("AA:BB:CC:DD:EE:FF", "custom", 0x0102, 10))
	// Register config query
	mock.ExpectQuery("SELECT r\\.write_addr, r\\.write_code FROM register").
		WithArgs(5, "start").
		WillReturnRows(pgxmock.NewRows([]string{"write_addr", "write_code"}).
			AddRow(100, "06"))

	gw := &gateway.Gateway{
		ID:        "AA:BB:CC:DD:EE:FF",
		Transport: &mockTransport{connected: true, sendResp: []byte{0x01, 0x02}},
	}
	h := NewDeviceHandler(nil, mock)
	h.SetGwMgr(&mockGatewayManager{gw: gw})
	dr := h.dispatchHardware(context.Background(), 5, "start")

	if !dr.Dispatched {
		t.Error("H-68: should dispatch successfully")
	}
	if dr.Message != "ok" {
		t.Errorf("H-68: message = %q, want 'ok'", dr.Message)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet expectations: %v", err)
	}
}

func TestDispatchHardware_NoRegisterConfig(t *testing.T) {
	// dispatchHardware: register config not found (edge case)
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	// Device query
	mock.ExpectQuery("SELECT COALESCE\\(d\\.gateway_imei").
		WithArgs(1).
		WillReturnRows(pgxmock.NewRows([]string{"gw_imei", "gw_type", "node_addr", "device_no"}).
			AddRow("AA:BB:CC:DD:EE:FF", "custom", 0x0102, 1))
	// Register config query: empty result
	mock.ExpectQuery("SELECT r\\.write_addr, r\\.write_code FROM register").
		WithArgs(1, "start").
		WillReturnError(pgx.ErrNoRows)

	gw := &gateway.Gateway{
		ID:        "AA:BB:CC:DD:EE:FF",
		Transport: &mockTransport{connected: true},
	}
	h := NewDeviceHandler(nil, mock)
	h.SetGwMgr(&mockGatewayManager{gw: gw})
	dr := h.dispatchHardware(context.Background(), 1, "start")

	if dr.Dispatched {
		t.Error("should not dispatch without register config")
	}
	if !strings.Contains(dr.Message, "no register") {
		t.Errorf("message = %q, want 'no register'", dr.Message)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet expectations: %v", err)
	}
}
