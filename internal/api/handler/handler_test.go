package handler

import (
	"errors"
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

func TestPathID(t *testing.T) {
	tests := []struct {
		name      string
		pathValue string
		want      int
	}{
		{"numeric", "5", 5},
		{"zero", "0", 0},
		{"empty", "", 0},
		{"non-numeric", "abc", 0},
		{"large number", "12345", 12345},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := httptest.NewRequest("GET", "/", nil)
			if tt.pathValue != "" {
				r.SetPathValue("id", tt.pathValue)
			}
			got := pathID(r)
			if got != tt.want {
				t.Errorf("pathID() = %d, want %d", got, tt.want)
			}
		})
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
	serverErr(rec, errors.New("database connection failed"))
	if rec.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want 500", rec.Code)
	}
	ct := rec.Header().Get("Content-Type")
	if ct != "application/json" {
		t.Errorf("Content-Type = %q, want application/json", ct)
	}
}

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
