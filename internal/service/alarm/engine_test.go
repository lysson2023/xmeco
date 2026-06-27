package alarm

import (
	"testing"
)

func TestTriggered(t *testing.T) {
	tests := []struct {
		cond      string
		val       float64
		threshold float64
		minV, maxV string
		want      bool
	}{
		// gt: greater than
		{"gt", 50.1, 50.0, "", "", true},
		{"gt", 50.0, 50.0, "", "", false},
		{"gt", 49.9, 50.0, "", "", false},

		// ge: greater or equal
		{"ge", 50.1, 50.0, "", "", true},
		{"ge", 50.0, 50.0, "", "", true},
		{"ge", 49.9, 50.0, "", "", false},

		// lt: less than
		{"lt", 49.9, 50.0, "", "", true},
		{"lt", 50.0, 50.0, "", "", false},
		{"lt", 50.1, 50.0, "", "", false},

		// le: less or equal
		{"le", 49.9, 50.0, "", "", true},
		{"le", 50.0, 50.0, "", "", true},
		{"le", 50.1, 50.0, "", "", false},

		// eq: equal
		{"eq", 50.0, 50.0, "", "", true},
		{"eq", 50.1, 50.0, "", "", false},
		{"eq", 49.9, 50.0, "", "", false},

		// range: outside [min, max] triggers
		{"range", 180.0, 0, "200", "240", true},   // below min
		{"range", 260.0, 0, "200", "240", true},   // above max
		{"range", 220.0, 0, "200", "240", false},  // inside range
		{"range", 200.0, 0, "200", "240", false},  // exactly at min
		{"range", 240.0, 0, "200", "240", false},  // exactly at max
		{"range", 5.0,   0, "10", "",    true},    // above max, min only

		// unknown condition
		{"xx", 100.0, 50.0, "", "", false},
		{"", 100.0, 50.0, "", "", false},
	}

	for _, tt := range tests {
		got := triggered(tt.cond, tt.val, tt.threshold, tt.minV, tt.maxV)
		if got != tt.want {
			t.Errorf("triggered(%q, %.1f, %.1f, %q, %q) = %v, want %v",
				tt.cond, tt.val, tt.threshold, tt.minV, tt.maxV, got, tt.want)
		}
	}
}

func TestTriggeredNegative(t *testing.T) {
	if !triggered("lt", -5.0, -2.0, "", "") {
		t.Error("lt: -5 < -2 should be true")
	}
	if !triggered("gt", -2.0, -5.0, "", "") {
		t.Error("gt: -2 > -5 should be true")
	}
	if triggered("gt", -8.0, -5.0, "", "") {
		t.Error("gt: -8 > -5 should be false")
	}
}

func TestTriggeredZero(t *testing.T) {
	if !triggered("eq", 0.0, 0.0, "", "") {
		t.Error("eq: 0 == 0 should be true")
	}
	if !triggered("le", 0.0, 0.0, "", "") {
		t.Error("le: 0 <= 0 should be true")
	}
	if triggered("lt", 0.0, 0.0, "", "") {
		t.Error("lt: 0 < 0 should be false")
	}
	if !triggered("gt", 0.01, 0.0, "", "") {
		t.Error("gt: 0.01 > 0 should be true")
	}
}

func TestTriggeredLargeNumbers(t *testing.T) {
	if !triggered("gt", 999999.9, 999999.0, "", "") {
		t.Error("gt: large numbers failed")
	}
	if triggered("eq", 1e10, 1e10+1, "", "") {
		t.Error("eq: large numbers false positive")
	}
}

func TestTriggeredRange(t *testing.T) {
	// Range with float min/max
	if !triggered("range", 2.5, 0, "3.0", "8.0") {
		t.Error("range: 2.5 outside [3.0, 8.0] → should trigger")
	}
	if triggered("range", 5.0, 0, "3.0", "8.0") {
		t.Error("range: 5.0 inside [3.0, 8.0] → should NOT trigger")
	}
}

func TestParseFloatOK(t *testing.T) {
	tests := []struct {
		s        string
		wantVal  float64
		wantOK   bool
	}{
		{"200", 200, true},
		{"3.5", 3.5, true},
		{"0.01", 0.01, true},
		{"0", 0, true},
		{"-10", -10, true},
		{"", 0, false},
		{"abc", 0, false},
		{"12.34.56", 0, false},
		{" 5 ", 0, false},
	}
	for _, tt := range tests {
		gotVal, gotOK := parseFloatOK(tt.s)
		if gotVal != tt.wantVal || gotOK != tt.wantOK {
			t.Errorf("parseFloatOK(%q) = (%f, %v), want (%f, %v)", tt.s, gotVal, gotOK, tt.wantVal, tt.wantOK)
		}
	}
}

func TestTriggeredRangeSkipOnParseFailure(t *testing.T) {
	// BUG6 regression: when range bounds fail to parse, the rule must be
	// skipped (return false) instead of falling back to 0 and false-triggering.
	if triggered("range", 100.0, 0, "abc", "200") {
		t.Error("range with invalid min should be skipped (return false)")
	}
	if triggered("range", 100.0, 0, "200", "abc") {
		t.Error("range with invalid max should be skipped (return false)")
	}
	if triggered("range", 100.0, 0, "abc", "def") {
		t.Error("range with both invalid should be skipped (return false)")
	}
	if triggered("range", 0.0, 0, "abc", "200") {
		t.Error("range with invalid min should NOT trigger even when val=0")
	}
	// Sanity: valid bounds still work
	if !triggered("range", 1.0, 0, "10", "20") {
		t.Error("range with valid bounds should still work")
	}
}

