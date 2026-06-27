package transport

import (
	"strings"
	"testing"
)

// testMAC is a fixed 6-byte MAC used for all wrap/unwrap tests
var testMAC = []byte{0xAA, 0xBB, 0xCC, 0xDD, 0xEE, 0xFF}

func newTestTransport() *CustomTransport {
	return &CustomTransport{mac: testMAC}
}

// =============================================================================
// Tier 2 — CT-01~CT-03: wrap 帧构建与校验和
// =============================================================================

func TestWrap(t *testing.T) {
	tests := []struct {
		name        string
		data        []byte
		checkFunc   func(t *testing.T, frame []byte)
	}{
		{
			name: "CT-01 基本包裹-Modbus读命令",
			data: []byte{0x01, 0x03, 0x00, 0x00, 0x00, 0x01},
			checkFunc: func(t *testing.T, frame []byte) {
				// 帧以 0x68 开头
				if frame[0] != 0x68 {
					t.Errorf("frame[0] = %02X, want 0x68", frame[0])
				}
				// 帧以 0x16 结尾
				if frame[len(frame)-1] != 0x16 {
					t.Errorf("frame[-1] = %02X, want 0x16", frame[len(frame)-1])
				}
				// 长度 >= 14
				if len(frame) < 14 {
					t.Errorf("frame len = %d, want >= 14", len(frame))
				}
				// MAC 在位置 1-6
				for i := 0; i < 6; i++ {
					if frame[1+i] != testMAC[i] {
						t.Errorf("frame[%d] = %02X, want MAC byte %02X", 1+i, frame[1+i], testMAC[i])
					}
				}
				// 命令字节
				if frame[7] != 0xE4 || frame[8] != 0xA1 {
					t.Errorf("cmd bytes = %02X %02X, want E4 A1", frame[7], frame[8])
				}
			},
		},
		{
			name: "CT-02 空数据最小帧14字节",
			data: []byte{},
			checkFunc: func(t *testing.T, frame []byte) {
				if len(frame) != 14 {
					t.Errorf("min frame len = %d, want 14", len(frame))
				}
				// 长度字段应为 0x0000
				dLen := int(frame[9])<<8 | int(frame[10])
				if dLen != 0 {
					t.Errorf("dataLen = %d, want 0", dLen)
				}
			},
		},
		{
			name: "CT-03 校验和正确性验证",
			data: []byte{0x01, 0x02, 0x03, 0x04, 0x05},
			checkFunc: func(t *testing.T, frame []byte) {
				dataLen := int(frame[9])<<8 | int(frame[10])
				// 手动计算校验和 (不含最后3字节: checksum_hi, checksum_lo, 0x16)
				sum := uint16(0)
				for i := 0; i < 11+dataLen; i++ {
					sum += uint16(frame[i])
				}
				expectedHi := byte(sum >> 8)
				expectedLo := byte(sum)
				if frame[11+dataLen] != expectedHi || frame[12+dataLen] != expectedLo {
					t.Errorf("checksum = %02X%02X, want %02X%02X",
						frame[11+dataLen], frame[12+dataLen], expectedHi, expectedLo)
				}
			},
		},
		{
			name: "长度字段正确编码",
			data: []byte{0x12, 0x34},
			checkFunc: func(t *testing.T, frame []byte) {
				// dataLen=2 → frame[9]=0x00, frame[10]=0x02
				if frame[9] != 0x00 || frame[10] != 0x02 {
					t.Errorf("len field = %02X%02X, want 0002", frame[9], frame[10])
				}
			},
		},
		{
			name: "大数据长度>255高字节非零",
			data: make([]byte, 300),
			checkFunc: func(t *testing.T, frame []byte) {
				dLen := int(frame[9])<<8 | int(frame[10])
				if dLen != 300 {
					t.Errorf("dataLen = %d, want 300", dLen)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ct := newTestTransport()
			frame := ct.wrap(tt.data)
			tt.checkFunc(t, frame)
		})
	}
}

// =============================================================================
// Tier 2 — CT-04~CT-11: unwrap 解码与错误处理
// =============================================================================

func TestUnwrapRoundTrip(t *testing.T) {
	tests := []struct {
		name string
		data []byte
	}{
		{
			name: "CT-04 正常解包Modbus读命令",
			data: []byte{0x01, 0x03, 0x00, 0x00, 0x00, 0x01},
		},
		{
			name: "CT-04 正常解包小数据",
			data: []byte{0x0A},
		},
		{
			name: "CT-04 正常解包多字节",
			data: []byte{0x01, 0x03, 0x02, 0x00, 0x01},
		},
		{
			name: "CT-04 空数据解包",
			data: []byte{},
		},
		{
			name: "CT-11 接近1024字节缓冲区限制",
			data: make([]byte, 1000),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ct := newTestTransport()
			frame := ct.wrap(tt.data)

			// unwrap 应返回完全一致的原始 payload
			got, err := ct.unwrap(frame)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if len(got) != len(tt.data) {
				t.Fatalf("len = %d, want %d", len(got), len(tt.data))
			}
			for i := range got {
				if got[i] != tt.data[i] {
					t.Errorf("byte[%d] = %02X, want %02X", i, got[i], tt.data[i])
				}
			}
		})
	}
}

