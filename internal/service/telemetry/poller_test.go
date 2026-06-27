package telemetry

import (
	"math"
	"testing"

	"xmeco/internal/domain"
)

// =============================================================================
// Tier 2 — T-01~T-12: decodeVal 多类型解码
// =============================================================================

func ptrStr(s string) *string { return &s }

func TestDecodeVal(t *testing.T) {
	tests := []struct {
		name     string
		data     []byte
		reg      domain.Register
		wantVal  float64
		wantPanic bool
	}{
		{
			name:    "T-01 空数据返回0",
			data:    []byte{},
			reg:     domain.Register{DataType: "u16", Magnification: 1},
			wantVal: 0.0,
		},
		{
			name:    "T-02 单字节无符号u8",
			data:    []byte{0x2A},
			reg:     domain.Register{DataType: "u8", Magnification: 1},
			wantVal: 42.0,
		},
		{
			name:    "T-03 双字节大端无符号u16",
			data:    []byte{0x01, 0x00},
			reg:     domain.Register{DataType: "u16", Magnification: 1},
			wantVal: 256.0,
		},
		{
			name:    "T-04 双字节小端(低位在前)",
			data:    []byte{0x00, 0x01},
			reg:     domain.Register{DataType: "u16", DataOrder: "低位在前", Magnification: 1},
			wantVal: 256.0,
		},
		{
			name:    "T-05 四字节大端无符号u32",
			data:    []byte{0x00, 0x00, 0x01, 0x00},
			reg:     domain.Register{DataType: "u32", Magnification: 1},
			wantVal: 256.0,
		},
		{
			name:    "T-06 四字节低字在前",
			data:    []byte{0x01, 0x00, 0x00, 0x00}, // 低字0x0100=256在前, 高字0x0000在后
			reg:     domain.Register{DataType: "u32", DataOrder: "低字在前", Magnification: 1},
			wantVal: 256.0,
		},
		{
			name:    "T-07 IEEE754浮点32",
			data:    []byte{0x42, 0x48, 0x00, 0x00}, // 50.0f
			reg:     domain.Register{DataType: "float", Magnification: 1},
			wantVal: 50.0,
		},
		{
			name:    "T-08 有符号s16负数",
			data:    []byte{0xFF, 0xCE}, // int16 = -50
			reg:     domain.Register{DataType: "s16", Magnification: 1},
			wantVal: -50.0,
		},
		{
			name:    "T-09 有符号s32负数",
			data:    []byte{0xFF, 0xFF, 0xFF, 0xCE}, // int32 = -50
			reg:     domain.Register{DataType: "s32", Magnification: 1},
			wantVal: -50.0,
		},
		{
			name:    "T-10 掩码提取低字节",
			data:    []byte{0x12, 0x34},
			reg:     domain.Register{DataType: "u16", DataMask: ptrStr("00FF"), Magnification: 1},
			wantVal: 52.0, // 0x34 = 52
		},
		{
			name:    "T-11 倍率除法1000/10=100",
			data:    []byte{0x03, 0xE8}, // 1000
			reg:     domain.Register{DataType: "u16", Magnification: 10.0},
			wantVal: 100.0,
		},
		{
			name:    "T-12 倍率为零不panic",
			data:    []byte{0x01, 0x00},
			reg:     domain.Register{DataType: "u16", Magnification: 0},
			// Go float64/0.0 = +Inf; 函数不应 panic
			wantPanic: false,
		},
		// 补充边界
		{
			name:    "float类型含'浮点'关键词",
			data:    []byte{0x42, 0x48, 0x00, 0x00},
			reg:     domain.Register{DataType: "浮点数", Magnification: 1},
			wantVal: 50.0,
		},
		{
			name:    "数据不足4字节走整数分支",
			data:    []byte{0x01, 0x02},
			reg:     domain.Register{DataType: "u16", Magnification: 1},
			wantVal: 258.0, // 0x0102 = 258
		},
		{
			name:    "4字节以上走default:取前2字节",
			data:    []byte{0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08},
			reg:     domain.Register{DataType: "u16", Magnification: 1},
			wantVal: 258.0, // 前2字节 0x0102 = 258
		},
		{
			name:    "掩码为空字符串",
			data:    []byte{0x12, 0x34},
			reg:     domain.Register{DataType: "u16", DataMask: ptrStr(""), Magnification: 1},
			wantVal: 4660.0, // 0x1234 = 4660
		},
		{
			name:    "掩码为nil",
			data:    []byte{0x12, 0x34},
			reg:     domain.Register{DataType: "u16", DataMask: nil, Magnification: 1},
			wantVal: 4660.0,
		},
		{
			name:    "无效hex掩码回退原始值",
			data:    []byte{0x12, 0x34},
			reg:     domain.Register{DataType: "u16", DataMask: ptrStr("ZZZ"), Magnification: 1},
			wantVal: 4660.0,
		},
		{
			name:    "有符号s16正数(符号识别)",
			data:    []byte{0x00, 0x64}, // 100
			reg:     domain.Register{DataType: "s16", Magnification: 1},
			wantVal: 100.0,
		},
		{
			name:    "int32关键词识别有符号",
			data:    []byte{0xFF, 0xFF, 0xFF, 0xCE},
			reg:     domain.Register{DataType: "int32", Magnification: 1},
			wantVal: -50.0,
		},
		{
			name:    "3字节数据走default取前2字节",
			data:    []byte{0x01, 0x02, 0x03},
			reg:     domain.Register{DataType: "u16", Magnification: 1},
			wantVal: 258.0, // 前2字节 0x0102
		},
		{
			name:    "float+signed同时存在走float路径",
			data:    []byte{0x42, 0x48, 0x00, 0x00},
			reg:     domain.Register{DataType: "float_signed", Magnification: 1},
			wantVal: 50.0, // float优先
		},
		{
			name:    "倍率+掩码+有符号组合",
			data:    []byte{0xFF, 0xCE},
			reg:     domain.Register{DataType: "s16", DataMask: ptrStr("00FF"), Magnification: 10.0},
			wantVal: math.Round(float64(int16(0xCE))/10.0*1000) / 1000, // 206/10=20.6 → int16(206)=206 → 20.6
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.wantPanic {
				defer func() {
					if r := recover(); r == nil {
						t.Error("expected panic but got none")
					}
				}()
			}

			got := decodeVal(tt.data, tt.reg)

			if !tt.wantPanic {
				// T-12: 只需要不panic, 值可以是 Inf
				if tt.name == "T-12 倍率为零不panic" {
					if math.IsInf(got, 1) || math.IsInf(got, -1) || math.IsNaN(got) {
						t.Logf("magnification=0 返回特殊值 %v (Go IEEE 754 行为, 未panic)", got)
					}
					return
				}
				if got != tt.wantVal {
					t.Errorf("decodeVal() = %v, want %v", got, tt.wantVal)
				}
			}
		})
	}
}

