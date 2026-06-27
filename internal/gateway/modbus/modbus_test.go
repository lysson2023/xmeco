package modbus

import "testing"

// =============================================================================
// Tier 2 — G-01~G-03: CRC16 计算
// =============================================================================

func TestCRC16(t *testing.T) {
	tests := []struct {
		name string
		data []byte
		want uint16
	}{
		{
			name: "G-01 标准CRC向量",
			data: []byte{0x01, 0x03, 0x00, 0x00, 0x00, 0x01},
			want: 0x0A84,
		},
		{
			name: "G-02 空数据返回初始值0xFFFF",
			data: []byte{},
			want: 0xFFFF,
		},
		{
			name: "G-03 单字节非零",
			data: []byte{0x00},
			want: 0x40BF,
		},
		{
			name: "单字节0xFF",
			data: []byte{0xFF},
			want: 0x00FF,
		},
		{
			name: "多字节",
			data: []byte{0x01, 0x02, 0x03, 0x04},
			want: 0x2BA1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := CRC16(tt.data)
			if got != tt.want {
				t.Errorf("CRC16(%X) = %04X, want %04X", tt.data, got, tt.want)
			}
		})
	}
}

// =============================================================================
// Tier 2 — G-04~G-10: BuildReadCommand / BuildWriteCommand
// =============================================================================

func TestBuildReadCommand(t *testing.T) {
	tests := []struct {
		name      string
		devAddr   byte
		funcCode  byte
		startAddr uint16
		count     uint16
		wantLen   int
		wantCRC   bool
	}{
		{
			name:      "G-04 标准读保持寄存器(03)",
			devAddr:   0x01,
			funcCode:  0x03,
			startAddr: 0x0000,
			count:     0x0001,
			wantLen:   8,
			wantCRC:   true,
		},
		{
			name:      "G-05 多寄存器读取count=10",
			devAddr:   0x01,
			funcCode:  0x03,
			startAddr: 100,
			count:     10,
			wantLen:   8,
			wantCRC:   true,
		},
		{
			name:      "读取输入寄存器(04)",
			devAddr:   0x02,
			funcCode:  0x04,
			startAddr: 0x000A,
			count:     0x0002,
			wantLen:   8,
			wantCRC:   true,
		},
		{
			name:      "读取线圈(01)",
			devAddr:   0x0A,
			funcCode:  0x01,
			startAddr: 0x0000,
			count:     0x0008,
			wantLen:   8,
			wantCRC:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := BuildReadCommand(tt.devAddr, tt.funcCode, tt.startAddr, tt.count)
			if len(cmd) != tt.wantLen {
				t.Fatalf("len = %d, want %d", len(cmd), tt.wantLen)
			}
			// Verify CRC bytes match computed CRC of first 6 bytes
			if tt.wantCRC {
				expectedCRC := CRC16(cmd[:6])
				actualCRC := uint16(cmd[6]) | uint16(cmd[7])<<8
				if actualCRC != expectedCRC {
					t.Errorf("CRC = %04X, want %04X", actualCRC, expectedCRC)
				}
			}
			// Verify fields
			if cmd[0] != tt.devAddr {
				t.Errorf("devAddr = %02X, want %02X", cmd[0], tt.devAddr)
			}
			if cmd[1] != tt.funcCode {
				t.Errorf("funcCode = %02X, want %02X", cmd[1], tt.funcCode)
			}
		})
	}
}

