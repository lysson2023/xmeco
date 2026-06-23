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

func TestParseFloat(t *testing.T) {
	tests := []struct{ s string; want float64 }{
		{"200", 200}, {"3.5", 3.5}, {"0.01", 0.01}, {"", 0}, {"abc", 0},
	}
	for _, tt := range tests {
		if got := parseFloat(tt.s); got != tt.want {
			t.Errorf("parseFloat(%q) = %f, want %f", tt.s, got, tt.want)
		}
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

func BenchmarkTriggered(b *testing.B) {
	for i := 0; i < b.N; i++ {
		triggered("gt", 75.5, 70.0, "", "")
	}
}