// =============================================================================
// Tier 2 — T-13~T-15: parseStatusMapping 状态码映射
// =============================================================================

func TestParseStatusMapping(t *testing.T) {
	tests := []struct {
		name       string
		statusCode string
		rawVal     float64
		wantLabel  string
		wantOK     bool
	}{
		{
			name:       "T-13 匹配运行状态",
			statusCode: "01=运行,02=停机",
			rawVal:     1.0,
			wantLabel:  "运行",
			wantOK:     true,
		},
		{
			name:       "T-14 不匹配返回空",
			statusCode: "01=运行",
			rawVal:     3.0, // hex "03"
			wantLabel:  "",
			wantOK:     false,
		},
		{
			name:       "T-15 大小写不敏感",
			statusCode: "0A=告警",
			rawVal:     10.0, // hex "0A" / "0a"
			wantLabel:  "告警",
			wantOK:     true,
		},
		{
			name:       "多组匹配中间项",
			statusCode: "01=运行,02=停机,03=故障,04=维护",
			rawVal:     3.0,
			wantLabel:  "故障",
			wantOK:     true,
		},
		{
			name:       "空statusCode",
			statusCode: "",
			rawVal:     1.0,
			wantLabel:  "",
			wantOK:     false,
		},
		{
			name:       "格式错误无等号",
			statusCode: "01运行",
			rawVal:     1.0,
			wantLabel:  "",
			wantOK:     false,
		},
		{
			name:       "含空格的values被TrimSpace处理",
			statusCode: "01=运行 , 02=停机",
			rawVal:     2.0,
			wantLabel:  "停机",
			wantOK:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotLabel, gotOK := parseStatusMapping(tt.statusCode, tt.rawVal)
			if gotOK != tt.wantOK {
				t.Errorf("ok = %v, want %v", gotOK, tt.wantOK)
			}
			if gotLabel != tt.wantLabel {
				t.Errorf("label = %q, want %q", gotLabel, tt.wantLabel)
			}
		})
	}
}