func TestBuildWriteCommand(t *testing.T) {
	tests := []struct {
		name     string
		devAddr  byte
		funcCode byte
		addr     uint16
		count    uint16
		value    uint16
		wantLen  int
		wantFC   byte
	}{
		{
			name:     "G-06 写单个寄存器(06)",
			devAddr:  0x01,
			funcCode: 0x06,
			addr:     0x0000,
			count:    1,
			value:    0xFF00,
			wantLen:  8,
			wantFC:   0x06,
		},
		{
			name:     "G-09 写单个线圈(05)实际输出func=06",
			devAddr:  0x01,
			funcCode: 0x05,
			addr:     0x0000,
			count:    1,
			value:    0xFF00,
			wantLen:  8,
			wantFC:   0x06, // BuildWriteSingleCommand 硬编码 0x06
		},
		{
			name:     "G-07 写多个寄存器(10)",
			devAddr:  0x01,
			funcCode: 0x10,
			addr:     0x0000,
			count:    2,
			value:    0x1234,
			wantLen:  13, // 9 + 2*2
			wantFC:   0x10,
		},
		{
			name:     "G-08 未知功能码默认回退WriteSingle",
			devAddr:  0x01,
			funcCode: 0x99,
			addr:     0x0000,
			count:    1,
			value:    0x0000,
			wantLen:  8,
			wantFC:   0x06,
		},
		{
			name:     "写多个线圈(0F)实际输出func=10",
			devAddr:  0x02,
			funcCode: 0x0F,
			addr:     0x0010,
			count:    1,
			value:    0xFF00,
			wantLen:  11,
			wantFC:   0x10, // BuildWriteMultiCommand 硬编码 0x10
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := BuildWriteCommand(tt.devAddr, tt.funcCode, tt.addr, tt.count, tt.value)
			if len(cmd) != tt.wantLen {
				t.Fatalf("len = %d, want %d", len(cmd), tt.wantLen)
			}
			if cmd[1] != tt.wantFC {
				t.Errorf("funcCode = %02X, want %02X", cmd[1], tt.wantFC)
			}
			// Verify CRC
			dataPart := cmd[:len(cmd)-2]
			expectedCRC := CRC16(dataPart)
			actualCRC := uint16(cmd[len(cmd)-2]) | uint16(cmd[len(cmd)-1])<<8
			if actualCRC != expectedCRC {
				t.Errorf("CRC = %04X, want %04X", actualCRC, expectedCRC)
			}
		})
	}
}

// =============================================================================
// Tier 2 — G-10: BuildWriteMultiCommand 直接调用
// =============================================================================

func TestBuildWriteMultiCommand(t *testing.T) {
	tests := []struct {
		name    string
		devAddr byte
		addr    uint16
		count   uint16
		data    []byte
		wantLen int
	}{
		{
			name:    "G-10 写多个寄存器含数据",
			devAddr: 0x01,
			addr:    0x0000,
			count:   2,
			data:    []byte{0x00, 0x01, 0x00, 0x02},
			wantLen: 13, // 9 + 4
		},
		{
			name:    "单个寄存器",
			devAddr: 0x01,
			addr:    0x0010,
			count:   1,
			data:    []byte{0x12, 0x34},
			wantLen: 11, // 9 + 2
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := BuildWriteMultiCommand(tt.devAddr, tt.addr, tt.count, tt.data)
			if len(cmd) != tt.wantLen {
				t.Fatalf("len = %d, want %d", len(cmd), tt.wantLen)
			}
			if cmd[1] != 0x10 {
				t.Errorf("funcCode = %02X, want 0x10", cmd[1])
			}
			// byteCount field = count * 2
			if cmd[6] != byte(int(tt.count)*2) {
				t.Errorf("byteCount = %d, want %d", cmd[6], int(tt.count)*2)
			}
			// Verify CRC
			dataPart := cmd[:len(cmd)-2]
			expectedCRC := CRC16(dataPart)
			actualCRC := uint16(cmd[len(cmd)-2]) | uint16(cmd[len(cmd)-1])<<8
			if actualCRC != expectedCRC {
				t.Errorf("CRC = %04X, want %04X", actualCRC, expectedCRC)
			}
		})
	}
}

// =============================================================================
// Tier 2 — G-11~G-17: ParseResponse 解析各类响应
// =============================================================================