func TestUnwrapErrors(t *testing.T) {
	ct := newTestTransport()

	tests := []struct {
		name    string
		buildFrame func() []byte
		wantErrSubstr string
	}{
		{
			name: "CT-05 帧太短(13字节)",
			buildFrame: func() []byte {
				return make([]byte, 13) // < 14
			},
			wantErrSubstr: "too short",
		},
		{
			name: "CT-05 帧太短(0字节)",
			buildFrame: func() []byte {
				return []byte{}
			},
			wantErrSubstr: "too short",
		},
		{
			name: "CT-06 起始标记错(非0x68)",
			buildFrame: func() []byte {
				frame := ct.wrap([]byte{0x01, 0x03})
				frame[0] = 0x00 // corrupt start marker
				return frame
			},
			wantErrSubstr: "markers",
		},
		{
			name: "CT-07 结束标记错(非0x16)",
			buildFrame: func() []byte {
				frame := ct.wrap([]byte{0x01, 0x03})
				frame[len(frame)-1] = 0x00 // corrupt end marker
				return frame
			},
			wantErrSubstr: "markers",
		},
		{
			name: "CT-08 命令字节Hi不匹配",
			buildFrame: func() []byte {
				frame := ct.wrap([]byte{0x01, 0x03})
				frame[7] = 0x00 // corrupt cmdHi
				return frame
			},
			wantErrSubstr: "command",
		},
		{
			name: "CT-08 命令字节Lo不匹配",
			buildFrame: func() []byte {
				frame := ct.wrap([]byte{0x01, 0x03})
				frame[8] = 0x00 // corrupt cmdLo
				return frame
			},
			wantErrSubstr: "command",
		},
		{
			name: "CT-09 声明长度超过实际缓冲区",
			buildFrame: func() []byte {
				frame := ct.wrap([]byte{0x01, 0x03})
				// Overwrite length field to claim 100 bytes of data
				frame[9] = 0x00
				frame[10] = 100
				return frame
			},
			wantErrSubstr: "length mismatch",
		},
		{
			name: "CT-10 校验和错误(翻转数据字节)",
			buildFrame: func() []byte {
				frame := ct.wrap([]byte{0x01, 0x03, 0x00, 0x10})
				frame[11] ^= 0xFF // flip a data byte, invalidates checksum
				return frame
			},
			wantErrSubstr: "checksum",
		},
		{
			name: "CT-10 校验和错误(翻转checksum字节)",
			buildFrame: func() []byte {
				frame := ct.wrap([]byte{0x01, 0x03})
				csPos := len(frame) - 3 // 0-indexed position of checksumHi
				frame[csPos] ^= 0xFF
				return frame
			},
			wantErrSubstr: "checksum",
		},
		{
			name: "起始和结束标记同时错误",
			buildFrame: func() []byte {
				frame := ct.wrap([]byte{0x01, 0x03})
				frame[0] = 0xFF
				frame[len(frame)-1] = 0xFF
				return frame
			},
			wantErrSubstr: "markers",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			frame := tt.buildFrame()
			payload, err := ct.unwrap(frame)
			if err == nil {
				t.Errorf("expected error containing %q, got nil (payload=%X)", tt.wantErrSubstr, payload)
				return
			}
			if !strings.Contains(err.Error(), tt.wantErrSubstr) {
				t.Errorf("error = %q, want to contain %q", err.Error(), tt.wantErrSubstr)
			}
			if payload != nil {
				t.Errorf("payload should be nil on error, got %X", payload)
			}
		})
	}
}