func TestCondCN(t *testing.T) {
	tests := []struct{ code, want string }{
		{"gt", "超过"},
		{"ge", "达到或超过"},
		{"lt", "低于"},
		{"le", "达到或低于"},
		{"eq", "等于"},
		{"range", "超出范围"},
		{"xx", "xx"},
		{"", ""},
	}
	for _, tt := range tests {
		got := condCN(tt.code)
		if got != tt.want {
			t.Errorf("condCN(%q) = %q, want %q", tt.code, got, tt.want)
		}
	}
}

func TestCondCNUnknownReturnsCode(t *testing.T) {
	if condCN("foo") != "foo" {
		t.Error("unknown code should return itself")
	}
}

// =============================================================================
// Tier 2 — AL-12: range 仅上限 (仅 maxVal 有效)
// =============================================================================

func TestTriggeredRangeMaxOnly(t *testing.T) {
	// AL-12: 仅设置上限(无下限)，值超出上限应触发
	tests := []struct {
		name  string
		minV  string
		maxV  string
		val   float64
		want  bool
	}{
		{
			name: "AL-12 仅上限-值超出触发",
			minV:  "",
			maxV:  "30",
			val:   35.0,
			want:  true,
		},
		{
			name: "AL-12 仅上限-值在范围内不触发",
			minV:  "",
			maxV:  "30",
			val:   25.0,
			want:  false,
		},
		{
			name: "AL-12 仅上限-值等于上限不触发(闭区间)",
			minV:  "",
			maxV:  "30",
			val:   30.0,
			want:  false,
		},
		{
			name: "仅下限-值低于触发",
			minV:  "10",
			maxV:  "",
			val:   5.0,
			want:  true,
		},
		{
			name: "仅下限-值在范围内不触发",
			minV:  "10",
			maxV:  "",
			val:   15.0,
			want:  false,
		},
		{
			name: "双限均空-不触发",
			minV:  "",
			maxV:  "",
			val:   100.0,
			want:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := triggered("range", tt.val, 0, tt.minV, tt.maxV)
			if got != tt.want {
				t.Errorf("triggered(range, %.1f, 0, %q, %q) = %v, want %v",
					tt.val, tt.minV, tt.maxV, got, tt.want)
			}
		})
	}
}

// =============================================================================
// Tier 2 — buildAlarmMsg 告警消息构建
// =============================================================================

func TestBuildAlarmMsg(t *testing.T) {
	tests := []struct {
		name       string
		deviceName string
		metric     string
		value      float64
		cond       string
		threshold  float64
		minV       string
		maxV       string
		wantSubstr string // 消息应包含的关键字
	}{
		{
			name:       "gt-超过阈值",
			deviceName: "冷水机组1",
			metric:     "temperature",
			value:      35.5,
			cond:       "gt",
			threshold:  30.0,
			wantSubstr: "冷水机组1 temperature 35.5 超过 阈值 30.0",
		},
		{
			name:       "ge-达到或超过",
			deviceName: "泵站A",
			metric:     "pressure",
			value:      1.5,
			cond:       "ge",
			threshold:  1.0,
			wantSubstr: "泵站A pressure 1.5 达到或超过 阈值 1.0",
		},
		{
			name:       "lt-低于阈值",
			deviceName: "锅炉1",
			metric:     "level",
			value:      0.5,
			cond:       "lt",
			threshold:  1.0,
			wantSubstr: "锅炉1 level 0.5 低于 阈值 1.0",
		},
		{
			name:       "le-达到或低于",
			deviceName: "冷却塔",
			metric:     "flow",
			value:      2.0,
			cond:       "le",
			threshold:  2.5,
			wantSubstr: "冷却塔 flow 2.0 达到或低于 阈值 2.5",
		},
		{
			name:       "eq-等于",
			deviceName: "传感器A",
			metric:     "status",
			value:      1.0,
			cond:       "eq",
			threshold:  1.0,
			wantSubstr: "传感器A status 1.0 等于 1.0",
		},
		{
			name:       "range-超出范围",
			deviceName: "机组B",
			metric:     "humidity",
			value:      95.0,
			cond:       "range",
			minV:       "30",
			maxV:       "80",
			wantSubstr: "机组B humidity 95.0 超出范围 [30, 80]",
		},
		{
			name:       "未知条件原样返回代码",
			deviceName: "dev1",
			metric:     "m",
			value:      1.0,
			cond:       "custom_cond",
			threshold:  0.5,
			wantSubstr: "dev1 m 1.0 custom_cond 阈值 0.5",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := buildAlarmMsg(tt.deviceName, tt.metric, tt.value, tt.cond, tt.threshold, tt.minV, tt.maxV)
			if got != tt.wantSubstr {
				// For buildAlarmMsg the format is exact, check full string match
				t.Errorf("buildAlarmMsg() = %q, want %q", got, tt.wantSubstr)
			}
		})
	}
}

func BenchmarkTriggered(b *testing.B) {
	for i := 0; i < b.N; i++ {
		triggered("gt", 75.5, 70.0, "", "")
	}
}
