package handler

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestMParse(t *testing.T) {
	// M is just map[string]any — verify usage
	m := M{"key": "value", "count": 42}
	if m["key"] != "value" {
		t.Errorf("M key = %v", m["key"])
	}
	if m["count"] != 42 {
		t.Errorf("M count = %v", m["count"])
	}
}

func TestPathIDTrailingSlash(t *testing.T) {
	// Paths with trailing slashes should still work
	if pathID("/api/v1/projects/7/") != 7 {
		t.Error("trailing slash not handled")
	}
}

func TestPathIDMultipleSegments(t *testing.T) {
	// pathID skips non-numeric segments to find the last numeric one
	if pathID("/a/b/c/d/e/f/123/g") != 123 {
		t.Error("should return 123 by skipping non-numeric 'g'")
	}
	if pathID("/a/b/c/d/e/f/123") != 123 {
		t.Error("last numeric segment should return 123")
	}
}

// ---- pathID (skips non-numeric suffixes like "control", "execute", "ack") ----

func TestPathID(t *testing.T) {
	tests := []struct {
		path string
		want int
	}{
		{"/api/v1/devices/5", 5},
		{"/api/v1/devices/5/control", 5},
		{"/api/v1/startup-plans/3/execute", 3},
		{"/api/v1/alarm-logs/7/ack", 7},
		{"/api/v1/devices/0", 0},
		{"/api/v1/devices/abc", 0},
		{"/", 0},
	}
	for _, tt := range tests {
		got := pathID(tt.path)
		if got != tt.want {
			t.Errorf("pathID(%q) = %d, want %d", tt.path, got, tt.want)
		}
	}
}

func TestPathIDMultiAction(t *testing.T) {
	// path with multiple non-numeric suffixes after ID
	if pathID("/api/devices/42/control/extra") != 42 {
		t.Error("should extract 42 even with extra segments")
	}
}

// ---- ok / created / notFound / serverErr ----

func TestOk(t *testing.T) {
	rec := httptest.NewRecorder()
	ok(rec, M{"status": "ok"})
	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", rec.Code)
	}
}

func TestCreated(t *testing.T) {
	rec := httptest.NewRecorder()
	created(rec, M{"id": 1})
	if rec.Code != http.StatusCreated {
		t.Errorf("status = %d, want 201", rec.Code)
	}
}

func TestNotFound(t *testing.T) {
	rec := httptest.NewRecorder()
	notFound(rec, "楼宇不存在")
	if rec.Code != http.StatusNotFound {
		t.Errorf("status = %d, want 404", rec.Code)
	}
	if s := rec.Body.String(); s == "" {
		t.Error("body should not be empty")
	}
}

func TestServerErr(t *testing.T) {
	rec := httptest.NewRecorder()
	serverErr(rec, &testError{"database connection failed"})
	if rec.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want 500", rec.Code)
	}
	ct := rec.Header().Get("Content-Type")
	if ct != "application/json" {
		t.Errorf("Content-Type = %q, want application/json", ct)
	}
}

type testError struct{ msg string }

func (e *testError) Error() string { return e.msg }

func TestQueryIntMultipleKeys(t *testing.T) {
	req := httptest.NewRequest("GET", "/test?id=10&device_id=5&metric=A", nil)
	if queryInt(req, "device_id") != 5 {
		t.Error("queryInt('device_id') should return 5")
	}
	if queryInt(req, "metric") != 0 {
		t.Error("queryInt('metric') should return 0 for non-numeric")
	}
	if queryInt(req, "absent") != 0 {
		t.Error("queryInt('absent') should return 0")
	}
}
