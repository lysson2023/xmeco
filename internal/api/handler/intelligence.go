package handler

import (
	"encoding/json"
	"net/http"

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