func TestParseResponse(t *testing.T) {
	tests := []struct {
		name    string
		raw     []byte
		wantOK  bool
		wantLen int // expected length of returned data, -1 means nil
	}{
		{
			name:    "G-11 解析读响应03含2字节数据",
			raw:     appendCRC([]byte{0x01, 0x03, 0x02, 0x00, 0x01}), // byteCount=2, data=[0x00,0x01]
			wantOK:  true,
			wantLen: 2,
		},
		{
			name:    "G-11 解析读响应04含4字节数据",
			raw:     appendCRC([]byte{0x01, 0x04, 0x04, 0x42, 0x48, 0x00, 0x00}), // byteCount=4
			wantOK:  true,
			wantLen: 4,
		},
		{
			name:    "G-12 解析写单响应(06)回显地址+值",
			raw:     appendCRC([]byte{0x01, 0x06, 0x00, 0x0A, 0xFF, 0x00}), // addr=10, val=0xFF00
			wantOK:  true,
			wantLen: 4,
		},
		{
			name:    "G-12 解析写单响应(05)线圈",
			raw:     appendCRC([]byte{0x01, 0x05, 0x00, 0x00, 0xFF, 0x00}),
			wantOK:  true,
			wantLen: 4,
		},
		{
			name:    "G-13 解析写多响应(10)返回地址+数量",
			raw:     appendCRC([]byte{0x01, 0x10, 0x00, 0x0A, 0x00, 0x02}), // addr=10, count=2
			wantOK:  true,
			wantLen: 2,
		},
		{
			name:    "G-14 CRC校验失败",
			raw:     corruptCRC([]byte{0x01, 0x03, 0x02, 0x00, 0x00}),
			wantOK:  false,
			wantLen: -1,
		},
		{
			name:    "G-15 响应太短(2字节)",
			raw:     []byte{0x01, 0x03},
			wantOK:  false,
			wantLen: -1,
		},
		{
			name:    "G-15 响应太短(4字节)",
			raw:     []byte{0x01, 0x03, 0x02, 0x00},
			wantOK:  false,
			wantLen: -1,
		},
		{
			name:    "G-16 未知功能码0x2B",
			raw:     appendCRC([]byte{0x01, 0x2B, 0x00, 0x00}),
			wantOK:  false,
			wantLen: -1,
		},
		{
			name:    "G-17 异常响应0x83",
			raw:     appendCRC([]byte{0x01, 0x83, 0x02}), // exception code 2
			wantOK:  false,
			wantLen: -1,
		},
		{
			name:    "读响应byteCount超过剩余长度",
			raw:     appendCRC([]byte{0x01, 0x03, 0xFF}), // claims 255 bytes but data is empty
			wantOK:  false,
			wantLen: -1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data, ok := ParseResponse(tt.raw)
			if ok != tt.wantOK {
				t.Errorf("ok = %v, want %v", ok, tt.wantOK)
			}
			if tt.wantLen < 0 {
				if data != nil {
					t.Errorf("data = %X, want nil", data)
				}
			} else if len(data) != tt.wantLen {
				t.Errorf("len(data) = %d, want %d", len(data), tt.wantLen)
			}
		})
	}
}

// =============================================================================
// Tier 2 — G-18~G-20: VerifyCRC
// =============================================================================

func TestVerifyCRCTable(t *testing.T) {
	tests := []struct {
		name string
		data []byte
		want bool
	}{
		{
			name: "G-18 CRC正确",
			data: []byte{0x01, 0x03, 0x02, 0x00, 0x00, 0xB8, 0x44},
			want: true,
		},
		{
			name: "G-19 CRC错误(最后字节翻转)",
			data: []byte{0x01, 0x03, 0x02, 0x00, 0x00, 0xB8, 0x45},
			want: false,
		},
		{
			name: "G-19 CRC错误(首字节翻转)",
			data: []byte{0x02, 0x03, 0x02, 0x00, 0x00, 0xB8, 0x44},
			want: false,
		},
		{
			name: "G-20 太短1字节",
			data: []byte{0x01},
			want: false,
		},
		{
			name: "G-20 太短2字节",
			data: []byte{0x01, 0x03},
			want: false,
		},
		{
			name: "恰好3字节最小帧(CRC正确)",
			data: appendCRC([]byte{0x01}), // CRC16([0x01]) = 0xC040 → frame = [0x01, 0x40, 0xC0]
			want: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := VerifyCRC(tt.data)
			if got != tt.want {
				t.Errorf("VerifyCRC(%X) = %v, want %v", tt.data, got, tt.want)
			}
		})
	}
}

// =============================================================================
// Tier 2 — G-21~G-22: CodeFromStr
// =============================================================================

func TestCodeFromStr(t *testing.T) {
	tests := []struct {
		name string
		code string
		want byte
	}{
		{"G-21 已知码01", "01", 0x01},
		{"G-21 已知码02", "02", 0x02},
		{"G-21 已知码03", "03", 0x03},
		{"G-21 已知码04", "04", 0x04},
		{"G-21 已知码05", "05", 0x05},
		{"G-21 已知码06", "06", 0x06},
		{"G-21 已知码10", "10", 0x10},
		{"G-21 已知码0F", "0F", 0x0F},
		{"G-22 未知码FF回退06", "FF", 0x06},
		{"G-22 未知码空字符串回退06", "", 0x06},
		{"G-22 未知码乱码回退06", "xyz", 0x06},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := CodeFromStr(tt.code)
			if got != tt.want {
				t.Errorf("CodeFromStr(%q) = %02X, want %02X", tt.code, got, tt.want)
			}
		})
	}
}

// =============================================================================
// 辅助函数: 为数据附加正确的 CRC 或损坏的 CRC
// =============================================================================

func appendCRC(data []byte) []byte {
	crc := CRC16(data)
	return append(data, byte(crc), byte(crc>>8))
}

func corruptCRC(data []byte) []byte {
	// Append deliberately wrong CRC (increment by 1)
	crc := CRC16(data) + 1
	return append(data, byte(crc), byte(crc>>8))
}
