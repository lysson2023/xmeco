package handler

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"xmeco/internal/service/intelligence"
)

type IntelligenceHandler struct {
	svc *intelligence.Service
}

func NewIntelligenceHandler(pool *pgxpool.Pool) *IntelligenceHandler {
	return &IntelligenceHandler{svc: intelligence.New(pool)}
}

// FullAnalysis runs all intelligence analyses and returns bundled results.
func (h *IntelligenceHandler) FullAnalysis(w http.ResponseWriter, r *http.Request) {
	result, err := h.svc.RunFullAnalysis(r.Context())
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, M{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, result)
}

// Efficiency returns device efficiency analysis.
func (h *IntelligenceHandler) Efficiency(w http.ResponseWriter, r *http.Request) {
	items, err := h.svc.AnalyzeEfficiency(r.Context())
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, M{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, items)
}

// Forecast returns 24h load forecast.
func (h *IntelligenceHandler) Forecast(w http.ResponseWriter, r *http.Request) {
	temp, _ := h.svc.GetWeatherTemp(r.Context())
	forecast := h.svc.ForecastLoad(r.Context(), temp)
	writeJSON(w, http.StatusOK, forecast)
}

// Recommendations returns setpoint optimization suggestions.
func (h *IntelligenceHandler) Recommendations(w http.ResponseWriter, r *http.Request) {
	temp, _ := h.svc.GetWeatherTemp(r.Context())
	recs := h.svc.RecommendSetpoints(r.Context(), temp)
	writeJSON(w, http.StatusOK, recs)
}

// Strategies runs all cooperative control strategies.
func (h *IntelligenceHandler) Strategies(w http.ResponseWriter, r *http.Request) {
	result, err := h.svc.RunStrategies(r.Context())
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, M{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, result)
}

// PriceConfig returns current electricity price configuration.
func (h *IntelligenceHandler) PriceConfig(w http.ResponseWriter, r *http.Request) {
	tactic := h.svc.PriceTacticPublic(r.Context())
	writeJSON(w, http.StatusOK, tactic)
}

// SavePriceConfig saves electricity price configuration.
func (h *IntelligenceHandler) SavePriceConfig(w http.ResponseWriter, r *http.Request) {
	var periods []intelligence.PricePeriod
	if err := json.NewDecoder(r.Body).Decode(&periods); err != nil {
		writeJSON(w, http.StatusBadRequest, M{"error": "请求格式错误"})
		return
	}
	if err := h.svc.SavePriceConfig(r.Context(), periods); err != nil {
		writeJSON(w, http.StatusInternalServerError, M{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, M{"status": "saved"})
}

// PowerQuality runs power quality analysis for a given device and time range.
func (h *IntelligenceHandler) PowerQuality(w http.ResponseWriter, r *http.Request) {
	deviceID := queryInt(r, "device_id")
	if deviceID <= 0 {
		writeJSON(w, http.StatusBadRequest, M{"error": "缺少 device_id 参数"})
		return
	}
	startStr := r.URL.Query().Get("start")
	endStr := r.URL.Query().Get("end")
	var start, end time.Time
	var err error
	parseTime := func(s string) (time.Time, error) {
		if s == "" {
			return time.Time{}, nil
		}
		t, e := time.Parse(time.RFC3339, s)
		if e != nil {
			t, e = time.Parse("2006-01-02T15:04:05", s)
		}
		if e != nil {
			t, e = time.Parse("2006-01-02", s)
		}
		return t, e
	}
	if startStr != "" {
		start, err = parseTime(startStr)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, M{"error": "start 时间格式错误"})
			return
		}
	}
	if endStr != "" {
		end, err = parseTime(endStr)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, M{"error": "end 时间格式错误"})
			return
		}
	}
	// default: last 24h
	if start.IsZero() && end.IsZero() {
		end = time.Now()
		start = end.Add(-24 * time.Hour)
	} else if start.IsZero() {
		end = time.Now()
		start = end.Add(-24 * time.Hour)
	} else if end.IsZero() {
		end = time.Now()
	}

	result, err := h.svc.AnalyzePowerQuality(r.Context(), deviceID, start, end)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, M{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, result)
}

// MeterDevices returns the list of electric meter devices for the dropdown.
func (h *IntelligenceHandler) MeterDevices(w http.ResponseWriter, r *http.Request) {
	buildingID := queryInt(r, "building_id")
	devices, err := h.svc.ListMeters(r.Context(), buildingID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, M{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, devices)
}