// =============================================================================
// Tier 2 — T-16~T-18: reorderForOrder 字节序重排
// =============================================================================

func TestReorderForOrder(t *testing.T) {
	tests := []struct {
		name string
		data []byte
		order string
		want []byte
	}{
		{
			name:  "T-16 大端空字符串不改变",
			data:  []byte{0x01, 0x02, 0x03, 0x04},
			order: "",
			want:  []byte{0x01, 0x02, 0x03, 0x04},
		},
		{
			name:  "T-16 大端高位在前不改变",
			data:  []byte{0x01, 0x02, 0x03, 0x04},
			order: "高位在前",
			want:  []byte{0x01, 0x02, 0x03, 0x04},
		},
		{
			name:  "T-17 低位在前交换字节",
			data:  []byte{0x01, 0x02, 0x03, 0x04},
			order: "低位在前",
			want:  []byte{0x02, 0x01, 0x04, 0x03},
		},
		{
			name:  "T-18 低字在前交换字序",
			data:  []byte{0x01, 0x02, 0x03, 0x04},
			order: "低字在前",
			want:  []byte{0x03, 0x04, 0x01, 0x02},
		},
		// 边界
		{
			name:  "单字节不变",
			data:  []byte{0xAA},
			order: "低位在前",
			want:  []byte{0xAA},
		},
		{
			name:  "空数据不变",
			data:  []byte{},
			order: "低位在前",
			want:  []byte{},
		},
		{
			name:  "低字在前不足4字节不变",
			data:  []byte{0x01, 0x02},
			order: "低字在前",
			want:  []byte{0x01, 0x02},
		},
		{
			name:  "低位在前奇数长度最后字节保留",
			data:  []byte{0x01, 0x02, 0x03},
			order: "低位在前",
			want:  []byte{0x02, 0x01, 0x03},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := reorderForOrder(tt.data, tt.order)
			if len(got) != len(tt.want) {
				t.Fatalf("len = %d, want %d", len(got), len(tt.want))
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Errorf("byte[%d] = 0x%02X, want 0x%02X", i, got[i], tt.want[i])
				}
			}
		})
	}
}

// =============================================================================
// Tier 2 — 补充: applyMask + parseHex
// =============================================================================

func TestApplyMask(t *testing.T) {
	tests := []struct {
		name string
		raw  uint64
		mask *string
		want uint64
	}{
		{"nil掩码不改变", 0x1234, nil, 0x1234},
		{"空字符串掩码不改变", 0x1234, ptrStr(""), 0x1234},
		{"提取低字节00FF", 0x1234, ptrStr("00FF"), 0x0034},
		{"提取高字节FF00", 0x1234, ptrStr("FF00"), 0x1200},
		{"全掩码FFFF", 0x5678, ptrStr("FFFF"), 0x5678},
		{"零掩码0000", 0x5678, ptrStr("0000"), 0},
		{"无效hex回退原始值", 0x1234, ptrStr("GGGG"), 0x1234},
		{"掩码位数少于原始值", 0xABCD, ptrStr("0F"), 0x000D},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := applyMask(tt.raw, tt.mask)
			if got != tt.want {
				t.Errorf("applyMask(0x%X, %v) = 0x%X, want 0x%X", tt.raw, tt.mask, got, tt.want)
			}
		})
	}
}

func TestParseHex(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    uint64
		wantErr bool
	}{
		{"小写hex", "ff", 255, false},
		{"大写hex", "FF", 255, false},
		{"混合hex", "0a0B", 2571, false},
		{"零值", "0000", 0, false},
		{"完整16位", "FFFF", 65535, false},
		{"空字符串", "", 0, false},
		{"非法字符", "GG", 0, true},
		{"含特殊字符", "12 34", 0, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseHex(tt.input)
			if tt.wantErr {
				if err == nil {
					t.Errorf("parseHex(%q) expected error, got %d", tt.input, got)
				}
				return
			}
			if err != nil {
				t.Errorf("parseHex(%q) unexpected error: %v", tt.input, err)
			}
			if got != tt.want {
				t.Errorf("parseHex(%q) = %d, want %d", tt.input, got, tt.want)
			}
		})
	}
}
