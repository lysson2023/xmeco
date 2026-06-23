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

func TestPathLast(t *testing.T) {
	tests := []struct {
		path string
		want int
	}{
		{"/api/v1/projects/1", 1},
		{"/api/v1/devices/42", 42},
		{"/api/v1/users/999", 999},
		{"/api/v1/projects/1/", 1},
		{"/api/v1/buildings/0", 0},
		{"/no/number/here", 0},
		{"/", 0},
	}
	for _, tt := range tests {
		got := pathLast(tt.path)
		if got != tt.want {
			t.Errorf("pathLast(%q) = %d, want %d", tt.path, got, tt.want)
		}
	}
}

func TestQueryInt(t *testing.T) {
	tests := []struct {
		query string
		want  int
	}{
		{"?id=5", 5},
		{"?page=10&size=20", 0}, // 'id' not present
		{"?id=abc", 0},           // non-numeric
		{"?id=", 0},              // empty
		{"", 0},                  // no query
	}
	for _, tt := range tests {
		req := httptest.NewRequest("GET", "/test"+tt.query, nil)
		got := queryInt(req, "id")
		if got != tt.want {
			t.Errorf("queryInt(%q, \"id\") = %d, want %d", tt.query, got, tt.want)
		}
	}
}

func TestWriteJSON(t *testing.T) {
	rec := httptest.NewRecorder()
	writeJSON(rec, http.StatusOK, M{"status": "ok"})

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", rec.Code)
	}
	ct := rec.Header().Get("Content-Type")
	if ct != "application/json" {
		t.Errorf("Content-Type = %q, want application/json", ct)
	}
	if rec.Body.Len() == 0 {
		t.Error("body is empty")
	}
}

func TestWriteJSONError(t *testing.T) {
	rec := httptest.NewRecorder()
	writeJSON(rec, http.StatusBadRequest, M{"error": "bad request"})

	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", rec.Code)
	}
}

func TestWriteJSONNil(t *testing.T) {
	rec := httptest.NewRecorder()
	writeJSON(rec, http.StatusOK, nil)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", rec.Code)
	}
	// nil marshals to "null\n" via json.NewEncoder which appends newline
	if s := rec.Body.String(); s != "null\n" && s != "null" {
		t.Errorf("nil body = %q, want null or null\\n", s)
	}
}

func TestPathLastTrailingSlash(t *testing.T) {
	// Paths with trailing slashes should still work
	if pathLast("/api/v1/projects/7/") != 7 {
		t.Error("trailing slash not handled")
	}
}

func TestPathLastMultipleSegments(t *testing.T) {
	if pathLast("/a/b/c/d/e/f/123/g") != 0 {
		t.Error("last segment non-numeric should return 0")
	}
	if pathLast("/a/b/c/d/e/f/123") != 123 {
		t.Error("last segment numeric should return value")
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
